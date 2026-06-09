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
		UserServiceURL:    env("USER_SERVICE_URL", "http://users.universidad.localhost:5000"),
		ExtractServiceURL: env("EXTRACT_SERVICE_URL", "http://extractor.universidad.localhost:5000"),
		SummaryServiceURL: env("SUMMARY_SERVICE_URL", "http://ai.universidad.localhost:5000"),
		PersistenceURL:    env("PERSISTENCE_URL", "http://persistence-java.universidad.localhost:8080"),
		RedisURL:          env("REDIS_URL", "redis://redis-controller:6379"),
		RedisPassword:     env("REDIS_PASSWORD", ""),
		RequestTimeout:    envInt("REQUEST_TIMEOUT", 60),
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
