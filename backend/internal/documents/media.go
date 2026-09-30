package documents

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/jpeg"
	_ "image/png" // Decoder registrieren
	"net/http"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // Decoder registrieren
)

// Erlaubte Inhalte nach Magic Bytes, nicht nach Endung (ADR-017).
var allowed = map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true, "image/heic": true, "application/pdf": true, "text/plain": true}

// DetectMediaType erkennt den Typ am Inhalt; "" = nicht erlaubt (D-7).
func DetectMediaType(head []byte) string {
	if len(head) >= 12 && string(head[4:8]) == "ftyp" {
		switch string(head[8:12]) {
		case "heic", "heix", "heim", "heis", "mif1", "msf1":
			return "image/heic"
		}
	}
	mt := http.DetectContentType(head)
	if i := bytes.IndexByte([]byte(mt), ';'); i >= 0 {
		mt = mt[:i]
	}
	if !allowed[mt] {
		return ""
	}
	return mt
}

// exifInfo liest aus einem JPEG die Ausrichtung (Tag 0x0112) und ob GPS-Daten vorhanden sind.
func exifInfo(data []byte) (orientation int, hasGPS bool) {
	orientation = 1
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return
	}
	for i := 2; i+4 <= len(data); {
		if data[i] != 0xFF {
			return
		}
		marker := data[i+1]
		size := int(binary.BigEndian.Uint16(data[i+2 : i+4]))
		if marker == 0xDA || size < 2 || i+2+size > len(data) {
			return
		}
		seg := data[i+4 : i+2+size]
		if marker == 0xE1 && len(seg) > 14 && string(seg[:6]) == "Exif\x00\x00" {
			tiff := seg[6:]
			var bo binary.ByteOrder = binary.BigEndian
			if string(tiff[:2]) == "II" {
				bo = binary.LittleEndian
			}
			ifd := int(bo.Uint32(tiff[4:8]))
			if ifd+2 > len(tiff) {
				return
			}
			n := int(bo.Uint16(tiff[ifd : ifd+2]))
			for k := 0; k < n; k++ {
				e := ifd + 2 + k*12
				if e+12 > len(tiff) {
					break
				}
				switch bo.Uint16(tiff[e : e+2]) {
				case 0x0112:
					orientation = int(bo.Uint16(tiff[e+8 : e+10]))
				case 0x8825:
					hasGPS = true
				}
			}
			return
		}
		i += 2 + size
	}
	return
}

// orient dreht/spiegelt das Bild gemäß EXIF-Ausrichtung, damit die bereinigte
// Fassung ohne EXIF richtig herum steht.
func orient(src image.Image, o int) image.Image {
	if o <= 1 || o > 8 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	swap := o >= 5
	dw, dh := w, h
	if swap {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var nx, ny int
			switch o {
			case 2:
				nx, ny = w-1-x, y
			case 3:
				nx, ny = w-1-x, h-1-y
			case 4:
				nx, ny = x, h-1-y
			case 5:
				nx, ny = y, x
			case 6:
				nx, ny = h-1-y, x
			case 7:
				nx, ny = h-1-y, w-1-x
			case 8:
				nx, ny = y, w-1-x
			}
			dst.Set(nx, ny, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

const maxPixels = 60_000_000 // Schutz vor Dekompressionsbomben

// Derivative ist eine bereinigte Ableitung (ohne EXIF, ADR-018).
type Derivative struct {
	Kind string // thumbnail | preview
	Data []byte
}

// MakeDerivatives erzeugt Vorschaubild (400 px) und Vorschau (1600 px) als JPEG.
// Für nicht dekodierbare Formate (HEIC, PDF) gibt es keine Ableitungen.
func MakeDerivatives(data []byte, mediaType string) ([]Derivative, bool) {
	var hasGPS bool
	orientation := 1
	if mediaType == "image/jpeg" {
		orientation, hasGPS = exifInfo(data)
	}
	if mediaType != "image/jpeg" && mediaType != "image/png" && mediaType != "image/webp" {
		return nil, hasGPS
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width*cfg.Height > maxPixels {
		return nil, hasGPS
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, hasGPS
	}
	var out []Derivative
	b := img.Bounds()
	for _, d := range []struct {
		kind string
		max  int
	}{{"thumbnail", 400}, {"preview", 1600}} {
		// zuerst verkleinern, dann drehen: spart Speicher bei großen Fotos
		w, h := b.Dx(), b.Dy()
		if w > d.max || h > d.max {
			if w >= h {
				w, h = d.max, h*d.max/w
			} else {
				w, h = w*d.max/h, d.max
			}
		}
		dst := image.NewRGBA(image.Rect(0, 0, max(w, 1), max(h, 1)))
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, orient(dst, orientation), &jpeg.Options{Quality: 82}); err != nil {
			return nil, hasGPS
		}
		out = append(out, Derivative{Kind: d.kind, Data: buf.Bytes()})
	}
	return out, hasGPS
}
