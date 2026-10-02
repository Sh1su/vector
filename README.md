<p align="center">
  <img src="web/public/brand/vectra-logo-quer.png" alt="Vectra" width="320">
</p>

<p align="center"><b>Intelligentes Fahrzeugmanagement – selbst gehostet.</b><br>
Kilometerstand, Tanken und Laden, Öl, Wartung und ein optionaler KI-Assistent. Deine Daten bleiben auf deinem Server.</p>

---

## Funktionen

| Bereich | Was Vectra kann |
|---|---|
| **Kilometerstand** | Stände erfassen, Plausibilitätsprüfung (rückläufig, unrealistische Sprünge), Korrekturen mit Historie, Tachotausch |
| **Kraftstoff & Laden** | Benzin, Diesel, Autogas, Strom; Verbrauch je Tankintervall, Durchschnitt und Monatswerte, Preis pro Liter/kWh, Netz- und Batterieverbrauch bei E-Autos |
| **Öl** | Messung, Nachfüllung, Ölwechsel; Verbrauch je Messreihe und Nachfüllrate (ml bzw. l je 1.000 km) |
| **Wartung** | Intervalle nach Zeit und/oder km, Stufen „demnächst / fällig / überfällig“, **Wartungsplan-Vorlagen** (u. a. Mercedes-Benz Sprinter und Vito CDI, Reisemobil-Aufbau) zum Vorab-Erfassen |
| **Assistent (optional)** | Fragen in natürlicher Sprache, Einträge per Satz vorbereiten („bei 143.520 km 0,7 l Öl nachgefüllt“); speichert nie selbst, jeder Vorschlag wird bestätigt. Claude (Anthropic) oder lokal mit Ollama |
| **Teilen & Rechte** | Rollen je Fahrzeug (Eigentümer, Bearbeiter, Leser), vollständige Änderungshistorie |
| **API** | REST-API (OpenAPI 3.1) mit persönlichen API-Tokens – z. B. für Home Assistant, Grafana oder eigene Skripte |
| **Apps** | Web-App (hell/dunkel), Android-App mit Offline-Erfassung |

## Schnellstart (Docker Compose)

