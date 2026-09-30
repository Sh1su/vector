# ADR-014 – OpenAPI 3.1 spec-first

- **Status:** akzeptiert · **Datum:** 2026-09-30

## Kontext
Der Auftrag verlangt spec-first mit OpenAPI 3.1 (5.2). Drei Clients sollen dieselben Typen nutzen; Abweichungen zwischen Doku und Verhalten wie in der Altanwendung (Phase 1, 05) sollen ausgeschlossen sein.

## Entscheidung
- **Eine Quelle:** `api/openapi.yaml` (aufteilbar in Dateien unter `api/`), gepflegt von Hand und im Review wie Code behandelt.
- **Generierung:**
  - Go-Server: `oapi-codegen` im *strict server*-Modus (Typen + Interface); Implementierung ruft Application Services.
  - Web-Client (TypeScript): Typen aus der Spezifikation (`openapi-typescript`) plus schlanker Fetch-Wrapper.
  - Android (Kotlin): OpenAPI Generator (`kotlin`, kotlinx.serialization, Retrofit/OkHttp).
  - Generierter Code wird nicht von Hand geändert; die CI prüft, dass er aktuell ist.
- **Qualität:** Linting (Spectral mit eigenem Regelset gemäß ADR-013), Beispiele je Operation, Contract-Tests gegen den laufenden Server in der CI.
- **Sicherheit in der Spezifikation:** jede Operation deklariert Security-Schema und benötigte Scopes (ADR-016); die CI verweigert Operationen ohne Deklaration (deny-by-default).
- **Veröffentlichung:** Die Spezifikation wird unter `/api/v1/openapi.json` ausgeliefert; eine API-Referenz ist optional im Web-Frontend.

## Ergänzung nach Spike S-2 (2026-09-30)
- `oapi-codegen` ab v2.8.0 mit `output-options.nullable-type: true` (Pflicht). Nur so bleiben bei JSON Merge Patch „fehlt“, `null` und Wert unterscheidbar.
- Die Spezifikation verwendet keine Mehrfachtypen außer `[T, "null"]`; die CI prüft das.
- `oapi-codegen` v2.8.0 verlangt Go ≥ 1.25.

## Konsequenzen
- (+) Server und Clients driften nicht auseinander.
- (−) Generator-Grenzen bei OpenAPI 3.1 (z. B. `oneOf`) müssen im Spike (AP-9) geprüft werden; Schemata werden bei Bedarf einfach gehalten.

## Bezug
Auftrag 5.2, 6.17; Phase 1 05; ADR-003, ADR-013, ADR-016.
