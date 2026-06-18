import type { Position } from "../api/types";

const LIMITS = { maxPositionPercent: 0.05, maxStopLossPercent: 0.01, dailyLossLimit: 10 };

export function RiskPanel({ positions, balance, dailyPnL }: { positions: Position[]; balance: number; dailyPnL: number }) {
  const posUsed = positions.reduce((s, p) => s + p.Quantity * p.MarkPrice, 0);
  const posPct = balance > 0 ? (posUsed / (balance * LIMITS.maxPositionPercent)) * 100 : 0;
  const lossPct = balance > 0 ? (Math.abs(dailyPnL) / LIMITS.dailyLossLimit) * 100 : 0;

  return (
    <div className="backdrop-blur-xl bg-white/5 border border-white/10 rounded-xl p-4">
      <h3 className="text-sm font-semibold text-gray-300 mb-3">Risk Limits</h3>
      <div className="space-y-3">
        <RiskBar label="Position" used={posPct} max={LIMITS.maxPositionPercent * 100} unit="%" detail={`$${posUsed.toFixed(0)} / $${((balance||0)*LIMITS.maxPositionPercent).toFixed(0)}`} />
        <RiskBar label="Daily Loss" used={lossPct} max={100} unit="%" detail={`$${Math.abs(dailyPnL).toFixed(2)} / $${LIMITS.dailyLossLimit}`} />
        <div className="text-[11px] text-gray-500 mt-2 space-y-0.5">
          <div>Max Position: {LIMITS.maxPositionPercent * 100}% of balance</div>
          <div>Max Stop Loss: {LIMITS.maxStopLossPercent * 100}% of balance</div>
          <div>Daily Loss Limit: ${LIMITS.dailyLossLimit}</div>
        </div>
      </div>
    </div>
  );
}

function RiskBar({ label, used, max, unit, detail }: { label: string; used: number; max: number; unit: string; detail: string }) {
  const pct = Math.min(used, 100);
  const color = pct > 80 ? "bg-red-500" : pct > 50 ? "bg-yellow-500" : "bg-green-500";
  return (
    <div>
      <div className="flex justify-between text-xs mb-1">
        <span className="text-gray-400">{label}</span>
        <span className="text-gray-500">{detail}</span>
      </div>
      <div className="h-2 bg-gray-800 rounded-full overflow-hidden">
        <div className={`h-full rounded-full transition-all duration-700 ${color}`} style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}
