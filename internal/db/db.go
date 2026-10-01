package db

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect initializes and returns a pgxpool.Pool configured for production use.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	// Production connection pool tuning
	config.MaxConns = 25
	config.MinConns = 5
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute
	config.HealthCheckPeriod = 1 * time.Minute

	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(connectCtx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create pgxpool: %w", err)
	}

	if err := pool.Ping(connectCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	slog.Info("Connected to PostgreSQL successfully", "max_conns", config.MaxConns, "min_conns", config.MinConns)
	return pool, nil
}

// RunMigrations reads the given SQL file and applies it to the database.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("unable to read migration file %s: %w", filePath, err)
	}

	txCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if _, err := pool.Exec(txCtx, string(content)); err != nil {
		return fmt.Errorf("failed to execute migration %s: %w", filePath, err)
	}

	slog.Info("Database migrations applied successfully", "file", filePath)
	return nil
}
