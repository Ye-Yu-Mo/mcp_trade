import { useMemo, useState } from "react";
import type { TradeRecord } from "../api/types";
import { fmtSignedUSD, fmtTime, fmtUSDCompact, toneClass } from "../lib/format";

type SeriesPoint = {
  value: number;
  at: string;
  symbol: string;
  pnl: number;
};

const W = 420;
const H = 192;
const PAD = { l: 46, r: 14, t: 16, b: 26 };

export function PnLChart({ trades }: { trades: TradeRecord[] }) {
  const [hover, setHover] = useState<number | null>(null);
  const [showTable, setShowTable] = useState(false);

  const series = useMemo<SeriesPoint[]>(() => {
    const closed = trades
      .filter((t) => (t.Status === "CLOSED" || t.Status === "FILLED") && t.PnL !== 0)
      .slice()
      .sort(
        (a, b) =>
          new Date(a.ClosedAt || a.CreatedAt).getTime() -
          new Date(b.ClosedAt || b.CreatedAt).getTime(),
      );

    let cumulative = 0;
    return closed.map((t) => {
      cumulative += t.PnL;
      return {
        value: cumulative,
        at: t.ClosedAt || t.CreatedAt,
        symbol: t.Symbol,
        pnl: t.PnL,
      };
    });
  }, [trades]);

  const total = series.length > 0 ? series[series.length - 1].value : 0;
  const active = hover !== null ? series[hover] : null;

  const geometry = useMemo(() => {
    if (series.length < 2) return null;

    const values = series.map((s) => s.value);
    const lo = Math.min(0, ...values);
    const hi = Math.max(0, ...values);
    const pad = (hi - lo) * 0.14 || 1;
    const min = lo - pad;
    const max = hi + pad;
    const range = max - min || 1;

    const x = (i: number) =>
      PAD.l + (i / (series.length - 1)) * (W - PAD.l - PAD.r);
    const y = (v: number) => PAD.t + ((max - v) / range) * (H - PAD.t - PAD.b);

    const points = series.map((s, i) => ({ x: x(i), y: y(s.value), ...s }));
    const line = points
      .map((p, i) => `${i === 0 ? "M" : "L"} ${p.x.toFixed(2)} ${p.y.toFixed(2)}`)
      .join(" ");
    const zeroY = y(0);
    const baselineY = H - PAD.b;
    const last = points[points.length - 1];
    const area = `${line} L ${last.x.toFixed(2)} ${baselineY.toFixed(2)} L ${points[0].x.toFixed(2)} ${baselineY.toFixed(2)} Z`;

    const ticks = Array.from(new Set([hi, 0, lo])).map((v) => ({ v, y: y(v) }));

    return { points, line, area, zeroY, ticks, last };
  }, [series]);

  return (
    <section className="panel overflow-hidden">
      <div className="panel-head items-start">
        <div className="min-w-0">
          <div className="panel-title">权益曲线</div>
          <div className="panel-sub mt-1 truncate" aria-live="polite">
            {active
              ? `${fmtTime(active.at)} · ${active.symbol} · ${fmtSignedUSD(active.pnl)} → 累计 ${fmtSignedUSD(active.value)}`
              : "已实现盈亏累计 · USDT"}
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <span className={`chip ${total >= 0 ? "chip-good" : "chip-bad"}`}>
            {fmtSignedUSD(total)}
          </span>
          <button
            type="button"
            onClick={() => setShowTable((v) => !v)}
            aria-expanded={showTable}
            className="chip transition-colors hover:border-[var(--line-strong)] hover:text-[var(--ink)]"
          >
            数据
          </button>
        </div>
      </div>

      <div className="px-4 pb-4 pt-3">
        {!geometry ? (
          <div className="empty">
            <svg
              viewBox="0 0 24 24"
              className="h-6 w-6 text-[var(--ink-3)]"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeLinecap="round"
              strokeLinejoin="round"
              aria-hidden="true"
            >
              <path d="M3 3v18h18" />
              <path d="M7 14l4-4 3 3 5-7" />
            </svg>
            <div className="text-[var(--ink-2)]">暂无权益曲线</div>
            <div className="text-center text-[11px] text-[var(--ink-3)]">
              完成第一笔平仓后开始记录
            </div>
          </div>
        ) : (
          <div className="relative">
            <svg
              viewBox={`0 0 ${W} ${H}`}
              className="h-auto w-full"
              role="img"
              aria-label="已实现盈亏累计曲线"
              onMouseLeave={() => setHover(null)}
            >
              <defs>
                <linearGradient id="equityFill" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="var(--accent)" stopOpacity="0.10" />
                  <stop offset="100%" stopColor="var(--accent)" stopOpacity="0" />
                </linearGradient>
              </defs>

              {geometry.ticks.map((t) => (
                <g key={t.v}>
                  <line
                    x1={PAD.l}
                    x2={W - PAD.r}
                    y1={t.y}
                    y2={t.y}
                    stroke={t.v === 0 ? "var(--line-strong)" : "var(--line)"}
                    strokeWidth="1"
                  />
                  <text
                    x={PAD.l - 7}
                    y={t.y + 3.5}
                    textAnchor="end"
                    fontSize="10"
                    fill="var(--ink-3)"
                    className="num"
                  >
                    {tickLabel(t.v)}
                  </text>
                </g>
              ))}

              <path d={geometry.area} fill="url(#equityFill)" />
              <path
                d={geometry.line}
                fill="none"
                stroke="var(--accent)"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
              />

              <circle
                cx={geometry.last.x}
                cy={geometry.last.y}
                r="4"
                fill="var(--accent)"
                stroke="var(--surface)"
                strokeWidth="2"
              />
              <text
                x={Math.min(geometry.last.x, W - PAD.r)}
                y={
                  geometry.last.y < PAD.t + 18
                    ? geometry.last.y + 16
                    : geometry.last.y - 9
                }
                textAnchor="end"
                fontSize="11"
                fill="var(--ink-2)"
                className="num"
              >
                {fmtSignedUSD(geometry.last.value)}
              </text>

              <text x={PAD.l} y={H - 6} fontSize="10" fill="var(--ink-3)">
                {fmtTime(series[0].at)}
              </text>
              <text
                x={W - PAD.r}
                y={H - 6}
                textAnchor="end"
                fontSize="10"
                fill="var(--ink-3)"
              >
                {fmtTime(series[series.length - 1].at)}
              </text>

              {hover !== null && geometry.points[hover] && (
                <g>
                  <line
                    x1={geometry.points[hover].x}
                    x2={geometry.points[hover].x}
                    y1={PAD.t}
                    y2={H - PAD.b}
                    stroke="var(--line-strong)"
                    strokeWidth="1"
                  />
                  <circle
                    cx={geometry.points[hover].x}
                    cy={geometry.points[hover].y}
                    r="4"
                    fill="var(--accent)"
                    stroke="var(--surface)"
                    strokeWidth="2"
                  />
                </g>
              )}

              <rect
                x={PAD.l}
                y={PAD.t}
                width={W - PAD.l - PAD.r}
                height={H - PAD.t - PAD.b}
                fill="transparent"
                onMouseMove={(e) => {
                  const rect = e.currentTarget.getBoundingClientRect();
                  const fraction = (e.clientX - rect.left) / rect.width;
                  const px = PAD.l + fraction * (W - PAD.l - PAD.r);
                  const step = (W - PAD.l - PAD.r) / Math.max(series.length - 1, 1);
                  const idx = Math.round((px - PAD.l) / step);
                  setHover(Math.max(0, Math.min(series.length - 1, idx)));
                }}
                onMouseLeave={() => setHover(null)}
              />
            </svg>
          </div>
        )}

        {showTable && series.length > 0 && (
          <div className="custom-scrollbar mt-3 max-h-[184px] overflow-auto border-t border-[var(--line)] pt-3">
            <table className="table min-w-[300px]">
              <thead>
                <tr>
                  <th>时间</th>
                  <th>标的</th>
                  <th className="text-right">本笔</th>
                  <th className="text-right">累计</th>
                </tr>
              </thead>
              <tbody>
                {series
                  .slice()
                  .reverse()
                  .slice(0, 12)
                  .map((s, i) => (
                    <tr key={`${s.at}-${i}`}>
                      <td className="num text-[11px] text-[var(--ink-3)]">{fmtTime(s.at)}</td>
                      <td className="text-[var(--ink-2)]">{s.symbol}</td>
                      <td className={`num text-right ${toneClass(s.pnl)}`}>
                        {fmtSignedUSD(s.pnl)}
                      </td>
                      <td className={`num text-right ${toneClass(s.value)}`}>
                        {fmtSignedUSD(s.value)}
                      </td>
                    </tr>
                  ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </section>
  );
}

function tickLabel(v: number): string {
  const sign = v > 0 ? "+" : "";
  return `${sign}${fmtUSDCompact(v)}`;
}
