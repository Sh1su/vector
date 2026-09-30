# ADR-030 – Deployment, Backup und Betrieb

- **Status:** akzeptiert · **Datum:** 2026-09-30 · **Bezug Entscheidung:** E-14 (Budgets, Bestätigung ausstehend)

## Kontext
Vectra wird selbst gehostet: Docker Compose mit Caddy (Auftrag 5.2), Basis-Stack auf 1 vCPU / 1 GB (5.3). In der Altanwendung enthielt das Backup bei PostgreSQL keine Fachdaten, Backups wurden mit Geheimnissen per E-Mail verschickt, und zur Laufzeit gab es Abhängigkeiten zu GitHub (Phase 1, D-16, R-03, T-11).

## Entscheidung

### Stack (Docker Compose)
| Dienst | Pflicht | Aufgabe |
|---|---|---|
| `caddy` | ja | TLS (automatisch), HSTS, Auslieferung der statischen Web-App, Reverse Proxy für `/api` |
| `vectra` | ja | Go-Binary: API, Job-Worker, Scheduler (ADR-001, ADR-019); Image `FROM scratch`, Nutzer ohne Rechte |
| `postgres` | ja | PostgreSQL 16 (Image mit pgvector, damit das optionale Assistant-Modul ohne Wechsel aktivierbar ist) |
| `ntfy` | optional | UnifiedPush-Server (ADR-020) |
| `ollama` | optional | lokale KI (ADR-024), nicht im 1-GB-Budget |
| `clamav` | optional | Virenscan von Uploads (ADR-017) |

- Konfiguration über Umgebungsvariablen (`VECTRA_…`) und optional eine Datei; Geheimnisse über Docker Secrets oder `.env` mit Rechten 600. Die Datenbank wird nicht nach außen veröffentlicht.
- Container laufen ohne Root, mit schreibgeschütztem Dateisystem (außer Datenvolumes) und ohne zusätzliche Capabilities.
- PostgreSQL-Vorgaben für kleine Hosts: `shared_buffers` 128 MB, `max_connections` 30, Autovacuum aktiv.

### Präzisierung Phase 3 (2026-09-30)
Die statische Web-App liegt im selben Image wie das Backend und wird vom Backend ausgeliefert (`VECTRA_WEB_DIR`, lange Cache-Zeiten für `/assets/`). Caddy übernimmt nur TLS, HSTS, Kompression und das Weiterleiten. So bleibt es bei **einem** Anwendungs-Image, und Web und API haben immer dieselbe Version. Messwerte Iteration 1: Binary 13,9 MB, Image 5,5 MB komprimiert, 22,8 MiB Speicher im Container (schreibgeschützt, ohne Root, ohne Capabilities).

### Budgets (Messungen aus Spike S-1/S-5)
| Ziel (Auftrag 5.3) | Messwert Spike | CI-Prüfung ab Phase 3 |
|---|---|---|
| Backend im Leerlauf < 50 MB RAM | 14 MB (Spitze unter Last 24 MB) | Start im Testcontainer, RSS nach 60 s |
| Backend-Image < 50 MB | 17,3 MB | Image-Größe |
| initiales JS < 300 KB gzip | 144 KB (Gerüst + UI) | Bundle-Analyse |
| Basis-Stack auf 1 vCPU / 1 GB | Backend + PostgreSQL ≈ 70–80 MB im Leerlauf | Compose-Test mit Ressourcengrenzen |

Die Messwerte stützen die Zielwerte. Ob sie als verbindliche Abnahmekriterien gelten, entscheidet der Auftraggeber (E-14).

### Backup und Wiederherstellung
- Befehl `vectra backup` (auch als periodischer Job konfigurierbar):
  1. `pg_dump` im Custom-Format;
  2. Kopie des Storage (lokal: tar; S3: Synchronisation in ein Backup-Ziel);
  3. **Manifest** mit Vectra-Version, Schema-Version, Zeitstempel und SHA-256 aller Teile.
- **Konsistenz:** Dateien sind unveränderlich, und ihr Löschen ist um die Aufbewahrungsfrist verzögert (ADR-017, DO-06). Deshalb sind alle Dateien, auf die der Dump verweist, beim anschließenden Kopieren noch vorhanden, solange die Frist länger ist als die Backup-Dauer.
- Optional wird das Backup verschlüsselt (age, öffentlicher Schlüssel in der Konfiguration). Backups werden **nie** per E-Mail verschickt und liegen nie in einem Web-Pfad.
- Aufbewahrung konfigurierbar (Default: 7 tägliche, 4 wöchentliche Backups).
- `vectra restore` prüft Manifest, Hashes und Versionskompatibilität, bevor etwas überschrieben wird.
- Ein monatlicher Job prüft, ob das letzte Backup lesbar ist (`pg_restore --list`, Hash-Prüfung).
- Vor jedem Update mit Datenmigration legt `vectra` automatisch ein Backup an (ADR-005).

### Updates
Neues Image ziehen und neu starten. Migrationen laufen beim Start unter Advisory-Lock (ADR-005). Die Release-Notes nennen Migrationen mit Datenumbau ausdrücklich.

### Beobachtbarkeit
- Logs: JSON (`slog`) mit `request_id`, ohne personenbezogene Inhalte und ohne Geheimnisse.
- Metriken: Prometheus-Format auf einem internen Port (nicht über Caddy veröffentlicht): Requests, Latenzen, Job-Queue, fehlgeschlagene Jobs, Speicher.
- Health: `GET /api/v1/health` (öffentlich nur Gesamtstatus, Details für Admins).
- **Keine** Telemetrie und keine Aufrufe externer Dienste ohne Konfiguration. Die Prüfung auf neue Versionen ist opt-in (korrigiert T-11).

## Konsequenzen
- (+) Vollständige, prüfbare Backups inklusive Dateien (korrigiert D-16); kleiner, sicherer Standard-Stack.
- (−) Betreiber müssen ein Backup-Ziel konfigurieren. Ohne Ziel zeigt die Admin-Oberfläche eine dauerhafte Warnung.

## Bezug
Auftrag 5.2, 5.3; Phase 1 D-16, R-03, T-08, T-11; ADR-005, ADR-017, ADR-019, ADR-020; Spikes S-1, S-5.
