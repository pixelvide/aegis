import { createContext, useContext, useState, useEffect, useCallback } from "react"
import type { Organization } from "@/lib/types"
import { orgsApi, setCurrentOrg } from "@/lib/api"

interface OrgContextValue {
  orgs: Organization[]
  currentOrg: Organization | null
  switchOrg: (org: Organization) => void
  loading: boolean
  refresh: () => void
  baseDomain: string
  accessDenied: boolean
}

const OrgContext = createContext<OrgContextValue>({
  orgs: [],
  currentOrg: null,
  switchOrg: () => {},
  loading: true,
  refresh: () => {},
  baseDomain: "",
  accessDenied: false,
})

export function useOrg() {
  return useContext(OrgContext)
}

const ORG_STORAGE_KEY = "aegis_current_org_slug"

export function OrgProvider({ children }: { children: React.ReactNode }) {
  const [orgs, setOrgs] = useState<Organization[]>([])
  const [currentOrg, setCurrentOrgState] = useState<Organization | null>(null)
  const [loading, setLoading] = useState(true)
  const [baseDomain, setBaseDomain] = useState("")
  const [accessDenied, setAccessDenied] = useState(false)

  const selectOrg = useCallback((orgList: Organization[], domain: string) => {
    let selected: Organization | null = null
    let onSubdomain = false

    // When base_domain is set, derive active org from the current subdomain
    if (domain) {
      const hostname = window.location.hostname
      if (hostname.endsWith(`.${domain}`)) {
        onSubdomain = true
        const subdomainSlug = hostname.slice(0, -(domain.length + 1))
        selected = orgList.find(o => o.slug === subdomainSlug) || null

        // On a subdomain but user is NOT a member of this org → access denied
        if (!selected) {
          setAccessDenied(true)
          setBaseDomain(domain)
          return
        }
      }
    }

    // Fallback: use localStorage (header-only mode / base domain without subdomain)
    // Only allowed when NOT on a subdomain
    if (!selected && !onSubdomain) {
      const savedSlug = localStorage.getItem(ORG_STORAGE_KEY)
      selected = orgList.find(o => o.slug === savedSlug) || orgList[0] || null
    }

    if (selected) {
      setCurrentOrgState(selected)
      setCurrentOrg(selected.slug, selected.id)
      localStorage.setItem(ORG_STORAGE_KEY, selected.slug)
    }

    setAccessDenied(false)
    setBaseDomain(domain)
  }, [])

  const loadOrgs = useCallback(() => {
    setLoading(true)
    orgsApi.list()
      .then((data) => {
        const orgList = data.orgs || []
        setOrgs(orgList)
        selectOrg(orgList, data.base_domain || "")
      })
      .catch(() => setOrgs([]))
      .finally(() => setLoading(false))
  }, [selectOrg])

  useEffect(() => {
    orgsApi.list()
      .then((data) => {
        const orgList = data.orgs || []
        setOrgs(orgList)
        selectOrg(orgList, data.base_domain || "")
      })
      .catch(() => setOrgs([]))
      .finally(() => setLoading(false))
  }, [selectOrg])

  const switchOrg = useCallback((org: Organization) => {
    // If base_domain is set, navigate to the org's subdomain
    if (baseDomain) {
      const currentHost = window.location.hostname
      const targetDomain = `${org.slug}.${baseDomain}`

      // Only navigate if we're on a different domain
      if (currentHost !== targetDomain) {
        const protocol = window.location.protocol
        const port = window.location.port ? `:${window.location.port}` : ""
        localStorage.setItem(ORG_STORAGE_KEY, org.slug)
        window.location.href = `${protocol}//${targetDomain}${port}/`
        return
      }
    }

    // No base_domain (dev mode) — just swap headers
    setCurrentOrgState(org)
    setCurrentOrg(org.slug, org.id)
    localStorage.setItem(ORG_STORAGE_KEY, org.slug)
  }, [baseDomain])

  return (
    <OrgContext.Provider value={{ orgs, currentOrg, switchOrg, loading, refresh: loadOrgs, baseDomain, accessDenied }}>
      {children}
    </OrgContext.Provider>
  )
}
