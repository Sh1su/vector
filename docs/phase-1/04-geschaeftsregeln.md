# 04 – Geschäfts- und Berechnungsregeln (Regelkatalog)

Quellenbezug: `hargata/lubelog` @ `dd69e59` (v1.7.3). Jede Regel hat genau einen Gesamtstatus; einzelne Teilaussagen können abweichend markiert sein. „(ausgeführt)“ = durch Ausführung der Altanwendung v1.7.3 bestätigt.

Konventionen in den Formeln: `Δ` = gefahrene Distanz, `V` = Menge (Liter/Gallonen/kWh), `D` = Distanzeinheit laut Einstellung. Regressionstests sind als Vorschlag formuliert; „Soll“ bezeichnet das **Ist-Verhalten der Altanwendung**, nicht zwingend das gewünschte Verhalten der Neuimplementierung. Abweichungen werden in Phase 2 per ADR entschieden.

Übersicht:

| Bereich | Regeln |
|---|---|
| Kraftstoff/Verbrauch | BR-001 – BR-014, BR-059 |
| Kilometerstand | BR-015 – BR-025, BR-060 |
| Reminder | BR-026 – BR-034 |
| Steuern/Gebühren | BR-035 |
| Kosten/Reporting | BR-036 – BR-041, BR-048 |
| Lager/Planer/Inspektion | BR-042 – BR-047 |
| Stammdaten/Sonstiges | BR-049 – BR-058 |

---

## Kraftstoff und Verbrauch

### BR-001 – Sortierung der Tankvorgänge vor jeder Berechnung
- **Beschreibung:** Vor der Verbrauchsberechnung werden alle Tankvorgänge eines Fahrzeugs sortiert.
- **Eingaben:** Liste `GasRecord` (Date, Mileage, EndingSoc).
- **Algorithmus:** `OrderBy(Date).ThenBy(Mileage).ThenBy(EndingSoc)`.
- **Randfälle:** Gleiche Datum/Kilometer-Kombination → Reihenfolge nach Ladezustand; Einträge ohne Kilometerstand (0) desselben Tages kommen zuerst.
- **Quelle:** `Helper/GasHelper.cs:37` (`GetGasRecordViewModels`).
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Records (2026-01-02, 1500), (2026-01-01, 2000) → Verarbeitungsreihenfolge 01-01 vor 01-02, obwohl 2000 > 1500; Δ des zweiten = max(0, 1500 − 2000) = 0.

### BR-002 – Delta-Distanz zwischen Tankvorgängen
- **Beschreibung:** Distanz seit dem letzten Tankvorgang mit bekanntem Kilometerstand.
- **Eingaben:** `Mileage` (D, int) des aktuellen und des letzten Eintrags mit `Mileage ≠ 0`.
- **Formel:** `Δ = Mileage − previousMileage`; bei `Δ < 0` → `Δ = 0`. `previousMileage` wird nur aktualisiert, wenn `Mileage ≠ 0`. Erster Eintrag: `Δ = 0`.
- **Randfälle:** Tachorücksprung/Tippfehler → Δ = 0 ohne Warnung; fehlender Kilometerstand (0) → Δ = max(0, 0 − prev) = 0.
- **Quelle:** `GasHelper.cs:160-166, 241-244` (Verbrenner), `68-72, 138-141` (EV).
- **Status:** [VERIFIZIERT]
- **Regressionstest:** (1000), (900), (1500) → Δ = 0, 0, 600 (Bezug ist der letzte Wert ≠ 0, also 900). *Erratum (Phase 2, AP-5): Die Erstfassung nannte 500; laut `GasHelper.cs:160-166, 241-244` wird `previousMileage` auch nach einem negativen Delta auf 900 gesetzt, das Ergebnis ist also 600.*

### BR-003 – Verbrauch bei Vollbetankung (Verbrenner)
- **Beschreibung:** Verbrauch wird nur bei einer Vollbetankung mit Kilometerstand berechnet, unter Einbeziehung vorher akkumulierter Teilbetankungen (BR-004).
- **Eingaben:** Δ (D), `V` (Menge), akkumulierte `ΔAkk`, `VAkk`, `UseMPG`.
- **Formel:** `effizienz = (ΔAkk + Δ) / (VAkk + V)` (D pro Mengeneinheit). Bei `UseMPG = true` wird `effizienz` angezeigt (mpg bzw. mi/imp gal), sonst `100 / effizienz` (l/100 km). Danach `ΔAkk = VAkk = 0`.
- **Bedingung:** `!MissedFuelUp && IsFillToFull && Mileage ≠ 0 && V > 0 && Δ > 0`; sonst Wert 0.
- **Randfälle:** Division durch 0 abgefangen → 0; bei `V ≤ 0` oder `Δ ≤ 0` wird trotzdem akkumuliert zurückgesetzt (Teilmengen gehen verloren); Anzeigeformat 2 oder 3 Nachkommastellen (`UseThreeDecimalGasConsumption`).
- **Quelle:** `GasHelper.cs:193-210`.
- **Status:** [VERIFIZIERT] (ausgeführt)
- **Regressionstest (ausgeführt):** (2026-01-01, 1000, 40, voll), (01-15, 1500, 20, teil), (02-01, 2000, 30, voll) → Eintrag 3: `UseMPG=true` → 20,00; `UseMPG=false` → 5,00.

### BR-004 – Akkumulation von Teilbetankungen
- **Beschreibung:** Teilbetankungen (`IsFillToFull = false`) oder Vollbetankungen ohne Kilometerstand liefern keinen Verbrauch; Menge und Distanz werden bis zur nächsten gültigen Vollbetankung aufsummiert.
- **Formel:** `VAkk += V`, `ΔAkk += Δ`, Anzeige 0.
- **Randfälle:** Teilbetankung als letzter Eintrag → Menge fließt nie in einen Einzelwert ein, wohl aber in den Durchschnitt (BR-011).
- **Quelle:** `GasHelper.cs:211-216`.
- **Status:** [VERIFIZIERT] (ausgeführt, siehe BR-003)
- **Regressionstest:** siehe BR-003 (Eintrag 2 → 0, Eintrag 3 → 20).

### BR-005 – Verpasste Betankung (Missed Fuel-Up)
- **Beschreibung:** Markiert, dass eine vorherige Betankung nicht erfasst wurde; die Berechnung wird zurückgesetzt.
- **Formel:** Verbrauch = 0; `ΔAkk = VAkk = 0` (Verbrenner). Bei EV nur Verbrauch = 0 (kein Akkumulator).
- **Randfälle:** Die Menge dieses Eintrags wird verworfen; Δ des Eintrags zählt nicht zum Durchschnitt (`IncludeInAverage = false`).
- **Quelle:** `GasHelper.cs:185-192` (Verbrenner), `93-97` (EV).
- **Status:** [VERIFIZIERT] (ausgeführt)
- **Regressionstest (ausgeführt):** nach BR-003-Daten: (02-15, 2400, 25, voll, missed) → 0; (03-01, 2800, 20, voll) → 20 mpg (400/20).

