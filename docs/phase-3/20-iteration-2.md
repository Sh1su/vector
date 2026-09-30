# Phase 3 – Iteration 2 (Teil 1): Fahrten, Wartung, Servicehistorie, Kosten

- **Status:** umgesetzt · **Datum:** 2026-09-30 · **Auftrag:** „Fahrten, Wartung, Servicehistorie und Kosten nun angeben“
- **Umfang:** die vier Module der Navigationsgruppe „Alltag“. Fahrten (planmäßig Iteration 3) wurden auf Wunsch vorgezogen. Kraftstoff und Öl folgen im zweiten Teil der Iteration 2.

## Backend

| Modul | Paket | Schema | Regeln |
|---|---|---|---|
| Maintenance | `internal/maintenance` | `maintenance.item`, `maintenance.completion` | MA-01 bis MA-08 (Fälligkeit wird immer berechnet, nie fortgeschrieben), I-MA-1 bis I-MA-4 |
| ServiceHistory | `internal/servicehistory` | `service.entry`, `service.cost_item`, `service.part_line` | SH-01 bis SH-04, I-SH-1 bis I-SH-4 |
| Costs | `internal/costs` | `costs.entry`, `costs.plan`, `costs.occurrence_dismissal`, `costs.ledger` | CO-01 bis CO-07, I-CO-1 bis I-CO-4 |
| Trips | `internal/trips` | `trips.trip`, `trips.category` | TR-01 bis TR-05, I-TR-1 bis I-TR-5 |

Querschnitt:
- **Odometer-Batch** (`odometer/batch.go`): Quellmodule legen, korrigieren und löschen ihre Messpunkte in ihrer eigenen Transaktion. Alle Kandidaten werden gemeinsam geprüft, damit sämtliche Befunde (z. B. Start- und Endstand einer Fahrt) in einer 422-Antwort erscheinen.
- **I-ODO-3** gilt jetzt auch für Korrekturen: Messpunkte aus Service oder Fahrten werden nur über ihren Eintrag geändert (409 über `/odometer/readings/{id}/corrections`).
- **Kostenbuch:** Serviceeinträge und sonstige Kosten schreiben ihre Zeilen in derselben Transaktion; alle Auswertungen lesen nur das Kostenbuch.
- **Erledigungen aus Serviceeinträgen** (SH-04) sind idempotent je (Definition, Eintrag), folgen Datums-/Standänderungen und verschwinden beim Löschen.
- Gemeinsame Bausteine: `kernel/date.go` (Kalenderdaten, Monatsaddition), `kernel/money.go`, `platform/pgconv`, `platform/mergepatch`, `platform/cursor`.

Spezifikation: `DueStatus.vehicle_id` (Feed über alle Fahrzeuge) und `TripCategory.version` (If-Match beim Ändern) ergänzt.

## Tests

- Unit-Tests der Fachlogik mit den Soll-Beispielen **M-1 bis M-13** und **C-1 bis C-7**.
- Integrationstests gegen PostgreSQL (`server/modules_test.go`): **S-1 bis S-6**, M-9, C-2 bis C-9, **T-1 bis T-9**, I-ODO-3, Idempotency-Key.
- Sichtprüfung im Browser (Playwright) aller vier Seiten und der Übersicht.

## Web

Neue Seiten `Fahrten`, `Wartung`, `Servicehistorie`, `Kosten`; die Übersicht zeigt die nächste Wartung, die Jahreskosten und Schnellerfassung. Initiales JS 125 KB gzip (Budget 300 KB).

## Annahmen und offene Punkte

- Schwellen: Definition → Installationsvorgabe (30/7 Tage, 1 500/500 km). Eine Nutzereinstellung dazwischen folgt mit der Einstellungsseite.
- Wertverlust bezieht sich auf die gesamte Besitzdauer; die Auswertung weist darauf hin.
- Android: Fahrten, Service und Kosten mobil sind Teil von Iteration 6.
- Benachrichtigungen bei Fälligkeit (MA-09) folgen mit Notifications in Iteration 4.
