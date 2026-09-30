# 20 – Soll-Regelkatalog Vectra

- **Status:** Entwurf (AP-5) · **Datum:** 2026-09-30
- **Grundlagen:** ADR-000 (Clean Room), ADR-031 (Altfehler korrigieren), Domänenmodelle `10-domaene-*.md`

## 1. Zweck und Lesehilfe

Der Katalog ordnet **jeder** der 60 Phase-1-Regeln (BR-001 bis BR-060) genau eine Klasse zu und verweist auf die Vectra-Regel, die das Verhalten neu beschreibt:

| Klasse | Bedeutung |
|---|---|
| **KEEP** | Das Verhalten ist fachlich richtig und wird in eigenen Worten neu spezifiziert. Das Ergebnis ist in den Beispielen gleich. |
| **FIX** | Das Verhalten wird korrigiert. Jede FIX-Regel hat ein Vergleichsbeispiel „LubeLogger ergibt X, Vectra ergibt Y“, das als Regressionstest dient. |
| **DROP** | Die Regel entfällt, weil die Funktion nicht im MVP ist (E-5) oder weil Vectra den Mechanismus nicht braucht. |

Die Vectra-Regeln selbst (Formeln, Randfälle) stehen in den Domänenmodellen. Dieser Katalog wiederholt sie nicht, sondern verweist auf sie (z. B. FU-02). **Maßgeblich ist das Soll-Verhalten.** LubeLogger-Werte sind dokumentierte Vergleichswerte, keine Zielwerte (ADR-031).

**Summe:** 60 Regeln → 11 KEEP · 36 FIX · 13 DROP.

Die Abgleichtabellen in den Domänenmodellen waren vorläufig. Bei Abweichungen gilt dieser Katalog; die Tabellen wurden angeglichen.

## 2. Kraftstoff und Verbrauch

| BR | Thema | Klasse | Vectra | Begründung | Vergleich LubeLogger → Vectra |
|---|---|---|---|---|---|
| BR-001 | Reihenfolge der Tankvorgänge | FIX | Übersicht §3.1, FU-01 | Sortierung nach Datum und Stand verdeckt Fehleingaben | (02.01., 1 500 km), (01.01., 2 000 km): LL rechnet Δ = 0 ohne Hinweis → Vectra meldet beim Erfassen P1; nach Bestätigung ist das Intervall „nicht berechenbar (Distanz ≤ 0)“ |
| BR-002 | Delta-Distanz | FIX | ODO-05, FU-02, FU-03 | negative Deltas werden still zu 0 | Stände 1 000 / 900 / 1 500 (alle voll): LL Δ = 0, 0, 600 → Vectra: 900 löst P1 aus; nach Bestätigung ist 1 000→900 nicht berechenbar, 900→1 500 = 600 km, der Grund ist sichtbar |
| BR-003 | Verbrauch bei Vollbetankung | FIX | FU-02, FU-03 | Grundformel bleibt; korrigiert wird der Randfall, in dem bei Distanz ≤ 0 aufsummierte Teilmengen verloren gehen | A (1 000 km, 40 l, voll), B (1 500, 20 l, teil), C (1 500, 30 l, voll), D (2 000, 35 l, voll): LL C = 0 und B+C verworfen, D = 7,00 l/100 km → Vectra A→C = 50 l / 500 km = 10,00; C→D = 7,00 |
| BR-004 | Teilbetankungen aufsummieren | KEEP | FU-02 | fachlich richtig | Daten BR-003 aus Phase 1: beide 5,00 l/100 km |
| BR-005 | ausgelassener Vorgang | KEEP | FU-03 | fachlich richtig | beide: Vorgang mit Kennzeichen liefert keinen Wert, nächster Abschnitt 20 mpg |
| BR-006 | erster Vorgang | KEEP | FU-02 (Anker) | fachlich richtig | beide: kein Verbrauch |
| BR-007 | Vorgang ohne Stand | KEEP | FU-02 | Menge zählt zum nächsten abgeschlossenen Intervall | beide: (1 000, 40, voll), (–, 20, voll), (2 000, 30, voll) → 50 l / 1 000 km |
| BR-008 | UK-Gallonen | FIX | ADR-007 | Näherungsfaktor 4,546 statt exakt 4,54609 | 40 l in imp gal: LL 8,7989 → Vectra 8,7988 |
| BR-009 | Stromverbrauch | FIX | FU-05 | Kapazitätsschätzung je Ladung enthält Ladeverluste | Ladungen (1 000 km, 20→80 %), (1 200 km, 40→80 %, 20 kWh), nutzbare Kapazität 45 kWh: LL 10,0 kWh/100 km → Vectra Batterie 18 kWh / 200 km = 9,0; Netzbezug 10,0 |
| BR-010 | Preis pro Einheit | KEEP | FU-06 | fachlich richtig | beide: 50 € / 40 l = 1,25 €/l |
| BR-011 | Durchschnittsverbrauch | FIX | FU-04 | Mengen nicht abgeschlossener Intervalle verfälschen den Durchschnitt | Phase-1-Daten plus Teilbetankung (10.03., 3 000 km, 15 l): LL 5,31 l/100 km → Vectra 5,00 |
| BR-012 | Monatswerte | FIX | FU-04 | ungewichtet und über Jahre zusammengelegt | Jan 2025 = 5,0, Jan 2026 = 7,0: LL „Januar“ = 6,0 → Vectra 2025-01 = 5,0 und 2026-01 = 7,0 |
| BR-013 | l/100 km ↔ km/l | KEEP | ADR-007 | reine Anzeige | beide: 5,00 l/100 km = 20,00 km/l |
| BR-014 | Preis × Menge | FIX | FU-07 | Rundung nur im Browser, API/CSV ungerundet | 1,799 €/l × 40,12 l per CSV: LL 72,17588 → Vectra 72,18 € auf allen Wegen |
| BR-059 | Einheitenmodell | FIX | ADR-007 | Werte ohne Einheit werden je Nutzer anders gedeutet | gespeicherte 40 bei Nutzer mit Gallonen-Einstellung: LL zeigt „40 gal“ → Vectra speichert 40 l und zeigt 10,57 US gal |

