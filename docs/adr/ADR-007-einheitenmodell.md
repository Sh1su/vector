# ADR-007 – Einheitenmodell: SI intern + Originalwert und Originaleinheit

- **Status:** akzeptiert · **Datum:** 2026-09-30 · **Entscheidung:** E-1

## Kontext
Die Altanwendung speichert Zahlen ohne Einheit; die Bedeutung hängt von Nutzereinstellungen ab. Das führt zu falschen Werten bei geteilten Fahrzeugen, Einstellungswechseln und Exporten (Phase 1, BR-059, D-04, D-06). Vectra braucht metrische und imperiale Einheiten (Auftrag 5.3).

## Optionen
- **A: SI kanonisch + Original** – eindeutige Berechnungen, die Eingabe bleibt nachweisbar.
- B: Einheit je Fahrzeug – Aggregation über Fahrzeuge muss umrechnen.
- C: Einheit je Wert ohne Kanon – jede Berechnung muss umrechnen.

## Entscheidung
Option A.

| Größe | Kanonische Einheit (gespeichert) | Datentyp |
|---|---|---|
| Distanz / Kilometerstand | Meter | `bigint` |
| Motorstunden (Fahrzeuge mit Stundenzähler) | Sekunden | `bigint` |
| Flüssigkeitsmenge (Kraftstoff, Öl) | Milliliter | `bigint` |
| Energie (Laden) | Wattstunden | `bigint` |
| Ölstand | Prozent 0–100 zwischen Min und Max (E-13) | `numeric(5,2)` |
| Ladezustand | Prozent 0–100 | `numeric(5,2)` |

Jeder **erfasste** Messwert speichert zusätzlich `input_value` (Dezimalzahl wie eingegeben) und `input_unit` (Code wie `km`, `mi`, `h`, `l`, `ml`, `gal_us`, `gal_imp`, `qt_us`, `qt_imp`, `kWh`; Quarts für Motoröl ergänzt in AP-4: 1 qt US = 946,352946 ml, 1 qt imp = 1 136,5225 ml). Berechnete Werte (Verbrauch, Kosten pro km) werden nicht gespeichert, sondern aus kanonischen Werten berechnet und für die Ausgabe in die **Anzeigeeinheit** umgerechnet.

- **Anzeigeeinheiten** sind eine Nutzereinstellung mit Default je Fahrzeug. Sie betreffen nur die Darstellung.
- **Umrechnungsfaktoren** sind exakt definiert: 1 mi = 1 609,344 m; 1 US gal = 3,785411784 l; 1 imp gal = 4,54609 l.
- **Verbrauch** wird intern als Menge pro Distanz (ml/m) berechnet und je nach Einstellung als l/100 km, km/l, mpg (US) oder mpg (UK) bzw. kWh/100 km ausgegeben.
- **Rundung** erfolgt ausschließlich bei der Ausgabe (kaufmännisch, Anzahl Stellen je Größe in der API-Spezifikation).

## Konsequenzen
- (+) Geteilte Fahrzeuge zeigen jedem Nutzer korrekte Werte in seiner Einheit.
- (+) Die Originaleingabe bleibt als Nachweis erhalten (Beweisfunktion, Auftrag 6.7).
- (−) Beim Import muss die Einheit je Fahrzeug festgelegt werden (ADR-027).
- (−) Ganzzahlige Speicherung verlangt Umrechnung an den API-Grenzen. Die API nimmt Dezimalwerte mit Einheit entgegen.

## Bezug
Phase 1 BR-008, BR-059, M-4, MG-1; Auftrag 5.3.
