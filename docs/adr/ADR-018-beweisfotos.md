# ADR-018 – Beweisfotos

- **Status:** akzeptiert · **Datum:** 2026-09-30

## Kontext
Kilometerstände, Tankquittungen, Ölstände und Schäden sollen per Foto belegt werden können (Auftrag 6.7). Ein Beleg ist nur etwas wert, wenn das Original unverändert und seine Herkunft nachvollziehbar ist. Die Altanwendung kennt nur einfache Anhänge (Phase 1, 07).

## Entscheidung
- **Original unveränderlich:** Das hochgeladene Foto wird bitgenau gespeichert (ADR-017), sein SHA-256 wird beim Empfang berechnet und gespeichert. Ein Austausch ist nicht möglich; eine Korrektur ist ein neues Foto, das das alte ersetzt (`supersedes_id`, ADR-011).
- **Zeitangaben getrennt:** `received_at` (Serverzeit, maßgeblich), `captured_at` aus EXIF bzw. aus der App (Aufnahmezeit laut Gerät, als Angabe des Nutzers gekennzeichnet). Weichen beide deutlich ab (> 24 h), wird das angezeigt, nicht verhindert.
- **EXIF getrennt:** Metadaten werden serverseitig ausgelesen und als JSONB gespeichert. Standortdaten werden nur gespeichert, wenn der Nutzer das in den Einstellungen erlaubt; sonst verworfen.
- **Ableitungen getrennt:** Vorschaubilder und bereinigte Versionen (ohne EXIF, gedreht, verkleinert) sind eigene Objekte mit Verweis auf das Original. Angezeigt und geteilt werden standardmäßig die bereinigten Versionen; das Original nur für Eigentümer und Bearbeiter.
- **Verknüpfung:** Ein Beweisfoto hängt an genau einem Fachobjekt (z. B. Messpunkt, Tankvorgang, Ölmessung, Serviceeintrag) über eine Verknüpfung `attachment_link` mit Rolle (`odometer_display`, `receipt`, `dipstick`, `damage`); Modell siehe `docs/phase-2/10-domaene-documents.md`.
- **App-Aufnahme:** Die Android-App kennzeichnet Fotos aus der Kamera (`capture_source = camera`) und aus der Galerie (`gallery`) unterschiedlich.
- **Optionale Auswertung:** Das Ablesen des Kilometerstands aus dem Foto ist eine Assistant-Funktion (ADR-026) und erzeugt nur einen Vorschlag, den der Nutzer bestätigt.

## Konsequenzen
- (+) Belege sind prüfbar (Hash, Serverzeit), Privatsphäre durch getrennte Ableitungen.
- (−) Mehr Speicherbedarf durch Ableitungen; Vorschaubilder werden asynchron erzeugt (ADR-019).

## Bezug
Auftrag 6.7; Phase 1 07 §8; ADR-011, ADR-017, ADR-019, ADR-026.
