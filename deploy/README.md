# Vectra betreiben

## Server (Docker)

Die CI veröffentlicht bei jedem Push ein Image für amd64 und arm64 unter `ghcr.io/sh1su/vector`:

| Tag | Inhalt |
|---|---|
| `latest` | letzter Stand des Standard-Branches |
| `sha-<commit>` | genau dieser Commit |
| `1.2.3` | Release-Tag `v1.2.3` |

**Am einfachsten mit dem Installationsskript** (erzeugt `.env`, `compose.yaml` und ggf. `Caddyfile`):

```bash
curl -fsSL https://raw.githubusercontent.com/Sh1su/vector/main/deploy/install.sh | bash
cd vectra && docker compose up -d
```

Manuell:

```bash
mkdir vectra && cd vectra
# compose.yaml, Caddyfile und .env.example aus deploy/ auf den Server kopieren
cp .env.example .env    # Domain und Datenbankpasswort eintragen
docker compose pull
docker compose up -d
```

Danach `https://<VECTRA_DOMAIN>` öffnen: Solange noch kein Konto existiert, leitet Vectra automatisch zur Ersteinrichtung des Administratorkontos. Ist `VECTRA_SETUP_TOKEN` gesetzt, wird dieses Token abgefragt (für Server im Internet empfohlen); ohne Token darf die erste Person die Einrichtung durchführen. Sobald ein Konto existiert, ist die Ersteinrichtung gesperrt. Aktualisieren: `docker compose pull && docker compose up -d`.

**Sichtbarkeit des Pakets:** Neue Pakete auf ghcr.io sind privat. Entweder das Paket unter GitHub → Profil → Packages → `vector` → Package settings auf *Public* stellen, oder auf dem Server anmelden: `echo <TOKEN> | docker login ghcr.io -u <github-name> --password-stdin` (Personal Access Token mit `read:packages`).

- Selbst bauen statt ziehen: `docker compose build` (im Repository-Ordner `deploy/`).
- Lokal ohne TLS: `docker compose run --rm -p 8080:8080 -e VECTRA_COOKIE_SECURE=false vectra` und `http://localhost:8080` öffnen.
- Backups (ADR-030) folgen mit Iteration 4. Bis dahin: `docker compose exec postgres pg_dump -U vectra -Fc vectra > vectra.dump`.

## KI-Assistent (optional)

Vectra ist ohne Assistent voll nutzbar. Er wird in zwei Schritten eingeschaltet:

1. **Anbieter konfigurieren** (in `.env`, danach `docker compose up -d`):

   | Variable | Bedeutung |
   |---|---|
   | `VECTRA_ASSISTANT_PROVIDER` | `anthropic` (Claude), `ollama` (lokal) oder `openai_compatible` (llama.cpp, vLLM, andere) |
   | `VECTRA_ASSISTANT_MODEL` | Modellname; bei `anthropic` leer = `claude-opus-5-5` |
   | `VECTRA_ASSISTANT_API_KEY` | API-Schlüssel (bei `anthropic` Pflicht) |
   | `VECTRA_ASSISTANT_BASE_URL` | bei `ollama` Standard `http://ollama:11434/v1`, bei `openai_compatible` Pflicht |
   | `VECTRA_ASSISTANT_EXTERNAL` | `true`, wenn Daten den Server verlassen; dann muss jede Person einmalig zustimmen |
   | `VECTRA_ASSISTANT_NAME` | Anzeigename des Anbieters |
   | `VECTRA_ASSISTANT_DAILY_LIMIT`, `VECTRA_ASSISTANT_RETENTION_DAYS`, `VECTRA_ASSISTANT_TIMEOUT_SECONDS` | Tageslimit je Person (200), Aufbewahrung der Unterhaltungen in Tagen (30), Zeitlimit je Modellaufruf (120 s) |

2. **Als Admin aktivieren:** Web-App → Einstellungen → Installation → „KI-Assistent aktivieren“. Danach schaltet jede Person ihn auf der Seite „Assistent“ für sich ein.

Der Assistent schreibt nie selbst. Er liest mit den Rechten der angemeldeten Person und legt Änderungen als **Vorschlag** an, den die Person bestätigt, bearbeitet oder verwirft (ADR-026). Bei Claude ist der serverseitige Ausweichpfad bei Ablehnungen aktiv (`VECTRA_ASSISTANT_FALLBACKS=false` schaltet ihn ab). Lokale Modelle über Ollama laufen nicht im Basis-Stack (Speicherbudget, ADR-030) und brauchen einen eigenen Container.

## Nur im Heimnetz (eigener Reverse-Proxy, z. B. Nginx Proxy Manager)

- Den `caddy`-Dienst weglassen und bei `vectra` `ports: ["8080:8080"]` setzen; der Proxy leitet auf `http://<Server-IP>:8080`.
- Läuft der Zugriff über **http://** (z. B. `http://vector.lan`): `VECTRA_COOKIE_SECURE: "false"` setzen, sonst sendet weder Browser noch App die Sitzung mit.
- Läuft er über **https://** mit eigenem Zertifikat: `VECTRA_COOKIE_SECURE` bleibt `"true"`; das CA-Zertifikat auf dem Handy installieren (Einstellungen → Sicherheit → Verschlüsselung → Zertifikat installieren). Die App vertraut vom Nutzer installierten Zertifizierungsstellen.
- In der App die Adresse **mit Schema** eintragen, z. B. `http://vector.lan`. Ohne Schema nimmt die App `https://` an. Klartext-HTTP erlaubt die App nur für lokale Namen (`*.lan`, `*.local`, `*.home.arpa`, `*.internal`, `*.fritz.box`).

## Android-App (APK)

Jeder CI-Lauf legt im Actions-Lauf das Artefakt **`vectra-apk`** ab. Bei einem Tag `v*` hängt die APK zusätzlich am GitHub-Release.

- `vectra-<version>.apk`: Release-APK, mit deinem Schlüssel signiert. Sie entsteht nur, wenn die Signatur-Secrets gesetzt sind (unten).
- `vectra-<version>-debug.apk`: immer vorhanden. Sie wird aber mit einem wechselnden Debug-Schlüssel signiert. Updates lassen sich deshalb nicht darüber installieren, sondern nur nach Deinstallation, und dabei gehen die lokalen Daten verloren.

In der App als Server die Domain eintragen, z. B. `vectra.example.org`.

### Signatur einrichten (einmalig)

```bash
keytool -genkeypair -v -keystore vectra.jks -alias vectra -keyalg RSA -keysize 4096 -validity 10000
base64 -w0 vectra.jks > vectra.jks.b64
```

Unter GitHub → Repository → Settings → Secrets and variables → Actions anlegen:

| Secret | Wert |
|---|---|
| `VECTRA_KEYSTORE_BASE64` | Inhalt von `vectra.jks.b64` |
| `VECTRA_KEYSTORE_PASSWORD` | Keystore-Passwort |
| `VECTRA_KEY_ALIAS` | `vectra` |
| `VECTRA_KEY_PASSWORD` | Schlüsselpasswort (falls abweichend) |

`vectra.jks` sicher aufbewahren. Ohne ihn lassen sich keine Updates der App mehr ausliefern.
