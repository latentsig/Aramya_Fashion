package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	Address         string
	DatabaseURL     string
	AllowedOrigins  []string
	DatabaseTimeout time.Duration
	QueryTimeout    time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Address:         strings.TrimSpace(os.Getenv("API_ADDR")),
		DatabaseURL:     strings.TrimSpace(os.Getenv("DATABASE_URL")),
		AllowedOrigins:  splitList(os.Getenv("CORS_ALLOWED_ORIGINS")),
		DatabaseTimeout: 5 * time.Second,
		QueryTimeout:    3 * time.Second,
	}
	if cfg.Address == "" {
		cfg.Address = ":8080"
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if value := strings.TrimSpace(os.Getenv("DATABASE_CONNECT_TIMEOUT")); value != "" {
		duration, err := time.ParseDuration(value)
		if err != nil || duration <= 0 {
			return Config{}, fmt.Errorf("DATABASE_CONNECT_TIMEOUT must be a positive duration")
		}
		cfg.DatabaseTimeout = duration
	}
	if value := strings.TrimSpace(os.Getenv("DATABASE_QUERY_TIMEOUT")); value != "" {
		duration, err := time.ParseDuration(value)
		if err != nil || duration <= 0 {
			return Config{}, fmt.Errorf("DATABASE_QUERY_TIMEOUT must be a positive duration")
		}
		cfg.QueryTimeout = duration
	}
	return cfg, nil
}

func splitList(value string) []string {
	var values []string
	for _, item := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}
