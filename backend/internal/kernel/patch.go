package kernel

import "encoding/json"

// MergePatch wendet einen JSON Merge Patch (RFC 7396) auf die JSON-Darstellung
// von cur an und liest das Ergebnis in out.
func MergePatch(cur any, patch []byte, out any) error {
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
	mergeInto(doc, p)
	b, _ := json.Marshal(doc)
	return json.Unmarshal(b, out)
}

func mergeInto(dst, patch map[string]any) {
	for k, v := range patch {
		if v == nil {
			delete(dst, k)
			continue
		}
		if pm, ok := v.(map[string]any); ok {
			if dm, ok := dst[k].(map[string]any); ok {
				mergeInto(dm, pm)
				continue
			}
		}
		dst[k] = v
	}
}
