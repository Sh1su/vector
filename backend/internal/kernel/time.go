package kernel

import (
	"fmt"
	"time"
)

// Zeitgenauigkeit fachlicher Ereignisse (ADR-008).
const (
	PrecisionExact    = "exact"
	PrecisionDateOnly = "date_only"
)

// EventTime ist der Zeitpunkt eines fachlichen Ereignisses.
type EventTime struct {
	At        time.Time
	TimeZone  string
	Precision string
}

// NormalizeEventTime prüft die Zeitzone und setzt bei date_only 12:00 lokal
// als Ordnungszeitpunkt (ADR-008).
func NormalizeEventTime(at time.Time, tz, precision string) (EventTime, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil || tz == "" {
		return EventTime{}, fmt.Errorf("invalid time zone %q", tz)
	}
	switch precision {
	case "", PrecisionExact:
		precision = PrecisionExact
	case PrecisionDateOnly:
		l := at.In(loc)
		at = time.Date(l.Year(), l.Month(), l.Day(), 12, 0, 0, 0, loc)
	default:
		return EventTime{}, fmt.Errorf("invalid time precision %q", precision)
	}
	return EventTime{At: at.UTC(), TimeZone: tz, Precision: precision}, nil
}

// LocalDate liefert das Kalenderdatum eines Zeitpunkts in einer Zeitzone.
func LocalDate(at time.Time, tz string) time.Time {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	l := at.In(loc)
	return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, time.UTC)
}
