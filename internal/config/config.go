// Package config loads runtime configuration from the environment.
package config

import "os"

type Config struct {
	Addr         string
	DatabaseURL  string
	BaseURL      string
	GitHubID     string
	GitHubSecret string
	ResendAPIKey string
	EmailFrom    string
	SecretEncKey string
}

func Load() Config {
	return Config{
		Addr:         env("KOVA_ADDR", ":8080"),
		DatabaseURL:  env("DATABASE_URL", "postgres://kova:kova@localhost:5433/kova?sslmode=disable"),
		BaseURL:      os.Getenv("KOVA_BASE_URL"),
		GitHubID:     os.Getenv("KOVA_GITHUB_CLIENT_ID"),
		GitHubSecret: os.Getenv("KOVA_GITHUB_CLIENT_SECRET"),
		ResendAPIKey: os.Getenv("RESEND_API_KEY"),
		EmailFrom:    env("EMAIL_FROM", "Kova <onboarding@resend.dev>"),
		SecretEncKey: os.Getenv("KOVA_SECRET_ENC_KEY"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
