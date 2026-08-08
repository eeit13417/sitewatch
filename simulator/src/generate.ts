import type { DeviceProfile, DeviceType } from "./devices.js";

interface MetricSpec {
  base: number;
  volatility: number; // max +/- change per tick, before clamping
  min: number;
  max: number;
  /** Occasional out-of-range value, used to exercise the seeded alert_rules. */
  spikeTo?: number;
  spikeChance?: number; // 0..1 per tick
}

const METRIC_SPECS: Record<string, MetricSpec> = {
  power_kw: { base: 40, volatility: 2, min: 20, max: 48, spikeTo: 58, spikeChance: 0.03 },
  voltage: { base: 230, volatility: 1.5, min: 215, max: 240 },
  current: { base: 50, volatility: 3, min: 30, max: 65 },
  temperature_c: { base: 24, volatility: 0.6, min: 18, max: 27, spikeTo: 33, spikeChance: 0.03 },
  humidity_pct: { base: 55, volatility: 2, min: 35, max: 70 },
  battery_pct: { base: 98, volatility: 0.3, min: 90, max: 100 },
  load_pct: { base: 40, volatility: 4, min: 10, max: 75 },
  setpoint_c: { base: 24, volatility: 0, min: 24, max: 24 },
};

const READINGS_BY_TYPE: Record<DeviceType, string[]> = {
  smart_meter: ["power_kw", "voltage", "current"],
  temp_sensor: ["temperature_c"],
  humidity_sensor: ["humidity_pct"],
  ups: ["battery_pct", "load_pct"],
  hvac: ["temperature_c", "setpoint_c"],
};

const OFFLINE_CHANCE = 0.01; // per tick, only rolled while online
const OFFLINE_MIN_TICKS = 5;
const OFFLINE_MAX_TICKS = 15;

/**
 * Holds per-device state (current metric values, offline countdown) between
 * ticks so readings drift smoothly instead of being independent random
 * numbers each time.
 */
export class DeviceSimulator {
  private readonly metrics: string[];
  private readonly values = new Map<string, number>();
  private offlineTicksRemaining = 0;

  constructor(profile: DeviceProfile) {
    this.metrics = READINGS_BY_TYPE[profile.type];
    for (const metric of this.metrics) {
      this.values.set(metric, METRIC_SPECS[metric].base);
    }
  }

  /** Returns readings for this tick, or null if the device is simulated offline. */
  tick(): Record<string, number> | null {
    if (this.offlineTicksRemaining > 0) {
      this.offlineTicksRemaining -= 1;
      return null;
    }

    if (Math.random() < OFFLINE_CHANCE) {
      this.offlineTicksRemaining =
        OFFLINE_MIN_TICKS + Math.floor(Math.random() * (OFFLINE_MAX_TICKS - OFFLINE_MIN_TICKS));
      return null;
    }

    const readings: Record<string, number> = {};
    for (const metric of this.metrics) {
      const spec = METRIC_SPECS[metric];
      const spike = spec.spikeTo !== undefined && Math.random() < (spec.spikeChance ?? 0);
      const next = spike
        ? spec.spikeTo!
        : clamp(this.values.get(metric)! + randomStep(spec.volatility), spec.min, spec.max);

      this.values.set(metric, next);
      readings[metric] = round2(next);
    }
    return readings;
  }
}

function randomStep(volatility: number): number {
  return (Math.random() * 2 - 1) * volatility;
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

function round2(value: number): number {
  return Math.round(value * 100) / 100;
}
