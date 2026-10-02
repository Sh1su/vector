// Package mergepatch wendet JSON Merge Patches (RFC 7396) auf Eingabestrukturen an.
package mergepatch

import "encoding/json"

// Apply serialisiert cur, wendet patch an und liest das Ergebnis in out.
func Apply(cur any, patch []byte, out any) error {
	base, err := json.Marshal(cur)
	if err != nil {
		return err
	}
	var doc map[string]any
	if err := json.Unmarshal(base, &doc); err != nil {
		return err
	}
	var p map[string]any
	if err := json.Unmarshal(patch, &p); err != nil {
		return err
	}
	merge(doc, p)
	b, _ := json.Marshal(doc)
	return json.Unmarshal(b, out)
}

// Has meldet, ob der Patch ein Feld enthält (auch mit null).
func Has(patch []byte, field string) bool {
	var p map[string]json.RawMessage
	if json.Unmarshal(patch, &p) != nil {
		return false
	}
	_, ok := p[field]
	return ok
}

func merge(dst, patch map[string]any) {
	for k, v := range patch {
		if v == nil {
			delete(dst, k)
			continue
		}
		if pm, ok := v.(map[string]any); ok {
			if dm, ok := dst[k].(map[string]any); ok {
				merge(dm, pm)
				continue
			}
		}
		dst[k] = v
	}
}
