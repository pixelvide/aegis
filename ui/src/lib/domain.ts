import { useEffect, useState } from "react"

// ─── Domain Detection ─────────────────────────────────────────────────────
// Determines whether we're on the base domain, an org subdomain, or
// in header-only mode (no base domain configured).
export type DomainMode = "base" | "org" | "header-only"

let cachedDomainResult: { mode: DomainMode; baseDomain: string } | null = null

export function useDomainMode(): { mode: DomainMode; baseDomain: string; loading: boolean } {
  const [result, setResult] = useState<{ mode: DomainMode; baseDomain: string }>(
    cachedDomainResult ?? { mode: "header-only", baseDomain: "" }
  )
  const [loading, setLoading] = useState(!cachedDomainResult)

  useEffect(() => {
    if (cachedDomainResult) return // Already resolved

    fetch("/api/v1/config/auth")
      .then(r => r.json())
      .then(body => {
        const data = body.result ?? body
        const bd = data.base_domain || ""
        let mode: DomainMode = "header-only"

        if (bd) {
          const host = window.location.hostname
          mode = host === bd ? "base" : "org"
        }

        cachedDomainResult = { mode, baseDomain: bd }
        setResult(cachedDomainResult)
      })
      .catch(() => {
        cachedDomainResult = { mode: "header-only", baseDomain: "" }
        setResult(cachedDomainResult)
      })
      .finally(() => setLoading(false))
  }, [])

  return { ...result, loading }
}

// Helper to build a base domain URL
export function baseDomainUrl(baseDomain: string, path: string): string {
  const protocol = window.location.protocol
  const port = window.location.port ? `:${window.location.port}` : ""
  return `${protocol}//${baseDomain}${port}${path}`
}
