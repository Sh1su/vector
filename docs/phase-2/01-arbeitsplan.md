# Phase 2 – Arbeitsplan und Status

Phase 2 liefert Entscheidungen und Spezifikationen für Vectra, **keinen Produktivcode** (Auftrag §3/§5). Leitlinie ist ADR-000: Clean-Room-Rebrand ohne LubeLogger-Code. Grundlage sind die Entscheidungen in `00-entscheidungslog.md`.

## MVP-Modulzuschnitt (nach E-5)

| Modul | Im MVP | Anmerkung |
|---|---|---|
| Identity (Konten, OIDC + lokal, Rollen je Fahrzeug, API-Tokens) | ✓ | ADR-015, ADR-016 |
| Vehicles | ✓ | inkl. VIN/FIN, Kraftstoffart, Fahrzeugbilder |
| Odometer | ✓ | Messpunkte mit Herkunft, Foto, Tachotausch, Korrekturen |
| Fuel (inkl. Laden bei E-Fahrzeugen) | ✓ | |
| Oil | ✓ | neu gegenüber LubeLogger |
| Trips | ✓ | neu; Fahrten append-only (Q-13) |
| Maintenance (Wartungsdefinitionen, Fälligkeit) | ✓ | ersetzt LubeLogger-Reminder |
| ServiceHistory (Wartung, Reparatur, Upgrade) | ✓ | Teile/Arbeit getrennt |
| Costs/Reporting (inkl. Steuern, Versicherung, Gebühren) | ✓ | |
| Documents (inkl. Beweisfotos) | ✓ | |
| Notes | ✓ | |
| Notifications (E-Mail, UnifiedPush, optional FCM, Webhooks) | ✓ | |
| Import/Migration (LubeLogger, CSV) | ✓ | |
| Assistant (KI) | optional | abschaltbar, zuletzt spezifiziert |
| Aufgabenplaner, Teilelager, Inspektionen, Ausstattung, Kiosk, Widgets, Themes, Imagemap, Sticker | – | zurückgestellt; Altdaten bleiben erhalten |

## Arbeitspakete

| AP | Inhalt | Ergebnis (Datei) | Status |
|---|---|---|---|
| AP-0 | Entscheidungsworkshop | `00-entscheidungslog.md` | ✅ erledigt |
| AP-1 | Grundsatz-ADRs aus Workshop | `docs/adr/ADR-000, 001, 006–010, 015, 016, 020, 027, 029, 031` | ✅ Erstfassung akzeptiert |
| AP-2 | Querschnitts-ADRs: Tech-Stack (003–005), Audit (011), Locking/Idempotenz (012), API-Konventionen (013), OpenAPI-Workflow (014), Storage/Beweisfotos (017/018), Jobs (019) | `docs/adr/ADR-003–005, 011–014, 017–019` | ✅ Erstfassung akzeptiert |
| AP-3 | Domänenmodelle Fachkern: Odometer, Fuel, ServiceHistory, Maintenance, Costs | `docs/phase-2/10-domaene-uebersicht.md`, `-odometer`, `-fuel`, `-servicehistory`, `-maintenance`, `-costs` | ✅ Entwurf |
| AP-4 | Domänenmodelle neue Module: Oil, Trips, Documents, Notes, Vehicles, Identity | `docs/phase-2/10-domaene-vehicles`, `-identity`, `-oil`, `-trips`, `-documents`, `-notes` | ✅ Entwurf |
| AP-5 | Soll-Regelkatalog (Neuformulierung, KEEP/FIX/DROP je Phase-1-Regel, neue Regeln) + Regressionstestliste | `docs/phase-2/20-regelkatalog-soll.md` | ✅ Entwurf |
| AP-6 | OpenAPI-3.1-Entwurf Kernressourcen | `api/openapi.yaml`, `api/README.md` (nur Spezifikation) | ✅ Entwurf, Lint grün |
| AP-7 | Migrationskonzept LubeLogger → Vectra | `docs/phase-2/30-migrationskonzept.md` | ✅ Entwurf |
| AP-8 | Betrieb, Notifications, Offline-Sync (021), Frontend-Design-System (022/023), i18n (028), Deployment/Backup (030) | `docs/adr/` | offen |
| AP-9 | Spikes (Wegwerfcode, nicht im Produkt): Speicherbudget Go-Backend, LiteDB-Leser in Go, River/sqlc | `docs/phase-2/40-spikes.md` | ✅ durchgeführt (S-6 Android offen) |
| AP-10 | KI: Anbieter-Abstraktion (024), RAG/pgvector (025), Werkzeugschicht (026) | `docs/adr/` | offen |
| AP-11 | Meldetext an LubeLogger-Maintainer (E-10) | `90-entwurf-meldung-lubelogger.md` | ✅ Entwurf |

Aufwandsschätzung unverändert: 36–54 Personentage. Durch die klaren Workshop-Entscheidungen und den reduzierten MVP-Umfang (E-5) eher am unteren Rand.

## Arbeitsregeln Phase 2

- Jede Abweichung vom Ist-Verhalten von LubeLogger bekommt eine Referenz auf die Phase-1-Regel (BR-xxx) und eine Begründung (ADR-031).
- Spezifikationen beschreiben Verhalten mit eigenen Begriffen, Formeln und Beispielen, nie mit übernommenem Code (ADR-000).
- Vectra-Begriffe sind Deutsch in der UI und Englisch in API und Code (z. B. `odometer_reading`, `fuel_fill`).
