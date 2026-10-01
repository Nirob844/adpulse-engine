package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/nirob844/mini-ad-server/internal/config"
)

func TestDBConnectAndMigrate(t *testing.T) {
	// Restore default port from .env or default for local container
	_ = os.Unsetenv("DATABASE_URL")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Skipf("skipping live database test: %v", err)
		return
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("failed to ping database: %v", err)
	}

	// Test RunMigrations with the real migrations file
	err = RunMigrations(ctx, pool, "../../migrations/001_init.sql")
	if err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}
}
