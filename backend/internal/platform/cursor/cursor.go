// Package cursor kodiert Keyset-Cursor für die Paginierung (ADR-013).
package cursor

import (
	"encoding/base64"
	"encoding/json"
)

// Encode verpackt die Schlüsselteile des letzten Elements.
func Encode(parts ...string) string {
	b, _ := json.Marshal(parts)
	return base64.RawURLEncoding.EncodeToString(b)
}

// Decode entpackt einen Cursor mit n Teilen.
func Decode(c string, n int) ([]string, bool) {
	b, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return nil, false
	}
	var p []string
	if json.Unmarshal(b, &p) != nil || len(p) != n {
		return nil, false
	}
	return p, true
}
