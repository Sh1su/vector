// Package pgconv wandelt zwischen Go-Werten und pgtype-Werten der sqlc-Stores.
package pgconv

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func U(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

func UP(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return U(*id)
}

func ID(u pgtype.UUID) uuid.UUID { return uuid.UUID(u.Bytes) }

func IDP(u pgtype.UUID) *uuid.UUID {
	if !u.Valid {
		return nil
	}
	id := uuid.UUID(u.Bytes)
	return &id
}

func T(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func TP(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	s := t.String
	return &s
}

func I4(i *int) pgtype.Int4 {
	if i == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*i), Valid: true}
}

func I4P(i pgtype.Int4) *int {
	if !i.Valid {
		return nil
	}
	v := int(i.Int32)
	return &v
}

func I8(i *int64) pgtype.Int8 {
	if i == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *i, Valid: true}
}

func I8P(i pgtype.Int8) *int64 {
	if !i.Valid {
		return nil
	}
	v := i.Int64
	return &v
}

func D(t time.Time) pgtype.Date { return pgtype.Date{Time: t, Valid: true} }

func DP(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}
	return D(*t)
}

// DateP liefert ein Datum als *time.Time (00:00 UTC) oder nil.
func DateP(d pgtype.Date) *time.Time {
	if !d.Valid {
		return nil
	}
	t := time.Date(d.Time.Year(), d.Time.Month(), d.Time.Day(), 0, 0, 0, 0, time.UTC)
	return &t
}

func TS(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

func TSP(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return TS(*t)
}

func TSPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

// Numeric liest eine Dezimalzahl in pgtype.Numeric.
func Numeric(dec string) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(dec)
	return n
}

// Float liest pgtype.Numeric als float64 (nur Ausgabe).
func Float(n pgtype.Numeric) float64 {
	f, _ := n.Float64Value()
	return f.Float64
}
