import type { PerformanceStats, Position } from "../api/types";
import { fmtSignedUSD, fmtUSD, toneClass } from "../lib/format";

export function RiskPanel({
  positions,
  balance,
  dailyPnL,
  perf,
}: {
  positions: Position[];
  balance: number;
  dailyPnL: number;
  perf: PerformanceStats | null;
}) {
  const marginUsed = positions.reduce(
    (s, p) => s + (p.Quantity * p.MarkPrice) / (p.Leverage || 1),
    0,
  );
  const notional = positions.reduce((s, p) => s + p.Quantity * p.MarkPrice, 0);
  const marginRatio = balance > 0 ? Math.min((marginUsed / balance) * 100, 100) : 0;
  const level = marginRatio > 80 ? "critical" : marginRatio > 50 ? "warning" : "good";
  const fill =
    level === "critical"
      ? "bg-[#d03b3b]"
      : level === "warning"
        ? "bg-[#fab219]"
        : "bg-[var(--accent)]";
  const statusLabel =
    level === "critical" ? "高风险" : level === "warning" ? "需关注" : "正常";

  return (
    <section className="panel overflow-hidden">
      <div className="panel-head">
        <div>
          <div className="panel-title">风控状态</div>
          <div className="panel-sub mt-0.5">服务端强制校验</div>
        </div>
        <span
          className={`chip ${
            level === "critical" ? "chip-bad" : level === "warning" ? "chip-warn" : "chip-good"
          }`}
        >
          <span
            className={`h-1.5 w-1.5 rounded-full ${
              level === "critical"
                ? "bg-[#d03b3b]"
                : level === "warning"
                  ? "bg-[#fab219]"
                  : "bg-[#0ca30c]"
            }`}
            aria-hidden="true"
          />
          {statusLabel}
        </span>
      </div>

      <div className="space-y-4 p-5">
        <div>
          <div className="mb-1.5 flex items-center justify-between text-[11px]">
            <span className="text-[var(--ink-2)]">保证金占用</span>
            <span className="num text-[var(--ink-2)]">{marginRatio.toFixed(1)}%</span>
          </div>
          <div className="risk-track">
            <div className={`risk-fill ${fill}`} style={{ width: `${marginRatio}%` }} />
          </div>
          <div className="mt-1.5 flex justify-between text-[10px] text-[var(--ink-3)]">
            <span>已用 {fmtUSD(marginUsed)}</span>
            <span>名义 {fmtUSD(notional)}</span>
          </div>
        </div>

        <div className="space-y-2.5 border-t border-[var(--line)] pt-4 text-[12px]">
          <Row label="持仓数量" value={`${positions.length} 个`} />
          <Row
            label="累计已实现"
            value={fmtSignedUSD(dailyPnL)}
            valueClass={toneClass(dailyPnL)}
          />
          <Row
            label="胜率 / 盈亏比"
            value={`${(perf?.win_rate ?? 0).toFixed(1)}% / ${
              perf?.profit_factor_infinite ? "∞" : (perf?.profit_factor ?? 0).toFixed(2)
            }`}
          />
          <Row
            label="总交易 / 最大单亏"
            value={`${perf?.total_trades ?? 0} / ${fmtUSD(perf?.max_loss ?? 0)}`}
          />
        </div>

        <div className="rounded-lg border border-[var(--line)] bg-[rgba(148,163,184,0.035)] px-3 py-2.5 text-[11px] leading-relaxed text-[var(--ink-3)]">
          所有开仓必须带止损；减仓与平仓不受日亏损限制。参数在服务端执行，AI 无法绕过。
        </div>
      </div>
    </section>
  );
}

function Row({
  label,
  value,
  valueClass = "text-[var(--ink-2)]",
}: {
  label: string;
  value: string;
  valueClass?: string;
}) {
  return (
    <div className="flex items-center justify-between gap-3">
      <span className="text-[var(--ink-3)]">{label}</span>
      <span className={`num font-medium ${valueClass}`}>{value}</span>
    </div>
  );
}
