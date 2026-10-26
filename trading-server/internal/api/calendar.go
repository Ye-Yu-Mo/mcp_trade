package api

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

// CalEvent represents an economic calendar event.
type CalEvent struct {
	Date     string `json:"date"`
	Time     string `json:"time"`
	Event    string `json:"event"`
	Currency string `json:"currency"`
	Impact   string `json:"impact"` // High / Medium / Low
}

// getUpcomingEvents returns recurring major US events in the next 30 days.
// Dates are approximate monthly rules (NFP = first Friday, CPI ~12th, FOMC
// ~22nd); this is a planning aid, not an authoritative calendar.
func getUpcomingEvents() []CalEvent {
	now := time.Now().UTC()
	cutoff := now.AddDate(0, 0, 30)

	var events []CalEvent
	for offset := 0; offset <= 1; offset++ {
		first := time.Date(now.Year(), now.Month()+time.Month(offset), 1, 0, 0, 0, 0, time.UTC)
		y, m := first.Year(), int(first.Month())

		add := func(date time.Time, clock, name, impact string) {
			t := time.Date(date.Year(), date.Month(), date.Day(), clockHour(clock), clockMinute(clock), 0, 0, time.UTC)
			if t.Before(now) || t.After(cutoff) {
				return
			}
			events = append(events, CalEvent{
				Date: date.Format("2006-01-02"), Time: clock,
				Event: name, Currency: "USD", Impact: impact,
			})
		}

		nfp := firstFriday(m, y)
		add(nfp, "12:30", "Non-Farm Payrolls (NFP)", "High")
		add(nfp, "12:30", "Unemployment Rate", "High")
		add(fmtDate(m, y, 12), "12:30", "CPI (Inflation) m/m", "High")
		add(fmtDate(m, y, 22), "18:00", "FOMC Interest Rate Decision", "High")
		add(fmtDate(m, y, 15), "12:30", "Retail Sales m/m", "Medium")
		add(fmtDate(m, y, 1), "14:00", "ISM Manufacturing PMI", "Medium")
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Date+events[i].Time < events[j].Date+events[j].Time
	})
	return events
}

func firstFriday(m int, y int) time.Time {
	t := time.Date(y, time.Month(m), 1, 0, 0, 0, 0, time.UTC)
	for t.Weekday() != time.Friday {
		t = t.AddDate(0, 0, 1)
	}
	return t
}

func fmtDate(m int, y int, d int) time.Time {
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
}

func clockHour(clock string) int {
	parts := strings.SplitN(clock, ":", 2)
	if len(parts) != 2 {
		return 0
	}
	h, _ := strconv.Atoi(parts[0])
	return h
}

func clockMinute(clock string) int {
	parts := strings.SplitN(clock, ":", 2)
	if len(parts) != 2 {
		return 0
	}
	m, _ := strconv.Atoi(parts[1])
	return m
}
