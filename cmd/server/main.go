package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/nirob844/mini-ad-server/internal/config"
	"github.com/nirob844/mini-ad-server/internal/db"
	"github.com/nirob844/mini-ad-server/internal/server"
)

func main() {
	// 1. Structured logging setup
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("Starting Mini Ad Server...")

	// 2. Configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// 3. Database connection & migrations
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("Database connection failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	migrationPath := filepath.Join("migrations", "001_init.sql")
	if err := db.RunMigrations(context.Background(), pool, migrationPath); err != nil {
		slog.Error("Failed to execute database migrations", "error", err)
		os.Exit(1)
	}

	// 4. Initialize Router
	workDir, _ := os.Getwd()
	r := server.NewRouter(server.RouterConfig{
		Pool:   pool,
		Config: cfg,
		WebDir: filepath.Join(workDir, "web"),
	})

	// 5. Start HTTP server with graceful shutdown
	if err := server.Run(r, cfg.ServerPort); err != nil {
		slog.Error("HTTP server terminated with error", "error", err)
		os.Exit(1)
	}
}
