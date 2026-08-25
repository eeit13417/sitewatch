import { useState } from "react"
import { AlertTable } from "../components/AlertTable"
import { PageHeader } from "../components/primitives"
import { useAcknowledgeAlert, useResolveAlert } from "../hooks/useAlertMutations"
import { useAlerts } from "../hooks/useAlerts"
import { useDevices } from "../hooks/useDevices"
import { useSites } from "../hooks/useSites"
import type { AlertStatus } from "../api/types"

const STATUS_OPTIONS: { value: AlertStatus | ""; label: string }[] = [
  { value: "", label: "All statuses" },
  { value: "open", label: "Open" },
  { value: "acknowledged", label: "Acknowledged" },
  { value: "resolved", label: "Resolved" },
]

export function AlertsPage() {
  const [status, setStatus] = useState<AlertStatus | "">("open")
  const [siteId, setSiteId] = useState("")

  const sites = useSites()
  const devices = useDevices()
  // Filtered server-side (status/site_id query params), not fetched in
  // full and filtered in the browser — matters once alert history is
  // large (see CLAUDE.md's efficiency-at-scale rule).
  const alerts = useAlerts({ status: status || undefined, site_id: siteId || undefined })

  const [busyAlertId, setBusyAlertId] = useState<string | null>(null)
  const acknowledge = useAcknowledgeAlert()
  const resolve = useResolveAlert()

  const runMutation = (mutate: typeof acknowledge, id: string) => {
    setBusyAlertId(id)
    mutate.mutate(id, { onSettled: () => setBusyAlertId(null) })
  }

  const deviceById = new Map((devices.data ?? []).map((d) => [d.id, d]))
  const siteNameById = new Map((sites.data ?? []).map((s) => [s.id, s.name]))

  return (
    <div className="mx-auto max-w-7xl px-4 py-6 sm:px-6">
      <PageHeader title="Alerts" subtitle={alerts.data ? `${alerts.data.length} alerts` : undefined} />

      <div className="mb-4 flex flex-wrap items-center gap-3">
        <FilterSelect label="Status" value={status} onChange={(v) => setStatus(v as AlertStatus | "")} options={STATUS_OPTIONS} />
        <FilterSelect
          label="Site"
          value={siteId}
          onChange={setSiteId}
          options={[{ value: "", label: "All sites" }, ...(sites.data ?? []).map((s) => ({ value: s.id, label: s.name }))]}
        />
      </div>

      {alerts.isPending ? (
        <p className="text-sm text-muted">Loading…</p>
      ) : alerts.isError ? (
        <p className="text-sm text-critical">Failed to load alerts: {alerts.error.message}</p>
      ) : (
        <AlertTable
          alerts={alerts.data ?? []}
          busyAlertId={busyAlertId}
          onAcknowledge={(id) => runMutation(acknowledge, id)}
          onResolve={(id) => runMutation(resolve, id)}
          deviceById={deviceById}
          siteNameById={siteNameById}
          emptyMessage="No alerts match these filters."
        />
      )}
    </div>
  )
}

function FilterSelect({
  label,
  value,
  onChange,
  options,
}: {
  label: string
  value: string
  onChange: (v: string) => void
  options: { value: string; label: string }[]
}) {
  return (
    <label className="flex items-center gap-2 text-xs text-muted">
      <span className="uppercase tracking-wide text-faint">{label}</span>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="rounded border border-border-strong bg-surface-2 px-2.5 py-1.5 text-sm text-foreground focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-primary"
      >
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    </label>
  )
}
