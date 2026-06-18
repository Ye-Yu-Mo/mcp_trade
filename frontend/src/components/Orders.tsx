import type { Order } from "../api/types";

export function Orders({ orders }: { orders: Order[] }) {
  return (
    <div className="backdrop-blur-xl bg-white/5 border border-white/10 rounded-xl p-4">
      <h3 className="text-sm font-semibold text-gray-300 mb-3">Orders ({orders.length})</h3>
      {orders.length === 0 ? (
        <div className="text-gray-600 text-center py-8">No open orders</div>
      ) : (
        <table className="w-full text-left text-sm">
          <thead>
            <tr className="border-b border-gray-800 text-[11px] uppercase tracking-wider text-gray-500">
              <th className="pb-2 pr-2 font-medium">ID</th>
              <th className="pb-2 pr-2 font-medium">Symbol</th>
              <th className="pb-2 pr-2 font-medium">Side</th>
              <th className="pb-2 pr-2 font-medium">Type</th>
              <th className="pb-2 pr-2 font-medium">Qty</th>
              <th className="pb-2 pr-2 font-medium">Price</th>
              <th className="pb-2 font-medium">Status</th>
            </tr>
          </thead>
          <tbody>
            {orders.map(o => (
              <tr key={o.orderId} className="border-b border-gray-800/50">
                <td className="py-2 pr-2 text-gray-500 font-mono text-xs">#{o.orderId}</td>
                <td className="py-2 pr-2 text-gray-200 font-mono">{o.symbol}</td>
                <td className={`py-2 pr-2 font-mono ${o.side==="BUY"?"text-green-400":"text-red-400"}`}>{o.side}</td>
                <td className="py-2 pr-2 text-gray-300 font-mono">{o.type}</td>
                <td className="py-2 pr-2 text-gray-300 font-mono">{o.origQty}</td>
                <td className="py-2 pr-2 text-gray-300 font-mono">{o.price==="0"?"MKT":o.price}</td>
                <td className={`py-2 font-mono text-xs ${o.status==="NEW"?"text-blue-400":o.status==="FILLED"?"text-green-400":"text-gray-500"}`}>{o.status}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
