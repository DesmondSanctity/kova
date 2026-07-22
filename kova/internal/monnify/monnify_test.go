package monnify

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestClient(h http.Handler) (*Client, *httptest.Server) {
	srv := httptest.NewServer(h)
	c := New()
	c.BaseURL = srv.URL
	c.APIKey = "MK_TEST_KEY"
	c.SecretKey = "SECRET"
	return c, srv
}

func TestTokenSendsBasicAuthAndCaches(t *testing.T) {
	calls := 0
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/login" {
			calls++
			want := "Basic " + base64.StdEncoding.EncodeToString([]byte("MK_TEST_KEY:SECRET"))
			if got := r.Header.Get("Authorization"); got != want {
				t.Errorf("auth header = %q, want %q", got, want)
			}
			w.Write([]byte(`{"requestSuccessful":true,"responseBody":{"accessToken":"tok123","expiresIn":3000}}`))
			return
		}
		t.Fatalf("unexpected path %s", r.URL.Path)
	}))
	defer srv.Close()

	tok, err := c.Token(context.Background())
	if err != nil || tok != "tok123" {
		t.Fatalf("token = %q err = %v", tok, err)
	}
	if _, err := c.Token(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("login called %d times, want 1 (cached)", calls)
	}
}

func TestVerifyAccount(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/login":
			w.Write([]byte(`{"requestSuccessful":true,"responseBody":{"accessToken":"tok","expiresIn":3000}}`))
		case "/api/v1/disbursements/account/validate":
			if r.Header.Get("Authorization") != "Bearer tok" {
				t.Errorf("missing bearer token")
			}
			if r.URL.Query().Get("accountNumber") != "0123456789" {
				t.Errorf("account number = %q", r.URL.Query().Get("accountNumber"))
			}
			w.Write([]byte(`{"requestSuccessful":true,"responseBody":{"accountNumber":"0123456789","accountName":"JOHN DOE","bankCode":"058"}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	acc, err := c.VerifyAccount(context.Background(), "0123456789", "058")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if acc.AccountName != "JOHN DOE" {
		t.Errorf("account name = %q", acc.AccountName)
	}
}

func TestVerifyAccountFailure(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/login" {
			w.Write([]byte(`{"requestSuccessful":true,"responseBody":{"accessToken":"tok","expiresIn":3000}}`))
			return
		}
		w.Write([]byte(`{"requestSuccessful":false,"responseMessage":"Invalid account"}`))
	}))
	defer srv.Close()

	if _, err := c.VerifyAccount(context.Background(), "000", "058"); err == nil {
		t.Fatal("expected error on failed verification")
	}
}
