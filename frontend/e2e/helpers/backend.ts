import mqtt from "mqtt";

// Same fixed seed ids as infra/postgres/init.sql — E2E tests exercise the
// real pipeline (MQTT → ingestion → Postgres → api → UI), not a mocked
// API, so they need real device/site ids that actually exist.
export const BANGKOK_SITE_ID = "11111111-1111-1111-1111-111111111111";
export const TEMP_SENSOR_01_ID = "a1111111-0000-0000-0000-000000000002";

const MQTT_URL = process.env.E2E_MQTT_URL ?? "tcp://localhost:1883";
const API_URL = process.env.VITE_API_URL ?? "http://localhost:8080";

function telemetryTopic(siteId: string, deviceId: string): string {
  return `sitewatch/${siteId}/${deviceId}/telemetry`;
}

async function publish(siteId: string, deviceId: string, deviceType: string, readings: Record<string, number>) {
  const client = await mqtt.connectAsync(MQTT_URL);
  try {
    await client.publishAsync(
      telemetryTopic(siteId, deviceId),
      JSON.stringify({
        device_id: deviceId,
        site_id: siteId,
        device_type: deviceType,
        readings,
        ts: new Date().toISOString(),
      }),
    );
  } finally {
    await client.endAsync();
  }
}

/** Publishes a reading that trips temp-sensor-01's seeded warning+critical rules. */
export function publishTempSpike() {
  return publish(BANGKOK_SITE_ID, TEMP_SENSOR_01_ID, "temp_sensor", { temperature_c: 33 });
}

/** Publishes a normal reading — clears any active alert on temp-sensor-01 (auto-resolve). */
export function publishTempNormal() {
  return publish(BANGKOK_SITE_ID, TEMP_SENSOR_01_ID, "temp_sensor", { temperature_c: 22 });
}

/**
 * Polls the real api until it reports the expected alert state for
 * temp-sensor-01, instead of a fixed sleep — the MQTT → ingestion →
 * Postgres path is asynchronous, and a fixed delay is either too slow
 * (wastes time every run) or too fast (flaky under load).
 */
export async function waitForAlertStatus(status: "open" | "resolved" | "none", timeoutMs = 10_000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const res = await fetch(`${API_URL}/alerts?device_id=${TEMP_SENSOR_01_ID}&status=open`);
    const openAlerts = (await res.json()) as unknown[];
    const hasOpen = openAlerts.length > 0;
    if (status === "open" && hasOpen) return;
    if ((status === "resolved" || status === "none") && !hasOpen) return;
    await new Promise((r) => setTimeout(r, 300));
  }
  throw new Error(`timed out waiting for alert status=${status} on temp-sensor-01`);
}
