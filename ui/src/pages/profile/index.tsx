import { useAuth } from "@/lib/auth-context"
import { Card, CardContent } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Mail, ShieldCheck, ShieldOff } from "lucide-react"

export default function ProfileOverviewPage() {
  const { user } = useAuth()

  return (
    <div className="space-y-6">
      {/* Page header */}
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Profile</h1>
        <p className="text-sm text-muted-foreground">
          Your personal account information.
        </p>
      </div>

      {/* User profile card */}
      <Card>
        <CardContent className="flex items-center gap-6 pt-6">
          {/* Avatar */}
          <div className="flex h-20 w-20 shrink-0 items-center justify-center rounded-full bg-muted text-2xl font-semibold text-muted-foreground uppercase">
            {user?.avatar_url ? (
              <img
                src={user.avatar_url}
                alt={user.name}
                className="h-20 w-20 rounded-full object-cover"
              />
            ) : (
              user?.name?.charAt(0) || "?"
            )}
          </div>

          {/* Info */}
          <div className="space-y-2">
            <div>
              <h2 className="text-xl font-semibold">{user?.name || "User"}</h2>
              <div className="flex items-center gap-2 text-sm text-muted-foreground">
                <Mail className="h-3.5 w-3.5" />
                {user?.email}
              </div>
            </div>
            <div className="flex items-center gap-2">
              {user?.mfa_enabled ? (
                <Badge variant="secondary" className="text-xs text-green-600">
                  <ShieldCheck className="mr-1 h-3 w-3" />
                  MFA Enabled
                </Badge>
              ) : (
                <Badge variant="outline" className="text-xs text-muted-foreground">
                  <ShieldOff className="mr-1 h-3 w-3" />
                  MFA Disabled
                </Badge>
              )}
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
