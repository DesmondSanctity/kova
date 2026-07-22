// Command dumptext extracts a PDF to plain text (go-fitz) for capturing parser-test fixtures.
//
//	go run ./cmd/dumptext <path-to-pdf> <out.txt>
package main

import (
	"context"
	"fmt"
	"os"

	"kova/internal/extract/gofitz"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: dumptext <file.pdf> <out.txt>")
		os.Exit(2)
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "read:", err)
		os.Exit(1)
	}
	doc, err := gofitz.New().Extract(context.Background(), os.Args[1], data)
	if err != nil {
		fmt.Fprintln(os.Stderr, "extract:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(os.Args[2], []byte(doc.Content), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%d bytes, %d pages)\n", os.Args[2], len(doc.Content), len(doc.Pages))
}
