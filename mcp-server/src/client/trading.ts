import type { Kline, Ticker, OrderBook, Balance, Position, Order, OrderPreview, BracketResult, TradeRecord, PerformanceStats, JournalEntry } from "./types.js";

export class TradingClient {
  constructor(
    private baseUrl: string,
    private token: string,
  ) {}

  private async get<T>(path: string): Promise<T> {
    const res = await fetch(`${this.baseUrl}${path}`, {
      headers: { Authorization: `Bearer ${this.token}` },
    });
    const body = await res.json();
    if (!res.ok) {
      const err = body?.error ?? { code: "UNKNOWN", message: `HTTP ${res.status}` };
      throw new Error(`[${err.code}] ${err.message}`);
    }
    return body.data as T;
  }

  private async post<T>(path: string, body: URLSearchParams): Promise<T> {
    const res = await fetch(`${this.baseUrl}${path}`, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${this.token}`,
        "Content-Type": "application/x-www-form-urlencoded",
      },
      body: body.toString(),
    });
    const json = await res.json();
    if (!res.ok) {
      const err = json?.error ?? { code: "UNKNOWN", message: `HTTP ${res.status}` };
      throw new Error(`[${err.code}] ${err.message}`);
    }
    return json.data as T;
  }

  private async del<T>(path: string): Promise<T> {
    const res = await fetch(`${this.baseUrl}${path}`, {
      method: "DELETE",
      headers: { Authorization: `Bearer ${this.token}` },
    });
    const body = await res.json();
    if (!res.ok) {
      const err = body?.error ?? { code: "UNKNOWN", message: `HTTP ${res.status}` };
      throw new Error(`[${err.code}] ${err.message}`);
    }
    return body.data as T;
  }

  async getKlines(symbol: string, interval: string, limit: number): Promise<Kline[]> {
    return this.get<Kline[]>(
      `/api/v1/market/klines?symbol=${encodeURIComponent(symbol)}&interval=${interval}&limit=${limit}`,
    );
  }

  async getCalendar(): Promise<any[]> {
    return this.get<any[]>("/api/v1/market/calendar");
  }

  async setAlert(symbol: string, price: number, direction: string, message: string): Promise<{ id: string }> {
    const form = new URLSearchParams();
    form.set("symbol", symbol);
    form.set("price", String(price));
    form.set("direction", direction);
    form.set("message", message);
    return this.post<{ id: string }>("/api/v1/market/alert", form);
  }

  async getAlerts(): Promise<any[]> {
    return this.get<any[]>("/api/v1/market/alerts");
  }

  async removeAlert(id: string): Promise<void> {
    await this.del<void>(`/api/v1/market/alert?id=${id}`);
  }

  async getOIHistory(symbol: string, period = "15m", limit = 50): Promise<any[]> {
    return this.get<any[]>(
      `/api/v1/market/oi_history?symbol=${symbol}&period=${period}&limit=${limit}`,
    );
  }

  async getLiquidations(opts: {
    symbol?: string;
    minutes?: number;
    limit?: number;
    minNotional?: number;
    summary?: boolean;
  } = {}): Promise<any[]> {
    const p = new URLSearchParams();
    if (opts.symbol) p.set("symbol", opts.symbol);
    p.set("minutes", String(opts.minutes ?? 60));
    p.set("limit", String(opts.limit ?? 50));
    if (opts.minNotional) p.set("min_notional", String(opts.minNotional));
    if (opts.summary) p.set("summary", "true");
    return this.get<any[]>(`/api/v1/market/liquidations?${p.toString()}`);
  }

  async getTicker(symbol: string): Promise<Ticker> {
    return this.get<Ticker>(`/api/v1/market/ticker?symbol=${symbol}`);
  }

  async getOrderBook(symbol: string, limit: number): Promise<OrderBook> {
    return this.get<OrderBook>(`/api/v1/market/orderbook?symbol=${symbol}&limit=${limit}`);
  }

  async getBalance(): Promise<Balance[]> {
    return this.get<Balance[]>("/api/v1/account/balance");
  }

  async getPositions(): Promise<Position[]> {
    return this.get<Position[]>("/api/v1/account/positions");
  }

  // --- Order methods ---

  async previewOrder(params: {
    symbol: string;
    side: string;
    type: string;
    quantity?: number;
    price?: number;
    stopPrice?: number;
    takeProfit?: number;
    riskPct?: number;
    leverage?: number;
    expireMinutes?: number;
    reduceOnly?: boolean;
    closePosition?: boolean;
    entryReason?: string;
    setup?: string;
    tags?: string;
    positionSide?: string;
  }): Promise<OrderPreview> {
    const form = new URLSearchParams();
    form.set("symbol", params.symbol);
    form.set("side", params.side);
    form.set("type", params.type);
    if (params.quantity !== undefined) form.set("quantity", String(params.quantity));
    if (params.price !== undefined) form.set("price", String(params.price));
    if (params.stopPrice !== undefined) form.set("stop_price", String(params.stopPrice));
    if (params.takeProfit !== undefined) form.set("take_profit_price", String(params.takeProfit));
    if (params.riskPct !== undefined) form.set("risk_pct", String(params.riskPct));
    if (params.leverage !== undefined) form.set("leverage", String(params.leverage));
    if (params.expireMinutes !== undefined) form.set("expire_minutes", String(params.expireMinutes));
    if (params.reduceOnly) form.set("reduce_only", "true");
    if (params.closePosition) form.set("close_position", "true");
    if (params.entryReason) form.set("entry_reason", params.entryReason);
    if (params.setup) form.set("setup", params.setup);
    if (params.tags) form.set("tags", params.tags);
    if (params.positionSide) form.set("position_side", params.positionSide);
    return this.post<OrderPreview>("/api/v1/order/place", form);
  }

  async placeOrder(params: {
    symbol: string;
    side: string;
    type: string;
    quantity?: number;
    price?: number;
    stopPrice?: number;
    takeProfit?: number;
    riskPct?: number;
    leverage?: number;
    expireMinutes?: number;
    reduceOnly?: boolean;
    closePosition?: boolean;
    entryReason?: string;
    setup?: string;
    tags?: string;
    positionSide?: string;
    planId: string;
  }): Promise<BracketResult> {
    const form = new URLSearchParams();
    form.set("symbol", params.symbol);
    form.set("side", params.side);
    form.set("type", params.type);
    if (params.quantity !== undefined) form.set("quantity", String(params.quantity));
    if (params.price !== undefined) form.set("price", String(params.price));
    if (params.stopPrice !== undefined) form.set("stop_price", String(params.stopPrice));
    if (params.takeProfit !== undefined) form.set("take_profit_price", String(params.takeProfit));
    if (params.riskPct !== undefined) form.set("risk_pct", String(params.riskPct));
    if (params.leverage !== undefined) form.set("leverage", String(params.leverage));
    if (params.expireMinutes !== undefined) form.set("expire_minutes", String(params.expireMinutes));
    if (params.reduceOnly) form.set("reduce_only", "true");
    if (params.closePosition) form.set("close_position", "true");
    if (params.entryReason) form.set("entry_reason", params.entryReason);
    if (params.setup) form.set("setup", params.setup);
    if (params.tags) form.set("tags", params.tags);
    if (params.positionSide) form.set("position_side", params.positionSide);
    form.set("confirm", "true");
    form.set("plan_id", params.planId);
    return this.post<BracketResult>("/api/v1/order/place", form);
  }

  async closePosition(symbol: string, quantity?: number, reason?: string): Promise<Order> {
    const form = new URLSearchParams();
    form.set("symbol", symbol);
    if (quantity !== undefined && quantity > 0) form.set("quantity", String(quantity));
    if (reason) form.set("reason", reason);
    return this.post<Order>("/api/v1/order/close", form);
  }

  async cancelOrder(symbol: string, orderId: number): Promise<Order> {
    return this.del<Order>(`/api/v1/order/cancel?symbol=${symbol}&order_id=${orderId}`);
  }

  async getOpenOrders(symbol?: string): Promise<Order[]> {
    const path = symbol
      ? `/api/v1/order/list?symbol=${symbol}`
      : "/api/v1/order/list";
    return this.get<Order[]>(path);
  }

  async getOrder(symbol: string, orderId: number): Promise<Order> {
    return this.get<Order>(`/api/v1/order/status?symbol=${symbol}&order_id=${orderId}`);
  }

  // --- Trade journal methods ---

  async getTradeHistory(symbol?: string, limit = 50): Promise<TradeRecord[]> {
    const path = symbol
      ? `/api/v1/trade/history?symbol=${symbol}&limit=${limit}`
      : `/api/v1/trade/history?limit=${limit}`;
    return this.get<TradeRecord[]>(path);
  }

  async getExchangeIncome(hours = 48, symbol?: string, limit = 500): Promise<any[]> {
    const start = Date.now() - hours * 3600_000;
    let path = `/api/v1/trade/exchange?start_time=${Math.floor(start)}&limit=${limit}`;
    if (symbol) path += `&symbol=${symbol}`;
    return this.get<any[]>(path);
  }

  async postJournal(entryType: string, reason: string, tags?: string, tradeId?: number): Promise<{ id: number }> {
    const form = new URLSearchParams();
    form.set("entry_type", entryType);
    form.set("reason", reason);
    if (tags) form.set("tags", tags);
    if (tradeId) form.set("trade_id", String(tradeId));
    return this.post<{ id: number }>("/api/v1/trade/journal", form);
  }

  async getJournals(limit = 20, entryType?: string, tags?: string): Promise<JournalEntry[]> {
    let path = `/api/v1/trade/journal?limit=${limit}`;
    if (entryType) path += `&entry_type=${entryType}`;
    if (tags) path += `&tags=${encodeURIComponent(tags)}`;
    return this.get<JournalEntry[]>(path);
  }

  async getPerformance(): Promise<PerformanceStats> {
    return this.get<PerformanceStats>("/api/v1/trade/performance");
  }

  async getWatch(): Promise<any> {
    return this.get<any>("/api/v1/market/watch");
  }

  async getScanner(limit = 20): Promise<any[]> {
    return this.get<any[]>(`/api/v1/market/scanner?limit=${limit}`);
  }

  async getFundingRate(symbol = "BTCUSDT"): Promise<any> {
    return this.get<any>(`/api/v1/market/funding?symbol=${symbol}`);
  }

  async getOpenInterest(symbol = "BTCUSDT"): Promise<any> {
    return this.get<any>(`/api/v1/market/oi?symbol=${symbol}`);
  }

  async getATR(symbol: string, interval = "1h", period = 14): Promise<any> {
    return this.get<any>(`/api/v1/market/atr?symbol=${symbol}&interval=${interval}&period=${period}`);
  }

  async getCandleInfo(symbol: string, interval = "1h"): Promise<any> {
    return this.get<any>(`/api/v1/market/candle_info?symbol=${symbol}&interval=${interval}`);
  }

  async modifyStop(params: {
    symbol: string;
    oldOrderId: number;
    newStopPrice: number;
    side: string;
    quantity: number;
    positionSide?: string;
  }): Promise<any> {
    const form = new URLSearchParams();
    form.set("symbol", params.symbol);
    form.set("old_order_id", String(params.oldOrderId));
    form.set("new_stop_price", String(params.newStopPrice));
    form.set("side", params.side);
    form.set("quantity", String(params.quantity));
    if (params.positionSide) form.set("position_side", params.positionSide);
    return this.post<any>("/api/v1/order/modify_stop", form);
  }

  async createOCO(params: {
    symbol: string;
    side: string;
    quantity: number;
    price: number;
    stopPrice: number;
  }): Promise<any> {
    const form = new URLSearchParams();
    form.set("symbol", params.symbol);
    form.set("side", params.side);
    form.set("quantity", String(params.quantity));
    form.set("price", String(params.price));
    form.set("stop_price", String(params.stopPrice));
    return this.post<any>("/api/v1/order/oco", form);
  }

  async cancelOCO(symbol: string, orderListId: number): Promise<void> {
    await this.del<void>(`/api/v1/order/oco?symbol=${symbol}&order_list_id=${orderListId}`);
  }
}
