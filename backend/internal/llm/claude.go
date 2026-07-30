package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ClaudeLLM talks to the Anthropic /v1/messages endpoint directly.
// - Uses tool_use to force structured output (draft returns {subject, body}
//   via the `emit_draft` tool; resume-match returns {resume_id, rationale}
//   via `pick_resume`).
// - Uses prompt caching on the system prompt to keep repeat drafts cheap.
type ClaudeLLM struct {
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

const anthropicAPIVersion = "2023-06-01"

// Default model. Sonnet 4.6 = good quality/cost balance for cold-email drafts.
// Bump to opus-4-7 if you want maximum quality (~5× more expensive).
const defaultModel = "claude-sonnet-4-6"

func NewClaudeLLM(apiKey string) *ClaudeLLM {
	return &ClaudeLLM{
		APIKey:     apiKey,
		Model:      defaultModel,
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
	}
}

var ErrNoAPIKey = errors.New("ANTHROPIC_API_KEY is not set")

// ---- DraftEmail ----

const draftSystemPrompt = `You are OneApply, an assistant that writes cold emails from job seekers to recruiters.

Rules — never break these:
- 100 to 160 words total. Never longer.
- No emojis. No em-dashes overused. Plain professional tone.
- Open with one specific reference to the recruiter's profile or company. Never generic ("I hope this finds you well").
- One clear ask — a short 15-minute chat. Never demand a decision or attach expectations.
- Reference the JD naturally, do NOT restate it back at them.
- If the resume text is provided, weave in ONE concrete relevant fact from it. Just one.
- Sign with the sender's name only. No slogans, no signatures, no "Sent from…".

Format your reply by calling the emit_draft tool with a subject line and body. Nothing else.`

func (c *ClaudeLLM) DraftEmail(ctx context.Context, req DraftRequest) (*Draft, error) {
	if c.APIKey == "" {
		return nil, ErrNoAPIKey
	}

	userMsg := buildDraftUserPrompt(req)

	body := map[string]any{
		"model":      c.Model,
		"max_tokens": 800,
		"system": []map[string]any{{
			"type": "text",
			"text": draftSystemPrompt,
			"cache_control": map[string]string{"type": "ephemeral"},
		}},
		"tools": []map[string]any{{
			"name":        "emit_draft",
			"description": "Emit the final cold email draft.",
			"input_schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"subject": map[string]any{
						"type":        "string",
						"description": "Subject line, under 60 chars. Specific, not generic.",
					},
					"body": map[string]any{
						"type":        "string",
						"description": "Email body, 100-160 words. Plain text with line breaks.",
					},
				},
				"required": []string{"subject", "body"},
			},
		}},
		"tool_choice": map[string]string{"type": "tool", "name": "emit_draft"},
		"messages": []map[string]any{{
			"role":    "user",
			"content": userMsg,
		}},
	}

	raw, err := c.call(ctx, body)
	if err != nil {
		return nil, err
	}
	toolInput, err := extractToolInput(raw, "emit_draft")
	if err != nil {
		return nil, err
	}
	var out struct {
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}
	if err := json.Unmarshal(toolInput, &out); err != nil {
		return nil, fmt.Errorf("draft tool_use decode: %w", err)
	}
	if out.Subject == "" || out.Body == "" {
		return nil, errors.New("empty draft returned")
	}
	return &Draft{Subject: strings.TrimSpace(out.Subject), Body: strings.TrimSpace(out.Body)}, nil
}

func buildDraftUserPrompt(req DraftRequest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Sender: %s <%s>\n", nonEmpty(req.SenderName, "the sender"), req.SenderEmail)
	fmt.Fprintf(&b, "Recruiter: %s\n", nonEmpty(req.RecruiterName, "the recruiter"))
	if req.RecruiterHeadline != "" {
		fmt.Fprintf(&b, "Recruiter headline: %s\n", req.RecruiterHeadline)
	}
	if req.Company != "" {
		fmt.Fprintf(&b, "Company: %s\n", req.Company)
	}
	fmt.Fprintf(&b, "\n--- Job description (verbatim) ---\n%s\n--- end JD ---\n", strings.TrimSpace(req.JobDescription))
	if req.ResumeText != "" {
		text := req.ResumeText
		if len(text) > 4000 {
			text = text[:4000] + "\n[…resume truncated…]"
		}
		fmt.Fprintf(&b, "\n--- Sender's resume (\"%s\", extracted text) ---\n%s\n--- end resume ---\n", req.ResumeLabel, text)
	}
	b.WriteString("\nWrite the draft now.")
	return b.String()
}

