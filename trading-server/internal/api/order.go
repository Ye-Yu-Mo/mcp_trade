package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/binance"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/risk"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/store"
)

// OrderHandler handles order endpoints with the Plan/Apply gate, R-based risk
// checks, and automatic protective orders.
type OrderHandler struct {
	client    binance.Trader
	risk      *risk.Manager
	store     *store.Store
	paperMode bool

	mu    sync.Mutex
	plans map[string]planRecord
}

type planRecord struct {
	params  orderParams
	preview orderPreviewResponse
	expires time.Time
}

type orderParams struct {
	Symbol        string
	Side          string
	Type          string
	Quantity      float64
	Price         float64
	StopPrice     float64
	TakeProfit    float64
	RiskPct       float64
	Leverage      float64
	ReduceOnly    bool
	ClosePosition bool
	EntryReason   string
	Setup         string
	Tags          string
	Confirm       bool
	PlanID        string
}

type riskView struct {
	Passed   bool     `json:"passed"`
	Code     string   `json:"code,omitempty"`
	Reason   string   `json:"reason,omitempty"`
	Checks   []string `json:"checks"`
	Warnings []string `json:"warnings"`
}

type orderPreviewResponse struct {
	PlanID        string   `json:"plan_id"`
	Symbol        string   `json:"symbol"`
	Side          string   `json:"side"`
	Type          string   `json:"type"`
	Quantity      float64  `json:"quantity"`
	Price         float64  `json:"price"`
	StopPrice     float64  `json:"stop_price"`
	TakeProfit    float64  `json:"take_profit"`
	Notional      float64  `json:"notional"`
	RiskAmount    float64  `json:"risk_amount"`
	RiskPct       float64  `json:"risk_pct"`
	Margin        float64  `json:"margin"`
	Leverage      float64  `json:"leverage"`
	ReduceOnly    bool     `json:"reduce_only"`
	ClosePosition bool     `json:"close_position"`
	Equity        float64  `json:"equity"`
	Available     float64  `json:"available"`
	OpenRisk      float64  `json:"open_risk"`
	OpenPositions int      `json:"open_positions"`
	DailyPnL      float64  `json:"daily_pnl"`
	Risk          riskView `json:"risk"`
}

type bracketResult struct {
	Entry      *binance.Order       `json:"entry"`
	Stop       *binance.Order       `json:"stop,omitempty"`
	TakeProfit *binance.Order       `json:"take_profit,omitempty"`
	TradeID    int64                `json:"trade_id,omitempty"`
	Warning    string               `json:"warning,omitempty"`
	Risk       orderPreviewResponse `json:"risk"`
}

// NewOrderHandler creates an OrderHandler.
func NewOrderHandler(client binance.Trader, riskMgr *risk.Manager, st *store.Store, paperMode bool) *OrderHandler {
	return &OrderHandler{
		client:    client,
		risk:      riskMgr,
		store:     st,
		paperMode: paperMode,
		plans:     make(map[string]planRecord),
	}
}

