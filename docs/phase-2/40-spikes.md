# 40 – Spikes (technische Machbarkeit)

- **Status:** durchgeführt (AP-9) · **Datum:** 2026-09-30
- **Regel:** Spike-Code ist Wegwerfcode. Er liegt **nicht** im Repository, sondern wurde in einer temporären Arbeitsumgebung ausgeführt. Dokumentiert sind Aufbau, Messmethode und Ergebnis, damit sich die Messungen wiederholen lassen.
- **Umgebung:** Linux-Container, 4 vCPU (bei den Budget-Messungen per Docker auf 1 CPU begrenzt), Go 1.24.7 bzw. 1.26.8 (automatisch geladen, siehe S-2), Node 22, Docker 29, PostgreSQL 16 (Alpine).

## Übersicht

| Spike | Frage | Ergebnis | Folge |
|---|---|---|---|
| S-1 | Hält ein Go-Backend das Budget (< 50 MB RAM im Leerlauf, Image < 50 MB)? | **ja**: 14 MB RSS im Leerlauf, 24 MB Spitze unter Last, Image 17,3 MB | Budget realistisch (E-14) |
| S-2 | Erzeugt ein Generator Go-Code aus unserer OpenAPI-3.1-Datei? | **ja**, `oapi-codegen` v2.8.0 und `ogen` v1.24.0, nach einer Anpassung der Spezifikation | ADR-014 bestätigt, Spezifikationsregel ergänzt |
| S-3 | Funktionieren sqlc, goose und River zusammen mit pgx? | **ja** | ADR-004, ADR-005, ADR-019 bestätigt |
| S-4 | Lässt sich die LubeLogger-Datenbank (LiteDB) in Go lesen? | **ja**, eigener Leser mit ca. 250 Zeilen, exakte Ergebnisse | Option A im Migrationskonzept (§9) |
| S-5 | Welches Web-Design-System und welche Diagramm-Bibliothek halten das Bundle-Budget? | shadcn/ui (Radix + Tailwind) 144 KB, Mantine 172 KB; Chart.js 52 KB, uPlot 22 KB (JS gzip) | ADR-022, ADR-023 |
| S-6 | Android-Grundgerüst (Compose, WorkManager, Keystore) | **nicht durchgeführt**: kein Android-SDK in der Umgebung | früh in Phase 3 nachholen |

## S-1 – Speicher- und Größenbudget Backend

**Aufbau:** ein Go-Programm mit chi (Router), pgx v5 (Pool, max. 8 Verbindungen), sqlc-generierten Queries, goose (eingebettete Migration), River (Queue, 2 Worker, periodischer Job alle 10 s) und slog (JSON-Logs). Endpunkte: Health, Tankvorgänge auflisten (Limit 50) und anlegen. Statisch gebaut mit `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w"`, Image `FROM scratch` + CA-Zertifikate, Nutzer 65532.

**Last:** 20 000 Requests, 20 parallel, 80 % Lesen (Liste mit 50 Einträgen), 20 % Schreiben.

| Messung | Wert |
|---|---|
| Binary | 12,3 MB |
| Container-Image (unkomprimiert / komprimiert) | 17,3 MB / 4,7 MB |
| RSS nach 60 s Leerlauf | 14,1 MB |
| RSS-Spitze unter Last (VmHWM) | 23,9 MB |
| RSS 150 s nach Last | 20,7 MB |
| Durchsatz ohne CPU-Grenze | ca. 3 080 Requests/s, 0 Fehler |
| Durchsatz im Container mit `--cpus=1 --memory=64m` | ca. 2 250 Requests/s, 0 Fehler; cgroup-Speicher 3,5 MiB Leerlauf / 9,5 MiB nach Last |
| River periodischer Job | lief wie konfiguriert (10 s-Takt) |
| PostgreSQL 16 im Leerlauf (Standardkonfiguration) | ca. 54 MiB |

**Bewertung:** Das Backend liegt deutlich unter den Zielwerten (Auftrag 5.3). Für den Basis-Stack aus PostgreSQL, Backend und Caddy auf 1 vCPU / 1 GB bleibt reichlich Reserve. Offen ist der Speicherbedarf mit echten Fachmodulen und Vorschaubild-Erzeugung; beides wird in Phase 3 als Messpunkt in der CI geführt (ADR-030).

## S-2 – Codegenerierung aus OpenAPI 3.1

