import { useQuery } from "@tanstack/react-query";
import { api } from "../api/client";

export function useDevices(siteId?: string) {
  return useQuery({
    queryKey: ["devices", siteId ?? "all"],
    queryFn: () => api.listDevices(siteId),
  });
}

// Devices report every few seconds; a UI that never refreshes shows a
// device as "online" long after it's gone quiet. Matches the simulator's
// publish interval order of magnitude, not the alerts list's 5s (this is
// a lower-urgency signal).
const DEVICE_REFRESH_MS = 15_000;

export function useDevice(id: string) {
  return useQuery({
    queryKey: ["device", id],
    queryFn: () => api.getDevice(id),
    refetchInterval: DEVICE_REFRESH_MS,
  });
}
