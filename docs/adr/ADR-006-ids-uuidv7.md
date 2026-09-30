# ADR-006 – IDs: UUIDv7, clientseitig erzeugbar

- **Status:** akzeptiert · **Datum:** 2026-09-30

## Kontext
Die Android-App soll offline erfassen und Einträge später idempotent synchronisieren (Auftrag 6.16). Fortlaufende Integer-IDs sind erratbar und wurden in der Altanwendung Teil einer Sicherheitslücke (Phase 1, R-01, M-1).

## Optionen
- **A: UUIDv7 für alle Entitäten, Client darf die ID vorgeben**
- B: Serverseitige Integer-IDs plus zusätzliche Client-Referenz – zwei Identitäten, komplexere Synchronisierung.
- C: UUIDv4 – zufällig, aber schlechte Index-Lokalität in PostgreSQL.

## Entscheidung
Option A. Jede Entität hat eine `id` vom Typ UUIDv7. Erstellende Requests dürfen die ID mitsenden. Existiert sie bereits mit identischem Inhalt, antwortet der Server idempotent. Bei abweichendem Inhalt antwortet er mit `409 Conflict`. Ohne mitgesendete ID erzeugt der Server eine. Zeitliche Reihenfolge wird **nie** aus der UUID abgeleitet, sondern aus fachlichen Zeitstempeln (ADR-008).

## Konsequenzen
- (+) Offline-Erstellung und idempotente Wiederholung ohne Mapping-Tabellen.
- (+) Nicht erratbar; die Autorisierung bleibt trotzdem Pflicht (ADR-016).
- (−) Beim Import von LubeLogger wird eine Zuordnung Alt-ID → UUID geführt (ADR-027).

## Bezug
Auftrag 6.16, 6.17; Phase 1 M-1, MG-6.
