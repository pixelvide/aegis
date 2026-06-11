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
import { Badge } from "@/components/ui/badge"
import { Copy, Check, AlertTriangle, Key } from "lucide-react"
import { tokensApi, projectsApi } from "@/lib/api"
import type { Project } from "@/lib/types"

interface CreateTokenDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreated: () => void
}

export function CreateTokenDialog({ open, onOpenChange, onCreated }: CreateTokenDialogProps) {
  const [step, setStep] = useState<"form" | "result">("form")
  const [name, setName] = useState("")
  const [projectId, setProjectId] = useState("")
  const [expiresIn, setExpiresIn] = useState(90)
  const [projects, setProjects] = useState<Project[]>([])
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState("")
  const [plaintext, setPlaintext] = useState("")
  const [copied, setCopied] = useState(false)

  const resetAndOpen = () => {
    setStep("form")
    setName("")
    setProjectId("")
    setExpiresIn(90)
    setError("")
    setPlaintext("")
    setCopied(false)
    projectsApi.list().then(setProjects).catch(() => setProjects([]))
  }

  // Reset form state whenever the dialog opens
  const handleOpenChange = (value: boolean) => {
    if (value) resetAndOpen()
    onOpenChange(value)
  }

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) return

    setCreating(true)
    setError("")
    try {
      const result = await tokensApi.create({
        name: name.trim(),
        project_id: projectId || undefined,
        expires_in: expiresIn || undefined,
      })
      setPlaintext(result.token)
      setStep("result")
      onCreated()
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create token")
    } finally {
      setCreating(false)
    }
  }

  const handleCopy = async () => {
    await navigator.clipboard.writeText(plaintext)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-md">
        {step === "form" ? (
          <>
            <DialogHeader>
              <DialogTitle className="flex items-center gap-2">
                <Key className="h-4 w-4" />
                Create API Token
              </DialogTitle>
              <DialogDescription>
                Generate a token for agent authentication. Tokens use Bearer auth and are scoped to this organization.
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
                <label className="text-sm font-medium block mb-1.5">
                  Project Scope <span className="text-muted-foreground font-normal">(optional)</span>
                </label>
                <select
                  className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-xs transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                  value={projectId}
                  onChange={(e) => setProjectId(e.target.value)}
                  id="token-project-select"
                >
                  <option value="">Org-wide (all projects)</option>
                  {projects.map((p) => (
                    <option key={p.id} value={p.id}>{p.name}</option>
                  ))}
                </select>
                <p className="text-xs text-muted-foreground mt-1.5">
                  Scope this token to a specific project, or leave empty for org-wide access.
                </p>
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
                  Copy this token now. It will only be shown once and cannot be retrieved later.
                </p>
              </div>

              {/* Token display */}
              <div className="relative">
                <div className="rounded-md bg-muted p-3 pr-12 font-mono text-xs break-all select-all">
                  {plaintext}
                </div>
                <button
                  onClick={handleCopy}
                  className="absolute top-2.5 right-2.5 p-1.5 rounded hover:bg-background/80 transition-colors"
                  title="Copy to clipboard"
                  id="copy-token-button"
                >
                  {copied ? (
                    <Check className="h-4 w-4 text-green-500" />
                  ) : (
                    <Copy className="h-4 w-4 text-muted-foreground" />
                  )}
                </button>
              </div>

              {/* Usage hint */}
              <div className="rounded-md bg-muted/50 p-3">
                <p className="text-xs font-medium mb-1.5">Usage</p>
                <code className="text-xs text-muted-foreground block">
                  Authorization: Bearer {plaintext.slice(0, 14)}...
                </code>
              </div>

              {copied && (
                <Badge variant="secondary" className="text-green-600">
                  <Check className="h-3 w-3 mr-1" />
                  Copied to clipboard
                </Badge>
              )}

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