### BR-006 – Erster Tankvorgang
- **Beschreibung:** Der zeitlich erste Tankvorgang hat keinen Vorwert.
- **Formel:** Δ = 0, Verbrauch = 0; er gilt nicht als Teilbetankung. Seine Menge wird **nicht** akkumuliert.
- **Quelle:** `GasHelper.cs:219-240` (Verbrenner), `114-137` (EV).
- **Status:** [VERIFIZIERT] (ausgeführt: Eintrag 1 → `fuelEconomy: 0`)
- **Regressionstest:** Ein einziger Eintrag → Verbrauch 0, Durchschnitt „0“ (BR-011: `IncludeInAverage` false, da voll und Kilometerstand ≠ 0).

### BR-007 – Tankvorgang ohne Kilometerstand (Odometer optional)
- **Beschreibung:** Fahrzeuge mit `OdometerOptional` erlauben `Mileage = 0`.
- **Formel:** Eintrag wird wie Teilbetankung akkumuliert (BR-004), `previousMileage` bleibt unverändert; `IncludeInAverage = true` (sofern nicht Missed).
- **Randfälle:** Durchschnitt enthält die Menge, aber Δ = 0 für diesen Eintrag – die Distanz kommt erst mit dem nächsten Eintrag mit Kilometerstand.
- **Quelle:** `GasHelper.cs:193, 241-244`; `Models/GasRecord/GasRecordViewModel.cs:29`; Client setzt `'0'` bei leerem Feld (`wwwroot/js/gasrecord.js:98-101`).
- **Status:** [VERIFIZIERT]
- **Regressionstest:** (1000, 40, voll), (0, 20, voll), (2000, 30, voll) → Eintrag 3 = (0+1000)/(20+30) = 20.

### BR-008 – UK-Gallonen („UK MPG“)
- **Beschreibung:** Bei `UseUKMPG = true` **und** `UseMPG = true` wird die gespeicherte Menge als Liter interpretiert und in imperiale Gallonen umgerechnet.
- **Formel:** `V_imp = V_l / 4.546`; Verbrauch = Δ(mi) / V_imp. Angezeigte/exportierte Menge ist `V_imp`.
- **Randfälle:** `UseUKMPG = true`, `UseMPG = false` → Menge in l, Distanz in Meilen, Anzeige „l/100mi.“; EV ist ausgenommen (Export setzt `useUKMPG = !IsElectric && …`). Faktor 4,546 statt exakt 4,54609.
- **Quelle:** `GasHelper.cs:150-159`; `StaticHelper.GetFuelEconomyUnit` (`StaticHelper.cs:302-309`); `ImportController.cs:618`.
- **Status:** [VERIFIZIERT] (ausgeführt)
- **Regressionstest (ausgeführt):** BR-003-Daten mit `useMPG=true&useUKMPG=true` → Eintrag 3: 90,92; Menge Eintrag 1: 8,7989… statt 40.

### BR-009 – Verbrauch Elektrofahrzeug (State of Charge)
- **Beschreibung:** Bei `Vehicle.IsElectric` wird der Verbrauch aus Ladezuständen abgeleitet, nicht aus der geladenen Energiemenge.
- **Eingaben:** `V` = geladene kWh, `StartingSoc`, `EndingSoc` (%), `EndingSoc` des vorherigen Eintrags, Δ.
- **Formel:** `Kapazität = V / ((EndingSoc − StartingSoc)/100)`; `Verbrauch_kWh = (prevEndingSoc − StartingSoc)/100 × Kapazität`, bei < 0 → 0, bei `prevEndingSoc = 0` → 0; Effizienz = `Δ / Verbrauch_kWh` (`UseMPG`) bzw. `100 / (Δ / Verbrauch_kWh)`.
- **Randfälle:** `EndingSoc = StartingSoc` → Division durch 0 → Verbrauch 0; `IsFillToFull` wird ignoriert; **keine** Akkumulation über Einträge (die Akkumulatoren bleiben 0); angezeigte „Menge“ ist der berechnete Verbrauch, nicht die geladene Energie.
- **Quelle:** `GasHelper.cs:47-146`.
- **Status:** [VERIFIZIERT] (Code gelesen, nicht ausgeführt)
- **Regressionstest:** EV: (Tag 1, 1000 km, 30 kWh, 20→80 %), (Tag 2, 1200 km, 20 kWh, 40→80 %) → Kapazität₂ = 20/0,4 = 50 kWh; Verbrauch₂ = (0,8−0,4)×50 = 20 kWh; Effizienz = 200/20 = 10 km/kWh bzw. 10 kWh/100 km.

### BR-010 – Preis pro Mengeneinheit
- **Formel:** `CostPerGallon = Cost / V` (bei UK: `Cost / V_imp`; bei EV: `Cost / geladene kWh`); `V ≤ 0` → 0.
- **Quelle:** `GasHelper.cs:83, 127, 177, 232`.
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Cost 50, V 40 → 1,25.

### BR-011 – Durchschnittsverbrauch (gewichtet)
- **Beschreibung:** Gesamtdurchschnitt über alle berücksichtigten Einträge.
- **Auswahl (`IncludeInAverage`):** `MilesPerGallon > 0` **oder** (Teilbetankung und nicht Missed) **oder** (`Mileage = 0` und nicht Missed).
- **Formel:** `avg = ΣΔ / ΣV` über die Auswahl; bei `UseMPG = false` und `avg > 0`: `avg = 100 / avg`; Ausgabe mit Format `"F"` (2 Nachkommastellen, kulturabhängiges Dezimalzeichen).
- **Randfälle:** Keine Auswahl → „0“; ΣV = 0 → Exception abgefangen → „0“; erster Eintrag und Missed-Einträge fließen nicht ein.
- **Quelle:** `GasHelper.cs:12-33`; `GasRecordViewModel.cs:29`.
- **Status:** [VERIFIZIERT] (ausgeführt: Bericht zeigt „5.00 l/100km“)
- **Regressionstest (ausgeführt):** 5 Einträge aus BR-003/BR-005 → berücksichtigt Einträge 2, 3, 5: ΣΔ = 1400, ΣV = 70 → 20 km/l → „5.00 l/100km“.

