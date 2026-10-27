import { z } from "zod/v4";
import type { TradingClient } from "../client/trading.js";

export function registerTradeTools(server: any, client: TradingClient) {
  // --- trade.history ---
  server.registerTool(
    "trade.history",
    {
      description:
        "查询历史交易记录。返回指定币种的交易列表，包含入场价、出场价、盈亏、入场理由等。用于AI回顾历史交易、总结经验教训、识别亏损模式。",
      inputSchema: {
        symbol: z.string().optional().describe("交易对，如 BTCUSDT。留空查所有"),
        limit: z.number().default(20).describe("返回数量，默认20"),
      },
    },
    async (args: { symbol?: string; limit?: number }) => {
      const records = await client.getTradeHistory(args.symbol, args.limit ?? 20);
      if (records.length === 0) {
        return {
          content: [{ type: "text" as const, text: "No trade history." }],
          structuredContent: { trades: [] },
        };
      }
      const header = "| ID | Symbol | Side | Qty | Entry | Status | PnL | R | Setup | Time |";
      const sep = "|---|---|---|---|---|---|---|---|---|---|";
      const rows = records.map((r) => {
        const pnlStr = r.PnL !== 0 ? `$${r.PnL.toFixed(2)}` : "-";
        const rStr = r.RMultiple ? `${r.RMultiple >= 0 ? "+" : ""}${r.RMultiple.toFixed(2)}R` : "-";
        return `| ${r.ID} | ${r.Symbol} | ${r.Side} | ${r.Quantity} | ${r.Price} | ${r.Status} | ${pnlStr} | ${rStr} | ${r.Setup || "-"} | ${r.CreatedAt.slice(0, 19)} |`;
      });
      return {
        content: [{ type: "text" as const, text: [header, sep, ...rows].join("\n") }],
        structuredContent: { trades: records },
      };
    },
  );

  // --- trade.exchange ---
  server.registerTool(
    "trade.exchange",
    {
      description:
        "拉取交易所账户的原始资金流水（REALIZED_PNL / COMMISSION / FUNDING_FEE 等）。" +
        "直接在币安手动下的单不会写进 trade.history，只能从这里看到。仅实盘可用，纸面盘会报错。",
      inputSchema: {
        hours: z.number().default(48).describe("回看小时数，默认 48"),
        symbol: z.string().optional().describe("交易对，留空查所有"),
        limit: z.number().default(200).describe("返回条数上限"),
      },
    },
    async (args: { hours?: number; symbol?: string; limit?: number }) => {
      const rows = await client.getExchangeIncome(args.hours ?? 48, args.symbol, args.limit ?? 200);
      if (rows.length === 0) {
        return {
          content: [{ type: "text" as const, text: "No exchange income records." }],
          structuredContent: { records: [] },
        };
      }
      const header = "| Time (UTC) | Symbol | Type | Income |";
      const sep = "|---|---|---|---|";
      const lines = rows.map((r: any) => {
        const t = new Date(r.time).toISOString().replace("T", " ").slice(0, 16);
        return `| ${t} | ${r.symbol || "-"} | ${r.incomeType} | ${r.income.toFixed(5)} |`;
      });
      const net = rows.reduce((a: number, r: any) => a + (r.income ?? 0), 0);
      return {
        content: [
          {
            type: "text" as const,
            text: [header, sep, ...lines, "", `**Net: ${net.toFixed(5)} USDT** (${rows.length} records)`].join("\n"),
          },
        ],
        structuredContent: { records: rows, net },
      };
    },
  );

  // --- trade.journal ---
  server.registerTool(
    "trade.journal",
    {
      description:
        "写入交易日志/经验总结。记录入场理由、出场理由、事后反思。用于AI在交易过程中积累经验，下次对话开始时可以回顾之前的教训。",
      inputSchema: {
        entry_type: z.string().describe("类型：ENTRY（入场）/ EXIT（出场）/ REVIEW（复盘）"),
        reason: z.string().describe("理由/总结内容"),
        tags: z.string().optional().describe("标签，逗号分隔，如 '趋势交易,假突破'"),
        trade_id: z.number().optional().describe("关联的交易ID（可选）"),
      },
    },
    async (args: { entry_type: string; reason: string; tags?: string; trade_id?: number }) => {
      const result = await client.postJournal(args.entry_type, args.reason, args.tags, args.trade_id);
      return {
        content: [{ type: "text" as const, text: `Journal entry #${result.id} recorded.` }],
        structuredContent: result,
      };
    },
  );

  // --- trade.journal_list ---
  server.registerTool(
    "trade.journal_list",
    {
      description:
        "查询历史交易日志/经验记录。返回之前写下的入场理由、出场理由、复盘总结。每次交易会话开始时必须调用此工具，回顾过去的经验教训，避免重复犯错。",
      inputSchema: {
        limit: z.number().default(20).describe("返回数量，默认20"),
        entry_type: z.string().optional().describe("按类型筛选：ENTRY / EXIT / REVIEW"),
        tags: z.string().optional().describe("按标签筛选，如 '趋势交易'"),
      },
    },
    async (args: { limit?: number; entry_type?: string; tags?: string }) => {
      const entries = await client.getJournals(args.limit ?? 20, args.entry_type, args.tags);
      if (entries.length === 0) {
        return {
          content: [{ type: "text" as const, text: "No journal entries yet." }],
          structuredContent: { entries: [] },
        };
      }
      const header = "| ID | Type | Tags | Reason | Time |";
      const sep = "|---|---|---|---|---|";
      const rows = entries.map((e) => {
        const reason = e.Reason.length > 80 ? e.Reason.slice(0, 80) + "..." : e.Reason;
        return `| ${e.ID} | ${e.EntryType} | ${e.Tags || "-"} | ${reason} | ${e.CreatedAt.slice(0, 19)} |`;
      });
      return {
        content: [{ type: "text" as const, text: [header, sep, ...rows].join("\n") }],
        structuredContent: { entries },
      };
    },
  );

  // --- trade.performance ---
  server.registerTool(
    "trade.performance",
    {
      description:
        "获取交易绩效统计。返回胜率、盈亏比、总盈亏、最大回撤等指标。用于AI判断策略是否有效、是否需要调整。",
      inputSchema: {},
    },
    async () => {
      const perf = await client.getPerformance();
      const text = [
        `## Trading Performance`,
        `| Metric | Value |`,
        `|--------|-------|`,
        `| Closed Trades | ${perf.total_trades} |`,
        `| Win Rate | ${perf.win_rate.toFixed(1)}% |`,
        `| Win/Loss/BE | ${perf.win_trades}W / ${perf.loss_trades}L / ${perf.breakeven_trades}BE |`,
        `| Total PnL | $${perf.total_pnl.toFixed(2)} |`,
        `| Avg PnL | $${perf.avg_pnl.toFixed(2)} |`,
        `| Avg R | ${perf.avg_r.toFixed(2)}R |`,
        `| Expectancy | ${perf.expectancy_r.toFixed(2)}R |`,
        `| Max Win | $${perf.max_win.toFixed(2)} |`,
        `| Max Loss | $${perf.max_loss.toFixed(2)} |`,
        `| Profit Factor | ${perf.profit_factor_infinite ? "∞" : perf.profit_factor.toFixed(2)} |`,
        `| Max Drawdown | $${perf.max_drawdown.toFixed(2)} |`,
        ``,
        `_胜率只统计有方向的交易（W+L），不把 0 PnL 算作亏损。_`,
      ].join("\n");
      return {
        content: [{ type: "text" as const, text }],
        structuredContent: perf,
      };
    },
  );
}
