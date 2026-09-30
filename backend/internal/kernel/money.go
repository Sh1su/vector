package kernel

import (
	"math"
	"regexp"
)

var reCurrency = regexp.MustCompile(`^[A-Z]{3}$`)

// ValidCurrency prüft einen ISO-4217-Code (Form, ADR-029).
func ValidCurrency(c string) bool { return reCurrency.MatchString(c) }

// Währungen mit abweichender Anzahl Nachkommastellen (ISO 4217); sonst 2.
var minorDigits = map[string]int{
	"JPY": 0, "KRW": 0, "ISK": 0, "CLP": 0, "VND": 0, "HUF": 2, "PYG": 0, "UGX": 0, "XAF": 0, "XOF": 0, "XPF": 0, "RWF": 0, "KMF": 0, "GNF": 0, "DJF": 0, "VUV": 0,
	"BHD": 3, "KWD": 3, "OMR": 3, "JOD": 3, "TND": 3, "LYD": 3, "IQD": 3,
}

// MinorDigits liefert die Nachkommastellen der kleinsten Einheit.
func MinorDigits(currency string) int {
	if d, ok := minorDigits[currency]; ok {
		return d
	}
	return 2
}

// MajorAmount rechnet einen Betrag in kleinster Einheit in Haupteinheiten um (nur Anzeige).
func MajorAmount(minor int64, currency string) float64 {
	return float64(minor) / math.Pow10(MinorDigits(currency))
}

// Money ist ein Geldbetrag in kleinster Einheit (API-Schema Money).
type Money struct {
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
}
