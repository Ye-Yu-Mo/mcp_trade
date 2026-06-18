package ws

import (
	"sync"
	"time"
)

// PriceAlert represents a user-set price alert.
type PriceAlert struct {
	ID        string    `json:"id"`
	Symbol    string    `json:"symbol"`
	Price     float64   `json:"price"`
	Direction string    `json:"direction"` // ABOVE / BELOW
	Message   string    `json:"message"`
	Triggered bool      `json:"triggered"`
	CreatedAt time.Time `json:"created_at"`
}

// AlertStore manages price alerts with automatic trigger detection.
type AlertStore struct {
	mu      sync.RWMutex
	alerts  map[string]*PriceAlert
	cache   *MarketCache
	counter int
}

// NewAlertStore creates an alert store that checks against the market cache.
func NewAlertStore(cache *MarketCache) *AlertStore {
	a := &AlertStore{
		alerts: make(map[string]*PriceAlert),
		cache:  cache,
	}
	go a.checkLoop()
	return a
}

// Add creates a new price alert. Returns the alert ID.
func (a *AlertStore) Add(symbol string, price float64, direction, message string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.counter++
	id := generateID(a.counter)
	a.alerts[id] = &PriceAlert{
		ID:        id,
		Symbol:    symbol,
		Price:     price,
		Direction: direction,
		Message:   message,
		CreatedAt: time.Now(),
	}
	return id
}

// List returns all alerts, most recent first.
func (a *AlertStore) List() []*PriceAlert {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]*PriceAlert, 0, len(a.alerts))
	for _, alert := range a.alerts {
		result = append(result, alert)
	}
	return result
}

// ListTriggered returns only triggered alerts.
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

// Remove deletes an alert by ID.
func (a *AlertStore) Remove(id string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, ok := a.alerts[id]
	if ok {
		delete(a.alerts, id)
	}
	return ok
}

// checkLoop periodically checks alerts against current prices.
func (a *AlertStore) checkLoop() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		a.mu.Lock()
		for _, alert := range a.alerts {
			if alert.Triggered {
				continue
			}
			price, ok := a.cache.GetPrice(alert.Symbol)
			if !ok {
				continue
			}
			switch alert.Direction {
			case "ABOVE":
				if price >= alert.Price {
					alert.Triggered = true
				}
			case "BELOW":
				if price <= alert.Price {
					alert.Triggered = true
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
