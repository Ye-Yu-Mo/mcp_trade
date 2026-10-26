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
  positionSide?: string;
  stopPrice: string;
  timeInForce: string;
  reduceOnly?: boolean;
  closePosition?: boolean;
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
  take_profit: number;
  notional: number;
  risk_amount: number;
  risk_pct: number;
  margin: number;
  leverage: number;
  reduce_only: boolean;
  close_position: boolean;
  equity: number;
  available: number;
  open_risk: number;
  open_positions: number;
  daily_pnl: number;
  risk: {
    passed: boolean;
    code?: string;
    reason?: string;
    checks: string[];
    warnings: string[] | null;
  };
}

export interface BracketResult {
  entry: Order;
  stop?: Order;
  take_profit?: Order;
  trade_id?: number;
  warning?: string;
  risk: OrderPreview;
}

// --- Trade Journal Types ---

export interface TradeRecord {
  ID: number;
  Symbol: string;
  Side: string;
  Quantity: number;
  Price: number;
  OrderID: string;
  Status: string;
  EntryReason: string;
  ExitReason: string;
  PnL: number;
  MarketSnapshot: string;
  StopPrice?: number;
  TakeProfit?: number;
  ExitPrice?: number;
  Setup?: string;
  Tags?: string;
  Leverage?: number;
  RiskAmount?: number;
  RMultiple?: number;
  Fees?: number;
  CreatedAt: string;
  ClosedAt?: string | null;
  UpdatedAt: string;
}

export interface PerformanceStats {
  total_trades: number;
  win_trades: number;
  loss_trades: number;
  breakeven_trades: number;
  win_rate: number;
  total_pnl: number;
  avg_pnl: number;
  max_win: number;
  max_loss: number;
  profit_factor: number;
  profit_factor_infinite: boolean;
  avg_r: number;
  expectancy_r: number;
  max_drawdown: number;
}

export interface JournalEntry {
  ID: number;
  TradeID: number | null;
  EntryType: string;
  Reason: string;
  Tags: string;
  CreatedAt: string;
}
