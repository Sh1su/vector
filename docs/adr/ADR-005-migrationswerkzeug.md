# ADR-005 – Schema-Migrationen mit goose

- **Status:** akzeptiert · **Datum:** 2026-09-30

## Kontext
Die Altanwendung hat keine Schema-Migrationen; Tabellen werden beim Start angelegt, Strukturänderungen passieren implizit (Phase 1, T-03). Vectra braucht nachvollziehbare, versionierte Schemaänderungen für Self-Hosting-Updates.

## Optionen
- **A: goose** – SQL-Dateien plus optionale Go-Migrationen für Datenumbauten, als Bibliothek einbettbar.
- B: golang-migrate – nur SQL/Up-Down-Paare, Datenmigrationen außerhalb.

## Entscheidung
Option A.
- Migrationen liegen als SQL-Dateien im Repository und werden in das Binary eingebettet (`embed`).
- Das Backend führt ausstehende Migrationen beim Start unter einem PostgreSQL-Advisory-Lock aus (nur eine Instanz migriert). Alternativ per Befehl `vectra migrate` für Betreiber, die das trennen wollen.
- **Nur vorwärts** in Produktion: Down-Migrationen existieren für die Entwicklung, das Rückrollen im Betrieb erfolgt über das Backup (ADR-030).
- Änderungen folgen dem Muster *expand → migrate → contract* über mindestens eine Release-Version, damit ein Update ohne Datenverlust abbrechbar ist.
- Vor jeder Migration mit Datenumbau erstellt das Backend eine Warnung im Log, dass ein aktuelles Backup vorliegen muss; die Update-Doku verlangt das explizit.
- CI prüft: Migrationen laufen gegen eine leere Datenbank und gegen den Stand der Vorversion durch; sqlc-Code passt zum resultierenden Schema.

## Konsequenzen
- (+) Reproduzierbare Updates, prüfbar in der CI.
- (−) Disziplin nötig: keine nachträgliche Änderung veröffentlichter Migrationen.

## Bezug
Auftrag 5.2; Phase 1 T-03; ADR-004, ADR-030.
