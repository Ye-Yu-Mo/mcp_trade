export interface WatchSnapshot {
  prices: Record<string, number>;
  klines: Record<string, Kline>;
  orderBooks: Record<string, OrderBook>;
}

export interface Kline {
  OpenTime: number;
  Open: number;
  High: number;
  Low: number;
  Close: number;
  Volume: number;
  CloseTime: number;
}

export interface OrderBook {
  Symbol: string;
  Bids: { Price: number; Quantity: number }[];
  Asks: { Price: number; Quantity: number }[];
}

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

export interface Order {
  orderId: number;
  symbol: string;
  status: string;
  type: string;
  side: string;
  price: string;
  origQty: string;
  executedQty: string;
}

export interface TradeRecord {
  ID: number;
  Symbol: string;
  Side: string;
  Price: number;
  Quantity: number;
  Status: string;
  PnL: number;
  CreatedAt: string;
}

export interface PerformanceStats {
  total_trades: number;
  win_trades: number;
  loss_trades: number;
  win_rate: number;
  total_pnl: number;
  avg_pnl: number;
  max_win: number;
  max_loss: number;
  profit_factor: number;
}
