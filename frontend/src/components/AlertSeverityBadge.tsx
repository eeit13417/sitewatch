import type { AlertSeverity } from "../api/types";

export function AlertSeverityBadge({ severity }: { severity: AlertSeverity }) {
  return <span className={`badge badge--${severity}`}>{severity}</span>;
}
