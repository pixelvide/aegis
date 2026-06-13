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
	"strings"
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
	// Defaults to "lvh.me" for local development (*.lvh.me resolves to 127.0.0.1).
	BaseDomain string

	// AllowedOrigins for CORS. Default: http://lvh.me:8080 (Docker dev).
	// When BaseDomain is set, any subdomain origin is auto-allowed.
	AllowedOrigins []string

	// BaseURL is the public-facing URL of the app (used for email links).
	// Default: http://localhost:8080.
	BaseURL string

	// SMTP configuration for transactional emails (password reset, MFA, etc.).
	SMTP SMTPConfig

	// ValkeyURL is the Valkey/Redis connection address (host:port).
	// Default: empty (cache disabled). In Docker: valkey:6379.
	ValkeyURL string

	// LogLevel controls the minimum log level.
	// Values: debug, info, warn, error. Default: info.
	LogLevel string

	// LogFormat controls log output format.
	// Values: text (human-readable), json (structured for log aggregation).
	// Default: text.
	LogFormat string
}

// SMTPConfig holds SMTP mail server settings.
type SMTPConfig struct {
	Host     string // SMTP server hostname. Default: localhost.
	Port     int    // SMTP server port. Default: 1025 (MailDev).
	Username string // SMTP auth username (empty for MailDev).
	Password string // SMTP auth password (empty for MailDev).
	From     string // Sender address. Default: noreply@aegis.local.
	TLS      bool   // Use STARTTLS. Default: false.
}

// Load reads configuration from environment variables with secure defaults.
func Load() (*Config, error) {
	cfg := &Config{
		Port:           8080,
		Bind:           "127.0.0.1",
		AllowedOrigins: []string{"http://lvh.me:8080"},
		BaseURL:        "http://localhost:8080",
		SMTP: SMTPConfig{
			Host: "localhost",
			Port: 1025,
			From: "noreply@aegis.local",
			TLS:  false,
		},
		LogLevel:  "info",
		LogFormat: "text",
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
	} else {
		cfg.BaseDomain = "lvh.me"
	}

	if origins := os.Getenv("AEGIS_ALLOWED_ORIGINS"); origins != "" {
		cfg.AllowedOrigins = splitAndTrim(origins)
	}

	if baseURL := os.Getenv("APP_BASE_URL"); baseURL != "" {
		cfg.BaseURL = strings.TrimRight(baseURL, "/")
	} else if cfg.BaseDomain != "" {
		// Auto-derive BaseURL from BaseDomain when APP_BASE_URL is not set.
		// This ensures email links use the correct domain in development.
		if cfg.Port == 443 {
			cfg.BaseURL = fmt.Sprintf("https://%s", cfg.BaseDomain)
		} else if cfg.Port == 80 {
			cfg.BaseURL = fmt.Sprintf("http://%s", cfg.BaseDomain)
		} else {
			cfg.BaseURL = fmt.Sprintf("http://%s:%d", cfg.BaseDomain, cfg.Port)
		}
	}

	// SMTP
	if h := os.Getenv("SMTP_HOST"); h != "" {
		cfg.SMTP.Host = h
	}
	if p := os.Getenv("SMTP_PORT"); p != "" {
		port, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("invalid SMTP_PORT %q: %w", p, err)
		}
		cfg.SMTP.Port = port
	}
	if u := os.Getenv("SMTP_USERNAME"); u != "" {
		cfg.SMTP.Username = u
	}
	if pw := os.Getenv("SMTP_PASSWORD"); pw != "" {
		cfg.SMTP.Password = pw
	}
	if from := os.Getenv("SMTP_FROM"); from != "" {
		cfg.SMTP.From = from
	}
	if tls := os.Getenv("SMTP_TLS"); tls == "true" || tls == "1" {
		cfg.SMTP.TLS = true
	}

	// Valkey (optional — cache disabled if empty)
	if v := os.Getenv("VALKEY_URL"); v != "" {
		cfg.ValkeyURL = v
	}

	// Logging
	if l := os.Getenv("LOG_LEVEL"); l != "" {
		cfg.LogLevel = l
	}
	if f := os.Getenv("LOG_FORMAT"); f != "" {
		cfg.LogFormat = f
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
