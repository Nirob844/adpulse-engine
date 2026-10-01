package server

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nirob844/mini-ad-server/internal/cache"
	"github.com/nirob844/mini-ad-server/internal/config"
	"github.com/nirob844/mini-ad-server/internal/handler"
	"github.com/nirob844/mini-ad-server/internal/repo"
	"github.com/nirob844/mini-ad-server/internal/service"
)

// RouterConfig defines dependencies required to build the Chi router.
type RouterConfig struct {
	Pool     *pgxpool.Pool
	Redis    *cache.Client
	Config   *config.Config
	WebDir   string
	Ingester *service.EventIngester
}

// NewRouter constructs and configures the complete application Chi router.
func NewRouter(cfg RouterConfig) chi.Router {
	r := chi.NewRouter()

	// 1. Core Middlewares
	r.Use(CORSMiddleware)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	// 2. Health check endpoint
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		dbStatus := "connected"
		pingCtx, pingCancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer pingCancel()

		statusCode := http.StatusOK
		if cfg.Pool != nil {
			if err := cfg.Pool.Ping(pingCtx); err != nil {
				dbStatus = "disconnected: " + err.Error()
				statusCode = http.StatusServiceUnavailable
			}
		}

		redisStatus := "disabled"
		if cfg.Redis != nil {
			if err := cfg.Redis.Ping(pingCtx); err != nil {
				redisStatus = "disconnected: " + err.Error()
			} else {
				redisStatus = "connected"
			}
		}

		handler.WriteJSON(w, statusCode, map[string]any{
			"status":   "ok",
			"database": dbStatus,
			"redis":    redisStatus,
			"time":     time.Now().UTC().Format(time.RFC3339),
		})
	})

	// 3. Initialize Repositories and Services
	var campaignRepo *repo.CampaignRepo
	var creativeRepo *repo.CreativeRepo
	var eventRepo *repo.AdEventRepo
	if cfg.Pool != nil {
		campaignRepo = repo.NewCampaignRepo(cfg.Pool)
		creativeRepo = repo.NewCreativeRepo(cfg.Pool)
		eventRepo = repo.NewAdEventRepo(cfg.Pool)
	}

	campaignService := service.NewCampaignService(campaignRepo)
	campaignHandler := handler.NewCampaignHandler(campaignService)

	creativeService := service.NewCreativeService(creativeRepo, campaignRepo)
	creativeHandler := handler.NewCreativeHandler(creativeService)

	freqCapMax := 100
	if cfg.Config != nil {
		freqCapMax = cfg.Config.FrequencyCapMax
	}
	adService := service.NewAdService(creativeRepo, eventRepo, cfg.Redis, freqCapMax)
	adHandler := handler.NewAdHandler(adService)

	var ingester *service.EventIngester
	if cfg.Ingester != nil {
		ingester = cfg.Ingester
	} else if eventRepo != nil {
		batchSize := 100
		flushInterval := 100 * time.Millisecond
		if cfg.Config != nil {
			if cfg.Config.BatchSize > 0 {
				batchSize = cfg.Config.BatchSize
			}
			if cfg.Config.BatchFlushInterval > 0 {
				flushInterval = cfg.Config.BatchFlushInterval
			}
		}
		ingester = service.NewEventIngester(eventRepo, service.IngesterConfig{
			BatchSize:     batchSize,
			FlushInterval: flushInterval,
			QueueCapacity: 10000,
		})
		ingester.Start()
	}

	trackingService := service.NewTrackingService(creativeRepo, eventRepo, cfg.Redis, ingester)
	trackingHandler := handler.NewTrackingHandler(trackingService)

	reportService := service.NewReportService(eventRepo)
	reportHandler := handler.NewReportHandler(reportService)

	// 4. API Endpoints
	r.Get("/config", func(w http.ResponseWriter, r *http.Request) {
		appURL := "http://localhost:8000"
		if cfg.Config != nil && cfg.Config.AppURL != "" {
			appURL = cfg.Config.AppURL
		}
		handler.WriteJSON(w, http.StatusOK, map[string]string{
			"app_url": appURL,
			"version": "1.0.0",
		})
	})
	r.Get("/ad", adHandler.ServeAd)
	r.Get("/track/impression", trackingHandler.TrackImpression)
	r.Get("/track/click", trackingHandler.TrackClick)
	r.Get("/report", reportHandler.GetReport)

	r.Route("/campaigns", func(r chi.Router) {
		r.Post("/", campaignHandler.Create)
		r.Get("/", campaignHandler.List)
		r.Get("/{id}", campaignHandler.GetByID)
		r.Patch("/{id}", campaignHandler.UpdateStatus)
		r.Put("/{id}", campaignHandler.Update)
		r.Delete("/{id}", campaignHandler.Delete)
	})

	r.Route("/creatives", func(r chi.Router) {
		r.Post("/", creativeHandler.Create)
		r.Get("/", creativeHandler.List)
		r.Get("/{id}", creativeHandler.GetByID)
		r.Put("/{id}", creativeHandler.Update)
		r.Delete("/{id}", creativeHandler.Delete)
	})

	// 5. Documentation endpoint
	webDir := cfg.WebDir
	if webDir == "" {
		workDir, _ := os.Getwd()
		webDir = filepath.Join(workDir, "web")
	}

	r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "docs.html"))
	})

	r.Get("/admin", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "admin.html"))
	})

	// 6. Static files (index.html, publisher.html, openapi.yaml, etc.)
	fileServer(r, "/", http.Dir(webDir))

	return r
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

	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.RouteContext(r.Context())
		pathPrefix := strings.TrimSuffix(rctx.RoutePattern(), "/*")
		fs := http.StripPrefix(pathPrefix, http.FileServer(root))
		fs.ServeHTTP(w, r)
	}

	r.Get(path, handlerFunc)
	r.Head(path, handlerFunc)
}
