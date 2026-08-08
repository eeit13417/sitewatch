import assert from "node:assert/strict";
import { test } from "node:test";

import { devices, type DeviceType } from "./devices.js";
import { DeviceSimulator } from "./generate.js";

// Mirrors docs/mqtt-contract.md's "readings shape per device type" table —
// this test fails the moment the two drift apart.
const READINGS_BY_TYPE: Record<DeviceType, string[]> = {
  smart_meter: ["power_kw", "voltage", "current"],
  temp_sensor: ["temperature_c"],
  humidity_sensor: ["humidity_pct"],
  ups: ["battery_pct", "load_pct"],
  hvac: ["temperature_c", "setpoint_c"],
};

// Includes each metric's spikeTo value — a spike is still a valid reading,
// not an out-of-bounds bug.
const BOUNDS: Record<string, [number, number]> = {
  power_kw: [20, 58],
  voltage: [215, 240],
  current: [30, 65],
  temperature_c: [18, 33],
  humidity_pct: [35, 70],
  battery_pct: [90, 100],
  load_pct: [10, 75],
  setpoint_c: [24, 24],
};

test("every device type produces exactly the readings keys the MQTT contract promises", () => {
  for (const device of devices) {
    const sim = new DeviceSimulator(device);

    let readings = sim.tick();
    for (let i = 0; i < 50 && readings === null; i++) readings = sim.tick();
    assert.ok(readings, `${device.name} never produced a reading in 50 ticks`);

    assert.deepEqual(
      Object.keys(readings!).sort(),
      [...READINGS_BY_TYPE[device.type]].sort(),
      `${device.name} (${device.type}) readings keys`,
    );
  }
});

test("readings stay within documented bounds over many ticks, spikes included", () => {
  for (const device of devices) {
    const sim = new DeviceSimulator(device);
    for (let i = 0; i < 500; i++) {
      const readings = sim.tick();
      if (!readings) continue;
      for (const [metric, value] of Object.entries(readings)) {
        const [min, max] = BOUNDS[metric];
        assert.ok(
          value >= min && value <= max,
          `${device.name} ${metric}=${value} outside documented bounds [${min}, ${max}]`,
        );
      }
    }
  }
});

test("a device simulates going offline at least once over a long run", () => {
  const sim = new DeviceSimulator(devices[0]);
  let sawOffline = false;
  for (let i = 0; i < 2000; i++) {
    if (sim.tick() === null) {
      sawOffline = true;
      break;
    }
  }
  assert.ok(sawOffline, "expected at least one offline tick in 2000 ticks (~1% chance per tick)");
});
