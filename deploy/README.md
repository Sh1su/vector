# Vectra betreiben

## Server (Docker)

Die CI veröffentlicht bei jedem Push ein Image für amd64 und arm64 unter `ghcr.io/sh1su/vector`:

| Tag | Inhalt |
|---|---|
| `latest` | letzter Stand des Standard-Branches |
| `sha-<commit>` | genau dieser Commit |
| `1.2.3` | Release-Tag `v1.2.3` |

```bash
mkdir vectra && cd vectra
# compose.yaml, Caddyfile und .env.example aus deploy/ auf den Server kopieren
cp .env.example .env    # Domain und Datenbankpasswort eintragen
docker compose pull
docker compose up -d
docker compose logs vectra | grep setup_token   # Einmal-Token für die Ersteinrichtung
```

Danach `https://<VECTRA_DOMAIN>/einrichtung` öffnen und das erste Administratorkonto anlegen. Aktualisieren: `docker compose pull && docker compose up -d`.

**Sichtbarkeit des Pakets:** Neue Pakete auf ghcr.io sind privat. Entweder das Paket unter GitHub → Profil → Packages → `vector` → Package settings auf *Public* stellen, oder auf dem Server anmelden: `echo <TOKEN> | docker login ghcr.io -u <github-name> --password-stdin` (Personal Access Token mit `read:packages`).

- Selbst bauen statt ziehen: `docker compose build` (im Repository-Ordner `deploy/`).
- Lokal ohne TLS: `docker compose run --rm -p 8080:8080 -e VECTRA_COOKIE_SECURE=false vectra` und `http://localhost:8080` öffnen.
- Backups (ADR-030) folgen mit Iteration 4. Bis dahin: `docker compose exec postgres pg_dump -U vectra -Fc vectra > vectra.dump` und das Volume `files` (Dokumente, Fahrzeugbilder) sichern, z. B. `docker run --rm -v vectra_files:/data -v "$PWD":/b alpine tar czf /b/files.tgz -C /data .`.

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
