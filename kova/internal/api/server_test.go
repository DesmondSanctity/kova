package api

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"kova/internal/extract"
)

// stubExtractor returns fixed content regardless of input, so handler tests run
// without the native PDF library.
type stubExtractor struct{ content string }

func (s stubExtractor) Extract(_ context.Context, name string, _ []byte) (*extract.Document, error) {
	return &extract.Document{Filename: name, Content: s.content}, nil
}

func fixtureContent(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "parse", "testdata", "opay_statement.txt"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return string(data)
}

func multipartBody(t *testing.T, field, filename string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile(field, filename)
	if err != nil {
		t.Fatal(err)
	}
	fw.Write([]byte("%PDF-1.4 dummy bytes"))
	mw.Close()
	return &buf, mw.FormDataContentType()
}

func TestHealth(t *testing.T) {
	srv := newServer(t, stubExtractor{})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestScoreEndpoint(t *testing.T) {
	t.Setenv("KOVA_PUBLISHABLE_KEYS", "pk_test")
	srv := newServer(t, stubExtractor{content: fixtureContent(t)})
	body, ct := multipartBody(t, "statements", "opay.pdf")
	req := httptest.NewRequest(http.MethodPost, "/v1/score", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Authorization", "Bearer pk_test")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var rep struct {
		Files []struct {
			Bank   string `json:"bank"`
			Parsed bool   `json:"parsed"`
		} `json:"files"`
		Score struct {
			Score int    `json:"score"`
			Band  string `json:"band"`
		} `json:"score"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &rep); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(rep.Files) != 1 || !rep.Files[0].Parsed || rep.Files[0].Bank != "OPay" {
		t.Errorf("files = %+v", rep.Files)
	}
	if rep.Score.Score <= 0 || rep.Score.Band == "" {
		t.Errorf("score = %+v", rep.Score)
	}
}

func TestScoreRequiresFiles(t *testing.T) {
	t.Setenv("KOVA_PUBLISHABLE_KEYS", "pk_test")
	srv := newServer(t, stubExtractor{})
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/v1/score", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer pk_test")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestVerifyWithoutMonnify(t *testing.T) {
	t.Setenv("KOVA_SECRET_KEYS", "sk_test")
	srv := newServer(t, stubExtractor{})
	req := httptest.NewRequest(http.MethodPost, "/v1/verify-account", bytes.NewBufferString(`{}`))
	req.Header.Set("Authorization", "Bearer sk_test")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}
