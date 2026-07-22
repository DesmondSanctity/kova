package extract

import (
	"context"
	"path/filepath"
	"strings"
)

// Page is one extracted page of a document. Kept for future bounding-box /
// citation support so a score can trace back to the source line.
type Page struct {
	Number  int    `json:"number"`
	Content string `json:"content"`
}

// Document is the raw extraction output for a single uploaded file.
type Document struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
	Pages    []Page `json:"pages"`
}

// Extractor turns a document (PDF, image, etc.) into text. Implementations are
// pluggable: go-fitz (MuPDF) today; Datalab or an open-banking adapter later.
type Extractor interface {
	Extract(ctx context.Context, filename string, data []byte) (*Document, error)
}

// MimeFor returns a best-effort MIME type for a filename, used by extractors
// that accept raw bytes.
func MimeFor(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".pdf":
		return "application/pdf"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".csv":
		return "text/csv"
	default:
		return "application/octet-stream"
	}
}
