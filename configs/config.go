// Package config centralises all configuration loaded from environment variables.
// The server refuses to start if required secrets are missing.
package configs

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	// Server
	Port      string
	PublicDir string
	UploadDir string
	IsDev     bool

	// Database
	DatabaseURL    string
	DBMaxOpen      int
	DBMaxIdle      int
	DBConnLifetime time.Duration

	// Migrations
	MigrationsDir string

	// Auth
	JWTSecret string

	// Rate limiting
	RateLimitPerMin int

	// CORS
	AllowedOrigins string

	// pprof
	EnablePprof bool
}

func Load() (*Config, error) {
	var errs []string

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		errs = append(errs, "JWT_SECRET is required (generate with: openssl rand -hex 32)")
	}
	if len(jwtSecret) < 32 {
		errs = append(errs, "JWT_SECRET must be at least 32 characters")
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		errs = append(errs, "DATABASE_URL is required (e.g. postgres://user:pass@host:5432/dbname?sslmode=disable)")
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("config errors:\n  - %s", joinErrs(errs))
	}

	publicDir := getEnv("PUBLIC_DIR", "./public")

	return &Config{
		Port:           getEnv("PORT", "3000"),
		PublicDir:      publicDir,
		UploadDir:      getEnv("UPLOAD_DIR", publicDir+"/uploads"),
		IsDev:          getEnv("APP_ENV", "production") == "development",
		DatabaseURL:    dbURL,
		DBMaxOpen:      getInt("DB_MAX_OPEN", 50),
		DBMaxIdle:      getInt("DB_MAX_IDLE", 25),
		DBConnLifetime: getDuration("DB_CONN_LIFETIME", time.Hour),
		MigrationsDir:  getEnv("MIGRATIONS_DIR", "./migrations"),
		JWTSecret:      jwtSecret,
		RateLimitPerMin: getInt("RATE_LIMIT_PER_MIN", 120),
		AllowedOrigins: getEnv("ALLOWED_ORIGINS", ""),
		EnablePprof:    getEnv("ENABLE_PPROF", "false") == "true",
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func joinErrs(errs []string) string {
	result := ""
	for i, e := range errs {
		if i > 0 {
			result += "\n  - "
		}
		result += e
	}
	return result
}

// Validate is called from tests.
func (c *Config) Validate() error {
	if c.JWTSecret == "" {
		return errors.New("JWT_SECRET required")
	}
	return nil
}
