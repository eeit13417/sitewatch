import { Link, useParams } from "react-router"
import { StatusPill } from "../components/indicators"
import { EmptyRow, PageHeader, Panel } from "../components/primitives"
import { useDevices } from "../hooks/useDevices"
import { useSites } from "../hooks/useSites"
import { deviceTypeLabels, formatRelative, formatTimestamp, siteTypeLabels } from "@/lib/utils"
import { isDeviceOnline } from "../utils/deviceStatus"
import { NotFound } from "../components/NotFound"

export function SiteDevices() {
  const { siteId = "" } = useParams<{ siteId: string }>()
  // No GET /sites/{id} endpoint exists (sites are only ever listed, not
  // fetched individually — see docs/openapi.yaml) — the list is 2 rows,
  // filtering client-side is simpler than adding an endpoint for it.
  const sites = useSites()
  const devices = useDevices(siteId)

  if (devices.isPending || sites.isPending) return <p className="p-6 text-sm text-muted">Loading…</p>
  if (devices.isError) return <p className="p-6 text-sm text-critical">Failed to load devices: {devices.error.message}</p>

  const site = sites.data?.find((s) => s.id === siteId)
  if (!site) return <NotFound message="Site not found." />

  const deviceList = devices.data ?? []
  const online = deviceList.filter((d) => isDeviceOnline(d.last_seen_at)).length

  return (
    <div className="mx-auto max-w-7xl px-4 py-6 sm:px-6">
      <PageHeader
        breadcrumb={
          <span>
            <Link to="/" className="hover:text-muted">
              Sites
            </Link>
            {" / "}
            <span className="text-muted">{site.name}</span>
          </span>
        }
        title={site.name}
        subtitle={`${siteTypeLabels[site.type]} · ${site.location} · ${online}/${deviceList.length} online`}
      />

      <Panel className="overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border text-left text-xs uppercase tracking-wide text-faint">
                <th className="px-4 py-3 font-medium">Device</th>
                <th className="px-4 py-3 font-medium">Type</th>
                <th className="px-4 py-3 font-medium">Status</th>
                <th className="px-4 py-3 font-medium">Last seen</th>
              </tr>
            </thead>
            <tbody>
              {deviceList.length === 0 && <EmptyRow colSpan={4}>No devices at this site.</EmptyRow>}
              {deviceList.map((device) => (
                <tr key={device.id} className="border-b border-border/60 last:border-0 hover:bg-surface-2/60">
                  <td className="px-4 py-3">
                    <Link
                      to={`/devices/${device.id}`}
                      className="font-medium text-primary hover:underline focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-primary"
                    >
                      {device.name}
                    </Link>
                  </td>
                  <td className="px-4 py-3 text-muted">{deviceTypeLabels[device.type]}</td>
                  <td className="px-4 py-3">
                    <StatusPill online={isDeviceOnline(device.last_seen_at)} />
                  </td>
                  <td className="px-4 py-3 text-muted">
                    {device.last_seen_at ? (
                      <>
                        <span className="text-foreground">{formatRelative(device.last_seen_at)}</span>
                        <span className="ml-2 text-xs text-faint">{formatTimestamp(device.last_seen_at)}</span>
                      </>
                    ) : (
                      "never"
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Panel>
    </div>
  )
}
