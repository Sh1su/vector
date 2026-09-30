# 08 – Import, Export und migrationsrelevante Quellstrukturen

Quellenbezug: `hargata/lubelog` @ `dd69e59` (v1.7.3).

## 1. Übersicht der Austauschformate

| Format | Richtung | Umfang | Quelle | Status |
|---|---|---|---|---|
| CSV je Record-Typ | Import + Export | Service, Repair, Upgrade, Gas, Odometer, Tax, Supply, Plan, Equipment | `Controllers/Vehicle/ImportController.cs` | [VERIFIZIERT] |
| JSON über REST-API | Lesen/Schreiben einzelner Records | siehe `05-bestehende-api.md` | `Controllers/API/*` | [VERIFIZIERT] |
| Backup-ZIP | Export + Restore (Root) | LiteDB-Datei, Dateien, Config | `Helper/FileHelper.cs:161-308, 360-443` | [VERIFIZIERT] (ausgeführt) |
| LiteDB-Datei ↔ PostgreSQL | Migration (Root, nur bei gesetzter PG-Verbindung) | alle 22 Tabellen | `Controllers/MigrationController.cs` | [VERIFIZIERT] |
| Anhänge-ZIP | Export | Dateien + `link_attachments.csv` | `FileHelper.MakeAttachmentsExport` | [VERIFIZIERT] |
| iCal | Export | Reminder mit Datum | `StaticHelper.RemindersToCalendar` | [VERIFIZIERT] |
| Fahrzeughistorie | Druckansicht (HTML) | Service, Repair, Upgrade, Tax + Kennzahlen | `ReportController.GetVehicleHistory` | [VERIFIZIERT] |

Nicht als CSV verfügbar: Reminder, Notizen, Inspektionen, Fahrzeuge, Benutzer. [VERIFIZIERT: Modi in `ExportFromVehicleToCsv`/`ImportToVehicleIdFromCsv`]

## 2. CSV-Export

### 2.1 Allgemeine Eigenschaften
| Aussage | Quelle | Status |
|---|---|---|
| Trennzeichen Komma, Quoting nach CsvHelper-Regeln (`CultureInfo.InvariantCulture` für den Writer) | `ImportController.cs:274-282` u. a. | [VERIFIZIERT] |
| Encoding: `StreamWriter`-Default UTF-8 ohne BOM | `new StreamWriter(path)` | [ABGELEITET] (.NET-Default) |
| **Werte werden vorab kulturabhängig zu Strings formatiert:** Datum `ToShortDateString()`, Kosten `ToString("C")` (mit Währungssymbol und Tausendertrennern), Zahlen mit Kultur-Dezimaltrennzeichen | `ImportController.cs:264-273` | [VERIFIZIERT] |
| Kraftstoff-Kosten ohne Währungsformat (`ToString()`) – inkonsistent zu anderen Typen | `ImportController.cs:649` | [VERIFIZIERT] |
| Extra-Fields als Spalten `extrafield_{Name}` (Vereinigung aller Namen) | `StaticHelper.WriteGenericRecordExportModel` (`StaticHelper.cs:560-590`) | [VERIFIZIERT] |
| Filter: Tags (ausschließen/nur einschließen), Datumsbereich (inklusiv) | `Models/Shared/CSVExportParameter.cs`, `ImportController.cs:238-261` | [VERIFIZIERT] |
| Anhänge, IDs, Anlagedaten werden nicht exportiert | Writer-Methoden | [VERIFIZIERT] |

### 2.2 Spalten je Typ [VERIFIZIERT: `StaticHelper.cs:560-762, 884-930`]

