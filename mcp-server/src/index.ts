#!/usr/bin/env node
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { loadConfig } from "./config.js";
import { TradingClient } from "./client/trading.js";
import { registerMarketTools } from "./tools/market.js";
import { registerAccountTools } from "./tools/account.js";
import { registerOrderTools } from "./tools/order.js";
import { registerTradeTools } from "./tools/trade.js";

async function main() {
  const cfg = loadConfig();
  const client = new TradingClient(cfg.tradingServerUrl, cfg.tradingApiToken);

  const server = new McpServer({
    name: "mcp-trade",
    version: "0.1.0",
    description: "Binance Futures trading MCP server — price action / naked K-line strategies. 提供行情查询、账户余额、持仓信息，支持 AI 主观交易决策。",
  });

  registerMarketTools(server, client);
  registerAccountTools(server, client);
  registerOrderTools(server, client);
  registerTradeTools(server, client);

  const transport = new StdioServerTransport();
  await server.connect(transport);

  console.error(`mcp-trade connected to ${cfg.tradingServerUrl}`);
}

main().catch((err) => {
  console.error("mcp-trade fatal:", err.message);
  process.exit(1);
});
