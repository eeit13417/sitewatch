import path from "node:path";
import { fileURLToPath } from "node:url";
import { config } from "dotenv";
import mqtt from "mqtt";

import { devices } from "./devices.js";
import { DeviceSimulator } from "./generate.js";
import { telemetryTopic } from "./mqtt-topic.js";

// Load the repo-root .env (sitewatch/.env), two levels up from this file
// whether it's running from src/ via tsx or dist/ via node.
const __dirname = path.dirname(fileURLToPath(import.meta.url));
config({ path: path.resolve(__dirname, "../../.env") });

const brokerUrl = process.env.MQTT_BROKER_URL ?? "tcp://localhost:1883";
const intervalMs = Number(process.env.SIM_INTERVAL_MS ?? 5000);

const sims = new Map(devices.map((device) => [device.id, new DeviceSimulator(device)]));

const client = mqtt.connect(brokerUrl, {
  clientId: `sitewatch-simulator-${Math.random().toString(16).slice(2, 8)}`,
});

client.on("connect", () => {
  console.log(
    `[simulator] connected to ${brokerUrl} — publishing ${devices.length} devices every ${intervalMs}ms`,
  );

  setInterval(() => {
    for (const device of devices) {
      const readings = sims.get(device.id)!.tick();
      if (readings === null) {
        continue; // simulated offline this tick
      }

      const payload = {
        device_id: device.id,
        site_id: device.siteId,
        device_type: device.type,
        readings,
        ts: new Date().toISOString(),
      };

      client.publish(telemetryTopic(device.siteId, device.id), JSON.stringify(payload));
    }
  }, intervalMs);
});

client.on("error", (err) => {
  console.error("[simulator] mqtt error:", err.message);
});
