package server

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/sh1su/vector/backend/internal/identity"
)

// upload sendet eine Datei als multipart/form-data (Felder vor der Datei).
func (c *client) upload(path, name string, data []byte, fields ...string) resp {
	c.e.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for i := 0; i+1 < len(fields); i += 2 {
		_ = mw.WriteField(fields[i], fields[i+1])
	}
	fw, _ := mw.CreateFormFile("file", name)
	_, _ = fw.Write(data)
	_ = mw.Close()
	req, _ := http.NewRequest("POST", c.e.srv.URL+"/api/v1"+path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-CSRF-Token", c.csrf)
	res, err := c.http.Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	out := resp{status: res.StatusCode, header: res.Header, raw: raw}
	_ = json.Unmarshal(raw, &out.body)
	return out
}

// jpegWithGPS erzeugt ein JPEG mit EXIF-Segment (Ausrichtung 6, GPS-Zeiger).
func jpegWithGPS(t *testing.T, w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 120, 255})
		}
	}
	var plain bytes.Buffer
	if err := jpeg.Encode(&plain, img, nil); err != nil {
		t.Fatal(err)
	}
	// TIFF (big endian): IFD0 mit 2 Einträgen: Orientation=6, GPSInfo-Zeiger
	tiff := []byte("MM\x00\x2a\x00\x00\x00\x08")
	ifd := make([]byte, 2+2*12+4)
	binary.BigEndian.PutUint16(ifd[0:], 2)
	binary.BigEndian.PutUint16(ifd[2:], 0x0112)
	binary.BigEndian.PutUint16(ifd[4:], 3)
	binary.BigEndian.PutUint32(ifd[6:], 1)
	binary.BigEndian.PutUint16(ifd[10:], 6)
	binary.BigEndian.PutUint16(ifd[14:], 0x8825)
	binary.BigEndian.PutUint16(ifd[16:], 4)
	binary.BigEndian.PutUint32(ifd[18:], 1)
	binary.BigEndian.PutUint32(ifd[22:], 0)
	seg := append([]byte("Exif\x00\x00"), append(tiff, ifd...)...)
	app1 := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(app1[2:], uint16(len(seg)+2))
	out := append([]byte{0xFF, 0xD8}, app1...)
	out = append(out, seg...)
	return append(out, plain.Bytes()[2:]...)
}

