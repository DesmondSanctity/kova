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
	Monnify      MonnifyConfig
}

type MonnifyConfig struct {
	BaseURL      string
	APIKey       string
	SecretKey    string
	ContractCode string
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
		Monnify: MonnifyConfig{
			BaseURL:      env("MONNIFY_BASE_URL", "https://sandbox.monnify.com"),
			APIKey:       env("MONNIFY_API_KEY", "MK_TEST_GC3B8XG2XX"),
			SecretKey:    env("MONNIFY_SECRET_KEY", "A663NRZA544DDPEM7KDN7Z8HRV6YXD8S"),
			ContractCode: env("MONNIFY_CONTRACT_CODE", "5867418298"),
		},
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