// ---- MatchResume ----

const matchSystemPrompt = `You are OneApply's resume router. You get a job description and a list of candidate resumes (labels + short extracted text). Pick the ONE resume that best matches the JD's core requirements.

Prefer:
- Direct skill overlap (languages, frameworks, systems)
- Domain fit (fintech vs healthcare vs consumer, etc.)
- Seniority match

Reply by calling the pick_resume tool with the chosen resume_id and a one-sentence rationale.`

func (c *ClaudeLLM) MatchResume(ctx context.Context, req MatchResumeRequest) (*ResumeMatch, error) {
	if c.APIKey == "" {
		return nil, ErrNoAPIKey
	}
	if len(req.Resumes) == 0 {
		return nil, nil
	}
	if len(req.Resumes) == 1 {
		return &ResumeMatch{ResumeID: req.Resumes[0].ID, Rationale: "only resume available"}, nil
	}

	var user strings.Builder
	fmt.Fprintf(&user, "Job description:\n%s\n\nResumes:\n", strings.TrimSpace(req.JobDescription))
	for _, r := range req.Resumes {
		text := r.Text
		if len(text) > 1500 {
			text = text[:1500]
		}
		fmt.Fprintf(&user, "\n---\nresume_id: %s\nlabel: %s\nextracted:\n%s\n", r.ID, r.Label, text)
	}
	user.WriteString("\n---\nPick one via the tool.")

	body := map[string]any{
		"model":      c.Model,
		"max_tokens": 300,
		"system": []map[string]any{{
			"type":          "text",
			"text":          matchSystemPrompt,
			"cache_control": map[string]string{"type": "ephemeral"},
		}},
		"tools": []map[string]any{{
			"name":        "pick_resume",
			"description": "Choose the best-matching resume.",
			"input_schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"resume_id": map[string]any{"type": "string"},
					"rationale": map[string]any{"type": "string"},
				},
				"required": []string{"resume_id"},
			},
		}},
		"tool_choice": map[string]string{"type": "tool", "name": "pick_resume"},
		"messages": []map[string]any{{
			"role":    "user",
			"content": user.String(),
		}},
	}
	raw, err := c.call(ctx, body)
	if err != nil {
		return nil, err
	}
	toolInput, err := extractToolInput(raw, "pick_resume")
	if err != nil {
		return nil, err
	}
	var out struct {
		ResumeID  string `json:"resume_id"`
		Rationale string `json:"rationale"`
	}
	if err := json.Unmarshal(toolInput, &out); err != nil {
		return nil, fmt.Errorf("match tool_use decode: %w", err)
	}
	// Validate against candidates so the model can't hallucinate an ID.
	valid := false
	for _, r := range req.Resumes {
		if r.ID == out.ResumeID {
			valid = true
			break
		}
	}
	if !valid {
		out.ResumeID = req.Resumes[0].ID
		out.Rationale = "model returned unknown id; fell back to first resume"
	}
	return &ResumeMatch{ResumeID: out.ResumeID, Rationale: out.Rationale}, nil
}

// ---- transport helpers ----

func (c *ClaudeLLM) call(ctx context.Context, body map[string]any) ([]byte, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", anthropicAPIVersion)

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic request: %w", err)
	}
	defer res.Body.Close()

	respBody, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("anthropic %d: %s", res.StatusCode, snippet(respBody))
	}
	return respBody, nil
}

// extractToolInput pulls the input JSON of the first tool_use content block
// with the given name.
func extractToolInput(raw []byte, toolName string) (json.RawMessage, error) {
	var resp struct {
		Content []struct {
			Type  string          `json:"type"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decode anthropic response: %w", err)
	}
	for _, c := range resp.Content {
		if c.Type == "tool_use" && c.Name == toolName {
			return c.Input, nil
		}
	}
	return nil, fmt.Errorf("no tool_use block for %s in response", toolName)
}

func nonEmpty(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

func snippet(b []byte) string {
	if len(b) > 400 {
		return string(b[:400]) + "…"
	}
	return string(b)
}