| Typ | Spalten (Reihenfolge) |
|---|---|
| Service / Repair / Upgrade | `Date, Description, Cost, Notes, Odometer, Tags, extrafield_*` |
| Gas | `Date, Odometer, FuelConsumed, Cost, FuelEconomy, IsFillToFull, MissedFuelUp, [StartingSoc, EndingSoc nur EV], Notes, Tags, extrafield_*` |
| Odometer | `Date, InitialOdometer, Odometer, Notes, Tags, extrafield_*` (Equipment-Verknüpfung fehlt) |
| Tax | `Date, Description, Cost, Notes, Tags, extrafield_*` (Wiederkehr fehlt) |
| Supply | `Date, PartNumber, PartSupplier, PartQuantity, Description, Notes, Cost, Tags, extrafield_*` (Verbrauchshistorie fehlt) |
| Plan | `DateCreated, DateModified, Description, Notes, Type, Priority, Progress, Cost, extrafield_*` |
| Equipment | `Description, Notes, Tags, IsEquipped, extrafield_*` |

## 3. CSV-Import

| Aussage | Quelle | Status |
|---|---|---|
| Ablauf: Datei-Upload nach `temp/`, dann `POST /Vehicle/ImportToVehicleIdFromCsv(vehicleId, mode, fileName)`; Edit-Recht am Fahrzeug erforderlich | `ImportController.cs:677-702` | [VERIFIZIERT] |
| Einstellung `EnableCsvImports` blendet nur die UI-Schaltflächen aus; der Endpunkt ist nicht gesperrt | `Views/Vehicle/*/_*Records.cshtml` (`enableCsvImports`), kein Check im Controller | [VERIFIZIERT] |
| Header: getrimmt, kleingeschrieben, Aliase (u. a. Fuelly-kompatibel) laut BR-052; fehlende Spalten → `null` | `ImportController.cs:708-716`, `MapProfile/ImportMappers.cs` | [VERIFIZIERT] |
| Datum/Zahlen werden mit **Serverkultur** geparst (`DateTime.Parse`, `decimal.Parse(..., NumberStyles.Any)`); fehlt das Datum, wird das **heutige** Datum gesetzt | `ImportController.cs:728-740, 756-771` | [VERIFIZIERT] |
| Konvertierung „alles oder nichts“: ein ungültiger Wert verwirft den ganzen Import vor dem Speichern; das Speichern selbst ist nicht transaktional | `ImportController.cs:747-805` | [VERIFIZIERT] |
| Keine Duplikaterkennung, keine Plausibilitätsprüfung (km, Mengen), keine Vorschau | – | [VERIFIZIERT] |
| Gas: Zeilen mit Menge ≤ 0 werden übersprungen, aber als importiert gemeldet | `ImportController.cs:808-827, 1162` | [VERIFIZIERT] |
| Plan: ungültige Enum-Werte werden stillschweigend zum Default (`Backlog`, `ServiceRecord`, `Critical`) | `ImportController.cs:1014-1024` (`Enum.TryParse` ohne Prüfung des Ergebnisses) | [VERIFIZIERT] |
| Auto-Odometer bei `EnableAutoOdometerInsert` auch beim Import | `ImportController.cs:814-824` | [VERIFIZIERT] |

## 4. Round-Trip-Fähigkeit (Export → Import)

| Typ | Ergebnis | Begründung | Status |
|---|---|---|---|
| Service/Repair/Upgrade | verlustfrei für Datum, Beschreibung, Kosten, Notizen, km, Tags, Extra-Field-Werte – **nur bei gleicher Serverkultur** | Formatierung/Parsing kulturabhängig; Anhänge, Supply-Historie gehen verloren | [ABGELEITET] aus 2.1/3 |
| Gas (Verbrenner, metrisch/US) | verlustfrei (Mengen/Kosten) | `FuelConsumed` = gespeicherte Menge | [ABGELEITET] |
| Gas bei `UseUKMPG` | **verlustbehaftet:** exportiert imperiale Gallonen, Import speichert sie als Liter (Faktor 4,546 zu klein) | `ImportController.cs:617-651` (Export aus ViewModel) | [VERIFIZIERT] (Code) |
| Gas EV | **verlustbehaftet:** exportiert berechneten Verbrauch statt geladener kWh | `GasHelper.cs:80` | [VERIFIZIERT] (Code) |
| Odometer | Equipment-Verknüpfungen gehen verloren | Writer ohne `EquipmentRecordId` | [VERIFIZIERT] |
| Tax | Wiederkehr-Einstellungen gehen verloren | s. o. | [VERIFIZIERT] |
| Plan | Reminder-/Supply-Verknüpfungen, Anhänge gehen verloren | s. o. | [VERIFIZIERT] |

