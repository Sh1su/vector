# Vectra API – Spezifikation

- **Datei:** `openapi.yaml` (OpenAPI 3.1, Entwurf `1.0.0-draft.1`, Phase 2 AP-6)
- **Status:** Spezifikation. Es gibt noch **keinen** Server- oder Client-Code (Phase 2).
- **Grundlagen:** ADR-013 (Konventionen), ADR-014 (spec-first), ADR-016 (Rechte), Domänenmodelle `docs/phase-2/10-domaene-*.md`

## Umfang

102 Pfade, 152 Operationen, 165 Schemas.

| Bereich | Operationen | Bereich | Operationen |
|---|---|---|---|
| Identity | 22 | ServiceHistory | 6 |
| Sharing | 6 | Costs | 15 |
| Vehicles (inkl. Dashboard) | 11 | Documents | 17 |
| Odometer | 10 | Notes | 5 |
| Fuel | 6 | Notifications | 8 |
| Oil | 7 | Import | 7 |
| Trips | 13 | Audit, Sync, Admin, Health | 9 |
| Maintenance | 10 | | |

Nicht enthalten: Assistant-Endpunkte (AP-10, ADR-026) und die Kompatibilitätsschicht zur LubeLogger-API (Q-07, nicht MVP).

## Gestaltungsregeln im Entwurf

- **Ressourcen je Fahrzeug:** `/vehicles/{vehicle_id}/…`. Die Rechteprüfung erfolgt trotzdem am geladenen Objekt (ID-01). Stimmt die `vehicle_id` im Pfad nicht mit der des Objekts überein, antwortet der Server mit `404`.
- **Schemas je Ressource:** `X` (lesen), `XCreate` (anlegen, optional mit client-erzeugter `id`) und `XPatch` (JSON Merge Patch, `application/merge-patch+json`, immer mit `If-Match`).
- **Messwerte:** Die Eingabe erfolgt als `QuantityInput {value, unit}`. Die Ausgabe ist `Quantity {canonical, canonical_unit, input_value, input_unit}`, berechnete Werte sind `DisplayValue` in der Anzeigeeinheit des Nutzers (ADR-007).
- **Plausibilität:** Die Antwort `422` enthält `anomalies[]`. Der Client wiederholt den Request mit `confirm_anomalies` und `anomaly_reason` (Schema `AnomalyConfirmation`, ADR-010). Nicht bestätigbare Befunde haben `confirmable: false`.
- **Messpunkte** haben kein `PATCH`; Änderungen laufen über `/corrections` (I-ODO-4). **Abgeschlossene Fahrten** haben ebenfalls nur `/corrections` und `/cancel` (TR-03).
- **Zeitabhängige Aktionen** (Erledigung, Bestätigung eines Kostenvorkommens, Import starten) sind mit `Idempotency-Key` wiederholbar (ADR-012).
- **Rechte je Operation:** `x-vectra-role` (mindestens nötige Rolle bzw. `self`, `admin`, `public`) und `x-vectra-scope` (API-Token-Scope). Operationen, die nur mit Web-Sitzung erlaubt sind (API-Token anlegen, Passwort ändern, Import hochladen, Webhook anlegen), deklarieren nur `sessionCookie`.

## Prüfen

```bash
npx @redocly/cli lint api/openapi.yaml --config api/redocly.yaml
```

Stand 30.09.2026: gültig, ohne Warnungen. Zusätzlich muss die CI prüfen (ADR-014, deny-by-default), dass jede Operation `security`, `x-vectra-role` und, außer bei `self`/`public`, `x-vectra-scope` hat. Die Rechte-Tests in `docs/phase-2/20-regelkatalog-soll.md` §10.3 werden aus diesen Angaben erzeugt.

## Offene Punkte

- **OP-API-1:** Ob `oapi-codegen` alle genutzten 3.1-Konstrukte (`type: [x, "null"]`, `contentMediaType`) verarbeitet, klärt Spike S-2 (`docs/phase-2/40-spikes.md`).
- **OP-API-2:** Mengen- und Kurznamen der Anzeigeeinheiten (`DisplayValue.unit`) bekommen eine feste Liste, sobald i18n festgelegt ist (ADR-028).
