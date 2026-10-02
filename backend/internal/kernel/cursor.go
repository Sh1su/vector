package kernel

import (
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Cursor ist die Position in einer nach (Zeitpunkt, ID) absteigend sortierten Liste (ADR-013).
type Cursor struct {
	At time.Time
	ID uuid.UUID
}

// Encode bildet den Cursor auf eine undurchsichtige Zeichenkette ab.
func (c Cursor) Encode() string {
	b, _ := json.Marshal([]string{c.At.UTC().Format(time.RFC3339Nano), c.ID.String()})
	return base64.RawURLEncoding.EncodeToString(b)
}

// DecodeCursor liest einen Cursor.
func DecodeCursor(s string) (Cursor, bool) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, false
	}
	var p []string
	if json.Unmarshal(b, &p) != nil || len(p) != 2 {
		return Cursor{}, false
	}
	t, err1 := time.Parse(time.RFC3339Nano, p[0])
	id, err2 := uuid.Parse(p[1])
	return Cursor{At: t, ID: id}, err1 == nil && err2 == nil
}
