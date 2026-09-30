package odometer

import (
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

func encodeCursor(t time.Time, id uuid.UUID) string {
	b, _ := json.Marshal([]string{t.UTC().Format(time.RFC3339Nano), id.String()})
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(c string) (time.Time, uuid.UUID, bool) {
	b, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return time.Time{}, uuid.Nil, false
	}
	var p []string
	if json.Unmarshal(b, &p) != nil || len(p) != 2 {
		return time.Time{}, uuid.Nil, false
	}
	t, err1 := time.Parse(time.RFC3339Nano, p[0])
	id, err2 := uuid.Parse(p[1])
	return t, id, err1 == nil && err2 == nil
}
