package finder

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ApolloFinder is the last stage of the cascade. It also satisfies EmailFinder
// so callers can use it standalone. If APIKey is empty it becomes a no-op
// that always returns ErrEmailNotFound (letting the app run without an
// Apollo key during MVP).
type ApolloFinder struct {
	APIKey     string
	HTTPClient *http.Client
	// BaseURL overridable for tests.
	BaseURL string
}

func NewApolloFinder(apiKey string) *ApolloFinder {
	return &ApolloFinder{
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		BaseURL:    "https://api.apollo.io",
	}
}

var ErrEmailNotFound = errors.New("email not found via apollo")

// Enabled reports whether the adapter has credentials.
func (a *ApolloFinder) Enabled() bool { return a != nil && a.APIKey != "" }

func (a *ApolloFinder) FindEmail(ctx context.Context, req FindEmailRequest) (*Result, error) {
	if !a.Enabled() {
		return nil, ErrEmailNotFound
	}

	reqBody := map[string]any{
		"first_name":       firstToken(req.Name),
		"last_name":        lastToken(req.Name),
		"organization_name": req.Company,
	}
	if req.LinkedInURL != "" {
		reqBody["linkedin_url"] = req.LinkedInURL
	}

	body, _ := json.Marshal(reqBody)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.BaseURL+"/v1/people/match", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Cache-Control", "no-cache")
	httpReq.Header.Set("X-Api-Key", a.APIKey)

	res, err := a.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("apollo request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return nil, ErrEmailNotFound
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return nil, fmt.Errorf("apollo %d: %s", res.StatusCode, string(snippet))
	}

	var payload struct {
		Person struct {
			Email        string `json:"email"`
			EmailStatus  string `json:"email_status"`
			Organization struct {
				PrimaryDomain string `json:"primary_domain"`
			} `json:"organization"`
		} `json:"person"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("apollo decode: %w", err)
	}
	if payload.Person.Email == "" {
		return nil, ErrEmailNotFound
	}

	// Map Apollo's status vocabulary to ours.
	verStatus := "unknown"
	switch payload.Person.EmailStatus {
	case "verified":
		verStatus = "deliverable"
	case "guessed", "unavailable":
		verStatus = "risky"
	case "bogus":
		verStatus = "invalid"
	}

	return &Result{
		Email:              payload.Person.Email,
		Source:             "apollo",
		VerificationStatus: verStatus,
		CompanyDomain:      payload.Person.Organization.PrimaryDomain,
	}, nil
}

func firstToken(full string) string {
	first, _ := splitFirstLast(full)
	return first
}

func lastToken(full string) string {
	_, last := splitFirstLast(full)
	return last
}
