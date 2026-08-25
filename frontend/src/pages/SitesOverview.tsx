import { ChevronRight, Cpu, Factory, Server } from "lucide-react"
import { Link } from "react-router"
import type { Site } from "../api/types"
import { AlertCountBadge, NoAlertsBadge } from "../components/indicators"
import { PageHeader } from "../components/primitives"
import { useAlerts } from "../hooks/useAlerts"
import { useDevices } from "../hooks/useDevices"
import { useSites } from "../hooks/useSites"
import { siteTypeLabels } from "@/lib/utils"

export function SitesOverview() {
  const sites = useSites()
  const devices = useDevices()
  const openAlerts = useAlerts({ status: "open" })

  if (sites.isPending || devices.isPending) return <p className="p-6 text-sm text-muted">Loading…</p>
  if (sites.isError) return <p className="p-6 text-sm text-critical">Failed to load sites: {sites.error.message}</p>

  const deviceCountBySite = new Map<string, number>()
  const deviceToSite = new Map<string, string>()
  for (const device of devices.data ?? []) {
    deviceCountBySite.set(device.site_id, (deviceCountBySite.get(device.site_id) ?? 0) + 1)
    deviceToSite.set(device.id, device.site_id)
  }

  const alertCountsBySite = new Map<string, { warning: number; critical: number }>()
  for (const alert of openAlerts.data ?? []) {
    const siteId = deviceToSite.get(alert.device_id)
    if (!siteId) continue
    const counts = alertCountsBySite.get(siteId) ?? { warning: 0, critical: 0 }
    counts[alert.severity]++
    alertCountsBySite.set(siteId, counts)
  }

  const totalDevices = devices.data?.length ?? 0
  const totalCritical = [...alertCountsBySite.values()].reduce((n, c) => n + c.critical, 0)
  const totalWarning = [...alertCountsBySite.values()].reduce((n, c) => n + c.warning, 0)

  return (
    <div className="mx-auto max-w-7xl px-4 py-6 sm:px-6">
      <PageHeader
        title="Sites"
        subtitle={`${sites.data?.length ?? 0} sites · ${totalDevices} devices monitored`}
        actions={
          (totalCritical > 0 || totalWarning > 0) && (
            <div className="flex items-center gap-2 text-xs">
              {totalCritical > 0 && (
                <span className="inline-flex items-center gap-1.5 rounded border border-critical/40 bg-critical/10 px-2 py-1 font-medium text-critical tabular-nums">
                  {totalCritical} critical
                </span>
              )}
              {totalWarning > 0 && (
                <span className="inline-flex items-center gap-1.5 rounded border border-warning/40 bg-warning/10 px-2 py-1 font-medium text-warning tabular-nums">
                  {totalWarning} warning
                </span>
              )}
            </div>
          )
        }
      />

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {sites.data?.map((site) => (
          <SiteCard
            key={site.id}
            site={site}
            deviceCount={deviceCountBySite.get(site.id) ?? 0}
            alertCounts={alertCountsBySite.get(site.id)}
          />
        ))}
      </div>
    </div>
  )
}

function SiteCard({
  site,
  deviceCount,
  alertCounts,
}: {
  site: Site
  deviceCount: number
  alertCounts: { warning: number; critical: number } | undefined
}) {
  const clear = !alertCounts || (alertCounts.critical === 0 && alertCounts.warning === 0)
  const TypeIcon = site.type === "factory" ? Factory : Server

  return (
    <Link
      to={`/sites/${site.id}`}
      data-testid="site-card"
      className="group flex flex-col rounded-lg border border-border bg-surface p-4 transition-colors hover:border-border-strong hover:bg-surface-2 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
    >
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-start gap-3">
          <span className="mt-0.5 flex size-9 items-center justify-center rounded-md border border-border bg-surface-2 text-primary">
            <TypeIcon className="size-4" aria-hidden />
          </span>
          <div>
            <h2 className="font-semibold text-foreground">{site.name}</h2>
            <p className="mt-0.5 text-xs text-muted">
              {siteTypeLabels[site.type]} · {site.location}
            </p>
          </div>
        </div>
        <ChevronRight className="size-4 text-faint transition-transform group-hover:translate-x-0.5 group-hover:text-muted" aria-hidden />
      </div>

      <div className="mt-4 flex items-center justify-between border-t border-border pt-3">
        <span className="inline-flex items-center gap-1.5 text-xs text-muted">
          <Cpu className="size-3.5" aria-hidden />
          <span className="tabular-nums text-foreground">{deviceCount}</span> devices
        </span>
        <div className="flex items-center gap-1.5">
          {clear ? (
            <NoAlertsBadge />
          ) : (
            <>
              {alertCounts!.critical > 0 && <AlertCountBadge severity="critical" count={alertCounts!.critical} />}
              {alertCounts!.warning > 0 && <AlertCountBadge severity="warning" count={alertCounts!.warning} />}
            </>
          )}
        </div>
      </div>
    </Link>
  )
}
