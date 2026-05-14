package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port             string
	DatabaseURL      string
	JWTSecret        string
	AccessTokenTTL   time.Duration
	AllowedOrigins   string
	BCryptCost       int
	Environment      string
}

func Load() (Config, error) {
	cfg := Config{
		Port:           getEnv("PORT", "8080"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		AllowedOrigins: getEnv("ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:8080"),
		Environment:    getEnv("APP_ENV", "development"),
		BCryptCost:     getEnvInt("BCRYPT_COST", 12),
	}

	accessTokenTTLMinutes := getEnvInt("ACCESS_TOKEN_TTL_MINUTES", 60)
	cfg.AccessTokenTTL = time.Duration(accessTokenTTLMinutes) * time.Minute

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" || len(cfg.JWTSecret) < 32 {
		return Config{}, fmt.Errorf("JWT_SECRET is required and must be at least 32 characters")
	}
	if cfg.BCryptCost < 10 || cfg.BCryptCost > 15 {
		return Config{}, fmt.Errorf("BCRYPT_COST must be between 10 and 15")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
