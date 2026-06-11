import { useEffect, useState } from "react"
import { useParams, Link } from "react-router-dom"
import { SeverityBadge, FindingStatusBadge } from "@/components/severity-badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
import { findingsApi } from "@/lib/api"
import { formatDate } from "@/lib/utils"
import type { Finding, FindingStatus } from "@/lib/types"
import {
  ArrowLeft, FileCode2, Copy, Check
} from "lucide-react"

export default function FindingDetailPage() {
  const { id } = useParams<{ id: string }>()
  const [finding, setFinding] = useState<Finding | null>(null)
  const [loading, setLoading] = useState(true)
  const [activeExploit, setActiveExploit] = useState(0)
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    if (!id) return
    findingsApi.get(id)
      .then(setFinding)
      .catch(() => setFinding(null))
      .finally(() => setLoading(false))
  }, [id])

  const handleStatusChange = async (status: FindingStatus) => {
    if (!finding) return
    const updated = await findingsApi.updateStatus(finding.id, status)
    setFinding(updated)
  }

  const handleCopy = async (code: string) => {
    await navigator.clipboard.writeText(code)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  if (loading) {
    return (
      <div className="space-y-4">
        <div className="h-8 bg-muted rounded w-48 animate-pulse" />
        <div className="h-64 bg-muted rounded animate-pulse" />
      </div>
    )
  }

  if (!finding) {
    return (
      <div className="text-center py-20">
        <p className="text-lg font-medium">Finding not found</p>
        <Link to="/findings" className="text-blue-600 hover:underline text-sm mt-2 inline-block">
          Back to findings
        </Link>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-start gap-4">
        <Link to="/findings" className="mt-1">
          <ArrowLeft className="h-5 w-5 text-muted-foreground hover:text-foreground transition-colors" />
        </Link>
        <div className="flex-1">
          <div className="flex items-center gap-3 flex-wrap">
            <h1 className="text-2xl font-bold tracking-tight">{finding.title}</h1>
            <SeverityBadge severity={finding.severity} />
            <FindingStatusBadge status={finding.status} />
          </div>
          <p className="text-sm text-muted-foreground mt-1 font-mono">{finding.id}</p>
        </div>
      </div>

      <div className="grid gap-6 lg:grid-cols-3">
        {/* Main Content — Report */}
        <div className="lg:col-span-2 space-y-6">
          {/* Description */}
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Report</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="prose prose-sm dark:prose-invert max-w-none whitespace-pre-wrap">
                {finding.description || "No description available."}
              </div>
            </CardContent>
          </Card>

          {/* Exploit Code */}
          {finding.exploits && finding.exploits.length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle className="text-base flex items-center gap-2">
                  <FileCode2 className="h-4 w-4" />
                  Exploit Code
                </CardTitle>
              </CardHeader>
              <CardContent>
                {/* Exploit Tabs */}
                <div className="flex gap-1 mb-3 border-b">
                  {finding.exploits.map((e, i) => (
                    <button
                      key={e.id}
                      onClick={() => setActiveExploit(i)}
                      className={`px-3 py-2 text-sm font-medium transition-colors border-b-2 -mb-px ${
                        i === activeExploit
                          ? "border-primary text-foreground"
                          : "border-transparent text-muted-foreground hover:text-foreground"
                      }`}
                    >
                      {e.filename}
                    </button>
                  ))}
                </div>

                {/* Code Block — always dark */}
                <div className="relative rounded-lg bg-[#1e1e1e] text-[#d4d4d4] overflow-x-auto">
                  <button
                    onClick={() => handleCopy(finding.exploits![activeExploit].code)}
                    className="absolute top-3 right-3 p-1.5 rounded bg-white/10 hover:bg-white/20 transition-colors"
                    title="Copy to clipboard"
                    id="copy-exploit-button"
                  >
                    {copied ? (
                      <Check className="h-4 w-4 text-green-400" />
                    ) : (
                      <Copy className="h-4 w-4" />
                    )}
                  </button>
                  <pre className="p-4 text-sm font-mono leading-relaxed overflow-x-auto">
                    <code>{finding.exploits[activeExploit].code}</code>
                  </pre>
                </div>

                {finding.exploits[activeExploit].validated && (
                  <Badge variant="success" className="mt-3">✓ Validated against target</Badge>
                )}
              </CardContent>
            </Card>
          )}
        </div>

        {/* Sidebar — Info Panel */}
        <div className="space-y-4">
          {/* Metadata */}
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Details</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3 text-sm">
              <div className="flex justify-between">
                <span className="text-muted-foreground">Severity</span>
                <SeverityBadge severity={finding.severity} />
              </div>
              <Separator />
              <div className="flex justify-between">
                <span className="text-muted-foreground">Status</span>
                <FindingStatusBadge status={finding.status} />
              </div>
              <Separator />
              {finding.cwe && (
                <>
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">CWE</span>
                    <span className="font-mono text-xs">{finding.cwe}</span>
                  </div>
                  <Separator />
                </>
              )}
              {finding.owasp && (
                <>
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">OWASP</span>
                    <span className="font-mono text-xs">{finding.owasp}</span>
                  </div>
                  <Separator />
                </>
              )}
              {finding.file && (
                <>
                  <div>
                    <span className="text-muted-foreground block mb-1">File</span>
                    <span className="font-mono text-xs break-all">{finding.file}</span>
                    {finding.line ? (
                      <span className="text-muted-foreground"> :L{finding.line}</span>
                    ) : null}
                  </div>
                  <Separator />
                </>
              )}
              <div className="flex justify-between">
                <span className="text-muted-foreground">Created</span>
                <span className="text-xs">{formatDate(finding.created_at)}</span>
              </div>
            </CardContent>
          </Card>

          {/* Status Actions */}
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Actions</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2">
              {finding.status !== "confirmed" && (
                <Button
                  variant="outline"
                  size="sm"
                  className="w-full justify-start"
                  onClick={() => handleStatusChange("confirmed")}
                  id="confirm-finding-button"
                >
                  Confirm Vulnerability
                </Button>
              )}
              {finding.status !== "fixed" && (
                <Button
                  variant="outline"
                  size="sm"
                  className="w-full justify-start text-green-600"
                  onClick={() => handleStatusChange("fixed")}
                  id="mark-fixed-button"
                >
                  Mark as Fixed
                </Button>
              )}
              {finding.status !== "false_positive" && (
                <Button
                  variant="outline"
                  size="sm"
                  className="w-full justify-start"
                  onClick={() => handleStatusChange("false_positive")}
                  id="false-positive-button"
                >
                  False Positive
                </Button>
              )}
              {finding.status !== "wontfix" && (
                <Button
                  variant="ghost"
                  size="sm"
                  className="w-full justify-start text-muted-foreground"
                  onClick={() => handleStatusChange("wontfix")}
                >
                  Won't Fix
                </Button>
              )}
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  )
}
