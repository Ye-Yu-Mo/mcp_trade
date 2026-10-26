import type { Position } from "../api/types";
import { fmtPct, fmtPrice, fmtQty, fmtSignedUSD, fmtUSD, toneClass } from "../lib/format";

export function Positions({ positions }: { positions: Position[] }) {
  return (
    <section className="panel overflow-hidden">
      <div className="panel-head">
        <div>
          <div className="panel-title">当前持仓</div>
          <div className="panel-sub mt-0.5">标记价格实时更新</div>
        </div>
        <span className="chip">{positions.length} 个标的</span>
      </div>

      <div className="custom-scrollbar overflow-x-auto px-5 pb-5 pt-4">
        {positions.length === 0 ? (
          <EmptyState
            title="暂无持仓"
            hint="等待引擎给出符合交易计划的入场信号"
          />
        ) : (
          <table className="table min-w-[760px]">
            <thead>
              <tr>
                <th>交易对</th>
                <th>方向</th>
                <th className="text-right">持仓量</th>
                <th className="text-right">开仓均价</th>
                <th className="text-right">标记价格</th>
                <th className="text-right">名义价值</th>
                <th className="text-right">未实现盈亏</th>
              </tr>
            </thead>
            <tbody>
              {positions.map((p) => {
                const long = p.Side === "LONG";
                const pnlPct =
                  p.EntryPrice > 0
                    ? ((p.MarkPrice - p.EntryPrice) / p.EntryPrice) * 100 * (long ? 1 : -1)
                    : 0;
                const notional = p.Quantity * p.MarkPrice;
                return (
                  <tr key={`${p.Symbol}-${p.Side}`}>
                    <td className="font-medium text-[var(--ink)]">{p.Symbol}</td>
                    <td>
                      <span className={`chip ${long ? "chip-good" : "chip-bad"}`}>
                        {long ? "做多" : "做空"} · {p.Leverage || 1}x
                      </span>
                    </td>
                    <td className="num text-right text-[var(--ink-2)]">{fmtQty(p.Quantity)}</td>
                    <td className="num text-right text-[var(--ink-2)]">
                      ${fmtPrice(p.EntryPrice)}
                    </td>
                    <td className="num text-right text-[var(--ink-2)]">
                      ${fmtPrice(p.MarkPrice)}
                    </td>
                    <td className="num text-right text-[var(--ink-3)]">{fmtUSD(notional)}</td>
                    <td className="text-right">
                      <div className={`num font-medium ${toneClass(p.UnrealizedPnL)}`}>
                        {fmtSignedUSD(p.UnrealizedPnL)}
                      </div>
                      <div className={`num mt-0.5 text-[11px] ${toneClass(pnlPct)}`}>
                        {fmtPct(pnlPct)}
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>
    </section>
  );
}

function EmptyState({ title, hint }: { title: string; hint: string }) {
  return (
    <div className="empty">
      <svg
        viewBox="0 0 24 24"
        className="h-6 w-6 text-[var(--ink-3)]"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
      >
        <path d="M3 3v18h18" />
        <path d="M7 15l4-4 3 3 5-6" />
      </svg>
      <div className="text-[var(--ink-2)]">{title}</div>
      <div className="text-[11px] text-[var(--ink-3)]">{hint}</div>
    </div>
  );
}
