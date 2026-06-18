package api

// Standardized error codes for all API responses.
// AI clients parse these codes to decide next actions.
const (
	CodeAuthUnauthorized = "AUTH_UNAUTHORIZED"
	CodeMissingParam     = "MISSING_PARAM"
	CodeInvalidParam     = "INVALID_PARAM"
	CodePlanMismatch     = "PLAN_MISMATCH"
	CodeRiskPositionSize = "RISK_POSITION_SIZE"
	CodeRiskStopLoss     = "RISK_STOP_LOSS"
	CodeRiskDailyLoss    = "RISK_DAILY_LOSS"
	CodeRiskMinNotional  = "RISK_MIN_NOTIONAL"
	CodeRiskRejected     = "RISK_REJECTED"
	CodeOrderFailed      = "ORDER_FAILED"
	CodeOrderNotFound    = "ORDER_NOT_FOUND"
	CodeCancelFailed     = "CANCEL_FAILED"
	CodeListFailed       = "LIST_FAILED"
	CodeQueryFailed      = "QUERY_FAILED"
	CodeInsertFailed     = "INSERT_FAILED"
	CodeBinanceError     = "BINANCE_ERROR"
	CodeNoCache          = "NO_CACHE"
	CodeMethodNotAllowed = "METHOD_NOT_ALLOWED"
)
