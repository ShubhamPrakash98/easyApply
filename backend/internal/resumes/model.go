package resumes

import (
	"errors"
	"time"
)

type Resume struct {
	ID            string
	UserID        string
	Label         string
	StorageURL    string // for local storage: "data/resumes/<user_id>/<uuid>.pdf"
	ExtractedText string
	CreatedAt     time.Time
}

var (
	ErrNotFound    = errors.New("resume not found")
	ErrTooLarge    = errors.New("resume file too large")
	ErrUnsupported = errors.New("only PDF is supported")
)
