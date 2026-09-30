# ADR-017 – Object Storage und Dateizugriff

- **Status:** akzeptiert · **Datum:** 2026-09-30

## Kontext
In der Altanwendung liegen Dateien im Web-Pfad, sind für jeden angemeldeten Nutzer abrufbar, werden ohne Typ- und Größenprüfung angenommen und inline ausgeliefert; Pfade werden nicht normalisiert, gelöschte Einträge hinterlassen verwaiste Dateien (Phase 1, R-03, R-04, R-12, D-15). Vectra braucht Dokumente, Fahrzeugbilder und Beweisfotos (Auftrag 6.7, 6.11).

## Optionen
- **A: Storage-Abstraktion** mit lokalem Dateisystem (Default) und S3-kompatiblem Backend (optional), Auslieferung ausschließlich über das Backend.
- B: Direkte, öffentlich lesbare Bucket-URLs – einfach, aber ohne Rechteprüfung.

## Entscheidung
Option A.
- **Schlüssel statt Pfade:** Objekte werden unter einem generierten Schlüssel (`<uuidv7>`) gespeichert, nie unter Nutzer-Dateinamen. Der Originalname steht nur in der Datenbank. Damit entfallen Pfadmanipulationen (R-12).
- **Metadaten** in `documents.files`: `id`, `vehicle_id`, `storage_key`, `original_name`, `media_type` (serverseitig erkannt), `size_bytes`, `sha256`, `uploaded_at` (Serverzeit), `uploaded_by`, `upload_time_origin` ∈ {`upload`, `import`}.
- **Annahme:** Größenlimit (Default 25 MB, konfigurierbar), Allowlist nach erkanntem Inhalt (Magic Bytes), nicht nach Endung: JPEG, PNG, WebP, HEIC, PDF, Textformate. Andere Typen nur, wenn der Betreiber sie freischaltet; sie werden stets als Download ausgeliefert.
- **Auslieferung:** nur über `GET /api/v1/files/{id}/content` nach Rechteprüfung am Fahrzeug (ADR-016); Header `Content-Disposition: attachment` (Ausnahme: Bilder und PDF in der Vorschau über eine eigene, sandboxed Vorschau-Route), `X-Content-Type-Options: nosniff`, `Content-Security-Policy: sandbox`. Optional kurzlebige signierte URLs (≤ 5 min) für S3, nur nach derselben Prüfung erzeugt.
- **Löschen:** Soft-Delete des Metadatensatzes; ein Job entfernt das Objekt nach Ablauf der Aufbewahrungsfrist. Ein Konsistenz-Job meldet verwaiste Objekte und fehlende Dateien.
- **Backups:** Das Storage ist Teil des Backup-Konzepts, liegt nie im Web-Pfad (ADR-030).
- **Virenscan** ist optional (ClamAV-Anbindung als Job), nicht im Basis-Stack.

## Konsequenzen
- (+) Schließt R-03, R-04 und R-12 konzeptionell.
- (−) Jeder Dateiabruf läuft über das Backend; bei großen Dateien wird gestreamt, um das Speicherbudget zu halten.

## Bezug
Auftrag 6.7, 6.11; Phase 1 07, R-03, R-04, R-12, D-15; ADR-016, ADR-018, ADR-030.
