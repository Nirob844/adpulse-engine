package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/nirob844/adpulse-engine/internal/cache"
	"github.com/nirob844/adpulse-engine/internal/config"
	"github.com/nirob844/adpulse-engine/internal/db"
	"github.com/nirob844/adpulse-engine/internal/repo"
	"github.com/nirob844/adpulse-engine/internal/server"
	"github.com/nirob844/adpulse-engine/internal/service"
)

func main() {
	// 1. Structured logging setup
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("Starting AdPulse Engine...")

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

	// 4. Redis connection (graceful fallback if unavailable)
	var redisClient *cache.Client
	if cfg.RedisURL != "" {
		rc, err := cache.Connect(cfg.RedisURL)
		if err != nil {
			slog.Warn("Redis connection failed, continuing without cache", "error", err)
		} else {
			slog.Info("Connected to Redis successfully", "url", cfg.RedisURL)
			redisClient = rc
			defer redisClient.Close()
		}
	}

	// 5. Initialize Asynchronous Event Ingester & Router
	eventRepo := repo.NewAdEventRepo(pool)
	ingester := service.NewEventIngester(eventRepo, service.IngesterConfig{
		BatchSize:     cfg.BatchSize,
		FlushInterval: cfg.BatchFlushInterval,
		QueueCapacity: 10000,
	})
	ingester.Start()

	workDir, _ := os.Getwd()
	r := server.NewRouter(server.RouterConfig{
		Pool:     pool,
		Redis:    redisClient,
		Config:   cfg,
		WebDir:   filepath.Join(workDir, "web"),
		Ingester: ingester,
	})

	// 6. Start HTTP server with graceful shutdown and ingester flush
	if err := server.Run(r, cfg.ServerPort, func() {
		slog.Info("Flushing remaining in-flight events before shutdown...")
		ingester.Stop()
	}); err != nil {
		slog.Error("HTTP server terminated with error", "error", err)
		os.Exit(1)
	}
}
