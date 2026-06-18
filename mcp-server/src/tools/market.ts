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

function fmtVolume(v: number): string {
  if (v >= 1e9) return (v / 1e9).toFixed(2) + "B";
  if (v >= 1e6) return (v / 1e6).toFixed(1) + "M";
  if (v >= 1e3) return (v / 1e3).toFixed(1) + "K";
  return v.toFixed(0);
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
      // Show triggered alerts prominently
      const alerts = snapshot.triggered_alerts || [];
      if (alerts.length > 0) {
        lines.push("", "### 🔔 Triggered Alerts", "");
        for (const a of alerts) {
          lines.push(`- **${a.symbol}** ${a.direction} $${a.price}: ${a.message || "(no message)"}`);
        }
      }
      return {
        content: [{ type: "text" as const, text: lines.join("\n") }],
        structuredContent: snapshot,
      };
    },
  );

  // --- market.alert ---
  server.registerTool(
    "market.alerts",
    {
      description:
        "管理价格提醒。AI可设置价格触发提醒（到达某个价格时通知），查询活跃和已触发的提醒，或删除提醒。用于等待入场信号时不用持续轮询——设好提醒后AI可以去做别的事，定期检查是否触发即可。" +
        "设置：传 action='set', symbol, price, direction(ABOVE/BELOW), message。查询：传 action='list'。删除：传 action='remove', id。",
      inputSchema: {
        action: z.string().describe("操作：set / list / remove"),
        symbol: z.string().optional().describe("交易对，action=set 时必填"),
        price: z.number().optional().describe("触发价格，action=set 时必填"),
        direction: z.string().optional().describe("ABOVE（突破上方触发）或 BELOW（跌破下方触发）"),
        message: z.string().optional().describe("提醒消息，如 'BNB反弹到592做空'"),
        id: z.string().optional().describe("提醒ID，action=remove 时必填"),
      },
    },
    async (args: { action: string; symbol?: string; price?: number; direction?: string; message?: string; id?: string }) => {
      if (args.action === "set") {
        if (!args.symbol || !args.price || !args.direction) throw new Error("symbol, price, direction required for set");
        const result = await client.setAlert(args.symbol, args.price, args.direction, args.message || "");
        return {
          content: [{ type: "text" as const, text: `Alert ${result.id} set: ${args.symbol} ${args.direction} $${args.price} — "${args.message}"` }],
          structuredContent: result,
        };
      }
      if (args.action === "remove") {
        if (!args.id) throw new Error("id required for remove");
        await client.removeAlert(args.id);
        return {
          content: [{ type: "text" as const, text: `Alert ${args.id} removed.` }],
          structuredContent: { status: "removed" },
        };
      }
      // list
      const alerts = await client.getAlerts();
      if (alerts.length === 0) {
        return {
          content: [{ type: "text" as const, text: "No alerts set." }],
          structuredContent: { alerts: [] },
        };
      }
      const header = "| ID | Symbol | Price | Dir | Status | Message |";
      const sep = "|---|---|---|---|---|---|";
      const rows = alerts.map((a: any) => {
        const status = a.triggered ? "🔔 TRIGGERED" : "⏳ waiting";
        const msg = (a.message || "").length > 50 ? a.message.slice(0, 50) + "..." : (a.message || "-");
        return `| ${a.id} | ${a.symbol} | $${a.price} | ${a.direction} | ${status} | ${msg} |`;
      });
      return {
        content: [{ type: "text" as const, text: [header, sep, ...rows].join("\n") }],
        structuredContent: { alerts },
      };
    },
  );

  // --- market.scanner ---
  server.registerTool(
    "market.scanner",
    {
      description:
        "市场扫描——一键获取全市场行情概览。返回按24h成交量排序的Top币种列表，包含价格、24h涨跌幅、24h高低价、成交量。用于发现交易机会：谁在逆势涨、谁跌得最惨、钱流到哪。主观交易第一步。",
      inputSchema: {
        limit: z.number().default(20).describe("返回数量，默认20"),
      },
    },
    async (args: { limit?: number }) => {
      const results = await client.getScanner(args.limit ?? 20);
      const header = "| Symbol | Price | 24h Chg% | Volume |";
      const sep = "|---|---|---|---|";
      const rows = results.map((r: any) => {
        const chg = r.change_24h_pct >= 0 ? `+${r.change_24h_pct.toFixed(2)}` : r.change_24h_pct.toFixed(2);
        const color = r.change_24h_pct >= 0 ? "🟢" : "🔴";
        return `| ${r.symbol} | $${r.last_price.toFixed(4)} | ${color} ${chg}% | ${fmtVolume(r.quote_volume_24h)} |`;
      });
      return {
        content: [{ type: "text" as const, text: [header, sep, ...rows].join("\n") }],
        structuredContent: { results },
      };
    },
  );

  // --- market.funding ---
  server.registerTool(
    "market.funding",
    {
      description:
        "获取永续合约当前资金费率。正值=多头付空头（多头拥挤），负值=空头付多头（空头拥挤，可能轧空）。极端费率是反向信号。返回当前费率和下次结算时间。",
      inputSchema: {
        symbol: z.string().default("BTCUSDT").describe("交易对，默认 BTCUSDT"),
      },
    },
    async (args: { symbol?: string }) => {
      const fr = await client.getFundingRate(args.symbol ?? "BTCUSDT");
      const ratePct = (fr.funding_rate * 100).toFixed(4);
      const sentiment = fr.funding_rate > 0.005 ? "🟡 多头拥挤" : fr.funding_rate < -0.005 ? "🟡 空头拥挤" : "🟢 中性";
      return {
        content: [{ type: "text" as const, text: `${fr.symbol} funding: ${ratePct}% ${sentiment} (next: ${new Date(fr.funding_time).toISOString().slice(0,19)})` }],
        structuredContent: fr,
      };
    },
  );

  // --- market.oi ---
  server.registerTool(
    "market.oi",
    {
      description:
        "获取未平仓合约数量。OI + 价格方向 = 趋势强度信号：价格涨+OI涨=健康上涨，价格涨+OI跌=空头平仓反弹（不可持续），价格跌+OI涨=健康下跌，价格跌+OI跌=多头平仓（可能见底）。",
      inputSchema: {
        symbol: z.string().default("BTCUSDT").describe("交易对，默认 BTCUSDT"),
      },
    },
    async (args: { symbol?: string }) => {
      const oi = await client.getOpenInterest(args.symbol ?? "BTCUSDT");
      const formatted = oi.open_interest >= 1e9 ? (oi.open_interest / 1e9).toFixed(2) + "B" :
        oi.open_interest >= 1e6 ? (oi.open_interest / 1e6).toFixed(1) + "M" : oi.open_interest.toFixed(0);
      return {
        content: [{ type: "text" as const, text: `${oi.symbol} OI: ${formatted} contracts` }],
        structuredContent: oi,
      };
    },
  );

  // --- market.calendar ---
  server.registerTool(
    "market.calendar",
    {
      description:
        "获取未来30天重大经济事件日历。返回非农、CPI、FOMC利率决议等高风险事件的时间和日期。价格行为交易中，技术结构在重大新闻前容易被破坏——此工具帮助AI提前规避事件风险。",
      inputSchema: {},
    },
    async () => {
      const events = await client.getCalendar();
      if (events.length === 0) {
        return {
          content: [{ type: "text" as const, text: "No upcoming events." }],
          structuredContent: { events: [] },
        };
      }
      const header = "| Date | Time | Event | Impact |";
      const sep = "|---|---|---|---|";
      const rows = events.map(
        (e: any) => `| ${e.date} | ${e.time} | ${e.event} | ${e.impact} |`,
      );
      return {
        content: [{ type: "text" as const, text: [header, sep, ...rows].join("\n") }],
        structuredContent: { events },
      };
    },
  );
}