// HandlePlaceOrder handles POST /api/v1/order/place with the Plan/Apply gate.
func (h *OrderHandler) HandlePlaceOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Error(w, http.StatusMethodNotAllowed, CodeMethodNotAllowed, "use POST")
		return
	}
	p, err := parseOrderParams(r)
	if err != nil {
		Error(w, http.StatusBadRequest, CodeInvalidParam, err.Error())
		return
	}
	if p.Symbol == "" || p.Side == "" || p.Type == "" {
		Error(w, http.StatusBadRequest, CodeMissingParam, "symbol, side, type are required")
		return
	}

	// Plan phase: evaluate and issue a server-side, single-use plan token.
	if !p.Confirm {
		preview, err := h.evaluate(p)
		if err != nil {
			Error(w, http.StatusInternalServerError, CodeBinanceError, err.Error())
			return
		}
		pid := newPlanID()
		h.mu.Lock()
		h.plans[pid] = planRecord{params: p, preview: preview, expires: time.Now().Add(2 * time.Minute)}
		h.prunePlansLocked()
		h.mu.Unlock()
		preview.PlanID = pid
		JSON(w, http.StatusOK, preview)
		return
	}

	// Apply phase: the stored plan is authoritative; the client cannot change
	// parameters between preview and execution.
	if p.PlanID == "" {
		Error(w, http.StatusBadRequest, CodeMissingParam, "plan_id is required when confirm=true")
		return
	}
	h.mu.Lock()
	rec, ok := h.plans[p.PlanID]
	if ok {
		delete(h.plans, p.PlanID) // single-use
	}
	h.mu.Unlock()
	if !ok {
		Error(w, http.StatusBadRequest, CodePlanExpired, "plan not found or already used; preview again")
		return
	}
	if time.Now().After(rec.expires) {
		Error(w, http.StatusBadRequest, CodePlanExpired, "plan expired; preview again")
		return
	}
	if !samePlan(rec.params, p) {
		Error(w, http.StatusBadRequest, CodePlanMismatch, "plan_id mismatch — order parameters changed since preview")
		return
	}

	preview, err := h.evaluate(rec.params)
	if err != nil {
		Error(w, http.StatusInternalServerError, CodeBinanceError, err.Error())
		return
	}
	// Lock the quantity shown at preview time. The market may have moved, so
	// re-check risk with that exact size; if it no longer passes, the caller
	// must preview again instead of silently executing a different size.
	execParams := rec.params
	if rec.preview.Quantity > 0 && !execParams.ClosePosition {
		execParams.Quantity = rec.preview.Quantity
		execParams.RiskPct = 0
		preview, err = h.evaluate(execParams)
		if err != nil {
			Error(w, http.StatusInternalServerError, CodeBinanceError, err.Error())
			return
		}
	}
	// Carry the effective side/close semantics produced by evaluate (e.g. an
	// omitted side is derived from the position, a conditional order is forced
	// reduce-only). The stored plan params may not have these normalized.
	execParams.Side = preview.Side
	execParams.ReduceOnly = preview.ReduceOnly
	execParams.ClosePosition = preview.ClosePosition

	if !preview.Risk.Passed {
		code := preview.Risk.Code
		if code == "" {
			code = CodeRiskRejected
		}
		Error(w, http.StatusForbidden, code, preview.Risk.Reason)
		return
	}

	result, err := h.execute(execParams, preview)
	if err != nil {
		Error(w, http.StatusInternalServerError, CodeOrderFailed, err.Error())
		return
	}
	JSON(w, http.StatusOK, result)
}

