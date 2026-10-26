import { fmtPrice, fmtTime } from "../lib/format";

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
  const sorted = alerts.slice().sort((a, b) => {
    if (a.triggered !== b.triggered) return a.triggered ? -1 : 1;
    return new Date(b.created_at).getTime() - new Date(a.created_at).getTime();
  });
  const active = alerts.filter((a) => !a.triggered).length;
  const triggered = alerts.length - active;

  return (
    <section className="panel overflow-hidden">
      <div className="panel-head">
        <div>
          <div className="panel-title">价格提醒</div>
          <div className="panel-sub mt-0.5">由服务端实时比较缓存价格</div>
        </div>
        <div className="flex items-center gap-1.5">
          {triggered > 0 && <span className="chip chip-bad">🔔 {triggered}</span>}
          <span className="chip">{active} 活跃</span>
        </div>
      </div>

      <div className="custom-scrollbar max-h-[286px] overflow-auto p-3">
        {alerts.length === 0 ? (
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
              <path d="M18 8a6 6 0 0 0-12 0c0 7-3 9-3 9h18s-3-2-3-9" />
              <path d="M13.7 21a2 2 0 0 1-3.4 0" />
            </svg>
            <div className="text-[var(--ink-2)]">暂无提醒</div>
            <div className="text-[11px] text-[var(--ink-3)]">等待关键价位触发</div>
          </div>
        ) : (
          <div className="space-y-2">
            {sorted.map((a) => (
              <div
                key={a.id}
                className={`flex items-start justify-between gap-3 rounded-xl border px-3 py-2.5 ${
                  a.triggered
                    ? "border-[rgba(208,59,59,0.28)] bg-[rgba(208,59,59,0.08)]"
                    : "border-[var(--line)] bg-[rgba(148,163,184,0.035)]"
                }`}
              >
                <div className="flex min-w-0 items-start gap-2.5">
                  <span
                    className={`mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full ${
                      a.triggered ? "bg-[#d03b3b]" : "bg-[var(--accent)]"
                    }`}
                    aria-hidden="true"
                  />
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-baseline gap-x-2">
                      <span className="text-[13px] font-medium text-[var(--ink)]">
                        {a.symbol}
                      </span>
                      <span className="num text-[12px] text-[var(--ink-2)]">
                        {a.direction === "ABOVE" ? "≥" : "≤"} ${fmtPrice(Number(a.price))}
                      </span>
                    </div>
                    <div className="mt-0.5 truncate text-[11px] text-[var(--ink-3)]">
                      {a.message || (a.triggered ? "已触发" : "等待中")}
                    </div>
                  </div>
                </div>
                <div className="shrink-0 text-right">
                  <div
                    className={`text-[10px] font-medium ${
                      a.triggered ? "text-[#ff9b9b]" : "text-[var(--ink-3)]"
                    }`}
                  >
                    {a.triggered ? "已触发" : "监听中"}
                  </div>
                  <div className="num mt-0.5 text-[10px] text-[var(--ink-3)]">
                    {fmtTime(a.created_at)}
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </section>
  );
}
