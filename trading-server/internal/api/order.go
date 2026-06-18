package api

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/binance"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/risk"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/store"
)

// OrderHandler handles order endpoints with risk management.
type OrderHandler struct {
	client binance.Trader
	risk   *risk.Manager
	store  *store.Store
}

// NewOrderHandler creates an OrderHandler.
func NewOrderHandler(client binance.Trader, riskMgr *risk.Manager, st *store.Store) *OrderHandler {
	return &OrderHandler{client: client, risk: riskMgr, store: st}
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
		Error(w, http.StatusMethodNotAllowed, CodeMethodNotAllowed, "use POST")
		return
	}

	symbol := r.FormValue("symbol")
	side := r.FormValue("side")
	orderType := r.FormValue("type")
	positionSide := r.FormValue("position_side")

	if symbol == "" || side == "" || orderType == "" {
		Error(w, http.StatusBadRequest, CodeMissingParam, "symbol, side, type are required")
		return
	}

	qty, err := strconv.ParseFloat(r.FormValue("quantity"), 64)
	if err != nil || qty <= 0 {
		Error(w, http.StatusBadRequest, CodeInvalidParam, "quantity must be a positive number")
		return
	}

	price, _ := strconv.ParseFloat(r.FormValue("price"), 64)
	stopPrice, _ := strconv.ParseFloat(r.FormValue("stop_price"), 64)

	// Plan phase: real-time risk preview with balance
	confirm := r.FormValue("confirm")
	if confirm != "true" {
		pid := planID(symbol, side, orderType, qty, price, stopPrice)

		// Price rounding for preview
		roundedPrice := risk.RoundPrice(symbol, price)
		roundedQty := risk.RoundQuantity(symbol, qty)

		// Get real balance for risk preview
		balances, _ := h.client.GetBalance()
		var usdtBalance float64
		for _, b := range balances {
			if b.Asset == "USDT" || b.Asset == "USDC" || b.Asset == "BUSD" {
				usdtBalance += b.TotalBalance
			}
		}
		dailyPnL, _ := h.store.GetDailyPnL()

		// For market orders, use current ticker price for risk check
		checkPrice := roundedPrice
		if roundedPrice == 0 {
			if ticker, err := h.client.GetTicker(symbol); err == nil {
				checkPrice = ticker.Price
			}
		}

		riskErr := h.risk.CheckOrder(risk.CheckInput{
			Symbol: symbol, Quantity: roundedQty, Price: checkPrice,
			StopPrice: stopPrice, Balance: usdtBalance, DailyPnL: dailyPnL,
		})
		riskPassed := riskErr == nil
		checks := []string{fmt.Sprintf("balance=%.2f, notional=%.2f, dailyPnL=%.2f", usdtBalance, roundedQty*roundedPrice, dailyPnL)}
		if riskErr != nil {
			checks = append(checks, riskErr.Error())
		}

		preview := binance.OrderPreview{
			PlanID:    pid,
			Symbol:    symbol,
			Side:      side,
			Type:      orderType,
			Quantity:  roundedQty,
			Price:     roundedPrice,
			StopPrice: stopPrice,
			Risk: binance.RiskCheck{
				Passed:   riskPassed,
				Checks:   checks,
			},
		}

		JSON(w, http.StatusOK, preview)
		return
	}

	// Apply phase: confirm=true + plan_id → validate, check risk, execute
	planIDParam := r.FormValue("plan_id")
	expectedPID := planID(symbol, side, orderType, qty, price, stopPrice)
	if planIDParam != expectedPID {
		Error(w, http.StatusBadRequest, CodePlanMismatch, "plan_id mismatch — order parameters changed since preview")
		return
	}

	roundedPrice := risk.RoundPrice(symbol, price)
	roundedQty := risk.RoundQuantity(symbol, qty)

	// Real-time risk check
	balances, _ := h.client.GetBalance()
	var usdtBalance float64
	for _, b := range balances {
		if b.Asset == "USDT" || b.Asset == "USDC" || b.Asset == "BUSD" {
			usdtBalance += b.TotalBalance
		}
	}
	dailyPnL, _ := h.store.GetDailyPnL()
	// For market orders, use current ticker price
	checkPrice := roundedPrice
	if checkPrice == 0 {
		if ticker, err := h.client.GetTicker(symbol); err == nil {
			checkPrice = ticker.Price
		}
	}
	if err := h.risk.CheckOrder(risk.CheckInput{
		Symbol: symbol, Quantity: roundedQty, Price: checkPrice,
		StopPrice: stopPrice, Balance: usdtBalance, DailyPnL: dailyPnL,
	}); err != nil {
		code := CodeRiskRejected
		if len(err.Error()) > 15 {
			code = err.Error()[:15]
		}
		Error(w, http.StatusForbidden, code, err.Error())
		return
	}

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
		Error(w, http.StatusInternalServerError, CodeOrderFailed, err.Error())
		return
	}

	// Auto-record trade
	entryReason := r.FormValue("entry_reason")
	snapshot, _ := json.Marshal(map[string]float64{"price": roundedPrice, "qty": roundedQty})
	h.store.InsertTrade(symbol, side, roundedQty, roundedPrice,
		strconv.FormatInt(order.OrderID, 10), entryReason, string(snapshot))

	JSON(w, http.StatusOK, order)
}

// HandleCancelOrder handles DELETE /api/v1/order/cancel
func (h *OrderHandler) HandleCancelOrder(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	orderIDStr := r.URL.Query().Get("order_id")

	if symbol == "" || orderIDStr == "" {
		Error(w, http.StatusBadRequest, CodeMissingParam, "symbol and order_id are required")
		return
	}

	orderID, err := strconv.ParseInt(orderIDStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, CodeInvalidParam, "order_id must be a number")
		return
	}

	order, err := h.client.CancelOrder(symbol, orderID)
	if err != nil {
		Error(w, http.StatusInternalServerError, CodeCancelFailed, err.Error())
		return
	}

	JSON(w, http.StatusOK, order)
}