func (h *OrderHandler) evaluate(p orderParams) (orderPreviewResponse, error) {
	acct, err := h.accountState()
	if err != nil {
		return orderPreviewResponse{}, err
	}

	entryPrice := p.Price
	if p.Type == "MARKET" || entryPrice <= 0 {
		t, err := h.client.GetTicker(p.Symbol)
		if err != nil {
			return orderPreviewResponse{}, fmt.Errorf("get price %s: %w", p.Symbol, err)
		}
		entryPrice = t.Price
	}

	intent := risk.OrderIntent{
		Symbol:        p.Symbol,
		Side:          p.Side,
		Type:          p.Type,
		Quantity:      p.Quantity,
		Price:         entryPrice,
		StopPrice:     p.StopPrice,
		TakeProfit:    p.TakeProfit,
		RiskPct:       p.RiskPct,
		ReduceOnly:    p.ReduceOnly,
		ClosePosition: p.ClosePosition,
		Leverage:      p.Leverage,
	}

	reducing := p.ReduceOnly || p.ClosePosition ||
		p.Type == "STOP_MARKET" || p.Type == "TAKE_PROFIT_MARKET"

	var d risk.Decision
	switch {
	case reducing:
		posQty, posSide := h.positionInfo(p.Symbol)
		if posQty <= 0 {
			return orderPreviewResponse{}, fmt.Errorf("no open position for %s", p.Symbol)
		}
		closingSide := "SELL"
		if posSide == "SHORT" {
			closingSide = "BUY"
		}
		if p.Side == "" {
			p.Side = closingSide
			intent.Side = closingSide
		}
		if p.Side != closingSide {
			d = risk.Decision{
				Passed: false,
				Code:   "RISK_REDUCE_SIDE",
				Reason: fmt.Sprintf("%s is %s; the closing side is %s, not %s", p.Symbol, posSide, closingSide, p.Side),
			}
		} else {
			// A protective/close order must never be able to open a position.
			if p.ClosePosition {
				p.ReduceOnly = false
			} else {
				p.ReduceOnly = true
			}
			intent.ReduceOnly = p.ReduceOnly
			intent.ClosePosition = p.ClosePosition
			d = h.risk.CheckReduce(acct, intent, posQty)
		}
	case !h.paperMode && p.Type != "MARKET":
		d = risk.Decision{
			Passed: false,
			Code:   "RISK_LIVE_MARKET_ONLY",
			Reason: "live entries must use MARKET; limit entries cannot be protected server-side yet",
		}
	case h.pendingEntryCount(p.Symbol) > 0:
		d = risk.Decision{
			Passed: false,
			Code:   "RISK_PENDING_ENTRY",
			Reason: fmt.Sprintf("a pending entry already exists on %s; cancel it or wait for it to fill", p.Symbol),
		}
	case h.positionQty(p.Symbol) > 0:
		d = risk.Decision{
			Passed: false,
			Code:   "RISK_POSITION_EXISTS",
			Reason: fmt.Sprintf("a position already exists on %s; close or reduce it before opening a new one", p.Symbol),
		}
	default:
		d = h.risk.CheckEntry(acct, intent)
	}

	checks := []string{fmt.Sprintf(
		"equity=%.2f available=%.2f openRisk=%.2f dailyPnL=%.2f weeklyPnL=%.2f positions=%d",
		acct.Equity, acct.Available, acct.OpenRisk, acct.DailyPnL, acct.WeeklyPnL, acct.OpenPositions,
	)}
	if d.Code != "" {
		checks = append(checks, d.Code+": "+d.Reason)
	}

	return orderPreviewResponse{
		Symbol:        p.Symbol,
		Side:          p.Side,
		Type:          p.Type,
		Quantity:      d.Quantity,
		Price:         d.EntryPrice,
		StopPrice:     d.StopPrice,
		TakeProfit:    d.TakeProfit,
		Notional:      d.Notional,
		RiskAmount:    d.RiskAmount,
		RiskPct:       d.RiskPct,
		Margin:        d.Margin,
		Leverage:      d.Leverage,
		ReduceOnly:    p.ReduceOnly,
		ClosePosition: p.ClosePosition,
		Equity:        acct.Equity,
		Available:     acct.Available,
		OpenRisk:      acct.OpenRisk,
		OpenPositions: acct.OpenPositions,
		DailyPnL:      acct.DailyPnL,
		Risk: riskView{
			Passed:   d.Passed,
			Code:     d.Code,
			Reason:   d.Reason,
			Checks:   checks,
			Warnings: d.Warnings,
		},
	}, nil
}

