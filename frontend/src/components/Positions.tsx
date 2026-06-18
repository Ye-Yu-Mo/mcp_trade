import type { Position } from "../api/types";

export function Positions({ positions }: { positions: Position[] }) {
  return (
    <div className="backdrop-blur-xl bg-white/5 border border-white/10 rounded-xl p-4">
      <h3 className="text-sm font-semibold text-gray-300 mb-3">Positions ({positions.length})</h3>
      {positions.length === 0 ? (
        <div className="text-gray-600 text-center py-8">No open positions</div>
      ) : (
        <table className="w-full text-left text-sm">
          <thead>
            <tr className="border-b border-gray-800 text-[11px] uppercase tracking-wider text-gray-500">
              <th className="pb-2 pr-3 font-medium">Symbol</th>
              <th className="pb-2 pr-3 font-medium">Side</th>
              <th className="pb-2 pr-3 font-medium">Qty</th>
              <th className="pb-2 pr-3 font-medium">Entry</th>
              <th className="pb-2 pr-3 font-medium">Mark</th>
              <th className="pb-2 font-medium">PnL</th>
            </tr>
          </thead>
          <tbody>
            {positions.map(p => (
              <tr key={p.Symbol} className="border-b border-gray-800/50">
                <td className="py-2 pr-3 text-gray-200 font-mono">{p.Symbol}</td>
                <td className={`py-2 pr-3 font-mono ${p.Side==="LONG"?"text-green-400":"text-red-400"}`}>{p.Side==="LONG"?"L":"S"}</td>
                <td className="py-2 pr-3 text-gray-300 font-mono">{p.Quantity}</td>
                <td className="py-2 pr-3 text-gray-300 font-mono">${p.EntryPrice.toFixed(2)}</td>
                <td className="py-2 pr-3 text-gray-300 font-mono">${p.MarkPrice.toFixed(2)}</td>
                <td className={`py-2 font-mono ${p.UnrealizedPnL>=0?"text-green-400":"text-red-400"}`}>{p.UnrealizedPnL>=0?"+":""}{p.UnrealizedPnL.toFixed(2)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
