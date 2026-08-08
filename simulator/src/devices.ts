export type DeviceType =
  | "smart_meter"
  | "temp_sensor"
  | "humidity_sensor"
  | "ups"
  | "hvac";

export interface DeviceProfile {
  id: string;
  siteId: string;
  name: string;
  type: DeviceType;
}

// Must match the ids seeded in infra/postgres/init.sql exactly — the
// simulator publishes telemetry for devices the database already knows
// about, it doesn't invent its own ids.
export const devices: DeviceProfile[] = [
  {
    id: "a1111111-0000-0000-0000-000000000001",
    siteId: "11111111-1111-1111-1111-111111111111",
    name: "smart-meter-01",
    type: "smart_meter",
  },
  {
    id: "a1111111-0000-0000-0000-000000000002",
    siteId: "11111111-1111-1111-1111-111111111111",
    name: "temp-sensor-01",
    type: "temp_sensor",
  },
  {
    id: "a1111111-0000-0000-0000-000000000003",
    siteId: "11111111-1111-1111-1111-111111111111",
    name: "humidity-sensor-01",
    type: "humidity_sensor",
  },
  {
    id: "a1111111-0000-0000-0000-000000000004",
    siteId: "11111111-1111-1111-1111-111111111111",
    name: "ups-01",
    type: "ups",
  },
  {
    id: "a2222222-0000-0000-0000-000000000001",
    siteId: "22222222-2222-2222-2222-222222222222",
    name: "smart-meter-02",
    type: "smart_meter",
  },
  {
    id: "a2222222-0000-0000-0000-000000000002",
    siteId: "22222222-2222-2222-2222-222222222222",
    name: "temp-sensor-02",
    type: "temp_sensor",
  },
  {
    id: "a2222222-0000-0000-0000-000000000003",
    siteId: "22222222-2222-2222-2222-222222222222",
    name: "hvac-01",
    type: "hvac",
  },
];
