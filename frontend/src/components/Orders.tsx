import type { Order } from "../api/types";
import { fmtPrice, fmtQty } from "../lib/format";

export function Orders({ orders }: { orders: Order[] }) {
  return (
    <section className="panel overflow-hidden">
      <div className="panel-head">
        <div>
          <div className="panel-title">当前委托</div>
          <div className="panel-sub mt-0.5">包含限价单与条件单</div>
        </div>
        <span className="chip">{orders.length} 挂单</span>
      </div>

      <div className="custom-scrollbar overflow-x-auto px-5 pb-5 pt-4">
        {orders.length === 0 ? (
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
              <path d="M8 6h13M8 12h13M8 18h13" />
              <path d="M3 6h.01M3 12h.01M3 18h.01" />
            </svg>
            <div className="text-[var(--ink-2)]">暂无挂单</div>
            <div className="text-[11px] text-[var(--ink-3)]">入场与保护单会显示在这里</div>
          </div>
        ) : (
          <table className="table min-w-[720px]">
            <thead>
              <tr>
                <th>订单</th>
                <th>交易对</th>
                <th>方向 / 类型</th>
                <th className="text-right">价格 / 触发</th>
                <th className="text-right">数量</th>
                <th className="text-right">状态</th>
              </tr>
            </thead>
            <tbody>
              {orders.map((o) => {
                const stop = Number(o.stopPrice || 0);
                const price = Number(o.price || 0);
                const conditional = /STOP|TAKE_PROFIT|TRAILING/i.test(o.type);
                const priceLabel =
                  conditional && stop > 0
                    ? `触发 $${fmtPrice(stop)}`
                    : price > 0
                      ? `$${fmtPrice(price)}`
                      : "市价";
                const buy = o.side === "BUY";
                return (
                  <tr key={o.orderId}>
                    <td>
                      <div className="num text-[11px] text-[var(--ink-3)]">#{o.orderId}</div>
                      {o.reduceOnly && (
                        <div className="mt-1 text-[10px] uppercase tracking-wider text-[var(--ink-3)]">
                          reduce only
                        </div>
                      )}
                    </td>
                    <td className="font-medium text-[var(--ink)]">{o.symbol}</td>
                    <td>
                      <span className={buy ? "text-[#3ddc84]" : "text-[#ff7a7a]"}>
                        {buy ? "买入" : "卖出"}
                      </span>
                      <span className="ml-2 rounded bg-[rgba(148,163,184,0.08)] px-1.5 py-0.5 text-[10px] text-[var(--ink-2)]">
                        {o.type}
                      </span>
                    </td>
                    <td className="num text-right text-[var(--ink-2)]">{priceLabel}</td>
                    <td className="num text-right text-[var(--ink-2)]">{fmtQty(Number(o.origQty))}</td>
                    <td className="text-right">
                      <StatusBadge status={o.status} />
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

function StatusBadge({ status }: { status: string }) {
  const cls =
    status === "NEW"
      ? "chip-accent"
      : status === "PARTIALLY_FILLED"
        ? "chip-warn"
        : status === "FILLED"
          ? "chip-good"
          : "";
  return <span className={`chip ${cls}`}>{status}</span>;
}
