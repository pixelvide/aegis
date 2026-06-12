import { useState, useEffect } from "react"
import { Building2, Plus, ArrowRight, Loader2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { useAuth } from "@/lib/auth-context"
import { orgsApi } from "@/lib/api"
import { useDomainMode, baseDomainUrl } from "@/lib/domain"
import { CreateOrgDialog } from "@/components/create-org-dialog"
import type { Organization } from "@/lib/types"

function navigateToOrg(slug: string, baseDomain: string): void {
  const protocol = window.location.protocol
  const port = window.location.port ? `:${window.location.port}` : ""
  localStorage.setItem("aegis_current_org_slug", slug)
  window.location.href = `${protocol}//${slug}.${baseDomain}${port}/`
}

export default function OrgsPage() {
  const { user } = useAuth()
  const { baseDomain } = useDomainMode()
  const [orgs, setOrgs] = useState<Organization[]>([])
  const [loading, setLoading] = useState(true)
  const [createDialogOpen, setCreateDialogOpen] = useState(false)

  useEffect(() => {
    let cancelled = false
    orgsApi.list()
      .then((data) => {
        if (!cancelled) {
          setOrgs(data.orgs || [])
          setLoading(false)
        }
      })
      .catch(() => {
        if (!cancelled) setLoading(false)
      })
    return () => { cancelled = true }
  }, [])

  const handleOrgCreated = (org: Organization) => {
    if (baseDomain) {
      navigateToOrg(org.slug, baseDomain)
    } else {
      // header-only mode: just reload list
      orgsApi.list()
        .then((data) => setOrgs(data.orgs || []))
        .catch(() => {})
    }
  }

  const handleOrgClick = (org: Organization) => {
    if (baseDomain) {
      navigateToOrg(org.slug, baseDomain)
    }
  }

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Organizations</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Welcome back, {user?.name || "there"}. Select an organization to continue.
          </p>
        </div>
        <Button onClick={() => setCreateDialogOpen(true)} id="create-org-btn">
          <Plus className="mr-2 h-4 w-4" />
          New Organization
        </Button>
      </div>

      {/* Org List */}
      {loading ? (
        <div className="flex items-center justify-center py-16">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        </div>
      ) : orgs.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-16 space-y-4">
            <Building2 className="h-12 w-12 text-muted-foreground" />
            <div className="text-center space-y-1">
              <h3 className="font-semibold text-lg">No organizations yet</h3>
              <p className="text-sm text-muted-foreground">
                Create your first organization to start scanning.
              </p>
            </div>
            <Button onClick={() => setCreateDialogOpen(true)}>
              <Plus className="mr-2 h-4 w-4" />
              Create Your First Organization
            </Button>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-3">
          {orgs.map((org) => (
            <button
              key={org.id}
              onClick={() => handleOrgClick(org)}
              className="flex items-center gap-4 rounded-lg border border-border p-4 text-left transition-colors hover:bg-muted/50 group"
              id={`org-item-${org.slug}`}
            >
              <div className="flex size-10 items-center justify-center rounded-lg bg-primary text-primary-foreground shrink-0">
                <span className="text-sm font-semibold">{org.name.charAt(0).toUpperCase()}</span>
              </div>
              <div className="flex-1 min-w-0">
                <div className="font-medium">{org.name}</div>
                <div className="text-xs text-muted-foreground">
                  {baseDomain ? `${org.slug}.${baseDomain}` : org.slug}
                  <span className="ml-2 capitalize">{org.plan}</span>
                </div>
              </div>
              <ArrowRight className="h-4 w-4 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity" />
            </button>
          ))}
        </div>
      )}

      {/* Back to profile link */}
      <div className="text-center">
        <a
          href={baseDomain ? baseDomainUrl(baseDomain, "/profile") : "/profile"}
          className="text-sm text-muted-foreground hover:text-foreground transition-colors"
        >
          Manage your account →
        </a>
      </div>

      <CreateOrgDialog
        open={createDialogOpen}
        onOpenChange={setCreateDialogOpen}
        onCreated={handleOrgCreated}
      />
    </div>
  )
}
