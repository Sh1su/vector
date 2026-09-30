# ADR-021 – Offline-Erfassung und Synchronisation (Android)

- **Status:** akzeptiert · **Datum:** 2026-09-30

## Kontext
Die Android-App soll Kilometerstände, Tankvorgänge, Ölstände und Fahrten auch ohne Netz erfassen und später synchronisieren. Konflikte werden dem Nutzer angezeigt, nie still überschrieben (Auftrag 6.16). Bausteine liegen bereits vor: client-erzeugte UUIDv7 (ADR-006), ETag/If-Match und Idempotency-Key (ADR-012), Plausibilitätsbestätigung per `422` (ADR-010) und der Änderungsfeed `GET /sync/changes` (OpenAPI).

## Optionen
- **A: Outbox + Idempotenz + ETag-Konflikte**, der Server bleibt maßgeblich.
- B: CRDT bzw. automatische Zusammenführung – passt nicht zu Messwerten mit Plausibilitätsregeln und Audit-Pflicht.
- C: nur online – verfehlt den Auftrag.

## Entscheidung
Option A.

**Lokaler Speicher (Room):**
- Lesecache für die Fahrzeuge des Kontos, deren Stammdaten, Wartungsstatus und die Einträge der letzten 12 Monate.
- **Outbox**-Tabelle: je ausstehender Änderung Operation (OpenAPI-`operationId`), Ziel-ID, Request-Body, `base_version` (ETag beim Lesen), `Idempotency-Key`, Erfassungszeit, Status (`pending`, `sending`, `needs_confirmation`, `conflict`, `failed`).

**Senden (WorkManager):**
- Eindeutiger Job „sync“ mit Netz-Bedingung und exponentiellem Backoff. Er startet nach jeder Erfassung und periodisch (15 min, wenn Einträge ausstehen).
- Reihenfolge: FIFO je Fahrzeug. Dateien (Fotos) gehen **vor** dem Eintrag, der sie verknüpft. Eine Datei hat eine eigene UUID, der Upload ist idempotent.
- Anlegen mit client-erzeugter UUID. Eine Wiederholung nach Netzabbruch liefert `200` statt eines Duplikats (ADR-006).

**Antworten des Servers:**
| Antwort | Verhalten der App |
|---|---|
| `2xx` | Outbox-Eintrag entfernen, Cache mit Serverantwort aktualisieren |
| `422` mit bestätigbaren Befunden | Status `needs_confirmation`, Benachrichtigung „1 Eintrag braucht Bestätigung“; Dialog mit den Befunden, danach erneut senden mit `confirm_anomalies` und Begründung |
| `422` nicht bestätigbar | Status `failed`, Eintrag zum Bearbeiten öffnen |
| `412` | Status `conflict`; Konfliktdialog mit eigener und Server-Fassung (aus `Problem.current`): „meine übernehmen“ (Patch mit neuem ETag erneut senden), „Server übernehmen“, „bearbeiten“ |
| `404`/`403` (z. B. Freigabe entzogen) | Status `failed`; Daten bleiben lokal sichtbar und lassen sich teilen oder kopieren, werden aber nicht gesendet |
| `5xx`, Netzfehler | Wiederholung mit Backoff |

**Empfangen:**
- `GET /sync/changes?cursor=…` liefert Änderungen über alle Fahrzeuge des Kontos, einschließlich Löschungen (Tombstones) und entzogener Mitgliedschaften. Die App lädt geänderte Objekte gezielt nach.
- Tombstones bewahrt der Server 90 Tage auf. Ist der Cursor älter, antwortet er mit `410`, und die App lädt vollständig neu.

**Zeit und Uhren:**
- `occurred_at` und `time_zone` stammen vom Gerät zum Zeitpunkt der Erfassung. `recorded_at` setzt der Server beim Eingang (ADR-008).
- Beim Sync vergleicht die App ihre Uhr mit dem `Date`-Header des Servers. Bei einer Abweichung über 2 Minuten warnt sie, weil der Server Zeitpunkte mehr als 5 Minuten in der Zukunft ablehnt (ODO-02).

**Beweisfotos:** Die App berechnet den SHA-256 bei der Aufnahme und sendet ihn mit (`client_sha256`). Der Server vergleicht ihn mit seinem eigenen Hash. Weichen die Werte ab, wird der Upload abgelehnt, weil die Datei unterwegs verändert wurde. Maßgeblich bleibt der Server-Hash (ADR-018).

**Web-App:** im MVP nur online. Offline-Unterstützung für das Web ist nicht geplant.

## Konsequenzen
- (+) Keine Duplikate, keine stillen Überschreibungen; dieselben Regeln wie online.
- (+) Plausibilität bleibt allein auf dem Server, sodass sie nicht in Kotlin nachgebaut werden muss.
- (−) Befunde zeigt die App erst nach dem Sync. Nutzer können Einträge also offline speichern, die später bestätigt werden müssen. Die App zeigt deshalb den letzten bekannten Stand als Orientierung beim Erfassen an.
- `client_sha256` (Datei-Upload) und `410` (Änderungsfeed) sind in `api/openapi.yaml` ergänzt.

## Bezug
Auftrag 6.16; ADR-006, ADR-008, ADR-010, ADR-012, ADR-018; OpenAPI `getChangeFeed`.
