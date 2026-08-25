import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"
import type { TelemetryPoint } from "../api/types"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatTimestamp(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleString(undefined, {
    month: "short",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  })
}

export function formatRelative(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime()
  const sec = Math.round(diff / 1000)
  if (sec < 60) return `${sec}s ago`
  const min = Math.round(sec / 60)
  if (min < 60) return `${min}m ago`
  const hr = Math.round(min / 60)
  if (hr < 24) return `${hr}h ago`
  const day = Math.round(hr / 24)
  return `${day}d ago`
}

export const deviceTypeLabels: Record<string, string> = {
  smart_meter: "Smart Meter",
  temp_sensor: "Temperature",
  humidity_sensor: "Humidity",
  ups: "UPS",
  hvac: "HVAC",
}

// Matches api/types.ts SiteType exactly — "building" is real (Postgres
// CHECK constraint allows it) even though no seeded site currently uses
// it, unlike the v0 mock data's two-value set.
export const siteTypeLabels: Record<string, string> = {
  data_center: "Data Center",
  building: "Building",
  factory: "Factory",
}

// Per-metric display metadata, keyed by the metric name itself (matches
// docs/mqtt-contract.md's readings keys) rather than by device type — one
// metric name means the same thing regardless of which device type
// reports it, so this stays a flat lookup instead of a per-device-type
// table that would just repeat entries.
export const metricLabels: Record<string, { label: string; unit: string }> = {
  voltage: { label: "Voltage", unit: "V" },
  current: { label: "Current", unit: "A" },
  power_kw: { label: "Power", unit: "kW" },
  temperature_c: { label: "Temperature", unit: "°C" },
  humidity_pct: { label: "Humidity", unit: "%" },
  battery_pct: { label: "Battery", unit: "%" },
  load_pct: { label: "Load", unit: "%" },
  setpoint_c: { label: "Setpoint", unit: "°C" },
}

// readings' keys depend on device_type (docs/mqtt-contract.md) — derived
// from the data itself rather than a per-device-type table, so the chart
// and its legend never need updating when a new device type is added.
export function extractMetrics(points: TelemetryPoint[]): string[] {
  return Array.from(new Set(points.flatMap((p) => Object.keys(p.readings))))
}
