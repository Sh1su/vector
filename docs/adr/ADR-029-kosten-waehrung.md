# ADR-029 – Kosten: Währung je Betrag, Teile/Arbeit getrennt

- **Status:** akzeptiert · **Datum:** 2026-09-30 · **Entscheidung:** E-8

## Kontext
Die Altanwendung speichert Beträge ohne Währung und einen einzigen Gesamtbetrag je Eintrag (Phase 1, M-5, MG-14). Vectra braucht eine konfigurierbare Währung (Auftrag 5.3) und eine getrennte Ausweisung von Teilen und Arbeit (6.10).

## Entscheidung
- **Geldbetrag** = `amount_minor` (`bigint`, kleinste Einheit, z. B. Cent) + `currency` (ISO 4217). Kein Gleitkomma.
- Default-Währung je Nutzer, voreingestellt aus der Installation. Beträge in Fremdwährung sind erlaubt. Auswertungen gruppieren nach Währung, eine **automatische Umrechnung** ist nicht im MVP.
- **Serviceeinträge** haben Kostenpositionen mit `kind` ∈ {`parts`, `labor`, `other`} und optional eine Teileliste (Bezeichnung, Teilenummer, Menge, Einzelpreis).
- **Tankvorgänge:** Gesamtbetrag ist führend; ein Preis pro Einheit wird serverseitig berechnet und nur bei Eingabe als Preis/Einheit gespeichert (`input_price_per_unit`), mit Rundung auf Cent erst beim Gesamtbetrag.
- **Kostenkategorien** im Modul Costs: Kraftstoff/Energie, Wartung, Reparatur, Upgrade, Steuer, Versicherung, Gebühr, Sonstiges. Wiederkehrende Kosten (z. B. Kfz-Steuer) werden als Plan mit fälligen Vorkommen modelliert und erzeugen **keine** Einträge im Hintergrund, ohne dass es der Nutzer sieht (korrigiert BR-035/D-11).
- **Migration:** Altbeträge werden mit der im Import gewählten Währung als `other` bzw. als Gesamtbetrag übernommen.

## Konsequenzen
- (+) Exakte Beträge, internationale Nutzung möglich.
- (−) Summen über Währungen hinweg werden getrennt angezeigt.

## Bezug
Auftrag 5.3, 6.10; Phase 1 BR-014, BR-035–BR-041, M-5, D-11.
