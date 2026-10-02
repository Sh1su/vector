# Phase 3 – Umsetzung: Plan und Iterationen

- **Status:** gestartet · **Datum:** 2026-09-30 · **Freigabe:** Auftraggeber („mach weiter … wenn du nun in Phase 3 startest“)
- **Grundlagen:** alle Dokumente aus Phase 2; Design-Vorlage „Vectra Brand“ (Leinwand mit Markengrundlagen, 17 Web- und 17 App-Entwürfen, Icon-Set), übernommen in `01-design-system.md`.
- **Leitlinie:** ADR-000. Kein LubeLogger-Code; jede Implementierung folgt den eigenen Spezifikationen (`docs/phase-2/10-domaene-*.md`, `20-regelkatalog-soll.md`, `api/openapi.yaml`).

## Annahmen zu offenen Entscheidungen (Phase 2, `99-abschluss.md` §3)

Bis zu einer anderen Entscheidung des Auftraggebers gelten die dort empfohlenen Werte:

| # | Annahme |
|---|---|
| 1 | Ressourcenbudgets sind Abnahmekriterien und werden in der CI geprüft |
| 2 | Wartungsschwellen: „demnächst fällig“ ab 1 500 km, „fällig“ ab 500 km |
| 3 | Kostenansicht Standard: Zahlungsdatum |
| 4 | Einladungen 7 Tage gültig |
| 5 | Ölstand ohne Peilstab: Prozent der Anzeige |
| 8 | Lizenz: bis zur Entscheidung „alle Rechte vorbehalten“ (kein Lizenztext im Repository) |

## Iterationen

| It. | Inhalt | Ergebnis |
|---|---|---|
| **1** | Fundament + erster vertikaler Schnitt: Repository-Gerüst, Codegenerierung, Migrationen, Identity-Kern (Ersteinrichtung, lokale Anmeldung, Sitzung, CSRF, Rechteprüfung), Vehicles, Odometer (alle ODO-Regeln), Web-App im Vectra-Design (Anmeldung, Übersicht, Fahrzeuge, Kilometerstand, Hell/Dunkel), Compose-Deployment, CI | lauffähiges System: Fahrzeug anlegen, Kilometerstände erfassen mit Plausibilitätsdialog |
| 2 | Fuel, Oil, ServiceHistory, Maintenance, Costs (Kostenbuch) mit allen Soll-Tests; Web-Seiten Kraftstoff, Öl, Servicehistorie, Wartung, Kosten. **Teil 1 umgesetzt** (Service, Wartung, Kosten, dazu vorgezogen Fahrten), siehe `20-iteration-2.md` | Fachkern vollständig |
| 3 | ~~Trips~~ (vorgezogen), Documents (Storage, Beweisfotos), Notes, Audit-Ansicht, Freigaben/Einladungen, OIDC | alle MVP-Module |
| 4 | Notifications (E-Mail, UnifiedPush, FCM optional, Webhooks), Jobs, Backup/Restore, i18n EN | betriebsbereit |
| 5 | Import LubeLogger (LiteDB-Leser), CSV-Import, Export | Migration |
| 6 | Android: Tachofoto mit Hash, UnifiedPush, Fahrten, Kraftstoff und Öl mobil (die App-Basis entsteht auf Wunsch des Auftraggebers ab Iteration 1 parallel, siehe `10-android.md`) | App |
| 7 | Assistant (optional) | KI |

Jede Iteration endet mit grüner CI, aktualisierten Tests und einem kurzen Bericht in diesem Ordner.

## Iteration 1 – Umfang im Detail

- **Backend** (`backend/`): Go-Modul, `cmd/vectra`, Pakete `internal/platform` (Konfiguration, Datenbank, HTTP, Problem Details, Sitzungen, CSRF), `internal/kernel` (Einheiten, Zeit), `internal/identity`, `internal/vehicles`, `internal/odometer`; goose-Migrationen eingebettet; sqlc; oapi-codegen (strict server) aus `api/openapi.yaml`. Nicht umgesetzte Operationen antworten mit `501 Not Implemented` im Problem-Details-Format.
- **Web** (`web/`): Vite, React 19, TypeScript, Tailwind 4 mit Vectra-Tokens, React Router, TanStack Query, Typen aus der OpenAPI.
- **Android** (`android/`, parallel auf Wunsch des Auftraggebers): Kernmodul (API-Client, Outbox nach ADR-021, Lesecache) und Compose-App mit Anmeldung, Übersicht, Kilometerstand (offline erfassen, Befunde bestätigen), Fahrzeuge, Mehr, Einstellungen. Details in `10-android.md`.
- **Betrieb** (`deploy/`): Compose mit PostgreSQL, Vectra, Caddy.
- **CI** (`.github/workflows/ci.yml`): Go-Tests (inkl. PostgreSQL-Dienst), Codegen-Abgleich, Lint der Spezifikation, Web-Build mit Bundle-Budget.
