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

export default function App() {
  const [connected, setConnected] = useState(api.configured);
  const [balances, setBalances] = useState<Balance[]>([]);
  const [positions, setPositions] = useState<Position[]>([]);
  const [orders, setOrders] = useState<Order[]>([]);
  const [trades, setTrades] = useState<TradeRecord[]>([]);
  const [perf, setPerf] = useState<PerformanceStats | null>(null);

  const fetch = useCallback(async () => {
    const [b, p, o, t, pf] = await Promise.all([
      api.balance(), api.positions(), api.openOrders(), api.tradeHistory(), api.performance()
    ]);
    if (b) setBalances(b);
    if (p) setPositions(p);
    if (o) setOrders(o);
    if (t) setTrades(t);
    if (pf) setPerf(pf);
  }, []);

  usePolling(fetch, 5000);

  if (!connected) {
    return <ConfigForm onConnect={() => setConnected(true)} />;
  }

  const usdt = balances.filter(b => ["USDT","USDC","BUSD"].includes(b.Asset)).reduce((s,b) => s + b.TotalBalance, 0);

  return (
    <div className="min-h-screen bg-[#0a0a0f] p-4 md:p-6">
      <div className="max-w-7xl mx-auto">
        <div className="flex items-center justify-between mb-4">
          <h1 className="text-lg font-semibold text-gray-300">Trading Dashboard</h1>
          <button onClick={() => setConnected(false)} className="text-xs text-gray-600 hover:text-gray-400 transition-colors">Disconnect</button>
        </div>
        <StatusBar balances={balances} positions={positions} perf={perf} />
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 mb-4">
          <Positions positions={positions} />
          <Orders orders={orders} />
        </div>
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          <PnLChart trades={trades} />
          <RiskPanel positions={positions} balance={usdt} dailyPnL={perf?.total_pnl ?? 0} />
        </div>
        <div className="text-center text-[11px] text-gray-700 mt-6">
          Auto-refresh every 5s · Trading Server
        </div>
      </div>
    </div>
  );
}
