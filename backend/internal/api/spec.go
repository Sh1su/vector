package api

import _ "embed"

// SpecJSON ist die Spezifikation als JSON (ausgeliefert unter /api/v1/openapi.json).
//
//go:embed openapi.gen.json
var SpecJSON []byte
