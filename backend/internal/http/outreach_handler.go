package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/shubham/oneapply/backend/internal/auth"
	"github.com/shubham/oneapply/backend/internal/contacts"
	"github.com/shubham/oneapply/backend/internal/outreach"
)

// OutreachHandler bundles the routes under /api/outreach.
type OutreachHandler struct {
	svc *outreach.Service
}

func NewOutreachHandler(svc *outreach.Service) *OutreachHandler {
	return &OutreachHandler{svc: svc}
}

func (h *OutreachHandler) Mount(r chi.Router) {
	r.Post("/outreach/find-email", h.findEmail)
	r.Post("/outreach/draft", h.draft)
	r.Get("/outreach", h.list)
	r.Get("/outreach/{id}", h.detail)
	r.Post("/outreach/{id}/approve", h.approve)
	r.Post("/outreach/{id}/cancel", h.cancel)
}

// -- Step 1: Find email --

type findEmailRequest struct {
	RecruiterName string `json:"recruiter_name"`
	Company       string `json:"company"`
	LinkedInURL   string `json:"linkedin_url"`
}

type contactBody struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Email              string `json:"email"`
	Company            string `json:"company"`
	Source             string `json:"source"`
	VerificationStatus string `json:"verification_status"`
}

func (h *OutreachHandler) findEmail(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	var req findEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.RecruiterName == "" {
		writeErr(w, http.StatusBadRequest, "recruiter_name is required")
		return
	}

	res, err := h.svc.FindContact(r.Context(), outreach.FindContactInput{
		UserID:        userID,
		RecruiterName: req.RecruiterName,
		Company:       req.Company,
		LinkedInURL:   req.LinkedInURL,
	})
	if err != nil {
		if errors.Is(err, outreach.ErrEmailNotFound) {
			writeErr(w, http.StatusNotFound, "email_not_found")
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"contact": contactBody{
			ID:                 res.Contact.ID,
			Name:               res.Contact.Name,
			Email:              res.Contact.Email,
			Company:            res.Company,
			Source:             res.Contact.Source,
			VerificationStatus: res.Contact.VerificationStatus,
		},
	})
}

// -- Step 2: Draft (takes contact_id from step 1) --

type draftRequest struct {
	ContactID         string `json:"contact_id"`
	RecruiterHeadline string `json:"recruiter_headline,omitempty"`
	Company           string `json:"company,omitempty"`
	JobDescription    string `json:"job_description"`
	ResumeID          string `json:"resume_id,omitempty"`
}

type draftResponse struct {
	OutreachID string      `json:"outreach_id"`
	Status     string      `json:"status"`
	Draft      draftBody   `json:"draft"`
	Contact    contactBody `json:"contact"`
}

type draftBody struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

