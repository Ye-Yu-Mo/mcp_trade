import type { Order } from "../api/types";

export function Orders({ orders }: { orders: Order[] }) {
  return (
    <div className="bg-slate-900/60 backdrop-blur-md border border-slate-700/50 rounded-2xl overflow-hidden shadow-xl">
      <div className="px-5 py-4 border-b border-slate-700/50 flex items-center justify-between bg-slate-800/20">
        <h3 className="text-sm font-medium text-slate-200">当前委托</h3>
        <span className="px-2 py-0.5 rounded text-xs font-medium border bg-slate-500/10 text-slate-400 border-slate-500/20">{orders.length} 挂单</span>
      </div>
      <div className="p-5 overflow-auto custom-scrollbar">
        {orders.length === 0 ? (
          <div className="h-40 flex items-center justify-center text-slate-500 text-sm">暂无挂单</div>
        ) : (
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="text-xs uppercase tracking-wider text-slate-500 border-b border-slate-700/50">
                <th className="pb-3 font-medium">ID</th>
                <th className="pb-3 font-medium">交易对</th>
                <th className="pb-3 font-medium">方向</th>
                <th className="pb-3 font-medium text-right">价格</th>
                <th className="pb-3 font-medium text-right">数量</th>
                <th className="pb-3 font-medium text-right">状态</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/50 text-sm">
              {orders.map(o => (
                <tr key={o.orderId} className="hover:bg-slate-800/20 transition-colors">
                  <td className="py-3 text-slate-500 font-mono text-xs">#{o.orderId}</td>
                  <td className="py-3 font-medium text-slate-200">{o.symbol}</td>
                  <td className="py-3">
                    <span className={o.side === "BUY" ? "text-emerald-400" : "text-rose-400"}>
                      {o.side === "BUY" ? "买入" : "卖出"}
                    </span>
                    <span className="text-slate-400 text-xs bg-slate-800 px-1.5 py-0.5 rounded ml-2">{o.type}</span>
                  </td>
                  <td className="py-3 text-right text-slate-300 font-mono">{o.price === "0" ? "市价" : "$" + o.price}</td>
                  <td className="py-3 text-right text-slate-300 font-mono">{o.origQty}</td>
                  <td className="py-3 text-right">
                    <span className={`px-2 py-0.5 rounded text-xs font-medium border ${o.status === "NEW" ? "bg-blue-500/10 text-blue-400 border-blue-500/20" : o.status === "FILLED" ? "bg-emerald-500/10 text-emerald-400 border-emerald-500/20" : "bg-slate-500/10 text-slate-400 border-slate-500/20"}`}>
                      {o.status}
                    </span>
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