## 3. Kilometerstand

| BR | Thema | Klasse | Vectra | Begründung | Vergleich LubeLogger → Vectra |
|---|---|---|---|---|---|
| BR-015 | aktueller Stand | FIX | ODO-04 | Maximum macht Tippfehler dauerhaft | Tankvorgang 2 800 km, danach Messung 500 km: LL 2 800 → Vectra `422` P1; nach Bestätigung 500 (spätester Messpunkt), sonst Korrektur |
| BR-016 | minimaler Stand | FIX | ODO-05 | Minimum über Typen verfälscht Distanzen; 0 = unbekannt | Werte 0 / 500 / 1 000: LL min 500 → Vectra kennt kein Minimum; 0 ist kein Messpunkt, sondern „kein Stand“ |
| BR-017 | letzter Odometer-Stand | FIX | ODO-04, ODO-06 | Maximum statt letzter Wert | Messungen in zeitlicher Folge 1 000 / 3 000 / 2 000 (2 000 bestätigt): LL 3 000 → Vectra Vorbelegung 2 000 |
| BR-018 | Startwert automatischer Einträge | FIX | I-ODO-3, ODO-05 | Startwert aus dem Maximum erzeugt negative Distanzen | Maximum 3 000, Service mit 2 500 (zeitlich davor): LL Eintrag 3 000→2 500 (−500) → Vectra Messpunkt 2 500 an seiner zeitlichen Stelle, keine gespeicherte Distanz |
| BR-019 | Distanzen neu berechnen | FIX | ODO-05 | gespeicherte Distanzen veralten | 1 000 / 1 500 / 1 400: LL speichert 0 / 500 / −100 → Vectra: 1 400 löst P1 aus; nach Bestätigung liefert `DistanceBetween` −100 mit Kennzeichnung „bestätigte Anomalie“, gespeichert wird nichts |
| BR-020 | Distanz eines Eintrags | FIX | ODO-05 | wie BR-019 | Start 1 500, Stand 1 400: LL −100 gespeichert → Vectra berechnet bei Abfrage |
| BR-021 | keine Plausibilitätsprüfung | FIX | ODO-02, ODO-03 | Auftrag 6.4 | 500 nach 2 800: LL `success: true` → Vectra `422` P1 |
| BR-022 | Tachokorrektur | FIX | ODO-07 | drei Rundungsvarianten, kein Gültigkeitszeitraum | Differenz 100, Faktor 1,6, Rohwert 999: LL 1 760 / 1 758 / 1 758,4 je nach Weg → Vectra speichert Rohwerte; Tachotausch als Zählerabschnitt mit Versatz, Faktor nicht im MVP |
| BR-023 | Distanz auf andere Fahrzeuge | DROP | – | nicht im MVP | – |
| BR-024 | Laufleistung Ausstattung | DROP | – | Ausstattung nicht im MVP (E-5) | – |
| BR-025 | Ausstattung verknüpfen | DROP | – | wie BR-024 | – |
| BR-060 | Distanz im Report-Kopf | FIX | ODO-05, CO-05 | zwei Distanzdefinitionen widersprechen sich | nur Tankvorgänge 1 000 … 2 800 km: LL Kopf 0 km, Tabelle 1 800 km → Vectra überall 1 800 km |

