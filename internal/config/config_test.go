package config

import (
	"os"
	"testing"
	"time"
)

func TestConfigLoad(t *testing.T) {
	// Set custom env vars to test overriding
	_ = os.Setenv("PORT", "9090")
	_ = os.Setenv("AD_FREQ_CAP_MAX", "100")
	_ = os.Setenv("BATCH_SIZE", "250")
	_ = os.Setenv("BATCH_FLUSH_INTERVAL_MS", "200")
	defer func() {
		_ = os.Unsetenv("PORT")
		_ = os.Unsetenv("AD_FREQ_CAP_MAX")
		_ = os.Unsetenv("BATCH_SIZE")
		_ = os.Unsetenv("BATCH_FLUSH_INTERVAL_MS")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error loading config, got %v", err)
	}

	if cfg.ServerPort != "9090" {
		t.Errorf("expected ServerPort 9090, got %s", cfg.ServerPort)
	}

	if cfg.FrequencyCapMax != 100 {
		t.Errorf("expected FrequencyCapMax 100, got %d", cfg.FrequencyCapMax)
	}

	if cfg.BatchSize != 250 {
		t.Errorf("expected BatchSize 250, got %d", cfg.BatchSize)
	}

	if cfg.BatchFlushInterval != 200*time.Millisecond {
		t.Errorf("expected BatchFlushInterval 200ms, got %v", cfg.BatchFlushInterval)
	}
}

func TestConfigLoad_FallbackFrequencyCapMax(t *testing.T) {
	_ = os.Unsetenv("AD_FREQ_CAP_MAX")
	_ = os.Setenv("FREQUENCY_CAP_MAX", "5")
	defer os.Unsetenv("FREQUENCY_CAP_MAX")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error loading config, got %v", err)
	}

	if cfg.FrequencyCapMax != 5 {
		t.Errorf("expected FrequencyCapMax 5, got %d", cfg.FrequencyCapMax)
	}
}

func TestConfigLoad_AppURL(t *testing.T) {
	// 1. Explicit APP_URL with trailing slash
	_ = os.Setenv("APP_URL", "https://ads.myproduction.com/")
	defer os.Unsetenv("APP_URL")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error loading config, got %v", err)
	}
	if cfg.AppURL != "https://ads.myproduction.com" {
		t.Errorf("expected AppURL 'https://ads.myproduction.com', got '%s'", cfg.AppURL)
	}

	// 2. Fallback to BASE_URL
	_ = os.Unsetenv("APP_URL")
	_ = os.Setenv("BASE_URL", "https://base.example.com")
	defer os.Unsetenv("BASE_URL")

	cfg, err = Load()
	if err != nil {
		t.Fatalf("expected no error loading config, got %v", err)
	}
	if cfg.AppURL != "https://base.example.com" {
		t.Errorf("expected AppURL 'https://base.example.com', got '%s'", cfg.AppURL)
	}
}
