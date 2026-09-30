package kernel

import (
	"fmt"
	"time"
)

// Kalenderdaten werden als time.Time um 00:00 UTC dargestellt (ADR-008).

// ParseDate liest „JJJJ-MM-TT“.
func ParseDate(s string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q", s)
	}
	return t, nil
}

// FormatDate schreibt „JJJJ-MM-TT“.
func FormatDate(t time.Time) string { return t.Format("2006-01-02") }

// Date normalisiert einen Zeitpunkt auf sein Kalenderdatum (UTC-Komponenten).
func Date(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// Today ist das heutige Datum in einer Zeitzone.
func Today(now time.Time, tz string) time.Time { return LocalDate(now, tz) }

// AddMonths addiert Monate; fehlt der Zieltag im Zielmonat, gilt der Monatsletzte
// (31.01. + 1 Monat = 28./29.02., MA-02).
func AddMonths(d time.Time, months int) time.Time {
	y, m := d.Year(), int(d.Month())-1+months
	y += m / 12
	m %= 12
	if m < 0 {
		m += 12
		y--
	}
	last := time.Date(y, time.Month(m+2), 0, 0, 0, 0, 0, time.UTC).Day()
	day := d.Day()
	if day > last {
		day = last
	}
	return time.Date(y, time.Month(m+1), day, 0, 0, 0, 0, time.UTC)
}

// DaysBetween zählt Kalendertage von a nach b (b − a).
func DaysBetween(a, b time.Time) int {
	return int(Date(b).Sub(Date(a)).Hours() / 24)
}

// StartOfDay liefert 00:00 des Datums in der Zeitzone als Zeitpunkt.
func StartOfDay(d time.Time, tz string) time.Time {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)
}

// Noon liefert 12:00 des Datums in der Zeitzone (Ordnungszeitpunkt bei date_only).
func Noon(d time.Time, tz string) time.Time { return StartOfDay(d, tz).Add(12 * time.Hour) }
