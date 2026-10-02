# Vectra-API: Daten abrufen und erfassen

Vectra hat eine REST-API unter `/api/v1` (JSON, UTF-8). Die Web-App und die Android-App nutzen genau diese API; alles, was dort geht, geht auch per Skript. Maßgeblich ist die Spezifikation [`api/openapi.yaml`](../api/openapi.yaml), die jede Installation auch unter `GET /api/v1/openapi.json` ausliefert (z. B. für Swagger UI, Postman oder Code-Generatoren).

## 1. Zugang: API-Token

1. In der Web-App **Einstellungen → KI-Zugang (MCP)** öffnen. Dasselbe Token gilt für den MCP-Server und die REST-API.
2. Namen, Gültigkeit, Rechte (Scopes) und optional einzelne Fahrzeuge wählen.
3. Das Token (`vct_…`) wird **nur einmal** angezeigt – sicher ablegen. Gespeichert wird nur ein Hash.

Jede Anfrage trägt das Token im Header:

```bash
export VECTRA=https://vectra.example.org      # bzw. http://<server-ip>:8080
export TOKEN=vct_…
curl -s -H "Authorization: Bearer $TOKEN" $VECTRA/api/v1/vehicles
```

| Scope | erlaubt |
|---|---|
| `vehicles:read` | alles lesen (Fahrzeuge, Stände, Verbrauch, Öl, Wartung, Kosten, Historie) – immer enthalten |
| `entries:write` | Einträge anlegen und ändern (Kilometerstand, Tanken, Öl, Wartung erledigen …) |
| `entries:delete` | Einträge löschen |

Welche Operation welchen Scope braucht, steht in der Spezifikation (`x-vectra-scope`); Operationen ohne Bearer-Freigabe (z. B. Tokens anlegen) sind mit Token gesperrt. Wirksam ist immer die **Schnittmenge** aus Token-Scopes und deinen Rollen am Fahrzeug: Ein Token kann nie mehr als du selbst. Ist das Token auf ein Fahrzeug beschränkt, sind alle anderen Fahrzeuge unsichtbar (`404`). API-Tokens können keine weiteren Tokens anlegen. Widerrufen geht jederzeit in den Einstellungen; Einträge über die API tragen die Herkunft `api` und erscheinen in der Änderungshistorie mit Akteur `api_token`.

## 2. Konventionen

- **Mengen** werden mit Einheit übergeben: `{"value": 55.2, "unit": "l"}`. Einheiten: `km`, `mi`, `h`, `ml`, `l`, `gal_us`, `gal_imp`, `qt_us`, `qt_imp`, `kWh`, `Wh`. Antworten enthalten den kanonischen Wert (`canonical`: m, ml, Wh, s) und die Originaleingabe; berechnete Werte kommen als `{"value": 6.88, "unit": "l/100km"}` in den Anzeigeeinheiten des Kontos.
- **Geld** in kleinster Einheit: `{"amount_minor": 9930, "currency": "EUR"}`.
- **Zeitpunkte** RFC 3339 plus IANA-Zeitzone: `"occurred_at": "2026-10-02T14:30:00+02:00", "time_zone": "Europe/Berlin"`. Datumsfilter `from`/`to` sind Kalenderdaten (`2026-01-01`), jeweils inklusive.
- **Listen** sind seitenweise: `?limit=50` (max. 200) und `?cursor=<next_cursor>` aus der vorigen Antwort, neueste zuerst.
- **Ändern** per `PATCH` (JSON Merge Patch) mit `If-Match: "<version>"` aus dem `ETag` bzw. dem Feld `version`.
- **Fehler** als `application/problem+json` mit `status`, `title`, `detail` und `errors[]` (Feld, Code).
- **Plausibilität:** Auffällige Werte (z. B. Stand kleiner als zuvor) liefern `422` mit `anomalies[]`. Bestätigbare Befunde sendest du erneut mit `"confirm_anomalies": ["P1"], "anomaly_reason": "…"`.

## 3. Daten abrufen – Beispiele

Die Fahrzeug-ID aus der Liste merken:

```bash
VID=$(curl -s -H "Authorization: Bearer $TOKEN" $VECTRA/api/v1/vehicles | jq -r '.items[0].id')
```

| Was | Aufruf |
|---|---|
| Fahrzeuge | `GET /vehicles` |
| Stammdaten | `GET /vehicles/{id}` |
| aktueller Kilometerstand | `GET /vehicles/{id}/odometer/current` |
| Stand zu einem Zeitpunkt | `GET /vehicles/{id}/odometer/value-at?at=2026-06-01T00:00:00Z` |
| gefahrene Strecke | `GET /vehicles/{id}/odometer/distance?from=…&to=…` (Zeitpunkte) |
| Kilometerstände (Verlauf) | `GET /vehicles/{id}/odometer/readings` |
| Tank-/Ladevorgänge | `GET /vehicles/{id}/fuel-fills?energy_carrier=diesel&from=2026-01-01` |
| Verbrauch | `GET /vehicles/{id}/fuel/consumption?energy_carrier=diesel` |
| Öleinträge | `GET /vehicles/{id}/oil-entries` |
| Ölverbrauch je Messreihe | `GET /vehicles/{id}/oil/series` |
| Nachfüllrate & Mengen | `GET /vehicles/{id}/oil/statistics?from=2026-01-01&to=2026-12-31` |
| Wartungen mit Fälligkeit | `GET /vehicles/{id}/maintenance-items` |
| nur Fälligkeiten | `GET /vehicles/{id}/maintenance/status` |
| Fälligkeiten aller Fahrzeuge | `GET /me/due?min_level=upcoming` |
| **Änderungshistorie** | `GET /vehicles/{id}/audit-events` (optional `?object_id=<eintrag>`) |
| Wartungsbücher (Vorlagen) | `GET /maintenance-books` |
| Kosten, Fahrten, Service, Dokumente | siehe `api/openapi.yaml` (Tags Costs, Trips, ServiceHistory, Documents) |

