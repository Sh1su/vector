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
