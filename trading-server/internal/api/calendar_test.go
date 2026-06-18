package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleCalendar(t *testing.T) {
	m := &MarketHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/market/calendar", nil)
	rec := httptest.NewRecorder()
	m.HandleCalendar(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp struct {
		Data []struct {
			Date  string `json:"date"`
			Event string `json:"event"`
			Impact string `json:"impact"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	events := resp.Data
	if len(events) < 4 {
		t.Errorf("expected at least 4 events, got %d", len(events))
	}

	hasNFP := false
	hasFOMC := false
	for _, e := range events {
		if e.Event == "Non-Farm Payrolls (NFP)" {
			hasNFP = true
		}
		if e.Event == "FOMC Interest Rate Decision" {
			hasFOMC = true
		}
		if e.Impact != "High" && e.Impact != "Medium" && e.Impact != "Low" {
			t.Errorf("invalid impact: %q", e.Impact)
		}
	}
	if !hasNFP {
		t.Error("missing NFP event")
	}
	if !hasFOMC {
		t.Error("missing FOMC event")
	}
}

func TestFirstFriday(t *testing.T) {
	// January 2026: 1st is Thursday, first Friday = Jan 2
	f := firstFriday(1, 2026)
	if f.Day() != 2 {
		t.Errorf("first Friday of Jan 2026 = %d, want 2", f.Day())
	}
	if f.Month() != 1 {
		t.Errorf("month = %d, want 1", f.Month())
	}

	// June 2026: 1st is Monday, first Friday = June 5
	f = firstFriday(6, 2026)
	if f.Day() != 5 {
		t.Errorf("first Friday of Jun 2026 = %d, want 5", f.Day())
	}
}