## 4. Wartung (Reminder)

| BR | Thema | Klasse | Vectra | Begründung | Vergleich LubeLogger → Vectra |
|---|---|---|---|---|---|
| BR-026 | Dringlichkeit nach Datum | FIX | MA-04 | Uhrzeitvergleich macht den Fälligkeitstag überfällig | jetzt 30.09.2026 12:00, fällig 05.10.: LL „sehr dringend“, 4 Tage → Vectra `due`, 5 Tage. Am 05.10.: LL überfällig → Vectra `due`, 0 Tage |
| BR-027 | Dringlichkeit nach Distanz | KEEP | MA-04 | Logik richtig (Gleichstand nicht überfällig); Schwellen siehe BR-029 | beide: fällig 2 800 bei Stand 2 800 → nicht überfällig |
| BR-028 | kombinierte Auslöser | KEEP | MA-06 | „was zuerst erreicht wird“ | beide: Datum in 20 Tagen, km-Rest 40 → höhere Stufe durch km |
| BR-029 | Schwellen | FIX | MA-05 | eigene Schwellen wirken auf folgende Einträge; Distanzschwellen ohne Einheit | A (eigene Schwelle 365 Tage, fällig in 200 Tagen), B (Vorgabe, fällig in 100 Tagen): LL B „dringend“ → Vectra B `ok` |
| BR-030 | nächste Fälligkeit | FIX | MA-07 | Auswahl mischt Metriken | Datum-Wartung in 30 Tagen, km-Wartung mit Zufallsdatum morgen: LL wählt die km-Wartung wegen des Datums → Vectra wählt nach Stufe, dann nach Prognose |
| BR-031 | Fortschreibung | FIX | MA-02, MA-03 | fortgeschriebene Werte driften (Monatsende), Fortschreibung als Seiteneffekt | monatlich ab 31.01.: LL 28.02. → 28.03. → Vectra 28.02. → 31.03. Ohne festes Raster bleibt das Ergebnis gleich (10.03.2026 / 45 000 → 10.03.2027 / 60 000) |
| BR-032 | automatische Fortschreibung überfälliger Einträge | DROP | MA-08 | Lesezugriff verändert Daten | 12 Monate, fällig 01.01.2024, heute 30.09.2026: LL nach Seitenaufruf 01.01.2025 → Vectra unverändert 01.01.2024, `overdue` |
| BR-033 | Erledigung durch Einträge | FIX | SH-04, MA-01 | keine Verknüpfung, Fortschreibung auch bei fehlgeschlagenem Speichern | Service zweimal gespeichert: LL schreibt zweimal fort → Vectra eine Erledigung, Fälligkeit einmal verschoben |
| BR-034 | Benachrichtigungen | FIX | MA-09, ADR-020 | Deduplizierung nur im Speicher; Haushaltsmitglieder ausgeschlossen | Neustart nach Meldung „dringend“: LL meldet erneut → Vectra keine zweite Meldung; alle Mitglieder mit Abo werden benachrichtigt |

