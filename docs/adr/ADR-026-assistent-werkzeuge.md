# ADR-026 – Assistent als Werkzeugschicht (Vorschlag → Bestätigung → Ausführung)

- **Status:** akzeptiert · **Datum:** 2026-09-30

## Kontext
Der Assistent schreibt nie direkt in die Datenbank. Er nutzt definierte Werkzeuge, die dieselben Application Services und Rechteprüfungen verwenden wie die API. Änderungen laufen über den Ablauf natürliche Sprache → strukturierter Vorschlag → Bestätigung → Ausführung → Audit mit Herkunft „Assistent“ (Auftrag 6.13).

## Entscheidung
**Werkzeuge** (Auswahl laut Auftrag, Namen wie in 6.13):
| Werkzeug | Art | Service |
|---|---|---|
| `get_vehicle`, `get_current_odometer`, `get_oil_consumption`, `get_due_maintenance`, `get_vehicle_history`, `search_documents` | lesen | Vehicles, Odometer, Oil, Maintenance, ServiceHistory, Assistant-RAG |
| `create_odometer_entry`, `create_oil_entry`, `create_trip`, `create_maintenance_event` | **Vorschlag** | Odometer, Oil, Trips, ServiceHistory/Maintenance |

- **Schemas:** Die Werkzeugeingaben werden aus den `…Create`-Schemas der OpenAPI-Spezifikation abgeleitet (ADR-014), also mit denselben Feldern, Einheiten und Enums. Wo der Anbieter es kann, werden strikte Schemas verwendet (bei Claude `strict: true`). Der Server validiert **jede** Eingabe trotzdem selbst, bevor etwas passiert.
- **Lesende Werkzeuge** laufen sofort, mit den Rechten des Nutzers (ID-01) und nur für seine Fahrzeuge. Das Ergebnis geht als Daten an das Modell zurück.
- **Schreibende Werkzeuge führen nichts aus.** Sie erzeugen einen **Vorschlag** (`assistant.proposal`: Operation, vollständiger Request-Body, Plausibilitätsbefunde aus einem Probelauf ohne Schreiben, Ablauf nach 30 min). Die UI zeigt ihn als Formular: **Bestätigen / Bearbeiten / Verwerfen**. Erst „Bestätigen“ ruft den Application Service auf, mit `origin = assistant`, Audit-Akteur `assistant` im Namen des Nutzers (ADR-011) und einem Idempotency-Key aus der Vorschlags-ID. Befunde (ADR-010) muss der Nutzer im selben Dialog bestätigen, das Modell kann das nicht.
- **Beispiel (Auftrag 6.13):** „Ich habe bei 143.520 km 0,7 Liter Öl nachgefüllt.“ → Vorschlag `create_oil_entry` mit `kind = top_up`, `odometer = {143520, km}`, `oil_added = {0.7, l}`, `occurred_at = jetzt` → Bestätigen, Bearbeiten oder Verwerfen.
- **Mehrdeutigkeit:** Hat der Nutzer mehrere Fahrzeuge und nennt keins, fragt der Assistent nach (oder nimmt das Fahrzeug aus dem aktuellen UI-Kontext) und rät nicht.
- **Werkzeugwahl:** Das Modell entscheidet selbst (`auto`); es gibt keine erzwungene Werkzeugwahl, weil aktuelle Modelle sie teils ablehnen. Mehrere Lesewerkzeuge in einer Antwort werden parallel ausgeführt; schreibende Vorschläge werden einzeln angezeigt.
- **Kein freier Code, keine SQL- oder Dateisystemwerkzeuge**, kein Internetzugriff.
- **API** (Ergänzung zu `api/openapi.yaml`, Tag „Assistant“): Status/Zustimmung, Unterhaltungen, Nachrichten (Streaming), Vorschläge bestätigen/verwerfen.

## Konsequenzen
- (+) Der Assistent kann nie mehr als der Nutzer. Jede Änderung ist bestätigt, validiert und auditiert.
- (+) Neue Werkzeuge entstehen aus bestehenden Services, ohne eigene Fachlogik.
- (−) Ein zusätzlicher Klick je Änderung; das ist gewollt.

## Bezug
Auftrag 6.13; ADR-010, ADR-011, ADR-012, ADR-014, ADR-016, ADR-024, ADR-025.
