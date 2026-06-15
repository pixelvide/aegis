package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pixelvide/localharness/adk"
	"gopkg.in/yaml.v3"
)

// globalConfigRelPath is the relative path under $HOME for global Aegis config.
const globalConfigRelPath = ".pixelvide/agents/aegis/config.yml"

// validProviders lists all supported LLM providers.
var validProviders = map[string]bool{
	"gemini":     true,
	"openai":     true,
	"ollama":     true,
	"cloudflare": true,
}

// ── Types ────────────────────────────────────────────────────────────────────

// AegisConfig is the fully resolved configuration for an Aegis run.
// All fields are optional — empty means "use default".
type AegisConfig struct {
	Provider      string `yaml:"provider"`       // gemini, openai, ollama, cloudflare
	Model         string `yaml:"model"`          // Model name (e.g., gemini-3.5-flash, gpt-4o)
	BaseURL       string `yaml:"base_url"`       // Base URL for OpenAI-compatible providers
	ThinkingLevel string `yaml:"thinking_level"` // off, low, medium, high (default: medium)
	Workspace     string `yaml:"workspace"`      // Workspace directory

	// Profile selects a preset configuration for the scan pipeline.
	// "fast" uses the cheap model for all roles with best_of_k=1.
	// "deep" uses the cheap model for scanning and the smart model for
	// reviewing/exploit writing with best_of_k=3.
	// Empty string means no profile (use explicit roles or global model).
	Profile string `yaml:"profile"`

	// Personas holds per-persona overrides. Each persona inherits the global
	// settings above and can override provider, model, base_url, and thinking_level.
	Personas map[string]PersonaConfig `yaml:"personas"`

	// Roles holds per-role model overrides for the scan pipeline.
	// Each role (scanner, reviewer, exploit_writer) can use a different
	// provider and model. If not set, falls back to the global provider/model.
	Roles RolesConfig `yaml:"roles"`

	// Scan holds tuning parameters for the scan pipeline.
	Scan ScanConfig `yaml:"scan"`

	// Reporting holds Aegis server connection settings.
	// Nested under "reporting:" in config.yml to avoid collision with
	// the top-level base_url (which is for LLM providers).
	Reporting ReportingConfig `yaml:"reporting"`
}

// ReportingConfig holds Aegis server connection settings.
// The API key is NOT stored here — it comes from AEGIS_API_KEY env var only.
type ReportingConfig struct {
	BaseURL   string `yaml:"base_url"`   // Aegis server URL (e.g., https://acme.aegis.io)
	ProjectID string `yaml:"project_id"` // Default project UUID
}

// PersonaConfig holds per-persona provider/model overrides.
type PersonaConfig struct {
	Provider      string `yaml:"provider"`
	Model         string `yaml:"model"`
	BaseURL       string `yaml:"base_url"`
	ThinkingLevel string `yaml:"thinking_level"`
}

// RoleConfig holds provider/model settings for a specific pipeline role.
// If empty, the role inherits the global provider/model from AegisConfig.
type RoleConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
	BaseURL  string `yaml:"base_url"`
}

// RolesConfig holds per-role model overrides for the scan pipeline.
// The tiered model strategy uses cheap models for high-volume scanning
// and smart models for reasoning-heavy review and exploit writing.
type RolesConfig struct {
	Scanner      RoleConfig `yaml:"scanner"`
	Reviewer     RoleConfig `yaml:"reviewer"`
	ExploitWriter RoleConfig `yaml:"exploit_writer"`
}

// ScanConfig holds tuning parameters for the scan pipeline.
type ScanConfig struct {
	MaxConcurrent int `yaml:"max_concurrent"` // Max parallel subagents (default: 5)
	BestOfK       int `yaml:"best_of_k"`      // Retry count per chunk (default: 3)
	ChunkMaxLines int `yaml:"chunk_max_lines"` // Max lines per chunk (default: 150)
}

// ScanDefaults returns a ScanConfig with default values applied.
func (s ScanConfig) WithDefaults() ScanConfig {
	if s.MaxConcurrent <= 0 {
		s.MaxConcurrent = 5
	}
	if s.BestOfK <= 0 {
		s.BestOfK = 3
	}
	if s.ChunkMaxLines <= 0 {
		s.ChunkMaxLines = 150
	}
	return s
}