**Aufbau:** `api/openapi.yaml` (152 Operationen) mit
- `oapi-codegen` v2.8.0 (`chi-server`, `strict-server`, `models`), Option `output-options.nullable-type: true`;
- `ogen` v1.24.0.

**Ergebnisse:**
1. Beide Generatoren lehnten zunächst ein Konstrukt ab: `type: [string, number, boolean, null]` (Mehrfachtyp bei `custom_fields`). Die Spezifikation wurde angepasst (Zusatzfeldwerte sind Text, `null` erlaubt). Danach haben **beide** vollständig generiert.
2. `oapi-codegen`: 1,25 MB generierter Code, **kompiliert**; `StrictServerInterface` mit allen 152 Operationen. Mit `nullable-type: true` werden `type: [T, "null"]`-Felder zu `nullable.Nullable[T]`. Damit sind bei JSON Merge Patch die drei Zustände **fehlt / null / Wert** unterscheidbar (ADR-013). Ohne die Option würden `null` und „fehlt“ zusammenfallen.
3. `ogen`: 20 Dateien, alle 152 Operationen; eigener Router (als `http.Handler` unter chi einhängbar).
4. `oapi-codegen` v2.8.0 verlangt **Go ≥ 1.25**.

**Folgen:**
- ADR-014 bleibt bei `oapi-codegen` (strict server, passt zu chi, ADR-003). Ergänzte Regeln: `nullable-type: true` ist Pflicht, und die Spezifikation verwendet **keine Mehrfachtypen außer `[T, "null"]`**. Die CI prüft beides.
- Go-Version des Projekts: mindestens 1.25 (Festlegung in Phase 3).

## S-3 – sqlc, goose, River

- `sqlc` v1.29.0 erzeugte aus goose-Migrationen und annotierten Queries typsicheren pgx-v5-Code (`sql_package: pgx/v5`). Die Migrationen dienen dabei direkt als Schemaquelle, eine Doppelpflege ist nicht nötig.
- goose v3.28 lief über `stdlib.OpenDBFromPool` auf demselben pgx-Pool; River v0.47 migrierte seine Tabellen über `rivermigrate` und arbeitete periodische Jobs ab.
- Beobachtung: goose zieht Treiber-Abhängigkeiten anderer Datenbanken in das Modul; Größe und Speicher bleiben trotzdem im Rahmen (S-1).

## S-4 – LiteDB-Datei in Go lesen

**Aufbau:** Eine LubeLogger-v1.7.3-Instanz (offizielles Container-Image, Zeitzone `Europe/Berlin`) wurde über ihre API befüllt: 2 Fahrzeuge, 304 Tankvorgänge, Kilometerstand, Service, Reminder, 2 Notizen (davon eine mit 60 KB). Danach wurde ein Tankvorgang gelöscht und die Instanz regulär beendet. Ein schreibgeschützter Go-Leser (ca. 250 Zeilen, nur Standardbibliothek) liest das Dateiformat LiteDB v5. Das Formatwissen stammt aus der öffentlichen LiteDB-Quelle (MIT-Lizenz) und ist **kein** LubeLogger-Code (ADR-000).

**Format in Kürze** (für die Implementierung in Phase 3):
- Seiten zu 8 192 Byte mit 32-Byte-Kopf (Seitentyp, Collection-ID, Anzahl Einträge, höchster Slot).
- Seite 0: Kopfzeile „This is a LiteDB file“, Dateiversion 8, ab Byte 192 ein BSON-Dokument *Collection-Name → Seiten-ID*.
- Datenseiten (Typ 4): Slot-Tabelle am Seitenende (je 4 Byte Länge/Position). Ein Dokument besteht aus Datenblöcken (1 Byte „Fortsetzung“, 5 Byte Adresse des nächsten Blocks, Nutzdaten), die über Seiten verkettet sind.
- BSON mit LiteDB-Besonderheit: Typ `0x13` ist ein .NET-`System.Decimal` (96-Bit-Ganzzahl + Skalierung), **nicht** IEEE decimal128. Datumswerte sind Millisekunden seit 1970 in UTC.

