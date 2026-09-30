# 10 – Domänenmodell Notes (Notizen)

- **Status:** Entwurf (AP-4) · **Datum:** 2026-09-30
- **Grundlagen:** ADR-011, ADR-016; Entscheidung E-5 (zurückgestellte Funktionen als Notizen)

## 1. Zweck

Freie Notizen zu einem Fahrzeug (z. B. „Reifengröße 205/55 R16 91V“, „Werkstatt-Ansprechpartner“). Das Modul nimmt außerdem beim Import Daten zurückgestellter LubeLogger-Funktionen auf (Aufgabenplaner, Teilelager, Inspektionen, Ausstattung), damit nichts verloren geht (E-5, ADR-027).

## 2. Datenmodell

| Feld | Regel |
|---|---|
| `id`, `vehicle_id`, `version`, Standardfelder | Übersicht §4 |
| `title` | Pflicht, 1–200 Zeichen |
| `body` | Markdown-Teilmenge (Überschriften, Listen, Hervorhebung, Links, Tabellen); max. 50 000 Zeichen |
| `pinned` | angeheftet (oben in der Liste, im Dashboard) |
| `tags` | wie Übersicht §4 |
| `origin_detail` | bei Import: Herkunftsfunktion, z. B. `lubelogger:planner` |
| Anhänge | über Verknüpfungen (`target_type = note`, Documents) |

## 3. Regeln

- **NO-01 – Sichere Darstellung:** Markdown wird serverseitig **nicht** zu HTML gerendert. Der Client rendert es mit einer Bibliothek, die rohes HTML verwirft. Links bekommen `rel="noopener noreferrer"`, nur `http(s)` und `mailto` sind erlaubt.
- **NO-02 – Anheften** ist eine normale Änderung per `PATCH` (If-Match), nie ein Nebeneffekt eines Lesezugriffs.
- **NO-03 – Importierte Notizen** enthalten eine lesbare Zusammenfassung und hängen die Originaldaten als JSON-Datei an (Documents, Typ `other`). Der Import legt sie als nicht angeheftet an und versieht sie mit dem Schlagwort `import`.
- **NO-04 – Suche:** Volltext über Titel, Text und Schlagwörter (wie DO-03).

## 4. Operationen

`CreateNote`, `UpdateNote`, `DeleteNote` (Bearbeiter), `ListNotes`, `SearchNotes` (Leser).

## 5. Soll-Beispiele

| # | Situation | Erwartung |
|---|---|---|
| N-1 | Notiz mit `<script>` im Text | gespeichert wie eingegeben; Anzeige als Text, nicht ausgeführt |
| N-2 | Link `javascript:…` | als Text angezeigt, nicht klickbar |
| N-3 | Leser heftet Notiz an | `403` |
| N-4 | Import eines LubeLogger-Planereintrags | Notiz „Planer: Bremsen hinten“ mit Status und Priorität im Text, JSON-Anhang, Schlagwort `import` |

## 6. Abgleich mit Phase 1 (vorläufig)

| Phase 1 | Verhalten LubeLogger (Kurzform) | Vectra | Klasse |
|---|---|---|---|
| T-06 (Notizen) | Anheften über einen schreibenden GET-Endpunkt | NO-02 | FIX |
| F-020 | Notizen je Fahrzeug mit Anheften | Notes | KEEP |