// profileDefaults maps profile name + provider to role-specific model defaults.
// The "fast" profile uses the cheap model everywhere; "deep" uses smart models
// for reviewer and exploit_writer roles.
var profileDefaults = map[string]map[string]RolesConfig{
	"fast": {
		"cloudflare": {
			Scanner:      RoleConfig{Model: "@cf/google/gemma-4-26b-a4b-it"},
			Reviewer:     RoleConfig{Model: "@cf/google/gemma-4-26b-a4b-it"},
			ExploitWriter: RoleConfig{Model: "@cf/google/gemma-4-26b-a4b-it"},
		},
		"gemini": {
			Scanner:      RoleConfig{Model: "gemini-2.5-flash"},
			Reviewer:     RoleConfig{Model: "gemini-2.5-flash"},
			ExploitWriter: RoleConfig{Model: "gemini-2.5-flash"},
		},
		"openai": {
			Scanner:      RoleConfig{Model: "gpt-4.1-mini"},
			Reviewer:     RoleConfig{Model: "gpt-4.1-mini"},
			ExploitWriter: RoleConfig{Model: "gpt-4.1-mini"},
		},
	},
	"deep": {
		"cloudflare": {
			Scanner:      RoleConfig{Model: "@cf/google/gemma-4-26b-a4b-it"},
			Reviewer:     RoleConfig{Model: "@cf/meta/llama-4-scout-17b-16e-instruct"},
			ExploitWriter: RoleConfig{Model: "@cf/meta/llama-4-scout-17b-16e-instruct"},
		},
		"gemini": {
			Scanner:      RoleConfig{Model: "gemini-2.5-flash"},
			Reviewer:     RoleConfig{Model: "gemini-2.5-pro"},
			ExploitWriter: RoleConfig{Model: "gemini-2.5-pro"},
		},
		"openai": {
			Scanner:      RoleConfig{Model: "gpt-4.1-mini"},
			Reviewer:     RoleConfig{Model: "gpt-5.4"},
			ExploitWriter: RoleConfig{Model: "gpt-5.4"},
		},
	},
}

// profileScanDefaults maps profile name to scan tuning overrides.
var profileScanDefaults = map[string]ScanConfig{
	"fast": {BestOfK: 1},
	"deep": {BestOfK: 3},
}

// ResolveRoles returns the effective RolesConfig by applying profile defaults
// and then explicit role overrides. The provider is needed to select the
// correct model defaults from the profile.
func (a *AegisConfig) ResolveRoles() RolesConfig {
	var roles RolesConfig

	// 1. Apply profile defaults (if a profile is set)
	if a.Profile != "" {
		provider := a.Provider
		if provider == "" {
			provider = "gemini" // default provider
		}
		if byProvider, ok := profileDefaults[a.Profile]; ok {
			if defaults, ok := byProvider[provider]; ok {
				roles = defaults
			}
		}
	}

	// 2. Apply explicit role overrides (highest priority)
	mergeRoleConfig(&roles.Scanner, a.Roles.Scanner)
	mergeRoleConfig(&roles.Reviewer, a.Roles.Reviewer)
	mergeRoleConfig(&roles.ExploitWriter, a.Roles.ExploitWriter)

	return roles
}

// ResolveScan returns the effective ScanConfig by applying profile defaults
// and then explicit scan overrides.
func (a *AegisConfig) ResolveScan() ScanConfig {
	scan := a.Scan

	// Apply profile defaults for any zero-valued fields
	if a.Profile != "" {
		if defaults, ok := profileScanDefaults[a.Profile]; ok {
			if scan.BestOfK <= 0 {
				scan.BestOfK = defaults.BestOfK
			}
		}
	}

	return scan.WithDefaults()
}

// mergeRoleConfig copies non-empty fields from src into dst.
func mergeRoleConfig(dst *RoleConfig, src RoleConfig) {
	if src.Provider != "" {
		dst.Provider = src.Provider
	}
	if src.Model != "" {
		dst.Model = src.Model
	}
	if src.BaseURL != "" {
		dst.BaseURL = src.BaseURL
	}
}

// CLIOverrides holds values explicitly set via CLI flags.
// Empty string means "not set" (don't override).
type CLIOverrides struct {
	Provider   string
	Model      string
	BaseURL    string
	Workspace  string
	ConfigFile string

	// Scan pipeline overrides
	Profile       string // --profile (fast or deep)
	MaxConcurrent int    // --max-concurrent
	BestOfK       int    // --best-of-k

	// Reporting overrides (AEGIS_API_KEY is env-var-only, not here)
	ReportBaseURL string // --report-base-url
	ProjectID     string // --project
}

