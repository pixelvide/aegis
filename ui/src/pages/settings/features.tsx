import { useEffect, useState, useCallback } from "react"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Switch } from "@/components/ui/switch"
import { ToggleLeft, Lock, Info } from "lucide-react"
import { orgFeaturesApi, membersApi } from "@/lib/api"
import type { OrgFeatureFlag } from "@/lib/api"
import { useAuth } from "@/lib/auth-context"

/** Human-readable labels for feature flag names. */
const flagLabels: Record<string, { label: string; description: string }> = {
  org_wide_tokens: {
    label: "Org-Wide API Tokens",
    description: "Allow creating API tokens that have access to all projects in this organization.",
  },
}

export default function FeaturesPage() {
  const { user } = useAuth()
  const [flags, setFlags] = useState<OrgFeatureFlag[]>([])
  const [loading, setLoading] = useState(true)
  const [updating, setUpdating] = useState<string | null>(null)
  const [error, setError] = useState("")
  const [isOwner, setIsOwner] = useState(false)

  const loadFlags = useCallback(() => {
    orgFeaturesApi
      .list()
      .then((data) => { setFlags(data || []); setLoading(false) })
      .catch(() => { setFlags([]); setLoading(false) })

    // Also check if the current user is the org owner
    if (user) {
      membersApi.list().then((members) => {
        const me = members.find((m) => m.user_id === user.id)
        setIsOwner(me?.role === "owner")
      }).catch(() => {})
    }
  }, [user])

  useEffect(() => { loadFlags() }, [loadFlags])

  const handleToggle = async (name: string, enabled: boolean) => {
    setUpdating(name)
    setError("")
    try {
      await orgFeaturesApi.update(name, enabled)
      setFlags((prev) =>
        prev.map((f) => (f.name === name ? { ...f, enabled } : f))
      )
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to update feature")
    } finally {
      setUpdating(null)
    }
  }

  const getFlagInfo = (name: string) => {
    return flagLabels[name] || {
      label: name.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase()),
      description: "",
    }
  }

  return (
    <div className="flex flex-col gap-4 md:gap-6">
      <div>
        <h1 className="text-lg font-semibold">Features</h1>
        <p className="text-sm text-muted-foreground">Manage feature availability for your organization</p>
      </div>

      {!isOwner && (
        <div className="flex items-start gap-2 rounded-md bg-amber-500/10 border border-amber-500/20 p-3">
          <Info className="h-4 w-4 text-amber-500 mt-0.5 shrink-0" />
          <p className="text-sm text-amber-700 dark:text-amber-400">
            Only the organization owner can enable or disable features.
          </p>
        </div>
      )}

      {error && <p className="text-sm text-destructive">{error}</p>}

      <Card>
        <CardHeader className="pb-4">
          <div className="flex items-center gap-2">
            <ToggleLeft className="h-4 w-4 text-muted-foreground" />
            <CardTitle className="text-sm font-medium">Organization Features</CardTitle>
          </div>
          <CardDescription>
            Features must be provisioned by the platform before they can be enabled. Provisioned features can be toggled by the organization owner.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="space-y-4">
              {[1, 2, 3].map((i) => (
                <div key={i} className="flex items-center justify-between py-3">
                  <div className="space-y-1.5">
                    <div className="h-4 bg-muted rounded w-40 animate-pulse" />
                    <div className="h-3 bg-muted rounded w-64 animate-pulse" />
                  </div>
                  <div className="h-5 w-9 bg-muted rounded-full animate-pulse" />
                </div>
              ))}
            </div>
          ) : flags.length === 0 ? (
            <div className="text-center py-10 text-muted-foreground">
              <ToggleLeft className="h-8 w-8 mx-auto mb-3 opacity-40" />
              <p className="text-sm font-medium">No feature flags configured</p>
              <p className="text-xs mt-1">Feature flags will appear here once provisioned by the platform.</p>
            </div>
          ) : (
            <div className="divide-y">
              {flags.map((flag) => {
                const info = getFlagInfo(flag.name)
                const canToggle = flag.provisioned && isOwner
                return (
                  <div key={flag.name} className="flex items-center justify-between py-4 first:pt-0 last:pb-0">
                    <div className="space-y-1 flex-1 mr-4">
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-medium">{info.label}</span>
                        {!flag.provisioned ? (
                          <Badge variant="outline" className="text-[10px] px-1.5 py-0 text-zinc-500 border-zinc-300 gap-1">
                            <Lock className="h-2.5 w-2.5" />
                            Not available
                          </Badge>
                        ) : flag.enabled ? (
                          <Badge variant="outline" className="text-[10px] px-1.5 py-0 text-emerald-600 border-emerald-300">
                            Enabled
                          </Badge>
                        ) : (
                          <Badge variant="outline" className="text-[10px] px-1.5 py-0 text-zinc-500 border-zinc-300">
                            Disabled
                          </Badge>
                        )}
                      </div>
                      {info.description && (
                        <p className="text-xs text-muted-foreground">{info.description}</p>
                      )}
                      {flag.description && !info.description && (
                        <p className="text-xs text-muted-foreground">{flag.description}</p>
                      )}
                    </div>
                    <Switch
                      checked={flag.enabled}
                      onCheckedChange={(checked) => handleToggle(flag.name, checked)}
                      disabled={!canToggle || updating === flag.name}
                      id={`feature-toggle-${flag.name}`}
                    />
                  </div>
                )
              })}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
