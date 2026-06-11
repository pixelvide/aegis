import { createContext, useContext, useState, useEffect, useCallback } from "react"
import type { Organization } from "@/lib/types"
import { orgsApi, setCurrentOrg } from "@/lib/api"

interface OrgContextValue {
  orgs: Organization[]
  currentOrg: Organization | null
  switchOrg: (org: Organization) => void
  loading: boolean
  refresh: () => void
}

const OrgContext = createContext<OrgContextValue>({
  orgs: [],
  currentOrg: null,
  switchOrg: () => {},
  loading: true,
  refresh: () => {},
})

export function useOrg() {
  return useContext(OrgContext)
}

const ORG_STORAGE_KEY = "aegis_current_org_slug"

export function OrgProvider({ children }: { children: React.ReactNode }) {
  const [orgs, setOrgs] = useState<Organization[]>([])
  const [currentOrg, setCurrentOrgState] = useState<Organization | null>(null)
  const [loading, setLoading] = useState(true)

  const loadOrgs = useCallback(() => {
    setLoading(true)
    orgsApi.list()
      .then((data) => {
        const orgList = data || []
        setOrgs(orgList)

        // Restore last selected org from localStorage
        const savedSlug = localStorage.getItem(ORG_STORAGE_KEY)
        const saved = orgList.find(o => o.slug === savedSlug)
        const selected = saved || orgList[0] || null

        if (selected) {
          setCurrentOrgState(selected)
          setCurrentOrg(selected.slug, selected.id)
        }
      })
      .catch(() => setOrgs([]))
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => {
    orgsApi.list()
      .then((data) => {
        const orgList = data || []
        setOrgs(orgList)

        const savedSlug = localStorage.getItem(ORG_STORAGE_KEY)
        const saved = orgList.find(o => o.slug === savedSlug)
        const selected = saved || orgList[0] || null

        if (selected) {
          setCurrentOrgState(selected)
          setCurrentOrg(selected.slug, selected.id)
        }
      })
      .catch(() => setOrgs([]))
      .finally(() => setLoading(false))
  }, [])

  const switchOrg = useCallback((org: Organization) => {
    setCurrentOrgState(org)
    setCurrentOrg(org.slug, org.id)
    localStorage.setItem(ORG_STORAGE_KEY, org.slug)
  }, [])

  return (
    <OrgContext.Provider value={{ orgs, currentOrg, switchOrg, loading, refresh: loadOrgs }}>
      {children}
    </OrgContext.Provider>
  )
}
