# Phase 3 – Iteration 2 (Teil 1): Kraftstoff, Öl, Wartung, Einstellungen, Assistent

- **Datum:** 2026-10-02 · **Anlass:** Rückmeldung des Auftraggebers nach dem Start des Docker-Images: Kraftstoff, Öl, Assistent und Einstellungen fehlten in der Web-App („in Arbeit“); gewünscht waren außerdem vorbereitete Wartungspläne für ein Mercedes-Benz-CDI-Fahrzeug, Modelljahr 2019.
- **Grundlagen:** `docs/phase-2/10-domaene-fuel.md`, `-oil.md`, `-maintenance.md`, ADR-007, ADR-010, ADR-012, ADR-024 bis ADR-026.

## Umgesetzt

| Bereich | Backend | Web |
|---|---|---|
| **Kraftstoff** (FU-01 bis FU-08) | `fuel-fills` (CRUD, Merge Patch, If-Match, idempotent), eigener Messpunkt je Vorgang (I-FU-3/4), Intervallverbrauch, Ø und Monatswerte je Abschlussmonat, Netzbezug und Batterieverbrauch bei Strom, Preis pro Einheit serverseitig (FU-07), F1–F4 und ODO-03 in **einer** 422-Antwort | Seite „Kraftstoff“: Erfassen (Betrag oder Preis je Einheit), Verlauf mit Intervallverbrauch und Gründen, Monatsbalken, Kennzahlen |
| **Öl** (OI-00 bis OI-05) | `oil-entries`, Messreihen ab Ölwechsel, Paarverbrauch (Volumen oder Prozentpunkte), Nachfüllrate mit Randfällen, Statistik, Hinweis „Verbrauch gestiegen“ | Seite „Öl“: Messung/Nachfüllung/Ölwechsel, Stand als Prozent oder Stufe, Messreihen, Kennzahlen |
| **Wartung** (MA-01 bis MA-07) | Definitionen, Erledigen/Auslassen mit Idempotency-Key, Fälligkeit nach Zeit/Distanz, festes Raster, Schwellen Definition → Nutzer → Installation, Prognose über ODO-08, `/me/due`, **Vorlagen** (`GET /maintenance-templates`, neu in der Spezifikation) | Seite „Wartung“: Liste nach Dringlichkeit, Anlegen/Bearbeiten, Erledigen, **Vorlagen-Dialog** zum Vorab-Erfassen (Intervalle anpassen, „zuletzt erledigt am / bei km“ je Position) |
| **Einstellungen** | Validierung der Nutzereinstellungen, Passwortwechsel (beendet andere Sitzungen), Sitzungsliste, Installationseinstellungen für Admins (`assistant_enabled`, Vorgabe-Schwellen, Aufbewahrung) | Seite „Einstellungen“: Profil, Darstellung, Einheiten, Wartungsschwellen, Passwort, Geräte, Assistent, Installation (Admin) |
| **Assistent** (ADR-024/026) | Anbieter `anthropic` (offizielles Go-SDK, serverseitiger Ausweichpfad bei Ablehnung) und `openai_compatible`/`ollama`; Zustimmung je Nutzer bei externem Anbieter; lesende Werkzeuge mit Nutzerrechten; schreibende Werkzeuge erzeugen **Vorschläge** mit Probelauf (Transaktion wird zurückgerollt) und Befunden; Bestätigen führt mit Herkunft `assistant` aus; SSE-Strom; Protokoll ohne Inhalte; Tageslimit; Aufbewahrungsfrist | Seite „Assistent“: Zustimmung, Unterhaltungen, Chat mit Zwischenständen, Vorschlagskarten (Bestätigen/Bearbeiten/Verwerfen, Befunde bestätigen) |
| Übersicht | – | echte Kennzahlen (Ø Verbrauch, Ölverbrauch), nächste Wartungen, Schnellerfassung |

## Wartungsplan-Vorlagen

Eingebaut sind vier Vorlagen (`backend/internal/maintenance/templates.go`):

- **Mercedes-Benz Sprinter CDI (Baureihe 907/910, ab 2018)** – Basisfahrzeug vieler Reisemobile, Modelljahr 2019 mit OM651/OM654.
- **Mercedes-Benz Vito / V-Klasse / Marco Polo CDI (W447)**.
- **Reisemobil-Aufbau (Ergänzung)** – Gasprüfung G 607, Dichtigkeitsprüfung, Wasseranlage, Aufbaubatterie u. a.
- **PKW allgemein (Deutschland)**.

Die Intervalle sind **Richtwerte** und deshalb nicht als Herstellervorgabe markiert (`manufacturer_recommended = false`, AP-10: eine Vorgabe braucht eine Quelle). Maßgeblich bleiben Serviceheft und ASSYST-PLUS-Anzeige. Optionale Positionen (z. B. Automatikgetriebeöl) sind beim Übernehmen abgewählt.

## Abweichungen und Entscheidungen

- `GET /assistant/status` antwortet auch bei ausgeschaltetem Assistenten (mit `enabled: false`; Admins sehen zusätzlich, ob ein Anbieter konfiguriert ist). Alle übrigen Assistant-Endpunkte bleiben dann `404`. In der Spezifikation vermerkt.
- Anbieter, Modell und Schlüssel kommen aus der Umgebung des Servers, nicht aus der Datenbank (keine Geheimnisse in der Oberfläche). Admins schalten nur ein/aus. Bei `anthropic` ist ohne Angabe `claude-opus-5-5` voreingestellt.
- RAG/Dokumentensuche (ADR-025) folgt mit dem Modul Documents (Iteration 3); das Werkzeug `search_documents` fehlt deshalb noch.
- Kosten-Ereignisse (Fuel → Costs) und die Verknüpfung Öl ↔ Serviceeintrag folgen mit Costs/ServiceHistory; `service_entry_id` wird bis dahin abgelehnt.
- Ohne „zuletzt erledigt“ zählt bei Wartungen das Anlegedatum als Start; die Liste weist darauf hin.

## Tests

- Fachregeln als Tabellentests mit den Soll-Beispielen: U-1 bis U-11 (Fuel), L-1 bis L-12 (Oil), M-1 bis M-13 (Maintenance), FU-07 (Rundung).
- Integrationstests gegen PostgreSQL: Fuel, Oil, Maintenance inkl. Vorlagen und Idempotenz, Einstellungen/Passwort/Sitzungen/Installation, Assistent mit Fake-Anbieter (Probelauf schreibt nichts, Bestätigung mit Audit-Akteur `assistant`, Befunde nur durch den Nutzer, fremde Fahrzeuge unsichtbar).
- Adaptertests gegen nachgebaute Anthropic- und OpenAI-Endpunkte (Werkzeugschleife, Ausweichpfad, Ablehnung).
- Browser-Durchlauf (Playwright) über alle neuen Seiten inkl. Assistent mit lokalem Fake-Modell.
- Web-Build: initiales JS 116 KB gzip (Budget 300 KB); Fachseiten werden nachgeladen.

## Offen für Iteration 2 (Teil 2)

ServiceHistory (Serviceeinträge erledigen Wartungen, SH-04) und Costs (Kostenbuch, Kraftstoffkosten per Event).
