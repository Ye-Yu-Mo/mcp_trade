package ws

import (
	"testing"
	"time"
)

func TestAlertStore_AddAndList(t *testing.T) {
	cache := NewMarketCache()
	cache.SetPrice("BTCUSDT", 64000)
	store := NewAlertStore(cache)

	id := store.Add("BTCUSDT", 65000, "ABOVE", "test alert")
	if id == "" {
		t.Fatal("expected non-empty alert id")
	}

	alerts := store.List()
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].Triggered {
		t.Error("alert should not be triggered yet")
	}
}

func TestAlertStore_Trigger(t *testing.T) {
	cache := NewMarketCache()
	cache.SetPrice("BTCUSDT", 64000)
	store := NewAlertStore(cache)

	store.Add("BTCUSDT", 63500, "BELOW", "should trigger when price drops")

	// Price below threshold
	cache.SetPrice("BTCUSDT", 63000)
	time.Sleep(4 * time.Second) // checkLoop runs every 3s

	alerts := store.List()
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert")
	}
	if !alerts[0].Triggered {
		t.Error("alert should be triggered (price 63000 <= 63500)")
	}
}

func TestAlertStore_Remove(t *testing.T) {
	cache := NewMarketCache()
	store := NewAlertStore(cache)
	id := store.Add("ETHUSDT", 2000, "ABOVE", "remove me")

	if !store.Remove(id) {
		t.Error("remove should return true")
	}
	if store.Remove(id) {
		t.Error("remove of non-existent should return false")
	}
}
