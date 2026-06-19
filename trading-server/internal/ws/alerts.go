package ws

import (
	"sync"
	"time"

	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/binance"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/store"
)

type PriceAlert struct {
	ID        string    `json:"id"`
	Symbol    string    `json:"symbol"`
	Price     float64   `json:"price"`
	Direction string    `json:"direction"`
	Message   string    `json:"message"`
	Triggered bool      `json:"triggered"`
	CreatedAt time.Time `json:"created_at"`
}

type AlertPersister interface {
	InsertAlert(id, symbol, direction, message string, price float64) error
	QueryAlerts() ([]store.Alert, error)
	UpdateAlertTriggered(id string) error
	DeleteAlert(id string) error
}

type AlertStore struct {
	mu      sync.RWMutex
	alerts  map[string]*PriceAlert
	cache   *MarketCache
	store   AlertPersister
	trader  binance.Trader
	counter int
}

func NewAlertStore(cache *MarketCache, store AlertPersister, trader binance.Trader) *AlertStore {
	a := &AlertStore{
		alerts: make(map[string]*PriceAlert),
		cache:  cache,
		store:  store,
		trader: trader,
	}
	if a.store != nil {
		if records, err := a.store.QueryAlerts(); err == nil {
			for _, r := range records {
				a.alerts[r.ID] = &PriceAlert{
					ID: r.ID, Symbol: r.Symbol, Price: r.Price,
					Direction: r.Direction, Message: r.Message,
					Triggered: r.Triggered, CreatedAt: r.CreatedAt,
				}
			}
			a.counter = len(records)
		}
	}
	go a.checkLoop()
	return a
}

func (a *AlertStore) Add(symbol string, price float64, direction string, message string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.counter++
	id := generateID(a.counter)
	a.alerts[id] = &PriceAlert{ID: id, Symbol: symbol, Price: price, Direction: direction, Message: message, CreatedAt: time.Now()}
	if a.store != nil {
		a.store.InsertAlert(id, symbol, direction, message, price)
	}
	return id
}

func (a *AlertStore) List() []*PriceAlert {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]*PriceAlert, 0, len(a.alerts))
	for _, alert := range a.alerts {
		result = append(result, alert)
	}
	return result
}

func (a *AlertStore) ListTriggered() []*PriceAlert {
	a.mu.RLock()
	defer a.mu.RUnlock()
	var result []*PriceAlert
	for _, alert := range a.alerts {
		if alert.Triggered {
			result = append(result, alert)
		}
	}
	return result
}

func (a *AlertStore) Remove(id string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, ok := a.alerts[id]
	if ok {
		delete(a.alerts, id)
		if a.store != nil {
			a.store.DeleteAlert(id)
		}
	}
	return ok
}

func (a *AlertStore) checkLoop() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		a.mu.Lock()
		for _, alert := range a.alerts {
			if alert.Triggered {
				continue
			}
			// Cache first, REST fallback for symbols not in watch list
			price, ok := a.cache.GetPrice(alert.Symbol)
			if !ok && a.trader != nil {
				if tk, err := a.trader.GetTicker(alert.Symbol); err == nil {
					price = tk.Price
					ok = true
				}
			}
			if !ok {
				continue
			}
			hit := (alert.Direction == "ABOVE" && price >= alert.Price) || (alert.Direction == "BELOW" && price <= alert.Price)
			if hit {
				alert.Triggered = true
				if a.store != nil {
					a.store.UpdateAlertTriggered(alert.ID)
				}
			}
		}
		a.mu.Unlock()
	}
}

func generateID(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, 6)
	for i := range result {
		result[i] = chars[n%len(chars)]
		n = n / len(chars)
	}
	return string(result)
}