### BR-012 – Verbrauch je Monat (Diagramm)
- **Formel:** Je Monat (1–12) ungewichteter Mittelwert der Einzelwerte `MilesPerGallon > 0`; ohne Jahresfilter werden gleiche Monate **verschiedener Jahre zusammengefasst**.
- **Randfälle:** Monate ohne Werte → 0; Mittelwert von l/100km-Einzelwerten ist nicht gleich dem gewichteten Mittel (inkonsistent zu BR-011).
- **Quelle:** `ReportController.cs:124-141` (`GetReportPartialView`), `687-730` (`GetMonthMPGByVehicle`).
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Jan 2025: 5,0; Jan 2026: 7,0 (l/100km), year=0 → Januar = 6,0.

### BR-013 – Umkehrung l/100km ↔ km/l
- **Formel:** Wenn Anzeigeeinheit `l/100km` und Präferenz `km/l`: Wert = `100 / Wert` (Wert ≠ 0).
- **Quelle:** `ReportController.cs:129-157, 231-241, 625-635`; Client: `wwwroot/js/gasrecord.js:284-300`.
- **Status:** [VERIFIZIERT]
- **Regressionstest:** 5,00 l/100km → 20,00 km/l.

### BR-014 – Eingabe Stückpreis statt Gesamtkosten (nur Client)
- **Formel:** Bei Kostenart „unit“: `Cost = round(Preis × V, 2)`; erst dann an Server gesendet.
- **Randfälle:** Rundung im Browser (`toFixed(2)`); API/CSV haben eigene Logik (CSV: `Cost = V × Price` ohne Rundung, BR-052).
- **Quelle:** `wwwroot/js/gasrecord.js:136-147`.
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Preis 1,799, Menge 40,12 → Cost 72,18.

### BR-059 – Einheitenmodell und Anzeigeeinheiten
- **Beschreibung:** Einheiten sind reine Interpretation; es gibt keine Umrechnung gespeicherter Werte außer UK-Gallonen (BR-008) und Anzeige-Präferenzen im Browser.
- **Zuordnung:**

| Bedingung | Distanz | Menge (gespeichert) | Verbrauchseinheit |
|---|---|---|---|
| EV | mi. (`UseMPG`) / km | kWh | `mi./kWh` bzw. `kWh/100km` |
| `UseMPG` ∧ `UseUKMPG` | mi. | **Liter** (Anzeige imp gal) | mpg (UK) |
| ¬`UseMPG` ∧ `UseUKMPG` | mi. | Liter | l/100mi. |
| `UseMPG` | mi. | US gal | mpg |
| sonst | km | Liter | l/100km |
| `Vehicle.UseHours` | h statt Distanz | wie oben | h/g bzw. l/100h |

- **Anzeige-Präferenzen (nur Client):** `PreferredGasUnit` rechnet angezeigte Mengen mit Faktoren 3,785 (US gal↔l) und 4,546 (imp gal↔l) um; `PreferredGasMileageUnit` erlaubt `km/l`.
- **Randfälle:** Wechselt ein Nutzer `UseMPG`, werden Bestandsdaten nicht konvertiert; geteilte Fahrzeuge werden je Nutzer anders beschriftet; API-Default `useMPG = false` weicht vom UI-Default `UseMPG = true` ab.
- **Quelle:** `StaticHelper.GetFuelEconomyUnit` (`StaticHelper.cs:294-315`), `Views/Vehicle/Gas/_Gas.cshtml:24-45`, `wwwroot/js/gasrecord.js:176-300`, `Models/API/MethodParameter.cs` (`UseMPG` ohne Default), `appsettings.json` (`UseMPG: true`).
- **Status:** [VERIFIZIERT] (ausgeführt: API ohne Parameter liefert l/100km)
- **Regressionstest:** Gleiche Daten, `/api/vehicle/gasrecords?vehicleId=1` → 5; `&useMPG=true` → 20.

---

## Kilometerstand (Odometer)

### BR-015 – Aktueller Kilometerstand eines Fahrzeugs („Max Mileage“)
- **Beschreibung:** Der „aktuelle“ Kilometerstand ist das Maximum über mehrere Record-Typen.
- **Formel:** `max(Service.Mileage, Repair.Mileage, Gas.Mileage, Upgrade.Mileage, Odometer.Mileage)`; keine Records → 0.
- **Randfälle:** Inspektionen und Planer-Einträge werden nicht berücksichtigt; ein einzelner Tippfehler nach oben bestimmt dauerhaft den Stand; Tachokorrektur wird nicht angewandt; zeitliche Reihenfolge spielt keine Rolle.
- **Quelle:** `Logic/VehicleLogic.cs:107-161` (`GetMaxMileage`).
- **Status:** [VERIFIZIERT] (ausgeführt: Odometer-Eintrag 500 nach Tankvorgang 2800 → „latest“ = 2800)
- **Regressionstest:** Gas 2800, Odometer 500 → 2800.

### BR-016 – Minimaler Kilometerstand
- **Formel:** Minimum der o. g. Typen, jeweils nur Werte `≠ 0`; keine → 0.
- **Quelle:** `VehicleLogic.cs:162-221`.
- **Status:** [VERIFIZIERT] (ausgeführt: Kosten-Tabelle nutzt 2800 − 500 = 2300)
- **Regressionstest:** Werte 0, 500, 1000 → 500.

### BR-017 – Letzter Odometer-Stand
- **Formel:** `max(OdometerRecord.Mileage)` der Odometer-Einträge des Fahrzeugs, 0 wenn keine.
- **Verwendung:** `InitialMileage` bei Neuanlage (UI-Vorbelegung, API, Auto-Insert).
- **Quelle:** `Logic/OdometerLogic.cs:273-285`.
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Odometer 1000, 3000, 2000 → 3000.

### BR-018 – InitialMileage bei automatisch erzeugten Odometer-Einträgen
- **Formel:** `InitialMileage = letzter Odometer-Stand (BR-017)`, falls ≠ 0, sonst `= Mileage`. Einträge mit `Mileage = 0` werden nicht erzeugt. Alle aktuell montierten Ausstattungen (`IsEquipped`) werden verknüpft.
- **Auslöser:** Neuanlage von Service/Repair/Upgrade/Gas/Inspection (UI, API, CSV) bei `EnableAutoOdometerInsert`, Plan → Done, Bulk-Aktion „Create Odometer Records“.
- **Randfälle:** Rückdatierter Eintrag erhält trotzdem den höchsten Stand als Initialwert → negative Distanz möglich.
- **Quelle:** `OdometerLogic.cs:286-303`; Aufrufer z. B. `Controllers/Vehicle/ServiceController.cs:62-72`, `Controllers/API/GasController.cs:185-196`.
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Odometer-Max 3000; neuer Service mit 2500 → Odometer-Eintrag Initial 3000, Mileage 2500, Distanz −500.

