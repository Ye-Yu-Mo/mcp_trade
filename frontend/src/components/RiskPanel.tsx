import type { Position } from "../api/types";

export function RiskPanel({ positions, balance, dailyPnL }: { positions: Position[]; balance: number; dailyPnL: number }) {
  const marginUsed = positions.reduce((s, p) => s + (p.Quantity * p.MarkPrice / (p.Leverage || 1)), 0);
  const marginRatio = balance > 0 ? Math.min((marginUsed / balance) * 100, 100) : 0;
  const marginColor = marginRatio > 80 ? "bg-rose-500 shadow-[0_0_10px_rgba(244,63,94,0.5)]" : marginRatio > 50 ? "bg-amber-500 shadow-[0_0_10px_rgba(245,158,11,0.5)]" : "bg-emerald-500 shadow-[0_0_10px_rgba(16,185,129,0.5)]";

  return (
    <div className="bg-slate-900/60 backdrop-blur-md border border-slate-700/50 rounded-2xl overflow-hidden shadow-xl">
      <div className="px-5 py-4 border-b border-slate-700/50 flex items-center justify-between bg-slate-800/20">
        <h3 className="text-sm font-medium text-slate-200">引擎健康度 & 风控</h3>
      </div>
      <div className="p-5 space-y-5">
        <div>
          <div className="flex justify-between text-xs mb-1.5">
            <span className="text-slate-300">保证金水位 (Position Limit)</span>
            <span className="text-slate-400 font-mono">{marginRatio.toFixed(1)}%</span>
          </div>
          <div className="h-1.5 bg-slate-800 rounded-full overflow-hidden">
            <div className={`h-full rounded-full transition-all duration-1000 ease-out ${marginColor}`} style={{ width: `${marginRatio}%` }} />
          </div>
        </div>
        <div className="pt-5 border-t border-slate-800 space-y-2 text-sm">
          <div className="flex justify-between">
            <span className="text-slate-500">API 节点延迟</span>
            <span className="text-emerald-400 flex items-center gap-1">
              <svg className="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg> 在线
            </span>
          </div>
          <div className="flex justify-between">
            <span className="text-slate-500">风控状态</span>
            <span className={marginRatio > 80 ? "text-rose-400" : "text-emerald-400"}>
              {marginRatio > 80 ? "⚠️ 警告" : "✅ 正常"}
            </span>
          </div>
          <div className="flex justify-between">
            <span className="text-slate-500">今日盈亏</span>
            <span className={dailyPnL >= 0 ? "text-emerald-400" : "text-rose-400"}>${dailyPnL.toFixed(2)}</span>
          </div>
        </div>
      </div>
    </div>
  );
}
