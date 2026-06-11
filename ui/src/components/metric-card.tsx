import { Card } from "@/components/ui/card"
import { cn } from "@/lib/utils"
import { TrendingUp, TrendingDown } from "lucide-react"

interface MetricCardProps {
  title: string
  value: number | string
  trend?: number
  description?: string
  trendText?: string
  className?: string
}

export function MetricCard({ title, value, trend, description, trendText, className }: MetricCardProps) {
  const trendPositive = trend !== undefined && trend > 0
  const trendNegative = trend !== undefined && trend < 0

  return (
    <Card className={cn("flex flex-col gap-6 py-6", className)}>
      {/* Header: label + badge */}
      <div className="grid auto-rows-min grid-rows-[auto_auto] items-start gap-2 px-6 grid-cols-[1fr_auto]">
        <p className="text-sm text-muted-foreground">{title}</p>
        <p className="text-2xl font-semibold tabular-nums">{typeof value === "number" ? value.toLocaleString() : value}</p>

        {/* Trend badge — top right */}
        {trend !== undefined && (
          <span
            className={cn(
              "col-start-2 row-span-2 row-start-1 self-start justify-self-end",
              "inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs font-medium",
              trendPositive && "text-foreground",
              trendNegative && "text-foreground",
            )}
          >
            {trendPositive && <TrendingUp className="h-3 w-3" />}
            {trendNegative && <TrendingDown className="h-3 w-3" />}
            {trend > 0 ? "+" : ""}{trend}%
          </span>
        )}
      </div>

      {/* Footer: trend description */}
      {(trendText || description) && (
        <div className="flex px-6 flex-col items-start gap-1 text-sm">
          {trendText && (
            <div className="flex items-center gap-2 font-medium">
              {trendText}
              {trendPositive && <TrendingUp className="h-4 w-4" />}
              {trendNegative && <TrendingDown className="h-4 w-4" />}
            </div>
          )}
          {description && (
            <p className="text-muted-foreground">{description}</p>
          )}
        </div>
      )}
    </Card>
  )
}
