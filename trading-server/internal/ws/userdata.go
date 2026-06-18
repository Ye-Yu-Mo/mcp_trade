package ws

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// UserDataStream connects to the Binance User Data Stream for account events.
type UserDataStream struct {
	client  UserDataClient
	store   TradeStore
	cache   *MarketCache
	baseURL string
	done    chan struct{}
}

// UserDataClient abstracts the Binance client for listen key management.
type UserDataClient interface {
	GetListenKey() (string, error)
	KeepAliveListenKey(listenKey string) error
}

// TradeStore abstracts the store for updating trade records.
type TradeStore interface {
	UpdateTradeByOrderID(orderID, status string, pnl float64, avgPrice float64) error
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

func (u *UserDataStream) Start() { go u.loop() }
func (u *UserDataStream) Stop()  { close(u.done) }

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

		// Keep-alive every 30 min
		go func() {
			ticker := time.NewTicker(30 * time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-u.done:
					return
				case <-ticker.C:
					u.client.KeepAliveListenKey(listenKey)
				}
			}
		}()

		if err := u.connect(listenKey); err != nil {
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
		wsHost = "wss://testnet.binancefuture.com/ws"
	}
	wsURL := fmt.Sprintf("%s/%s", wsHost, listenKey)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()
	log.Printf("ws userdata: connected to %s", wsHost)

	for {
		select {
		case <-u.done:
			return nil
		default:
		}

		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}
		u.handleMessage(msg)
	}
}

func (u *UserDataStream) handleMessage(msg []byte) {
	var event struct{ EventType string `json:"e"` }
	if json.Unmarshal(msg, &event) != nil {
		return
	}
	switch event.EventType {
	case "ORDER_TRADE_UPDATE":
		u.handleOrderUpdate(msg)
	case "ACCOUNT_UPDATE":
		u.handleAccountUpdate(msg)
	}
}

func (u *UserDataStream) handleOrderUpdate(msg []byte) {
	var evt struct {
		Order struct {
			Symbol       string `json:"s"`
			OrderID      int64  `json:"i"`
			Status       string `json:"X"`
			Side         string `json:"S"`
			AvgPrice     string `json:"ap"`
			OrigQty      string `json:"q"`
			RealizedPnL  string `json:"rp"`
		} `json:"o"`
	}
	if json.Unmarshal(msg, &evt) != nil {
		return
	}

	orderID := strconv.FormatInt(evt.Order.OrderID, 10)
	status := evt.Order.Status

	if status == "FILLED" || status == "CANCELED" || status == "EXPIRED" {
		pnl, _ := strconv.ParseFloat(evt.Order.RealizedPnL, 64)
		avgPrice, _ := strconv.ParseFloat(evt.Order.AvgPrice, 64)
		log.Printf("ws userdata: order %s %s, pnl=%.2f", orderID, status, pnl)
		u.store.UpdateTradeByOrderID(orderID, status, pnl, avgPrice)
	}
}

func (u *UserDataStream) handleAccountUpdate(msg []byte) {
	var evt struct {
		Balances []struct {
			Asset  string `json:"a"`
			Balance string `json:"wb"`
		} `json:"a"`
	}
	if json.Unmarshal(msg, &evt) != nil {
		return
	}
	for _, b := range evt.Balances {
		bal, _ := strconv.ParseFloat(b.Balance, 64)
		u.cache.SetPrice(b.Asset+"_balance", bal)
	}
}