## 5. Backup und Restore

| Aussage | Quelle | Status |
|---|---|---|
| ZIP-Struktur: `images/`, `documents/`, `translations/`, `themes/`, `data/cartracker.db`, `data/widgets.html`, `config/userConfig.json`, `config/serverConfig.json` | `FileHelper.MakeBackup` | [VERIFIZIERT] (ausgeführt: `unzip -l`) |
| **Bei PostgreSQL-Backend enthält das Backup keine Fachdaten** (nur die – ungenutzte – LiteDB-Datei); PG muss separat gesichert werden | `FileHelper.cs:368, 414-419` (`StaticHelper.DbName`) | [VERIFIZIERT] (Code) |
| Restore überschreibt Datenbankdatei und Config, kopiert Dateien hinzu (bzw. löscht vorher bei Demo-Restore); keine Versions-/Kompatibilitätsprüfung | `FileHelper.RestoreBackup` | [VERIFIZIERT] |
| Backups werden optional täglich per E-Mail verschickt (Anhang enthält Hashes und Geheimnisse) | `NotificationLogic.cs:100-126` | [VERIFIZIERT] |

## 6. Migration LiteDB ↔ PostgreSQL (Bestandsfunktion)

| Aussage | Quelle | Status |
|---|---|---|
| Export: liest alle PG-Tabellen (`data`-JSON bzw. Spalten) und schreibt eine LiteDB-Datei in `temp/` | `MigrationController.Export` (`MigrationController.cs:69-533`) | [VERIFIZIERT] |
| Import: liest eine hochgeladene LiteDB-Datei und fügt alle Datensätze mit **Original-IDs** in PG ein; Sequenzen werden per `setval` nachgezogen; kein `ON CONFLICT`, keine Transaktion (Abbruch hinterlässt Teilimport) | `MigrationController.Import` (`MigrationController.cs:534-951`) | [VERIFIZIERT] |
| Die Funktion ist nur erreichbar, wenn `POSTGRES_CONNECTION` gesetzt ist | `MigrationController.cs:24-33` | [VERIFIZIERT] |

## 7. Quellstrukturen für die Datenmigration in das neue System

### 7.1 Zu migrierende Quellen