func (h *OrderHandler) execute(p orderParams, preview orderPreviewResponse) (*bracketResult, error) {
	result := &bracketResult{Risk: preview}

	// Reduce / close path: no new bracket is created.
	if p.ReduceOnly || p.ClosePosition || p.Type == "STOP_MARKET" || p.Type == "TAKE_PROFIT_MARKET" {
		conditional := p.Type == "STOP_MARKET" || p.Type == "TAKE_PROFIT_MARKET"
		req := binance.NewOrderRequest{
			Symbol:        p.Symbol,
			Side:          p.Side,
			OrderType:     p.Type,
			Quantity:      preview.Quantity,
			Price:         p.Price,
			StopPrice:     p.StopPrice,
			ReduceOnly:    p.ReduceOnly,
			ClosePosition: p.ClosePosition,
			ClientOrderID: newClientOrderID("reduce"),
		}
		if !conditional {
			// MARKET/LIMIT do not support closePosition; use a full-size
			// reduce-only order instead.
			req.ClosePosition = false
			req.ReduceOnly = true
			req.Quantity = preview.Quantity
		} else if !p.ClosePosition {
			// Never allow a protective order to open a position.
			req.ReduceOnly = true
		}
		order, err := h.client.CreateOrder(req)
		if err != nil {
			return nil, err
		}
		result.Entry = order
		// Do not cancel existing protection here: the close may not have
		// filled yet, and a conditional stop must stay active.
		return result, nil
	}

	// Live limit entries would sit unprotected until filled; require MARKET.
	if !h.paperMode && p.Type != "MARKET" {
		return nil, fmt.Errorf("live entries must use MARKET; limit entries are not yet protected server-side")
	}

	// The exchange leverage must match the leverage the risk check assumed.
	if !h.paperMode {
		if ls, ok := h.client.(interface {
			SetLeverage(symbol string, leverage int) error
		}); ok {
			if err := ls.SetLeverage(p.Symbol, int(math.Round(preview.Leverage))); err != nil {
				return nil, fmt.Errorf("set leverage: %w", err)
			}
		}
	}

	clientID := newClientOrderID("entry")
	entry, err := h.client.CreateOrder(binance.NewOrderRequest{
		Symbol:        p.Symbol,
		Side:          p.Side,
		OrderType:     p.Type,
		Quantity:      preview.Quantity,
		Price:         p.Price,
		StopPrice:     preview.StopPrice, // paper metadata
		TakeProfit:    preview.TakeProfit,
		Leverage:      preview.Leverage,
		RiskAmount:    preview.RiskAmount,
		EntryReason:   p.EntryReason,
		Setup:         p.Setup,
		Tags:          p.Tags,
		ClientOrderID: clientID,
	})
	if err != nil {
		return nil, err
	}
	result.Entry = entry

	closingSide := "SELL"
	if strings.EqualFold(p.Side, "SELL") {
		closingSide = "BUY"
	}

	// Protective stop first. If it cannot be placed, reverse the entry.
	stop, err := h.client.CreateOrder(binance.NewOrderRequest{
		Symbol:        p.Symbol,
		Side:          closingSide,
		OrderType:     "STOP_MARKET",
		StopPrice:     preview.StopPrice,
		ClosePosition: true,
		WorkingType:   "MARK_PRICE",
		ClientOrderID: newClientOrderID("stop"),
	})
	if err != nil {
		undoErr := h.undoEntry(p, entry, preview.Quantity)
		if undoErr != nil {
			return nil, fmt.Errorf("protective stop failed; MANUAL CHECK REQUIRED: %v (reversal: %v)", err, undoErr)
		}
		return nil, fmt.Errorf("protective stop failed; entry reversed: %w", err)
	}
	result.Stop = stop

	// Take-profit is optional; a failure leaves the stop in place.
	if preview.TakeProfit > 0 {
		tp, err := h.client.CreateOrder(binance.NewOrderRequest{
			Symbol:        p.Symbol,
			Side:          closingSide,
			OrderType:     "TAKE_PROFIT_MARKET",
			StopPrice:     preview.TakeProfit,
			ClosePosition: true,
			WorkingType:   "MARK_PRICE",
			ClientOrderID: newClientOrderID("tp"),
		})
		if err != nil {
			result.Warning = "take-profit order failed; protective stop is active: " + err.Error()
		} else {
			result.TakeProfit = tp
		}
	}

	// Journal the position for live accounts. The paper broker records its own
	// lifecycle when the simulated position opens/closes.
	if !h.paperMode && h.store != nil {
		qty, entryPrice, ok := h.confirmedFill(p, entry, preview.Quantity, preview.Price)
		if !ok {
			result.Warning = strings.TrimSpace(result.Warning +
				" entry fill could not be confirmed; trade was not journaled — verify the position in Binance")
		} else {
			side := "LONG"
			if strings.EqualFold(p.Side, "SELL") {
				side = "SHORT"
			}
			id, err := h.store.OpenTrade(store.OpenTradeParams{
				Symbol:      p.Symbol,
				Side:        side,
				Quantity:    qty,
				EntryPrice:  entryPrice,
				StopPrice:   preview.StopPrice,
				TakeProfit:  preview.TakeProfit,
				Leverage:    preview.Leverage,
				RiskAmount:  preview.RiskAmount,
				OrderID:     strconv.FormatInt(entry.OrderID, 10),
				EntryReason: p.EntryReason,
				Setup:       p.Setup,
				Tags:        p.Tags,
			})
			if err != nil {
				result.Warning = strings.TrimSpace(result.Warning + " journal failed: " + err.Error())
			} else {
				result.TradeID = id
			}
		}
	}

	return result, nil
}

