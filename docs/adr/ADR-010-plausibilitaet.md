# ADR-010 – Plausibilitätsprüfung: Warnen und bestätigen

- **Status:** akzeptiert · **Datum:** 2026-09-30 · **Entscheidung:** E-3

## Kontext
Die Altanwendung prüft Kilometerstände nicht (Phase 1, BR-021, im Test bestätigt). Der Auftrag verlangt: Verstöße ablehnen oder als Warnung zur Bestätigung vorlegen, nie stillschweigend übernehmen (6.4).

## Entscheidung
- Beim Anlegen oder Ändern eines Messpunkts prüft der Odometer-Service gegen die zeitlichen **Nachbarn** im selben Zählerabschnitt, nicht gegen das Maximum:
  - `P1` Wert kleiner als der vorherige Messpunkt → Anomalie „rückläufig“.
  - `P2` Wert größer als der nachfolgende Messpunkt → Anomalie „überholt Folgewert“.
  - `P3` Durchschnittsgeschwindigkeit zum Nachbarn > 250 km/h → Anomalie „unrealistischer Sprung“. Der Schwellwert ist konfigurierbar.
  - `P4` `occurred_at` liegt mehr als 5 Minuten in der Zukunft → Ablehnung.
- Eine Anomalie führt zu `422` mit Problem-Details-Typ `odometer-anomaly` und der Liste der Befunde. Der Client kann den Request mit `confirm_anomalies: ["P1", …]` wiederholen. Dann wird der Messpunkt mit `status = confirmed_anomaly` gespeichert und im Audit vermerkt (wer, wann, welche Befunde).
- Die Regeln gelten gleichermaßen für Web, Android, Import (dort gesammelt im Prüfbericht) und Assistent (Vorschlag zeigt Befunde vor der Bestätigung).
- Weitere Plausibilitätsregeln folgen demselben Muster: Fahrten überlappen (Trips), Tankmenge größer als der Tankinhalt laut Fahrzeugdaten (Fuel).

## Konsequenzen
- (+) Keine stillen Fehler; Nacherfassung alter Belege bleibt möglich.
- (−) Clients müssen den Bestätigungsdialog umsetzen. Dafür gibt es ein einheitliches Fehlerschema (ADR-013).

## Bezug
Phase 1 BR-021, D-01; Auftrag 6.4, 6.8, 6.13.