### BR-019 – Neuberechnung der Distanzen („Recalculate“)
- **Formel:** Sortierung nach `Date`, dann `Mileage`; erster Eintrag `InitialMileage = Mileage`; jeder weitere `InitialMileage = Mileage des Vorgängers`. Jeder Eintrag wird gespeichert.
- **Auslöser:** manuell (UI/API) und automatisch beim Laden, wenn **alle** Einträge `InitialMileage = 0` haben (Legacy-Konvertierung).
- **Randfälle:** Vorgänger mit `Mileage = 0` → nächster wird wie „erster“ behandelt (`previousMileage == default`); Rücksprünge ergeben negative Distanzen; nicht transaktional.
- **Quelle:** `OdometerLogic.cs:304-327`; `Controllers/Vehicle/OdometerController.cs:11-21, 27-31`; `Controllers/API/OdometerController.cs:28-39, 95-99`.
- **Status:** [VERIFIZIERT]
- **Regressionstest:** (01-01, 1000), (02-01, 1500), (03-01, 1400) → Initial 1000/1000/1500, Distanz 0/500/−100.

### BR-020 – Gefahrene Distanz eines Odometer-Eintrags
- **Formel:** `DistanceTraveled = Mileage − InitialMileage` (int, kann negativ sein).
- **Quelle:** `Models/OdometerRecord/OdometerRecord.cs` (berechnete Property).
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Initial 1500, Mileage 1400 → −100.

### BR-021 – Keine Plausibilitätsprüfung für Kilometerstände
- **Beschreibung:** Weder UI noch API noch CSV-Import prüfen Monotonie gegenüber Datum oder Vorwerten. Einzige Prüfung: Client verlangt Zahl ≥ 0.
- **Randfälle:** Rückläufige Stände, Stände in der Zukunft, Duplikate werden ohne Warnung gespeichert.
- **Quelle:** Kein serverseitiger Vergleich in `Controllers/**` (Suche nach Vergleichsoperatoren auf `Mileage`); Client `wwwroot/js/gasrecord.js:108-113`.
- **Status:** [VERIFIZIERT] (ausgeführt: Odometer 500 nach 2800 → `success: true`)
- **Regressionstest:** POST Odometer 500 nach bestehendem 2800 → Ist: akzeptiert. (Neuimplementierung: Ablehnung oder Bestätigungspflicht, siehe Auftrag 6.4.)

### BR-022 – Tachokorrektur (Odometer Multiplier / Difference)
- **Beschreibung:** Fahrzeugbezogene Korrektur für abweichende Tachos (z. B. Tachotausch, falsche Übersetzung).
- **Eingaben:** Rohwert `odo` (int), `OdometerDifference` (int, als String gespeichert), `OdometerMultiplier` (decimal, als String gespeichert), `HasOdometerAdjustment`.
- **Formel:** `korrigiert = (odo + Difference) × Multiplier`.
- **Rundung – drei Varianten:**
  1. **UI-Neuanlage:** im Browser, `toFixed(0)` (kaufmännisch); **nur** bei neuen Einträgen (`id = 0`), nicht beim Bearbeiten. `wwwroot/js/vehicle.js:537-547`, Aufruf z. B. `gasrecord.js:102`.
  2. **Bulk „Adjust Odometer“:** serverseitig `decimal.ToInt32` (Abschneiden). `Controllers/VehicleController.cs:868-924`.
  3. **API `/api/vehicle/adjustedodometer`:** unrundeter Dezimalwert. `Controllers/APIController.cs:256-267`.
- **Randfälle:** API-, CSV- und Auto-Insert-Pfade wenden keine Korrektur an; keine Historie, ab wann eine Korrektur gilt (kein Tachotausch-Datum); Multiplier wird kulturabhängig geparst.
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Difference 100, Multiplier 1,6: UI-Neuanlage mit Rohwert 1000 → 1760; Bulk-Anpassung eines gespeicherten Werts 999 → 1099 × 1,6 = 1758,4 → 1758 (abgeschnitten); `GET /api/vehicle/adjustedodometer?odometer=999` → 1758,4.

### BR-023 – Distanz auf andere Fahrzeuge übertragen
- **Beschreibung:** Distanzen ausgewählter Odometer-Einträge (z. B. Zugfahrzeug) werden als neuer Eintrag auf andere Fahrzeuge (z. B. Anhänger) übertragen.
- **Formel:** `Summe = Σ DistanceTraveled` der Quelleinträge, `Stichtag = max(Date)`; je Ziel: `Basis = max(Mileage)` der Zieleinträge mit `Date ≤ Stichtag` (sonst 0); neuer Eintrag `Initial = Basis`, `Mileage = Basis + Summe`, `Date = Stichtag`. Optional „shift“: alle späteren Zieleinträge `Mileage += Summe`, danach Neuberechnung (BR-019).
- **Randfälle:** Quelleinträge ohne Leserecht werden stillschweigend ignoriert; negative Distanzen werden mit übertragen.
- **Quelle:** `Controllers/Vehicle/OdometerController.cs:210-276`.
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Quelle Distanzen 100 + 50 (bis 03-01); Ziel hat 2000 (02-01) und 2100 (04-01), shift=true → neuer Eintrag 2000→2150 am 03-01, späterer Eintrag 2250.

### BR-024 – Laufleistung von Ausstattung (Equipment)
- **Formel:** `Distanz(Equipment) = Σ DistanceTraveled` aller Odometer-Einträge, deren `EquipmentRecordId` die Equipment-ID enthält.
- **Randfälle:** Negative Distanzen reduzieren die Laufleistung; Equipment ohne Odometer-Einträge → 0.
- **Quelle:** `Helper/EquipmentHelper.cs:357-389`.
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Odometer-Einträge mit Distanzen 500 (Equipment 1), 300 (Equipment 1, 2) → Equipment 1: 800, Equipment 2: 300.

### BR-025 – Automatische Verknüpfung montierter Ausstattung
- **Formel:** Neue automatisch erzeugte Odometer-Einträge (BR-018) und API-Einträge mit `autoIncludeEquipment=true` erhalten alle Equipment-IDs mit `IsEquipped = true`.
- **Quelle:** `OdometerLogic.cs:294-300`; `Controllers/API/OdometerController.cs:190-200`.
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Equipment A (montiert), B (nicht) → Auto-Eintrag verknüpft nur A.

### BR-060 – „Distance Traveled“ im Report-Kopf
- **Formel:** `Σ DistanceTraveled` aller Odometer-Einträge (gefiltert nach Jahr). Unterscheidet sich von der Distanz der Kostentabelle (`max − min`, BR-038).
- **Quelle:** `ReportController.cs:171, 252`.
- **Status:** [VERIFIZIERT] (ausgeführt: Kopf zeigt 0, Kostentabelle 2300 für dieselben Daten)
- **Regressionstest:** Nur Tankvorgänge 1000–2800 und ein Odometer-Eintrag 500/500 → Kopf 0, Kostentabelle 2300.

---

## Reminder

