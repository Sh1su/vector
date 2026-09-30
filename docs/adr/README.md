# Architecture Decision Records (Vectra)

Jede bewusste Architektur- oder Fachentscheidung, insbesondere jede Abweichung vom Verhalten der Altanwendung LubeLogger, wird hier als ADR festgehalten.

**Format:** `ADR-NNN-kurzname.md` mit den Abschnitten Status, Kontext, Optionen, Entscheidung, Konsequenzen, Bezug.
**Status:** `vorgeschlagen` → `akzeptiert` → ggf. `ersetzt durch ADR-XXX`.

| ADR | Titel | Status |
|---|---|---|
| [000](ADR-000-clean-room-rebrand.md) | Clean-Room-Neuentwicklung als Rebrand | akzeptiert |
| [001](ADR-001-modularer-monolith.md) | Modularer Monolith mit fachlichen Modulen | akzeptiert |
| [006](ADR-006-ids-uuidv7.md) | IDs: UUIDv7, clientseitig erzeugbar | akzeptiert |
| [007](ADR-007-einheitenmodell.md) | Einheitenmodell: SI intern + Originalwert | akzeptiert |
| [008](ADR-008-zeitmodell.md) | Zeitmodell: Zeitstempel mit Zeitzone | akzeptiert |
| [009](ADR-009-kilometerstand-modell.md) | Kilometerstand als eigene Messreihe mit Herkunft und Tachotausch | akzeptiert |
| [010](ADR-010-plausibilitaet.md) | Plausibilitätsprüfung: Warnen und bestätigen | akzeptiert |
| [015](ADR-015-authentifizierung.md) | Authentifizierung: OIDC und lokale Konten | akzeptiert |
| [016](ADR-016-autorisierung.md) | Autorisierung: Rollen je Fahrzeug, zentrale Prüfung | akzeptiert |
| [020](ADR-020-benachrichtigungen.md) | Benachrichtigungen: E-Mail, UnifiedPush, optional FCM | akzeptiert |
| [027](ADR-027-migration-lubelogger.md) | Migration aus LubeLogger: Offline-Import mit Prüfbericht | akzeptiert |
| [029](ADR-029-kosten-waehrung.md) | Kosten: Währung je Betrag, Teile/Arbeit getrennt | akzeptiert |
| [031](ADR-031-altfehler-korrigieren.md) | Fehler der Altanwendung korrigieren statt nachbauen | akzeptiert |

Geplant (AP-2 ff.): 003 HTTP-Framework, 004 Datenzugriff, 005 Migrationswerkzeug, 011 Audit, 012 Locking/Idempotenz, 013 API-Konventionen, 014 OpenAPI-Workflow, 017 Object Storage, 018 Beweisfotos, 019 Jobs, 021 Offline-Sync, 022 Web-Design-System, 023 Diagramme, 024–026 KI, 028 i18n, 030 Deployment/Backup.
