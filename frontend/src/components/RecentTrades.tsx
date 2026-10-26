import type { TradeRecord } from "../api/types";
import { fmtPrice, fmtQty, fmtSignedUSD, fmtTime, toneClass } from "../lib/format";

export function RecentTrades({ trades }: { trades: TradeRecord[] }) {
  const rows = trades.slice(0, 8);

  return (
    <section className="panel overflow-hidden">
      <div className="panel-head">
        <div>
          <div className="panel-title">最近成交</div>
          <div className="panel-sub mt-0.5">来自服务端交易记录</div>
        </div>
        <span className="chip">{trades.length} 笔</span>
      </div>

      <div className="custom-scrollbar overflow-x-auto px-5 pb-5 pt-4">
        {rows.length === 0 ? (
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
              <circle cx="12" cy="12" r="9" />
              <path d="M12 7v5l3 2" />
            </svg>
            <div className="text-[var(--ink-2)]">还没有成交记录</div>
            <div className="text-[11px] text-[var(--ink-3)]">第一笔交易会出现在这里</div>
          </div>
        ) : (
          <table className="table min-w-[720px]">
            <thead>
              <tr>
                <th>时间</th>
                <th>交易对</th>
                <th>方向</th>
                <th className="text-right">成交价</th>
                <th className="text-right">数量</th>
                <th className="text-right">状态</th>
                <th className="text-right">盈亏</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((t) => {
                const buy = t.Side === "BUY" || t.Side === "LONG";
                return (
                  <tr key={t.ID}>
                    <td className="num text-[11px] text-[var(--ink-3)]">
                      {fmtTime(t.CreatedAt)}
                    </td>
                    <td className="font-medium text-[var(--ink)]">{t.Symbol}</td>
                    <td className={buy ? "text-[#3ddc84]" : "text-[#ff7a7a]"}>
                      {buy ? "买入" : "卖出"}
                    </td>
                    <td className="num text-right text-[var(--ink-2)]">
                      ${fmtPrice(t.Price)}
                    </td>
                    <td className="num text-right text-[var(--ink-2)]">{fmtQty(t.Quantity)}</td>
                    <td className="text-right">
                      <span
                        className={`chip ${
                          t.Status === "CLOSED" || t.Status === "FILLED"
                            ? "chip-good"
                            : t.Status === "OPEN"
                              ? "chip-accent"
                              : ""
                        }`}
                      >
                        {t.Status}
                      </span>
                    </td>
                    <td className={`num text-right font-medium ${toneClass(t.PnL)}`}>
                      {t.PnL === 0 ? "—" : fmtSignedUSD(t.PnL)}
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
