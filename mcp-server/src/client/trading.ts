import type { Kline, Ticker, OrderBook, Balance, Position } from "./types.js";

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

  async getKlines(symbol: string, interval: string, limit: number): Promise<Kline[]> {
    return this.get<Kline[]>(
      `/api/v1/market/klines?symbol=${symbol}&interval=${interval}&limit=${limit}`,
    );
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
}