### BR-026 – Dringlichkeit bei Metrik „Datum“
- **Formel (Vergleich mit `jetzt` inkl. Uhrzeit):** `Date < jetzt` → PastDue; sonst `Date < jetzt + VeryUrgentDays` → VeryUrgent; sonst `Date < jetzt + UrgentDays` → Urgent; sonst NotUrgent. `DueDays = (Date − jetzt).Days` (abgeschnitten).
- **Randfälle:** Fälligkeit „heute“ (Mitternacht) ist ab 00:00:01 bereits PastDue; DueDays ist um 1 kleiner als die Kalendertage (ausgeführt: 200 Tage → 199).
- **Quelle:** `Helper/ReminderHelper.cs:137-152`.
- **Status:** [VERIFIZIERT] (ausgeführt)
- **Regressionstest:** jetzt = 2026-09-30 12:00, Date = 2026-10-05 → VeryUrgent (Default 7 Tage), DueDays = 4.

### BR-027 – Dringlichkeit bei Metrik „Kilometerstand“
- **Formel:** `aktuell` = BR-015. `Mileage < aktuell` → PastDue; `Mileage < aktuell + VeryUrgentDistance` → VeryUrgent; `Mileage < aktuell + UrgentDistance` → Urgent; sonst NotUrgent. `DueMileage = Mileage − aktuell`.
- **Randfälle:** Gleichstand (`Mileage = aktuell`) ist nicht PastDue; Schwellen in „Distanzeinheiten“ unabhängig von km/mi.
- **Quelle:** `ReminderHelper.cs:153-169`.
- **Status:** [VERIFIZIERT] (ausgeführt: Mileage 2850 bei aktuell 2800 → Urgent, DueDistance 50)
- **Regressionstest:** aktuell 2800, Fälligkeit 2849 → VeryUrgent; 2850 → Urgent; 2900 → NotUrgent.

### BR-028 – Dringlichkeit bei Metrik „Beides“
- **Formel (Reihenfolge ist maßgeblich):** Datum überfällig → PastDue(Date); km überfällig → PastDue(Odometer); Datum < VeryUrgent → VeryUrgent(Date); km < VeryUrgent → VeryUrgent(Odometer); Datum < Urgent → Urgent(Date); km < Urgent → Urgent(Odometer); sonst NotUrgent. Beide `DueDays` und `DueMileage` werden gesetzt. Es gilt „was zuerst erreicht wird“.
- **Quelle:** `ReminderHelper.cs:100-136`.
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Datum in 20 Tagen, km-Rest 40 → VeryUrgent mit Metric=Odometer (Datum wäre nur Urgent).

### BR-029 – Dringlichkeitsschwellen
- **Beschreibung:** Server-Defaults `UrgentDays = 30`, `VeryUrgentDays = 7`, `UrgentDistance = 100`, `VeryUrgentDistance = 50`, überschreibbar über `ReminderUrgencyConfig`; pro Reminder überschreibbar (`UseCustomThresholds`).
- **Fehlverhalten (Bug):** Die Variable mit den Schwellen wird in der Schleife überschrieben und nicht zurückgesetzt; ab dem ersten Reminder mit eigenen Schwellen gelten diese für **alle folgenden** Reminder der Liste.
- **Quelle:** `Models/Reminder/ReminderConfig.cs`; `ReminderHelper.cs:80-86`.
- **Status:** [VERIFIZIERT] (ausgeführt)
- **Regressionstest (ausgeführt):** Reminder A (eigene Schwelle 365 Tage, fällig in 200 Tagen), danach Reminder B (Default, fällig in 100 Tagen) → Ist: B = **Urgent**; korrekt wäre NotUrgent.

### BR-030 – Nächster Reminder (Fahrzeug-Info/Kiosk)
- **Formel:** Wenn es Reminder mit Metrik Date/Both und `Date ≥ heute` gibt: frühester nach Datum unter allen Remindern mit `Date ≥ heute` (Metrik wird beim Auswählen nicht mehr gefiltert); sonst frühester nach Kilometer unter Remindern mit `Mileage ≥ aktuell`.
- **Quelle:** `VehicleLogic.cs:323-331`.
- **Status:** [VERIFIZIERT] (Fehlauswahl eines reinen Kilometer-Reminders mit zufälligem Datum ist möglich: [ABGELEITET])
- **Regressionstest:** Date-Reminder in 30 Tagen, Odometer-Reminder mit (UI-Default) Datum morgen → Ist: Odometer-Reminder wird als „nächster“ gewählt.

### BR-031 – Fortschreibung wiederkehrender Reminder
- **Eingaben:** Reminder, optional Erledigungsdatum `d` und -stand `m`.
- **Formel:** Basis `B_Datum = FixedIntervals ? Date : (d ?? Date)`, `B_km = FixedIntervals ? Mileage : (m ?? Mileage)`. Datum (Metrik Date/Both): Enum-Intervall → `B_Datum.AddMonths(n)`; `Other` → `AddMonths(Custom)` bzw. `AddDays(Custom)` je Einheit. Kilometer (Odometer/Both): `B_km + Intervall` (Enum-Wert oder Custom).
- **Randfälle:** Monatsende-Semantik von `AddMonths` (31.01. + 1 Monat = 28./29.02.); bei Tax-Pushback ist `m = null` → Kilometer vom alten Fälligkeitswert; kein Schutz gegen mehrfaches Fortschreiben.
- **Quelle:** `ReminderHelper.cs:17-76`.
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Both, 12 Monate/15 000, Service am 2026-03-10 bei 45 000, FixedIntervals=false → 2027-03-10 / 60 000; FixedIntervals=true (alt 2026-04-01/50 000) → 2027-04-01 / 65 000.

### BR-032 – Automatische Fortschreibung überfälliger Reminder
- **Beschreibung:** Bei `EnableAutoReminderRefresh` (Benutzereinstellung) werden beim Abruf des Dringlichkeits-Status überfällige wiederkehrende Reminder **einmal** um ein Intervall ab dem alten Fälligkeitswert verschoben – nur wenn der abrufende Nutzer Edit-Recht hat.
- **Randfälle:** Stark überfällige Reminder bleiben nach einem Schritt ggf. überfällig; der Zustand hängt davon ab, welcher Nutzer die Seite öffnet (Schreibzugriff bei GET).
- **Quelle:** `Controllers/Vehicle/ReminderController.cs:17-47`.
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Reminder 12 Monate fällig 2024-01-01, heute 2026-09-30 → nach Abruf 2025-01-01 (weiterhin PastDue).

