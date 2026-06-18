// --- Market Data Types ---

export interface Kline {
  OpenTime: number;
  Open: number;
  High: number;
  Low: number;
  Close: number;
  Volume: number;
  CloseTime: number;
}

export interface Ticker {
  Symbol: string;
  Price: number;
}

export interface OrderBookLevel {
  Price: number;
  Quantity: number;
}

export interface OrderBook {
  Symbol: string;
  Bids: OrderBookLevel[];
  Asks: OrderBookLevel[];
}

// --- Account Types ---

export interface Balance {
  Asset: string;
  AvailableBalance: number;
  TotalBalance: number;
}

export interface Position {
  Symbol: string;
  Side: string;
  Quantity: number;
  EntryPrice: number;
  MarkPrice: number;
  UnrealizedPnL: number;
  Leverage: number;
}

// --- Order Types ---

export interface Order {
  orderId: number;
  symbol: string;
  status: string;
  clientOrderId: string;
  price: string;
  avgPrice: string;
  origQty: string;
  executedQty: string;
  type: string;
  side: string;
  stopPrice: string;
  timeInForce: string;
  updateTime: number;
}

export interface OrderPreview {
  plan_id: string;
  symbol: string;
  side: string;
  type: string;
  quantity: number;
  price: number;
  stop_price: number;
  risk: {
    passed: boolean;
    checks: string[];
    warnings: string[] | null;
  };
}
