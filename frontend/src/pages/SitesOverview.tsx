import { Link } from "react-router";
import { useAlerts } from "../hooks/useAlerts";
import { useDevices } from "../hooks/useDevices";
import { useSites } from "../hooks/useSites";

export function SitesOverview() {
  const sites = useSites();
  const devices = useDevices();
  const openAlerts = useAlerts({ status: "open" });

  if (sites.isPending || devices.isPending) return <p>Loading…</p>;
  if (sites.isError) return <p className="error">Failed to load sites: {sites.error.message}</p>;

  const deviceCountBySite = new Map<string, number>();
  const deviceToSite = new Map<string, string>();
  for (const device of devices.data ?? []) {
    deviceCountBySite.set(device.site_id, (deviceCountBySite.get(device.site_id) ?? 0) + 1);
    deviceToSite.set(device.id, device.site_id);
  }

  const alertCountsBySite = new Map<string, { warning: number; critical: number }>();
  for (const alert of openAlerts.data ?? []) {
    const siteId = deviceToSite.get(alert.device_id);
    if (!siteId) continue;
    const counts = alertCountsBySite.get(siteId) ?? { warning: 0, critical: 0 };
    counts[alert.severity]++;
    alertCountsBySite.set(siteId, counts);
  }

  return (
    <div className="page">
      <h1>Sites</h1>
      <div className="card-grid">
        {sites.data?.map((site) => {
          const alertCounts = alertCountsBySite.get(site.id);
          return (
            <Link key={site.id} to={`/sites/${site.id}`} className="card">
              <h2>{site.name}</h2>
              <p className="card__meta">
                {site.type} · {site.location}
              </p>
              <p>{deviceCountBySite.get(site.id) ?? 0} devices</p>
              {alertCounts && (alertCounts.critical > 0 || alertCounts.warning > 0) ? (
                <p className="card__alerts">
                  {alertCounts.critical > 0 && <span className="badge badge--critical">{alertCounts.critical} critical</span>}
                  {alertCounts.warning > 0 && <span className="badge badge--warning">{alertCounts.warning} warning</span>}
                </p>
              ) : (
                <p className="card__alerts card__alerts--clear">No open alerts</p>
              )}
            </Link>
          );
        })}
      </div>
    </div>
  );
}
