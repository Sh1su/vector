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
	"github.com/sh1su/vector/backend/internal/costs"
	"github.com/sh1su/vector/backend/internal/documents"
	"github.com/sh1su/vector/backend/internal/identity"
	"github.com/sh1su/vector/backend/internal/maintenance"
	"github.com/sh1su/vector/backend/internal/odometer"
	"github.com/sh1su/vector/backend/internal/platform/config"
	"github.com/sh1su/vector/backend/internal/platform/db"
	"github.com/sh1su/vector/backend/internal/platform/storage"
	"github.com/sh1su/vector/backend/internal/server"
	"github.com/sh1su/vector/backend/internal/servicehistory"
	"github.com/sh1su/vector/backend/internal/trips"
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
	deps := server.Deps{Identity: ids, Vehicles: veh, Odometer: odo, Costs: costs.NewService(pool, odo),
		Maintenance: maintenance.NewService(pool, odo), Service: servicehistory.NewService(pool, odo), Trips: trips.NewService(pool, odo),
		Documents: documents.NewService(pool, storage.Local{Dir: cfg.StorageDir}, int64(cfg.MaxUploadMB)<<20),
		Assistant: assistant.NewService(pool, assistant.Config{Provider: cfg.Assistant.Provider, APIKey: cfg.Assistant.APIKey, Model: cfg.Assistant.Model,
			BaseURL: cfg.Assistant.BaseURL, WebSearch: cfg.Assistant.WebSearch, DailyLimit: cfg.Assistant.DailyLimit, RetentionDays: cfg.Assistant.RetentionDays}),
		Log: log, CookieSecure: cfg.CookieSecure, WebDir: cfg.WebDir}
	if cfg.Assistant.Provider != "" {
		log.Info("Assistent aktiv", "provider", cfg.Assistant.Provider, "model", cfg.Assistant.Model, "web_search", cfg.Assistant.WebSearch)
	}

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           server.Handler(deps),
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