func (h *OutreachHandler) draft(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	var req draftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.ContactID == "" || req.JobDescription == "" {
		writeErr(w, http.StatusBadRequest, "contact_id and job_description are required")
		return
	}

	res, err := h.svc.Draft(r.Context(), outreach.DraftInput{
		UserID:            userID,
		ContactID:         req.ContactID,
		RecruiterHeadline: req.RecruiterHeadline,
		Company:           req.Company,
		JobDescription:    req.JobDescription,
		ResumeID:          req.ResumeID,
	})
	if err != nil {
		switch {
		case errors.Is(err, outreach.ErrRateLimited):
			writeErr(w, http.StatusTooManyRequests, "daily rate limit exceeded")
		case errors.Is(err, outreach.ErrEmailNotFound):
			writeErr(w, http.StatusNotFound, "email_not_found")
		default:
			writeErr(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, draftResponse{
		OutreachID: res.Outreach.ID,
		Status:     string(res.Outreach.Status),
		Draft: draftBody{
			Subject: res.Outreach.EmailSubject,
			Body:    res.Outreach.EmailBody,
		},
		Contact: contactBody{
			ID:                 res.Contact.ID,
			Name:               res.Contact.Name,
			Email:              res.Contact.Email,
			Company:            res.Company,
			Source:             res.Contact.Source,
			VerificationStatus: res.Contact.VerificationStatus,
		},
	})
}

type approveRequest struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

func (h *OutreachHandler) approve(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	var req approveRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid json")
			return
		}
	}

	o, err := h.svc.Approve(r.Context(), outreach.ApproveInput{
		UserID:       userID,
		OutreachID:   id,
		FinalSubject: req.Subject,
		FinalBody:    req.Body,
	})
	if err != nil {
		switch {
		case errors.Is(err, outreach.ErrNotFound):
			writeErr(w, http.StatusNotFound, "outreach not found")
		case errors.Is(err, outreach.ErrEmailNotFound):
			writeErr(w, http.StatusConflict, "no email on contact")
		case errors.Is(err, outreach.ErrEmptyDraft):
			writeErr(w, http.StatusBadRequest, "subject and body are required")
		default:
			writeErr(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":              o.ID,
		"status":          string(o.Status),
		"gmail_thread_id": o.GmailThreadID,
		"sent_at":         o.SentAt,
	})
}

func (h *OutreachHandler) cancel(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.svc.Cancel(r.Context(), userID, id); err != nil {
		if errors.Is(err, outreach.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "outreach not found or not pending")
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type listRowResponse struct {
	ID             string  `json:"id"`
	Status         string  `json:"status"`
	Subject        string  `json:"subject"`
	RecruiterName  string  `json:"recruiter_name"`
	RecruiterURL   string  `json:"recruiter_url"`
	Company        string  `json:"company"`
	SentAt         *string `json:"sent_at"`
	CreatedAt      string  `json:"created_at"`
	FollowUpCount  int     `json:"follow_up_count"`
}

func (h *OutreachHandler) list(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	rows, err := h.svc.ListForUser(r.Context(), userID, outreach.ListFilter{
		Status: r.URL.Query().Get("status"),
		Limit:  50,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]listRowResponse, 0, len(rows))
	for _, row := range rows {
		var sentAt *string
		if row.SentAt != nil {
			s := row.SentAt.UTC().Format("2006-01-02T15:04:05Z")
			sentAt = &s
		}
		out = append(out, listRowResponse{
			ID:            row.ID,
			Status:        string(row.Status),
			Subject:       row.EmailSubject,
			RecruiterName: row.RecruiterName,
			RecruiterURL:  row.RecruiterURL,
			Company:       row.CompanyName,
			SentAt:        sentAt,
			CreatedAt:     row.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
			FollowUpCount: row.FollowUpCount,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

type detailResponse struct {
	ID             string  `json:"id"`
	Status         string  `json:"status"`
	Subject        string  `json:"subject"`
	Body           string  `json:"body"`
	JobDescription string  `json:"job_description"`
	CreatedAt      string  `json:"created_at"`
	SentAt         *string `json:"sent_at"`
	GmailThreadID  *string `json:"gmail_thread_id"`
	FollowUpCount  int     `json:"follow_up_count"`
	Contact        contactDetail `json:"contact"`
}

type contactDetail struct {
	Name               string `json:"name"`
	Email              string `json:"email"`
	LinkedInURL        string `json:"linkedin_url"`
	Source             string `json:"source"`
	VerificationStatus string `json:"verification_status"`
}

func (h *OutreachHandler) detail(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	d, err := h.svc.GetDetail(r.Context(), userID, id)
	if err != nil {
		if errors.Is(err, outreach.ErrNotFound) || errors.Is(err, contacts.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "outreach not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	var sentAt *string
	if d.Outreach.SentAt != nil {
		s := d.Outreach.SentAt.UTC().Format("2006-01-02T15:04:05Z")
		sentAt = &s
	}
	writeJSON(w, http.StatusOK, detailResponse{
		ID:             d.Outreach.ID,
		Status:         string(d.Outreach.Status),
		Subject:        d.Outreach.EmailSubject,
		Body:           d.Outreach.EmailBody,
		JobDescription: d.Outreach.JobDescription,
		CreatedAt:      d.Outreach.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		SentAt:         sentAt,
		GmailThreadID:  d.Outreach.GmailThreadID,
		FollowUpCount:  d.Outreach.FollowUpCount,
		Contact: contactDetail{
			Name:               d.Contact.Name,
			Email:              d.Contact.Email,
			LinkedInURL:        d.Contact.LinkedInURL,
			Source:             d.Contact.Source,
			VerificationStatus: d.Contact.VerificationStatus,
		},
	})
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	if status >= 500 {
		slog.Error("http 5xx", "status", status, "msg", msg)
	}
	writeJSON(w, status, map[string]string{"error": msg})
}
