import { AlertOctagon, AlertTriangle, CheckCircle2, Circle, CircleDot, WifiOff } from "lucide-react"
import type { AlertSeverity, AlertStatus } from "@/api/types"
import { cn } from "@/lib/utils"

/**
 * Status pill for device connectivity. Distinguished by BOTH color and
 * icon/shape, not color alone (accessibility) — `online` is the derived
 * boolean from utils/deviceStatus.ts's isDeviceOnline(), not a field the
 * API returns directly.
 */
export function StatusPill({ online }: { online: boolean }) {
  return (
    <span
      data-testid="device-status"
      data-online={online}
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-xs font-medium",
        online ? "border-ok/40 bg-ok/10 text-ok" : "border-critical/40 bg-critical/10 text-critical",
      )}
    >
      {online ? <CircleDot className="size-3" aria-hidden /> : <WifiOff className="size-3" aria-hidden />}
      {online ? "Online" : "Offline"}
    </span>
  )
}

/**
 * Severity badge. Critical = red octagon, Warning = amber triangle —
 * shape differs so it isn't color-only.
 */
export function SeverityBadge({ severity }: { severity: AlertSeverity }) {
  const critical = severity === "critical"
  return (
    <span
      data-testid="alert-severity"
      data-severity={severity}
      className={cn(
        "inline-flex items-center gap-1.5 rounded border px-2 py-0.5 text-xs font-semibold uppercase tracking-wide",
        critical ? "border-critical/50 bg-critical/15 text-critical" : "border-warning/50 bg-warning/15 text-warning",
      )}
    >
      {critical ? <AlertOctagon className="size-3" aria-hidden /> : <AlertTriangle className="size-3" aria-hidden />}
      {severity}
    </span>
  )
}

const alertStatusMeta: Record<AlertStatus, { label: string; className: string; Icon: typeof Circle }> = {
  open: { label: "Open", className: "border-critical/40 bg-critical/10 text-critical", Icon: AlertOctagon },
  acknowledged: { label: "Acknowledged", className: "border-warning/40 bg-warning/10 text-warning", Icon: CircleDot },
  resolved: { label: "Resolved", className: "border-ok/40 bg-ok/10 text-ok", Icon: CheckCircle2 },
}

export function AlertStatusPill({ status }: { status: AlertStatus }) {
  const meta = alertStatusMeta[status]
  const { Icon } = meta
  return (
    <span data-testid="alert-status" data-status={status} className={cn("inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-xs font-medium", meta.className)}>
      <Icon className="size-3" aria-hidden />
      {meta.label}
    </span>
  )
}

/** Small colored alert-count badge for site cards. */
export function AlertCountBadge({ severity, count }: { severity: AlertSeverity; count: number }) {
  const critical = severity === "critical"
  return (
    <span
      data-testid="alert-count-badge"
      data-severity={severity}
      className={cn(
        "inline-flex items-center gap-1 rounded border px-1.5 py-0.5 text-xs font-semibold tabular-nums",
        critical ? "border-critical/50 bg-critical/15 text-critical" : "border-warning/50 bg-warning/15 text-warning",
      )}
    >
      {critical ? <AlertOctagon className="size-3" aria-hidden /> : <AlertTriangle className="size-3" aria-hidden />}
      {count}
      <span className="sr-only">{critical ? "critical" : "warning"} alerts</span>
    </span>
  )
}

export function NoAlertsBadge() {
  return (
    <span data-testid="no-alerts-badge" className="inline-flex items-center gap-1.5 rounded border border-ok/40 bg-ok/10 px-2 py-0.5 text-xs font-medium text-ok">
      <CheckCircle2 className="size-3" aria-hidden />
      No open alerts
    </span>
  )
}