// confirmedFill returns the filled quantity and average price for an entry.
// It falls back to the exchange position when the order response is ACK-only.
func (h *OrderHandler) confirmedFill(p orderParams, entry *binance.Order, fallbackQty, fallbackPrice float64) (float64, float64, bool) {
	if entry != nil {
		if entry.ExecutedQty > 0 && entry.AvgPrice > 0 {
			return entry.ExecutedQty, entry.AvgPrice, true
		}
	}
	positions, err := h.client.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if pos.Symbol == p.Symbol && pos.Quantity > 0 {
				return pos.Quantity, pos.EntryPrice, true
			}
		}
	}
	if entry != nil && entry.Status == "FILLED" {
		qty := entry.ExecutedQty
		if qty <= 0 {
			qty = fallbackQty
		}
		price := entry.AvgPrice
		if price <= 0 {
			price = fallbackPrice
		}
		if qty > 0 {
			return qty, price, true
		}
	}
	return 0, 0, false
}

// undoEntry reverses an entry when the protective stop cannot be placed.
// It verifies the position rather than trusting an ACK-only order status.
func (h *OrderHandler) undoEntry(p orderParams, entry *binance.Order, fallbackQty float64) error {
	if entry == nil {
		return fmt.Errorf("no entry order")
	}
	if entry.Status == "NEW" || entry.Status == "PARTIALLY_FILLED" {
		if _, err := h.client.CancelOrder(p.Symbol, entry.OrderID); err != nil {
			log.Printf("[CRITICAL] cancel entry %d failed: %v", entry.OrderID, err)
		}
	}

	qty, _, posErr := h.positionInfoErr(p.Symbol)
	if posErr == nil && qty <= 1e-9 {
		return nil
	}
	if qty <= 0 {
		qty = entry.ExecutedQty
		if qty <= 0 {
			qty = entry.OrigQty
		}
		if qty <= 0 {
			qty = fallbackQty
		}
	}
	if qty <= 0 {
		return fmt.Errorf("cannot verify whether the entry filled; check the position manually")
	}

	closingSide := "SELL"
	if strings.EqualFold(p.Side, "SELL") {
		closingSide = "BUY"
	}
	if _, err := h.client.CreateOrder(binance.NewOrderRequest{
		Symbol:     p.Symbol,
		Side:       closingSide,
		OrderType:  "MARKET",
		Quantity:   qty,
		ReduceOnly: true,
	}); err != nil {
		log.Printf("[CRITICAL] failed to reverse unprotected entry %s %s: %v", p.Symbol, p.Side, err)
		return err
	}

	if q, _, err := h.positionInfoErr(p.Symbol); err == nil && q > 1e-9 {
		return fmt.Errorf("position still open after reversal (%.8f)", q)
	}
	return nil
}

func (h *OrderHandler) accountState() (risk.AccountState, error) {
	balances, err := h.client.GetBalance()
	if err != nil {
		return risk.AccountState{}, fmt.Errorf("get balance: %w", err)
	}
	positions, err := h.client.GetPositions()
	if err != nil {
		return risk.AccountState{}, fmt.Errorf("get positions: %w", err)
	}

	var wallet, available, unrealized float64
	for _, b := range balances {
		if b.Asset == "USDT" || b.Asset == "USDC" || b.Asset == "BUSD" {
			wallet += b.TotalBalance
			available += b.AvailableBalance
		}
	}
	for _, p := range positions {
		unrealized += p.UnrealizedPnL
	}

	equity := wallet + unrealized
	pendingEntries := h.pendingEntryCount("")
	daily, weekly := 0.0, 0.0
	var openTrades []store.TradeRecord
	if h.store != nil {
		if dailyStart, weeklyStart, err := h.store.EnsureEquityBaseline(equity); err == nil {
			daily = equity - dailyStart
			weekly = equity - weeklyStart
		} else {
			daily, _ = h.store.GetDailyPnL()
			weekly, _ = h.store.GetWeeklyPnL()
		}
		openTrades, _ = h.store.OpenTrades()
	}
	var openRisk float64
	for _, t := range openTrades {
		openRisk += t.RiskAmount
	}
	// Reserved risk for resting entry orders. Their protective stop is a
	// separate order, so be conservative and reserve the full per-trade budget.
	openRisk += float64(pendingEntries) * equity * h.risk.Config().RiskPerTradePct

	return risk.AccountState{
		Equity:        equity,
		Wallet:        wallet,
		Available:     available,
		DailyPnL:      daily,
		WeeklyPnL:     weekly,
		OpenPositions: len(positions) + pendingEntries,
		OpenRisk:      openRisk,
	}, nil
}

