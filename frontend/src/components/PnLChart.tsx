import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer } from "recharts";
import type { TradeRecord } from "../api/types";

export function PnLChart({ trades }: { trades: TradeRecord[] }) {
  const filled = trades.filter(t => t.Status === "FILLED" && t.PnL !== 0).reverse();
  let cumulative = 0;
  const data = filled.map(t => {
    cumulative += t.PnL;
    return { time: t.CreatedAt.slice(5, 19).replace("T", " "), pnl: +cumulative.toFixed(2) };
  });

  return (
    <div className="backdrop-blur-xl bg-white/5 border border-white/10 rounded-xl p-4">
      <h3 className="text-sm font-semibold text-gray-300 mb-3">PnL Curve</h3>
      {data.length === 0 ? (
        <div className="text-gray-600 text-center py-8">No completed trades yet</div>
      ) : (
        <ResponsiveContainer width="100%" height={200}>
          <LineChart data={data}>
            <XAxis dataKey="time" tick={{fontSize:10,fill:"#6b7280"}} axisLine={false} tickLine={false} interval="preserveStartEnd" />
            <YAxis tick={{fontSize:10,fill:"#6b7280"}} axisLine={false} tickLine={false} width={60} />
            <Tooltip contentStyle={{background:"#1f2937",border:"1px solid #374151",borderRadius:8,fontSize:12}} labelStyle={{color:"#9ca3af"}} />
            <Line type="monotone" dataKey="pnl" stroke={cumulative >= 0 ? "#22c55e" : "#ef4444"} strokeWidth={2} dot={false} />
          </LineChart>
        </ResponsiveContainer>
      )}
    </div>
  );
}
