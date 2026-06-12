import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
import {
  Save, Server, Shield, Database, Globe,
} from "lucide-react"

export default function SettingsPage() {
  return (
    <div className="flex flex-col gap-4 md:gap-6">
      <div>
        <h1 className="text-lg font-semibold">Settings</h1>
        <p className="text-sm text-muted-foreground">General organization settings</p>
      </div>

      <div className="grid gap-4 md:gap-6 lg:grid-cols-2">
        {/* Agent Configuration */}
        <Card>
          <CardHeader className="pb-4">
            <div className="flex items-center gap-2">
              <Shield className="h-4 w-4 text-muted-foreground" />
              <CardTitle className="text-sm font-medium">Agent Configuration</CardTitle>
            </div>
            <CardDescription>Configure how agents are launched</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div>
              <label className="text-sm font-medium block mb-1.5">Agent Binary Path</label>
              <Input type="text" defaultValue="aegis" id="agent-binary-input" />
              <p className="text-xs text-muted-foreground mt-1.5">
                Path to the aegis binary or command name in PATH
              </p>
            </div>
            <div>
              <label className="text-sm font-medium block mb-1.5">Docker Image</label>
              <Input type="text" defaultValue="aegis-security:latest" id="docker-image-input" />
              <p className="text-xs text-muted-foreground mt-1.5">Image used for sandboxed scans</p>
            </div>
            <div>
              <label className="text-sm font-medium block mb-1.5">Default Persona</label>
              <select
                className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-xs transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                id="default-persona-select"
              >
                <option value="sharingan">Sharingan — Deep analysis</option>
                <option value="senku">Senku — Scientific approach</option>
                <option value="killua">Killua — Speed-focused</option>
              </select>
            </div>
            <Separator />
            <Button size="sm" className="gap-2">
              <Save className="h-3.5 w-3.5" />
              Save Configuration
            </Button>
          </CardContent>
        </Card>

        {/* Server Info */}
        <Card>
          <CardHeader className="pb-4">
            <div className="flex items-center gap-2">
              <Server className="h-4 w-4 text-muted-foreground" />
              <CardTitle className="text-sm font-medium">Server</CardTitle>
            </div>
            <CardDescription>Server and environment information</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex items-center justify-between py-1">
              <div className="flex items-center gap-2 text-sm text-muted-foreground">
                <Globe className="h-3.5 w-3.5" />
                API Endpoint
              </div>
              <Badge variant="secondary" className="font-mono text-xs">
                {window.location.host}
              </Badge>
            </div>
            <Separator />
            <div className="flex items-center justify-between py-1">
              <div className="flex items-center gap-2 text-sm text-muted-foreground">
                <Database className="h-3.5 w-3.5" />
                Database
              </div>
              <Badge variant="secondary" className="font-mono text-xs">PostgreSQL 16</Badge>
            </div>
            <Separator />
            <div className="flex items-center justify-between py-1">
              <div className="flex items-center gap-2 text-sm text-muted-foreground">
                <Shield className="h-3.5 w-3.5" />
                Version
              </div>
              <Badge variant="secondary" className="font-mono text-xs">0.1.0</Badge>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