### BR-033 – Reminder-Pushback durch erledigende Einträge
- **Beschreibung:** Beim Speichern eines Service-/Repair-/Upgrade-/Inspektions-Eintrags (mit Datum und km), eines Tax-Eintrags (nur Datum) oder beim Verschieben eines Plans nach „Done“ (jetzt, eingegebener km-Stand) werden ausgewählte wiederkehrende Reminder nach BR-031 fortgeschrieben.
- **Randfälle:** Fortschreibung erfolgt **vor** dem Speichern des Eintrags und auch, wenn dieses fehlschlägt; die Verknüpfung wird nicht gespeichert (keine Nachvollziehbarkeit, welcher Service welchen Reminder erledigt hat); nicht-wiederkehrende Reminder werden ignoriert; bearbeitete Einträge lösen erneut aus.
- **Quelle:** `ServiceController.cs:48-55`, `RepairController`, `UpgradeController` (analog), `TaxController.cs:51-57`, `InspectionController.cs:187-193`, `PlanController.cs:361-371`, `ReminderController.cs:78-111`.
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Reminder 10 000 km/12 Monate; Service 2026-05-01 bei 80 000 mit Verknüpfung → Reminder 2027-05-01 / 90 000.

### BR-034 – Reminder-Benachrichtigungen (Auswahl der Empfänger und Deduplizierung)
- **Formel:** Empfänger = `DefaultReminderEmail` (falls gesetzt) + E-Mail aller **direkten** Kollaboratoren (Haushaltsmitglieder nicht). Zustandswechsel-Benachrichtigung: gesendet wird, wenn (Reminder-ID, Dringlichkeit) nicht im In-Memory-Cache ist; Cache-Einträge verfallen nach `DaysToCache` (Default 7).
- **Randfälle:** Neustart leert den Cache → erneute Benachrichtigungen; mehrere Instanzen → Mehrfachversand.
- **Quelle:** `Logic/Event/NotificationLogic.cs:154-332`, `Models/Settings/NotificationConfig.cs`.
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Reminder wird Urgent → 1 Nachricht; erneuter Lauf am selben Tag → keine; nach Wechsel auf VeryUrgent → neue Nachricht.

---

## Steuern und Gebühren

### BR-035 – Wiederkehrende Gebühren (Recurring Tax)
- **Beschreibung:** Überfällige wiederkehrende Gebühren erzeugen automatisch Folgeeinträge.
- **Formel:** Intervall `I` = Enum-Monate oder Custom (Monate/Tage). Wenn `jetzt > Date + I`: Original `IsRecurring = false`; für k = 1, 2, …: neuer Eintrag mit `Date + k·I`, gleiche Beschreibung/Kosten/Anhänge, `IsRecurring = (jetzt ≤ Date + (k+1)·I)`; Abbruch beim ersten wiederkehrenden Eintrag.
- **Auslöser:** Speichern eines Tax-Eintrags, UI-Aufruf `CheckRecurringTaxRecords`, API `GET /api/vehicle/taxrecords/check`, automatisches Ereignis `UpdateRecurringTax`.
- **Randfälle:** Verkaufte Fahrzeuge (`SoldDate` gesetzt) werden übersprungen; Kosten werden unverändert übernommen; Custom-Intervall 0 → Endlosschleife mit fortlaufender Erzeugung neuer Einträge möglich, wenn die Client-Validierung (`wwwroot/js/taxrecord.js:112`, Wert > 0) umgangen wird, z. B. per direktem POST [ABGELEITET: `AddMonths(0)` ergibt nie ein späteres Datum, Abbruchbedingung wird nie wahr]; nicht transaktional.
- **Quelle:** `VehicleLogic.cs:416-469`.
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Gebühr 2025-01-15, 3 Monate, jetzt 2025-09-30 → neue Einträge 2025-04-15 (nicht wiederkehrend), 2025-07-15 (wiederkehrend); Original nicht mehr wiederkehrend.

---

## Kosten und Reporting

### BR-036 – Gesamtkosten eines Fahrzeugs
- **Formel:** `Σ Service.Cost + Σ Repair.Cost + Σ Upgrade.Cost + Σ Tax.Cost + Σ Gas.Cost`.
- **Randfälle:** Inspektionskosten erscheinen nur über die automatisch erzeugte Service-Kopie (BR-047); Supplies (Einkauf) und Plan-Kosten zählen nicht; Kaufpreis nicht enthalten.
- **Quelle:** `VehicleLogic.cs:98-106` (`GetVehicleTotalCost`), `ReportController.cs:168`.
- **Status:** [VERIFIZIERT] (ausgeführt: 5 × 50 = 250,00)
- **Regressionstest:** Service 100, Repair 50, Gas 250 → 400.

### BR-037 – Kosten und Distanz je Monat
- **Formel:** Je Record-Typ Summe `Cost` gruppiert nach Monat (Jahresfilter optional, sonst alle Jahre pro Kalendermonat zusammen); Distanz je Monat = Σ Odometer-Distanz; zusammengeführt als `Cost = Σ`, `Distance = max` über die Typen; `CostPerDistance = Cost / Distance` bei Distance > 0, sonst 0.
- **Quelle:** `Helper/ReportHelper.cs:192-343`, `ReportController.cs:733-822`, `Models/Report/CostForVehicleByMonth.cs:10`.
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Jan 2025 Gas 100, Jan 2026 Gas 50, year=0 → Januar 150.

### BR-038 – Kostentabelle je Distanz und je Tag
- **Formel:** `TotalDistance = BR-015 − BR-016` (nach Jahresfilter); `je Distanz = Summe / TotalDistance`; `je Tag = Summe / Besitztage (BR-039)`; Gesamt = Summe der Kategorien. Bezeichnung „Cost Per Mile/Kilometer/Hour“ nach Einstellung.
- **Randfälle:** Distanz 0 bzw. Tage 0 → 0; Tippfehler bei km-Ständen verfälschen die Distanz.
- **Quelle:** `ReportController.cs:286-322`, `Models/Report/CostTableForVehicle.cs`.
- **Status:** [VERIFIZIERT] (ausgeführt: 250 / 2300 = 0,11; 250 / 272 = 0,92)
- **Regressionstest:** wie ausgeführt.

### BR-039 – Besitzdauer in Tagen
- **Formel (ohne Jahr):** `Start = min(Kaufdatum oder jetzt, alle Record-Daten)`, `Ende = Verkaufsdatum oder jetzt`, `Tage = floor(Ende − Start)`. **Mit Jahr y:** Ende = Verkaufsdatum, falls `y ≥ Verkaufsjahr`, sonst `min(jetzt, 01.01.(y+1))`; Start = Kaufdatum, falls `y ≤ Kaufjahr`, sonst 01.01.y.
- **Randfälle:** Kulturabhängiges Parsen der Datumsstrings; Jahr vor Kaufjahr → negative Tage möglich [ABGELEITET].
- **Quelle:** `VehicleLogic.cs:222-275`.
- **Status:** [VERIFIZIERT] (ausgeführt: 01.01.2026 bis 30.09.2026 ≈ 272 Tage)
- **Regressionstest:** kein Kaufdatum, erster Record 2026-01-01, jetzt 2026-09-30 12:00 → 272.

