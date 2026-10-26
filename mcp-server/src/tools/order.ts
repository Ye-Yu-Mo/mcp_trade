import { z } from "zod/v4";
import type { TradingClient } from "../client/trading.js";
import type { BracketResult, Order, OrderPreview } from "../client/types.js";

type PlanParams = {
  symbol: string;
  side: string;
  type: string;
  quantity?: number;
  price?: number;
  stopPrice?: number;
  takeProfit?: number;
  riskPct?: number;
  leverage?: number;
  reduceOnly?: boolean;
  closePosition?: boolean;
  entryReason?: string;
  setup?: string;
  tags?: string;
};

export function registerOrderTools(server: any, client: TradingClient) {
  // Preview state lives in this MCP process. Confirm can be called with only a
  // plan_id; the original parameters are replayed so they cannot drift.
  const plans = new Map<string, PlanParams>();

  // --- order.place ---
  server.registerTool(
    "order.place",
    {
      description:
        "下单（带服务端风控与自动保护单）。两步流程：" +
        "(1) 不带 confirm 调用 → 返回预览、风险检查、单次有效的 plan_id；" +
        "(2) 用户确认后，带 confirm=true 和 plan_id 调用 → 执行。" +
        "开仓必须提供 stop_price（强制止损）；可用 risk_pct 让服务端按账户权益自动算仓位，" +
        "也可直接给 quantity。成交后服务端会自动挂 STOP_MARKET（closePosition）和可选的止盈单。" +
        "实盘开仓目前只支持 MARKET；平仓/减仓用 reduce_only=true 或 close_position=true，这类订单不受日亏损限制。",
      inputSchema: {
        symbol: z.string().optional().describe("交易对，如 BTCUSDT。confirm 时可省略（沿用预览参数）"),
        side: z.string().optional().describe("BUY / SELL。confirm 时可省略"),
        type: z.string().optional().describe("MARKET / LIMIT。开仓建议 MARKET；confirm 时可省略"),
        quantity: z.number().optional().describe("数量；与 risk_pct 二选一"),
        risk_pct: z.number().optional().describe("单笔风险占权益百分比，如 1 = 1%；服务端据此计算数量"),
        price: z.number().optional().describe("限价（LIMIT 必填）"),
        stop_price: z.number().optional().describe("止损触发价（开仓必填）"),
        take_profit_price: z.number().optional().describe("止盈触发价（可选）"),
        leverage: z.number().optional().describe("杠杆，默认服务端上限（通常 5x）"),
        reduce_only: z.boolean().optional().describe("仅减仓"),
        close_position: z.boolean().optional().describe("平掉整个仓位"),
        entry_reason: z.string().optional().describe("入场理由，写入交易日志"),
        setup: z.string().optional().describe("策略/形态标签，如 YTC-突破回踩"),
        tags: z.string().optional().describe("逗号分隔标签"),
        confirm: z.boolean().optional().describe("用户已确认时传 true"),
        plan_id: z.string().optional().describe("预览返回的 plan_id"),
      },
    },
    async (args: {
      symbol?: string;
      side?: string;
      type?: string;
      quantity?: number;
      risk_pct?: number;
      price?: number;
      stop_price?: number;
      take_profit_price?: number;
      leverage?: number;
      reduce_only?: boolean;
      close_position?: boolean;
      entry_reason?: string;
      setup?: string;
      tags?: string;
      confirm?: boolean;
      plan_id?: string;
    }) => {
      if (args.confirm) {
        if (!args.plan_id) throw new Error("plan_id is required when confirm=true");
        const saved = plans.get(args.plan_id);
        if (!saved) {
          throw new Error("plan_id not found in this MCP session; call order.place without confirm to preview again");
        }
        plans.delete(args.plan_id);
        const result = await client.placeOrder({ ...saved, planId: args.plan_id });
        return {
          content: [{ type: "text" as const, text: formatResult(result) }],
          structuredContent: result,
        };
      }

      const params: PlanParams = {
        symbol: (args.symbol ?? "").toUpperCase(),
        side: (args.side ?? "").toUpperCase(),
        type: (args.type ?? "MARKET").toUpperCase(),
        quantity: args.quantity,
        price: args.price,
        stopPrice: args.stop_price,
        takeProfit: args.take_profit_price,
        riskPct: args.risk_pct,
        leverage: args.leverage,
        reduceOnly: args.reduce_only,
        closePosition: args.close_position,
        entryReason: args.entry_reason,
        setup: args.setup,
        tags: args.tags,
      };
      if (!params.symbol || !params.side || !params.type) {
        throw new Error("symbol, side and type are required for a preview");
      }
      const reducing = params.reduceOnly || params.closePosition;
      if (!reducing && !params.stopPrice) {
        throw new Error("stop_price is mandatory for every entry — no preview without a protective stop");
      }
      if (!reducing && params.quantity === undefined && params.riskPct === undefined) {
        throw new Error("provide quantity or risk_pct to size the entry");
      }

      const preview = await client.previewOrder(params);
      plans.set(preview.plan_id, params);
      // Keep the in-memory plan table bounded.
      if (plans.size > 200) {
        const oldest = plans.keys().next().value;
        if (oldest) plans.delete(oldest);
      }
      return {
        content: [{ type: "text" as const, text: formatPreview(preview) }],
        structuredContent: preview,
      };
    },
  );

  // --- order.close ---
  server.registerTool(
    "order.close",
    {
      description:
        "市价平仓。传 symbol 平掉整个仓位；可选 quantity 只平一部分。服务端会同时撤销该交易对的保护单。",
      inputSchema: {
        symbol: z.string().describe("交易对，如 BTCUSDT"),
        quantity: z.number().optional().describe("只平部分仓位时填写"),
        reason: z.string().optional().describe("平仓理由（可选）"),
      },
    },
    async (args: { symbol: string; quantity?: number; reason?: string }) => {
      const order = await client.closePosition(args.symbol, args.quantity, args.reason);
      const qty = positive(order.executedQty) ? order.executedQty : order.origQty;
      const avg = positive(order.avgPrice) ? order.avgPrice : "市价";
      return {
        content: [
          {
            type: "text" as const,
            text: `## 已平仓\n\n| 字段 | 值 |\n|---|---|\n| 订单 | #${order.orderId} |\n| 交易对 | ${order.symbol} |\n| 方向 | ${order.side} |\n| 数量 | ${qty} |\n| 成交价 | ${avg} |\n| 状态 | ${order.status} |`,
          },
        ],
        structuredContent: order,
      };
    },
  );

  // --- order.cancel ---
  server.registerTool(
    "order.cancel",
    {
      description: "撤销指定订单。传入 symbol 和 order_id 取消未成交的挂单。",
      inputSchema: {
        symbol: z.string().describe("交易对，如 BTCUSDT"),
        order_id: z.number().describe("要撤销的订单 ID"),
      },
    },
    async (args: { symbol: string; order_id: number }) => {
      const order = await client.cancelOrder(args.symbol, args.order_id);
      return {
        content: [
          {
            type: "text" as const,
            text: `Order #${order.orderId} ${order.symbol} cancelled. Status: ${order.status}`,
          },
        ],
        structuredContent: order,
      };
    },
  );

  // --- order.list ---
  server.registerTool(
    "order.list",
    {
      description: "查询当前活跃订单（挂单），可按交易对过滤。",
      inputSchema: {
        symbol: z.string().optional().describe("交易对，留空查所有"),
      },
    },
    async (args: { symbol?: string }) => {
      const orders = await client.getOpenOrders(args.symbol);
      if (orders.length === 0) {
        return {
          content: [{ type: "text" as const, text: "No open orders." }],
          structuredContent: { orders: [] },
        };
      }
      const rows = orders.map((o) => {
        const conditional = /STOP|TAKE_PROFIT|TRAILING/i.test(o.type);
        const priceLabel = conditional
          ? o.stopPrice !== "0"
            ? "$" + o.stopPrice
            : "—"
          : o.price !== "0"
            ? "$" + o.price
            : "市价";
        return `| #${o.orderId} | ${o.symbol} | ${o.side} | ${o.type} | ${o.origQty} | ${priceLabel} | ${o.status} |`;
      });
      const header = "| Order ID | Symbol | Side | Type | Qty | Price/Trigger | Status |";
      const sep = "|---|---|---|---|---|---|---|";
      return {
        content: [{ type: "text" as const, text: [header, sep, ...rows].join("\n") }],
        structuredContent: { orders },
      };
    },
  );

  // --- order.modify_stop ---
  server.registerTool(
    "order.modify_stop",
    {
      description:
        "修改已有仓位的止损价。服务端先下新止损、成功后再撤旧止损，避免持仓裸奔。" +
        "前置条件：旧止损单 order_id（用 order.list 查询）。",
      inputSchema: {
        symbol: z.string().describe("交易对，如 BTCUSDT"),
        old_order_id: z.number().describe("要替换的旧止损单 ID"),
        new_stop_price: z.number().describe("新的止损价格"),
        side: z.string().optional().describe("SELL（做多止损）或 BUY（做空止损）；留空自动判断"),
        quantity: z.number().optional().describe("持仓数量（closePosition 模式下可省略）"),
        position_side: z.string().optional().describe("LONG / SHORT，默认自动"),
      },
    },
    async (args: {
      symbol: string;
      old_order_id: number;
      new_stop_price: number;
      side?: string;
      quantity?: number;
      position_side?: string;
    }) => {
      const result = await client.modifyStop({
        symbol: args.symbol,
        oldOrderId: args.old_order_id,
        newStopPrice: args.new_stop_price,
        side: args.side ?? "",
        quantity: args.quantity ?? 0,
        positionSide: args.position_side,
      });
      const text = [
        `## 止损已更新`,
        `| 字段 | 值 |`,
        `|-------|-------|`,
        `| Symbol | ${args.symbol} |`,
        `| Old Order | #${args.old_order_id} |`,
        `| New Stop | $${args.new_stop_price} |`,
        `| New Order | #${result.new_stop?.orderId ?? "—"} |`,
        `| Status | ${result.new_stop?.status ?? "—"} |`,
        result.warning ? `\n⚠️ ${result.warning}` : "",
      ].join("\n");
      return {
        content: [{ type: "text" as const, text }],
        structuredContent: result,
      };
    },
  );

  // --- order.status ---
  server.registerTool(
    "order.status",
    {
      description: "查询单个订单的详细状态（NEW/PARTIALLY_FILLED/FILLED/CANCELED）。",
      inputSchema: {
        symbol: z.string().describe("交易对，如 BTCUSDT"),
        order_id: z.number().describe("订单 ID"),
      },
    },
    async (args: { symbol: string; order_id: number }) => {
      const o = await client.getOrder(args.symbol, args.order_id);
      const executed = parseFloat(o.executedQty);
      const orig = parseFloat(o.origQty);
      const pct = orig > 0 ? ((executed / orig) * 100).toFixed(1) : "0.0";
      const text = [
        `## Order #${o.orderId}`,
        `| Field | Value |`,
        `|-------|-------|`,
        `| Symbol | ${o.symbol} |`,
        `| Side | ${o.side} |`,
        `| Type | ${o.type} |`,
        `| Price | ${o.price} |`,
        `| Avg Fill | ${o.avgPrice} |`,
        `| Qty | ${o.origQty} |`,
        `| Filled | ${o.executedQty} (${pct}%) |`,
        `| Status | ${o.status} |`,
      ].join("\n");
      return {
        content: [{ type: "text" as const, text }],
        structuredContent: o,
      };
    },
  );

  // --- order.oco (deprecated) ---
  server.registerTool(
    "order.oco",
    {
      description:
        "已弃用：Binance 合约没有 OCO 产品。请改用 order.place 并同时提供 stop_price 与 take_profit_price；服务端会自动挂止损和止盈。",
      inputSchema: {
        symbol: z.string().describe("交易对"),
        side: z.string().describe("SELL / BUY"),
        quantity: z.number().describe("数量"),
        price: z.number().describe("止盈价"),
        stop_price: z.number().describe("止损价"),
      },
    },
    async () => {
      throw new Error(
        "OCO is not supported on Binance Futures. Use order.place with stop_price and take_profit_price.",
      );
    },
  );
}

