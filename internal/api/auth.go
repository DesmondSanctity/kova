package api

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"kova/internal/store"
)

// Keyring holds optional env-configured keys (useful for a fixed demo/integration
// key). Per-workspace keys live in the database via the store.
type Keyring struct {
	publishable map[string]struct{}
	secret      map[string]struct{}
	enabled     bool
}

type scope int

const (
	scopeAny scope = iota
	scopeSecret
)

type ctxKey int

const keyAuthCtxKey ctxKey = 0

func LoadKeyring() Keyring {
	pk := splitKeys(os.Getenv("KOVA_PUBLISHABLE_KEYS"))
	sk := splitKeys(os.Getenv("KOVA_SECRET_KEYS"))
	return Keyring{publishable: pk, secret: sk, enabled: len(pk)+len(sk) > 0}
}

func splitKeys(s string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, k := range strings.Split(s, ",") {
		if k = strings.TrimSpace(k); k != "" {
			out[k] = struct{}{}
		}
	}
	return out
}

func (kr Keyring) allow(key string, sc scope) bool {
	if _, ok := kr.secret[key]; ok {
		return true
	}
	if sc == scopeAny {
		_, ok := kr.publishable[key]
		return ok
	}
	return false
}

func apiKey(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(h[len("Bearer "):])
	}
	if h := r.Header.Get("X-Kova-Key"); h != "" {
		return h
	}
	return r.URL.Query().Get("key")
}

func (s *Server) protect(sc scope, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := s.authorizeKey(r, sc)
		if !ok {
			writeErr(w, http.StatusUnauthorized, "missing or invalid API key")
			return
		}
		h(w, r.WithContext(context.WithValue(r.Context(), keyAuthCtxKey, auth)))
	}
}

func keyAuthFrom(ctx context.Context) (store.KeyAuth, bool) {
	a, ok := ctx.Value(keyAuthCtxKey).(store.KeyAuth)
	return a, ok
}

// authorizeKey validates the request's API key (DB + env keys) and enforces per-key domain/IP allowlists.
func (s *Server) authorizeKey(r *http.Request, sc scope) (store.KeyAuth, bool) {
	key := apiKey(r)
	if key == "" {
		return store.KeyAuth{}, false
	}
	if s.store != nil {
		if auth, ok := s.store.Authorize(r.Context(), key, sc == scopeSecret); ok {
			if !allowlistOK(r, auth) {
				return store.KeyAuth{}, false
			}
			return auth, true
		}
	}
	if s.keyring.enabled && s.keyring.allow(key, sc) {
		return store.KeyAuth{}, true
	}
	return store.KeyAuth{}, false
}

// allowlistOK enforces a key's domain and IP allowlists when configured.
func allowlistOK(r *http.Request, a store.KeyAuth) bool {
	if len(a.AllowedDomains) > 0 {
		host := originHost(r)
		if host == "" || !contains(a.AllowedDomains, host) {
			return false
		}
	}
	if len(a.AllowedIPs) > 0 {
		if !contains(a.AllowedIPs, clientIP(r)) {
			return false
		}
	}
	return true
}

func originHost(r *http.Request) string {
	src := r.Header.Get("Origin")
	if src == "" {
		src = r.Header.Get("Referer")
	}
	if u, err := url.Parse(src); err == nil {
		return u.Hostname()
	}
	return ""
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if strings.EqualFold(strings.TrimSpace(x), v) {
			return true
		}
	}
	return false
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, X-Kova-Key, Content-Type")
		w.Header().Set("Access-Control-Max-Age", "86400")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
