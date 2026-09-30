# ADR-031 – Fehler der Altanwendung korrigieren statt nachbauen

- **Status:** akzeptiert · **Datum:** 2026-09-30 · **Entscheidung:** E-7

## Kontext
Phase 1 hat Regeln der Altanwendung dokumentiert, die fachlich fehlerhaft oder widersprüchlich sind (u. a. BR-012, BR-022, BR-026, BR-029, BR-030, BR-038/BR-060, BR-044, D-06, D-11).

## Entscheidung
- Vectra spezifiziert jede Regel fachlich korrekt neu. Maßstab ist das fachlich richtige Ergebnis, nicht die Übereinstimmung mit LubeLogger.
- Der Soll-Regelkatalog (`docs/phase-2/20-regelkatalog-soll.md`) führt je Phase-1-Regel eine Klassifikation:
  - **KEEP** – Verhalten fachlich richtig, wird neu formuliert übernommen;
  - **FIX** – Verhalten wird korrigiert, mit Begründung und Beispiel „LubeLogger ergibt X, Vectra ergibt Y“;
  - **DROP** – Regel entfällt (z. B. weil die Funktion nicht im MVP ist).
- Der Import-Prüfbericht weist darauf hin, dass Kennzahlen wie Durchschnittsverbrauch oder Fälligkeiten nach dem Wechsel abweichen können, und verlinkt die Erklärung.
- Die Regressionstests prüfen das Soll-Verhalten. Ist-Werte der Altanwendung dienen nur als dokumentierte Vergleichswerte.

## Konsequenzen
- (+) Keine Übernahme bekannter Fehler; erklärbare Abweichungen.
- (−) Nutzer sehen nach der Migration teils andere Zahlen. Das wird erklärt, nicht versteckt.

## Bezug
Phase 1 04, 09 §2; ADR-000.