```bash
# Durchschnittsverbrauch
curl -s -H "Authorization: Bearer $TOKEN" "$VECTRA/api/v1/vehicles/$VID/fuel/consumption?energy_carrier=diesel" \
  | jq '.consumption.average'
# { "value": 9.84, "unit": "l/100km" }

# Was ist als Nächstes fällig?
curl -s -H "Authorization: Bearer $TOKEN" "$VECTRA/api/v1/vehicles/$VID/maintenance/status" \
  | jq '.items[] | {title, level, due_date, distance_remaining}'

# Änderungshistorie: wer hat wann was geändert (neueste zuerst, seitenweise)
curl -s -H "Authorization: Bearer $TOKEN" "$VECTRA/api/v1/vehicles/$VID/audit-events?limit=20" \
  | jq '.items[] | {occurred_at, actor_kind, action, object_type, changes, reason}'
```

Ein Historieneintrag sieht so aus:

```json
{
  "id": "01a0fc…",
  "occurred_at": "2026-10-02T10:30:08Z",
  "actor_account_id": "01a0fb…",
  "actor_kind": "user",
  "action": "odometer.reading_corrected",
  "object_type": "odometer_reading",
  "object_id": "01a0fc…",
  "changes": { "value": { "old": 45000000, "new": 44000000 }, "supersedes": "01a0fb…" },
  "reason": "Tippfehler",
  "request_id": "01a0fc…"
}
```

`actor_kind` ist `user` (Web/App), `api_token`, `assistant` (bestätigter Vorschlag des Assistenten), `import` oder `system`.

## 4. Daten erfassen (Scope `entries:write`)

```bash
# Kilometerstand
curl -s -X POST -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  "$VECTRA/api/v1/vehicles/$VID/odometer/readings" \
  -d '{"occurred_at":"2026-10-02T08:00:00+02:00","time_zone":"Europe/Berlin","value":{"value":98300,"unit":"km"}}'

# Tankvorgang mit Preis pro Liter (der Server rechnet den Gesamtbetrag)
curl -s -X POST -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  "$VECTRA/api/v1/vehicles/$VID/fuel-fills" \
  -d '{"occurred_at":"2026-10-02T08:05:00+02:00","time_zone":"Europe/Berlin","energy_carrier":"diesel",
       "quantity":{"value":62.4,"unit":"l"},"fill_level":"full","odometer":{"value":98300,"unit":"km"},
       "price_per_unit":{"value":1.669,"currency":"EUR","per_unit":"l"}}'

# Öl nachgefüllt
curl -s -X POST -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  "$VECTRA/api/v1/vehicles/$VID/oil-entries" \
  -d '{"occurred_at":"2026-10-02T08:10:00+02:00","time_zone":"Europe/Berlin","kind":"top_up",
       "level_before":{"step":"quarter"},"oil_added":{"value":0.7,"unit":"l"},"odometer":{"value":98300,"unit":"km"}}'

# Wartung als erledigt markieren (Idempotency-Key schützt vor Doppelbuchung)
curl -s -X POST -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -H "Idempotency-Key: $(uuidgen)" \
  "$VECTRA/api/v1/vehicles/$VID/maintenance-items/<item_id>/completions" \
  -d '{"kind":"done","completed_on":"2026-10-02","completed_odometer":{"value":98300,"unit":"km"}}'
```

Wiederholbare Anlage: Gibst du im Body eine eigene `id` (UUID) mit, liefert ein erneutes Senden mit gleichem Inhalt `200` statt eines Duplikats.

## 5. Einbindung

**Python**

```python
import requests
S = requests.Session()
S.headers["Authorization"] = "Bearer vct_…"
base = "https://vectra.example.org/api/v1"
for v in S.get(f"{base}/vehicles").json()["items"]:
    cur = S.get(f"{base}/vehicles/{v['id']}/odometer/current").json()
    print(v["display_name"], cur.get("display"))
```

**Home Assistant** (`configuration.yaml`)

```yaml
rest:
  - resource: https://vectra.example.org/api/v1/vehicles/<VID>/odometer/current
    headers:
      Authorization: !secret vectra_token     # "Bearer vct_…"
    scan_interval: 3600
    sensor:
      - name: Sprinter Kilometerstand
        value_template: "{{ value_json.display.value }}"
        unit_of_measurement: km
  - resource: https://vectra.example.org/api/v1/me/due?min_level=upcoming
    headers:
      Authorization: !secret vectra_token
    scan_interval: 3600
    sensor:
      - name: Fällige Wartungen
        value_template: "{{ value_json['items'] | length }}"
```

## 6. Ersteinrichtung per API

Solange noch kein Konto existiert, meldet `GET /api/v1/auth/setup` `{"setup_required": true, "token_required": …}`. Das erste Administratorkonto legt dann `POST /api/v1/auth/setup` an (`email`, `display_name`, `password`, bei gesetztem `VECTRA_SETUP_TOKEN` zusätzlich `setup_token`). Sobald ein Konto existiert, antwortet der Aufruf mit `409`.