function formatPreview(p: OrderPreview): string {
  const ok = p.risk.passed ? "✅ PASSED" : `❌ REJECTED${p.risk.code ? " [" + p.risk.code + "]" : ""}`;
  const warnings = (p.risk.warnings ?? []).length
    ? (p.risk.warnings ?? []).map((w) => `- ⚠️ ${w}`).join("\n")
    : "- 无";
  return [
    `## Order Preview ${ok}`,
    ``,
    `| 字段 | 值 |`,
    `|---|---|`,
    `| Symbol | ${p.symbol} |`,
    `| Side / Type | ${p.side} / ${p.type} |`,
    `| Quantity | ${fmt(p.quantity)} |`,
    `| Entry | ${p.price ? "$" + fmt(p.price) : "market"} |`,
    `| Stop | ${p.stop_price ? "$" + fmt(p.stop_price) : "—"} |`,
    `| Take Profit | ${p.take_profit ? "$" + fmt(p.take_profit) : "—"} |`,
    `| Notional | $${fmt(p.notional)} |`,
    `| Risk | $${fmt(p.risk_amount)} (${fmt(p.risk_pct)}% of equity) |`,
    `| Margin @ ${fmt(p.leverage)}x | $${fmt(p.margin)} |`,
    `| Equity / Available | $${fmt(p.equity)} / $${fmt(p.available)} |`,
    `| Open Risk / Positions | $${fmt(p.open_risk)} / ${p.open_positions} |`,
    ``,
    `### Risk ${ok}`,
    p.risk.reason ? `${p.risk.code ? "[" + p.risk.code + "] " : ""}${p.risk.reason}` : "",
    ``,
    `### Warnings`,
    warnings,
    ``,
    `### Plan ID`,
    `\`${p.plan_id}\``,
    ``,
    p.risk.passed
      ? `确认后调用 \`order.place({ confirm: true, plan_id: "${p.plan_id}" })\` 执行。plan 2 分钟内有效、单次使用。`
      : `风险检查未通过，不要执行。调整入场/止损/仓位后重新预览。`,
    ``,
    `检查明细：${(p.risk.checks ?? []).join(" · ")}`,
  ]
    .filter((line) => line !== "")
    .join("\n");
}

