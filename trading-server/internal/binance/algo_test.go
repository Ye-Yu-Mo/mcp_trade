package binance

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestAlgoOrderRawMapping(t *testing.T) {
	raw := algoOrderRaw{
		AlgoID:        123,
		ClientAlgoID:  "stop-1",
		OrderType:     "STOP_MARKET",
		Symbol:        "SOLUSDT",
		Side:          "SELL",
		PositionSide:  "BOTH",
		Quantity:      "0.06",
		TriggerPrice:  "101.67",
		ClosePosition: true,
		AlgoStatus:    "NEW",
		BookTime:      123,
	}
	o := raw.toOrder()
	if o.OrderID != 123 || !o.IsAlgo {
		t.Fatalf("unexpected order identity: %+v", o)
	}
	if o.Status != "NEW" || o.StopPrice != 101.67 || o.OrigQty != 0.06 || !o.ClosePosition {
		t.Fatalf("unexpected mapping: %+v", o)
	}
	if o.Type != "STOP_MARKET" || o.Side != "SELL" || o.ClientOrderID != "stop-1" {
		t.Fatalf("unexpected fields: %+v", o)
	}
}

func TestCreateAlgoOrderParams(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"algoId":123,"clientAlgoId":"stop-1","algoType":"CONDITIONAL","orderType":"STOP_MARKET","symbol":"SOLUSDT","side":"SELL","positionSide":"BOTH","triggerPrice":"101.67","closePosition":true,"algoStatus":"NEW","bookTime":123}`)
	}))
	defer srv.Close()

	c := NewClient("key", "secret", srv.URL)
	o, err := c.CreateOrder(NewOrderRequest{
		Symbol: "SOLUSDT", Side: "SELL", OrderType: "STOP_MARKET",
		StopPrice: 101.67, ClosePosition: true,
		ClientOrderID: "stop-1", WorkingType: "MARK_PRICE",
	})
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}
	if got.Get("algoType") != "CONDITIONAL" || got.Get("type") != "STOP_MARKET" {
		t.Fatalf("unexpected algo params: %v", got)
	}
	if got.Get("triggerPrice") != "101.67" || got.Get("closePosition") != "true" {
		t.Fatalf("unexpected trigger/close params: %v", got)
	}
	if got.Get("quantity") != "" || got.Get("reduceOnly") != "" {
		t.Fatalf("closePosition must not send quantity/reduceOnly: %v", got)
	}
	if got.Get("positionSide") != "BOTH" || got.Get("clientAlgoId") != "stop-1" {
		t.Fatalf("unexpected position/client id: %v", got)
	}
	if got.Get("workingType") != "MARK_PRICE" {
		t.Fatalf("unexpected workingType: %v", got)
	}
	if o.OrderID != 123 || !o.IsAlgo || o.Status != "NEW" {
		t.Fatalf("unexpected response order: %+v", o)
	}
}

func TestCancelOrderFallsBackToAlgo(t *testing.T) {
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path)
		if r.URL.Path == "/fapi/v1/order" {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, `{"code":-2011,"msg":"Unknown order sent."}`)
			return
		}
		if r.URL.Path == "/fapi/v1/algoOrder" {
			io.WriteString(w, `{"algoId":123,"clientAlgoId":"stop-1","orderType":"STOP_MARKET","symbol":"SOLUSDT","side":"SELL","positionSide":"BOTH","triggerPrice":"101.67","closePosition":true,"algoStatus":"CANCELED","bookTime":123}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewClient("key", "secret", srv.URL)
	o, err := c.CancelOrder("SOLUSDT", 123)
	if err != nil {
		t.Fatalf("CancelOrder: %v", err)
	}
	if o.OrderID != 123 || !o.IsAlgo || o.Status != "CANCELED" {
		t.Fatalf("unexpected cancel result: %+v", o)
	}
	if len(calls) != 2 {
		t.Fatalf("expected regular then algo cancel, got %v", calls)
	}
}
