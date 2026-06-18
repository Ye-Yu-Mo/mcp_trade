import { useMemo } from "react";
import type { TradeRecord } from "../api/types";

export function PnLChart({ trades }: { trades: TradeRecord[] }) {
  const { path, area } = useMemo(() => {
    const filled = trades.filter(t => t.Status === "FILLED" && t.PnL !== 0).reverse();
    if (filled.length < 2) return { path: "", area: "" };

    let cumulative = 0;
    const values = filled.map(t => { cumulative += t.PnL; return cumulative; });

    const w = 400, h = 120;
    const max = Math.max(...values, Math.abs(values[0] || 1)) * 1.1;
    const min = Math.min(...values, -Math.abs(values[0] || 1) * 0.1) * 1.1;
    const range = max - min || 1;
    const step = w / Math.max(values.length - 1, 1);
    const getY = (val: number) => h - ((val - min) / range) * (h - 20) - 10;
    const points = values.map((v, i) => `${(i * step).toFixed(1)},${getY(v).toFixed(1)}`);
    const pathStr = `M ${points.join(" L ")}`;
    return { path: pathStr, area: `${pathStr} L ${w},${h} L 0,${h} Z` };
  }, [trades]);

  return (
    <div className="bg-slate-900/60 backdrop-blur-md border border-slate-700/50 rounded-2xl overflow-hidden shadow-xl h-[320px]">
      <div className="px-5 py-4 border-b border-slate-700/50 bg-slate-800/20">
        <h3 className="text-sm font-medium text-slate-200">资金曲线 (原生SVG)</h3>
      </div>
      <div className="p-5 flex-1 relative flex items-end pt-8 h-[calc(100%-53px)]">
        {!path ? (
          <div className="absolute inset-0 flex items-center justify-center text-slate-500 text-sm">暂无数据</div>
        ) : (
          <svg viewBox="0 0 400 120" className="w-full h-full overflow-visible" preserveAspectRatio="none">
            <defs>
              <linearGradient id="chartGrad" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor="#818cf8" stopOpacity="0.5" />
                <stop offset="100%" stopColor="#818cf8" stopOpacity="0" />
              </linearGradient>
            </defs>
            <line x1="0" y1="30" x2="400" y2="30" stroke="#334155" strokeDasharray="2 4" opacity="0.5" />
            <line x1="0" y1="60" x2="400" y2="60" stroke="#334155" strokeDasharray="2 4" opacity="0.5" />
            <line x1="0" y1="90" x2="400" y2="90" stroke="#334155" strokeDasharray="2 4" opacity="0.5" />
            <path d={area} fill="url(#chartGrad)" />
            <path d={path} fill="none" stroke="#818cf8" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" vectorEffect="non-scaling-stroke" />
          </svg>
        )}
      </div>
    </div>
  );
}
