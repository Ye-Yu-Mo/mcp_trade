import type { Balance, Position, PerformanceStats } from "../api/types";
import { fmtSignedUSD, fmtUSD, toneClass } from "../lib/format";

type Tone = "good" | "bad" | "warn" | "neutral";

export function StatusBar({
  balances,
  positions,
  perf,
}: {
  balances: Balance[];
  positions: Position[];
  perf: PerformanceStats | null;
}) {
  const usdt = balances
    .filter((b) => ["USDT", "USDC", "BUSD"].includes(b.Asset))
    .reduce((s, b) => s + b.TotalBalance, 0);
  const available = balances
    .filter((b) => ["USDT", "USDC", "BUSD"].includes(b.Asset))
    .reduce((s, b) => s + b.AvailableBalance, 0);
  const unrealized = positions.reduce((s, p) => s + p.UnrealizedPnL, 0);
  const equity = usdt + unrealized;
  const todayPnL = perf?.total_pnl ?? 0;
  const marginUsed = positions.reduce(
    (s, p) => s + (p.Quantity * p.MarkPrice) / (p.Leverage || 1),
    0,
  );
  const marginRatio = usdt > 0 ? Math.min((marginUsed / usdt) * 100, 100) : 0;
  const marginTone: Tone = marginRatio > 80 ? "bad" : marginRatio > 50 ? "warn" : "good";

  return (
    <div className="grid grid-cols-1 gap-4 xl:grid-cols-12">
      <section className="panel relative overflow-hidden p-5 xl:col-span-5">
        <div className="pointer-events-none absolute -right-20 -top-24 h-56 w-56 rounded-full bg-[var(--accent)]/10 blur-3xl" />
        <div className="relative">
          <div className="flex items-center justify-between gap-3">
            <span className="metric-label">账户总权益</span>
            <span className="chip chip-accent">USDT 永续</span>
          </div>
          <div className="hero-num mt-3 text-[46px] font-semibold leading-none text-[var(--ink)]">
            {fmtUSD(equity)}
          </div>
          <div className="mt-4 flex flex-wrap items-center gap-x-5 gap-y-1.5 text-[11px] text-[var(--ink-3)]">
            <span>
              可用 <b className="num font-medium text-[var(--ink-2)]">{fmtUSD(available)}</b>
            </span>
            <span>
              保证金{" "}
              <b className="num font-medium text-[var(--ink-2)]">{fmtUSD(marginUsed)}</b>
            </span>
            <span>
              持仓 <b className="num font-medium text-[var(--ink-2)]">{positions.length}</b>
            </span>
          </div>
        </div>
      </section>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3 xl:col-span-7">
        <Stat
          label="未实现盈亏"
          value={fmtSignedUSD(unrealized)}
          valueClass={toneClass(unrealized)}
          sub={`${positions.length} 个持仓`}
          tone={unrealized >= 0 ? "good" : "bad"}
        />
        <Stat
          label="累计已实现"
          value={fmtSignedUSD(todayPnL)}
          valueClass={toneClass(todayPnL)}
          sub={`胜率 ${(perf?.win_rate ?? 0).toFixed(1)}%`}
          tone={todayPnL >= 0 ? "good" : "bad"}
        />
        <Stat
          label="保证金水位"
          value={`${marginRatio.toFixed(1)}%`}
          valueClass="text-[var(--ink)]"
          sub={`已用 ${fmtUSD(marginUsed)}`}
          tone={marginTone}
        />
      </div>
    </div>
  );
}

function Stat({
  label,
  value,
  sub,
  tone,
  valueClass,
}: {
  label: string;
  value: string;
  sub: string;
  tone: Tone;
  valueClass: string;
}) {
  const dot: Record<Tone, string> = {
    good: "bg-[#0ca30c]",
    bad: "bg-[#d03b3b]",
    warn: "bg-[#fab219]",
    neutral: "bg-[var(--accent)]",
  };

  return (
    <div className="panel flex min-h-[118px] flex-col justify-between p-4">
      <div className="flex items-center justify-between gap-2">
        <span className="metric-label">{label}</span>
        <span className={`h-1.5 w-1.5 rounded-full ${dot[tone]}`} aria-hidden="true" />
      </div>
      <div className={`metric-value mt-3 ${valueClass}`}>{value}</div>
      <div className="mt-1 text-[11px] text-[var(--ink-3)]">{sub}</div>
    </div>
  );
}
