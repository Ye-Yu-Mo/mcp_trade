import { useState, useCallback, useRef } from "react";
import { api } from "./api/client";
import { usePolling } from "./hooks/usePolling";
import { StatusBar } from "./components/StatusBar";
import { Positions } from "./components/Positions";
import { Orders } from "./components/Orders";
import { PnLChart } from "./components/PnLChart";
import { RiskPanel } from "./components/RiskPanel";
import { RecentTrades } from "./components/RecentTrades";
import { ConfigForm } from "./components/ConfigForm";
import { Alerts } from "./components/Alerts";
import type { Balance, Position, Order, TradeRecord, PerformanceStats } from "./api/types";
import type { Alert } from "./components/Alerts";
import { fmtDateTime } from "./lib/format";

export default function App() {
  const [connected, setConnected] = useState(api.configured);
  const [balances, setBalances] = useState<Balance[]>([]);
  const [positions, setPositions] = useState<Position[]>([]);
  const [orders, setOrders] = useState<Order[]>([]);
  const [trades, setTrades] = useState<TradeRecord[]>([]);
  const [perf, setPerf] = useState<PerformanceStats | null>(null);
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);
  const [refreshing, setRefreshing] = useState(false);
  const hasLoaded = useRef(false);

  const fetchData = useCallback(async () => {
    setRefreshing(true);
    const [b, p, o, t, pf, a] = await Promise.all([
      api.balance(),
      api.positions(),
      api.openOrders(),
      api.tradeHistory(),
      api.performance(),
      api.alerts(),
    ]);
    const any = b || p || o || t || pf || a;
    if (any) {
      hasLoaded.current = true;
      setLastUpdated(new Date());
    } else if (!hasLoaded.current) {
      // A persisted token may be invalid; return to the login form instead
      // of showing an empty dashboard forever.
      setConnected(false);
    }
    if (b) setBalances(b);
    if (p) setPositions(p);
    if (o) setOrders(o);
    if (t) setTrades(t);
    if (pf) setPerf(pf);
    if (a) setAlerts(a);
    setRefreshing(false);
  }, []);

  usePolling(fetchData, 5000, connected);

  if (!connected) {
    return <ConfigForm onConnect={() => setConnected(true)} />;
  }

  const usdt = balances
    .filter((b) => ["USDT", "USDC", "BUSD"].includes(b.Asset))
    .reduce((s, b) => s + b.TotalBalance, 0);
  const online = lastUpdated !== null;
  const stale = lastUpdated !== null && Date.now() - lastUpdated.getTime() > 15000;

  return (
    <div className="min-h-screen">
      <header className="sticky top-0 z-20 border-b border-[var(--line)] bg-[rgba(6,10,17,0.78)] backdrop-blur-xl">
        <div className="mx-auto flex max-w-[1440px] flex-wrap items-center justify-between gap-3 px-4 py-3.5 md:px-6 lg:px-8">
          <div className="flex items-center gap-3">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl border border-[rgba(57,135,229,0.35)] bg-[var(--accent-soft)] text-[#9cc4ff]">
              <svg
                viewBox="0 0 24 24"
                className="h-[18px] w-[18px]"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
              >
                <polyline points="22 12 18 12 15 21 9 3 6 12 2 12" />
              </svg>
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h1 className="text-[15px] font-semibold tracking-tight text-[var(--ink)]">
                  MCP Trade
                </h1>
                <span className="chip hidden sm:inline-flex">BINANCE FUTURES</span>
              </div>
              <div className="mt-0.5 text-[11px] text-[var(--ink-3)]">
                主观交易终端 · 5s 轮询
              </div>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <span className={`chip ${online ? (stale ? "chip-warn" : "chip-good") : "chip-warn"}`}>
              <span
                className={`h-1.5 w-1.5 rounded-full ${
                  online ? (stale ? "bg-[#fab219]" : "live-dot") : "bg-[#fab219]"
                }`}
                aria-hidden="true"
              />
              {online ? (stale ? "数据延迟" : "引擎在线") : "等待连接"}
            </span>
            <span className="hidden text-[11px] text-[var(--ink-3)] md:inline">
              {lastUpdated ? fmtDateTime(lastUpdated.toISOString()) : "—"}
            </span>
            <button
              type="button"
              onClick={() => void fetchData()}
              disabled={refreshing}
              title="立即刷新"
              className="rounded-lg border border-[var(--line)] p-2 text-[var(--ink-2)] transition-colors hover:border-[var(--line-strong)] hover:text-[var(--ink)] disabled:opacity-50"
            >
              <svg
                viewBox="0 0 24 24"
                className={`h-4 w-4 ${refreshing ? "animate-spin" : ""}`}
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
              >
                <path d="M21 12a9 9 0 1 1-2.64-6.36" />
                <polyline points="21 3 21 9 15 9" />
              </svg>
            </button>
            <button
              type="button"
              onClick={() => setConnected(false)}
              className="rounded-lg border border-[var(--line)] px-3 py-2 text-[12px] text-[var(--ink-2)] transition-colors hover:border-[var(--line-strong)] hover:text-[var(--ink)]"
            >
              断开
            </button>
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-[1440px] space-y-4 px-4 py-5 md:px-6 lg:px-8 lg:py-6">
        <StatusBar balances={balances} positions={positions} perf={perf} />

        <div className="grid grid-cols-1 gap-4 xl:grid-cols-12">
          <div className="space-y-4 xl:col-span-8">
            <Positions positions={positions} />
            <Orders orders={orders} />
            <RecentTrades trades={trades} />
          </div>
          <div className="space-y-4 xl:col-span-4">
            <PnLChart trades={trades} />
            <Alerts alerts={alerts} />
            <RiskPanel
              positions={positions}
              balance={usdt}
              dailyPnL={perf?.total_pnl ?? 0}
              perf={perf}
            />
          </div>
        </div>

        <footer className="flex flex-wrap items-center justify-between gap-2 pt-1 text-[10px] text-[var(--ink-3)]">
          <span>数据每 5 秒刷新 · 只读看板不会下单</span>
          <span>交易有风险，AI 决策不保证盈利</span>
        </footer>
      </main>
    </div>
  );
}
