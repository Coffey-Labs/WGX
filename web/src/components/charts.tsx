import { useMemo } from "react";
import type { TrafficPoint } from "../api";
import { bytes } from "../format";

// Both charts are plain SVG: no library, no runtime dependency, and they
// pick their colours up from the CSS variables so light and dark just work.

export function TrafficChart({ points, from, to, bucketSeconds = 300 }: { points: TrafficPoint[]; from: number; to: number; bucketSeconds?: number }) {
  const W = 800;
  const H = 180;
  const padL = 48;
  const padB = 22;
  const padT = 8;
  const padR = 8;

  const { rxPath, txPath, max, ticks, xTicks } = useMemo(() => {
    const span = Math.max(1, to - from);
    const byBucket = new Map<number, TrafficPoint>();
    for (const p of points) byBucket.set(Math.floor(new Date(p.t).getTime() / 1000), p);
    // One bar per bucket across the whole window, zeros where nothing was
    // recorded, so quiet periods read as quiet rather than missing.
    const start = Math.floor(from / 1000 / bucketSeconds) * bucketSeconds;
    const end = Math.floor(to / 1000);
    const rows: { t: number; rx: number; tx: number }[] = [];
    for (let t = start; t <= end; t += bucketSeconds) {
      const p = byBucket.get(t);
      rows.push({ t, rx: p?.rx ?? 0, tx: p?.tx ?? 0 });
    }
    const max = Math.max(1, ...rows.map((r) => Math.max(r.rx, r.tx)));
    const x = (t: number) => padL + ((t * 1000 - from) / span) * (W - padL - padR);
    const y = (v: number) => padT + (1 - v / max) * (H - padT - padB);
    const path = (key: "rx" | "tx") => {
      if (rows.length === 0) return "";
      let d = `M${x(rows[0].t).toFixed(1)},${y(0).toFixed(1)}`;
      for (const r of rows) d += ` L${x(r.t).toFixed(1)},${y(r[key]).toFixed(1)}`;
      d += ` L${x(rows[rows.length - 1].t + bucketSeconds).toFixed(1)},${y(rows[rows.length - 1][key]).toFixed(1)}`;
      d += ` L${x(rows[rows.length - 1].t + bucketSeconds).toFixed(1)},${y(0).toFixed(1)} Z`;
      return d;
    };
    const ticks = [0, 0.5, 1].map((f) => ({ v: max * f, y: y(max * f) }));
    const xTicks: { x: number; label: string }[] = [];
    const n = 6;
    for (let i = 0; i <= n; i++) {
      const t = from + (span * i) / n;
      const d = new Date(t);
      const label = span > 2 * 86400 * 1000 ? d.toLocaleDateString(undefined, { month: "short", day: "numeric" }) : d.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
      xTicks.push({ x: padL + (i / n) * (W - padL - padR), label });
    }
    return { rxPath: path("rx"), txPath: path("tx"), max, ticks, xTicks };
  }, [points, from, to, bucketSeconds]);

  return (
    <svg className="chart" viewBox={`0 0 ${W} ${H}`} preserveAspectRatio="none" role="img" aria-label="Traffic over time">
      {ticks.map((t) => (
        <g key={t.v}>
          <line x1={padL} x2={W - padR} y1={t.y} y2={t.y} stroke="var(--line)" strokeWidth="1" />
          <text x={padL - 6} y={t.y + 4} fontSize="10" textAnchor="end" fill="var(--fg-faint)">
            {bytes(t.v)}
          </text>
        </g>
      ))}
      <path d={txPath} fill="var(--tx)" fillOpacity="0.35" stroke="var(--tx)" strokeWidth="1.2" />
      <path d={rxPath} fill="var(--rx)" fillOpacity="0.35" stroke="var(--rx)" strokeWidth="1.2" />
      {xTicks.map((t, i) => (
        <text key={i} x={t.x} y={H - 6} fontSize="10" textAnchor={i === 0 ? "start" : i === xTicks.length - 1 ? "end" : "middle"} fill="var(--fg-faint)">
          {t.label}
        </text>
      ))}
      <title>peak {bytes(max)} per bucket</title>
    </svg>
  );
}

export function Sparkline({ values, color = "var(--accent)" }: { values: number[]; color?: string }) {
  const W = 110;
  const H = 26;
  const d = useMemo(() => {
    if (values.length < 2) return "";
    const max = Math.max(1, ...values);
    const step = W / (values.length - 1);
    return values.map((v, i) => `${i === 0 ? "M" : "L"}${(i * step).toFixed(1)},${(H - 2 - (v / max) * (H - 4)).toFixed(1)}`).join(" ");
  }, [values]);
  return (
    <svg className="sparkline" viewBox={`0 0 ${W} ${H}`} preserveAspectRatio="none" aria-hidden="true">
      <path d={d} fill="none" stroke={color} strokeWidth="1.5" strokeLinejoin="round" />
    </svg>
  );
}

export function Legend() {
  return (
    <div className="legend">
      <span>
        <i style={{ background: "var(--rx)" }} /> received from peers
      </span>
      <span>
        <i style={{ background: "var(--tx)" }} /> sent to peers
      </span>
    </div>
  );
}
