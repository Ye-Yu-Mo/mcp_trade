import type { Balance, Position, PerformanceStats } from "../api/types";

function StatBox({ label, value, color = "text-gray-100" }: { label: string; value: string; color?: string }) {
  return (
    <div className="min-w-[120px]">
      <div className="text-[11px] uppercase tracking-wider text-gray-500">{label}</div>
      <div className={`text-xl font-bold ${color}`}>{value}</div>
    </div>
  );
}

export function StatusBar({ balances, positions, perf }: { balances: Balance[]; positions: Position[]; perf: PerformanceStats | null }) {
  const usdt = balances.filter(b => ["USDT","USDC","BUSD"].includes(b.Asset)).reduce((s,b) => s + b.TotalBalance, 0);
  const unrealized = positions.reduce((s,p) => s + p.UnrealizedPnL, 0);
  const todayPnL = perf?.total_pnl ?? 0;
  const riskUsed = positions.length > 0 ? (positions.reduce((s,p) => s + p.Quantity * p.MarkPrice, 0) / (usdt||1)) * 100 : 0;
  const riskColor = riskUsed > 80 ? "#ef4444" : riskUsed > 50 ? "#f59e0b" : "#22c55e";

  return (
    <div className="flex flex-wrap justify-between gap-3 backdrop-blur-xl bg-white/5 border border-white/10 rounded-xl p-4 mb-4">
      <StatBox label="Balance" value={`$${usdt.toFixed(2)}`} />
      <StatBox label="Unrealized PnL" value={`${unrealized>=0?"+":""}${unrealized.toFixed(2)}`} color={unrealized>=0?"text-green-400":"text-red-400"} />
      <StatBox label="Today PnL" value={`${todayPnL>=0?"+":""}${todayPnL.toFixed(2)}`} color={todayPnL>=0?"text-green-400":"text-red-400"} />
      <div className="flex items-center gap-2">
        <span className="w-2.5 h-2.5 rounded-full" style={{background:riskColor}} />
        <span className="text-xs text-gray-500">RISK {riskUsed.toFixed(0)}%</span>
      </div>
      {perf && <StatBox label="Win Rate" value={`${perf.win_rate.toFixed(1)}%`} />}
    </div>
  );
}
