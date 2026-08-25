import { useState } from "react"
import { Link, useParams } from "react-router"
import { AlertTable } from "../components/AlertTable"
import { StatusPill } from "../components/indicators"
import { PageHeader, Panel } from "../components/primitives"
import { ChartLegend, TelemetryChart } from "../components/TelemetryChart"
import { useAcknowledgeAlert, useResolveAlert } from "../hooks/useAlertMutations"
import { useAlerts } from "../hooks/useAlerts"
import { useDevice } from "../hooks/useDevices"
import { useDeviceTelemetry } from "../hooks/useDeviceTelemetry"
import { useSites } from "../hooks/useSites"
import { deviceTypeLabels, extractMetrics, formatRelative } from "@/lib/utils"
import { isDeviceOnline } from "../utils/deviceStatus"
import { NotFound } from "../components/NotFound"

// This panel is "recent context for this device", not a full audit log —
// a busy device can accumulate a lot of resolved-alert history (caught
// during Phase 3 QA: an unbounded query here rendered 30+ historical rows
// with no pagination). The full, filterable, paginated history lives on
// the Alerts page.
const RECENT_ALERTS_LIMIT = 10

export function DeviceDetail() {
  const { deviceId = "" } = useParams<{ deviceId: string }>()
  const device = useDevice(deviceId)
  const telemetry = useDeviceTelemetry(deviceId)
  const alerts = useAlerts({ device_id: deviceId, limit: RECENT_ALERTS_LIMIT })
  const sites = useSites()

  const [busyAlertId, setBusyAlertId] = useState<string | null>(null)
  const acknowledge = useAcknowledgeAlert()
  const resolve = useResolveAlert()

  const runMutation = (mutate: typeof acknowledge, id: string) => {
    setBusyAlertId(id)
    mutate.mutate(id, { onSettled: () => setBusyAlertId(null) })
  }

  if (device.isPending) return <p className="p-6 text-sm text-muted">Loading…</p>
  if (device.isError) {
    if (device.error.status === 404) return <NotFound message="Device not found." />
    return <p className="p-6 text-sm text-critical">Failed to load device: {device.error.message}</p>
  }

  const points = telemetry.data ?? []
  const metrics = extractMetrics(points)
  const site = sites.data?.find((s) => s.id === device.data.site_id)

  return (
    <div className="mx-auto max-w-7xl px-4 py-6 sm:px-6">
      <PageHeader
        breadcrumb={
          <span>
            <Link to="/" className="hover:text-muted">
              Sites
            </Link>
            {" / "}
            <Link to={`/sites/${device.data.site_id}`} className="hover:text-muted">
              {site?.name ?? device.data.site_id}
            </Link>
            {" / "}
            <span className="text-muted">{device.data.name}</span>
          </span>
        }
        title={
          <span className="flex items-center gap-3">
            {device.data.name}
            <StatusPill online={isDeviceOnline(device.data.last_seen_at)} />
          </span>
        }
        subtitle={`${deviceTypeLabels[device.data.type]} · last seen ${device.data.last_seen_at ? formatRelative(device.data.last_seen_at) : "never"}`}
      />

      <Panel className="p-4">
        <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
          <h2 className="text-sm font-semibold text-foreground">Telemetry</h2>
          <ChartLegend metrics={metrics} />
        </div>
        {telemetry.isPending ? <p className="py-10 text-center text-sm text-muted">Loading…</p> : <TelemetryChart points={points} />}
      </Panel>

      <div className="mt-6">
        <h2 className="mb-3 text-sm font-semibold text-foreground">
          Recent alerts
          <span className="ml-2 text-xs font-normal text-faint">(most recent {RECENT_ALERTS_LIMIT})</span>
        </h2>
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
            emptyMessage="No alerts for this device."
          />
        )}
      </div>
    </div>
  )
}