// HandleListOrders handles GET /api/v1/order/list
func (h *OrderHandler) HandleListOrders(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")

	orders, err := h.client.GetOpenOrders(symbol)
	if err != nil {
		Error(w, http.StatusInternalServerError, CodeListFailed, err.Error())
		return
	}

	JSON(w, http.StatusOK, orders)
}

// HandleOrderStatus handles GET /api/v1/order/status
func (h *OrderHandler) HandleOrderStatus(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	orderIDStr := r.URL.Query().Get("order_id")

	if symbol == "" || orderIDStr == "" {
		Error(w, http.StatusBadRequest, CodeMissingParam, "symbol and order_id are required")
		return
	}

	orderID, err := strconv.ParseInt(orderIDStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, CodeInvalidParam, "order_id must be a number")
		return
	}

	order, err := h.client.GetOrder(symbol, orderID)
	if err != nil {
		Error(w, http.StatusInternalServerError, CodeOrderNotFound, err.Error())
		return
	}

	JSON(w, http.StatusOK, order)
}

// HandleModifyStop handles POST /api/v1/order/modify_stop — replaces existing stop loss.
func (h *OrderHandler) HandleModifyStop(w http.ResponseWriter, r *http.Request) {
	symbol := r.FormValue("symbol")
	oldOrderIDStr := r.FormValue("old_order_id")
	newStopStr := r.FormValue("new_stop_price")

	if symbol == "" || oldOrderIDStr == "" || newStopStr == "" {
		Error(w, http.StatusBadRequest, CodeMissingParam, "symbol, old_order_id, new_stop_price are required")
		return
	}
	oldOrderID, err := strconv.ParseInt(oldOrderIDStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, CodeInvalidParam, "invalid old_order_id")
		return
	}
	newStop, err := strconv.ParseFloat(newStopStr, 64)
	if err != nil || newStop <= 0 {
		Error(w, http.StatusBadRequest, CodeInvalidParam, "invalid new_stop_price")
		return
	}

	// Cancel old stop
	if _, err := h.client.CancelOrder(symbol, oldOrderID); err != nil {
		Error(w, http.StatusInternalServerError, CodeCancelFailed, fmt.Sprintf("cancel old stop: %v", err))
		return
	}

	// Place new STOP_MARKET with reduce_only
	req := binance.NewOrderRequest{
		Symbol:       symbol,
		Side:         r.FormValue("side"), // SELL for long, BUY for short
		PositionSide: r.FormValue("position_side"),
		OrderType:    "STOP_MARKET",
		Quantity:     qtyFromForm(r, "quantity"),
		StopPrice:    newStop,
		ReduceOnly:   true,
	}
	if req.Side == "" {
		req.Side = "SELL"
	}

	order, err := h.client.CreateOrder(req)
	if err != nil {
		Error(w, http.StatusInternalServerError, CodeOrderFailed, fmt.Sprintf("new stop: %v", err))
		return
	}
	JSON(w, http.StatusOK, map[string]interface{}{
		"cancelled_order": oldOrderID,
		"new_stop":        order,
	})
}

func qtyFromForm(r *http.Request, key string) float64 {
	qty, _ := strconv.ParseFloat(r.FormValue(key), 64)
	return qty
}

// HandleOCOOrder handles POST /api/v1/order/oco — One-Cancels-Other (止盈+止损).
func (h *OrderHandler) HandleOCOOrder(w http.ResponseWriter, r *http.Request) {
	symbol := r.FormValue("symbol")
	side := r.FormValue("side")
	qtyStr := r.FormValue("quantity")
	priceStr := r.FormValue("price")
	stopPriceStr := r.FormValue("stop_price")

	if symbol == "" || side == "" || qtyStr == "" || priceStr == "" || stopPriceStr == "" {
		Error(w, http.StatusBadRequest, CodeMissingParam, "symbol, side, quantity, price, stop_price are required")
		return
	}
	qty, err := strconv.ParseFloat(qtyStr, 64)
	if err != nil || qty <= 0 {
		Error(w, http.StatusBadRequest, CodeInvalidParam, "invalid quantity")
		return
	}
	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil || price <= 0 {
		Error(w, http.StatusBadRequest, CodeInvalidParam, "invalid price")
		return
	}
	stopPrice, err := strconv.ParseFloat(stopPriceStr, 64)
	if err != nil || stopPrice <= 0 {
		Error(w, http.StatusBadRequest, CodeInvalidParam, "invalid stop_price")
		return
	}

	result, err := h.client.CreateOCOOrder(symbol, side, qty, price, stopPrice)
	if err != nil {
		Error(w, http.StatusInternalServerError, CodeOrderFailed, err.Error())
		return
	}
	JSON(w, http.StatusOK, result)
}

// HandleCancelOCO handles DELETE /api/v1/order/oco — cancels an OCO pair.
func (h *OrderHandler) HandleCancelOCO(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	listIDStr := r.URL.Query().Get("order_list_id")
	if symbol == "" || listIDStr == "" {
		Error(w, http.StatusBadRequest, CodeMissingParam, "symbol and order_list_id are required")
		return
	}
	listID, err := strconv.ParseInt(listIDStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, CodeInvalidParam, "invalid order_list_id")
		return
	}
	if err := h.client.CancelOCOOrder(symbol, listID); err != nil {
		Error(w, http.StatusInternalServerError, CodeCancelFailed, err.Error())
		return
	}
	JSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}
