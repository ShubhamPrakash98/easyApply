package resumes

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/ledongthuc/pdf"
)

// ExtractText reads a PDF from r and returns its plain text.
// Extraction quality varies (image-only PDFs → empty text). We return
// what we get; the caller can fall back to just the resume label.
func ExtractText(r io.ReaderAt, size int64) (string, error) {
	reader, err := pdf.NewReader(r, size)
	if err != nil {
		return "", fmt.Errorf("pdf open: %w", err)
	}
	var b strings.Builder
	nPages := reader.NumPage()
	for i := 1; i <= nPages; i++ {
		p := reader.Page(i)
		if p.V.IsNull() {
			continue
		}
		text, err := p.GetPlainText(nil)
		if err != nil {
			// Skip pages we can't decode rather than fail the whole upload.
			continue
		}
		b.WriteString(text)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String()), nil
}

// ExtractTextFromBytes is a convenience wrapper.
func ExtractTextFromBytes(data []byte) (string, error) {
	return ExtractText(bytes.NewReader(data), int64(len(data)))
}
