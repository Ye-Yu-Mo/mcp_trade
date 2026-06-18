import { z } from "zod/v4";
import type { TradingClient } from "../client/trading.js";
import type { Kline } from "../client/types.js";

function formatKlines(klines: Kline[]): string {
  if (klines.length === 0) return "No kline data.";
  const header = "| OpenTime | Open | High | Low | Close | Volume |";
  const sep = "|----------|------|------|-----|-------|--------|";
  const rows = klines.map((k) => {
    const time = new Date(k.OpenTime).toISOString().replace("T", " ").slice(0, 19);
    return `| ${time} | ${k.Open} | ${k.High} | ${k.Low} | ${k.Close} | ${k.Volume.toFixed(4)} |`;
  });
  return [header, sep, ...rows].join("\n");
}

function formatOrderBook(ob: { Symbol: string; Bids: { Price: number; Quantity: number }[]; Asks: { Price: number; Quantity: number }[] }): string {
  const lines = [`## Order Book: ${ob.Symbol}`, "", "| Price (Bid) | Qty (Bid) | | Price (Ask) | Qty (Ask) |", "|------------|----------|---|------------|----------|"];
  const n = Math.max(ob.Bids.length, ob.Asks.length);
  for (let i = 0; i < n; i++) {
    const bid = ob.Bids[i] ? `${ob.Bids[i].Price} | ${ob.Bids[i].Quantity.toFixed(4)}` : " | ";
    const ask = ob.Asks[i] ? `${ob.Asks[i].Price} | ${ob.Asks[i].Quantity.toFixed(4)}` : " | ";
    lines.push(`| ${bid} | | ${ask} |`);
  }
  return lines.join("\n");
}

export function registerMarketTools(server: any, client: TradingClient) {
  // --- market.klines ---
  server.registerTool(
    "market.klines",
    {
      description:
        "获取 Binance 合约 K 线数据。返回指定交易对和周期的 K 线列表。用于分析价格走势、识别支撑阻力位、判断市场结构。价格行为交易的核心工具。",
      inputSchema: {
        symbol: z.string().describe("交易对，如 BTCUSDT、ETHUSDT"),
        interval: z.string().default("1h").describe("K 线周期：1m, 5m, 15m, 30m, 1h, 4h, 1d, 1w"),
        limit: z.number().default(100).describe("返回数量，默认 100，最大 1500"),
      },
    },
    async (args: { symbol: string; interval?: string; limit?: number }) => {
      const klines = await client.getKlines(args.symbol, args.interval ?? "1h", args.limit ?? 100);
      return {
        content: [{ type: "text" as const, text: formatKlines(klines) }],
        structuredContent: { klines },
      };
    },
  );

  // --- market.ticker ---
  server.registerTool(
    "market.ticker",
    {
      description:
        "获取指定交易对的最新价格。返回当前价格，用于快速判断市场行情、决定入场出场时机。",
      inputSchema: {
        symbol: z.string().describe("交易对，如 BTCUSDT"),
      },
    },
    async (args: { symbol: string }) => {
      const ticker = await client.getTicker(args.symbol);
      return {
        content: [{ type: "text" as const, text: `${ticker.Symbol}: ${ticker.Price}` }],
        structuredContent: ticker,
      };
    },
  );

  // --- market.price ---
  server.registerTool(
    "market.price",
    {
      description:
        "快速获取交易对当前价格（纯数字）。轻量级工具，用于快速扫一眼价格，比 market.ticker 更省 token。一次性可查多个币种。",
      inputSchema: {
        symbol: z.string().describe("交易对，如 BTCUSDT"),
      },
    },
    async (args: { symbol: string }) => {
      const ticker = await client.getTicker(args.symbol);
      return {
        content: [{ type: "text" as const, text: `${args.symbol} = ${ticker.Price}` }],
        structuredContent: { symbol: ticker.Symbol, price: ticker.Price },
      };
    },
  );

  // --- market.orderbook ---
  server.registerTool(
    "market.orderbook",
    {
      description:
        "获取交易对的订单簿深度。返回当前买卖盘口的挂单价格和数量。用于判断市场流动性、评估大单滑点风险、识别支撑阻力区域的挂单密度。",
      inputSchema: {
        symbol: z.string().describe("交易对，如 BTCUSDT"),
        limit: z.number().default(20).describe("深度档位，5/10/20/50/100，默认 20"),
      },
    },
    async (args: { symbol: string; limit?: number }) => {
      const ob = await client.getOrderBook(args.symbol, args.limit ?? 20);
      return {
        content: [{ type: "text" as const, text: formatOrderBook(ob) }],
        structuredContent: ob,
      };
    },
  );

  // --- market.watch ---
  server.registerTool(
    "market.watch",
    {
      description:
        "一键获取所有订阅币种的最新行情快照。返回价格、K线、订单簿数据。用于AI快速掌握全局市场状态，比逐个调用 market.price 更高效。数据来自内存缓存，延迟 < 100ms。",
      inputSchema: {},
    },
    async () => {
      const snapshot = await client.getWatch();
      const prices = snapshot.prices || {};
      const symbols = Object.keys(prices).filter((k) => !k.includes("_balance"));
      const lines = ["## Market Snapshot", "", "| Symbol | Price |", "|--------|-------|"];
      for (const sym of symbols) {
        lines.push(`| ${sym} | ${prices[sym]} |`);
      }
      return {
        content: [{ type: "text" as const, text: lines.join("\n") }],
        structuredContent: snapshot,
      };
    },
  );
}
