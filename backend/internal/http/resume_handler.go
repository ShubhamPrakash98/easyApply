package http

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/shubham/oneapply/backend/internal/auth"
	"github.com/shubham/oneapply/backend/internal/resumes"
)

const maxResumeBytes = 5 * 1024 * 1024 // 5 MB

type ResumeHandler struct {
	repo    *resumes.Repo
	storage resumes.Storage
}

func NewResumeHandler(repo *resumes.Repo, storage resumes.Storage) *ResumeHandler {
	return &ResumeHandler{repo: repo, storage: storage}
}

func (h *ResumeHandler) Mount(r chi.Router) {
	r.Post("/resumes", h.upload)
	r.Get("/resumes", h.list)
	r.Delete("/resumes/{id}", h.delete)
}

type resumeResponse struct {
	ID                string `json:"id"`
	Label             string `json:"label"`
	CreatedAt         string `json:"created_at"`
	ExtractedTextLen  int    `json:"extracted_text_len"`
}

func (h *ResumeHandler) upload(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())

	if err := r.ParseMultipartForm(maxResumeBytes); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid form: "+err.Error())
		return
	}
	label := strings.TrimSpace(r.FormValue("label"))
	if label == "" {
		writeErr(w, http.StatusBadRequest, "label is required")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	if header.Size > maxResumeBytes {
		writeErr(w, http.StatusRequestEntityTooLarge, "file too large (max 5MB)")
		return
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".pdf" {
		writeErr(w, http.StatusUnsupportedMediaType, "only PDF is supported")
		return
	}

	// Buffer so we can both save and extract text from the same bytes.
	buf, err := io.ReadAll(io.LimitReader(file, maxResumeBytes+1))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "read: "+err.Error())
		return
	}
	if len(buf) > maxResumeBytes {
		writeErr(w, http.StatusRequestEntityTooLarge, "file too large (max 5MB)")
		return
	}

	relPath, err := h.storage.Save(userID, header.Filename, bytes.NewReader(buf))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "storage: "+err.Error())
		return
	}

	text, err := resumes.ExtractTextFromBytes(buf)
	if err != nil {
		// Non-fatal: proceed with empty text (image-only PDFs).
		text = ""
	}

	created, err := h.repo.Create(r.Context(), resumes.CreateParams{
		UserID:        userID,
		Label:         label,
		StorageURL:    relPath,
		ExtractedText: text,
	})
	if err != nil {
		_ = h.storage.Delete(relPath)
		writeErr(w, http.StatusInternalServerError, "db: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, resumeResponse{
		ID:               created.ID,
		Label:            created.Label,
		CreatedAt:        created.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		ExtractedTextLen: len(created.ExtractedText),
	})
}

func (h *ResumeHandler) list(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	list, err := h.repo.ListForUser(r.Context(), userID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]resumeResponse, 0, len(list))
	for _, r := range list {
		out = append(out, resumeResponse{
			ID:               r.ID,
			Label:            r.Label,
			CreatedAt:        r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
			ExtractedTextLen: len(r.ExtractedText),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *ResumeHandler) delete(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	// Load first so we know the storage path.
	res, err := h.repo.GetForUser(r.Context(), id, userID)
	if err != nil {
		if errors.Is(err, resumes.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "resume not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.repo.DeleteForUser(r.Context(), id, userID); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = h.storage.Delete(res.StorageURL)
	w.WriteHeader(http.StatusNoContent)
}
