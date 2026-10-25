package configs

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port          string
	DatabaseURL   string
	JWTSecret     string
	RedisURL      string
	CorsOrigins   string
	RateLimit     int
	CacheTTL      int
	AdminName     string
	AdminEmail    string
	AdminPassword string
}

func Load() (Config, error) {
	cfg := Config{
		Port:          env("PORT", "8080"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		RedisURL:      os.Getenv("REDIS_URL"),
		CorsOrigins:   env("CORS_ORIGINS", "http://localhost:3000"),
		AdminName:     os.Getenv("ADMIN_NAME"),
		AdminEmail:    os.Getenv("ADMIN_EMAIL"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
	}
	var err error
	cfg.RateLimit, err = strconv.Atoi(env("RATE_LIMIT", "60"))
	if err != nil || cfg.RateLimit < 1 {
		return Config{}, fmt.Errorf("RATE_LIMIT must be a positive integer")
	}
	cfg.CacheTTL, err = strconv.Atoi(env("CACHE_TTL_SECONDS", "30"))
	if err != nil || cfg.CacheTTL < 1 {
		return Config{}, fmt.Errorf("CACHE_TTL_SECONDS must be a positive integer")
	}
	if cfg.DatabaseURL == "" || cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("DATABASE_URL and JWT_SECRET are required")
	}
	if len(cfg.JWTSecret) < 32 {
		return Config{}, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	adminCount := 0
	for _, value := range []string{cfg.AdminName, cfg.AdminEmail, cfg.AdminPassword} {
		if value != "" {
			adminCount++
		}
	}
	if adminCount != 0 && (adminCount != 3 || len(cfg.AdminPassword) < 12 || len(cfg.AdminPassword) > 72) {
		return Config{}, fmt.Errorf("ADMIN_NAME, ADMIN_EMAIL, and ADMIN_PASSWORD must all be set; ADMIN_PASSWORD must have at least 12 characters")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
