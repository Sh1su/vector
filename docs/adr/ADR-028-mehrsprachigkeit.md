# ADR-028 – Mehrsprachigkeit (Deutsch und Englisch)

- **Status:** akzeptiert · **Datum:** 2026-09-30 · **Bezug Entscheidung:** E-0 / Q-16

## Kontext
Vectra unterstützt mindestens Deutsch und Englisch (Auftrag 5.3). Alle Texte werden neu verfasst; Übersetzungen der Altanwendung werden nicht übernommen (E-0, ADR-000). Die Altanwendung formatierte Zahlen und Daten kulturabhängig bis in API, CSV und Speicherung hinein (Phase 1, T-05). Außerdem lud sie Übersetzungen zur Laufzeit aus dem Internet (T-11).

## Entscheidung
- **Grundsatz:** Sprache und Region betreffen **nur die Darstellung**. API, Speicherung und Exporte sind sprachunabhängig (ISO 8601, Punkt als Dezimaltrenner, Einheitencodes; ADR-013).
- **Nachrichtenformat:** ICU MessageFormat (Plural, Auswahl, Zahlen- und Datumsplatzhalter) auf allen Plattformen.
- **Web:** Lingui. Die Kataloge werden zur Build-Zeit kompiliert, zur Laufzeit wird wenig Code geladen, und die Texte werden aus dem Quelltext extrahiert. Zahlen, Daten und Einheiten formatiert `Intl`.
- **Android:** String-Ressourcen mit Plural-Regeln; Formatierung mit `android.icu`.
- **Server** (E-Mails, Push-Texte, Prüfberichte): Go-Kataloge mit CLDR-Pluralregeln (`golang.org/x/text` bzw. eine vergleichbare Bibliothek, Auswahl in Phase 3). Die Sprache ist die Nutzereinstellung des Empfängers.
- **Schlüssel:** sprechend und plattformübergreifend gleich benannt (`fuel.fill.saved`), damit Übersetzungen abgeglichen werden können.
- **Sprachwahl:** Nutzereinstellung, sonst `Accept-Language`, sonst Installationsvorgabe (Default `en`).
- **Einheiten:** Namen und Kürzel (l/100 km, mpg) sind Teil der Kataloge. Die Umrechnung selbst ist nicht sprachabhängig (ADR-007).
- **Keine Laufzeitabhängigkeit** zu externen Diensten; alle Kataloge sind im Build bzw. Binary enthalten.
- **CI:** Vollständigkeit DE/EN je Plattform wird geprüft; fehlende Schlüssel lassen den Build fehlschlagen.
- **Weitere Sprachen** sind möglich, sobald jemand sie pflegt. Die Struktur ist darauf ausgelegt.

## Konsequenzen
- (+) Kein kulturabhängiges Parsen mehr (Klasse von Fehlern aus Phase 1 entfällt).
- (−) Drei Kataloge (Web, Android, Server); der gemeinsame Schlüsselraum und die CI-Prüfung halten sie synchron.

## Bezug
Auftrag 5.3; Phase 1 T-05, T-11; ADR-000, ADR-007, ADR-013.
