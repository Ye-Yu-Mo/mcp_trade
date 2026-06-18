import { z } from "zod/v4";
import type { TradingClient } from "../client/trading.js";
import type { Order, OrderPreview } from "../client/types.js";

export function registerOrderTools(server: any, client: TradingClient) {
  // --- order.place ---
  server.registerTool(
    "order.place",
    {
      description:
        "下单工具。两步流程：(1) 先不带 confirm 参数调用，获取订单预览和风控检查结果 (2) 用户确认后，带 confirm=true 和 plan_id 执行下单。" +
        "不得在用户未确认时自行带 confirm=true。遇到风控拒绝必须向用户说明原因。",
      inputSchema: {
        symbol: z.string().describe("交易对，如 BTCUSDT"),
        side: z.string().describe("方向：BUY 或 SELL"),
        type: z.string().describe("订单类型：LIMIT / MARKET / STOP_MARKET"),
        quantity: z.number().describe("数量"),
        price: z.number().optional().describe("限价（LIMIT 必填）"),
        stop_price: z.number().optional().describe("止损价（STOP_MARKET 必填）"),
        position_side: z.string().optional().describe("持仓方向：LONG / SHORT，默认 LONG"),
        confirm: z.boolean().optional().describe("用户已确认时传 true"),
        plan_id: z.string().optional().describe("从预览返回的 plan_id，confirm=true 时必填"),
      },
    },
    async (args: {
      symbol: string;
      side: string;
      type: string;
      quantity: number;
      price?: number;
      stop_price?: number;
      position_side?: string;
      confirm?: boolean;
      plan_id?: string;
    }) => {
      // Plan phase
      if (!args.confirm) {
        const preview = await client.previewOrder({
          symbol: args.symbol,
          side: args.side,
          type: args.type,
          quantity: args.quantity,
          price: args.price,
          stopPrice: args.stop_price,
          positionSide: args.position_side,
        });

        const riskIcon = preview.risk.passed ? "✅" : "❌";
        const checks = preview.risk.checks.join(", ");
        const text = [
          `## Order Preview`,
          `| Field | Value |`,
          `|-------|-------|`,
          `| Symbol | ${preview.symbol} |`,
          `| Side | ${preview.side} |`,
          `| Type | ${preview.type} |`,
          `| Quantity | ${preview.quantity} |`,
          `| Price | ${preview.price || "market"} |`,
          `| Stop | ${preview.stop_price || "-"} |`,
          `| Plan ID | \`${preview.plan_id}\` |`,
          ``,
          `### Risk Check: ${riskIcon} ${preview.risk.passed ? "PASSED" : "REJECTED"}`,
          checks,
        ].join("\n");

        return {
          content: [{ type: "text" as const, text }],
          structuredContent: preview,
        };
      }

      // Apply phase
      if (!args.plan_id) {
        throw new Error("plan_id is required when confirm=true");
      }

      const order = await client.placeOrder({
        symbol: args.symbol,
        side: args.side,
        type: args.type,
        quantity: args.quantity,
        price: args.price,
        stopPrice: args.stop_price,
        positionSide: args.position_side,
        planId: args.plan_id,
      });

      const text = [
        `## Order Placed`,
        `| Field | Value |`,
        `|-------|-------|`,
        `| Order ID | ${order.orderId} |`,
        `| Symbol | ${order.symbol} |`,
        `| Side | ${order.side} |`,
        `| Type | ${order.type} |`,
        `| Qty | ${order.origQty} |`,
        `| Price | ${order.price} |`,
        `| Status | ${order.status} |`,
      ].join("\n");

      return {
        content: [{ type: "text" as const, text }],
        structuredContent: order,
      };
    },
  );

  // --- order.cancel ---
  server.registerTool(
    "order.cancel",
    {
      description:
        "撤销指定订单。传入 symbol 和 order_id 取消未成交的挂单。用于撤销错误下单或过期的限价单。",
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
      description:
        "查询当前活跃订单（挂单）。可选按交易对过滤。用于查看当前有哪些挂单、判断是否需要撤单。",
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
      const rows = orders.map(
        (o) =>
          `| #${o.orderId} | ${o.symbol} | ${o.side} | ${o.type} | ${o.origQty} @ ${o.price} | ${o.status} |`,
      );
      const header = "| Order ID | Symbol | Side | Type | Qty @ Price | Status |";
      const sep = "|---|---|---|---|---|---|";
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
        "修改已有仓位的止损价。两步操作：(1) 撤销旧的止损单 (2) 下新的 STOP_MARKET 止损单（reduce_only）。用于价格向有利方向移动后将止损移到保本或更好位置。" +
        "前置条件：需要知道旧的止损单 order_id（通过 order.list 查询当前挂单获取）。",
      inputSchema: {
        symbol: z.string().describe("交易对，如 BTCUSDT"),
        old_order_id: z.number().describe("要替换的旧止损单 ID"),
        new_stop_price: z.number().describe("新的止损价格"),
        side: z.string().describe("SELL（做多止损）或 BUY（做空止损）"),
        quantity: z.number().describe("持仓数量"),
        position_side: z.string().optional().describe("LONG / SHORT，默认 LONG"),
      },
    },
    async (args: { symbol: string; old_order_id: number; new_stop_price: number; side: string; quantity: number; position_side?: string }) => {
      const result = await client.modifyStop({
        symbol: args.symbol,
        oldOrderId: args.old_order_id,
        newStopPrice: args.new_stop_price,
        side: args.side,
        quantity: args.quantity,
        positionSide: args.position_side,
      });
      const text = [
        `## Stop Loss Modified`,
        `| Field | Value |`,
        `|-------|-------|`,
        `| Symbol | ${args.symbol} |`,
        `| Old Order | #${args.old_order_id} |`,
        `| New Stop | $${args.new_stop_price} |`,
        `| New Order | #${result.new_stop.orderId} |`,
        `| Status | ${result.new_stop.status} |`,
      ].join("\n");
      return {
        content: [{ type: "text" as const, text }],
        structuredContent: result,
      };
    },
  );

  // --- order.oco ---
  server.registerTool(
    "order.oco",
    {
      description:
        "OCO 订单（One-Cancels-Other）：同时挂止盈限价单和止损市价单，一个触发另一个自动取消。入场后必须立即设 OCO——AI 不在场内盯盘，退出条件必须在入场时就写入币安服务器。参数：symbol, side(SELL=平多/BUY=平空), quantity, price(止盈价), stop_price(止损价)。",
      inputSchema: {
        symbol: z.string().describe("交易对，如 BTCUSDT"),
        side: z.string().describe("SELL（平多单止盈+止损）或 BUY（平空单止盈+止损）"),
        quantity: z.number().describe("数量"),
        price: z.number().describe("止盈价格"),
        stop_price: z.number().describe("止损价格"),
      },
    },
    async (args: { symbol: string; side: string; quantity: number; price: number; stop_price: number }) => {
      const result = await client.createOCO({
        symbol: args.symbol,
        side: args.side,
        quantity: args.quantity,
        price: args.price,
        stopPrice: args.stop_price,
      });
      const text = [
        `## OCO Order Placed`,
        `| Field | Value |`,
        `|-------|-------|`,
        `| OCO ID | ${result.orderListId} |`,
        `| Symbol | ${args.symbol} |`,
        `| Qty | ${args.quantity} |`,
        `| Take Profit | $${args.price} |`,
        `| Stop Loss | $${args.stop_price} |`,
        `| Stop Order | #${result.stop_order?.orderId} |`,
        `| Limit Order | #${result.limit_order?.orderId} |`,
        ``,
        `⚠️ 止盈和止损已部署到币安服务器。触发后自动执行，无需 AI 盯盘。`,
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
      description:
        "查询单个订单的详细状态。返回订单的成交状态（NEW/PARTIALLY_FILLED/FILLED/CANCELED）。用于跟踪下单后的成交进度。",
      inputSchema: {
        symbol: z.string().describe("交易对，如 BTCUSDT"),
        order_id: z.number().describe("订单 ID"),
      },
    },
    async (args: { symbol: string; order_id: number }) => {
      const o = await client.getOrder(args.symbol, args.order_id);
      const pct = (parseFloat(o.executedQty) / parseFloat(o.origQty) * 100).toFixed(1);
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
}
