package api

import "time"

// CalEvent represents an economic calendar event.
type CalEvent struct {
	Date     string `json:"date"`
	Time     string `json:"time"`
	Event    string `json:"event"`
	Currency string `json:"currency"`
	Impact   string `json:"impact"` // High / Medium / Low
}

// getUpcomingEvents returns major events for the next 30 days.
// This is a static list of recurring monthly events. For real-time data, integrate
// with an economic calendar API (e.g., ForexFactory, TradingEconomics).
func getUpcomingEvents() []CalEvent {
	now := time.Now().UTC()
	year := now.Year()
	month := now.Month()
	nextMonth := month + 1
	if nextMonth > 12 {
		nextMonth = 1
		year++
	}

	// Non-Farm Payrolls: first Friday of each month
	nfp := firstFriday(int(nextMonth), year)

	return []CalEvent{
		{Date: nfp.Format("2006-01-02"), Time: "12:30", Event: "Non-Farm Payrolls (NFP)", Currency: "USD", Impact: "High"},
		{Date: nfp.Format("2006-01-02"), Time: "12:30", Event: "Unemployment Rate", Currency: "USD", Impact: "High"},
		{Date: cpiDate(int(nextMonth), year).Format("2006-01-02"), Time: "12:30", Event: "CPI (Inflation) m/m", Currency: "USD", Impact: "High"},
		{Date: fomcDate(int(nextMonth), year).Format("2006-01-02"), Time: "18:00", Event: "FOMC Interest Rate Decision", Currency: "USD", Impact: "High"},
		{Date: fmtDate(int(nextMonth), year, 15).Format("2006-01-02"), Time: "12:30", Event: "Retail Sales m/m", Currency: "USD", Impact: "Medium"},
		{Date: fmtDate(int(nextMonth), year, 1).Format("2006-01-02"), Time: "14:00", Event: "ISM Manufacturing PMI", Currency: "USD", Impact: "Medium"},
	}
}

func firstFriday(m int, y int) time.Time {
	t := time.Date(y, time.Month(m), 1, 0, 0, 0, 0, time.UTC)
	for t.Weekday() != time.Friday {
		t = t.AddDate(0, 0, 1)
	}
	return t
}

func cpiDate(m int, y int) time.Time {
	return fmtDate(m, y, 12)
}

func fomcDate(m int, y int) time.Time {
	// FOMC typically meets around the 20th-25th, approximating
	return fmtDate(m, y, 22)
}

func fmtDate(m int, y int, d int) time.Time {
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
}
