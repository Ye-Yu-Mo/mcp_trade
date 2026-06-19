import type { WatchSnapshot, Balance, Position, Order, TradeRecord, PerformanceStats } from "./types";

export class ApiClient {
  private baseUrl: string;
  private token: string;

  constructor() {
    this.baseUrl = localStorage.getItem("trade_server_url") || "";
    this.token = localStorage.getItem("trade_api_token") || "";
  }

  setConfig(url: string, token: string) {
    this.baseUrl = url;
    this.token = token;
    localStorage.setItem("trade_server_url", url);
    localStorage.setItem("trade_api_token", token);
  }

  get configured() { return !!this.baseUrl && !!this.token; }

  private async get<T>(path: string): Promise<T | null> {
    try {
      const res = await fetch(`${this.baseUrl}${path}`, {
        headers: { Authorization: `Bearer ${this.token}` },
      });
      if (!res.ok) return null;
      const body = await res.json();
      return body.data as T;
    } catch { return null; }
  }

  watch() { return this.get<WatchSnapshot>("/api/v1/market/watch"); }
  balance() { return this.get<Balance[]>("/api/v1/account/balance"); }
  positions() { return this.get<Position[]>("/api/v1/account/positions"); }
  openOrders() { return this.get<Order[]>("/api/v1/order/list"); }
  tradeHistory(limit = 50) { return this.get<TradeRecord[]>(`/api/v1/trade/history?limit=${limit}`); }
  performance() { return this.get<PerformanceStats>("/api/v1/trade/performance"); }
  alerts() { return this.get<any[]>("/api/v1/market/alerts"); }
}

export const api = new ApiClient();
