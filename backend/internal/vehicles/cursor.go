package vehicles

import "encoding/base64"

func base64URL(b []byte) string            { return base64.RawURLEncoding.EncodeToString(b) }
func unbase64URL(s string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(s) }
