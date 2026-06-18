import type { Position } from "../api/types";

export function Positions({ positions }: { positions: Position[] }) {
  return (
    <div className="bg-slate-900/60 backdrop-blur-md border border-slate-700/50 rounded-2xl overflow-hidden shadow-xl">
      <div className="px-5 py-4 border-b border-slate-700/50 flex items-center justify-between bg-slate-800/20">
        <h3 className="text-sm font-medium text-slate-200">当前持仓</h3>
        <span className="px-2 py-0.5 rounded text-xs font-medium border bg-blue-500/10 text-blue-400 border-blue-500/20">{positions.length} 个标的</span>
      </div>
      <div className="p-5 overflow-auto custom-scrollbar">
        {positions.length === 0 ? (
          <div className="h-40 flex items-center justify-center text-slate-500 text-sm">暂无持仓</div>
        ) : (
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="text-xs uppercase tracking-wider text-slate-500 border-b border-slate-700/50">
                <th className="pb-3 font-medium">交易对</th>
                <th className="pb-3 font-medium">方向</th>
                <th className="pb-3 font-medium text-right">持仓量</th>
                <th className="pb-3 font-medium text-right">开仓均价</th>
                <th className="pb-3 font-medium text-right">标记价格</th>
                <th className="pb-3 font-medium text-right">未实现盈亏</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/50 text-sm">
              {positions.map(p => (
                <tr key={p.Symbol} className="hover:bg-slate-800/20 transition-colors">
                  <td className="py-3 font-medium text-slate-200">{p.Symbol}</td>
                  <td className="py-3">
                    <span className={`px-2 py-0.5 rounded text-xs font-medium border ${p.Side === "LONG" ? "bg-emerald-500/10 text-emerald-400 border-emerald-500/20" : "bg-rose-500/10 text-rose-400 border-rose-500/20"}`}>
                      {p.Side === "LONG" ? "做多" : "做空"} {p.Leverage}x
                    </span>
                  </td>
                  <td className="py-3 text-right text-slate-300 font-mono">{p.Quantity}</td>
                  <td className="py-3 text-right text-slate-300 font-mono">${p.EntryPrice.toFixed(2)}</td>
                  <td className="py-3 text-right text-slate-300 font-mono">${p.MarkPrice.toFixed(4)}</td>
                  <td className={`py-3 text-right font-mono font-medium ${p.UnrealizedPnL >= 0 ? "text-emerald-400" : "text-rose-400"}`}>
                    {p.UnrealizedPnL >= 0 ? "+" : ""}{p.UnrealizedPnL.toFixed(2)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
