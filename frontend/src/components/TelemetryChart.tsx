import { CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts"
import type { TelemetryPoint } from "../api/types"
import { extractMetrics, formatTimestamp, metricLabels } from "@/lib/utils"

const LINE_COLORS = ["var(--color-primary)", "var(--color-warning)", "var(--color-ok)", "var(--color-critical)"]

export function TelemetryChart({ points }: { points: TelemetryPoint[] }) {
  if (points.length === 0) {
    return <p className="py-10 text-center text-sm text-faint">No telemetry yet.</p>
  }

  const metrics = extractMetrics(points)
  const data = points.map((p) => ({ t: p.ts, ...p.readings }))

  return (
    <div className="h-72 w-full">
      <ResponsiveContainer width="100%" height="100%">
        <LineChart data={data} margin={{ top: 8, right: 12, bottom: 4, left: -8 }}>
          <CartesianGrid stroke="var(--color-border)" strokeDasharray="3 3" vertical={false} />
          <XAxis
            dataKey="t"
            tickFormatter={(v) => formatTimestamp(v as string).split(",")[1] ?? ""}
            tick={{ fill: "var(--color-faint)", fontSize: 11 }}
            stroke="var(--color-border-strong)"
            minTickGap={40}
          />
          <YAxis tick={{ fill: "var(--color-faint)", fontSize: 11 }} stroke="var(--color-border-strong)" width={44} />
          <Tooltip
            contentStyle={{
              background: "var(--color-surface-2)",
              border: "1px solid var(--color-border-strong)",
              borderRadius: 8,
              fontSize: 12,
              color: "var(--color-foreground)",
            }}
            labelFormatter={(v) => formatTimestamp(v as string)}
            labelStyle={{ color: "var(--color-muted)", marginBottom: 4 }}
            formatter={(value, name) => {
              const meta = metricLabels[name as string]
              return [`${value} ${meta?.unit ?? ""}`, meta?.label ?? name]
            }}
          />
          {metrics.map((metric, i) => (
            <Line
              key={metric}
              type="monotone"
              dataKey={metric}
              stroke={LINE_COLORS[i % LINE_COLORS.length]}
              strokeWidth={2}
              dot={false}
              activeDot={{ r: 3 }}
              isAnimationActive={false}
            />
          ))}
        </LineChart>
      </ResponsiveContainer>
    </div>
  )
}

export function ChartLegend({ metrics }: { metrics: string[] }) {
  return (
    <div className="flex flex-wrap items-center gap-4">
      {metrics.map((metric, i) => {
        const meta = metricLabels[metric]
        return (
          <span key={metric} className="inline-flex items-center gap-1.5 text-xs text-muted">
            <span
              className="inline-block h-0.5 w-4 rounded"
              style={{ background: LINE_COLORS[i % LINE_COLORS.length] }}
              aria-hidden
            />
            {meta?.label ?? metric}
            {meta && <span className="text-faint">({meta.unit})</span>}
          </span>
        )
      })}
    </div>
  )
}
