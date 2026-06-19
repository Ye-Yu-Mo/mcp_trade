export interface Alert {
  id: string;
  symbol: string;
  price: number;
  direction: string;
  message: string;
  triggered: boolean;
  created_at: string;
}

export function Alerts({ alerts }: { alerts: Alert[] }) {
  const active = alerts.filter(a => !a.triggered);
  const triggered = alerts.filter(a => a.triggered);

  return (
    <div className="bg-slate-900/60 backdrop-blur-md border border-slate-700/50 rounded-2xl overflow-hidden shadow-xl">
      <div className="px-5 py-4 border-b border-slate-700/50 flex items-center justify-between bg-slate-800/20">
        <h3 className="text-sm font-medium text-slate-200">价格提醒</h3>
        <div className="flex gap-2">
          {triggered.length > 0 && (
            <span className="px-2 py-0.5 rounded text-xs font-medium border bg-rose-500/10 text-rose-400 border-rose-500/20">
              🔔 {triggered.length} 已触发
            </span>
          )}
          <span className="px-2 py-0.5 rounded text-xs font-medium border bg-blue-500/10 text-blue-400 border-blue-500/20">
            {active.length} 活跃
          </span>
        </div>
      </div>
      <div className="p-4 overflow-auto custom-scrollbar max-h-[280px]">
        {alerts.length === 0 ? (
          <div className="text-slate-500 text-sm text-center py-6">暂无提醒</div>
        ) : (
          <div className="space-y-2">
            {triggered.map(a => (
              <div key={a.id} className="flex items-center justify-between p-2.5 rounded-lg bg-rose-500/10 border border-rose-500/20">
                <div className="flex items-center gap-3">
                  <span className="text-lg">🔔</span>
                  <div>
                    <div className="text-sm font-medium text-slate-200">{a.symbol} {a.direction} ${a.price}</div>
                    <div className="text-xs text-rose-400">{a.message || "已触发"}</div>
                  </div>
                </div>
                <span className="text-xs text-rose-400 font-mono">触发</span>
              </div>
            ))}
            {active.map(a => (
              <div key={a.id} className="flex items-center justify-between p-2.5 rounded-lg bg-slate-800/30 border border-slate-700/30">
                <div className="flex items-center gap-3">
                  <span className="text-lg">⏳</span>
                  <div>
                    <div className="text-sm font-medium text-slate-300">{a.symbol} {a.direction} ${a.price}</div>
                    <div className="text-xs text-slate-500">{a.message || "等待中"}</div>
                  </div>
                </div>
                <span className="text-xs text-slate-600 font-mono">#{a.id}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
