package api

import (
	"net/http"

	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/binance"
)

// AccountHandler handles account-related endpoints.
type AccountHandler struct {
	client binance.Trader
}

// NewAccountHandler creates an AccountHandler.
func NewAccountHandler(client binance.Trader) *AccountHandler {
	return &AccountHandler{client: client}
}

// HandleBalance handles GET /api/v1/account/balance
func (h *AccountHandler) HandleBalance(w http.ResponseWriter, r *http.Request) {
	balances, err := h.client.GetBalance()
	if err != nil {
		Error(w, http.StatusInternalServerError, "BINANCE_ERROR", err.Error())
		return
	}

	JSON(w, http.StatusOK, balances)
}

// HandlePositions handles GET /api/v1/account/positions
func (h *AccountHandler) HandlePositions(w http.ResponseWriter, r *http.Request) {
	positions, err := h.client.GetPositions()
	if err != nil {
		Error(w, http.StatusInternalServerError, "BINANCE_ERROR", err.Error())
		return
	}

	JSON(w, http.StatusOK, positions)
}
