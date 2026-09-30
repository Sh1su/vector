# ADR-022 – Web-Design-System: shadcn/ui-Muster (Radix + Tailwind)

- **Status:** akzeptiert · **Datum:** 2026-09-30

## Kontext
Die Web-App ist eine statische React/TypeScript/Vite-SPA (Auftrag 5.2) mit einem Budget von 300 KB initialem JS (gzip) (5.3). Sie soll responsiv, barrierearm und einfach bedienbar sein (6.15) und das Erscheinungsbild der Marke Vectra tragen. Zur Wahl standen shadcn/ui und Mantine (Phase 1, Vorschlag ADR-022).

## Optionen und Messung (Spike S-5)
| | JS gzip | CSS gzip | Anpassbarkeit |
|---|---|---|---|
| Grundgerüst (React, Router, Query) | 104 KB | – | – |
| **shadcn/ui-Muster** (Radix-Primitives, Tailwind 4) | **144 KB** | **3 KB** | Komponenten als eigener Quelltext |
| Mantine | 172 KB | 33 KB | Theming über API |

## Entscheidung
**shadcn/ui-Muster.** Die Komponenten werden als eigener Quelltext ins Projekt übernommen und an Vectra angepasst. Das Verhalten (Fokus, Tastatur, ARIA) kommt von Radix-Primitives.
- **Design-Tokens** als CSS-Variablen (Farben, Abstände, Radien, Typografie) mit hellem und dunklem Modus, abgeleitet aus den Vectra-Markengrundlagen. Dieselben Token-Werte werden für Android (Material 3) exportiert, damit beide Clients gleich aussehen.
- **Barrierefreiheit:** Ziel WCAG 2.2 AA; automatisierte Prüfung (axe) in den Komponententests.
- **Formulare:** `react-hook-form`. Die Validierung übernimmt der Server; `422`-Fehler werden über `FieldError.pointer` den Feldern zugeordnet. Eine zweite Schemabibliothek gibt es nicht, die Typen kommen aus der OpenAPI (ADR-014).
- **Daten:** TanStack Query (Cache, Wiederholung, ETag-Handhabung im Wrapper); **Routing:** React Router.
- **Icons:** lucide (ISC-Lizenz), nur einzeln importiert.
- **Layout:** mobil zuerst; das Fahrzeug ist der zentrale Kontext (Fahrzeugwechsel in der Kopfzeile, Auftrag 6.15).
- **Budget-Prüfung in der CI:** Build schlägt fehl, wenn das initiale JS 300 KB gzip überschreitet. Routen und Diagramme werden lazy geladen.

## Konsequenzen
- (+) 28 KB JS und 30 KB CSS weniger als Mantine; volle Kontrolle über das Aussehen.
- (−) Komponenten-Updates kommen nicht automatisch; der Quelltext gehört dem Projekt und muss gepflegt werden.

## Bezug
Auftrag 5.2, 5.3, 6.15; Spike S-5 (`docs/phase-2/40-spikes.md`); ADR-014, ADR-023.
