import { useState, useCallback } from "react";
import { api } from "./api/client";
import { usePolling } from "./hooks/usePolling";
import { StatusBar } from "./components/StatusBar";
import { Positions } from "./components/Positions";
import { Orders } from "./components/Orders";
import { PnLChart } from "./components/PnLChart";
import { RiskPanel } from "./components/RiskPanel";
import { ConfigForm } from "./components/ConfigForm";
import type { Balance, Position, Order, TradeRecord, PerformanceStats } from "./api/types";
import type { Alert } from "./components/Alerts";
import { Alerts } from "./components/Alerts";

export default function App() {
  const [connected, setConnected] = useState(api.configured);
  const [balances, setBalances] = useState<Balance[]>([]);
  const [positions, setPositions] = useState<Position[]>([]);
  const [orders, setOrders] = useState<Order[]>([]);
  const [trades, setTrades] = useState<TradeRecord[]>([]);
  const [perf, setPerf] = useState<PerformanceStats | null>(null);
  const [alerts, setAlerts] = useState<Alert[]>([]);

  const fetch = useCallback(async () => {
    const [b, p, o, t, pf, a] = await Promise.all([
      api.balance(), api.positions(), api.openOrders(), api.tradeHistory(), api.performance(), api.getAlerts()
    ]);
    if (b) setBalances(b);
    if (p) setPositions(p);
    if (o) setOrders(o);
    if (t) setTrades(t);
    if (pf) setPerf(pf);
    if (a) setAlerts(a);
  }, []);

  usePolling(fetch, 5000);

  if (!connected) {
    return <ConfigForm onConnect={() => setConnected(true)} />;
  }

  const usdt = balances.filter(b => ["USDT","USDC","BUSD"].includes(b.Asset)).reduce((s,b) => s + b.TotalBalance, 0);

  return (
    <div className="min-h-screen bg-slate-950 text-slate-300 font-sans p-4 md:p-6 lg:p-8">
      <div className="max-w-[1400px] mx-auto space-y-6">
        {/* Header */}
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 bg-indigo-500/20 text-indigo-400 rounded-xl flex items-center justify-center border border-indigo-500/30">
              <svg viewBox="0 0 24 24" className="w-5 h-5" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>
            </div>
            <div>
              <h1 className="text-xl font-bold text-slate-100">量化交易控制台</h1>
              <div className="flex items-center gap-2 text-xs text-emerald-400 mt-0.5">
                <span className="relative flex h-2 w-2"><span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"/><span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"/></span>
                MCP 信号引擎 · 5s 轮询
              </div>
            </div>
          </div>
          <button onClick={() => setConnected(false)} className="flex items-center gap-2 text-sm text-slate-400 hover:text-slate-200 transition-colors px-4 py-2 rounded-lg hover:bg-slate-800">
            <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" y1="12" x2="9" y2="12"/></svg>断开
          </button>
        </div>

        {/* Stat cards */}
        <StatusBar balances={balances} positions={positions} perf={perf} />

        {/* Main grid */}
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          <div className="lg:col-span-2 space-y-6">
            <Positions positions={positions} />
            <Orders orders={orders} />
          </div>
          <div className="space-y-6">
            <PnLChart trades={trades} />
            <Alerts alerts={alerts} />
            <RiskPanel positions={positions} balance={usdt} dailyPnL={perf?.total_pnl ?? 0} />
          </div>
        </div>
      </div>
    </div>
  );
}
