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
      const header = "| ID | Symbol | Side | Qty | Price | Status | PnL | Time |";
      const sep = "|---|---|---|---|---|---|---|---|";
      const rows = records.map((r) => {
        const pnlStr = r.PnL !== 0 ? `$${r.PnL.toFixed(2)}` : "-";
        return `| ${r.ID} | ${r.Symbol} | ${r.Side} | ${r.Quantity} | ${r.Price} | ${r.Status} | ${pnlStr} | ${r.CreatedAt.slice(0, 19)} |`;
      });
      return {
        content: [{ type: "text" as const, text: [header, sep, ...rows].join("\n") }],
        structuredContent: { trades: records },
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
        `| Total Trades | ${perf.total_trades} |`,
        `| Win Rate | ${perf.win_rate.toFixed(1)}% |`,
        `| Win/Loss | ${perf.win_trades}W / ${perf.loss_trades}L |`,
        `| Total PnL | $${perf.total_pnl.toFixed(2)} |`,
        `| Avg PnL | $${perf.avg_pnl.toFixed(2)} |`,
        `| Max Win | $${perf.max_win.toFixed(2)} |`,
        `| Max Loss | $${perf.max_loss.toFixed(2)} |`,
        `| Profit Factor | ${perf.profit_factor.toFixed(2)} |`,
      ].join("\n");
      return {
        content: [{ type: "text" as const, text }],
        structuredContent: perf,
      };
    },
  );
}
