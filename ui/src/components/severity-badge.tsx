import { Badge } from "@/components/ui/badge"
import type { Severity, FindingStatus, ScanStatus } from "@/lib/types"

const severityMap: Record<Severity, { label: string; variant: "critical" | "high" | "medium" | "low" | "info" }> = {
  critical: { label: "Critical", variant: "critical" },
  high: { label: "High", variant: "high" },
  medium: { label: "Medium", variant: "medium" },
  low: { label: "Low", variant: "low" },
  info: { label: "Info", variant: "info" },
}

export function SeverityBadge({ severity }: { severity: Severity }) {
  const config = severityMap[severity] || severityMap.info
  return <Badge variant={config.variant}>{config.label}</Badge>
}

const statusMap: Record<FindingStatus, { label: string; variant: "default" | "success" | "secondary" | "outline" }> = {
  open: { label: "Open", variant: "outline" },
  confirmed: { label: "Confirmed", variant: "default" },
  fixed: { label: "Fixed", variant: "success" },
  false_positive: { label: "False Positive", variant: "secondary" },
  wontfix: { label: "Won't Fix", variant: "secondary" },
  verified: { label: "Verified", variant: "success" },
}

export function FindingStatusBadge({ status }: { status: FindingStatus }) {
  const config = statusMap[status] || statusMap.open
  return <Badge variant={config.variant}>{config.label}</Badge>
}

const scanStatusMap: Record<ScanStatus, { label: string; className: string }> = {
  pending: { label: "Pending", className: "bg-gray-500/15 text-gray-600 dark:text-gray-400" },
  running: { label: "Running", className: "bg-blue-500/15 text-blue-700 dark:text-blue-400 animate-pulse" },
  completed: { label: "Completed", className: "bg-green-500/15 text-green-700 dark:text-green-400" },
  failed: { label: "Failed", className: "bg-red-500/15 text-red-700 dark:text-red-400" },
  cancelled: { label: "Cancelled", className: "bg-gray-500/15 text-gray-600 dark:text-gray-400" },
}

export function ScanStatusBadge({ status }: { status: ScanStatus }) {
  const config = scanStatusMap[status] || scanStatusMap.pending
  return (
    <span className={`inline-flex items-center gap-1.5 rounded-full border border-transparent px-2.5 py-0.5 text-xs font-semibold ${config.className}`}>
      {status === "running" && (
        <span className="h-1.5 w-1.5 rounded-full bg-blue-500 animate-pulse" />
      )}
      {config.label}
    </span>
  )
}
