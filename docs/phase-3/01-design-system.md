# Design-System Vectra (aus der Vorlage „Vectra Brand“)

- **Quelle:** Design-Leinwand „Vectra Brand“ des Auftraggebers (Markengrundlagen, Web-Entwürfe `Web · …`, App-Entwürfe, Icon-Set). Die Werte hier sind 1:1 übernommen und werden in `web/src/styles/tokens.css` als CSS-Variablen umgesetzt (ADR-022).
- **Markendateien:** `web/public/brand/` – Logo farbig und negativ, Querformat, Bildmarke farbig und negativ, Schriftzug negativ.

## Marke
- Claim: „Intelligent Vehicle Management“; Tonalität professionell, klar, auf Augenhöhe, Du-Form. Leitsatz: „Dein Auto ist bereit für die nächste Fahrt.“
- Werte: Zuverlässigkeit, Präzision, Übersicht, Modernität.

## Farben

| Token | Hell | Dunkel | Verwendung |
|---|---|---|---|
| `primary` (Deep Tech Blue) | #1E3A5F | – | Logo, Navigation, Kopfbereiche |
| `accent` (Motion Teal) | #0EA5E9 | #0EA5E9 | Primärbuttons (Text #0F172A), aktive Menüs |
| `success` (Ready) | #10B981 | – | Bestätigt |
| `warning` (Service Needed) | #F59E0B | – | Fällig, Hinweise |
| `bg` | #F8FAFC | #0F172A | Seitenhintergrund |
| `card` | #FFFFFF | #1E293B | Karten, Kopfzeile |
| `text` | #0F172A | #F8FAFC | Text |
| `muted` | #64748B | #94A3B8 | Nebentext |
| `border` | #E2E8F0 | #334155 | Rahmen |
| `soft` | #F1F5F9 | #172136 | Flächen |
| `link` | #0369A1 | #38BDF8 | Links, Icons |
| `info-bg` | #E0F2FE | rgba(14,165,233,.16) | Info-Flächen |
| `ok` / `ok-bg` | #047857 / #D1FAE5 | #34D399 / rgba(16,185,129,.16) | Status erledigt |
| `warn` / `warn-bg` | #92400E / #FEF3C7 | #FBBF24 / rgba(245,158,11,.16) | Status bald fällig |
| `bad` / `bad-bg` | #B91C1C / #FEE2E2 | #F87171 / rgba(239,68,68,.16) | Status überfällig, Fehler |
| `hero` | #1E3A5F | #172A45 | Hervorgehobene Karte |
| Seitenleiste | #1E3A5F | #0B1526 | Navigation (Text #CBD5E1, aktiv #F8FAFC auf rgba(14,165,233,.22), Icon aktiv #7DD3FC) |

Kontrastregel der Vorlage: Auf Teal, Grün und Amber steht dunkler Text (#0F172A); für Status-Text auf hellem Grund die dunklen Töne.

## Typografie
- **Poppins** 600 für Überschriften, Kennzahlen und Buttons; **Inter** 400/500/600 für Fließtext und Tabellen, Ziffern tabellarisch (`font-variant-numeric: tabular-nums`).
- Größen der Web-Entwürfe: Seitentitel 22 px, Kartenüberschrift 16 px, Kennzahl 26 px (Hervorhebung 40 px), Text 14–15 px, Nebentext 12–13 px.
- Schriften werden mit der Anwendung ausgeliefert (Pakete `@fontsource/*`, SIL OFL), nicht zur Laufzeit von Google geladen (ADR-028, keine externen Laufzeitabhängigkeiten).

## Formen
- Radien: Karten 16 px, Buttons und Felder 12 px, Listen-Icons 10 px, Status-Chips voll gerundet.
- Buttons 40–48 px hoch; Felder 48 px mit Icon links; Touch-Ziele ≥ 44 px.
- Layout Web: Seitenleiste 248 px, Kopfzeile 76 px, Inhalt mit 24/32 px Innenabstand, Raster mit 16–20 px Abstand.

## Icons
Eigenes Outline-Set (24er-Raster, Strich 1,8 px, ab 18 px Größe 2 px, runde Enden) mit 54 Symbolen: home, car, route, gauge, fuel, oil, wrench, euro, doc, sparkles, gear, more, plus, camera, play, stop, right, left, down, up, check, alert, calendar, clock, pin, user, users, lock, bell, shield, search, upload, download, edit, info, globe, moon, logout, key, send, mic, receipt, filter, chart, sync, image, x, garage, book, scan, trend, cloudoff, mail, ruler. Umgesetzt als React-Komponente `web/src/components/Icon.tsx`.

## Bildschirme (Web) und Stand der Umsetzung
| Entwurf | Iteration |
|---|---|
| Anmeldung, Übersicht (hell/dunkel), Fahrzeuge, Kilometerstand, Seitenleiste, Kopfzeile | 1 |
| Kraftstoff, Öl, Wartung, Servicehistorie, Kosten | 2 |
| Fahrten, Dokumente, Fahrzeugdetails, Einstellungen | 3 |
| Assistent | 7 |
