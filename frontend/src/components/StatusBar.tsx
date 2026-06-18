import type { Balance, Position, PerformanceStats } from "../api/types";

export function StatusBar({ balances, positions, perf }: { balances: Balance[]; positions: Position[]; perf: PerformanceStats | null }) {
  const usdt = balances.filter(b => ["USDT","USDC","BUSD"].includes(b.Asset)).reduce((s,b) => s + b.TotalBalance, 0);
  const unrealized = positions.reduce((s,p) => s + p.UnrealizedPnL, 0);
  const todayPnL = perf?.total_pnl ?? 0;
  const marginUsed = positions.reduce((s,p) => s + (p.Quantity * p.MarkPrice / (p.Leverage||1)), 0);
  const marginRatio = usdt > 0 ? Math.min((marginUsed / usdt) * 100, 100) : 0;
  const marginColor = marginRatio > 80 ? "text-rose-400" : marginRatio > 50 ? "text-amber-400" : "text-emerald-400";

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
      <Stat icon={<WalletIcon />} label="总资产 (USDT)" value={`$${(usdt + unrealized).toFixed(2)}`} sub={`可用 $${usdt.toFixed(2)}`} />
      <Stat icon={<ChartIcon />} label="未实现盈亏" value={`${unrealized>=0?"+":""}${unrealized.toFixed(2)}`} sub="实时计算中" color={unrealized>=0?"text-emerald-400":"text-rose-400"} />
      <Stat icon={<BarIcon />} label="今日已实现" value={`${todayPnL>=0?"+":""}${todayPnL.toFixed(2)}`} sub={`胜率 ${perf?.win_rate.toFixed(1) ?? "0.0"}%`} color={todayPnL>=0?"text-emerald-400":"text-rose-400"} />
      <Stat icon={<ShieldIcon />} label="保证金水位" value={`${marginRatio.toFixed(1)}%`} sub={`已用 $${marginUsed.toFixed(2)}`} color={marginColor} />
    </div>
  );
}

function Stat({ icon, label, value, sub, color = "text-slate-100" }: { icon: React.ReactNode; label: string; value: string; sub: string; color?: string }) {
  return (
    <div className="p-4 rounded-xl bg-slate-800/30 border border-slate-700/50 flex items-start gap-4">
      <div className="p-3 rounded-lg bg-slate-900 text-indigo-400">{icon}</div>
      <div>
        <div className="text-xs text-slate-400 mb-1">{label}</div>
        <div className={`text-xl font-bold tracking-tight ${color}`}>{value}</div>
        <div className="text-xs text-slate-500 mt-1">{sub}</div>
      </div>
    </div>
  );
}

function WalletIcon() { return <svg className="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M20 12V8H6a2 2 0 0 1-2-2c0-1.1.9-2 2-2h12v4"/><path d="M4 6v12c0 1.1.9 2 2 2h14v-4"/><path d="M18 12a2 2 0 0 0-2 2c0 1.1.9 2 2 2h4v-4h-4z"/></svg>; }
function ChartIcon() { return <svg className="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>; }
function BarIcon() { return <svg className="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><line x1="12" y1="20" x2="12" y2="10"/><line x1="18" y1="20" x2="18" y2="4"/><line x1="6" y1="20" x2="6" y2="16"/></svg>; }
function ShieldIcon() { return <svg className="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>; }
