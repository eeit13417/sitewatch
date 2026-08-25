import { CartesianGrid, Legend, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import type { TelemetryPoint } from "../api/types";

const LINE_COLORS = ["#2563eb", "#dc2626", "#16a34a", "#d97706"];

export function TelemetryChart({ points }: { points: TelemetryPoint[] }) {
  if (points.length === 0) {
    return <p className="empty-state">No telemetry yet.</p>;
  }

  // readings' keys depend on device_type (docs/mqtt-contract.md) — derive
  // which lines to draw from the data itself instead of hardcoding a
  // metric list per device type here too.
  const metrics = Array.from(new Set(points.flatMap((p) => Object.keys(p.readings))));
  const data = points.map((p) => ({ ts: new Date(p.ts).toLocaleTimeString(), ...p.readings }));

  return (
    <ResponsiveContainer width="100%" height={280}>
      <LineChart data={data}>
        <CartesianGrid strokeDasharray="3 3" />
        <XAxis dataKey="ts" minTickGap={24} />
        <YAxis />
        <Tooltip />
        <Legend />
        {metrics.map((metric, i) => (
          <Line
            key={metric}
            type="monotone"
            dataKey={metric}
            stroke={LINE_COLORS[i % LINE_COLORS.length]}
            dot={false}
            isAnimationActive={false}
          />
        ))}
      </LineChart>
    </ResponsiveContainer>
  );
}
