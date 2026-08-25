// Mirrors docs/openapi.yaml. Go, TypeScript, and the database schema can't
// literally share one definition across languages — openapi.yaml is the
// documented contract all three are kept aligned to by hand (same
// situation as the MQTT topic convention between the simulator and
// ingestion).

export type SiteType = "data_center" | "building" | "factory";

export interface Site {
  id: string;
  name: string;
  type: SiteType;
  location: string;
}

export type DeviceType =
  | "smart_meter"
  | "temp_sensor"
  | "humidity_sensor"
  | "ups"
  | "hvac";

export interface Device {
  id: string;
  site_id: string;
  name: string;
  type: DeviceType;
  last_seen_at: string | null;
}

export interface TelemetryPoint {
  device_id: string;
  readings: Record<string, number>;
  ts: string;
  received_at: string;
}

export type AlertSeverity = "warning" | "critical";
export type AlertStatus = "open" | "acknowledged" | "resolved";
export type AlertOperator = "gt" | "gte" | "lt" | "lte";

export interface AlertRule {
  id: string;
  device_id: string;
  metric: string;
  operator: AlertOperator;
  threshold: number;
  severity: AlertSeverity;
  enabled: boolean;
}

export interface Alert {
  id: string;
  alert_rule_id: string;
  device_id: string;
  triggered_value: number;
  severity: AlertSeverity;
  status: AlertStatus;
  triggered_at: string;
  acknowledged_at: string | null;
  acknowledged_by: string | null;
  resolved_at: string | null;
  resolved_by: string | null;
}
