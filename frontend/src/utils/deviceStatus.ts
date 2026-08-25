// The simulator publishes every 5s, but ingestion debounces
// devices.last_seen_at to at most once per 60s (see ingestion/store.go).
// A 3-minute staleness threshold gives that debounce window plenty of
// margin without taking minutes to notice a genuinely dropped device.
const ONLINE_THRESHOLD_MS = 3 * 60 * 1000;

export function isDeviceOnline(lastSeenAt: string | null): boolean {
  if (!lastSeenAt) return false;
  return Date.now() - new Date(lastSeenAt).getTime() < ONLINE_THRESHOLD_MS;
}
