package kernel

import (
	"math/big"
	"strconv"
)

// MinorDigits liefert die Nachkommastellen einer Währung (ISO 4217, ADR-029).
func MinorDigits(currency string) int {
	switch currency {
	case "JPY", "KRW", "ISK", "CLP", "VND", "XAF", "XOF", "PYG", "UGX":
		return 0
	case "BHD", "KWD", "OMR", "JOD", "TND", "IQD", "LYD":
		return 3
	}
	return 2
}

// PriceTotal berechnet den Gesamtbetrag aus Preis pro Einheit und Menge (FU-07):
// round_half_up(preis × menge × 10^Stellen). Die Menge wird exakt aus ihrem
// kanonischen Wert in die Preiseinheit umgerechnet, bei gleicher Einheit aus der
// Originaleingabe.
func PriceTotal(price float64, currency, perUnit string, qty Quantity) (int64, error) {
	p, ok := new(big.Rat).SetString(strconv.FormatFloat(price, 'f', -1, 64))
	if !ok {
		return 0, ErrUnknownUnit
	}
	var amount *big.Rat
	if perUnit == qty.InputUnit {
		amount, _ = new(big.Rat).SetString(qty.InputValue)
	} else {
		u, ok := inputUnits[perUnit]
		if !ok || u.canonical != qty.CanonicalUnit {
			return 0, ErrUnknownUnit
		}
		amount = new(big.Rat).Quo(new(big.Rat).SetInt64(qty.Canonical), u.factor)
	}
	if amount == nil {
		return 0, ErrUnknownUnit
	}
	r := new(big.Rat).Mul(p, amount)
	r.Mul(r, new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(MinorDigits(currency))), nil)))
	return RoundHalfUp(r), nil
}

// RoundHalfUp rundet kaufmännisch (bei .5 vom Nullpunkt weg).
func RoundHalfUp(r *big.Rat) int64 {
	num, den := new(big.Int).Set(r.Num()), r.Denom()
	q, m := new(big.Int).QuoRem(num, den, new(big.Int))
	twice := new(big.Int).Mul(new(big.Int).Abs(m), big.NewInt(2))
	if twice.Cmp(den) >= 0 {
		if num.Sign() < 0 {
			q.Sub(q, big.NewInt(1))
		} else {
			q.Add(q, big.NewInt(1))
		}
	}
	return q.Int64()
}
