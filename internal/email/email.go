// Package email sends transactional email. Resend talks to the Resend API; Noop
// logs instead of sending and is used when no provider is configured.
package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// Sender delivers a single HTML email.
type Sender interface {
	Send(ctx context.Context, to, subject, html string) error
}

// Noop logs emails instead of sending them.
type Noop struct{}

func (Noop) Send(_ context.Context, to, subject, _ string) error {
	log.Printf("email(noop): to=%s subject=%q (not sent — no provider configured)", to, subject)
	return nil
}

// Resend sends via https://resend.com.
type Resend struct {
	apiKey string
	from   string
	client *http.Client
}

func NewResend(apiKey, from string) *Resend {
	return &Resend{apiKey: apiKey, from: from, client: http.DefaultClient}
}

func (r *Resend) Send(ctx context.Context, to, subject, html string) error {
	payload, _ := json.Marshal(map[string]any{
		"from":    r.from,
		"to":      []string{to},
		"subject": subject,
		"html":    html,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resend: %s: %s", resp.Status, string(b))
	}
	return nil
}
