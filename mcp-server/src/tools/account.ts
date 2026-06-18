import { z } from "zod/v4";
import type { TradingClient } from "../client/trading.js";
import type { Balance, Position } from "../client/types.js";

function formatBalances(balances: Balance[]): string {
  if (balances.length === 0) return "No balance data.";
  const header = "| Asset | Available | Total |";
  const sep = "|-------|-----------|-------|";
  const rows = balances.map(
    (b) => `| ${b.Asset} | ${b.AvailableBalance.toFixed(4)} | ${b.TotalBalance.toFixed(4)} |`,
  );
  return [header, sep, ...rows].join("\n");
}

function formatPositions(positions: Position[]): string {
  if (positions.length === 0) return "No open positions.";
  const header = "| Symbol | Side | Qty | Entry | Mark | PnL |";
  const sep = "|--------|------|-----|-------|------|-----|";
  const rows = positions.map((p) => {
    const pnlStr = p.UnrealizedPnL >= 0 ? `+${p.UnrealizedPnL.toFixed(2)}` : p.UnrealizedPnL.toFixed(2);
    return `| ${p.Symbol} | ${p.Side} | ${p.Quantity} | ${p.EntryPrice} | ${p.MarkPrice} | ${pnlStr} |`;
  });
  return [header, sep, ...rows].join("\n");
}

export function registerAccountTools(server: any, client: TradingClient) {
  // --- account.balance ---
  server.registerTool(
    "account.balance",
    {
      description:
        "获取 Binance 合约账户余额。返回所有非零资产的可用余额和总余额。用于判断可用资金、计算仓位规模、风控检查。下单前必须调用此工具确认资金充足。",
      inputSchema: {},
    },
    async () => {
      const balances = await client.getBalance();
      const total = balances
        .filter((b) => b.Asset === "USDT" || b.Asset === "USDC" || b.Asset === "BUSD")
        .reduce((sum, b) => sum + b.TotalBalance, 0);
      const summary = `Total stablecoin balance: ~${total.toFixed(2)} USD`;
      return {
        content: [{ type: "text" as const, text: summary + "\n\n" + formatBalances(balances) }],
        structuredContent: { balances, totalStablecoin: total },
      };
    },
  );

  // --- account.positions ---
  server.registerTool(
    "account.positions",
    {
      description:
        "获取 Binance 合约当前持仓。返回所有未平仓头寸，包括开仓价、标记价、未实现盈亏。用于判断现有仓位、计算风险敞口、决定是否加仓/减仓/平仓。",
      inputSchema: {},
    },
    async () => {
      const positions = await client.getPositions();
      const totalPnL = positions.reduce((sum, p) => sum + p.UnrealizedPnL, 0);
      const pnlStr = totalPnL >= 0 ? `+${totalPnL.toFixed(2)}` : totalPnL.toFixed(2);
      const summary = positions.length > 0
        ? `${positions.length} positions, total unrealized PnL: ${pnlStr} USD`
        : "No open positions.";
      return {
        content: [{ type: "text" as const, text: summary + "\n\n" + formatPositions(positions) }],
        structuredContent: { positions, totalUnrealizedPnL: totalPnL },
      };
    },
  );
}
