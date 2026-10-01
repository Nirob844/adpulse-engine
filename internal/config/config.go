package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds the application configuration parameters.
type Config struct {
	ServerPort         string
	DatabaseURL        string
	RedisURL           string
	FrequencyCapMax    int
	BatchSize          int
	BatchFlushInterval time.Duration
	LogLevel           string
}

// Load loads configuration from environment variables, falling back to .env if present.
func Load() (*Config, error) {
	loadDotEnv(".env")

	port := getEnv("PORT", "")
	if port == "" {
		port = getEnv("SERVER_PORT", "8000")
	}

	dbURL := getEnv("DATABASE_URL", "postgres://adserver:adserver_secret@localhost:5434/adserver_db?sslmode=disable")
	redisURL := getEnv("REDIS_URL", "redis://localhost:6381/0")

	freqCap := 100
	if val := getEnv("AD_FREQ_CAP_MAX", ""); val != "" {
		if v, err := strconv.Atoi(val); err == nil {
			freqCap = v
		}
	} else if val := getEnv("FREQUENCY_CAP_MAX", ""); val != "" {
		if v, err := strconv.Atoi(val); err == nil {
			freqCap = v
		}
	}
	batchSize := getEnvAsInt("BATCH_SIZE", 100)
	batchFlushMs := getEnvAsInt("BATCH_FLUSH_INTERVAL_MS", 100)
	logLevel := getEnv("LOG_LEVEL", "info")

	cfg := &Config{
		ServerPort:         port,
		DatabaseURL:        dbURL,
		RedisURL:           redisURL,
		FrequencyCapMax:    freqCap,
		BatchSize:          batchSize,
		BatchFlushInterval: time.Duration(batchFlushMs) * time.Millisecond,
		LogLevel:           logLevel,
	}

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	valStr := getEnv(key, "")
	if val, err := strconv.Atoi(valStr); err == nil {
		return val
	}
	return defaultVal
}

// loadDotEnv reads key=value pairs from a file if it exists, without overriding existing env vars.
func loadDotEnv(filepath string) {
	file, err := os.Open(filepath)
	if err != nil {
		return // file doesn't exist, ignore
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			// Strip matching quotes if present
			if (strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) ||
				(strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) {
				val = val[1 : len(val)-1]
			}
			if _, exists := os.LookupEnv(key); !exists {
				_ = os.Setenv(key, val)
			}
		}
	}
}
