import { useState, useRef } from "react"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { Eye, EyeOff, Loader2 } from "lucide-react"

interface PasswordModalProps {
  open: boolean
  onConfirm: (password: string) => void
  onCancel: () => void
  title?: string
  description?: string
  loading?: boolean
  error?: string
}

export function PasswordModal({
  open,
  onConfirm,
  onCancel,
  title = "Confirm Password",
  description = "Enter your password to continue.",
  loading = false,
  error,
}: PasswordModalProps) {
  const [password, setPassword] = useState("")
  const [showPassword, setShowPassword] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)

  const handleOpenChange = (isOpen: boolean) => {
    if (!isOpen) {
      onCancel()
    } else {
      // Reset state when opening
      setPassword("")
      setShowPassword(false)
    }
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (password.trim()) {
      onConfirm(password)
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent
        className="sm:max-w-[400px]"
        onOpenAutoFocus={(e) => {
          e.preventDefault()
          setTimeout(() => inputRef.current?.focus(), 50)
        }}
      >
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{title}</DialogTitle>
            <DialogDescription>{description}</DialogDescription>
          </DialogHeader>

          <div className="py-4 space-y-3">
            <div className="relative">
              <Input
                ref={inputRef}
                type={showPassword ? "text" : "password"}
                placeholder="Password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                disabled={loading}
                autoComplete="current-password"
                className="pr-10"
              />
              <button
                type="button"
                onClick={() => setShowPassword(!showPassword)}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
                tabIndex={-1}
              >
                {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
              </button>
            </div>

            {error && (
              <p className="text-sm text-red-600">{error}</p>
            )}
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={onCancel} disabled={loading}>
              Cancel
            </Button>
            <Button type="submit" disabled={loading || !password.trim()}>
              {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              Confirm
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
