// Package config handles server configuration.
//
// Configuration is loaded from environment variables.
// Secrets are never hardcoded — the server errors out if required
// variables are missing in production.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all server configuration.
type Config struct {
	// Port is the HTTP listen port. Default: 8080.
	Port int

	// Bind is the address to bind to. Default: 127.0.0.1.
	// Set to 0.0.0.0 when running inside Docker.
	Bind string

	// DatabaseURL is the PostgreSQL connection string.
	// Format: postgres://user:pass@host:port/dbname?sslmode=disable
	DatabaseURL string

	// BaseDomain is the base domain for subdomain-based org resolution.
	// e.g., "aegis.io" means orgs are accessed via acme.aegis.io.
	// Empty = no subdomain resolution (use X-Org-Slug header instead).
	BaseDomain string

	// AllowedOrigins for CORS. Default: http://localhost:5173 (Vite dev server).
	AllowedOrigins []string
}

// Load reads configuration from environment variables with secure defaults.
func Load() (*Config, error) {
	cfg := &Config{
		Port:           8080,
		Bind:           "127.0.0.1",
		AllowedOrigins: []string{"http://localhost:5173"},
	}

	if p := os.Getenv("AEGIS_PORT"); p != "" {
		port, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("invalid AEGIS_PORT %q: %w", p, err)
		}
		if port < 1 || port > 65535 {
			return nil, fmt.Errorf("AEGIS_PORT %d out of range (1-65535)", port)
		}
		cfg.Port = port
	}

	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		cfg.DatabaseURL = dbURL
	}

	if bind := os.Getenv("AEGIS_BIND"); bind != "" {
		cfg.Bind = bind
	}

	if domain := os.Getenv("AEGIS_BASE_DOMAIN"); domain != "" {
		cfg.BaseDomain = domain
	}

	if origins := os.Getenv("AEGIS_ALLOWED_ORIGINS"); origins != "" {
		cfg.AllowedOrigins = splitAndTrim(origins)
	}

	return cfg, nil
}

// Addr returns the listen address string (e.g., "127.0.0.1:8080").
// Binds to localhost only — never 0.0.0.0 in development.
func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Bind, c.Port)
}

func splitAndTrim(s string) []string {
	var result []string
	for _, part := range splitComma(s) {
		trimmed := trimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func splitComma(s string) []string {
	var parts []string
	start := 0
	for i := range len(s) {
		if s[i] == ',' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func trimSpace(s string) string {
	i, j := 0, len(s)
	for i < j && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	for j > i && (s[j-1] == ' ' || s[j-1] == '\t') {
		j--
	}
	return s[i:j]
}
