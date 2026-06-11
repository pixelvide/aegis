import { useState, useCallback } from "react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog"
import { Building2, Globe, AlertCircle } from "lucide-react"
import { orgsApi } from "@/lib/api"
import type { Organization } from "@/lib/types"

interface CreateOrgDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreated: (org: Organization) => void
}

/** Converts a name to a URL-friendly slug (matches Go's store.SanitizeSlug). */
function toSlug(name: string): string {
  return name
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9-]/g, "")
}

/** Validates a slug: 3-50 lowercase alphanumeric or hyphens, can't start/end with hyphen. */
function isValidSlug(slug: string): boolean {
  return /^[a-z0-9][a-z0-9-]{1,48}[a-z0-9]$/.test(slug)
}

export function CreateOrgDialog({ open, onOpenChange, onCreated }: CreateOrgDialogProps) {
  const [name, setName] = useState("")
  const [slug, setSlug] = useState("")
  const [slugTouched, setSlugTouched] = useState(false)
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState("")

  // Reset form when dialog opens; pass through close events
  const handleOpenChange = useCallback((nextOpen: boolean) => {
    if (nextOpen) {
      setName("")
      setSlug("")
      setSlugTouched(false)
      setError("")
      setCreating(false)
    }
    onOpenChange(nextOpen)
  }, [onOpenChange])

  // Auto-generate slug from name (until user manually edits slug)
  const handleNameChange = useCallback((value: string) => {
    setName(value)
    if (!slugTouched) {
      setSlug(toSlug(value))
    }
  }, [slugTouched])

  const handleSlugChange = useCallback((value: string) => {
    // Only allow valid slug characters
    const sanitized = value.toLowerCase().replace(/[^a-z0-9-]/g, "")
    setSlug(sanitized)
    setSlugTouched(true)
  }, [])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    const trimmedName = name.trim()

    if (!trimmedName) {
      setError("Organization name is required")
      return
    }

    if (!isValidSlug(slug)) {
      setError("Slug must be 3-50 lowercase alphanumeric characters or hyphens, and can't start or end with a hyphen")
      return
    }

    setCreating(true)
    setError("")

    try {
      const org = await orgsApi.create({ name: trimmedName, slug })
      onCreated(org)
      onOpenChange(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create organization")
    } finally {
      setCreating(false)
    }
  }

  const slugValid = slug.length === 0 || isValidSlug(slug)

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-md" id="create-org-dialog">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Building2 className="h-4 w-4" />
            Create Organization
          </DialogTitle>
          <DialogDescription>
            Organizations are isolated workspaces with their own members, projects, and findings.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-4 mt-2">
          {/* Name */}
          <div className="space-y-1.5">
            <label htmlFor="org-name-input" className="text-sm font-medium block">Organization Name</label>
            <Input
              id="org-name-input"
              placeholder="e.g., Acme Security"
              value={name}
              onChange={(e) => handleNameChange(e.target.value)}
              maxLength={100}
              autoFocus
            />
          </div>

          {/* Slug */}
          <div className="space-y-1.5">
            <label htmlFor="org-slug-input" className="text-sm font-medium block">URL Slug</label>
            <div className="flex items-center gap-2">
              <div className="flex items-center gap-1.5 text-xs text-muted-foreground shrink-0">
                <Globe className="h-3 w-3" />
                <span className="font-mono">
                  {slug || "slug"}.aegis.io
                </span>
              </div>
            </div>
            <Input
              id="org-slug-input"
              placeholder="acme-security"
              value={slug}
              onChange={(e) => handleSlugChange(e.target.value)}
              maxLength={50}
              className={`font-mono text-sm ${!slugValid ? "border-destructive focus-visible:ring-destructive" : ""}`}
            />
            {!slugValid && slug.length > 0 && (
              <p className="text-xs text-destructive">
                Must be 3-50 lowercase letters, numbers, or hyphens. Can't start or end with a hyphen.
              </p>
            )}
            {slugValid && slug.length > 0 && (
              <p className="text-xs text-muted-foreground">
                This will be your organization's subdomain.
              </p>
            )}
          </div>

          {/* Error */}
          {error && (
            <div className="flex items-start gap-2 text-sm text-destructive bg-destructive/5 border border-destructive/20 rounded-md px-3 py-2">
              <AlertCircle className="h-4 w-4 shrink-0 mt-0.5" />
              <span>{error}</span>
            </div>
          )}

          {/* Actions */}
          <div className="flex justify-end gap-2 pt-2">
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={creating}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              disabled={creating || !name.trim() || !slugValid || slug.length === 0}
              id="create-org-submit"
            >
              {creating ? "Creating..." : "Create Organization"}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