func (h *OrderHandler) positionQty(symbol string) float64 {
	qty, _ := h.positionInfo(symbol)
	return qty
}

// pendingEntryCount counts resting orders that could still open a position.
// Protective orders (reduceOnly/closePosition) are excluded.
func (h *OrderHandler) pendingEntryCount(symbol string) int {
	orders, err := h.client.GetOpenOrders(symbol)
	if err != nil {
		return 0
	}
	n := 0
	for _, o := range orders {
		if o.Status != "NEW" && o.Status != "PARTIALLY_FILLED" {
			continue
		}
		if o.ReduceOnly || o.ClosePosition {
			continue
		}
		n++
	}
	return n
}

func (h *OrderHandler) positionInfo(symbol string) (float64, string) {
	qty, side, _ := h.positionInfoErr(symbol)
	return qty, side
}

func (h *OrderHandler) positionInfoErr(symbol string) (float64, string, error) {
	positions, err := h.client.GetPositions()
	if err != nil {
		return 0, "", err
	}
	for _, p := range positions {
		if p.Symbol == symbol {
			return p.Quantity, p.Side, nil
		}
	}
	return 0, "", nil
}

func (h *OrderHandler) cancelProtective(symbol string) {
	orders, err := h.client.GetOpenOrders(symbol)
	if err != nil {
		return
	}
	for _, o := range orders {
		if o.ReduceOnly || o.ClosePosition || strings.Contains(o.Type, "STOP") || strings.Contains(o.Type, "TAKE_PROFIT") {
			h.client.CancelOrder(symbol, o.OrderID)
		}
	}
}

// HandleClose handles POST /api/v1/order/close — market-close a position.
func (h *OrderHandler) HandleClose(w http.ResponseWriter, r *http.Request) {
	symbol := strings.ToUpper(strings.TrimSpace(r.FormValue("symbol")))
	if symbol == "" {
		Error(w, http.StatusBadRequest, CodeMissingParam, "symbol is required")
		return
	}

	positions, err := h.client.GetPositions()
	if err != nil {
		Error(w, http.StatusInternalServerError, CodeBinanceError, err.Error())
		return
	}
	var pos *binance.Position
	for i := range positions {
		if positions[i].Symbol == symbol {
			pos = &positions[i]
			break
		}
	}
	if pos == nil || pos.Quantity <= 0 {
		Error(w, http.StatusNotFound, CodePositionNotFound, "no open position for "+symbol)
		return
	}

	qty := pos.Quantity
	full := true
	if v := strings.TrimSpace(r.FormValue("quantity")); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) || f <= 0 {
			Error(w, http.StatusBadRequest, CodeInvalidParam, "invalid quantity")
			return
		}
		if f > pos.Quantity+1e-9 {
			Error(w, http.StatusBadRequest, CodeInvalidParam,
				fmt.Sprintf("quantity %.8f exceeds position %.8f", f, pos.Quantity))
			return
		}
		qty = f
		full = math.Abs(qty-pos.Quantity) <= 1e-9
	}
	side := "SELL"
	if pos.Side == "SHORT" {
		side = "BUY"
	}

	acct, err := h.accountState()
	if err != nil {
		Error(w, http.StatusInternalServerError, CodeBinanceError, err.Error())
		return
	}
	price, _ := h.client.GetTicker(symbol)
	mark := 0.0
	if price != nil {
		mark = price.Price
	}
	d := h.risk.CheckReduce(acct, risk.OrderIntent{
		Symbol: symbol, Side: side, Type: "MARKET", Quantity: qty, Price: mark, ReduceOnly: true,
	}, pos.Quantity)
	if !d.Passed {
		Error(w, http.StatusForbidden, CodeRiskRejected, d.Reason)
		return
	}

	order, err := h.client.CreateOrder(binance.NewOrderRequest{
		Symbol:        symbol,
		Side:          side,
		OrderType:     "MARKET",
		Quantity:      d.Quantity,
		ReduceOnly:    true,
		ClientOrderID: newClientOrderID("close"),
	})
	if err != nil {
		Error(w, http.StatusInternalServerError, CodeOrderFailed, err.Error())
		return
	}
	// Only remove protection once the close is confirmed filled. A partial
	// close must leave the remaining position's stop/TP in place.
	filled := order.Status == "FILLED" || (order.ExecutedQty > 0 && order.ExecutedQty >= qty-1e-9)
	if full && filled {
		h.cancelProtective(symbol)
	} else if full {
		log.Printf("[WARN] close %s not confirmed filled (status=%s executed=%.8f); protective orders left active",
			symbol, order.Status, order.ExecutedQty)
	}
	JSON(w, http.StatusOK, order)
}

