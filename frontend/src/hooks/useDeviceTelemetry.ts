import { useQuery } from "@tanstack/react-query";
import { api } from "../api/client";

const TELEMETRY_REFRESH_MS = 10_000;
const DEFAULT_LIMIT = 50;

export function useDeviceTelemetry(deviceId: string, limit = DEFAULT_LIMIT) {
  return useQuery({
    queryKey: ["telemetry", deviceId, limit],
    queryFn: () => api.getDeviceTelemetry(deviceId, limit),
    refetchInterval: TELEMETRY_REFRESH_MS,
  });
}
