import type { Kline, Ticker, OrderBook, Balance, Position, Order, OrderPreview, TradeRecord, PerformanceStats, JournalEntry } from "./types.js";

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
    quantity: number;
    price?: number;
    stopPrice?: number;
    positionSide?: string;
  }): Promise<OrderPreview> {
    const form = new URLSearchParams();
    form.set("symbol", params.symbol);
    form.set("side", params.side);
    form.set("type", params.type);
    form.set("quantity", String(params.quantity));
    if (params.price) form.set("price", String(params.price));
    if (params.stopPrice) form.set("stop_price", String(params.stopPrice));
    if (params.positionSide) form.set("position_side", params.positionSide);
    return this.post<OrderPreview>("/api/v1/order/place", form);
  }

  async placeOrder(params: {
    symbol: string;
    side: string;
    type: string;
    quantity: number;
    price?: number;
    stopPrice?: number;
    positionSide?: string;
    planId: string;
  }): Promise<Order> {
    const form = new URLSearchParams();
    form.set("symbol", params.symbol);
    form.set("side", params.side);
    form.set("type", params.type);
    form.set("quantity", String(params.quantity));
    if (params.price) form.set("price", String(params.price));
    if (params.stopPrice) form.set("stop_price", String(params.stopPrice));
    if (params.positionSide) form.set("position_side", params.positionSide);
    form.set("confirm", "true");
    form.set("plan_id", params.planId);
    return this.post<Order>("/api/v1/order/place", form);
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
}