| Quelle | LiteDB | PostgreSQL | Inhalt | Status |
|---|---|---|---|---|
| Fahrzeuge | Collection `vehicles` | `app.vehicles(id, data)` | `Vehicle` | [VERIFIZIERT] |
| Wartung | `servicerecords`, `collisionrecords`, `upgraderecords` | `app.<name>(id, vehicleId, data)` | `GenericRecord` | [VERIFIZIERT] |
| Kraftstoff | `gasrecords` | `app.gasrecords` | `GasRecord` | [VERIFIZIERT] |
| Kilometer | `odometerrecords` | `app.odometerrecords` | `OdometerRecord` | [VERIFIZIERT] |
| Gebühren | `taxrecords` | `app.taxrecords` | `TaxRecord` | [VERIFIZIERT] |
| Reminder | `reminderrecords` | `app.reminderrecords` | `ReminderRecord` | [VERIFIZIERT] |
| Notizen | `notes` | `app.notes` | `Note` | [VERIFIZIERT] |
| Lager | `supplyrecords` | `app.supplyrecords` | `SupplyRecord` (VehicleId 0 = Werkstatt) | [VERIFIZIERT] |
| Planer | `planrecords`, `planrecordtemplates` | analog | `PlanRecord`, `PlanRecordInput` | [VERIFIZIERT] |
| Inspektion | `inspectionrecords`, `inspectionrecordtemplates` | analog | `InspectionRecord`, `InspectionRecordInput` | [VERIFIZIERT] |
| Ausstattung | `equipmentrecords` | analog | `EquipmentRecord` | [VERIFIZIERT] |
| Benutzer | `userrecords` | `app.userrecords(id, username, emailaddress, password, isadmin)` | `UserData` | [VERIFIZIERT] |
| Freigaben | `useraccessrecords` (`_id: {UserId, VehicleId}`) | `app.useraccessrecords(userId, vehicleId)` | `UserAccess` | [VERIFIZIERT] |
| Haushalte | `userhouseholdrecords` | `app.userhouseholdrecords(parentUserId, childUserId, data)` | `UserHousehold` | [VERIFIZIERT] |
| API-Keys | `apikeyrecords` | `app.apikeyrecords(id, userId, apiKey, data)` | `APIKey` (Hash) | [VERIFIZIERT] |
| Tokens | `tokenrecords` | `app.tokenrecords(id, body, emailaddress)` | offene Einladungen/Resets | [VERIFIZIERT] |
| Benutzereinstellungen | `userconfigrecords` | `app.userconfigrecords(id, data)` | `UserConfig` je Nutzer | [VERIFIZIERT] |
| Extra-Field-Vorlagen | `extrafields` (`_id` = ImportMode) | `app.extrafields(id, data)` | `RecordExtraField` | [VERIFIZIERT] |
| Server-/Root-Einstellungen | `data/config/userConfig.json`, `serverConfig.json`, Umgebungsvariablen | – | Einheiten-Defaults, Root-Hashes, Kultur | [VERIFIZIERT] |
| Dateien | `data/images/`, `data/documents/` | – | Binärdaten | [VERIFIZIERT] |

### 7.2 Migrationskritische Semantik