func TestDocumentsFilesAndImages(t *testing.T) {
	e := newEnv(t)
	c := e.adminClient()
	vid := c.do("POST", "/vehicles", vehiclePayload("Golf")).body["id"].(string)
	base := "/vehicles/" + vid
	photo := jpegWithGPS(t, 60, 40)

	r := c.upload(base+"/files", "auto.jpg", photo, "capture_source", "camera")
	expect(t, r, 201, "upload jpeg")
	fid := r.body["id"].(string)
	if r.body["media_type"] != "image/jpeg" || r.body["has_location"] != true || len(r.body["derivatives"].([]any)) != 2 {
		t.Fatalf("file meta: %s", r.raw)
	}
	// D-5: gleiche Datei erneut → Hinweis, nichts gespeichert
	r = c.upload(base+"/files", "kopie.jpg", photo)
	expect(t, r, 200, "D-5 duplicate")
	if r.body["duplicate_of"] != fid {
		t.Fatalf("D-5: %s", r.raw)
	}
	// D-7: Endung .pdf, Inhalt HTML → abgelehnt
	expect(t, c.upload(base+"/files", "rechnung.pdf", []byte("<!DOCTYPE html><html><script>alert(1)</script></html>")), 422, "D-7")
	// Größenlimit (Testkonfiguration 2 MB)
	expect(t, c.upload(base+"/files", "gross.pdf", append([]byte("%PDF-1.4\n"), make([]byte, 3<<20)...)), 422, "too large")

	// Vorschau: JPEG ohne EXIF, gedreht (Ausrichtung 6 → 40×60)
	res, err := c.http.Get(e.srv.URL + "/api/v1" + base + "/files/" + fid + "/preview?size=thumbnail")
	if err != nil {
		t.Fatal(err)
	}
	prev, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || res.Header.Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(res.Header.Get("Content-Security-Policy"), "sandbox") {
		t.Fatalf("preview headers: %d %v", res.StatusCode, res.Header)
	}
	if bytes.Contains(prev, []byte("Exif")) {
		t.Fatal("preview contains EXIF")
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(prev))
	if err != nil || cfg.Width != 40 || cfg.Height != 60 {
		t.Fatalf("preview orientation: %+v %v", cfg, err)
	}
	res, _ = c.http.Get(e.srv.URL + "/api/v1" + base + "/files/" + fid + "/content")
	res.Body.Close()
	if !strings.HasPrefix(res.Header.Get("Content-Disposition"), "attachment") {
		t.Fatalf("content disposition: %v", res.Header)
	}

	// Fahrzeugbild
	img := c.do("POST", base+"/images", map[string]any{"file_id": fid})
	expect(t, img, 201, "vehicle image")
	if img.body["primary"] != true {
		t.Fatalf("first image primary: %s", img.raw)
	}
	expect(t, c.do("DELETE", base+"/files/"+fid, nil), 409, "file in use")

	// D-2: Dokument mit 3 Seiten in Reihenfolge
	var pages []string
	for i := 0; i < 3; i++ {
		p := c.upload(base+"/files", "seite.pdf", []byte("%PDF-1.4\nSeite "+string(rune('1'+i))))
		expect(t, p, 201, "page")
		pages = append(pages, p.body["id"].(string))
	}
	doc := c.do("POST", base+"/documents", map[string]any{"doc_type": "invoice", "title": "Rechnung Inspektion", "issuer": "Autohaus", "file_ids": pages})
	expect(t, doc, 201, "D-2")
	if doc.body["nature"] != "record" || doc.body["file_ids"].([]any)[2] != pages[2] {
		t.Fatalf("D-2: %s", doc.raw)
	}
	expect(t, c.do("POST", base+"/documents", map[string]any{"doc_type": "invoice", "title": "leer", "file_ids": []string{}}), 422, "I-DO-4")
	list := c.do("GET", base+"/documents", nil)
	if len(list.body["items"].([]any)) != 1 {
		t.Fatalf("documents: %s", list.raw)
	}

	// Anhang an Serviceeintrag; D-4: fremdes Fahrzeug
	se := serviceEntry("2026-03-10T12:00:00+01:00", 45000, item("labor", 1000))
	sid := c.do("POST", base+"/service-entries", se).body["id"].(string)
	att := c.do("POST", base+"/attachments", map[string]any{"document_id": doc.body["id"], "target_type": "service_entry", "target_id": sid, "role": "invoice"})
	expect(t, att, 201, "attach")
	if n := len(c.do("GET", base+"/attachments?target_type=service_entry&target_id="+sid, nil).body["items"].([]any)); n != 1 {
		t.Fatalf("attachments: %d", n)
	}
	other := c.do("POST", "/vehicles", vehiclePayload("Anderes")).body["id"].(string)
	expect(t, c.do("POST", "/vehicles/"+other+"/attachments", map[string]any{"file_id": fid, "target_type": "service_entry", "target_id": sid, "role": "photo"}), 422, "D-4")

	// D-3: Leser bekommt kein Original, aber die Vorschau
	b, bID := e.extraUser("leser@example.org")
	if err := identity.AddMembership(context.Background(), e.pool, uuid.MustParse(vid), bID, identity.RoleViewer); err != nil {
		t.Fatal(err)
	}
	expect(t, b.do("GET", base+"/files/"+fid+"/original", nil), 403, "D-3 original")
	res, _ = b.http.Get(e.srv.URL + "/api/v1" + base + "/files/" + fid + "/preview")
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("D-3 preview: %d", res.StatusCode)
	}
	expect(t, b.upload(base+"/files", "x.pdf", []byte("%PDF-1.4\nx")), 403, "viewer upload")
}
