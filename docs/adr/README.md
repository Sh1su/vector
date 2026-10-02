# Architecture Decision Records (Vectra)

Jede bewusste Architektur- oder Fachentscheidung, insbesondere jede Abweichung vom Verhalten der Altanwendung LubeLogger, wird hier als ADR festgehalten.

**Format:** `ADR-NNN-kurzname.md` mit den Abschnitten Status, Kontext, Optionen, Entscheidung, Konsequenzen, Bezug.
**Status:** `vorgeschlagen` → `akzeptiert` → ggf. `ersetzt durch ADR-XXX`.

| ADR | Titel | Status |
|---|---|---|
| [000](ADR-000-clean-room-rebrand.md) | Clean-Room-Neuentwicklung als Rebrand | akzeptiert |
| [001](ADR-001-modularer-monolith.md) | Modularer Monolith mit fachlichen Modulen | akzeptiert |
| [003](ADR-003-http-framework.md) | HTTP-Schicht: chi auf net/http | akzeptiert |
| [004](ADR-004-datenzugriff.md) | Datenzugriff: PostgreSQL mit pgx und sqlc | akzeptiert |
| [005](ADR-005-migrationswerkzeug.md) | Schema-Migrationen mit goose | akzeptiert |
| [006](ADR-006-ids-uuidv7.md) | IDs: UUIDv7, clientseitig erzeugbar | akzeptiert |
| [007](ADR-007-einheitenmodell.md) | Einheitenmodell: SI intern + Originalwert | akzeptiert |
| [008](ADR-008-zeitmodell.md) | Zeitmodell: Zeitstempel mit Zeitzone | akzeptiert |
| [009](ADR-009-kilometerstand-modell.md) | Kilometerstand als eigene Messreihe mit Herkunft und Tachotausch | akzeptiert |
| [010](ADR-010-plausibilitaet.md) | Plausibilitätsprüfung: Warnen und bestätigen | akzeptiert |
| [011](ADR-011-audit.md) | Audit und Änderungsverfolgung | akzeptiert |
| [012](ADR-012-locking-idempotenz.md) | Optimistisches Locking und Idempotenz | akzeptiert |
| [013](ADR-013-api-konventionen.md) | API-Konventionen | akzeptiert |
| [014](ADR-014-openapi-workflow.md) | OpenAPI 3.1 spec-first | akzeptiert |
| [015](ADR-015-authentifizierung.md) | Authentifizierung: OIDC und lokale Konten | akzeptiert |
| [016](ADR-016-autorisierung.md) | Autorisierung: Rollen je Fahrzeug, zentrale Prüfung | akzeptiert |
| [017](ADR-017-object-storage.md) | Object Storage und Dateizugriff | akzeptiert |
| [018](ADR-018-beweisfotos.md) | Beweisfotos | akzeptiert |
| [019](ADR-019-background-jobs.md) | Background Jobs mit River | akzeptiert |
| [020](ADR-020-benachrichtigungen.md) | Benachrichtigungen: E-Mail, UnifiedPush, optional FCM | akzeptiert |
| [021](ADR-021-offline-sync.md) | Offline-Erfassung und Synchronisation (Android) | akzeptiert |
| [022](ADR-022-web-design-system.md) | Web-Design-System: shadcn/ui-Muster (Radix + Tailwind) | akzeptiert |
| [023](ADR-023-diagramme.md) | Diagramme: Chart.js, lazy geladen | akzeptiert |
| [024](ADR-024-ki-anbieter-datenschutz.md) | KI-Anbieter-Abstraktion und Datenschutz | akzeptiert |
| [025](ADR-025-rag-pgvector.md) | Dokumentenanalyse (RAG) mit pgvector | akzeptiert |
| [026](ADR-026-assistent-werkzeuge.md) | Assistent als Werkzeugschicht | akzeptiert |
| [027](ADR-027-migration-lubelogger.md) | Migration aus LubeLogger: Offline-Import mit Prüfbericht | akzeptiert |
| [028](ADR-028-mehrsprachigkeit.md) | Mehrsprachigkeit (Deutsch und Englisch) | akzeptiert |
| [029](ADR-029-kosten-waehrung.md) | Kosten: Währung je Betrag, Teile/Arbeit getrennt | akzeptiert |
| [030](ADR-030-deployment-betrieb.md) | Deployment, Backup und Betrieb | akzeptiert |
| [031](ADR-031-altfehler-korrigieren.md) | Fehler der Altanwendung korrigieren statt nachbauen | akzeptiert |
| [032](ADR-032-mcp-server-und-websuche.md) | MCP-Server, Claude als Chat-Anbieter, Websuche für Herstellerangaben | akzeptiert |

ADR-002 (Modulgrenzen) ist in ADR-001 aufgegangen.

Alle in Phase 1 vorgeschlagenen ADRs (001–031) liegen vor; ADR-002 ist in ADR-001 aufgegangen.