Voraussetzung: ein Linux-Server mit [Docker](https://docs.docker.com/engine/install/) und dem Compose-Plugin.

```bash
curl -fsSL https://raw.githubusercontent.com/Sh1su/vector/main/deploy/install.sh | bash
cd vectra && docker compose up -d
```

Das Skript fragt, ob Vectra **im Heimnetz** (`http://<server-ip>:8080`) oder **im Internet mit eigener Domain** (HTTPS über Let's Encrypt) laufen soll, und legt im Ordner `vectra/` an:

| Datei | Inhalt |
|---|---|
| `.env` | zufälliges Datenbankpasswort, ggf. Domain und Setup-Token, Platz für den Assistenten |
| `compose.yaml` | PostgreSQL + Vectra (+ Caddy für HTTPS im Domain-Modus) |
| `Caddyfile` | nur im Domain-Modus |

Danach die angezeigte Adresse im Browser öffnen. **Beim ersten Aufruf erscheint automatisch die Ersteinrichtung**, in der du das Administratorkonto anlegst. Sie ist nur möglich, solange noch kein Konto existiert; danach ist sie gesperrt.

- **Heimnetz:** Die Ersteinrichtung ist ohne Token möglich – also gleich nach dem Start selbst durchführen.
- **Domain/Internet:** Das Skript erzeugt ein **Setup-Token** (wird am Ende angezeigt und steht in `.env`). Ohne dieses Token kann niemand die Installation übernehmen, bevor du sie eingerichtet hast.

Ohne Rückfragen, z. B. in Automatisierungen:

```bash
curl -fsSL https://raw.githubusercontent.com/Sh1su/vector/main/deploy/install.sh \
  | VECTRA_MODE=domain VECTRA_DOMAIN=vectra.example.org bash
```

Weitere Variablen: `VECTRA_MODE=local|domain`, `VECTRA_PORT` (Standard 8080), `VECTRA_DIR` (Standard `vectra`), `VECTRA_IMAGE`, `VECTRA_SETUP_TOKEN`, `VECTRA_FORCE=1` (vorhandene Dateien überschreiben).

> **Hinweis zum Image:** Die CI veröffentlicht `ghcr.io/sh1su/vector` (amd64 und arm64). Ist das Paket auf GitHub privat, vorher anmelden: `echo <TOKEN> | docker login ghcr.io -u <github-name> --password-stdin` (Personal Access Token mit `read:packages`), oder das Paket unter *Packages → vector → Package settings* auf *Public* stellen.

### Betrieb

```bash
docker compose pull && docker compose up -d     # aktualisieren (Migrationen laufen automatisch)
docker compose logs -f vectra                   # Logs
docker compose exec postgres pg_dump -U vectra -Fc vectra > vectra.dump   # Sicherung
```

Manuelle Installation, eigener Reverse-Proxy (z. B. Nginx Proxy Manager) und die Android-App: [`deploy/README.md`](deploy/README.md).

## Konfiguration

Alle Einstellungen kommen aus der Umgebung (`.env`):

| Variable | Standard | Bedeutung |
|---|---|---|
| `VECTRA_DATABASE_URL` | – | PostgreSQL ≥ 16 (setzt `compose.yaml`) |
| `VECTRA_LISTEN` | `:8080` | Adresse des Servers im Container |
| `VECTRA_COOKIE_SECURE` | `true` | `false` nur bei Zugriff über `http://` (Heimnetz) |
| `VECTRA_SETUP_TOKEN` | leer | gesetzt: Ersteinrichtung nur mit diesem Token |
| `VECTRA_ODOMETER_VMAX_KMH` | `250` | Grenze für „unrealistischer Sprung“ |
| `VECTRA_ASSISTANT_*` | leer | KI-Assistent, siehe unten |

Einheiten, Zeitzone, Währung und Wartungsschwellen stellt jede Person in der Web-App unter **Einstellungen** ein; Admins zusätzlich die Vorgaben der Installation.

### KI-Assistent (optional)

In `.env` einen Anbieter eintragen, `docker compose up -d`, dann in der Web-App unter **Einstellungen → Installation** aktivieren:

```bash
# Claude (Anthropic) – Daten gehen an Anthropic; jede Person stimmt einmalig zu
VECTRA_ASSISTANT_PROVIDER=anthropic
VECTRA_ASSISTANT_API_KEY=sk-ant-…
# VECTRA_ASSISTANT_MODEL=claude-opus-5-5   (Standard)

# oder lokal mit Ollama (eigener Container, nicht im Basis-Stack)
VECTRA_ASSISTANT_PROVIDER=ollama
VECTRA_ASSISTANT_MODEL=qwen2.5:14b
VECTRA_ASSISTANT_BASE_URL=http://ollama:11434/v1
```

Details: [`deploy/README.md`](deploy/README.md#ki-assistent-optional).

## API

Vectra hat eine vollständige REST-API (`/api/v1`, JSON). Für Skripte legst du unter **Einstellungen → API-Tokens** ein Token mit passenden Rechten an:

```bash
curl -H "Authorization: Bearer vct_…" https://vectra.example.org/api/v1/vehicles
```

- Anleitung mit Beispielen (Verbrauch, Öl, Wartung, Änderungshistorie, Erfassen per API): [`docs/api.md`](docs/api.md)
- Maschinenlesbare Spezifikation: `GET /api/v1/openapi.json` bzw. [`api/openapi.yaml`](api/openapi.yaml)

## Entwicklung

| Teil | Technik | Start |
|---|---|---|
| Backend | Go 1.26, PostgreSQL 16, sqlc, goose, oapi-codegen | `cd backend && VECTRA_DATABASE_URL=… go run ./cmd/vectra` |
| Web | React 19, Vite, Tailwind 4, TanStack Query | `cd web && npm ci && npm run dev` (Proxy auf `:8080`) |
| Android | Kotlin, Jetpack Compose | `cd android && ./gradlew :app:assembleDebug` |

```bash
cd backend
go generate ./internal/api/...                                   # Code aus api/openapi.yaml
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0 generate          # Datenbankzugriff
VECTRA_TEST_DATABASE_URL=postgres://… go test -p 1 ./...           # Tests inkl. Integration
cd ../web && npm run gen:api && npm run build                      # Web mit Bundle-Budget
```

Die API ist **spec-first**: Änderungen beginnen in `api/openapi.yaml`. Architekturentscheidungen stehen in [`docs/adr/`](docs/adr), die Fachspezifikation in [`docs/phase-2/`](docs/phase-2), der Projektstand in [`docs/phase-3/`](docs/phase-3).

## Lizenz

Bis zur Festlegung: alle Rechte vorbehalten.
