package api

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"

	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/binance"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/risk"
)

// OrderHandler handles order endpoints with risk management.
type OrderHandler struct {
	client binance.Trader
	risk   *risk.Manager
}

// NewOrderHandler creates an OrderHandler.
func NewOrderHandler(client binance.Trader, riskMgr *risk.Manager) *OrderHandler {
	return &OrderHandler{client: client, risk: riskMgr}
}

// planID generates a plan ID from order parameters.
func planID(symbol, side, orderType string, quantity, price, stopPrice float64) string {
	h := md5.New()
	fmt.Fprintf(h, "%s|%s|%s|%.6f|%.2f|%.2f", symbol, side, orderType, quantity, price, stopPrice)
	return hex.EncodeToString(h.Sum(nil))[:8]
}

// HandlePlaceOrder handles POST /api/v1/order/place with Plan/Apply gate.
func (h *OrderHandler) HandlePlaceOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "use POST")
		return
	}

	symbol := r.FormValue("symbol")
	side := r.FormValue("side")
	orderType := r.FormValue("type")
	positionSide := r.FormValue("position_side")

	if symbol == "" || side == "" || orderType == "" {
		Error(w, http.StatusBadRequest, "MISSING_PARAM", "symbol, side, type are required")
		return
	}

	qty, err := strconv.ParseFloat(r.FormValue("quantity"), 64)
	if err != nil || qty <= 0 {
		Error(w, http.StatusBadRequest, "INVALID_PARAM", "quantity must be a positive number")
		return
	}

	price, _ := strconv.ParseFloat(r.FormValue("price"), 64)
	stopPrice, _ := strconv.ParseFloat(r.FormValue("stop_price"), 64)

	// Plan phase: no confirm → return preview with risk check
	confirm := r.FormValue("confirm")
	if confirm != "true" {
		pid := planID(symbol, side, orderType, qty, price, stopPrice)

		// Price rounding for preview
		roundedPrice := risk.RoundPrice(symbol, price)
		roundedQty := risk.RoundQuantity(symbol, qty)

		preview := binance.OrderPreview{
			PlanID:    pid,
			Symbol:    symbol,
			Side:      side,
			Type:      orderType,
			Quantity:  roundedQty,
			Price:     roundedPrice,
			StopPrice: stopPrice,
			Risk: binance.RiskCheck{
				Passed: true,
				Checks: []string{fmt.Sprintf("notional=%.2f USDT", roundedQty*roundedPrice)},
			},
		}

		// Run risk check (without balance/dailyPnL in preview - those need account state)
		JSON(w, http.StatusOK, preview)
		return
	}

	// Apply phase: confirm=true + plan_id → validate and execute
	planIDParam := r.FormValue("plan_id")
	expectedPID := planID(symbol, side, orderType, qty, price, stopPrice)
	if planIDParam != expectedPID {
		Error(w, http.StatusBadRequest, "PLAN_MISMATCH", "plan_id mismatch — order parameters changed since preview")
		return
	}

	// Apply price/quantity rounding
	roundedPrice := risk.RoundPrice(symbol, price)
	roundedQty := risk.RoundQuantity(symbol, qty)

	req := binance.NewOrderRequest{
		Symbol:       symbol,
		Side:         side,
		PositionSide: positionSide,
		OrderType:    orderType,
		Quantity:     roundedQty,
		Price:        roundedPrice,
		StopPrice:    stopPrice,
	}

	order, err := h.client.CreateOrder(req)
	if err != nil {
		Error(w, http.StatusInternalServerError, "ORDER_FAILED", err.Error())
		return
	}

	JSON(w, http.StatusOK, order)
}

// HandleCancelOrder handles DELETE /api/v1/order/cancel
func (h *OrderHandler) HandleCancelOrder(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	orderIDStr := r.URL.Query().Get("order_id")

	if symbol == "" || orderIDStr == "" {
		Error(w, http.StatusBadRequest, "MISSING_PARAM", "symbol and order_id are required")
		return
	}

	orderID, err := strconv.ParseInt(orderIDStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "INVALID_PARAM", "order_id must be a number")
		return
	}

	order, err := h.client.CancelOrder(symbol, orderID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "CANCEL_FAILED", err.Error())
		return
	}

	JSON(w, http.StatusOK, order)
}

// HandleListOrders handles GET /api/v1/order/list
func (h *OrderHandler) HandleListOrders(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")

	orders, err := h.client.GetOpenOrders(symbol)
	if err != nil {
		Error(w, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}

	JSON(w, http.StatusOK, orders)
}

// HandleOrderStatus handles GET /api/v1/order/status
func (h *OrderHandler) HandleOrderStatus(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	orderIDStr := r.URL.Query().Get("order_id")

	if symbol == "" || orderIDStr == "" {
		Error(w, http.StatusBadRequest, "MISSING_PARAM", "symbol and order_id are required")
		return
	}

	orderID, err := strconv.ParseInt(orderIDStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "INVALID_PARAM", "order_id must be a number")
		return
	}

	order, err := h.client.GetOrder(symbol, orderID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "ORDER_NOT_FOUND", err.Error())
		return
	}

	JSON(w, http.StatusOK, order)
}