## 5. Steuern, Gebühren, Kosten

| BR | Thema | Klasse | Vectra | Begründung | Vergleich LubeLogger → Vectra |
|---|---|---|---|---|---|
| BR-035 | wiederkehrende Gebühren | FIX | CO-02, I-CO-1 | stille Erzeugung im Hintergrund; Intervall 0 serverseitig nicht verhindert | Gebühr 15.01.2025, 3 Monate, heute 30.09.2025: LL legt 15.04. und 15.07. an → Vectra zeigt zwei offene Vorkommen, Einträge erst nach Bestätigung; Intervall 0 → `422` |
| BR-036 | Gesamtkosten | KEEP | CO-01, CO-03 | Summe über alle Kostenquellen | beide: Service 100 + Reparatur 50 + Tanken 250 = 400 |
| BR-037 | Kosten je Monat | FIX | CO-03, CO-05 | Jahre zusammengelegt; Monatsdistanz als Maximum über Typen | Jan 2025 = 100 €, Jan 2026 = 50 €: LL „Januar“ 150 € → Vectra 2025-01 = 100 €, 2026-01 = 50 € |
| BR-038 | Kosten je Distanz/Tag | FIX | CO-05 | Distanz = Maximum − Minimum, Tippfehler wirken voll | Phase-1-Daten (250 €, Stände 1 000–2 800 plus Fehlwert 500): LL 250 / 2 300 = 0,11 €/km → Vectra nach Klärung der Anomalie 250 / 1 800 = 0,14 €/km |
| BR-039 | Besitztage | FIX | CO-05 | Abschneiden der Uhrzeit, kulturabhängiges Parsen | 01.01.–30.09.2026: LL 272 → Vectra 273 (Kalendertage inklusive) |
| BR-040 | Druckbericht, Wertverlust | FIX | CO-06, CO-07 | Gesamtkosten ohne Kraftstoff | Service 1 000 €, Tanken 2 000 €, 10 000 km: LL „Gesamt“ 1 000 € (0,10 €/km) → Vectra laufende Kosten 3 000 € (0,30 €/km), nach Kategorie aufgeschlüsselt. Wertverlust bei beiden 13,66 €/Tag, 0,50 €/km |
| BR-041 | Garage-Kennzahlen | FIX | CO-05, `FleetCostReport` | übernimmt die Distanz aus BR-038 | wie BR-038 |
| BR-048 | Kiosk-Statistiken | DROP | – | Kiosk nicht im MVP | – |

## 6. Lager, Planer, Inspektion

| BR | Thema | Klasse | Vectra | Begründung |
|---|---|---|---|---|
| BR-042 | Abbuchung Lagerteile | DROP | – | Teilelager nicht im MVP (E-5); Altdaten als Notiz (NO-03) |
| BR-043 | Rückbuchung | DROP | – | wie BR-042 |
| BR-044 | Plan „erledigt“ erzeugt Service | DROP | – | Planer nicht im MVP |
| BR-045 | Planvorlage | DROP | – | wie BR-044 |
| BR-046 | Bewertung Inspektionsfelder | DROP | – | Checklisten nicht im MVP; Inspektion als Eintragsart `inspection` |
| BR-047 | Seiteneffekte Inspektion | DROP | – | keine Kopien in Vectra |

## 7. Stammdaten und Sonstiges

