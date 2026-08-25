import { useState } from "react";
import { AlertTable } from "../components/AlertTable";
import { useAcknowledgeAlert, useResolveAlert } from "../hooks/useAlertMutations";
import { useAlerts } from "../hooks/useAlerts";
import { useSites } from "../hooks/useSites";
import type { AlertStatus } from "../api/types";

const STATUS_OPTIONS: (AlertStatus | "")[] = ["", "open", "acknowledged", "resolved"];

export function AlertsPage() {
  const [status, setStatus] = useState<AlertStatus | "">("open");
  const [siteId, setSiteId] = useState("");

  const sites = useSites();
  const alerts = useAlerts({ status: status || undefined, site_id: siteId || undefined });

  const [busyAlertId, setBusyAlertId] = useState<string | null>(null);
  const acknowledge = useAcknowledgeAlert();
  const resolve = useResolveAlert();

  const runMutation = (mutate: typeof acknowledge, id: string) => {
    setBusyAlertId(id);
    mutate.mutate(id, { onSettled: () => setBusyAlertId(null) });
  };

  return (
    <div className="page">
      <h1>Alerts</h1>
      <div className="filters">
        <label>
          Status
          <select value={status} onChange={(e) => setStatus(e.target.value as AlertStatus | "")}>
            {STATUS_OPTIONS.map((s) => (
              <option key={s} value={s}>
                {s || "all"}
              </option>
            ))}
          </select>
        </label>
        <label>
          Site
          <select value={siteId} onChange={(e) => setSiteId(e.target.value)}>
            <option value="">all</option>
            {sites.data?.map((site) => (
              <option key={site.id} value={site.id}>
                {site.name}
              </option>
            ))}
          </select>
        </label>
      </div>

      {alerts.isPending ? (
        <p>Loading…</p>
      ) : alerts.isError ? (
        <p className="error">Failed to load alerts: {alerts.error.message}</p>
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
