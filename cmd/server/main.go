// Command server runs the Kova HTTP API and app.
package main

import (
	"context"
	"log"
	"net/http"

	"kova/internal/api"
	"kova/internal/config"
	"kova/internal/db"
	"kova/internal/email"
	"kova/internal/extract/gofitz"
	"kova/internal/secretbox"
	"kova/internal/store"
)

func main() {
	cfg := config.Load()

	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	enc, err := secretbox.New(cfg.SecretEncKey)
	if err != nil {
		log.Fatalf("secret encryption: %v", err)
	}

	var mailer email.Sender = email.Noop{}
	if cfg.ResendAPIKey != "" && cfg.EmailFrom != "" {
		mailer = email.NewResend(cfg.ResendAPIKey, cfg.EmailFrom)
	}

	srv := api.New(
		gofitz.New(),
		store.New(pool),
		api.GitHubConfig{ClientID: cfg.GitHubID, ClientSecret: cfg.GitHubSecret, BaseURL: cfg.BaseURL},
		mailer,
		enc,
	)

	log.Printf("kova listening on %s", cfg.Addr)
	srv.StartRepaymentScheduler()
	if err := http.ListenAndServe(cfg.Addr, srv.Handler()); err != nil {
		log.Fatal(err)
	}
}
