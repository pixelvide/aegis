import { useState } from "react"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { Copy, Check, AlertTriangle, Key } from "lucide-react"
import { orgTokensApi, projectTokensApi } from "@/lib/api"
import { useOrg } from "@/lib/org-context"
import { copyToClipboard } from "@/lib/utils"

interface CreateTokenDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreated: () => void
  /** When set, creates a project-scoped token. When absent, creates an org-wide token. */
  projectId?: string
  /** Display name for the project (used in description text). */
  projectName?: string
}

export function CreateTokenDialog({
  open,
  onOpenChange,
  onCreated,
  projectId,
  projectName,
}: CreateTokenDialogProps) {
  const isProjectScoped = !!projectId
  const { currentOrg, baseDomain } = useOrg()

  const [step, setStep] = useState<"form" | "result">("form")
  const [name, setName] = useState("")
  const [expiresIn, setExpiresIn] = useState(90)
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState("")
  const [plaintext, setPlaintext] = useState("")
  const [copiedEnv, setCopiedEnv] = useState(false)

  const resetAndOpen = () => {
    setStep("form")
    setName("")
    setExpiresIn(90)
    setError("")
    setPlaintext("")
    setCopiedEnv(false)
  }

  // Reset form state whenever the dialog opens
  const handleOpenChange = (value: boolean) => {
    if (value) resetAndOpen()
    onOpenChange(value)
  }

  // Compute the base URL for the org
  const getBaseUrl = () => {
    const protocol = window.location.protocol
    if (baseDomain && currentOrg) {
      const port = window.location.port ? `:${window.location.port}` : ""
      return `${protocol}//${currentOrg.slug}.${baseDomain}${port}`
    }
    // Dev mode: use current origin
    return window.location.origin
  }

  // Build the .env content string
  const buildEnvContent = (token: string) => {
    const baseUrl = getBaseUrl()
    const pid = isProjectScoped ? projectId! : "<your-project-id>"
    return `AEGIS_BASE_URL=${baseUrl}\nAEGIS_PROJECT_ID=${pid}\nAEGIS_API_KEY=${token}`
  }

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) return

    setCreating(true)
    setError("")
    try {
      const payload = { name: name.trim(), expires_in: expiresIn || undefined }
      const result = isProjectScoped
        ? await projectTokensApi.create(payload, projectId!)
        : await orgTokensApi.create(payload)

      setPlaintext(result.token)
      setStep("result")
      onCreated()
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create token")
    } finally {
      setCreating(false)
    }
  }

  const handleCopyEnv = async () => {
    await copyToClipboard(buildEnvContent(plaintext))
    setCopiedEnv(true)
    setTimeout(() => setCopiedEnv(false), 2000)
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-lg">
        {step === "form" ? (
          <>
            <DialogHeader>
              <DialogTitle className="flex items-center gap-2">
                <Key className="h-4 w-4" />
                {isProjectScoped ? "Create Project Token" : "Create API Token"}
              </DialogTitle>
              <DialogDescription>
                {isProjectScoped
                  ? <>Generate a token scoped to <span className="font-medium text-foreground">{projectName}</span>. This token can only access data within this project.</>
                  : "Generate an org-wide token for agent authentication. Tokens use Bearer auth and are scoped to this organization."
                }
              </DialogDescription>
            </DialogHeader>

            <form onSubmit={handleCreate} className="space-y-4 mt-2">
              <div>
                <label className="text-sm font-medium block mb-1.5">Name</label>
                <Input
                  placeholder="e.g., CI Pipeline Token"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  id="token-name-input"
                  autoFocus
                />
              </div>

              <div>
                <label className="text-sm font-medium block mb-1.5">Expiration</label>
                <select
                  className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-xs transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                  value={expiresIn}
                  onChange={(e) => setExpiresIn(Number(e.target.value))}
                  id="token-expiry-select"
                >
                  <option value={30}>30 days</option>
                  <option value={60}>60 days</option>
                  <option value={90}>90 days</option>
                  <option value={180}>180 days</option>
                  <option value={365}>1 year</option>
                  <option value={0}>Never</option>
                </select>
              </div>

              {error && <p className="text-sm text-destructive">{error}</p>}

              <div className="flex justify-end gap-2 pt-2">
                <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
                  Cancel
                </Button>
                <Button type="submit" disabled={creating || !name.trim()}>
                  {creating ? "Creating..." : "Create Token"}
                </Button>
              </div>
            </form>
          </>
        ) : (
          <>
            <DialogHeader>
              <DialogTitle className="flex items-center gap-2">
                <Check className="h-4 w-4 text-green-500" />
                Token Created
              </DialogTitle>
            </DialogHeader>

            <div className="space-y-4 mt-2">
              {/* Warning */}
              <div className="flex items-start gap-2 rounded-md bg-amber-500/10 border border-amber-500/20 p-3">
                <AlertTriangle className="h-4 w-4 text-amber-500 mt-0.5 shrink-0" />
                <p className="text-sm text-amber-700 dark:text-amber-400">
                  Copy these credentials now. The API key will only be shown once and cannot be retrieved later.
                </p>
              </div>

              {/* .env block */}
              <div>
                <div className="flex items-center justify-between mb-1.5">
                  <span className="text-xs font-medium text-muted-foreground">.env</span>
                  <button
                    onClick={handleCopyEnv}
                    className="inline-flex items-center gap-1.5 px-2 py-1 rounded-md text-xs text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
                    title="Copy .env to clipboard"
                    id="copy-env-button"
                  >
                    {copiedEnv ? (
                      <>
                        <Check className="h-3 w-3 text-green-500" />
                        <span className="text-green-600">Copied</span>
                      </>
                    ) : (
                      <>
                        <Copy className="h-3 w-3" />
                        Copy
                      </>
                    )}
                  </button>
                </div>
                <div className="rounded-md bg-muted border border-border p-3 font-mono text-xs leading-relaxed select-all">
                  <div>
                    <span className="text-muted-foreground">AEGIS_BASE_URL</span>
                    <span className="text-muted-foreground/60">=</span>
                    <span className="text-foreground">{getBaseUrl()}</span>
                  </div>
                  <div>
                    <span className="text-muted-foreground">AEGIS_PROJECT_ID</span>
                    <span className="text-muted-foreground/60">=</span>
                    <span className="text-foreground">{isProjectScoped ? projectId : "<your-project-id>"}</span>
                  </div>
                  <div>
                    <span className="text-muted-foreground">AEGIS_API_KEY</span>
                    <span className="text-muted-foreground/60">=</span>
                    <span className="text-foreground break-all">{plaintext}</span>
                  </div>
                </div>
              </div>

              <div className="flex justify-end pt-2">
                <Button onClick={() => onOpenChange(false)}>
                  Done
                </Button>
              </div>
            </div>
          </>
        )}
      </DialogContent>
    </Dialog>
  )
}
