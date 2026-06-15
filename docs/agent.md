# Aegis Agent

The Aegis agent is an AI-powered security scanning tool that analyzes codebases for vulnerabilities using specialized personas. Each persona targets a different security research domain and produces detailed reports with working exploit PoC code.

---

## Quick Start

```bash
# Build from source
cd agent && go build -o aegis .

# Run a full security audit
GEMINI_API_KEY=your-key ./aegis sharingan

# Push findings to Aegis server
AEGIS_API_KEY=aegis_xxx AEGIS_BASE_URL=https://acme.aegis.io AEGIS_PROJECT_ID=<uuid> \
  GEMINI_API_KEY=your-key ./aegis sharingan
```

---

## Installation

### From Source

Requires Go 1.25+.

```bash
cd agent
go build -o aegis .
```

The `agent/go.mod` uses a `replace` directive pointing at the local `local-harness/` checkout. For production builds, remove the `replace` directive and ensure the `github.com/pixelvide/localharness` module is accessible.

### Docker (pull from registry)

```bash
docker pull ghcr.io/pixelvide/aegis-agent:latest
```

### Docker (build locally)

Requires Go 1.25+ and Docker. The build cross-compiles both the `localharness` runtime and the `aegis` agent, then packages them into an Ubuntu 24.04 image.

```bash
cd agent
make docker
```

> **Note:** The Makefile expects the `local-harness/` repo at `../../local-harness` (sibling directory). Adjust `LOCALHARNESS_REPO` in the Makefile if your layout differs.

**Pre-installed security tools** (available to personas at runtime):
- **SAST:** `semgrep` — static analysis
- **SAST:** `codeql` — deep semantic code analysis (GitHub CodeQL bundle with standard query packs)
- **Go:** `govulncheck` — Go vulnerability scanner
- **Python:** `pip-audit` — Python dependency audit
- **PHP:** `composer` — PHP package manager + `composer audit`
- **Node.js:** `npm audit` — JavaScript dependency audit
- **General:** `curl`, `wget`, `git`, `jq`, `ripgrep`, `nmap`, `dnsutils`
- **Runtime:** `python3`, `pip`, `venv`, `build-essential`
- Full `apt` access for installing additional tools at runtime

---

## Personas

| Persona | Icon | Focus | Default Model |
|---|---|---|---|
| `sharingan` | 👁️ | Full security audit & reconnaissance | gemini-3.5-flash |
| `senku` | 🧪 | Supply chain & dependency analysis | gemini-3.5-flash |
| `killua` | ⚡ | Targeted penetration testing | gemini-3.5-flash |

### Sharingan — Full Security Audit

Performs comprehensive security auditing inspired by the Sharingan's all-seeing eye. Covers:
- Code injection (SQLi, XSS, command injection)
- Authentication & session management flaws
- Cryptographic weaknesses
- Access control issues
- Business logic vulnerabilities

```bash
aegis sharingan                                    # Audit current directory
aegis --workspace=/path/to/project sharingan       # Audit specific project
```

### Senku — Supply Chain Analysis

Analyzes dependencies and supply chain security. Covers:
- Known CVEs in dependencies
- Typosquatting detection
- License compliance issues
- Abandoned/unmaintained packages
- Build pipeline security

```bash
aegis senku                                        # Analyze current project
```

### Killua — Targeted Penetration Testing

Focused, high-speed penetration testing on specific targets. Covers:
- Targeted code path analysis
- Exploit development and validation
- Attack chain construction

```bash
aegis killua "Test the auth flow in src/auth/"     # Target specific code
aegis --target=https://staging.example.com killua  # With live validation
```

---

## Configuration

Configuration uses a layered resolution system. Higher-priority sources override lower ones:

```
CLI flags > env vars > workspace config > global config
```

### Config Sources

| Priority | Source | Path |
|---|---|---|
| 5 (highest) | CLI flags | `--provider`, `--model`, etc. |
| 4 | Environment variables | `AEGIS_BASE_URL`, `AEGIS_API_KEY`, etc. |
| 3 | Explicit config file | `--config=/path/to/config.yml` |
| 2 | Workspace config | `.aegis/config.yml` |
| 1 (lowest) | Global config | `~/.pixelvide/agents/aegis/config.yml` |

### LLM Provider Settings

| Setting | CLI Flag | Env Var | config.yml key |
|---|---|---|---|
| Provider | `--provider` | — | `provider` |
| Model | `--model` | — | `model` |
| Base URL | `--base-url` | — | `base_url` |

**Supported providers:** `gemini`, `openai`, `ollama`, `cloudflare`

### Server Reporting Settings

| Setting | CLI Flag | Env Var | config.yml key | Notes |
|---|---|---|---|---|
| Server URL | `--report-base-url` | `AEGIS_BASE_URL` | `reporting.base_url` | Not a secret |
| API Key | — | `AEGIS_API_KEY` | — | **Env-var-only** |
| Project ID | `--project` | `AEGIS_PROJECT_ID` | `reporting.project_id` | Not a secret |

> **Security:** `AEGIS_API_KEY` is env-var-only by design. No CLI flag (would leak in `ps aux` and `.bash_history`) and no config.yml support (would risk committing secrets to git).

### Example `config.yml`

```yaml
provider: gemini
model: gemini-3.5-flash

reporting:
  base_url: https://acme.aegis.io
  project_id: "550e8400-e29b-41d4-a716-446655440000"

personas:
  sharingan:
    model: gemini-2.5-pro  # Use bigger model for deep analysis
```

### Show Resolved Config

```bash
aegis config show
```

---

## Server Reporting

When `AEGIS_BASE_URL` and `AEGIS_API_KEY` are set, the agent integrates with the Aegis server for scan tracking and finding reporting.

