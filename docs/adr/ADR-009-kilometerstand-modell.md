# ADR-009 – Kilometerstand als eigene Messreihe mit Herkunft und Tachotausch

- **Status:** akzeptiert · **Datum:** 2026-09-30 · **Entscheidung:** E-3

## Kontext
In der Altanwendung ist der „aktuelle Kilometerstand“ das Maximum über fünf Record-Typen. Ein Tippfehler bestimmt dauerhaft alle Fälligkeiten; die Tachokorrektur kennt keinen Gültigkeitszeitraum (Phase 1, BR-015, BR-022, D-01, D-13).

## Entscheidung
1. **Eine Messreihe je Fahrzeug:** Das Modul Odometer ist die einzige Quelle für Kilometerstände. Tanken, Ölmessung, Service und Fahrten erzeugen oder referenzieren einen Messpunkt (`odometer_reading_id`), statt einen eigenen Stand zu speichern.
2. **Messpunkt:** `id`, `vehicle_id`, `occurred_at`/`time_zone`/`time_precision` (ADR-008), `value_m` (kanonisch, ADR-007), `input_value`/`input_unit`, `source` ∈ {`manual`, `fuel`, `oil`, `trip_start`, `trip_end`, `service`, `import`, `assistant`}, `source_ref`, optional `photo_document_id`, `status` ∈ {`valid`, `confirmed_anomaly`, `superseded`}.
3. **Korrekturen überschreiben nicht:** Eine Korrektur erzeugt einen neuen Messpunkt, der den alten über `supersedes_id` ersetzt. Der alte erhält `status = superseded` und bleibt im Audit sichtbar.
4. **Zählerabschnitte (Tachotausch):** Ein Fahrzeug hat einen oder mehrere `odometer_segments` mit `started_at` und `offset_m`. Beim Tachotausch wird ein neuer Abschnitt begonnen. **Angezeigter Stand** = Zählerwert im Abschnitt; **Gesamtlaufleistung** = Zählerwert + `offset_m`. Monotonie wird je Abschnitt geprüft.
5. **Aktueller Stand** = der Messpunkt mit dem spätesten `occurred_at` im aktuellen Abschnitt mit Status `valid` oder `confirmed_anomaly`. Es gilt ausdrücklich **nicht** das Maximum.
6. **Distanz zwischen zwei Zeitpunkten** = Differenz der Gesamtlaufleistung der jeweils zeitlich nächstgelegenen gültigen Messpunkte. Alle Module (Fuel, Oil, Costs, Maintenance) nutzen diese eine Funktion. Die widersprüchlichen Distanzdefinitionen der Altanwendung (D-12) entfallen damit.
7. **Umrechnungsfaktoren** (Tachoabweichung, Multiplikator) sind **nicht** im MVP. Ein fester Multiplikator wird bei Bedarf später als Eigenschaft eines Zählerabschnitts ergänzt.

## Konsequenzen
- (+) Nachvollziehbare Historie; eine einzige Definition von „Stand“ und „Distanz“.
- (+) Tachotausch fachlich sauber abgebildet.
- (−) Mehr Joins; Messpunkte sind zentrale Abhängigkeit aller Fachmodule (akzeptiert, Odometer hat keine Abhängigkeit zu anderen Fachmodulen).

## Bezug
Phase 1 BR-015 bis BR-025, BR-060, D-01, D-12, D-13; Auftrag 6.4.
