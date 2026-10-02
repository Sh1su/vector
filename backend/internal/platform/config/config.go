// Package config liest die Konfiguration aus Umgebungsvariablen (ADR-030).
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL     string
	Listen          string
	CookieSecure    bool
	SetupToken      string
	OdometerVMaxKmh int
	WebDir          string // optional: statische Web-App ausliefern (Entwicklung/ohne Caddy)
	Assistant       Assistant
}

// Assistant konfiguriert den optionalen KI-Anbieter (ADR-024). Ohne Provider
// bleibt der Assistent aus; zusätzlich muss ein Admin ihn einschalten.
type Assistant struct {
	Provider      string // anthropic | openai_compatible | ollama
	Name          string
	Model         string
	BaseURL       string
	APIKey        string
	External      bool
	Fallbacks     bool
	DailyLimit    int
	RetentionDays int
	Timeout       time.Duration
}

func Load() (Config, error) {
	c := Config{
		DatabaseURL:     os.Getenv("VECTRA_DATABASE_URL"),
		Listen:          env("VECTRA_LISTEN", ":8080"),
		CookieSecure:    env("VECTRA_COOKIE_SECURE", "true") == "true",
		SetupToken:      os.Getenv("VECTRA_SETUP_TOKEN"),
		WebDir:          os.Getenv("VECTRA_WEB_DIR"),
		OdometerVMaxKmh: 250,
	}
	if v := os.Getenv("VECTRA_ODOMETER_VMAX_KMH"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return c, fmt.Errorf("VECTRA_ODOMETER_VMAX_KMH: ungültig")
		}
		c.OdometerVMaxKmh = n
	}
	a, err := loadAssistant()
	if err != nil {
		return c, err
	}
	c.Assistant = a
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("VECTRA_DATABASE_URL fehlt")
	}
	return c, nil
}

func loadAssistant() (Assistant, error) {
	a := Assistant{Provider: os.Getenv("VECTRA_ASSISTANT_PROVIDER"), Model: os.Getenv("VECTRA_ASSISTANT_MODEL"),
		BaseURL: os.Getenv("VECTRA_ASSISTANT_BASE_URL"), APIKey: os.Getenv("VECTRA_ASSISTANT_API_KEY"), DailyLimit: 200, RetentionDays: 30,
		Timeout: 120 * time.Second, Name: os.Getenv("VECTRA_ASSISTANT_NAME")}
	switch a.Provider {
	case "":
		return a, nil
	case "anthropic":
		if a.APIKey == "" {
			a.APIKey = os.Getenv("ANTHROPIC_API_KEY")
		}
		if a.APIKey == "" {
			return a, fmt.Errorf("VECTRA_ASSISTANT_API_KEY fehlt für den Anbieter anthropic")
		}
		if a.Model == "" {
			a.Model = "claude-opus-5-5"
		}
		a.External, a.Fallbacks = true, true
		if a.Name == "" {
			a.Name = "Claude (Anthropic)"
		}
	case "ollama":
		a.Provider = "openai_compatible"
		if a.BaseURL == "" {
			a.BaseURL = "http://ollama:11434/v1"
		}
		if a.Name == "" {
			a.Name = "Ollama (lokal)"
		}
	case "openai_compatible":
		if a.BaseURL == "" {
			return a, fmt.Errorf("VECTRA_ASSISTANT_BASE_URL fehlt")
		}
		a.External = true // im Zweifel extern; lokal mit VECTRA_ASSISTANT_EXTERNAL=false
		if a.Name == "" {
			a.Name = "OpenAI-kompatibler Dienst"
		}
	default:
		return a, fmt.Errorf("VECTRA_ASSISTANT_PROVIDER: unbekannt (anthropic, openai_compatible, ollama)")
	}
	if a.Model == "" {
		return a, fmt.Errorf("VECTRA_ASSISTANT_MODEL fehlt")
	}
	if v := os.Getenv("VECTRA_ASSISTANT_EXTERNAL"); v != "" {
		a.External = v == "true"
	}
	if v := os.Getenv("VECTRA_ASSISTANT_FALLBACKS"); v != "" {
		a.Fallbacks = a.Provider == "anthropic" && v == "true"
	}
	for _, x := range []struct {
		k   string
		dst *int
	}{{"VECTRA_ASSISTANT_DAILY_LIMIT", &a.DailyLimit}, {"VECTRA_ASSISTANT_RETENTION_DAYS", &a.RetentionDays}} {
		if v := os.Getenv(x.k); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return a, fmt.Errorf("%s: ungültig", x.k)
			}
			*x.dst = n
		}
	}
	if v := os.Getenv("VECTRA_ASSISTANT_TIMEOUT_SECONDS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return a, fmt.Errorf("VECTRA_ASSISTANT_TIMEOUT_SECONDS: ungültig")
		}
		a.Timeout = time.Duration(n) * time.Second
	}
	return a, nil
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
