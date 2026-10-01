package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/nirob844/mini-ad-server/internal/config"
	"github.com/nirob844/mini-ad-server/internal/db"
	"github.com/nirob844/mini-ad-server/internal/handler"
	"github.com/nirob844/mini-ad-server/internal/repo"
	"github.com/nirob844/mini-ad-server/internal/service"
)

func main() {
	// 1. Initialize structured logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("Starting Mini Ad Server...")

	// 2. Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// 3. Connect to PostgreSQL
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("Database connection failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// 4. Run database migrations on startup
	migrationPath := filepath.Join("migrations", "001_init.sql")
	if err := db.RunMigrations(context.Background(), pool, migrationPath); err != nil {
		slog.Error("Failed to execute database migrations", "error", err)
		os.Exit(1)
	}

	// 5. Initialize Chi Router & Middleware
	r := chi.NewRouter()
	r.Use(corsMiddleware)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	// Health check endpoint
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		dbStatus := "connected"
		pingCtx, pingCancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer pingCancel()

		w.Header().Set("Content-Type", "application/json")
		if err := pool.Ping(pingCtx); err != nil {
			dbStatus = "disconnected: " + err.Error()
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":   "ok",
			"database": dbStatus,
			"time":     time.Now().UTC().Format(time.RFC3339),
		})
	})

	// 6. Initialize Repositories, Services, and Handlers
	campaignRepo := repo.NewCampaignRepo(pool)
	campaignService := service.NewCampaignService(campaignRepo)
	campaignHandler := handler.NewCampaignHandler(campaignService)

	// Campaign endpoints
	r.Post("/campaigns", campaignHandler.Create)
	r.Patch("/campaigns/{id}", campaignHandler.UpdateStatus)
	r.Get("/campaigns/{id}", campaignHandler.GetByID)
	r.Get("/campaigns", campaignHandler.List)

	// Documentation endpoint
	workDir, _ := os.Getwd()
	filesDir := filepath.Join(workDir, "web")
	r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(filesDir, "docs.html"))
	})

	// 7. Serve static files from web/ folder (e.g. / -> web/index.html, /publisher.html)
	fileServer(r, "/", http.Dir(filesDir))

	// 7. Setup HTTP server with production timeouts
	serverAddr := ":" + cfg.ServerPort
	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 8. Start server in a background goroutine
	go func() {
		slog.Info("HTTP server listening", "addr", serverAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	// 9. Graceful shutdown on SIGINT / SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	sig := <-quit

	slog.Info("Shutting down server gracefully...", "signal", sig.String())

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}

	slog.Info("Server stopped cleanly")
}

// fileServer sets up static file serving from an http.FileSystem on a Chi router.
func fileServer(r chi.Router, path string, root http.FileSystem) {
	if strings.ContainsAny(path, "{}*") {
		panic("FileServer does not permit URL parameters.")
	}

	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", http.StatusMovedPermanently).ServeHTTP)
		path += "/"
	}
	path += "*"

	r.Get(path, func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.RouteContext(r.Context())
		pathPrefix := strings.TrimSuffix(rctx.RoutePattern(), "/*")
		fs := http.StripPrefix(pathPrefix, http.FileServer(root))
		fs.ServeHTTP(w, r)
	})
}

// corsMiddleware enables Cross-Origin Resource Sharing for all origins and handles OPTIONS preflight.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token, X-Requested-With")
		w.Header().Set("Access-Control-Expose-Headers", "Link, Content-Length")
		w.Header().Set("Access-Control-Max-Age", "300")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
