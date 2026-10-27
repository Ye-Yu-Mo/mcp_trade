import { z } from "zod/v4";
import type { TradingClient } from "../client/trading.js";
import type { Kline } from "../client/types.js";

function formatKlines(klines: Kline[]): string {
  if (klines.length === 0) return "No kline data.";
  const header = "| OpenTime | Open | High | Low | Close | Volume | Buy% | Delta |";
  const sep = "|----------|------|------|-----|-------|--------|------|-------|";
  const rows = klines.map((k) => {
    const time = new Date(k.OpenTime).toISOString().replace("T", " ").slice(0, 19);
    // Buy% = share of volume that lifted the ask. >55% on an up candle means
    // real buyers; <45% on an up candle means the move was short covering.
    const buyVol = k.TakerBuyVolume ?? NaN;
    const buyPct = k.Volume > 0 && Number.isFinite(buyVol) ? (buyVol / k.Volume) * 100 : NaN;
    const delta = k.Volume > 0 && Number.isFinite(buyVol) ? 2 * buyVol - k.Volume : NaN;
    const buyStr = Number.isFinite(buyPct) ? buyPct.toFixed(1) + "%" : "-";
    const deltaStr = Number.isFinite(delta)
      ? (delta >= 0 ? "+" : "") + delta.toFixed(2)
      : "-";
    return `| ${time} | ${k.Open} | ${k.High} | ${k.Low} | ${k.Close} | ${k.Volume.toFixed(4)} | ${buyStr} | ${deltaStr} |`;
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
      const result = await client.getKlines(args.symbol, args.interval ?? "1h", args.limit ?? 100);
      // Handle multi-interval response (nested map) vs single interval (flat array)
      if (Array.isArray(result)) {
        return {
          content: [{ type: "text" as const, text: formatKlines(result) }],
          structuredContent: { klines: result },
        };
      }
      // Nested: { "BTCUSDT": { "1h": [...], "4h": [...] } }
      const lines: string[] = [];
      for (const [sym, intervals] of Object.entries(result)) {
        lines.push(`## ${sym}`);
        for (const [iv, klines] of Object.entries(intervals as any)) {
          lines.push(`### ${iv} (${(klines as any[]).length} candles)`);
          if ((klines as any[]).length > 0) {
            const last = (klines as any[])[(klines as any[]).length - 1];
            lines.push(`O:${last.Open} H:${last.High} L:${last.Low} C:${last.Close}`);
          }
        }
      }
      return {
        content: [{ type: "text" as const, text: lines.join("\n") }],
        structuredContent: result,
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
      // Typical funding is ~0.01% (0.0001). 0.05% (0.0005) is already crowded;
      // the old 0.5% threshold almost never fired.
      const sentiment =
        fr.funding_rate > 0.0005
          ? "🟡 多头拥挤"
          : fr.funding_rate < -0.0005
            ? "🟡 空头拥挤"
            : "🟢 中性";
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
        content: [{ type: "text" as const, text: `${oi.symbol} OI: ${formatted} (base asset units)` }],
        structuredContent: oi,
      };
    },
  );

  // --- market.oi_history ---
  server.registerTool(
    "market.oi_history",
    {
      description:
        "获取未平仓合约（OI）历史序列。判断回撤质量的核心工具：" +
        "价格回落但 OI 下降 = 多头在离场、洗盘（健康，可找机会买）；" +
        "价格回落但 OI 上升 = 空头进场/多头死扛（趋势可能继续，别接飞刀）。",
      inputSchema: {
        symbol: z.string().describe("交易对，如 BTCUSDT"),
        period: z.string().default("15m").describe("周期：5m,15m,30m,1h,2h,4h,6h,12h,1d"),
        limit: z.number().default(24).describe("返回数量，默认 24，最大 500（币安只保留 30 天）"),
      },
    },
    async (args: { symbol: string; period?: string; limit?: number }) => {
      const points = await client.getOIHistory(args.symbol, args.period ?? "15m", args.limit ?? 24);
      if (points.length === 0) {
        return { content: [{ type: "text" as const, text: "No OI history." }], structuredContent: { points: [] } };
      }
      const first = points[0];
      const last = points[points.length - 1];
      const chg = first.open_interest > 0 ? ((last.open_interest - first.open_interest) / first.open_interest) * 100 : 0;
      const fmt = (v: number) =>
        v >= 1e9 ? (v / 1e9).toFixed(2) + "B" : v >= 1e6 ? (v / 1e6).toFixed(2) + "M" : v.toFixed(0);
      const header = "| Time | OI | OI Value ($) |";
      const rows = points.map((p: any) => {
        const t = new Date(p.time).toISOString().replace("T", " ").slice(0, 16);
        return `| ${t} | ${fmt(p.open_interest)} | ${fmt(p.open_interest_value)} |`;
      });
      return {
        content: [
          {
            type: "text" as const,
            text: `### ${args.symbol} OI ${args.period ?? "15m"} — ${chg >= 0 ? "+" : ""}${chg.toFixed(2)}% over ${points.length} buckets

` +
              [header, "|---|---|---|", ...rows].join("\n"),
          },
        ],
        structuredContent: { change_pct: chg, points },
      };
    },
  );

  // --- market.liquidations ---
  server.registerTool(
    "market.liquidations",
    {
      description:
        "查询最近的强制平仓（爆仓）记录。side=SELL 表示多头被强平，BUY 表示空头被强平。" +
        "一方集中爆仓的位置往往是止损堆积区，价格容易被插针扫到。summary=true 时按币种汇总。",
      inputSchema: {
        symbol: z.string().optional().describe("交易对，留空查全市场"),
        minutes: z.number().default(60).describe("回看分钟数，默认 60"),
        limit: z.number().default(30).describe("返回条数"),
        min_notional: z.number().optional().describe("最小名义价值（USDT）过滤，建议 1000"),
        summary: z.boolean().default(false).describe("true = 按币种汇总（多空爆仓笔数与金额）"),
      },
    },
    async (args: { symbol?: string; minutes?: number; limit?: number; min_notional?: number; summary?: boolean }) => {
      const rows = await client.getLiquidations({
        symbol: args.symbol,
        minutes: args.minutes ?? 60,
        limit: args.limit ?? 30,
        minNotional: args.min_notional,
        summary: args.summary,
      });
      if (rows.length === 0) {
        return { content: [{ type: "text" as const, text: "No liquidations in the window." }], structuredContent: { liquidations: [] } };
      }
      if (args.summary) {
        const header = "| Symbol | Long liqs | Long $ | Short liqs | Short $ |";
        const lines = rows.map((r: any) =>
          `| ${r.symbol} | ${r.long_liquidations} | ${r.long_notional.toFixed(0)} | ${r.short_liquidations} | ${r.short_notional.toFixed(0)} |`,
        );
        return {
          content: [{ type: "text" as const, text: [header, "|---|---|---|---|---|", ...lines].join("\n") }],
          structuredContent: { summary: rows },
        };
      }
      const header = "| Time | Symbol | Side | Price | Notional $ |";
      const lines = rows.map((r: any) => {
        const t = new Date(r.time).toISOString().replace("T", " ").slice(0, 16);
        return `| ${t} | ${r.symbol} | ${r.side} | ${r.price} | ${r.notional.toFixed(0)} |`;
      });
      return {
        content: [{ type: "text" as const, text: [header, "|---|---|---|---|---|", ...lines].join("\n") }],
        structuredContent: { liquidations: rows },
      };
    },
  );

  // --- market.atr ---
  server.registerTool(
    "market.atr",
    {
      description:
        "计算平均真实波幅（ATR），用于衡量波动率。ATR值越大=波动越大，止损应放得更宽。返回绝对ATR值和相对价格百分比。设止损时参考：止损距离≥1.5x ATR。",
      inputSchema: {
        symbol: z.string().describe("交易对，如 BTCUSDT"),
        interval: z.string().default("1h").describe("K线周期，默认 1h"),
        period: z.number().default(14).describe("ATR周期，默认 14"),
      },
    },
    async (args: { symbol: string; interval?: string; period?: number }) => {
      const atr = await client.getATR(args.symbol, args.interval ?? "1h", args.period ?? 14);
      return {
        content: [{ type: "text" as const, text: `${args.symbol} ATR(${args.period ?? 14},${args.interval ?? "1h"}): ${atr.atr.toFixed(4)} (${atr.atr_pct.toFixed(4)}% of price)` }],
        structuredContent: atr,
      };
    },
  );

  // --- market.candle_info ---
  server.registerTool(
    "market.candle_info",
    {
      description:
        "获取当前K线的开盘时间和剩余时间。用于判断蜡烛何时收盘——影响入场时机（如等4h线收盘确认信号）。返回当前蜡烛开盘时间+剩余秒数。",
      inputSchema: {
        symbol: z.string().describe("交易对，如 BTCUSDT"),
        interval: z.string().default("4h").describe("K线周期，默认 4h"),
      },
    },
    async (args: { symbol: string; interval?: string }) => {
      const info = await client.getCandleInfo(args.symbol, args.interval ?? "4h");
      const min = Math.floor((info.remaining_sec || 0) / 60);
      return {
        content: [{ type: "text" as const, text: `${info.symbol} ${info.interval} 蜡烛: 剩余 ${min}分${(info.remaining_sec||0) % 60}秒收盘` }],
        structuredContent: info,
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

  // --- market.monitor ---
  server.registerTool(
    "market.monitor",
    {
      description:
        "一键监控快照：账户权益、持仓浮动盈亏、已触发/等待中的价格提醒。用于会话恢复、定时巡检或等待入场时快速确认状态。服务端每 3 秒检测提醒，触发后会在这里出现。",
      inputSchema: {},
    },
    async () => {
      const [alerts, positions, balances] = await Promise.all([
        client.getAlerts(),
        client.getPositions(),
        client.getBalance(),
      ]);
      const total = balances
        .filter((b: any) => ["USDT", "USDC", "BUSD"].includes(b.Asset))
        .reduce((s: number, b: any) => s + b.TotalBalance, 0);
      const available = balances
        .filter((b: any) => ["USDT", "USDC", "BUSD"].includes(b.Asset))
        .reduce((s: number, b: any) => s + b.AvailableBalance, 0);
      const unrealized = positions.reduce((s, p) => s + p.UnrealizedPnL, 0);
      const triggered = alerts.filter((a: any) => a.triggered);
      const active = alerts.filter((a: any) => !a.triggered);

      const lines: string[] = [
        "## Monitor",
        "",
        `**Equity:** $${(total + unrealized).toFixed(2)} · **Available:** $${available.toFixed(2)} · **Unrealized:** ${unrealized >= 0 ? "+" : ""}${unrealized.toFixed(2)} · **Positions:** ${positions.length}`,
      ];

      if (positions.length > 0) {
        lines.push("", "### Positions", "", "| Symbol | Side | Qty | Entry | Mark | PnL |", "|---|---|---|---|---|---|");
        for (const p of positions) {
          lines.push(
            `| ${p.Symbol} | ${p.Side} ${p.Leverage}x | ${p.Quantity} | $${p.EntryPrice} | $${p.MarkPrice} | ${p.UnrealizedPnL >= 0 ? "+" : ""}${p.UnrealizedPnL.toFixed(2)} |`,
          );
        }
      }

      if (triggered.length > 0) {
        lines.push("", "### 🔔 Triggered Alerts", "");
        for (const a of triggered) {
          lines.push(
            `- **${a.symbol}** ${a.direction === "ABOVE" ? "≥" : "≤"} $${a.price} — ${a.message || "triggered"}`,
          );
        }
      }

      if (active.length > 0) {
        lines.push("", "### ⏳ Active Alerts", "");
        for (const a of active) {
          lines.push(
            `- ${a.symbol} ${a.direction === "ABOVE" ? "≥" : "≤"} $${a.price} — ${a.message || "(no message)"} · \`${a.id}\``,
          );
        }
      }

      if (positions.length === 0 && alerts.length === 0) {
        lines.push("", "_No open positions and no alerts._");
      }

      return {
        content: [{ type: "text" as const, text: lines.join("\n") }],
        structuredContent: { equity: total + unrealized, available, unrealized, positions, triggered, active },
      };
    },
  );
}
