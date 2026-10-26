package ws

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/binance"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/store"
)

// UserDataStream connects to the Binance User Data Stream for account events.
type UserDataStream struct {
	connected atomic.Bool
	client    UserDataClient
	store     TradeStore
	cache     *MarketCache
	baseURL   string
	done      chan struct{}
}

// UserDataClient abstracts the Binance client for listen key management.
type UserDataClient interface {
	GetListenKey() (string, error)
	KeepAliveListenKey(listenKey string) error
	GetPositions() ([]binance.Position, error)
}

// TradeStore abstracts the store for the position-level trade lifecycle.
type TradeStore interface {
	FindOpenTradeBySymbol(symbol string) (*store.TradeRecord, error)
	CloseTrade(tradeID int64, exitPrice, pnl, fees, rMultiple float64, exitReason string) error
}

// NewUserDataStream creates a user data stream handler.
func NewUserDataStream(client UserDataClient, store TradeStore, cache *MarketCache, baseURL string) *UserDataStream {
	return &UserDataStream{
		client:  client,
		store:   store,
		cache:   cache,
		baseURL: baseURL,
		done:    make(chan struct{}),
	}
}

func (u *UserDataStream) Start()          { go u.loop() }
func (u *UserDataStream) Connected() bool { return u.connected.Load() }
func (u *UserDataStream) Stop()           { close(u.done) }

func (u *UserDataStream) loop() {
	backoff := 1 * time.Second
	const maxBackoff = 30 * time.Second

	for {
		select {
		case <-u.done:
			return
		default:
		}

		listenKey, err := u.client.GetListenKey()
		if err != nil {
			log.Printf("ws userdata: get listenKey: %v, retrying in %v", err, backoff)
			time.Sleep(backoff)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		// Keep-alive is scoped to this connection: close it before reconnecting
		// so old goroutines cannot keep extending a stale listen key.
		keepAliveDone := make(chan struct{})
		go func() {
			ticker := time.NewTicker(30 * time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-u.done:
					return
				case <-keepAliveDone:
					return
				case <-ticker.C:
					if err := u.client.KeepAliveListenKey(listenKey); err != nil {
						log.Printf("ws userdata: keepalive: %v", err)
					}
				}
			}
		}()

		err = u.connect(listenKey)
		close(keepAliveDone)
		if err != nil {
			log.Printf("ws userdata: %v, reconnecting in %v", err, backoff)
			time.Sleep(backoff)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		} else {
			backoff = 1 * time.Second
		}
	}
}

func (u *UserDataStream) connect(listenKey string) error {
	// Use testnet WS for testnet, mainnet WS for mainnet
	wsHost := "wss://fstream.binance.com/ws"
	if strings.Contains(u.baseURL, "testnet") {
		wsHost = "wss://stream.binancefuture.com/ws"
	}
	wsURL := fmt.Sprintf("%s/%s", wsHost, listenKey)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()
	log.Printf("ws userdata: connected to %s", wsHost)
	u.connected.Store(true)
	defer u.connected.Store(false)

	// The user data stream can be silent for minutes. Keep the connection
	// alive with control pings and use a generous read deadline; otherwise a
	// 60s deadline forces a reconnect even when the socket is healthy.
	pingDone := make(chan struct{})
	defer close(pingDone)
	go func() {
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-u.done:
				return
			case <-pingDone:
				return
			case <-ticker.C:
				conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second))
			}
		}
	}()

	for {
		select {
		case <-u.done:
			return nil
		default:
		}

		conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}
		u.handleMessage(msg)
	}
}

func (u *UserDataStream) handleMessage(msg []byte) {
	var event struct {
		EventType string `json:"e"`
	}
	if json.Unmarshal(msg, &event) != nil {
		return
	}
	switch event.EventType {
	case "ORDER_TRADE_UPDATE":
		u.handleOrderUpdate(msg)
	case "ALGO_UPDATE":
		u.handleAlgoUpdate(msg)
	case "ACCOUNT_UPDATE":
		u.handleAccountUpdate(msg)
	}
}

func (u *UserDataStream) handleOrderUpdate(msg []byte) {
	var evt struct {
		Order struct {
			Symbol        string `json:"s"`
			OrderID       int64  `json:"i"`
			Status        string `json:"X"`
			Side          string `json:"S"`
			OrderType     string `json:"o"`
			AvgPrice      string `json:"ap"`
			OrigQty       string `json:"q"`
			RealizedPnL   string `json:"rp"`
			Commission    string `json:"n"`
			ReduceOnly    bool   `json:"R"`
			ClosePosition bool   `json:"cp"`
		} `json:"o"`
	}
	if json.Unmarshal(msg, &evt) != nil {
		return
	}
	if evt.Order.Status != "FILLED" {
		return
	}

	isReduce := evt.Order.ReduceOnly || evt.Order.ClosePosition ||
		strings.Contains(evt.Order.OrderType, "STOP") ||
		strings.Contains(evt.Order.OrderType, "TAKE_PROFIT")
	if !isReduce {
		// Entry fills are journaled by the order API, which has the full
		// thesis/stop/target metadata.
		return
	}

	trade, err := u.store.FindOpenTradeBySymbol(evt.Order.Symbol)
	if err != nil {
		log.Printf("ws userdata: find trade %s: %v", evt.Order.Symbol, err)
		return
	}
	if trade == nil {
		return
	}

	// Only close the position-level trade when the position is actually flat;
	// a partial reduce leaves the trade OPEN.
	positions, err := u.client.GetPositions()
	if err != nil {
		log.Printf("ws userdata: positions check failed for %s: %v; leaving trade open",
			evt.Order.Symbol, err)
		return
	}
	var qty float64
	for _, p := range positions {
		if p.Symbol == evt.Order.Symbol {
			qty += p.Quantity
		}
	}
	if qty > 1e-9 {
		log.Printf("ws userdata: partial reduce on %s (%.8f left); trade %d stays OPEN",
			evt.Order.Symbol, qty, trade.ID)
		return
	}

	pnl, _ := strconv.ParseFloat(evt.Order.RealizedPnL, 64)
	commission, _ := strconv.ParseFloat(evt.Order.Commission, 64)
	avgPrice, _ := strconv.ParseFloat(evt.Order.AvgPrice, 64)

	netPnL := pnl - commission
	rMultiple := 0.0
	if trade.RiskAmount > 0 {
		rMultiple = netPnL / trade.RiskAmount
	}
	reason := "EXCHANGE"
	switch {
	case strings.Contains(evt.Order.OrderType, "TAKE_PROFIT"):
		reason = "TAKE_PROFIT"
	case strings.Contains(evt.Order.OrderType, "STOP"):
		reason = "STOP"
	}
	if err := u.store.CloseTrade(trade.ID, avgPrice, netPnL, commission, rMultiple, reason); err != nil {
		log.Printf("ws userdata: close trade %d: %v", trade.ID, err)
		return
	}
	log.Printf("ws userdata: closed trade %d %s pnl=%.4f (%.2fR) reason=%s",
		trade.ID, evt.Order.Symbol, netPnL, rMultiple, reason)
}

