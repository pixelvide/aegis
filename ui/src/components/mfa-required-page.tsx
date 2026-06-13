import { baseDomainUrl } from "@/lib/domain"
import { ShieldAlert } from "lucide-react"

interface MfaRequiredPageProps {
  baseDomain: string
}

/**
 * Full-page interstitial shown when the org has `require_mfa` enabled
 * but the current user hasn't set up MFA yet.
 *
 * Directs the user to the MFA setup page on the base domain.
 */
export function MfaRequiredPage({ baseDomain }: MfaRequiredPageProps) {
  const mfaSetupUrl = baseDomainUrl(baseDomain, "/profile/access-management/authentication")
  const orgsUrl = baseDomainUrl(baseDomain, "/orgs")

  return (
    <div className="flex min-h-screen items-center justify-center">
      <div className="text-center space-y-4 max-w-md px-4">
        <div className="flex justify-center">
          <ShieldAlert className="h-12 w-12 text-amber-500" />
        </div>
        <h1 className="text-xl font-semibold">Multi-Factor Authentication Required</h1>
        <p className="text-sm text-muted-foreground">
          This organization requires all members to enable multi-factor authentication.
          Set up MFA on your account to continue.
        </p>
        <div className="flex flex-col gap-2">
          <a
            href={mfaSetupUrl}
            className="inline-flex items-center justify-center rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 transition-colors"
          >
            Set Up MFA
          </a>
          <a
            href={orgsUrl}
            className="text-sm text-muted-foreground hover:text-foreground transition-colors"
          >
            Go to My Organizations
          </a>
        </div>
      </div>
    </div>
  )
}
