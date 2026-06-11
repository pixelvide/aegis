import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Bot, Eye, Zap, FlaskConical, Play, Circle } from "lucide-react"

const personas = [
  {
    id: "sharingan",
    name: "Sharingan",
    icon: Eye,
    description: "Deep vulnerability analysis with thorough code review. Covers OWASP Top 10, CWE patterns, and custom rules.",
    tags: ["Deep Analysis", "OWASP", "CWE"],
    color: "text-red-500",
  },
  {
    id: "senku",
    name: "Senku",
    icon: FlaskConical,
    description: "Scientific approach to security testing. Methodical exploitation and proof-of-concept generation.",
    tags: ["PoC Generation", "Exploit Chains", "Methodical"],
    color: "text-green-500",
  },
  {
    id: "killua",
    name: "Killua",
    icon: Zap,
    description: "Speed-optimized scanning for quick security checks. Ideal for CI/CD pipelines and rapid assessments.",
    tags: ["Fast", "CI/CD", "Surface Scan"],
    color: "text-blue-500",
  },
]

export default function AgentsPage() {
  return (
    <div className="flex flex-col gap-4 md:gap-6">
      {/* Persona Cards */}
      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        {personas.map((p) => {
          const Icon = p.icon
          return (
            <Card key={p.id}>
              <CardHeader className="pb-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2.5">
                    <div className={`${p.color}`}>
                      <Icon className="h-5 w-5" />
                    </div>
                    <CardTitle className="text-sm font-medium">{p.name}</CardTitle>
                  </div>
                  <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                    <Circle className="h-2 w-2 fill-emerald-500 text-emerald-500" />
                    Available
                  </div>
                </div>
              </CardHeader>
              <CardContent className="space-y-3">
                <CardDescription className="text-xs leading-relaxed">{p.description}</CardDescription>
                <div className="flex flex-wrap gap-1.5">
                  {p.tags.map((tag) => (
                    <Badge key={tag} variant="secondary" className="text-[10px] px-1.5 py-0">
                      {tag}
                    </Badge>
                  ))}
                </div>
                <Button size="sm" variant="outline" className="w-full gap-2 text-xs">
                  <Play className="h-3 w-3" />
                  Run Scan
                </Button>
              </CardContent>
            </Card>
          )
        })}
      </div>

      {/* Active Agents */}
      <Card>
        <CardHeader className="pb-4">
          <CardTitle className="text-sm font-medium">Active Agents</CardTitle>
          <CardDescription>Running agent processes</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="text-center py-10 text-muted-foreground">
            <Bot className="h-8 w-8 mx-auto mb-3 opacity-40" />
            <p className="text-sm font-medium">No active agents</p>
            <p className="text-xs mt-1">Agents will appear here when a scan is running</p>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
