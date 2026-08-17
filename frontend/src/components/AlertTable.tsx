import type { Alert } from "../api/types";
import { AlertSeverityBadge } from "./AlertSeverityBadge";

interface AlertTableProps {
  alerts: Alert[];
  onAcknowledge: (id: string) => void;
  onResolve: (id: string) => void;
  /** id of the alert currently being mutated, to disable its own buttons only */
  busyAlertId: string | null;
}

// Presentational only — no data fetching, no mutation logic — so it's
// reusable from both the alerts page and a device's own alert history
// without either owning how the other's mutations are wired.
export function AlertTable({ alerts, onAcknowledge, onResolve, busyAlertId }: AlertTableProps) {
  if (alerts.length === 0) {
    return <p className="empty-state">No alerts.</p>;
  }

  return (
    <table className="alert-table">
      <thead>
        <tr>
          <th>Severity</th>
          <th>Status</th>
          <th>Value</th>
          <th>Triggered</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        {alerts.map((alert) => {
          const busy = busyAlertId === alert.id;
          return (
            <tr key={alert.id}>
              <td>
                <AlertSeverityBadge severity={alert.severity} />
              </td>
              <td>{alert.status}</td>
              <td>{alert.triggered_value}</td>
              <td>{new Date(alert.triggered_at).toLocaleString()}</td>
              <td className="alert-table__actions">
                {alert.status === "open" && (
                  <button type="button" disabled={busy} onClick={() => onAcknowledge(alert.id)}>
                    Acknowledge
                  </button>
                )}
                {(alert.status === "open" || alert.status === "acknowledged") && (
                  <button type="button" disabled={busy} onClick={() => onResolve(alert.id)}>
                    Resolve
                  </button>
                )}
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}