### BR-040 – Fahrzeughistorie (Druckbericht) inkl. Abschreibung
- **Formel:** `Distanz = BR-015 − BR-016`; `TotalCost = Service + Repair + Upgrade + Tax` (**ohne** Kraftstoff); `TotalGasCost` separat; je Distanz jeweils `/ Distanz`. Abschreibung nur wenn `SoldPrice ≠ 0`: `Total = PurchasePrice − SoldPrice`, `je Tag = |Total / Besitztage|` mit `Besitztage = (Verkauf oder jetzt) − Kauf`, `je Distanz = |Total / Distanz|`.
- **Randfälle:** Ohne Kaufdatum keine Besitztage/Abschreibung; Filter nach Tags/Zeitraum wirkt auf alle Summen.
- **Quelle:** `ReportController.cs:517-684`.
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Kauf 20 000 am 2024-01-01, Verkauf 15 000 am 2025-01-01 (366 Tage), Distanz 10 000 → 13,66/Tag, 0,50/Distanz.

### BR-041 – Garage-Kennzahlen
- **Formel:** `LastReportedMileage = BR-015`; `HasReminders` = mindestens ein Reminder VeryUrgent/PastDue; `CostPerMile = BR-036 / (BR-015 − BR-016)` (0 bei Distanz 0); `TotalCost = BR-036` – jeweils nur, wenn die Metrik am Fahrzeug aktiviert ist.
- **Quelle:** `HomeController.cs:69-124`; `VehicleLogic.cs:276-281`.
- **Status:** [VERIFIZIERT]
- **Regressionstest:** analog BR-038.

### BR-048 – Kiosk-Statistiken
- **Formel:** Je Typ (Service/Repair/Upgrade): Anzahl, Summe; „häufigster“ Eintrag = Beschreibung mit meisten Vorkommen (nur wenn > 1), Durchschnittskosten, letztes Datum; „teuerster“ = erster Eintrag mit maximalen Kosten.
- **Quelle:** `VehicleLogic.cs:509-585`.
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Beschreibungen „Öl“, „Öl“, „Bremse“ → häufigster „Öl“, Vorkommen 2.

---

## Lager, Planer, Inspektion

### BR-042 – Abbuchung von Lagerteilen (Requisition)
- **Formel:** `Stückkosten = Cost / Quantity` (0 bei Quantity 0); `Quantity −= verbraucht`; `Cost −= verbraucht × Stückkosten`, auf 3 Nachkommastellen gerundet; Historieneintrag mit `Cost = verbraucht × Stückkosten` (ungerundet), Datum = Datum des verbrauchenden Records.
- **Randfälle:** Kein serverseitiger Bestandscheck → negativer Bestand möglich (Ausnahme: Plan-Vorlagen prüfen Verfügbarkeit); bei fehlendem Recht auf das Lagerfahrzeug bricht die Schleife still ab und liefert Teilergebnisse; die Kosten des verbrauchenden Records werden **nicht** serverseitig gesetzt.
- **Quelle:** `Controllers/Vehicle/SupplyController.cs:39-80`, `PlanController.cs:129-141`.
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Supply Qty 4, Cost 40; Verbrauch 1,5 → Qty 2,5, Cost 25,000, Historie 15.

### BR-043 – Rückbuchung beim Löschen/Bearbeiten
- **Formel:** Für jeden Historieneintrag mit Supply-ID: `Quantity += q`, `Cost += c`, Historieneintrag „Restored from …“ mit heutigem Datum. Plan-Einträge im Status Done buchen nicht zurück.
- **Quelle:** `VehicleLogic.cs:470-508`; `PlanController.cs:429-433`.
- **Status:** [VERIFIZIERT]
- **Regressionstest:** nach BR-042 Service löschen → Qty 4, Cost 40.

### BR-044 – Plan „Done“ erzeugt Wartungseintrag
- **Formel:** Wechsel auf `Done` erzeugt einen neuen Service/Repair/Upgrade-Eintrag (nach `ImportMode`) mit `Date = heute`, `Mileage = eingegebener Stand` (clientseitig tachokorrigiert), Kosten/Notizen/Anhänge/Historie/Extra-Fields des Plans; optional Auto-Odometer; Pushback verknüpfter Reminder.
- **Randfälle:** Kein Schutz gegen wiederholtes „Done“ (z. B. zurück auf „Testing“ und erneut „Done“) → doppelte Einträge [ABGELEITET]; kein Rollback bei Teilfehlern.
- **Quelle:** `PlanController.cs:277-378`.
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Plan (Service, 120 €) → Done mit km 50 000 → genau ein Service-Eintrag 120 € / 50 000 / heute.

### BR-045 – Planvorlage → Plan
- **Formel:** Prüft: alle Supplies existieren und `Required ≤ InStock`; alle verknüpften Reminder existieren und sind wiederkehrend; dann neuer Plan mit `DateCreated/Modified = jetzt`, Requisition nach BR-042.
- **Quelle:** `PlanController.cs:116-180`; `Models/Supply/SupplyAvailability.cs`.
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Vorlage benötigt 5, Bestand 4 → Fehler „Insufficient Supplies“.

### BR-046 – Bewertung von Inspektionsfeldern
- **Formel:** Radio: fehlgeschlagen, wenn eine ausgewählte Option als „Fail“ markiert ist. Check: fehlgeschlagen, wenn eine **nicht** ausgewählte Option als „Fail“ markiert ist. Text: nie. Inspektion fehlgeschlagen, wenn ein Feld fehlgeschlagen.
- **Quelle:** `Models/InspectionRecord/InspectionRecordTemplateField.cs:21`, `Models/InspectionRecord/InspectionRecord.cs` (`Failed`).
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Check-Feld „Licht ok“ (IsFail=true) nicht angehakt → Failed.

### BR-047 – Seiteneffekte beim Speichern einer Inspektion
- **Formel:** Speichern erzeugt zusätzlich einen Service-Eintrag (gleiches Datum/km/Kosten, Anhang-Link auf die Inspektion) und je fehlgeschlagenem Feld mit Action-Item einen Plan-Eintrag (Backlog, Priorität/Typ aus Vorlage). Inspektionen sind danach nur noch in Tags/Anhängen änderbar.
- **Randfälle:** Plan-Notiz nutzt fälschlich den Text „Auto Insert From Fuel Record“ (`ImportMode.GasRecord`).
- **Quelle:** `InspectionController.cs:177-270`.
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Inspektion 30 € mit einem fehlgeschlagenen Action-Item → +1 Service (30 €), +1 Plan (Backlog).

---

## Stammdaten und Sonstiges