### Scan Lifecycle

Each agent run follows a structured lifecycle:

1. **Start scan** — The agent calls `POST /api/v1/agent/scans` with `{scan_id, project_id, persona, target}`. The server creates a scan record with `status: "running"`.
2. **Push findings** — As the persona discovers vulnerabilities, each finding is pushed via `POST /api/v1/agent/findings` with the `scan_id`.
3. **Complete scan** — When the scan finishes, the agent calls `PATCH /api/v1/agent/scans/{id}` with `{status: "completed"}`. The server computes the severity summary automatically from the findings.

If the agent crashes or is killed (SIGINT/SIGTERM), a signal handler sends `{status: "failed", error_message: "agent terminated by signal: ..."}` before exiting. The Aegis UI also detects "stale" scans that have been running for more than 2 hours and displays a warning.

### How Findings Are Pushed

1. The agent registers a `report_finding` host tool with the LLM
2. When a persona discovers a vulnerability, it calls `report_finding()` with structured data
3. The reporter pushes the finding to `POST /api/v1/agent/findings` with Bearer token auth
4. If the push fails, it retries 3 times with exponential backoff (1s, 2s, 4s)
5. All findings are always saved locally to `.aegis/findings.json` regardless of server connectivity

### Scan Correlation

Each agent run generates a UUID v7 `scan_id` at startup. All findings from that run share the same `scan_id`, which the server uses to group findings into scans. The `scan_id` is also used as the scan record's primary key.

### Local Output

Findings are always written to `.aegis/findings.json` in the workspace:

```json
{
  "scan_id": "0195xxxx-xxxx-7xxx-xxxx-xxxxxxxxxxxx",
  "started_at": "2026-06-12T10:00:00Z",
  "findings": [
    {
      "fingerprint": "AEGIS-001",
      "title": "SQL Injection in UserController",
      "severity": "critical",
      "cwe": "CWE-89",
      "description": "...",
      "server_synced": true,
      "scan_id": "0195xxxx-xxxx-7xxx-xxxx-xxxxxxxxxxxx"
    }
  ]
}
```


---

## Docker Usage

### Basic

```bash
docker run --rm \
  -v /path/to/project:/workspace \
  -e GEMINI_API_KEY=your-key \
  ghcr.io/pixelvide/aegis-agent sharingan
```

### With Server Reporting

```bash
docker run --rm \
  -v /path/to/project:/workspace \
  -e GEMINI_API_KEY=your-key \
  -e AEGIS_API_KEY=aegis_xxx \
  -e AEGIS_BASE_URL=https://acme.aegis.io \
  -e AEGIS_PROJECT_ID=<uuid> \
  ghcr.io/pixelvide/aegis-agent sharingan
```

### Using `aegis-run` Script

The `aegis-run` convenience script handles Docker mounts and env var forwarding:

```bash
./agent/aegis-run sharingan                              # Audit current directory
./agent/aegis-run --workspace=/path/to/project sharingan # Audit specific project
./agent/aegis-run killua "Test PaymentController"        # Targeted pentest
```

The script automatically:
- Mounts the workspace directory into the container
- Forwards all `GEMINI_API_KEY`, `OPENAI_API_KEY`, `AEGIS_*` env vars
- Overrides `--workspace` to the container mount point

---

## CI/CD Integration

### GitHub Actions

```yaml
name: Security Scan
on:
  push:
    branches: [main]
  pull_request:

jobs:
  aegis-scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run Aegis Security Scan
        run: |
          docker run --rm \
            -v ${{ github.workspace }}:/workspace \
            -e GEMINI_API_KEY=${{ secrets.GEMINI_API_KEY }} \
            -e AEGIS_API_KEY=${{ secrets.AEGIS_API_KEY }} \
            -e AEGIS_BASE_URL=${{ secrets.AEGIS_BASE_URL }} \
            -e AEGIS_PROJECT_ID=${{ secrets.AEGIS_PROJECT_ID }} \
            ghcr.io/pixelvide/aegis-agent sharingan

      - name: Upload Findings
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: aegis-findings
          path: .aegis/findings.json
```

### GitLab CI

```yaml
aegis-scan:
  stage: test
  image: ghcr.io/pixelvide/aegis-agent:latest
  variables:
    GEMINI_API_KEY: $GEMINI_API_KEY
    AEGIS_API_KEY: $AEGIS_API_KEY
    AEGIS_BASE_URL: $AEGIS_BASE_URL
    AEGIS_PROJECT_ID: $AEGIS_PROJECT_ID
  script:
    - aegis --workspace=/builds/$CI_PROJECT_PATH sharingan
  artifacts:
    when: always
    paths:
      - .aegis/findings.json
```

---

## Troubleshooting

### "no LLM provider configured"

Set one of the provider env vars:
```bash
export GEMINI_API_KEY=your-key       # For Gemini
export OPENAI_API_KEY=your-key       # For OpenAI
```

### "reporting.base_url is set but AEGIS_API_KEY env var is missing"

You've configured a server URL but no API key:
```bash
export AEGIS_API_KEY=aegis_xxx
```

### "server returned 401 (not retrying)"

Your API key is invalid or expired. Generate a new one from the Aegis UI: **Settings → API Tokens → Create Token**.

### "server returned 403 (not retrying)"

The API token doesn't have access to the specified project. Check the token's project scope in the Aegis UI.

### Findings not appearing in Aegis UI

1. Check the agent output for "server_synced: false" in findings
2. Verify `AEGIS_BASE_URL` points to the correct server
3. Verify `AEGIS_PROJECT_ID` matches an existing project
4. Check `AEGIS_API_KEY` has not expired
5. Check `.aegis/findings.json` for locally saved findings
