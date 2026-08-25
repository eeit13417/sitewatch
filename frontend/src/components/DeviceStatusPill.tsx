import { isDeviceOnline } from "../utils/deviceStatus";

export function DeviceStatusPill({ lastSeenAt }: { lastSeenAt: string | null }) {
  const online = isDeviceOnline(lastSeenAt);
  return (
    <span className={`pill ${online ? "pill--online" : "pill--offline"}`}>
      {online ? "online" : "offline"}
    </span>
  );
}
