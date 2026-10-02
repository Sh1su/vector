// Command vectra startet API, Migrationen und (optional) die statische Web-App.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // IANA-Zeitzonen im Binary (Image FROM scratch, ADR-008)

	"github.com/google/uuid"

	"github.com/sh1su/vector/backend/internal/assistant"
	"github.com/sh1su/vector/backend/internal/fuel"
	"github.com/sh1su/vector/backend/internal/identity"
	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/maintenance"
	"github.com/sh1su/vector/backend/internal/odometer"
	"github.com/sh1su/vector/backend/internal/oil"
	"github.com/sh1su/vector/backend/internal/platform/config"
	"github.com/sh1su/vector/backend/internal/platform/db"
	"github.com/sh1su/vector/backend/internal/server"
	"github.com/sh1su/vector/backend/internal/vehicles"
	vstore "github.com/sh1su/vector/backend/internal/vehicles/store"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("vectra beendet", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		return err
	}

	ids := identity.NewService(pool, cfg.SetupToken, log)
	if err := ids.PrepareSetup(ctx); err != nil {
		return err
	}
	veh := vehicles.NewService(pool)
	veh.Settings = ids.Settings
	veh.HasReadings = func(ctx context.Context, q vstore.DBTX, id uuid.UUID) (bool, error) {
		return odometer.HasReadings(ctx, q, id)
	}
	odo := odometer.NewService(pool, odometer.Config{VMaxKmh: cfg.OdometerVMaxKmh})
	fu := fuel.NewService(pool, odo)
	oi := oil.NewService(pool, odo)
	ma := maintenance.NewService(pool, odo)
	ma.UserSettings = ids.Settings
	ma.InstallSettings = ids.Installation

	var chat assistant.Chat
	switch ac := cfg.Assistant; ac.Provider {
	case "anthropic":
		chat = assistant.NewAnthropic(ac.APIKey, ac.BaseURL, ac.Model, ac.Fallbacks, ac.Timeout)
	case "openai_compatible":
		chat = assistant.NewOpenAICompatible(ac.BaseURL, ac.APIKey, ac.Model, ac.Timeout)
	}
	as := assistant.NewService(pool, assistant.Deps{Vehicles: veh, Odometer: odo, Fuel: fu, Oil: oi, Maintenance: ma,
		Units: func(ctx context.Context, id uuid.UUID) kernel.Units {
			st, _ := ids.Settings(ctx, id)
			return kernel.UnitsFromSettings(st)
		}}, chat, assistant.Config{Provider: cfg.Assistant.Provider, Name: cfg.Assistant.Name, Model: cfg.Assistant.Model,
		External: cfg.Assistant.External, DailyLimit: cfg.Assistant.DailyLimit, RetentionDays: cfg.Assistant.RetentionDays}, log)
	as.Installation = ids.Installation
	if chat != nil {
		log.Info("assistant provider configured", "provider", cfg.Assistant.Provider, "name", cfg.Assistant.Name, "model", cfg.Assistant.Model, "external", cfg.Assistant.External)
		go func() { // Aufbewahrungsfrist der Unterhaltungen (ADR-024)
			t := time.NewTicker(time.Hour)
			defer t.Stop()
			for {
				if n, err := as.Cleanup(ctx); err == nil && n > 0 {
					log.Info("assistant conversations removed", "count", n)
				}
				select {
				case <-ctx.Done():
					return
				case <-t.C:
				}
			}
		}()
	}

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           server.Handler(server.Deps{Identity: ids, Vehicles: veh, Odometer: odo, Fuel: fu, Oil: oi, Maintenance: ma, Assistant: as, Log: log, CookieSecure: cfg.CookieSecure, OdometerVMaxKmh: cfg.OdometerVMaxKmh, WebDir: cfg.WebDir}),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	errc := make(chan error, 1)
	go func() {
		log.Info("vectra läuft", "addr", cfg.Listen)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
		close(errc)
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(shutdown)
}
