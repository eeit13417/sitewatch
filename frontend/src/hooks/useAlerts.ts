import { useQuery } from "@tanstack/react-query";
import { api } from "../api/client";
import type { AlertStatus } from "../api/types";

export interface AlertFilters {
  status?: AlertStatus;
  device_id?: string;
  site_id?: string;
  limit?: number;
}

const ALERTS_REFRESH_MS = 5_000;

export function useAlerts(filters: AlertFilters = {}) {
  return useQuery({
    queryKey: ["alerts", filters],
    queryFn: () => api.listAlerts(filters),
    refetchInterval: ALERTS_REFRESH_MS,
  });
}
