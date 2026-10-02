#!/usr/bin/env bash
# Vectra – Schnellinstallation mit Docker Compose.
#
#   curl -fsSL https://raw.githubusercontent.com/Sh1su/vector/main/deploy/install.sh | bash
#
# Legt einen Ordner (Standard: ./vectra) mit compose.yaml, .env und ggf. Caddyfile an.
# Danach: cd vectra && docker compose up -d
#
# Ohne Rückfragen (z. B. für Skripte) über Umgebungsvariablen steuerbar:
#   VECTRA_MODE=local|domain   local: http://<server>:8080 im Heimnetz, domain: HTTPS mit Let's Encrypt (Caddy)
#   VECTRA_DOMAIN=vectra.example.org   (nur domain)
#   VECTRA_PORT=8080                   (nur local)
#   VECTRA_DIR=vectra                  Zielordner
#   VECTRA_IMAGE=ghcr.io/sh1su/vector:latest
#   VECTRA_SETUP_TOKEN=…               Einrichtung nur mit diesem Token (domain: wird sonst erzeugt)
#   VECTRA_FORCE=1                     vorhandene Dateien überschreiben
set -euo pipefail

DIR="${VECTRA_DIR:-vectra}"
IMAGE="${VECTRA_IMAGE:-ghcr.io/sh1su/vector:latest}"
MODE="${VECTRA_MODE:-}"
DOMAIN="${VECTRA_DOMAIN:-}"
PORT="${VECTRA_PORT:-8080}"
SETUP_TOKEN="${VECTRA_SETUP_TOKEN:-}"

say()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33mHinweis:\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31mFehler:\033[0m %s\n' "$*" >&2; exit 1; }

# Eingaben auch bei „curl … | bash“ vom Terminal lesen.
ask() {
  local prompt="$1" default="${2:-}" answer=""
  if [ -r /dev/tty ]; then
    read -r -p "$prompt${default:+ [$default]}: " answer </dev/tty || true
  fi
  printf '%s' "${answer:-$default}"
}

secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -base64 48 | tr -dc 'A-Za-z0-9' | head -c "${1:-32}"
  else
    LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | head -c "${1:-32}"
  fi
}

command -v docker >/dev/null 2>&1 || warn "Docker ist nicht installiert: https://docs.docker.com/engine/install/"
if command -v docker >/dev/null 2>&1 && ! docker compose version >/dev/null 2>&1; then
  warn "„docker compose“ (Compose v2) fehlt. Bitte das Docker-Compose-Plugin installieren."
fi

if [ -z "$MODE" ]; then
  echo "Wie soll Vectra erreichbar sein?"
  echo "  1) Im Heimnetz über http://<server-ip>:$PORT (ohne Domain)"
  echo "  2) Im Internet über eine eigene Domain mit HTTPS (Let's Encrypt)"
  case "$(ask 'Auswahl' 1)" in
    2) MODE=domain ;;
    *) MODE=local ;;
  esac
fi
case "$MODE" in
  local) ;;
  domain)
    [ -n "$DOMAIN" ] || DOMAIN="$(ask 'Domain (DNS muss auf diesen Server zeigen), z. B. vectra.example.org')"
    [ -n "$DOMAIN" ] || die "Ohne Domain ist der Modus „domain“ nicht möglich (VECTRA_DOMAIN setzen)."
    # Im Internet die Ersteinrichtung absichern: nur mit Token.
    [ -n "$SETUP_TOKEN" ] || SETUP_TOKEN="$(secret 24)"
    ;;
  *) die "VECTRA_MODE muss „local“ oder „domain“ sein." ;;
esac

mkdir -p "$DIR"
cd "$DIR"
if [ -e .env ] && [ "${VECTRA_FORCE:-}" != "1" ]; then
  die "$(pwd)/.env existiert bereits. Zum Überschreiben VECTRA_FORCE=1 setzen (Passwörter gehen dabei verloren!)."
fi

say "Schreibe $(pwd)/.env"
umask 077
cat > .env <<EOF
# Vectra – erzeugt von install.sh am $(date -u +%Y-%m-%dT%H:%MZ)
VECTRA_IMAGE=$IMAGE
POSTGRES_PASSWORD=$(secret 32)
# Ersteinrichtung: leer = die erste Person, die /einrichtung öffnet, legt das Admin-Konto an
# (nur solange noch kein Konto existiert). Gesetzt = zusätzlich dieses Token nötig.
VECTRA_SETUP_TOKEN=$SETUP_TOKEN
VECTRA_DOMAIN=$DOMAIN
VECTRA_PORT=$PORT