// HandleModifyStop handles POST /api/v1/order/modify_stop. The new stop is
// placed before the old one is cancelled, so the position is never naked.
func (h *OrderHandler) HandleModifyStop(w http.ResponseWriter, r *http.Request) {
	symbol := strings.ToUpper(strings.TrimSpace(r.FormValue("symbol")))
	oldOrderIDStr := r.FormValue("old_order_id")
	newStopStr := r.FormValue("new_stop_price")
	side := strings.ToUpper(strings.TrimSpace(r.FormValue("side")))

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
	if err != nil || newStop <= 0 || math.IsNaN(newStop) || math.IsInf(newStop, 0) {
		Error(w, http.StatusBadRequest, CodeInvalidParam, "invalid new_stop_price")
		return
	}

	posQty, posSide, err := h.positionInfoErr(symbol)
	if err != nil {
		Error(w, http.StatusInternalServerError, CodeBinanceError, err.Error())
		return
	}
	if posQty <= 0 {
		Error(w, http.StatusNotFound, CodePositionNotFound, "no open position for "+symbol)
		return
	}
	closingSide := "SELL"
	if posSide == "SHORT" {
		closingSide = "BUY"
	}
	if side == "" {
		side = closingSide
	} else if side != closingSide {
		Error(w, http.StatusBadRequest, CodeInvalidParam,
			fmt.Sprintf("%s is %s; closing side must be %s", symbol, posSide, closingSide))
		return
	}

	ticker, err := h.client.GetTicker(symbol)
	if err != nil {
		Error(w, http.StatusInternalServerError, CodeBinanceError, err.Error())
		return
	}
	mark := ticker.Price
	if posSide == "LONG" && newStop >= mark {
		Error(w, http.StatusBadRequest, CodeInvalidParam,
			fmt.Sprintf("long stop must be below mark (stop=%.8f mark=%.8f)", newStop, mark))
		return
	}
	if posSide == "SHORT" && newStop <= mark {
		Error(w, http.StatusBadRequest, CodeInvalidParam,
			fmt.Sprintf("short stop must be above mark (stop=%.8f mark=%.8f)", newStop, mark))
		return
	}
	newStop = h.risk.RoundPrice(symbol, newStop)

	newStopOrder, err := h.client.CreateOrder(binance.NewOrderRequest{
		Symbol:        symbol,
		Side:          side,
		OrderType:     "STOP_MARKET",
		StopPrice:     newStop,
		ClosePosition: true,
		WorkingType:   "MARK_PRICE",
		ClientOrderID: newClientOrderID("stop"),
	})
	if err != nil {
		Error(w, http.StatusInternalServerError, CodeOrderFailed, fmt.Sprintf("new stop failed; old stop untouched: %v", err))
		return
	}
	if h.store != nil {
		if err := h.store.UpdateOpenTradeStop(symbol, newStop); err != nil {
			log.Printf("modify_stop: update trade stop %s: %v", symbol, err)
		}
	}

	cancelWarning := ""
	if _, err := h.client.CancelOrder(symbol, oldOrderID); err != nil {
		cancelWarning = fmt.Sprintf("new stop active, but old stop cancel failed: %v", err)
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"cancelled_order": oldOrderID,
		"new_stop":        newStopOrder,
		"warning":         cancelWarning,
	})
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
	if orders == nil {
		orders = []binance.Order{}
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

// HandleOCOOrder is kept for API compatibility. Binance USDⓈ-M futures has
// no OCO product; callers should use order.place with take_profit_price.
func (h *OrderHandler) HandleOCOOrder(w http.ResponseWriter, r *http.Request) {
	Error(w, http.StatusBadRequest, CodeInvalidParam,
		"OCO is not supported on Binance Futures; use order.place with stop_price and take_profit_price")
}

// HandleCancelOCO handles DELETE /api/v1/order/oco.
func (h *OrderHandler) HandleCancelOCO(w http.ResponseWriter, r *http.Request) {
	Error(w, http.StatusBadRequest, CodeInvalidParam, "OCO is not supported on Binance Futures")
}

func parseOrderParams(r *http.Request) (orderParams, error) {
	p := orderParams{
		Symbol:      strings.ToUpper(strings.TrimSpace(r.FormValue("symbol"))),
		Side:        strings.ToUpper(strings.TrimSpace(r.FormValue("side"))),
		Type:        strings.ToUpper(strings.TrimSpace(r.FormValue("type"))),
		EntryReason: r.FormValue("entry_reason"),
		Setup:       r.FormValue("setup"),
		Tags:        r.FormValue("tags"),
		PlanID:      r.FormValue("plan_id"),
		Confirm:     r.FormValue("confirm") == "true",
	}
	var err error
	if p.Quantity, err = optFloat(r, "quantity"); err != nil {
		return p, fmt.Errorf("invalid quantity")
	}
	if p.Price, err = optFloat(r, "price"); err != nil {
		return p, fmt.Errorf("invalid price")
	}
	if p.StopPrice, err = optFloat(r, "stop_price"); err != nil {
		return p, fmt.Errorf("invalid stop_price")
	}
	if p.TakeProfit, err = optFloat(r, "take_profit_price"); err != nil {
		return p, fmt.Errorf("invalid take_profit_price")
	}
	if p.RiskPct, err = optFloat(r, "risk_pct"); err != nil {
		return p, fmt.Errorf("invalid risk_pct")
	}
	if p.Leverage, err = optFloat(r, "leverage"); err != nil {
		return p, fmt.Errorf("invalid leverage")
	}
	p.ReduceOnly = truthy(r.FormValue("reduce_only"))
	p.ClosePosition = truthy(r.FormValue("close_position"))
	return p, nil
}

func optFloat(r *http.Request, key string) (float64, error) {
	v := strings.TrimSpace(r.FormValue(key))
	if v == "" {
		return 0, nil
	}
	return strconv.ParseFloat(v, 64)
}

func truthy(v string) bool {
	return v == "true" || v == "1" || v == "yes"
}

func samePlan(a, b orderParams) bool {
	const eps = 1e-9
	return a.Symbol == b.Symbol &&
		a.Side == b.Side &&
		a.Type == b.Type &&
		a.ReduceOnly == b.ReduceOnly &&
		a.ClosePosition == b.ClosePosition &&
		math.Abs(a.Quantity-b.Quantity) < eps &&
		math.Abs(a.Price-b.Price) < eps &&
		math.Abs(a.StopPrice-b.StopPrice) < eps &&
		math.Abs(a.TakeProfit-b.TakeProfit) < eps &&
		math.Abs(a.RiskPct-b.RiskPct) < eps &&
		math.Abs(a.Leverage-b.Leverage) < eps
}

func (h *OrderHandler) prunePlansLocked() {
	now := time.Now()
	for id, rec := range h.plans {
		if now.After(rec.expires) {
			delete(h.plans, id)
		}
	}
}

func newPlanID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("plan-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func newClientOrderID(prefix string) string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return fmt.Sprintf("%s-%d-%s", prefix, time.Now().UnixMilli(), hex.EncodeToString(b))
}
