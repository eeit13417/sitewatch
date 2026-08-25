import { Link } from "react-router"
import type { Alert, Device } from "../api/types"
import { formatRelative, formatTimestamp } from "@/lib/utils"
import { AlertActions } from "./AlertActions"
import { AlertStatusPill, SeverityBadge } from "./indicators"
import { EmptyRow, Panel } from "./primitives"

interface AlertTableProps {
  alerts: Alert[]
  onAcknowledge: (id: string) => void
  onResolve: (id: string) => void
  busyAlertId: string | null
  /**
   * Only passed on the all-sites Alerts page. A device's own page omits
   * these — the device is already the page's whole context, repeating it
   * per row would be noise, not information.
   */
  deviceById?: Map<string, Device>
  siteNameById?: Map<string, string>
  emptyMessage?: string
}

export function AlertTable({
  alerts,
  onAcknowledge,
  onResolve,
  busyAlertId,
  deviceById,
  siteNameById,
  emptyMessage = "No alerts.",
}: AlertTableProps) {
  const showDeviceColumn = deviceById !== undefined

  return (
    <Panel className="overflow-hidden">
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-left text-xs uppercase tracking-wide text-faint">
              <th className="px-4 py-3 font-medium">Severity</th>
              <th className="px-4 py-3 font-medium">Status</th>
              {showDeviceColumn && <th className="px-4 py-3 font-medium">Device</th>}
              <th className="px-4 py-3 font-medium">Value</th>
              <th className="px-4 py-3 font-medium">Triggered</th>
              <th className="px-4 py-3 font-medium text-right">Actions</th>
            </tr>
          </thead>
          <tbody>
            {alerts.length === 0 && <EmptyRow colSpan={showDeviceColumn ? 6 : 5}>{emptyMessage}</EmptyRow>}
            {alerts.map((alert) => {
              const device = deviceById?.get(alert.device_id)
              const siteName = device ? siteNameById?.get(device.site_id) : undefined
              return (
                <tr key={alert.id} data-testid="alert-row" className="border-b border-border/60 last:border-0 hover:bg-surface-2/60">
                  <td className="px-4 py-3">
                    <SeverityBadge severity={alert.severity} />
                  </td>
                  <td className="px-4 py-3">
                    <AlertStatusPill status={alert.status} />
                  </td>
                  {showDeviceColumn && (
                    <td className="px-4 py-3">
                      {device ? (
                        <Link to={`/devices/${device.id}`} className="font-medium text-primary hover:underline">
                          {device.name}
                        </Link>
                      ) : (
                        <span className="text-faint">unknown</span>
                      )}
                      {siteName && <span className="ml-2 text-xs text-faint">{siteName}</span>}
                    </td>
                  )}
                  <td className="px-4 py-3 font-mono text-xs text-foreground">{alert.triggered_value}</td>
                  <td className="px-4 py-3 text-muted">
                    <span className="text-foreground">{formatRelative(alert.triggered_at)}</span>
                    <span className="ml-2 text-xs text-faint">{formatTimestamp(alert.triggered_at)}</span>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex justify-end">
                      <AlertActions alert={alert} onAcknowledge={onAcknowledge} onResolve={onResolve} busy={busyAlertId === alert.id} />
                    </div>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </Panel>
  )
}
