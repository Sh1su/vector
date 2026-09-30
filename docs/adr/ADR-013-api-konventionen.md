# ADR-013 – API-Konventionen

- **Status:** akzeptiert · **Datum:** 2026-09-30

## Kontext
Die Alt-API ist uneinheitlich: Erfolg mit `200` und `success:false`, Exception-Texte an Clients, kulturabhängige Zahlen- und Datumsformate, keine Paginierung (Phase 1, T-05, T-09, T-10). Vectra hat drei Clients (Web, Android, Assistent) und eine öffentliche API.

## Entscheidung
- **Basis:** `/api/v1`, JSON (UTF-8), Feldnamen `snake_case`, Ressourcen im Plural (`/vehicles/{vehicle_id}/fuel-fills`).
- **Formate:** Zeitpunkte RFC 3339 mit Offset plus `time_zone` (IANA) und `time_precision` (ADR-008); Mengen als Objekt `{ "value": 45.2, "unit": "l" }` bei Eingabe, Antwort enthält kanonischen Wert und Originalwert (ADR-007); Geld `{ "amount_minor": 8450, "currency": "EUR" }` (ADR-029). Zahlen immer mit Punkt, nie kulturabhängig.
- **Fehler:** RFC 9457 Problem Details (`application/problem+json`) mit `type`, `title`, `status`, `detail`, `instance`, `request_id` und bei Validierung `errors[]` (`pointer`, `code`, `message`). Plausibilitätswarnungen: `422` mit `type=…/plausibility` und `anomalies[]`; erneutes Senden mit `confirm_anomalies` und Begründung (ADR-010). Keine internen Fehlermeldungen oder Stacktraces nach außen.
- **Statuscodes:** `201` + `Location` beim Anlegen, `204` beim Löschen, `404` auch für fremde Ressourcen ohne Leserecht (kein Informationsleck), `403` nur bei sichtbarer Ressource ohne ausreichende Rolle.
- **Paginierung:** Cursor-basiert (`limit` ≤ 200, Default 50, `cursor`), Antwort `{ "items": [], "next_cursor": … }`. Sortierung Default `occurred_at desc, id desc`.
- **Filter:** explizite Query-Parameter je Ressource (`from`, `to`, `category`, `include_deleted`), keine generische Abfragesprache.
- **Änderungen:** `PATCH` mit JSON Merge Patch (RFC 7396) + `If-Match` (ADR-012). Aktionen als Unterressourcen mit `POST` (`/maintenance-items/{id}/complete`).
- **Versionierung:** Brechende Änderungen nur mit neuer Hauptversion; Felder werden additiv ergänzt, Clients ignorieren unbekannte Felder.
- **Rate-Limits:** Antwort `429` mit `Retry-After`.

## Konsequenzen
- (+) Einheitliches Verhalten für alle Clients, gut generierbar.
- (−) Keine Kompatibilität zur LubeLogger-API (Q-07 bleibt außerhalb des MVP).

## Bezug
Auftrag 6.17; Phase 1 05, T-05, T-09, T-10; ADR-007, ADR-008, ADR-010, ADR-012, ADR-029.
