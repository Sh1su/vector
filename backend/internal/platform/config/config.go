// Package config liest die Konfiguration aus Umgebungsvariablen (ADR-030).
package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL     string
	Listen          string
	CookieSecure    bool
	SetupToken      string
	OdometerVMaxKmh int
	WebDir          string // optional: statische Web-App ausliefern (Entwicklung/ohne Caddy)
	StorageDir      string // Ablage der Dateien (ADR-017), nie im Web-Pfad
	MaxUploadMB     int
	Assistant       Assistant
}

// Assistant konfiguriert den KI-Assistenten (ADR-024). Ohne Provider ist er aus.
type Assistant struct {
	Provider      string // "" | "anthropic"
	APIKey        string
	Model         string
	BaseURL       string
	WebSearch     bool
	DailyLimit    int
	RetentionDays int
}

func Load() (Config, error) {
	c := Config{
		DatabaseURL:     os.Getenv("VECTRA_DATABASE_URL"),
		Listen:          env("VECTRA_LISTEN", ":8080"),
		CookieSecure:    env("VECTRA_COOKIE_SECURE", "true") == "true",
		SetupToken:      os.Getenv("VECTRA_SETUP_TOKEN"),
		WebDir:          os.Getenv("VECTRA_WEB_DIR"),
		OdometerVMaxKmh: 250,
		StorageDir:      env("VECTRA_STORAGE_DIR", "./data/files"),
		MaxUploadMB:     25,
	}
	if v := os.Getenv("VECTRA_ODOMETER_VMAX_KMH"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return c, fmt.Errorf("VECTRA_ODOMETER_VMAX_KMH: ungültig")
		}
		c.OdometerVMaxKmh = n
	}
	if v := os.Getenv("VECTRA_MAX_UPLOAD_MB"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 || n > 200 {
			return c, fmt.Errorf("VECTRA_MAX_UPLOAD_MB: 1–200")
		}
		c.MaxUploadMB = n
	}
	c.Assistant = Assistant{
		Provider:  os.Getenv("VECTRA_ASSISTANT_PROVIDER"),
		APIKey:    env("VECTRA_ASSISTANT_API_KEY", os.Getenv("ANTHROPIC_API_KEY")),
		Model:     os.Getenv("VECTRA_ASSISTANT_MODEL"),
		BaseURL:   os.Getenv("VECTRA_ASSISTANT_BASE_URL"),
		WebSearch: env("VECTRA_ASSISTANT_WEB_SEARCH", "true") == "true",
	}
	for k, dst := range map[string]*int{"VECTRA_ASSISTANT_DAILY_LIMIT": &c.Assistant.DailyLimit, "VECTRA_ASSISTANT_RETENTION_DAYS": &c.Assistant.RetentionDays} {
		if v := os.Getenv(k); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 {
				return c, fmt.Errorf("%s: ungültig", k)
			}
			*dst = n
		}
	}
	switch c.Assistant.Provider {
	case "":
	case "anthropic":
		if c.Assistant.APIKey == "" || c.Assistant.Model == "" {
			return c, fmt.Errorf("VECTRA_ASSISTANT_PROVIDER=anthropic braucht ANTHROPIC_API_KEY und VECTRA_ASSISTANT_MODEL (z. B. claude-sonnet-4-5)")
		}
	default:
		return c, fmt.Errorf("VECTRA_ASSISTANT_PROVIDER: unbekannt (erlaubt: anthropic)")
	}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("VECTRA_DATABASE_URL fehlt")
	}
	return c, nil
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
