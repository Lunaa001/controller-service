package config

import (
	"log"
	"os"
	"strconv"
)

type Config struct {
	Port              string
	CorrelationHeader string
	UserIDHeader      string
	UserServiceURL    string
	ExtractServiceURL string
	SummaryServiceURL string
	PersistenceURL    string
	RedisURL          string
	RedisPassword     string
	RequestTimeout    int // segundos
}

// Load reads configuration from Consul KV first, then falls back to env vars.
func Load() Config {
	// Try to load from Consul KV
	consulCfg := LoadConsulConfig()
	kvConfig := consulCfg.FetchKVConfig("controller-service")

	// Helper: read from KV first, then env var, then default
	getVal := func(key, fallback string) string {
		if v, ok := kvConfig[key]; ok && v != "" {
			return v
		}
		return env(key, fallback)
	}

	getIntVal := func(key string, fallback int) int {
		if v, ok := kvConfig[key]; ok && v != "" {
			n, err := strconv.Atoi(v)
			if err == nil {
				return n
			}
		}
		return envInt(key, fallback)
	}

	cfg := Config{
		Port:              getVal("PORT", "5000"),
		CorrelationHeader: getVal("X_CORRELATION_HEADER", "X-Correlation-ID"),
		UserIDHeader:      getVal("X_USER_ID_HEADER", "X-User-ID"),
		UserServiceURL:    getVal("USER_SERVICE_URL", "http://users.universidad.localhost:5000"),
		ExtractServiceURL: getVal("EXTRACT_SERVICE_URL", "http://extractor.universidad.localhost:5000"),
		SummaryServiceURL: getVal("SUMMARY_SERVICE_URL", "http://ai.universidad.localhost:5000"),
		PersistenceURL:    getVal("PERSISTENCE_URL", "http://persistence-java.universidad.localhost:8080"),
		RedisURL:          getVal("REDIS_URL", "redis://redis-controller:6379"),
		RedisPassword:     getVal("REDIS_PASSWORD", ""),
		RequestTimeout:    getIntVal("REQUEST_TIMEOUT", 60),
	}

	if len(kvConfig) > 0 {
		log.Printf("✓ Controller config loaded from Consul KV (%d keys)", len(kvConfig))
	} else {
		log.Printf("⚠ Controller config loaded from environment variables (Consul KV unavailable)")
	}

	return cfg
}

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		n, err := strconv.Atoi(value)
		if err == nil {
			return n
		}
	}
	return fallback
}
