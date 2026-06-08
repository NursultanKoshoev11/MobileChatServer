package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port                 string
	DatabaseURL          string
	JWTSecret            string
	AccessTokenTTL       time.Duration
	AllowedOrigins       string
	BCryptCost           int
	Environment          string
	SMSProvider          string
	SMSFrom              string
	FCMProjectID         string
	FCMClientEmail       string
	FCMPrivateKey        string
	RunMigrationsOnStart bool
	SuperAdminPhones     []string
	PlatformAdminPhones  []string
	TestAuthEnabled     bool
	TestAuthPhone       string
	TestAuthCode        string
	TestAuthDisplayName string
}

func Load() (Config, error) {
	cfg := Config{
		Port:                getEnv("PORT", "8080"),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		JWTSecret:           os.Getenv("JWT_SECRET"),
		AllowedOrigins:      getEnv("ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:8080"),
		Environment:         getEnv("APP_ENV", "development"),
		BCryptCost:          getEnvInt("BCRYPT_COST", 12),
		SMSProvider:         getEnv("SMS_PROVIDER", "dev"),
		SMSFrom:             getEnv("SMS_FROM", "MobileChat"),
		FCMProjectID:        os.Getenv("FCM_PROJECT_ID"),
		FCMClientEmail:      os.Getenv("FCM_CLIENT_EMAIL"),
		FCMPrivateKey:       os.Getenv("FCM_PRIVATE_KEY"),
		SuperAdminPhones:    getEnvList("SUPER_ADMIN_PHONES"),
		PlatformAdminPhones:  getEnvList("PLATFORM_ADMIN_PHONES"),
		TestAuthEnabled:     getEnvBool("TEST_AUTH_ENABLED", false),
		TestAuthPhone:       getEnv("TEST_AUTH_PHONE", "+996700000001"),
		TestAuthCode:        getEnv("TEST_AUTH_CODE", "111111"),
		TestAuthDisplayName: getEnv("TEST_AUTH_DISPLAY_NAME", "Firebase Test User"),
	}

	cfg.RunMigrationsOnStart = getEnvBool("RUN_MIGRATIONS_ON_START", cfg.Environment != "production")

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
	if cfg.Environment == "production" && cfg.SMSProvider == "dev" {
		return Config{}, fmt.Errorf("SMS_PROVIDER=dev is not allowed in production")
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

func getEnvBool(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}

func getEnvList(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		values = append(values, value)
	}
	return values
}
