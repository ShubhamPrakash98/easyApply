package resumes

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Storage abstracts where the resume file lives. LocalStorage writes under
// a base directory (default: `data/resumes/`). Swap for an S3/R2 backed
// implementation later — the interface stays the same.
type Storage interface {
	// Save persists r under the user's namespace and returns the relative
	// storage path (used as the DB storage_url).
	Save(userID string, filename string, r io.Reader) (relPath string, err error)
	// Open returns a ReadCloser for a previously-saved path.
	Open(relPath string) (io.ReadCloser, error)
	// Delete removes the file if present.
	Delete(relPath string) error
}

type LocalStorage struct {
	Root string
}

func NewLocalStorage(root string) *LocalStorage {
	if root == "" {
		root = "data/resumes"
	}
	return &LocalStorage{Root: root}
}

func (l *LocalStorage) Save(userID, filename string, r io.Reader) (string, error) {
	if userID == "" {
		return "", fmt.Errorf("user id required")
	}
	// Random suffix so users can upload two files with the same name.
	suffix := randomHex(6)
	safeName := sanitizeFilename(filename)
	rel := filepath.Join(userID, suffix+"-"+safeName)
	abs := filepath.Join(l.Root, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}
	f, err := os.Create(abs)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return "", err
	}
	return rel, nil
}

func (l *LocalStorage) Open(rel string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(l.Root, rel))
}

func (l *LocalStorage) Delete(rel string) error {
	err := os.Remove(filepath.Join(l.Root, rel))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func sanitizeFilename(name string) string {
	// Keep only alphanumerics, dashes, underscores, dots. Everything else → underscore.
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" {
		return "resume.pdf"
	}
	return out
}

func randomHex(n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
