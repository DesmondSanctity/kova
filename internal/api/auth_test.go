package api

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func multipartN(t *testing.T, n int) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for i := 0; i < n; i++ {
		fw, err := mw.CreateFormFile("statements", "s.pdf")
		if err != nil {
			t.Fatal(err)
		}
		fw.Write([]byte("%PDF-1.4"))
	}
	mw.Close()
	return &buf, mw.FormDataContentType()
}

func TestAuthEnforcedWhenKeysConfigured(t *testing.T) {
	t.Setenv("KOVA_PUBLISHABLE_KEYS", "pk_test")
	srv := newServer(t, stubExtractor{content: "not a statement"})

	body, ct := multipartN(t, 1)
	req := httptest.NewRequest(http.MethodPost, "/v1/score", body)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no key: status = %d, want 401", rec.Code)
	}

	body2, ct2 := multipartN(t, 1)
	req2 := httptest.NewRequest(http.MethodPost, "/v1/score", body2)
	req2.Header.Set("Content-Type", ct2)
	req2.Header.Set("Authorization", "Bearer pk_test")
	rec2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec2, req2)
	if rec2.Code == http.StatusUnauthorized {
		t.Fatalf("valid key rejected: %d", rec2.Code)
	}
}

func TestPublishableKeyCannotDisburse(t *testing.T) {
	t.Setenv("KOVA_PUBLISHABLE_KEYS", "pk_test")
	t.Setenv("KOVA_SECRET_KEYS", "sk_test")
	srv := newServer(t, stubExtractor{})

	req := httptest.NewRequest(http.MethodPost, "/v1/disburse", bytes.NewBufferString(`{}`))
	req.Header.Set("Authorization", "Bearer pk_test")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("publishable key allowed to disburse: %d", rec.Code)
	}
}

func TestMaxBanksLimit(t *testing.T) {
	t.Setenv("KOVA_PUBLISHABLE_KEYS", "pk_test")
	srv := newServer(t, stubExtractor{})
	body, ct := multipartN(t, 4)
	req := httptest.NewRequest(http.MethodPost, "/v1/score", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Authorization", "Bearer pk_test")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("4 banks: status = %d, want 400", rec.Code)
	}
}

func TestCORSPreflight(t *testing.T) {
	srv := newServer(t, stubExtractor{})
	req := httptest.NewRequest(http.MethodOptions, "/v1/score", nil)
	req.Header.Set("Origin", "https://merchant.example")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("missing CORS allow-origin header")
	}
}

func TestBanksEndpoint(t *testing.T) {
	srv := newServer(t, stubExtractor{})
	req := httptest.NewRequest(http.MethodGet, "/v1/banks", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte("OPay")) {
		t.Fatalf("banks endpoint: %d body=%s", rec.Code, rec.Body.String())
	}
}
