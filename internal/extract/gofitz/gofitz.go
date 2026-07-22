// Package gofitz implements extract.Extractor using go-fitz (MuPDF bindings).
// Isolated so pure-logic packages and tests never depend on the native library.
package gofitz

import (
	"context"
	"fmt"
	"strings"

	"github.com/gen2brain/go-fitz"

	"kova/internal/extract"
)

// Extractor extracts text (and per-page text) from PDFs via MuPDF.
type Extractor struct{}

func New() *Extractor { return &Extractor{} }

func (e *Extractor) Extract(ctx context.Context, filename string, data []byte) (*extract.Document, error) {
	_ = ctx
	doc, err := fitz.NewFromMemory(data)
	if err != nil {
		return nil, fmt.Errorf("open pdf %q: %w", filename, err)
	}
	defer doc.Close()

	out := &extract.Document{Filename: filename}
	var sb strings.Builder
	for n := 0; n < doc.NumPage(); n++ {
		text, err := doc.Text(n)
		if err != nil {
			return nil, fmt.Errorf("extract page %d of %q: %w", n+1, filename, err)
		}
		out.Pages = append(out.Pages, extract.Page{Number: n + 1, Content: text})
		sb.WriteString(text)
		sb.WriteString("\n")
	}
	out.Content = sb.String()
	return out, nil
}