func (u *UserDataStream) handleAlgoUpdate(msg []byte) {
	var evt struct {
		Algo struct {
			ClientAlgoID    string `json:"caid"`
			AlgoID          int64  `json:"aid"`
			AlgoType        string `json:"at"`
			OrderType       string `json:"o"`
			Symbol          string `json:"s"`
			Side            string `json:"S"`
			PositionSide    string `json:"ps"`
			Status          string `json:"X"`
			ClosePosition   bool   `json:"cp"`
			ReduceOnly      bool   `json:"R"`
			MatchingOrderID string `json:"ai"`
			AvgPrice        string `json:"ap"`
			ExecutedQty     string `json:"aq"`
		} `json:"o"`
	}
	if json.Unmarshal(msg, &evt) != nil {
		return
	}
	if evt.Algo.Status != "FINISHED" {
		return
	}
	if !evt.Algo.ClosePosition && !evt.Algo.ReduceOnly {
		return
	}
	executed, _ := strconv.ParseFloat(evt.Algo.ExecutedQty, 64)
	if executed <= 0 {
		return
	}

	trade, err := u.store.FindOpenTradeBySymbol(evt.Algo.Symbol)
	if err != nil {
		log.Printf("ws userdata: ALGO find trade %s: %v", evt.Algo.Symbol, err)
		return
	}
	if trade == nil {
		// The matching-engine ORDER_TRADE_UPDATE may have closed it already.
		return
	}

	positions, err := u.client.GetPositions()
	if err != nil {
		log.Printf("ws userdata: ALGO positions check %s: %v; leaving trade open", evt.Algo.Symbol, err)
		return
	}
	var remaining float64
	for _, p := range positions {
		if p.Symbol == evt.Algo.Symbol {
			remaining += p.Quantity
		}
	}
	if remaining > 1e-9 {
		log.Printf("ws userdata: ALGO partial close %s (%.8f left); trade %d stays OPEN",
			evt.Algo.Symbol, remaining, trade.ID)
		return
	}

	avgPrice, _ := strconv.ParseFloat(evt.Algo.AvgPrice, 64)
	if avgPrice <= 0 {
		log.Printf("ws userdata: ALGO finished without avg price for %s", evt.Algo.Symbol)
		return
	}
	var gross float64
	if trade.Side == "LONG" {
		gross = (avgPrice - trade.Price) * executed
	} else {
		gross = (trade.Price - avgPrice) * executed
	}
	// Best-effort fee estimate (taker on both sides); the matching-engine
	// ORDER_TRADE_UPDATE path is more precise when it arrives first.
	fees := (trade.Price + avgPrice) * executed * 0.0005
	net := gross - fees
	rMultiple := 0.0
	if trade.RiskAmount > 0 {
		rMultiple = net / trade.RiskAmount
	}
	reason := "ALGO"
	if strings.Contains(evt.Algo.OrderType, "TAKE_PROFIT") {
		reason = "TAKE_PROFIT"
	} else if strings.Contains(evt.Algo.OrderType, "STOP") {
		reason = "STOP"
	}
	if err := u.store.CloseTrade(trade.ID, avgPrice, net, fees, rMultiple, reason); err != nil {
		log.Printf("ws userdata: ALGO close trade %d: %v", trade.ID, err)
		return
	}
	log.Printf("ws userdata: ALGO closed trade %d %s pnl=%.4f (%.2fR) reason=%s",
		trade.ID, evt.Algo.Symbol, net, rMultiple, reason)
}

func (u *UserDataStream) handleAccountUpdate(msg []byte) {
	var evt struct {
		Account struct {
			Balances []struct {
				Asset         string `json:"a"`
				WalletBalance string `json:"wb"`
			} `json:"B"`
		} `json:"a"`
	}
	if json.Unmarshal(msg, &evt) != nil {
		return
	}
	if u.cache == nil {
		return
	}
	for _, b := range evt.Account.Balances {
		bal, _ := strconv.ParseFloat(b.WalletBalance, 64)
		u.cache.SetPrice(b.Asset+"_balance", bal)
	}
}