function formatResult(r: BracketResult): string {
  const entry = r.entry;
  const qty = positive(entry.executedQty) ? entry.executedQty : entry.origQty;
  const avg = positive(entry.avgPrice)
    ? entry.avgPrice
    : positive(entry.price)
      ? entry.price
      : "市价";
  const lines = [
    `## 订单已执行`,
    ``,
    `| 字段 | 值 |`,
    `|---|---|`,
    `| Entry | #${entry.orderId} ${entry.symbol} ${entry.side} ${entry.type} |`,
    `| Quantity | ${qty} |`,
    `| Avg Price | ${avg} |`,
    `| Status | ${entry.status} |`,
    `| Stop | ${r.stop ? `#${r.stop.orderId} @ $${r.stop.stopPrice}` : "—"} |`,
    `| Take Profit | ${r.take_profit ? `#${r.take_profit.orderId} @ $${r.take_profit.stopPrice}` : "—"} |`,
    `| Trade ID | ${r.trade_id ?? "—"} |`,
  ];
  if (r.warning) lines.push(``, `⚠️ ${r.warning}`);
  if (r.stop) {
    lines.push(``, `保护单已部署到交易所，触发后自动平仓，无需盯盘。`);
  }
  return lines.join("\n");
}

function positive(v: string | undefined): boolean {
  return !!v && Number.isFinite(parseFloat(v)) && parseFloat(v) > 0;
}

function fmt(n: number | undefined): string {
  if (n === undefined || !Number.isFinite(n)) return "—";
  if (Math.abs(n) >= 1000) return n.toFixed(2);
  if (Math.abs(n) >= 1) return n.toFixed(4);
  return n.toFixed(8);
}