| BR | Thema | Klasse | Vectra | Begründung | Vergleich LubeLogger → Vectra |
|---|---|---|---|---|---|
| BR-049 | Zusatzfelder zusammenführen | FIX | Vehicles §2.5 | stiller Datenverlust | Werte {A=1, B=2}, Vorlage [B, C]: LL [B=2, C=""], A verloren → Vectra B=2, C leer, A als verwaistes Feld erhalten |
| BR-050 | Fahrzeugkennung | KEEP | VE-02 | fachlich sinnvoll | beide: Kennung „FIN“ zeigt die FIN |
| BR-051 | Sitzungsdauer | FIX | ID-02 | Cookie-basierte Sitzung ohne Widerruf | Laufzeit 120 Tage konfiguriert: LL Cookie 90 Tage → Vectra serverseitig, 7 Tage Leerlauf, höchstens 30 Tage, jederzeit widerrufbar |
| BR-052 | CSV-Import | FIX | Migrationskonzept (AP-7) | kulturabhängiges Parsen, fehlendes Datum wird „heute“, Zeilen werden still übersprungen | `01/05/2026,12.345,40,…`: LL Stand 12 oder 12 345 je nach Serverkultur → Vectra: Formate werden im Analyseschritt festgelegt; unklare Zeilen erscheinen im Prüfbericht, fehlendes Datum ist ein Fehler |
| BR-053 | iCal-Export | DROP | – | nicht im MVP; später mit stabiler UID (Definitions-ID) und Zeitzone | – |
| BR-054 | Webhook-Zustellung | FIX | ADR-020, ADR-019 | Wartezeit nur bei 429, Netzwerkfehler unbehandelt, keine Signatur | Antworten 500, 500, 200: LL drei Aufrufe ohne Pause → Vectra drei Aufrufe mit exponentieller Wartezeit, signiert, nach 10 Fehlversuchen Dead-Letter |
| BR-055 | Zeitplan automatischer Ereignisse | FIX | ADR-019, MA-09 | Tageslauf zur Serverzeit, Zustand im Speicher | Start 07:00 bei Laufzeit 06:30: LL erster Lauf am Folgetag → Vectra stündliche Fälligkeitsprüfung plus sofortige Neubewertung bei Änderungen |
| BR-056 | Health-Status | KEEP | Betrieb (AP-8) | Semantik pass/degraded/fail sinnvoll; Vectra prüft zusätzlich Storage und Job-Queue | beide: DB nicht erreichbar → `fail` |
| BR-057 | Typwechsel | FIX | I-SH-4 | Kopieren und Löschen ohne Transaktion, neue ID | Service 7 → Reparatur: LL neue ID, Verweise ins Leere → Vectra gleiche ID, Audit „kind“ |
| BR-058 | Verweise als Anhang | DROP | Documents §2.2 | typisierte Verknüpfungen ersetzen den Mechanismus | – |

## 8. Datenbefunde ohne eigene BR-Nummer

| Befund (Phase 1, 09 §2) | Vectra | Klasse |
|---|---|---|
| D-03 Datum verschiebt sich bei Zeitzonenwechsel | ADR-008 | FIX |
| D-06 CSV-Export mit berechneten statt gespeicherten Mengen | Export liefert gespeicherte Originalwerte mit Einheit (AP-7) | FIX |
| D-07 keine Transaktionen | ADR-004 | FIX |
| D-08 Fahrzeug-ID-Spalte bei PostgreSQL veraltet | relationales Schema, ADR-004; Import prüft (ADR-027) | FIX |
| D-10 Duplikate durch Planer und Inspektion | Funktionen nicht im MVP; Import erkennt Duplikate (ADR-027) | DROP |
| D-13 Tachokorrektur uneinheitlich | ODO-07 | FIX |
| D-14 Zusatzfeld-Verlust | Vehicles §2.5 (BR-049) | FIX |
| D-15 harte Löschungen | ADR-011, DO-06 | FIX |
| D-16 Backup ohne Fachdaten bei PostgreSQL | Backup-Konzept (ADR-030, AP-8) | FIX |
| D-17 Deduplizierung im Speicher | MA-09, ADR-020 | FIX |
| D-18 falscher Herkunftstext bei Inspektionen | entfällt mit BR-047 | DROP |

## 9. Neue Regeln ohne Vorbild

Diese Regeln haben kein Gegenstück in LubeLogger. Sie sind in den Domänenmodellen spezifiziert:

