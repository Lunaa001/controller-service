package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port               string
	CorrelationHeader  string
	UserIDHeader       string
	UserServiceURL     string
	ExtractServiceURL  string
	SummaryServiceURL  string
	PersistenceURL     string
	RedisURL           string
	RedisPassword      string
	RequestTimeout     int // segundos
}

func Load() Config {
	return Config{
		Port:              env("PORT", "5000"),
		CorrelationHeader: env("X_CORRELATION_HEADER", "X-Correlation-ID"),
		UserIDHeader:      env("X_USER_ID_HEADER", "X-User-ID"),
		UserServiceURL:    env("USER_SERVICE_URL", "http://localhost:8001"),
		ExtractServiceURL: env("EXTRACT_SERVICE_URL", "http://localhost:8002"),
		SummaryServiceURL: env("SUMMARY_SERVICE_URL", "http://localhost:8003"),
		PersistenceURL:    env("PERSISTENCE_URL", "http://localhost:8004"),
		RedisURL:          env("REDIS_URL", "redis://localhost:6379"),
		RedisPassword:     env("REDIS_PASSWORD", ""),
		RequestTimeout:    envInt("REQUEST_TIMEOUT", 30),
	}
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