### BR-049 – Zusammenführen von Extra-Field-Vorlage und Record-Werten
- **Formel:** Vorlage leer → **leere Liste** (Record-Werte werden beim nächsten Bearbeiten verworfen); Record ohne Werte → Vorlage; sonst: Felder, die nicht mehr in der Vorlage sind, entfernen; `IsRequired`/`FieldType` aus Vorlage übernehmen; fehlende Felder ergänzen; Reihenfolge der Vorlage.
- **Quelle:** `StaticHelper.AddExtraFields` (`StaticHelper.cs:256-292`).
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Record {A=1, B=2}, Vorlage [B, C] → [B=2, C=""].

### BR-050 – Fahrzeugkennung
- **Formel:** `VehicleIdentifier = "LicensePlate"` → Kennzeichen; sonst Wert des gleichnamigen Extra-Fields, sonst „N/A“.
- **Quelle:** `StaticHelper.GetVehicleIdentifier` (`StaticHelper.cs:475-510`).
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Identifier „VIN“, Extra-Field VIN = „WVW…“ → „WVW…“.

### BR-051 – Sitzungsdauer
- **Formel:** Login ohne „angemeldet bleiben“: 1 Tag; mit: `LUBELOGGER_COOKIE_LIFESPAN` (Default 30), begrenzt auf 1–90 Tage; OIDC-Login: 1 Tag.
- **Quelle:** `ConfigHelper.GetAuthCookieLifeSpan` (`ConfigHelper.cs:114-133`), `LoginController.cs:264, 511, 563`.
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Lifespan 120 → 90 Tage.

### BR-052 – CSV-Import-Interpretation
- **Formel:** Header werden getrimmt/kleingeschrieben und über Aliase gemappt (z. B. `odometer|odo`, `gallons|liters|litres|consumption|quantity|fuelconsumed|qty`, `cost|total cost|totalcost|total price`, `partial_fuelup|partial tank|partial_fill`). Datum aus `date|fuelup_date` (kulturabhängig) oder `day`/`month`/`year`; **fehlt beides → heutiges Datum**. Zahlen mit `NumberStyles.Any` (Währungssymbole erlaubt), km auf int abgeschnitten. Kraftstoff: fehlende Kosten + `price` → `Cost = V × price`; `partial…="1"` → Teilbetankung; `isfilltofull ∈ {1,true,full}`; Einträge mit `V ≤ 0` werden stillschweigend übersprungen, aber als importiert gezählt. Spalten `extrafield_<Name>` → Extra-Fields.
- **Quelle:** `MapProfile/ImportMappers.cs`, `Controllers/Vehicle/ImportController.cs:679-1168`.
- **Status:** [VERIFIZIERT]
- **Regressionstest:** CSV `Date,Odometer,Liters,Price,Partial_Fuelup` / `01/05/2026,12.345,40,1.799,1` → Gallons 40, Cost 71,96, Teilbetankung; Odometer abhängig von Kultur (en-US: 12; de-DE: 12345).

### BR-053 – iCal-Export von Remindern
- **Formel:** Nur Reminder mit Metrik Date/Both; Ereignis ganztägig als „floating time“ (`DTSTART` = Datum 00:00, `DTEND` = 23:59), `UID = MD5(DTSTART + "_" + Beschreibung)`, Priorität 3/2/1/1 für NotUrgent/Urgent/VeryUrgent/PastDue.
- **Randfälle:** Gleiche Beschreibung am selben Tag → gleiche UID (Kollision); keine Zeitzone.
- **Quelle:** `StaticHelper.RemindersToCalendar` (`StaticHelper.cs:931-977`), `VehicleLogic.GetReminders` (`VehicleLogic.cs:336-356`).
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Reminder 2026-10-05 „Ölwechsel“ → `DTSTART:20261005T000000`.

### BR-054 – Webhook-Zustellung
- **Formel:** Bis zu 5 Versuche; Wartezeit nur bei HTTP 429: `2^Versuch + Retry-After + Zufall(1..19)` Sekunden; `discord://` wird zu `https://` mit Discord-Payload. Payload: `type`, `timestamp` (UTC, ISO 8601), `data`, `vehicleId`, `username`, `action`.
- **Randfälle:** Netzwerkfehler (Exception) werden nicht abgefangen (`async void`) [ABGELEITET: kann den Prozess beeinträchtigen]; andere Fehlercodes ohne Wartezeit erneut versucht.
- **Quelle:** `Logic/Event/EventLogic.cs:26-92`, `Models/Shared/WebHookPayload.cs`.
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Mock-Endpoint 500,500,200 → 3 Aufrufe ohne Wartezeit.

### BR-055 – Zeitplan automatischer Ereignisse
- **Formel:** Täglich einmal zur lokalen Serverzeit `HourToCheck:MinuteToCheck`; Prüfung minütlich; Reihenfolge: Cleanup/DeepClean, Backup-Mail, Recurring Tax, Reminder-Mail (alle Dringlichkeiten laut Konfiguration), State-Changed-Benachrichtigungen. Zusätzlich löst jede Datenänderung (`PublishEvent`) eine State-Changed-Prüfung aus.
- **Quelle:** `Logic/Event/AutomatedEventLogic.cs:113-157`, `NotificationLogic.cs:56-217`, `EventLogic.cs:88-91`.
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Konfiguration 06:30, Start 07:00 → erster Lauf am Folgetag 06:30.

### BR-056 – Health-Status
- **Formel:** alle Checks ok → `pass`; alle fehlgeschlagen → `fail` (HTTP 500); sonst `degraded`. Aktuell nur ein DB-Check.
- **Quelle:** `Models/API/ServerHealth.cs`, `APIController.cs:135-155`.
- **Status:** [VERIFIZIERT] (ausgeführt: `{"status":"pass"…}`)
- **Regressionstest:** DB nicht erreichbar → 500, `fail`.

### BR-057 – Record-Typ verschieben (Service ↔ Repair ↔ Upgrade)
- **Formel:** Kopie in Zieltabelle mit **neuer ID**, danach Löschen der Quelle (nicht atomar).
- **Randfälle:** Record-Links (`::ServiceRecord:<id>`) und Auto-Odometer-Anhänge zeigen danach ins Leere.
- **Quelle:** `VehicleController.cs:705-817`, `StaticHelper.GenericToServiceRecord` (`StaticHelper.cs:207-222`).
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Service Id 7 → Repair; Service 7 existiert nicht mehr, Repair hat neue Id.

### BR-058 – Record-Links als Anhang
- **Formel:** Anhang-Location `::<ImportMode>:<id>` verweist auf einen anderen Record; wird für Auto-Inserts und Inspektions-Service-Kopien verwendet.
- **Quelle:** `StaticHelper.cs:808-838`.
- **Status:** [VERIFIZIERT]
- **Regressionstest:** Location `::GasRecord:12` → Typ GasRecord, Id 12.
