# Vectra betreiben (Iteration 1)

```bash
cd deploy
cp .env.example .env    # Domain und Datenbankpasswort eintragen
docker compose up -d --build
docker compose logs vectra | grep setup_token   # Einmal-Token für die Ersteinrichtung
```

Danach `https://<VECTRA_DOMAIN>/einrichtung` öffnen und das erste Administratorkonto anlegen.

- Lokal ohne TLS: `docker compose run --rm -p 8080:8080 -e VECTRA_COOKIE_SECURE=false vectra` und `http://localhost:8080` öffnen.
- Backups (ADR-030) folgen mit Iteration 4. Bis dahin: `docker compose exec postgres pg_dump -U vectra -Fc vectra > vectra.dump`.
