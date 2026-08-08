// Kept as a single function so the simulator and, later, the ingestion
// service derive the same topic string from the same two ids instead of
// each hardcoding the pattern (see docs/mqtt-contract.md).
export function telemetryTopic(siteId: string, deviceId: string): string {
  return `sitewatch/${siteId}/${deviceId}/telemetry`;
}
