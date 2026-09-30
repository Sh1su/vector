// Package db stellt den Verbindungspool, Migrationen und Transaktionen bereit (ADR-004, ADR-005).
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	"github.com/sh1su/vector/backend/migrations"
)

func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	if cfg.MaxConns > 10 {
		cfg.MaxConns = 10
	}
	return pgxpool.NewWithConfig(ctx, cfg)
}

// Migrate führt ausstehende Migrationen unter einem Advisory-Lock aus.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	sqldb := stdlib.OpenDBFromPool(pool)
	defer sqldb.Close()
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return err
	}
	p, err := goose.NewProvider(goose.DialectPostgres, sqldb, migrations.FS, goose.WithSessionLocker(locker))
	if err != nil {
		return err
	}
	if _, err := p.Up(ctx); err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	return nil
}

// InTx führt fn in genau einer Transaktion aus (Unit of Work, ADR-004).
func InTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	return pgx.BeginFunc(ctx, pool, fn)
}