| Bereich | Regeln |
|---|---|
| Odometer | ODO-01 bis ODO-08 (Plausibilität, Zählerabschnitte, Interpolation, Tagesleistung) |
| Fuel | FU-05 (Netzbezug/Batterie), FU-08 (Plausibilität) |
| ServiceHistory | SH-01 bis SH-04 (Kostenpositionen, idempotente Erledigung) |
| Maintenance | MA-02 Raster ab Anker, MA-07 Prognose, MA-08 keine Fortschreibung |
| Costs | CO-01 Kostenbuch, CO-04 zeitanteilig, CO-06/07 Wertverlust und TCO |
| Oil | OI-00 bis OI-05 (komplett neu, Auftrag 6.6) |
| Trips | TR-01 bis TR-06 (komplett neu, Auftrag 6.8) |
| Vehicles | VE-01 FIN-Prüfung, I-VE-2/5 |
| Identity | ID-01 bis ID-05 |
| Documents | DO-01 bis DO-06 |
| Notes | NO-01 bis NO-04 |

## 10. Regressionstestliste

### 10.1 Soll-Tests aus den Domänenmodellen
Jedes Soll-Beispiel wird ein automatisierter Test. Die ID des Beispiels wird Teil des Testnamens (z. B. `TestFuel_U5_WeightedAverage`).

| Modul | Beispiele | Anzahl | Ebene |
|---|---|---|---|
| Odometer | O-1 … O-10 | 10 | Domäne + Integration (Transaktion bei Korrektur) |
| Fuel | U-1 … U-11 | 11 | Domäne |
| ServiceHistory | S-1 … S-7 | 7 | Integration (Idempotenz, Löschkaskade) |
| Maintenance | M-1 … M-13 | 13 | Domäne (Uhr injiziert) |
| Costs | C-1 … C-9 | 9 | Domäne + Integration (Kostenbuch) |
| Vehicles | V-1 … V-7 | 7 | Domäne + API |
| Identity | I-1 … I-8 | 8 | API (Rechte, Statuscodes) |
| Oil | L-1 … L-12 | 12 | Domäne |
| Trips | T-1 … T-9 | 9 | Domäne + Integration (Überlappung unter Parallelität) |
| Documents | D-1 … D-7 | 7 | API + Integration (Storage) |
| Notes | N-1 … N-4 | 4 | API + Web-Komponententest (Darstellung) |
| **Summe** | | **97** | |

### 10.2 Vergleichstests (FIX)
Für jede FIX-Regel in §2 bis §7 gibt es einen Test `TestCompat_BRxxx`. Er rechnet das Vergleichsbeispiel mit Vectra und prüft den **Vectra-Wert**. Der LubeLogger-Wert steht als Kommentar im Test und dient nur zur Dokumentation (36 Tests).

### 10.3 Querschnittstests
- **Rechte (ID-01):** Für jede API-Operation aus der OpenAPI-Spezifikation (AP-6) wird automatisch geprüft: ohne Anmeldung → `401`; fremdes Fahrzeug → `404`; Leser bei schreibender Operation → `403`; Token ohne Scope → `403`. Die Tests werden aus der Spezifikation generiert, damit neue Endpunkte nicht vergessen werden.
- **Idempotenz (ADR-012):** jede `POST`-Operation zweimal mit gleicher ID bzw. gleichem Idempotency-Key.
- **Locking:** jede `PATCH`-Operation mit veraltetem `If-Match` → `412`.
- **Einheiten (ADR-007):** Rundlauftest Eingabe → kanonisch → Anzeige für alle Einheitencodes.
- **Zeit (ADR-008):** Fälligkeits- und Monatsgrenzen in den Zeitzonen `Europe/Berlin`, `America/Los_Angeles` und `Pacific/Auckland`, inklusive Sommerzeitwechsel.
- **Import (AP-7):** anonymisierte LubeLogger-Datenbank → Prüfbericht mit erwarteten Befunden (golden file).

## 11. Korrekturen an Phase 1

- **BR-002, Regressionstest:** Die Erstfassung nannte für (1 000), (900), (1 500) die Deltas 0, 0, 500. Richtig ist 0, 0, **600**, weil der Bezugswert auch nach einem negativen Delta auf 900 gesetzt wird. In `docs/phase-1/04-geschaeftsregeln.md` als Erratum vermerkt. Die Klassifikation ändert sich nicht.