**Ergebnisse:**
| Prüfung | Ergebnis |
|---|---|
| Collections und Anzahlen | alle 6 Collections, 310 Dokumente, Anzahlen stimmen mit der API überein |
| Dezimalwerte | exakt (`71.96`, `432.5`), ohne Rundungsfehler |
| Unicode | korrekt (`Škoda`, `Äpfel & Öl – Unicode`) |
| Dokument über mehrere Seiten (60 KB) | identisch mit dem gesendeten Text (LubeLogger entfernt nur den abschließenden Zeilenumbruch) |
| gelöschter Datensatz | nicht mehr enthalten |
| Datum | `2026-01-01` (Berlin) steht als `2025-12-31T23:00:00Z` in der Datei → Umrechnung mit Quell-Zeitzone nötig (bestätigt MG-2) |
| Enums | als **Text** gespeichert (`"Metric": "Date"`, `"ReminderMileageInterval": "FiveThousandMiles"`) |

**Einschränkungen und Regeln für den Import:**
- Die Begleitdatei `cartracker-log.db` (Write-Ahead-Log) muss leer sein. Das ist nach einem regulären Beenden von LubeLogger der Fall. Ist sie nicht leer, lehnt die Analyse mit einem klaren Hinweis ab („LubeLogger vor dem Export beenden“). Ein Leser für das Log ist nicht geplant.
- Passwortgeschützte LiteDB-Dateien werden nicht unterstützt (LubeLogger nutzt keine).
- Korrektur an Phase 1: **MG-13** („Enums als Integer“, dort nur abgeleitet) ist widerlegt. Die Enums sind Namen als Text; das Mapping im Migrationskonzept verwendet deshalb Namen. In `docs/phase-1/08-import-export-und-migration.md` als Erratum vermerkt.

**Entscheidung:** Option A aus `30-migrationskonzept.md` §9 (eigener Go-Leser). Die Hilfscontainer-Variante B ist damit überflüssig.

## S-5 – Web-Bundle-Größen

**Aufbau:** Vite 8, React 19, React Router 7, TanStack Query 5. Je Variante eine gleichwertige Seite: Kopfzeile, Tabs, Tabelle mit Badge, Dialog mit Zahlen-, Text- und Auswahlfeld, Buttons. Gemessen wurde die Summe aller JS- bzw. CSS-Dateien, gzip Stufe 9.

| Variante | JS gzip | CSS gzip |
|---|---|---|
| Grundgerüst ohne UI-Bibliothek (React, Router, Query) | 104,2 KB | – |
| + Mantine 9.6 | 171,8 KB | 33,0 KB |
| + shadcn/ui-Muster (Radix Dialog/Select/Tabs, Tailwind 4, cva, tailwind-merge) | 143,8 KB | 3,0 KB |

| Diagramm (Liniendiagramm mit Achsen und Tooltip, separat gebaut) | JS gzip |
|---|---|
| Recharts 3.10 (inkl. React) | 164,5 KB |
| ECharts 6.1 (modular, SVG-Renderer) | 165,1 KB |
| Chart.js 4.5 (nur registrierte Bausteine) | 52,0 KB |
| uPlot 1.6 | 21,9 KB |

**Bewertung:** Beide UI-Varianten halten das Budget von 300 KB für das initiale JS (Auftrag 5.3). shadcn/ui ist um 28 KB JS und 30 KB CSS kleiner, und die Komponenten liegen als eigener Quelltext im Projekt, lassen sich also ohne Umwege an die Vectra-Marke anpassen. Diagramme gehören nicht ins initiale Bundle. Chart.js deckt Linien, Balken und gestapelte Balken ab und bleibt bei 52 KB. uPlot ist kleiner, bietet aber nur Zeitreihen.

## S-6 – Android (nicht durchgeführt)

In der Umgebung gibt es kein Android-SDK und keinen Emulator. Folgende Fragen bleiben für den Beginn von Phase 3:
- Größe der App mit Compose, Retrofit/OkHttp, kotlinx.serialization und WorkManager;
- Outbox-Synchronisation mit ETag-Konflikten (ADR-021) gegen den generierten Kotlin-Client (ADR-014);
- Kamera-Aufnahme mit Hash vor dem Upload und EXIF-Behandlung (ADR-018);
- UnifiedPush-Registrierung mit ntfy (ADR-020).

## Wiederholen

Die Spikes lassen sich ohne Spike-Code aus dieser Beschreibung nachbauen. Für Phase 3 werden S-1 (Speicher- und Image-Größe) und S-5 (Bundle-Größe) als **automatische Budget-Prüfungen in der CI** übernommen (ADR-030).
