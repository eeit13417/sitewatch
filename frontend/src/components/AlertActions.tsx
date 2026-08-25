import { Check, CheckCheck } from "lucide-react"
import type { Alert } from "../api/types"

interface AlertActionsProps {
  alert: Alert
  onAcknowledge: (id: string) => void
  onResolve: (id: string) => void
  busy: boolean
}

/**
 * Acknowledge only when status === "open"; Resolve when "open" or
 * "acknowledged" — same rule the backend enforces (api/alerts.go's
 * transitionSpec.fromStatuses), so a button never offers a transition the
 * API would 409 on.
 */
export function AlertActions({ alert, onAcknowledge, onResolve, busy }: AlertActionsProps) {
  const showAck = alert.status === "open"
  const showResolve = alert.status === "open" || alert.status === "acknowledged"

  if (!showAck && !showResolve) {
    return <span className="text-xs text-faint">—</span>
  }

  return (
    <div className="flex items-center gap-2">
      {showAck && (
        <button
          type="button"
          disabled={busy}
          onClick={() => onAcknowledge(alert.id)}
          className="inline-flex items-center gap-1 rounded border border-border-strong bg-surface-2 px-2 py-1 text-xs font-medium text-foreground transition-colors hover:border-warning/60 hover:text-warning focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-warning disabled:opacity-50"
        >
          <Check className="size-3" aria-hidden />
          Acknowledge
        </button>
      )}
      {showResolve && (
        <button
          type="button"
          disabled={busy}
          onClick={() => onResolve(alert.id)}
          className="inline-flex items-center gap-1 rounded border border-border-strong bg-surface-2 px-2 py-1 text-xs font-medium text-foreground transition-colors hover:border-ok/60 hover:text-ok focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-ok disabled:opacity-50"
        >
          <CheckCheck className="size-3" aria-hidden />
          Resolve
        </button>
      )}
    </div>
  )
}