# Optional: KI-Assistent (siehe README, Abschnitt „KI-Assistent“). Leer = aus.
VECTRA_ASSISTANT_PROVIDER=
VECTRA_ASSISTANT_MODEL=
VECTRA_ASSISTANT_API_KEY=
VECTRA_ASSISTANT_BASE_URL=
VECTRA_ASSISTANT_EXTERNAL=
VECTRA_ASSISTANT_NAME=
EOF
umask 022

say "Schreibe $(pwd)/compose.yaml"
cat > compose.yaml <<'EOF'
# Vectra (ADR-030): PostgreSQL + Vectra (Backend und Web-App in einem Image).
services:
  postgres:
    image: pgvector/pgvector:pg16
    restart: unless-stopped
    environment:
      POSTGRES_DB: vectra
      POSTGRES_USER: vectra
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?POSTGRES_PASSWORD fehlt}
    command: ["postgres", "-c", "shared_buffers=128MB", "-c", "max_connections=30"]
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U vectra -d vectra"]
      interval: 10s
      retries: 5

  vectra:
    image: ${VECTRA_IMAGE:-ghcr.io/sh1su/vector:latest}
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      VECTRA_DATABASE_URL: postgres://vectra:${POSTGRES_PASSWORD}@postgres:5432/vectra
      VECTRA_SETUP_TOKEN: ${VECTRA_SETUP_TOKEN:-}
      VECTRA_COOKIE_SECURE: "__COOKIE_SECURE__"
      VECTRA_ASSISTANT_PROVIDER: ${VECTRA_ASSISTANT_PROVIDER:-}
      VECTRA_ASSISTANT_MODEL: ${VECTRA_ASSISTANT_MODEL:-}
      VECTRA_ASSISTANT_BASE_URL: ${VECTRA_ASSISTANT_BASE_URL:-}
      VECTRA_ASSISTANT_API_KEY: ${VECTRA_ASSISTANT_API_KEY:-}
      VECTRA_ASSISTANT_EXTERNAL: ${VECTRA_ASSISTANT_EXTERNAL:-}
      VECTRA_ASSISTANT_NAME: ${VECTRA_ASSISTANT_NAME:-}
__PORTS__    read_only: true
    cap_drop: [ALL]
    security_opt: ["no-new-privileges:true"]
    mem_limit: 128m
__CADDY__
volumes:
  pgdata:
__CADDY_VOLUMES__
EOF

if [ "$MODE" = "domain" ]; then
  sed -i.bak \
    -e 's/__COOKIE_SECURE__/true/' \
    -e '/__PORTS__/{s/__PORTS__//;}' \
    compose.yaml
  caddy_block='  caddy:
    image: caddy:2-alpine
    restart: unless-stopped
    depends_on: [vectra]
    ports: ["80:80", "443:443"]
    environment:
      VECTRA_DOMAIN: ${VECTRA_DOMAIN:?VECTRA_DOMAIN fehlt}
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy_data:/data
      - caddy_config:/config
'
  awk -v block="$caddy_block" '{ if ($0 == "__CADDY__") printf "%s", block; else if ($0 == "__CADDY_VOLUMES__") print "  caddy_data:\n  caddy_config:"; else print }' compose.yaml > compose.tmp && mv compose.tmp compose.yaml
  say "Schreibe $(pwd)/Caddyfile"
  cat > Caddyfile <<'EOF'
# TLS automatisch (Let's Encrypt); die übrigen Sicherheits-Header setzt Vectra.
{$VECTRA_DOMAIN} {
	encode zstd gzip
	header Strict-Transport-Security "max-age=31536000; includeSubDomains"
	reverse_proxy vectra:8080
}
EOF
  URL="https://$DOMAIN"
else
  sed -i.bak -e 's/__COOKIE_SECURE__/false/' compose.yaml
  awk '{ if ($0 == "__PORTS__    read_only: true") { print "    ports: [\"${VECTRA_PORT:-8080}:8080\"]"; print "    read_only: true" } else if ($0 == "__CADDY__" || $0 == "__CADDY_VOLUMES__") next; else print }' compose.yaml > compose.tmp && mv compose.tmp compose.yaml
  IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
  URL="http://${IP:-localhost}:$PORT"
fi
rm -f compose.yaml.bak

echo
say "Fertig. Vectra starten:"
echo
echo "    cd $(pwd) && docker compose up -d"
echo
echo "Danach $URL im Browser öffnen. Beim ersten Aufruf erscheint die Ersteinrichtung"
echo "für das Administratorkonto (nur solange noch kein Konto existiert)."
if [ -n "$SETUP_TOKEN" ]; then
  echo
  echo "Setup-Token für die Ersteinrichtung:  $SETUP_TOKEN"
  echo "(steht auch in $(pwd)/.env)"
fi
echo
echo "Aktualisieren:  docker compose pull && docker compose up -d"
echo "Logs:           docker compose logs -f vectra"
