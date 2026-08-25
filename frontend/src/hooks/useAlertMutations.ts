import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api/client";
import { ACTING_USER_ID } from "../config";

// Both mutations invalidate every "alerts" query (regardless of filter
// params) rather than trying to patch the cache in place — the dedup/
// auto-resolve state machine in ingestion/alerts.go can also change alert
// state server-side between polls, so trusting a refetch over a hand
// -rolled cache update avoids the UI quietly drifting from the database.
export function useAcknowledgeAlert() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (alertId: string) => api.acknowledgeAlert(alertId, ACTING_USER_ID),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["alerts"] }),
  });
}

export function useResolveAlert() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (alertId: string) => api.resolveAlert(alertId, ACTING_USER_ID),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["alerts"] }),
  });
}
