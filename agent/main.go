// Aegis — AI-powered security research agent.
//
// Run a focused security scan by persona name:
//
//	aegis sharingan              # 👁️ Full security audit
//	aegis senku                  # 🧪 Supply chain & dependency audit
//	aegis killua                 # ⚡ Targeted penetration testing
//
// Each persona follows a different security research methodology
// and produces detailed reports with working exploit PoC code.
//
// Usage:
//
//	export GEMINI_API_KEY=your-key
//	go run . <persona> [prompt]
//	go build -o bin/aegis . && bin/aegis sharingan
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/pixelvide/aegis/agent/personas"
	"github.com/pixelvide/aegis/agent/scanners"
	"github.com/pixelvide/localharness/adk"
	"github.com/pixelvide/localharness/adk/policy"
)

func main() {
	// CLI flags
	workspace := flag.String("workspace", "", "Workspace directory (default: cwd)")
	provider := flag.String("provider", "", "LLM provider: gemini, openai, ollama, cloudflare")
	model := flag.String("model", "", "Model name (e.g., gemini-3.5-flash, gpt-4o)")
	baseURL := flag.String("base-url", "", "Base URL for OpenAI-compatible providers")
	configFile := flag.String("config", "", "Path to YAML config file")
	targetURL := flag.String("target", "", "Target URL for live exploit validation (e.g., https://staging.example.com)")
	verbose := flag.Bool("verbose", false, "Enable verbose debug logging")
	flag.BoolVar(verbose, "v", false, "Enable verbose debug logging (shorthand)")

	// Scan pipeline flags
	profile := flag.String("profile", "", "Scan profile: fast (cheap+fast), deep (tiered+thorough)")
	maxConcurrent := flag.Int("max-concurrent", 0, "Max parallel scanner subagents (default: 5)")
	bestOfK := flag.Int("best-of-k", 0, "Scanner retries per chunk (default: 3)")

	// Server reporting flags (AEGIS_API_KEY is env-var-only — not a CLI flag)
	reportBaseURL := flag.String("report-base-url", "", "Aegis server URL for pushing findings (e.g., https://acme.aegis.io)")
	reportProject := flag.String("project", "", "Project ID for server reporting")

	flag.Usage = printUsage
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		printUsage()
		os.Exit(1)
	}

	// Handle "config show" subcommand
	if args[0] == "config" && len(args) >= 2 && args[1] == "show" {
		resolved := ResolveConfig(CLIOverrides{
			Provider:      *provider,
			Model:         *model,
			BaseURL:       *baseURL,
			Workspace:     *workspace,
			ConfigFile:    *configFile,
			Profile:       *profile,
			MaxConcurrent: *maxConcurrent,
			BestOfK:       *bestOfK,
			ReportBaseURL: *reportBaseURL,
			ProjectID:     *reportProject,
		})
		fmt.Print(resolved.String())
		os.Exit(0)
	}

	name := strings.ToLower(args[0])

	p, ok := personas.Get(name)
	if !ok {
		fmt.Fprintf(os.Stderr, "Unknown persona: %q\n\n", name)
		printUsage()
		os.Exit(1)
	}

	// Optional custom prompt; otherwise use the persona's default.
	prompt := p.DefaultMessage()
	if len(args) >= 2 {
		prompt = strings.Join(args[1:], " ")
	}

	// If --target is provided, inject live validation instructions into the prompt.
	if *targetURL != "" {
		prompt += fmt.Sprintf(`

--- LIVE EXPLOIT VALIDATION MODE ---
Target URL: %s

AFTER writing each exploit script, you MUST validate it:
1. Run the exploit script against the target: run_command "bash .aegis/findings/AEGIS-NNN/exploit.sh %s"
2. Check the output — did the exploit succeed?
3. If YES → set "validated: true" in the finding's frontmatter
4. If NO → analyze the error, refine the exploit, and retry (max 3 attempts)
5. After 3 failed attempts → set "validated: false" and note the failure reason

Update the finding.md frontmatter with:
  validated: true/false
  validated_against: %s
  validation_output: "<first 200 chars of exploit output>"

⚠️ ONLY run exploits against the target URL above. NEVER guess or construct other URLs.`, *targetURL, *targetURL, *targetURL)
	}

	cfg := adk.NewLocalAgentConfig()
	cfg.Capabilities = adk.AllTools()
	cfg.Policies = []policy.Policy{policy.AllowAll()}
	cfg.EnablePlanningMode = false
	cfg.StructuredPrompt = p.Prompt()
	cfg.Verbose = *verbose

	// Register security-focused subagent types.
	// These extend the built-in "research" and "self" types with
	// domain-specific agents that personas can delegate to.
	cfg.SubagentTypes = []adk.SubagentTypeDef{
		{
			Name:             "exploit-writer",
			Description:      exploitWriterDesc,
			SystemPrompt:     exploitWriterPrompt,
			EnableWriteTools: true, // Creates files in .aegis/findings/
		},
		{
			Name:         "deep-tracer",
			Description:  deepTracerDesc,
			SystemPrompt: deepTracerPrompt,
			// EnableWriteTools defaults to false — read-only analysis
		},
		{
			Name:         "chunk-scanner",
			Description:  chunkScannerDesc,
			SystemPrompt: chunkScannerPrompt,
			// Read-only — returns JSON findings
		},
		{
			Name:         "finding-reviewer",
			Description:  findingReviewerDesc,
			SystemPrompt: findingReviewerPrompt,
			// Read-only — returns JSON review verdicts
		},
	}

	// Enable auto-wake so the agent can process async subagent completion
	// notifications without manual follow-up prompts.
	cfg.MaxAutoWakeTurns = 15

	// Resolve config: CLI flags > env vars > --config file > .aegis/config.yml > ~/.pixelvide/agents/aegis/config.yml
	resolved := ResolveConfig(CLIOverrides{
		Provider:      *provider,
		Model:         *model,
		BaseURL:       *baseURL,
		Workspace:     *workspace,
		ConfigFile:    *configFile,
		Profile:       *profile,
		MaxConcurrent: *maxConcurrent,
		BestOfK:       *bestOfK,
		ReportBaseURL: *reportBaseURL,
		ProjectID:     *reportProject,
	})

	// Apply per-persona overrides (e.g., sharingan uses a different model)
	resolved = resolved.ForPersona(name)

	// Validate config before applying
	if err := resolved.Validate(); err != nil {
		log.Fatal(err)
	}

	// Apply resolved config to SDK
	if err := resolved.ApplyTo(cfg); err != nil {
		log.Fatal(err)
	}

	// Generate a UUID v7 scan ID for this run (time-sortable, unique per scan).
	scanID := uuid.Must(uuid.NewV7()).String()

	// Initialize the reporter for server push + local JSON.
	reporter := NewReporter(ReporterConfig{
		BaseURL:   resolved.Reporting.BaseURL,
		APIKey:    os.Getenv("AEGIS_API_KEY"),
		ProjectID: resolved.Reporting.ProjectID,
		ScanID:    scanID,
		Workspace: resolved.Workspace,
	})

	// Register the report_finding host tool so the LLM can push findings
	// directly with structured data — no file scraping or hooks needed.
	cfg.HostTools = append(cfg.HostTools, reporter.HostTool())

	// Register the run_scanner host tool so the LLM can invoke external
	// static analysis tools (semgrep, trivy, bandit, gosec) and get
	// normalized findings back as structured JSON.
	scannerRegistry := scanners.NewRegistry()
	cfg.HostTools = append(cfg.HostTools, scannerRegistry.HostTool(resolved.Workspace))

	agent, err := adk.NewAgent(cfg)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ctx := context.Background()
	if err := agent.Start(ctx); err != nil {
		log.Fatalf("Failed to start: %v", err)
	}
	defer agent.Close()

	providerName := resolved.Provider
	if providerName == "" {
		providerName = "gemini"
	}
	fmt.Printf("🛡️  Aegis — %s\n", p.Description())
	fmt.Printf("🤖 Provider: %s | Model: %s\n", providerName, resolved.Model)
	fmt.Printf("🔑 Scan ID: %s\n", scanID)
	if reporter.IsServerEnabled() {
		apiKeyPrefix := os.Getenv("AEGIS_API_KEY")
		if len(apiKeyPrefix) > 14 {
			apiKeyPrefix = apiKeyPrefix[:14] + "..."
		}
		fmt.Printf("📡 Server: %s (key: %s)\n", resolved.Reporting.BaseURL, apiKeyPrefix)
		if resolved.Reporting.ProjectID != "" {
			fmt.Printf("📂 Project: %s\n", resolved.Reporting.ProjectID)
		}
	} else {
		fmt.Println("📡 Server: offline (local findings.json only)")
	}
	if *targetURL != "" {
		fmt.Printf("🎯 Target: %s (live validation ON)\n", *targetURL)
	}
	fmt.Printf("📎 Conversation ID: %s\n", agent.ConversationID())

	// ── Execution path: Pipeline vs Legacy Chat ────────────────────────
	if p.SupportsPipeline() {
		// Pipeline mode: parallel chunked scanning with best-of-K.
		scanCfg := resolved.ResolveScan()
		fmt.Printf("🔬 Mode: Pipeline (chunks=%d, best-of-k=%d, concurrent=%d)\n",
			scanCfg.ChunkMaxLines, scanCfg.BestOfK, scanCfg.MaxConcurrent)
		if resolved.Profile != "" {
			fmt.Printf("📊 Profile: %s\n", resolved.Profile)
		}
		fmt.Printf("📤 Prompt: %s\n\n", prompt)

		runner, err := NewPipelineRunner(agent, resolved, reporter)
		if err != nil {
			log.Fatalf("Failed to create pipeline runner: %v", err)
		}

		if err := runner.Run(ctx, prompt); err != nil {
			log.Fatalf("Pipeline failed: %v", err)
		}
	} else {
		// Legacy mode: single agent.Chat() call.
		fmt.Printf("📤 Prompt: %s\n\n", prompt)

		resp, err := agent.Chat(ctx, prompt)
		if err != nil {
			log.Fatalf("Chat failed: %v", err)
		}

		fmt.Println(resp.Text)

		fmt.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("Steps: %d\n", len(resp.Steps))
		if resp.Usage != nil {
			fmt.Printf("Tokens: prompt=%d, completion=%d, total=%d\n",
				resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
		}
		fmt.Printf("Scan ID: %s\n", scanID)
		fmt.Printf("Findings: %s\n", reporter.SummaryLine())
	}

	// Flush any buffered findings to local JSON.
	if err := reporter.Close(); err != nil {
		log.Printf("Warning: failed to flush reporter: %v", err)
	}
}


