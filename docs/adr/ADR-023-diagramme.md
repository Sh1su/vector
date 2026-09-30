# ADR-023 – Diagramme: Chart.js, lazy geladen

- **Status:** akzeptiert · **Datum:** 2026-09-30

## Kontext
Das Dashboard und die Auswertungen brauchen einige Diagramme: Verbrauch und Ölverbrauch als Zeitreihe, Kosten je Monat als (gestapelte) Balken, Anteile je Kategorie. Diagramme sollen nur dort stehen, wo sie einen Informationsgewinn bieten (Auftrag 6.15), und das Bundle-Budget nicht belasten.

## Optionen und Messung (Spike S-5, Liniendiagramm mit Achsen und Tooltip, JS gzip)
| Bibliothek | Größe | Diagrammtypen |
|---|---|---|
| Recharts | 165 KB (inkl. React) | viele |
| ECharts (modular) | 165 KB | sehr viele |
| **Chart.js** (nur registrierte Bausteine) | **52 KB** | Linie, Balken, gestapelt, Ring |
| uPlot | 22 KB | nur Zeitreihen |

## Entscheidung
**Chart.js**, ausschließlich mit den benötigten Bausteinen registriert und **lazy** geladen (eigener Chunk, nie im initialen Bundle).
- Jedes Diagramm hat eine **Tabellenalternative** mit denselben Zahlen (umschaltbar, für Screenreader immer vorhanden).
- Farben kommen aus den Design-Tokens (ADR-022); Reihen sind nicht allein durch Farbe unterscheidbar.
- Werte kommen fertig in der Anzeigeeinheit von der API (`DisplayValue`). Im Client wird nichts nachgerechnet (ADR-007).
- Wird eine reine Zeitreihe mit sehr vielen Punkten nötig (z. B. Kilometerstand über Jahre), wird uPlot gesondert geprüft. Das ist kein Teil des MVP.

## Konsequenzen
- (+) Ein Werkzeug für alle MVP-Diagramme bei ca. 52 KB, die nur bei Bedarf geladen werden.
- (−) Canvas-Diagramme sind nicht per DOM zugänglich; das gleicht die Tabellenalternative aus.

## Bezug
Auftrag 5.2, 5.3, 6.15; Spike S-5; ADR-022.
