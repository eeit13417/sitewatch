import type { Alert, AlertRule, AlertStatus, Device, Site, TelemetryPoint } from "./types";

const BASE_URL = import.meta.env.VITE_API_URL ?? "http://localhost:8080";

// A class is the right call here (not a plain object/interface) — an
// error is state (status + message) plus identity (instanceof checks in
// callers), which is exactly what a class is for. Field declared and
// assigned explicitly, not via constructor-parameter-property shorthand —
// this project's tsconfig enables `erasableSyntaxOnly`, which that
// shorthand isn't compatible with.
export class ApiError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`, {
    ...init,
    headers: { "Content-Type": "application/json", ...init?.headers },
  });

  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as { error?: string } | null;
    throw new ApiError(res.status, body?.error ?? `request failed with status ${res.status}`);
  }
  if (res.status === 204) {
    return undefined as T;
  }
  return (await res.json()) as T;
}

function query(params: Record<string, string | number | undefined>): string {
  const entries = Object.entries(params).filter(
    (entry): entry is [string, string | number] => entry[1] !== undefined && entry[1] !== "",
  );
  if (entries.length === 0) return "";
  return `?${new URLSearchParams(entries.map(([k, v]) => [k, String(v)]))}`;
}

// The single fetch layer every hook goes through — no component calls
// fetch()/request() directly, so base URL handling and error shaping stay
// in one place (see docs/frontend-design.md).
export const api = {
  listSites: () => request<Site[]>("/sites"),

  listDevices: (siteId?: string) => request<Device[]>(`/devices${query({ site_id: siteId })}`),
  getDevice: (id: string) => request<Device>(`/devices/${id}`),
  getDeviceTelemetry: (id: string, limit = 100) =>
    request<TelemetryPoint[]>(`/devices/${id}/telemetry${query({ limit })}`),

  listAlerts: (params: { status?: AlertStatus; device_id?: string; site_id?: string; limit?: number } = {}) =>
    request<Alert[]>(`/alerts${query(params)}`),
  acknowledgeAlert: (id: string, userId: string) =>
    request<Alert>(`/alerts/${id}/acknowledge`, { method: "POST", body: JSON.stringify({ user_id: userId }) }),
  resolveAlert: (id: string, userId: string) =>
    request<Alert>(`/alerts/${id}/resolve`, { method: "POST", body: JSON.stringify({ user_id: userId }) }),

  listAlertRules: (deviceId?: string) => request<AlertRule[]>(`/alert-rules${query({ device_id: deviceId })}`),
};
