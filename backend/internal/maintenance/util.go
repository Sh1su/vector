package maintenance

import (
	"encoding/json"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"
)

type pgUUID = pgtype.UUID

func boolP(b *bool) pgtype.Bool {
	if b == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *b, Valid: true}
}

func rawJSON(b []byte) json.RawMessage { return json.RawMessage(b) }

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func itoa(i int) string { return strconv.Itoa(i) }