func printUsage() {
	fmt.Fprintf(os.Stderr, `Aegis — AI-powered security research agent.

Usage: aegis [flags] <persona> [prompt]
       aegis config show                     Show resolved config and exit

  Note: Flags must come BEFORE the persona name.

Flags:
  --workspace=<path>       Workspace directory (default: cwd)
  --provider=<name>        LLM provider: gemini, openai, ollama, cloudflare (default: auto-detect from env)
  --model=<name>           Model name (default: provider-specific)
  --base-url=<url>         Base URL for OpenAI-compatible providers
  --target=<url>           Target URL for live exploit validation (runs exploits against this URL)
  --config=<path>          Path to YAML config file (overrides auto-discovery)
  --profile=<name>         Scan profile: fast (cheap+fast), deep (tiered+thorough)
  --max-concurrent=<n>     Max parallel scanner subagents (default: 5)
  --best-of-k=<n>          Scanner retries per chunk (default: 3, higher = better coverage)
  --report-base-url=<url>  Aegis server URL for pushing findings
  --project=<uuid>         Project ID for server reporting
  --verbose, -v            Enable verbose debug logging
  --help, -h               Show this help

Environment:
  GEMINI_API_KEY           Gemini API key (auto-selects gemini provider)
  OPENAI_API_KEY           OpenAI API key (auto-selects openai provider)
  CF_ACCOUNT_ID            Cloudflare account ID (auto-selects cloudflare provider)
  CF_GATEWAY_ID            Cloudflare AI Gateway ID
  CF_API_TOKEN             Cloudflare API token
  AEGIS_BASE_URL           Aegis server URL (alternative to --report-base-url)
  AEGIS_API_KEY            API key for Aegis server (env-var-only, no CLI flag)
  AEGIS_PROJECT_ID         Project ID (alternative to --project)
  LOCALHARNESS_BIN         Optional. Path to localharness binary (default: auto-detect).

Config files (YAML):
  .aegis/config.yml          Per-workspace config (highest priority)
  ~/.pixelvide/agents/aegis/config.yml  Global config

  Example config.yml:
    provider: gemini
    model: gemini-2.5-flash
    profile: deep                        # fast or deep scan profile
    reporting:
      base_url: https://acme.aegis.io
      project_id: "your-project-uuid"

  Priority: CLI flags > env vars > workspace config > global config

Available personas:
`)
	all := personas.All()
	names := make([]string, 0, len(all))
	for n := range all {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, n := range names {
		fmt.Fprintf(os.Stderr, "  %-20s %s\n", n, all[n].Description())
	}

	fmt.Fprintf(os.Stderr, `
Examples:
  aegis sharingan                                      # Full security audit (Gemini default)
  aegis --model=gemini-2.5-pro sharingan               # Use bigger model for deep analysis
  aegis senku                                          # Dependency & supply chain audit
  aegis killua "Test src/controllers/PaymentCtrl.php"  # Targeted pentest
  aegis --target=https://staging.example.com sharingan # Live exploit validation
  aegis --provider=openai --model=gpt-4o sharingan     # OpenAI provider
  aegis --workspace=/path/to/project sharingan

  # Push findings to Aegis server:
  AEGIS_API_KEY=aegis_xxx aegis --report-base-url=https://acme.aegis.io --project=<uuid> sharingan

  # All via env vars (ideal for CI/Docker):
  AEGIS_BASE_URL=https://acme.aegis.io AEGIS_API_KEY=aegis_xxx AEGIS_PROJECT_ID=<uuid> aegis sharingan
`)
}
