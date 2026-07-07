// Package db opens and configures the PostgreSQL connection pool.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"nexlog/configs"
	"nexlog/internal/logger"
)

// Open opens a PostgreSQL connection pool with production-ready settings.
func Open(cfg *configs.Config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("db: open: %w", err)
	}

	// Connection pool — tuned for 2 CPU / 2 GB RAM, 10k users
	db.SetMaxOpenConns(cfg.DBMaxOpen)
	db.SetMaxIdleConns(cfg.DBMaxIdle)
	db.SetConnMaxLifetime(cfg.DBConnLifetime)
	db.SetConnMaxIdleTime(10 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("db: ping failed: %w", err)
	}

	logger.Info("✅ PostgreSQL connected",
		"max_open", cfg.DBMaxOpen,
		"max_idle", cfg.DBMaxIdle,
		"lifetime", cfg.DBConnLifetime.String())

	return db, nil
}
