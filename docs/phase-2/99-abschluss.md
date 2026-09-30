# 99 – Abschluss Phase 2

- **Status:** alle Arbeitspakete AP-0 bis AP-11 als Entwurf erledigt · **Datum:** 2026-09-30
- **Leitlinie eingehalten:** Clean-Room-Rebrand nach ADR-000. Kein LubeLogger-Code, -Text oder -Asset ist in Vectra-Dokumenten enthalten. Verweise auf LubeLogger beschränken sich auf Phase-1-IDs (BR, D, R, MG) und auf Fakten über das Fremdformat im Migrationskonzept.

## 1. Ergebnisse

| Bereich | Dokumente |
|---|---|
| Entscheidungen | `00-entscheidungslog.md` (E-0 bis E-14), `01-arbeitsplan.md` |
| Architektur | 30 ADRs in `docs/adr/` (001, 003–031; 002 in 001 aufgegangen) |
| Domänenmodelle | `10-domaene-uebersicht.md` + 11 Module (Odometer, Fuel, ServiceHistory, Maintenance, Costs, Vehicles, Identity, Oil, Trips, Documents, Notes) |
| Regeln | `20-regelkatalog-soll.md`: 60 Phase-1-Regeln → 11 KEEP, 36 FIX, 13 DROP; 97 Soll-Tests, 36 Vergleichstests, Querschnittstests |
| API | `api/openapi.yaml`: 109 Pfade, 162 Operationen, Lint grün, Codegenerierung (oapi-codegen) kompiliert |
| Migration | `30-migrationskonzept.md` (Abbildung aller LubeLogger-Collections, Befundkatalog, CSV-Import, Export) |
| Machbarkeit | `40-spikes.md`: Backend-Budget, Codegenerierung, LiteDB-Leser in Go, Web-Bundles (Android offen) |
| Meldung an LubeLogger | `90-entwurf-meldung-lubelogger.md` (Versand durch den Auftraggeber) |

## 2. Korrekturen an Phase 1 (Errata)

| Stelle | Korrektur | Quelle |
|---|---|---|
| BR-002, Regressionstest | Delta 600 statt 500 | Code erneut gelesen (AP-5) |
| MG-13 | Enums stehen in der LiteDB-Datei als Namen, nicht als Zahlen | Spike S-4 |

## 3. Offene Entscheidungen für den Auftraggeber

| # | Frage | Empfehlung | Quelle |
|---|---|---|---|
| 1 | Ressourcenbudgets als verbindliche Abnahmekriterien (E-14)? | ja. Die Messwerte liegen deutlich darunter | `40-spikes.md` |
| 2 | Wartungsschwellen nach Strecke | „demnächst“ ab 1 500 km, „fällig“ ab 500 km | OP-MA-2 |
| 3 | Kostenansicht Standard | Zahlungsdatum; zeitanteilig als Umschalter | OP-CO-2 |
| 4 | Einladungen gültig für | 7 Tage | ID-04 |
| 5 | Ölstand ohne Peilstab | Prozent der elektronischen Anzeige | OP-OI-1 |
| 6 | Steuerlich anerkanntes Fahrtenbuch als späteres Ziel? | nach MVP entscheiden | OP-TR-1 / Q-13 |
| 7 | Kompatibilitätsschicht zur LubeLogger-API | nicht im MVP; Bedarf nach Pilotphase | Q-07 |
| 8 | Lizenz von Vectra | vor der ersten Veröffentlichung festlegen (in der OpenAPI noch Platzhalter) | ADR-000 |
| 9 | Anonymisierte Beispieldaten von Pilotnutzern | anfragen; werden Golden Files für den Import | Q-18 |

## 4. Einstieg in Phase 3 (Vorschlag)

1. Repository-Gerüst nach ADR-001: Go ≥ 1.25, Module als `internal`-Pakete, Import-Lint, CI mit Lint, Tests, Codegenerierung aus der OpenAPI, sqlc-Prüfung und Budget-Prüfungen (ADR-030).
2. **Android-Spike S-6** zu Beginn (Outbox, Kamera mit Hash, UnifiedPush).
3. Umsetzungsreihenfolge nach Abhängigkeit: Identity → Vehicles → Odometer → Fuel/Oil/ServiceHistory/Maintenance → Costs → Trips → Documents/Notes → Notifications → Import → Web → Android → Assistant.
4. Jede Regel aus `20-regelkatalog-soll.md` wird mit ihrem Soll-Test umgesetzt; FIX-Regeln zusätzlich mit ihrem Vergleichstest.
5. Jeder PR enthält die ADR-000-Checkbox („kein LubeLogger-Code“).

Phase 3 beginnt erst nach Freigabe durch den Auftraggeber.