// ── Core Methods ─────────────────────────────────────────────────────────────

// ForPersona returns a copy with persona-specific overrides applied.
// If the persona has no overrides, the global config is returned unchanged.
func (a AegisConfig) ForPersona(name string) AegisConfig {
	pc, ok := a.Personas[name]
	if !ok {
		return a
	}
	mergeProviderFields(&a.Provider, &a.Model, &a.BaseURL, &a.ThinkingLevel, pc.Provider, pc.Model, pc.BaseURL, pc.ThinkingLevel)
	return a
}

// Validate checks the config for invalid values and returns all errors found.
func (a *AegisConfig) Validate() error {
	var errs []string

	if a.Provider != "" && !validProviders[strings.ToLower(a.Provider)] {
		errs = append(errs, fmt.Sprintf("unknown provider %q (supported: gemini, openai, ollama, cloudflare)", a.Provider))
	}

	if a.Profile != "" && a.Profile != "fast" && a.Profile != "deep" {
		errs = append(errs, fmt.Sprintf("unknown profile %q (supported: fast, deep)", a.Profile))
	}

	for name, pc := range a.Personas {
		if pc.Provider != "" && !validProviders[strings.ToLower(pc.Provider)] {
			errs = append(errs, fmt.Sprintf("persona %q: unknown provider %q", name, pc.Provider))
		}
	}

	// Validate reporting config: if base_url is set, AEGIS_API_KEY must be present.
	if a.Reporting.BaseURL != "" {
		if os.Getenv("AEGIS_API_KEY") == "" {
			errs = append(errs, "reporting.base_url is set but AEGIS_API_KEY env var is missing")
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed:\n  • %s", strings.Join(errs, "\n  • "))
	}
	return nil
}

// ApplyTo configures the SDK agent config from the resolved Aegis config.
// Handles provider auto-detection from env vars when not explicitly set.
func (a *AegisConfig) ApplyTo(cfg *adk.LocalAgentConfig) error {
	geminiKey := os.Getenv("GEMINI_API_KEY")
	openaiKey := os.Getenv("OPENAI_API_KEY")
	cfAccountID := os.Getenv("CF_ACCOUNT_ID")
	cfGatewayID := os.Getenv("CF_GATEWAY_ID")
	cfAPIToken := os.Getenv("CF_API_TOKEN")
	cfProvider := os.Getenv("CF_PROVIDER")

	provider := a.Provider

	// Auto-detect provider from env vars if not set
	if provider == "" {
		switch {
		case cfAccountID != "" && cfGatewayID != "":
			provider = "cloudflare"
		case geminiKey != "":
			provider = "gemini"
		case openaiKey != "":
			provider = "openai"
		default:
			return fmt.Errorf("no LLM provider configured.\n\nSet one of:\n  GEMINI_API_KEY=...   (for Gemini)\n  OPENAI_API_KEY=...   (for OpenAI/Ollama)\n  CF_ACCOUNT_ID + CF_GATEWAY_ID + CF_API_TOKEN  (for Cloudflare)\n\nOr use --provider=gemini|openai|cloudflare with the appropriate env vars.")
		}
		a.Provider = provider
	}

	switch strings.ToLower(provider) {
	case "gemini":
		if geminiKey == "" {
			return fmt.Errorf("--provider=gemini requires GEMINI_API_KEY env var")
		}
		cfg.GeminiAPIKey = geminiKey
		if a.Model != "" {
			cfg.GeminiModel = a.Model
		}

	case "openai", "ollama":
		if openaiKey == "" && provider != "ollama" {
			return fmt.Errorf("--provider=openai requires OPENAI_API_KEY env var")
		}
		cfg.OpenAIAPIKey = openaiKey
		if cfg.OpenAIAPIKey == "" {
			cfg.OpenAIAPIKey = "ollama" // Ollama doesn't need a real key
		}
		if a.Model != "" {
			cfg.OpenAIModel = a.Model
		}
		if a.BaseURL != "" {
			cfg.OpenAIBaseURL = a.BaseURL
		} else if provider == "ollama" {
			cfg.OpenAIBaseURL = "http://localhost:11434/v1"
		}

	case "cloudflare":
		if cfAccountID == "" {
			return fmt.Errorf("--provider=cloudflare requires CF_ACCOUNT_ID env var")
		}
		if cfGatewayID == "" {
			return fmt.Errorf("--provider=cloudflare requires CF_GATEWAY_ID env var")
		}
		if cfAPIToken == "" {
			return fmt.Errorf("--provider=cloudflare requires CF_API_TOKEN env var")
		}
		cfg.CloudflareAccountID = cfAccountID
		cfg.CloudflareGatewayID = cfGatewayID
		cfg.CloudflareAPIToken = cfAPIToken
		if cfProvider == "" {
			cfProvider = "workers-ai" // default to Workers AI
		}
		cfg.CloudflareProvider = cfProvider
		cfg.CloudflareCacheTTL = 3600 // Cache responses for 1 hour
		if a.Model != "" {
			cfg.CloudflareModel = a.Model
		} else {
			cfg.CloudflareModel = "@cf/google/gemma-4-26b-a4b-it" // best free model for agentic work
		}

	default:
		return fmt.Errorf("unknown provider %q (supported: gemini, openai, ollama, cloudflare)", provider)
	}

	if a.Workspace != "" {
		cfg.Workspaces = []adk.WorkspaceDef{{Directory: a.Workspace}}
	}

	if a.ThinkingLevel != "" {
		cfg.GeminiThinkingLevel = a.ThinkingLevel
	}

	return nil
}

// String returns a human-readable summary of the resolved config.
func (a AegisConfig) String() string {
	var b strings.Builder
	b.WriteString("Resolved Aegis Config:\n")
	writeField(&b, "provider", a.Provider, "(auto-detect)")
	writeField(&b, "model", a.Model, "(provider default)")
	writeField(&b, "base_url", a.BaseURL, "(none)")
	writeField(&b, "workspace", a.Workspace, "(cwd)")

	if a.Reporting.BaseURL != "" {
		b.WriteString("\n  reporting:\n")
		b.WriteString(fmt.Sprintf("    base_url: %s\n", a.Reporting.BaseURL))
		writeField(&b, "  project_id", a.Reporting.ProjectID, "(none)")
		if os.Getenv("AEGIS_API_KEY") != "" {
			b.WriteString("    api_key: (set via AEGIS_API_KEY)\n")
		} else {
			b.WriteString("    api_key: (NOT SET — required for server push)\n")
		}
	}

	if len(a.Personas) > 0 {
		b.WriteString("\n  personas:\n")
		for name, pc := range a.Personas {
			b.WriteString(fmt.Sprintf("    %s:\n", name))
			if pc.Provider != "" {
				b.WriteString(fmt.Sprintf("      provider: %s\n", pc.Provider))
			}
			if pc.Model != "" {
				b.WriteString(fmt.Sprintf("      model: %s\n", pc.Model))
			}
			if pc.BaseURL != "" {
				b.WriteString(fmt.Sprintf("      base_url: %s\n", pc.BaseURL))
			}
		}
	}
	return b.String()
}

// ── Resolution ───────────────────────────────────────────────────────────────

// ResolveConfig builds a fully resolved AegisConfig by merging sources.
//
// Priority (highest wins):
//
//	CLI flags > env vars > --config file > .aegis/config.yml (workspace) > ~/.pixelvide/agents/aegis/config.yml (global)
func ResolveConfig(cli CLIOverrides) AegisConfig {
	var cfg AegisConfig

	// 1. Global config — lowest priority
	if home, err := os.UserHomeDir(); err == nil {
		globalPath := filepath.Join(home, globalConfigRelPath)
		if c, err := readConfigFile(globalPath); err == nil {
			cfg = c
		}
	}

	// 2. Workspace config (.aegis/config.yml) — overrides global
	//    Use CLI workspace first, then config workspace, then cwd
	wsDir := cli.Workspace
	if wsDir == "" {
		wsDir = cfg.Workspace // from global config
	}
	if wsDir == "" {
		wsDir, _ = os.Getwd()
	}
	if wsDir != "" {
		wsPath := filepath.Join(wsDir, ".aegis", "config.yml")
		if c, err := readConfigFile(wsPath); err == nil {
			mergeConfig(&cfg, &c)
		}
	}

	// 3. Explicit --config file — overrides workspace config
	if cli.ConfigFile != "" {
		if c, err := readConfigFile(cli.ConfigFile); err == nil {
			mergeConfig(&cfg, &c)
		} else {
			fmt.Fprintf(os.Stderr, "⚠️  Warning: could not read config file %s: %v\n", cli.ConfigFile, err)
		}
	}

	// 4. Env vars — override config files but not CLI flags
	if v := os.Getenv("AEGIS_BASE_URL"); v != "" && cfg.Reporting.BaseURL == "" {
		cfg.Reporting.BaseURL = v
	}
	if v := os.Getenv("AEGIS_PROJECT_ID"); v != "" && cfg.Reporting.ProjectID == "" {
		cfg.Reporting.ProjectID = v
	}
	// NOTE: AEGIS_API_KEY is read directly from env at point of use (not stored in config).

	// 5. CLI flags — highest priority, override everything
	mergeProviderFields(&cfg.Provider, &cfg.Model, &cfg.BaseURL, &cfg.ThinkingLevel, cli.Provider, cli.Model, cli.BaseURL, "")
	if cli.Workspace != "" {
		cfg.Workspace = cli.Workspace
	}
	if cli.ReportBaseURL != "" {
		cfg.Reporting.BaseURL = cli.ReportBaseURL
	}
	if cli.ProjectID != "" {
		cfg.Reporting.ProjectID = cli.ProjectID
	}

	// Scan pipeline CLI overrides
	if cli.Profile != "" {
		cfg.Profile = cli.Profile
	}
	if cli.MaxConcurrent > 0 {
		cfg.Scan.MaxConcurrent = cli.MaxConcurrent
	}
	if cli.BestOfK > 0 {
		cfg.Scan.BestOfK = cli.BestOfK
	}

	return cfg
}

// ── Internal Helpers ─────────────────────────────────────────────────────────

// readConfigFile reads and parses a YAML config file.
func readConfigFile(path string) (AegisConfig, error) {
	var cfg AegisConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
}

// mergeConfig merges src into dst, overriding only non-empty fields.
func mergeConfig(dst, src *AegisConfig) {
	mergeProviderFields(&dst.Provider, &dst.Model, &dst.BaseURL, &dst.ThinkingLevel, src.Provider, src.Model, src.BaseURL, src.ThinkingLevel)
	if src.Workspace != "" {
		dst.Workspace = src.Workspace
	}
	if src.Profile != "" {
		dst.Profile = src.Profile
	}
	// Merge reporting config
	if src.Reporting.BaseURL != "" {
		dst.Reporting.BaseURL = src.Reporting.BaseURL
	}
	if src.Reporting.ProjectID != "" {
		dst.Reporting.ProjectID = src.Reporting.ProjectID
	}
	// Merge roles config
	mergeRoleConfig(&dst.Roles.Scanner, src.Roles.Scanner)
	mergeRoleConfig(&dst.Roles.Reviewer, src.Roles.Reviewer)
	mergeRoleConfig(&dst.Roles.ExploitWriter, src.Roles.ExploitWriter)
	// Merge scan config (non-zero values only)
	if src.Scan.MaxConcurrent > 0 {
		dst.Scan.MaxConcurrent = src.Scan.MaxConcurrent
	}
	if src.Scan.BestOfK > 0 {
		dst.Scan.BestOfK = src.Scan.BestOfK
	}
	if src.Scan.ChunkMaxLines > 0 {
		dst.Scan.ChunkMaxLines = src.Scan.ChunkMaxLines
	}
	// Merge per-persona configs
	for name, pc := range src.Personas {
		if dst.Personas == nil {
			dst.Personas = make(map[string]PersonaConfig)
		}
		existing := dst.Personas[name]
		mergeProviderFields(&existing.Provider, &existing.Model, &existing.BaseURL, &existing.ThinkingLevel, pc.Provider, pc.Model, pc.BaseURL, pc.ThinkingLevel)
		dst.Personas[name] = existing
	}
}

// mergeProviderFields is the single merge function for the provider/model/baseURL triple.
// It sets dst fields from src values, but only if the src value is non-empty.
func mergeProviderFields(dstProvider, dstModel, dstBaseURL, dstThinkingLevel *string, srcProvider, srcModel, srcBaseURL, srcThinkingLevel string) {
	if srcProvider != "" {
		*dstProvider = srcProvider
	}
	if srcModel != "" {
		*dstModel = srcModel
	}
	if srcBaseURL != "" {
		*dstBaseURL = srcBaseURL
	}
	if srcThinkingLevel != "" {
		*dstThinkingLevel = srcThinkingLevel
	}
}

// writeField writes a key-value line to the builder, showing a fallback label if empty.
func writeField(b *strings.Builder, key, value, fallback string) {
	if value != "" {
		b.WriteString(fmt.Sprintf("  %s: %s\n", key, value))
	} else {
		b.WriteString(fmt.Sprintf("  %s: %s\n", key, fallback))
	}
}
