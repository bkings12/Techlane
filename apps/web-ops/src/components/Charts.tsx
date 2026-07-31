// Tiny dependency-free SVG charts. No charting library required — these are
// deliberately simple (trend lines + comparison bars) and scale via viewBox
// so they stay crisp at any container width.

const CHART_WIDTH = 600;

export type ChartSeries = {
  label: string;
  color: string;
  values: number[];
};

export function LineChart({
  labels,
  series,
  height = 200,
  formatValue,
}: {
  labels: string[];
  series: ChartSeries[];
  height?: number;
  formatValue?: (v: number) => string;
}) {
  const padTop = 10;
  const padBottom = 22;
  const plotHeight = height - padTop - padBottom;
  const max = Math.max(1, ...series.flatMap((s) => s.values));
  const stepX = labels.length > 1 ? CHART_WIDTH / (labels.length - 1) : CHART_WIDTH;
  const toY = (v: number) => padTop + plotHeight - (v / max) * plotHeight;
  const fmt = formatValue ?? ((v: number) => v.toLocaleString());

  if (labels.length === 0) {
    return <p className="muted">No data for this period yet.</p>;
  }

  return (
    <div className="chart-wrap">
      <svg viewBox={`0 0 ${CHART_WIDTH} ${height}`} preserveAspectRatio="none" className="line-chart" role="img" aria-label="Trend chart">
        {[0, 0.25, 0.5, 0.75, 1].map((f) => {
          const y = padTop + plotHeight - f * plotHeight;
          return (
            <line key={f} x1={0} x2={CHART_WIDTH} y1={y} y2={y} className="chart-grid" vectorEffect="non-scaling-stroke" />
          );
        })}
        {series.map((s) => {
          const points = s.values.map((v, i) => `${i * stepX},${toY(v)}`).join(" ");
          return (
            <polyline
              key={s.label}
              points={points}
              fill="none"
              stroke={s.color}
              strokeWidth={2}
              strokeLinejoin="round"
              strokeLinecap="round"
              vectorEffect="non-scaling-stroke"
            />
          );
        })}
      </svg>
      <div className="chart-axis-labels">
        <span>{labels[0]}</span>
        {labels.length > 2 ? <span>{labels[Math.floor(labels.length / 2)]}</span> : null}
        <span>{labels[labels.length - 1]}</span>
      </div>
      <div className="chart-legend">
        {series.map((s) => {
          const total = s.values.reduce((a, b) => a + b, 0);
          return (
            <span key={s.label} className="chart-legend-item">
              <i style={{ background: s.color }} /> {s.label} <strong>{fmt(total)}</strong>
            </span>
          );
        })}
      </div>
    </div>
  );
}

export function BarChart({
  items,
  height = 180,
  formatValue,
}: {
  items: { label: string; value: number; color?: string }[];
  height?: number;
  formatValue?: (v: number) => string;
}) {
  const max = Math.max(1, ...items.map((i) => i.value));
  const fmt = formatValue ?? ((v: number) => v.toLocaleString());

  if (items.length === 0) {
    return <p className="muted">No data for this period yet.</p>;
  }

  return (
    <div className="bar-chart" style={{ height }}>
      {items.map((i) => (
        <div key={i.label} className="bar-chart-col" title={`${i.label}: ${fmt(i.value)}`}>
          <span className="bar-chart-value">{i.value > 0 ? fmt(i.value) : ""}</span>
          <div
            className="bar-chart-bar"
            style={{ height: `${Math.max(2, (i.value / max) * 100)}%`, background: i.color ?? "var(--navy-700, #063086)" }}
          />
          <span className="bar-chart-label">{i.label}</span>
        </div>
      ))}
    </div>
  );
}
