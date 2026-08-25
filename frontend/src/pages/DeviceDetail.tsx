import { useState } from "react";
import { Link, useParams } from "react-router";
import { AlertTable } from "../components/AlertTable";
import { DeviceStatusPill } from "../components/DeviceStatusPill";
import { TelemetryChart } from "../components/TelemetryChart";
import { useAcknowledgeAlert, useResolveAlert } from "../hooks/useAlertMutations";
import { useAlerts } from "../hooks/useAlerts";
import { useDevice } from "../hooks/useDevices";
import { useDeviceTelemetry } from "../hooks/useDeviceTelemetry";

// This panel is "recent context for this device", not a full audit log —
// a busy device can accumulate a lot of resolved-alert history (caught
// during Phase 3 QA: an unbounded query here rendered 30+ historical rows
// with no pagination). The full, filterable, paginated history lives on
// the /alerts page.
const RECENT_ALERTS_LIMIT = 10;

export function DeviceDetail() {
  const { deviceId } = useParams<{ deviceId: string }>();
  const device = useDevice(deviceId!);
  const telemetry = useDeviceTelemetry(deviceId!);
  const alerts = useAlerts({ device_id: deviceId, limit: RECENT_ALERTS_LIMIT });

  const [busyAlertId, setBusyAlertId] = useState<string | null>(null);
  const acknowledge = useAcknowledgeAlert();
  const resolve = useResolveAlert();

  const runMutation = (mutate: typeof acknowledge, id: string) => {
    setBusyAlertId(id);
    mutate.mutate(id, { onSettled: () => setBusyAlertId(null) });
  };

  if (device.isPending) return <p>Loading…</p>;
  if (device.isError) return <p className="error">Failed to load device: {device.error.message}</p>;

  return (
    <div className="page">
      <Link to={`/sites/${device.data.site_id}`} className="back-link">
        ← Devices
      </Link>
      <h1>
        {device.data.name} <DeviceStatusPill lastSeenAt={device.data.last_seen_at} />
      </h1>
      <p className="card__meta">{device.data.type}</p>

      <h2>Telemetry</h2>
      {telemetry.isPending ? <p>Loading…</p> : <TelemetryChart points={telemetry.data ?? []} />}

      <h2>Alerts</h2>
      <p className="card__meta">
        Most recent {RECENT_ALERTS_LIMIT} — full history on the <Link to="/alerts">Alerts page</Link>.
      </p>
      {alerts.isPending ? (
        <p>Loading…</p>
      ) : (
        <AlertTable
          alerts={alerts.data ?? []}
          busyAlertId={busyAlertId}
          onAcknowledge={(id) => runMutation(acknowledge, id)}
          onResolve={(id) => runMutation(resolve, id)}
        />
      )}
    </div>
  );
}
