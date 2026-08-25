import { Link, useParams } from "react-router";
import { DeviceStatusPill } from "../components/DeviceStatusPill";
import { useDevices } from "../hooks/useDevices";
import { useSites } from "../hooks/useSites";

export function SiteDevices() {
  const { siteId } = useParams<{ siteId: string }>();
  // No GET /sites/{id} endpoint exists (sites are only ever listed, not
  // fetched individually — see docs/openapi.yaml) — the list is 2 rows,
  // filtering client-side is simpler than adding an endpoint for it.
  const sites = useSites();
  const devices = useDevices(siteId);

  const site = sites.data?.find((s) => s.id === siteId);

  if (devices.isPending) return <p>Loading…</p>;
  if (devices.isError) return <p className="error">Failed to load devices: {devices.error.message}</p>;

  return (
    <div className="page">
      <Link to="/" className="back-link">
        ← Sites
      </Link>
      <h1>{site?.name ?? "Site"}</h1>
      <table className="device-table">
        <thead>
          <tr>
            <th>Device</th>
            <th>Type</th>
            <th>Status</th>
            <th>Last seen</th>
          </tr>
        </thead>
        <tbody>
          {devices.data?.map((device) => (
            <tr key={device.id}>
              <td>
                <Link to={`/devices/${device.id}`}>{device.name}</Link>
              </td>
              <td>{device.type}</td>
              <td>
                <DeviceStatusPill lastSeenAt={device.last_seen_at} />
              </td>
              <td>{device.last_seen_at ? new Date(device.last_seen_at).toLocaleString() : "never"}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
