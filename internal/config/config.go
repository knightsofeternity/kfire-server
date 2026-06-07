// Package config loads server configuration from environment variables.
package config

import (
	"fmt"
	"os"
)

// Config holds all runtime configuration. Every value comes from the
// environment so the same binary works locally and in Docker.
type Config struct {
	// ListenAddr is the host:port the HTTP server binds to.
	ListenAddr string
	// DatabaseURL is the PostgreSQL connection string.
	DatabaseURL string
	// RedisURL is the Redis connection string (presence + pub/sub).
	RedisURL string
	// JWTSecret signs the 15-minute access tokens.
	JWTSecret string
	// MasterKey is the 32-byte (base64) AES-256-GCM key used to encrypt
	// OAuth tokens at rest. Generate with: openssl rand -base64 32
	MasterKey string
	// PublicURL is the externally visible base URL (used in OAuth callbacks).
	PublicURL string
	// OrgName names the instance's organization, created on first boot.
	OrgName string
	// OpenRegistration allows anyone to register. When false, registration
	// is closed once the first (admin) account exists — invites are TODO.
	OpenRegistration bool
}

// Load reads configuration from the environment. Required variables that are
// missing produce an error so misconfiguration fails fast at startup.
func Load() (*Config, error) {
	cfg := &Config{
		ListenAddr:       getEnv("KFIRE_LISTEN_ADDR", ":8080"),
		DatabaseURL:      os.Getenv("KFIRE_DATABASE_URL"),
		RedisURL:         getEnv("KFIRE_REDIS_URL", "redis://localhost:6379/0"),
		JWTSecret:        os.Getenv("KFIRE_JWT_SECRET"),
		MasterKey:        os.Getenv("KFIRE_MASTER_KEY"),
		PublicURL:        getEnv("KFIRE_PUBLIC_URL", "http://localhost:8080"),
		OrgName:          getEnv("KFIRE_ORG_NAME", "My Organization"),
		OpenRegistration: getEnv("KFIRE_OPEN_REGISTRATION", "true") == "true",
	}

	for name, val := range map[string]string{
		"KFIRE_DATABASE_URL": cfg.DatabaseURL,
		"KFIRE_JWT_SECRET":   cfg.JWTSecret,
		"KFIRE_MASTER_KEY":   cfg.MasterKey,
	} {
		if val == "" {
			return nil, fmt.Errorf("missing required environment variable %s", name)
		}
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