| # | Thema | Befund | Status |
|---|---|---|---|
| MG-1 | **Einheiten** | Werte tragen keine Einheit. Die Einheit ergibt sich aus der Einstellung des Nutzers, der die Daten erfasst hat – diese ist nicht gespeichert. Pro Fahrzeug muss beim Import eine Einheit festgelegt werden (Vorschlag: aus `UseMPG`/`UseUKMPG` der Kollaboratoren bzw. Server-Default ableiten, bei Widerspruch nachfragen). Sonderfall `UseUKMPG`: Menge in Litern, Distanz in Meilen. EV: Menge kWh. `UseHours`: Motorstunden statt Distanz. | [VERIFIZIERT] (Befund), Vorgehen [ABGELEITET] |
| MG-2 | **Datum/Zeitzone (LiteDB)** | LiteDB speichert lokale Mitternacht als UTC-Zeitpunkt; die Rückrechnung muss mit der **Zeitzone des Quellservers** erfolgen, sonst Verschiebung um ±1 Tag. Die Server-Zeitzone ist nicht in der DB gespeichert. | [VERIFIZIERT] (ausgeführt) |
| MG-3 | **Datum als String** | `Vehicle.PurchaseDate/SoldDate` und Planvorlagen-Daten sind kulturformatierte Strings; zum Parsen wird die Kultur des Quellservers benötigt (`LUBELOGGER_LOCALE_OVERRIDE` o. ä.). | [VERIFIZIERT] |
| MG-4 | **Kilometerstände** | Nicht monoton, Tippfehler, 0 = „unbekannt“; Tachokorrektur-Parameter ohne Gültigkeitszeitraum. Import muss Werte übernehmen, darf sie aber nicht stillschweigend „reparieren“ – Anomalien als Prüfliste ausgeben. | [VERIFIZIERT] (Befund), Vorgehen [ABGELEITET] |
| MG-5 | **Herkunft Kilometerstand** | Automatisch erzeugte Odometer-Einträge sind nur über Notiztext („Auto Insert From …“, übersetzbar!) und Anhang `::Typ:Id` erkennbar. | [VERIFIZIERT] |
| MG-6 | **IDs** | Integer je Tabelle; Referenzen in eingebetteten Listen (`ReminderRecordIds`, `EquipmentRecordId`, `RequisitionHistory[].Id`, `Supplies[].SupplyId`, `::Typ:Id`) müssen auf neue IDs (UUIDv7) umgeschlüsselt werden; verwaiste Referenzen sind möglich. | [VERIFIZIERT] |
| MG-7 | **Passwörter** | Ungesalzenes SHA-256 (Hex). Übernahme nur als Legacy-Hash mit Zwangs-Rehash beim ersten Login oder Passwort-Reset für alle. Root-User hat keinen DB-Datensatz (Hash von Name + Passwort in Config). | [VERIFIZIERT] |
| MG-8 | **API-Keys** | SHA-256 des Schlüssels; übernehmbar als Legacy-Hash, wenn API-Kompatibilität gewünscht. | [VERIFIZIERT] |
| MG-9 | **Rechte** | Kollaborator = Vollzugriff ohne Eigentümer; Haushalte geben Zugriff auf alle Fahrzeuge des Parents. Abbildung auf Eigentümer/Bearbeiter/Leser erfordert Regel (Q-04). | [VERIFIZIERT] |
| MG-10 | **Dateien** | Zuordnung nur über `Location`-Strings; dieselbe Datei kann mehrfach referenziert sein; unreferenzierte Dateien möglich; externe URLs; SHA-256 für Originale ist beim Import zu berechnen (Upload-Zeitpunkt unbekannt → als „importiert“ kennzeichnen). | [VERIFIZIERT] (Befund), Vorgehen [ABGELEITET] |
| MG-11 | **Duplikate durch Automatiken** | Inspektionen erzeugen Service-Kopien, Plan-„Done“ erzeugt Records, wiederkehrende Gebühren erzeugen Folgeeinträge – beim Import nicht doppelt interpretieren (z. B. Kosten nicht doppelt zählen). | [VERIFIZIERT] |
| MG-12 | **Extra-Fields** | Freie Name/Wert-Paare (Wert immer String); z. B. VIN, Reifengröße, Ölsorte – müssen auf Zielfelder gemappt oder als generische Attribute übernommen werden. | [VERIFIZIERT] (Struktur), typische Nutzung [UNKLAR] |
| MG-13 | **Enums als Integer** | Werte sind Ordinalzahlen (z. B. `Metric: 2` = Both). | [ABGELEITET] |
| MG-14 | **Kosten/Währung** | Keine Währung gespeichert; Serverkultur definiert Währung. | [VERIFIZIERT] |
| MG-15 | **PG `vehicleId`-Drift** | Bei PG kann die Spalte `vehicleId` vom `data.VehicleId` abweichen (Update schreibt nur JSON, D-08). Für die Migration ist zu entscheiden, welche Quelle maßgeblich ist; Abweichungen sind als Anomalie zu melden. | [VERIFIZIERT] (Code) |

### 7.3 Empfohlener Migrationsweg (Vorschlag für Phase 2/3)

[ABGELEITET] – Begründung: Die Bestandsfunktion „Export PG → LiteDB“ existiert bereits; damit genügt **ein** Importpfad.

1. Quelle vereinheitlichen: bei PG-Installationen die vorhandene Migration „Export“ nutzen → LiteDB-Datei; bei LiteDB die Datei direkt. Alternativ direkter JSONB-Leser für PG.
2. Offline-Importwerkzeug (Teil des Moduls Import/Migration) liest LiteDB + `data/`-Ordner + Config, fragt fehlende Parameter ab (Quell-Zeitzone, Kultur, Einheiten je Fahrzeug, Rechte-Mapping) und erzeugt einen **Prüfbericht** (Anomalien, verwaiste Referenzen) vor dem Schreiben.
3. Import in einer Transaktion je Fahrzeug, idempotent (Quell-ID-Mapping-Tabelle), Herkunft „Import LubeLogger“ an jedem Datensatz.
