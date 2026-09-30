# ADR-003 – HTTP-Schicht: chi auf net/http

- **Status:** akzeptiert · **Datum:** 2026-09-30

## Kontext
Der Auftrag gibt Go mit chi vor (5.2). Die HTTP-Schicht soll dünn bleiben: Handler übersetzen nur zwischen HTTP und Application Services (ADR-001). Das Speicherbudget (< 50 MB Backend im Leerlauf) spricht gegen schwere Frameworks.

## Optionen
- **A: chi** – kompatibel zu `net/http`, Middleware-Ökosystem, keine eigene Context-Abstraktion.
- B: Echo / Gin – eigene Context-Typen, bindet Handler an das Framework.
- C: `net/http` pur (Go-1.22-Routing) – ausreichend, aber Middleware-Gruppen müssen selbst gebaut werden.

## Entscheidung
Option A. Regeln:
- Handler werden aus der OpenAPI-Spezifikation generiert (strict server interface, ADR-014); chi dient nur als Router und Middleware-Kette.
- Feste Middleware-Reihenfolge: Request-ID → strukturiertes Logging (`slog`) → Panic-Recovery → Sicherheits-Header → Größenlimit des Request-Body → Authentifizierung (ADR-015) → CSRF (nur Cookie-Sitzungen) → Rate-Limit → Handler.
- Sicherheits-Header zentral: `Content-Security-Policy` (restriktiv, ohne `unsafe-inline`), `X-Content-Type-Options: nosniff`, `Referrer-Policy: same-origin`, `Permissions-Policy`; HSTS setzt der Reverse Proxy (ADR-030).
- Keine Geschäftslogik in Handlern, keine zustandsändernden GET-Endpunkte (korrigiert T-06).
- Serverseitig gerenderte Seiten gibt es nicht; das Web-Frontend ist eine statische SPA (Auftrag 5.2).

## Präzisierung Phase 3 (2026-09-30)
Die Content-Security-Policy erlaubt für **Styles** `'unsafe-inline'`. Radix-Dialoge fügen für die Scroll-Sperre zur Laufzeit ein `<style>`-Element ein, das ohne diese Freigabe blockiert wird (im Ende-zu-Ende-Test beobachtet). Für Skripte gilt weiterhin nur `'self'`, deshalb ist Inline-JavaScript weiterhin ausgeschlossen. Das Risiko von Inline-Styles ist deutlich geringer als das von Inline-Skripten und wird so akzeptiert.

## Konsequenzen
- (+) Standardkonforme Handler, leicht testbar mit `httptest`.
- (−) Validierung und Fehlerabbildung müssen über die generierte Schicht einheitlich verdrahtet werden (ADR-013).

## Bezug
Auftrag 5.2; Phase 1 T-06, T-09; ADR-001, ADR-013, ADR-014.
