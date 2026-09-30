# 02 – Feature-Inventar (Ist-Zustand)

Quellenbezug: `hargata/lubelog` @ `dd69e59` (v1.7.3). Die Feature-IDs (F-001 …) sind stabil und werden in Phase 3 um Zielspalten ergänzt. Spalte „Status“ bewertet die Aussagen der jeweiligen Zeile; „(ausgeführt)“ = zusätzlich durch Ausführung bestätigt. API-Spalte: ✓ = über öffentliche `/api` verfügbar, T = teilweise, – = nur UI. Import/Export: C = CSV, B = Backup, A = Anhänge-ZIP, I = iCal.

## Feature-Matrix

| Feature-ID | Feature | Beschreibung | Nutzer-Workflow | Implementierung (Dateien) | Datenmodell | API vorhanden | Import/Export | Status | Anmerkungen |
|---|---|---|---|---|---|---|---|---|---|
| F-001 | Fahrzeugverwaltung | Fahrzeug anlegen, bearbeiten, löschen (inkl. aller Records); Bild, Kennzeichen/Kennung, Kraftstoffart, Motorstunden, Kilometer optional, Kauf/Verkauf, Tags, Extra-Fields | Garage → „Add Vehicle“ → Modal; Fahrzeug öffnen → Bearbeiten/Löschen | `Controllers/VehicleController.cs:111-194`, `Views/Vehicle/_VehicleModal.cshtml`, `wwwroot/js/shared.js`, `APIController.cs:199-449` | Vehicle | T (ohne Kauf/Verkauf/Bild/Tachokorrektur) | B | [VERIFIZIERT] (ausgeführt) | keine VIN-Felder; Löschen ohne Datei-Löschung |
| F-002 | Garage-Übersicht & Kennzahlen | Kacheln aller zugänglichen Fahrzeuge; optionale Metriken letzter km-Stand, dringende Reminder, Gesamtkosten, Kosten/Distanz | Startseite „Garage“ | `HomeController.Garage` (`HomeController.cs:69-124`), `Views/Home/_GarageDisplay.cshtml`, `APIController.VehicleInfo` | Vehicle.DashboardMetrics | ✓ (`/api/vehicle/info`) | – | [VERIFIZIERT] | BR-041 |
| F-003 | Kilometerstand-Einträge | Odometer-Einträge mit Initial-/Endstand, Notizen, Anhängen, Equipment | Tab „Odometer“ → Add; Bulk-Bearbeitung | `Controllers/Vehicle/OdometerController.cs`, `Controllers/API/OdometerController.cs`, `Logic/OdometerLogic.cs`, `Views/Vehicle/Odometer/*` | OdometerRecord | ✓ | C | [VERIFIZIERT] (ausgeführt) | keine Plausibilitätsprüfung (BR-021) |
| F-004 | Distanz-Neuberechnung | InitialMileage aller Einträge neu setzen | Odometer-Tab → „Recalculate Distance“ | `OdometerLogic.AutoConvertOdometerRecord`, `OdometerController.cs:11-21` | OdometerRecord | ✓ (`recalculate`) | – | [VERIFIZIERT] | BR-019 |
| F-005 | Tachokorrektur | Multiplikator/Offset je Fahrzeug; Anwendung bei Neuanlage (Browser) und per Bulk-Aktion | Fahrzeug bearbeiten → „Odometer Adjustments“; Record-Auswahl → „Adjust Odometer“ | `wwwroot/js/vehicle.js:537-547`, `VehicleController.AdjustRecordsOdometer`, `APIController.AdjustedOdometer` | Vehicle.OdometerMultiplier/Difference | T | – | [VERIFIZIERT] | drei Rundungsvarianten (BR-022) |
| F-006 | Automatische Odometer-Einträge | Aus Service/Repair/Upgrade/Gas/Inspection/Plan automatisch Odometer-Eintrag erzeugen; Bulk-Erzeugung | Einstellung „Auto Insert Odometer“; Record-Auswahl → „Create Odometer“ | `OdometerLogic.AutoInsertOdometerRecord`, `VehicleController.BulkCreateOdometerRecords` | OdometerRecord | T (bei API-Add) | – | [VERIFIZIERT] | BR-018 |
| F-007 | Distanz auf andere Fahrzeuge übertragen | Gefahrene Distanz als Eintrag auf z. B. Anhänger übertragen, optional spätere Stände verschieben | Odometer-Auswahl → „Duplicate distance to vehicles“ | `OdometerController.DuplicateDistanceToOtherVehicles` | OdometerRecord | – | – | [VERIFIZIERT] | BR-023 |
| F-008 | Tankvorgänge | Menge, Kosten (gesamt oder Stückpreis), km, Voll/Teil, verpasst, EV-Ladezustand | Tab „Fuel“ → Add; Bulk-Bearbeitung | `Controllers/Vehicle/GasController.cs`, `Controllers/API/GasController.cs`, `wwwroot/js/gasrecord.js`, `Views/Vehicle/Gas/*` | GasRecord | ✓ | C | [VERIFIZIERT] (ausgeführt) | |
| F-009 | Verbrauchsberechnung & Einheiten | Verbrauch je Tankvorgang, Durchschnitt, Einheitenwahl (mpg, l/100km, UK, kWh, Stunden), Anzeigeumrechnung | automatisch im Fuel-Tab und Dashboard | `Helper/GasHelper.cs`, `StaticHelper.GetFuelEconomyUnit`, `GasController.SaveUserGasTabPreferences` | GasRecordViewModel, UserConfig | T (berechnete Felder) | – | [VERIFIZIERT] (ausgeführt) | BR-001–BR-014, BR-059 |
| F-010 | Service-Einträge | Wartungen mit Datum, km, Beschreibung, Kosten, Supplies, Reminder-Pushback | Tab „Service Records“ → Add | `Controllers/Vehicle/ServiceController.cs`, `Controllers/API/ServiceController.cs`, `Views/Vehicle/Service/*` | ServiceRecord | ✓ | C, A | [VERIFIZIERT] | keine Trennung Teile/Arbeit, keine Werkstatt |
| F-011 | Reparaturen | wie F-010 (Tabelle `collisionrecords`) | Tab „Repairs“ | `Controllers/Vehicle/RepairController.cs`, `Controllers/API/RepairController.cs` | CollisionRecord | ✓ | C, A | [VERIFIZIERT] | |
| F-012 | Upgrades | wie F-010 | Tab „Upgrades“ | `Controllers/Vehicle/UpgradeController.cs`, `Controllers/API/UpgradeController.cs` | UpgradeRecord | ✓ | C, A | [VERIFIZIERT] | |
| F-013 | Record-Typ verschieben | Service ↔ Repair ↔ Upgrade | Auswahl → „Move to …“ | `VehicleController.MoveRecord(s)` | GenericRecord | – | – | [VERIFIZIERT] | neue ID (BR-057) |
| F-014 | Steuern & Gebühren | Einträge ohne km; wiederkehrend mit automatischer Fortschreibung | Tab „Taxes“ → Add, „recurring“ | `Controllers/Vehicle/TaxController.cs`, `Controllers/API/TaxController.cs`, `VehicleLogic.UpdateRecurringTaxes` | TaxRecord | T (ohne Wiederkehr) | C, A | [VERIFIZIERT] | BR-035 |
| F-015 | Reminder | Fälligkeit nach Datum/km/beidem, wiederkehrend, Dringlichkeitsstufen, eigene Schwellen | Tab „Reminders“ → Add; Badge im Dashboard | `Controllers/Vehicle/ReminderController.cs`, `Controllers/API/ReminderController.cs`, `Helper/ReminderHelper.cs` | ReminderRecord | T (ohne Wiederkehr/Schwellen) | I | [VERIFIZIERT] (ausgeführt) | Schwellen-Bug BR-029 |
| F-016 | Reminder-Erledigung (Pushback) | Wiederkehrende Reminder durch Service/Repair/Upgrade/Tax/Inspection/Plan fortschreiben; manuell „Done“; Auto-Refresh | Record-Modal → Reminder auswählen; Reminder → „Mark as done“ | `ReminderController.PushbackRecurringReminderRecord`, Aufrufer in Record-Controllern | ReminderRecord | – | – | [VERIFIZIERT] | keine gespeicherte Verknüpfung (BR-033) |
| F-017 | Kalender | Reminder im Kalender, iCal-Abonnement | Menü „Calendar“; `/api/calendar` | `HomeController.Calendar/ViewCalendarReminder`, `API/ReminderController.Calendar`, `StaticHelper.RemindersToCalendar` | ReminderRecord | ✓ | I | [VERIFIZIERT] | BR-053 |
| F-018 | Planer (Kanban) & Vorlagen | Aufgaben mit Priorität/Status; „Done“ erzeugt Wartungseintrag; Vorlagen mit Teilen und Reminder | Tab „Planner“, Drag&Drop | `Controllers/Vehicle/PlanController.cs`, `Controllers/API/PlanController.cs`, `wwwroot/js/planrecord.js` | PlanRecord, PlanRecordInput | T (ohne Done/Vorlagen) | C | [VERIFIZIERT] | BR-044/045 |
| F-019 | Teilelager (Supplies) | Teile mit Menge/Kosten je Fahrzeug oder Werkstatt; Verbrauch in Records, Rückbuchung | Tab „Supplies“; Record-Modal → „Supplies“ | `Controllers/Vehicle/SupplyController.cs`, `Controllers/API/SupplyController.cs` | SupplyRecord | ✓ | C | [VERIFIZIERT] | BR-042/043 |
| F-020 | Notizen | Freitext-Notizen, anheftbar, Markdown | Tab „Notes“ | `Controllers/Vehicle/NoteController.cs`, `Controllers/API/NoteController.cs` | Note | ✓ | – | [VERIFIZIERT] | |
| F-021 | Inspektionen | Checklisten-Vorlagen; Durchführung mit Pass/Fail; Action-Items → Planer; Service-Kopie | Tab „Inspections“ | `Controllers/Vehicle/InspectionController.cs`, `Views/Vehicle/Inspection/*` | InspectionRecord(+Template) | – | – | [VERIFIZIERT] | BR-046/047 |
| F-022 | Ausstattung (Equipment) | Teile/Anbauten mit Laufleistung über Odometer-Einträge | Tab „Equipment“; Odometer-Modal → Equipment | `Controllers/Vehicle/EquipmentController.cs`, `Controllers/API/EquipmentController.cs`, `Helper/EquipmentHelper.cs` | EquipmentRecord | ✓ | C | [VERIFIZIERT] | BR-024/025 |
| F-023 | Anhänge | Dateien/Links/Record-Links an Records; Upload, Vorschau, Download | Record-Modal → Upload | `FilesController.cs`, `APIController.UploadDocument`, `Views/Vehicle/_FileUploader.cshtml` | UploadedFiles | T (Upload) | A, B | [VERIFIZIERT] (ausgeführt) | siehe 07 |
| F-024 | Fahrzeug-Imagemap | Klickbare Bildregionen, die Records per Tag filtern | Fahrzeug bearbeiten → Map-JSON; Dashboard | `ReportController.GetVehicleImageMap`, `FilesController.UploadCoordinates`, `Models/Vehicle/VehicleImageMap.cs` | Vehicle.MapLocation | – | – | [VERIFIZIERT] | Nischenfunktion [ABGELEITET] |
| F-025 | Dashboard/Report-Tab | Kosten-Zusammensetzung, Kosten/Monat, Verbrauch/Monat, Kostentabelle, Reminder-Übersicht, Kollaboratoren | Tab „Dashboard“ | `Controllers/Vehicle/ReportController.cs:12-351, 687-822`, `Helper/ReportHelper.cs`, `wwwroot/js/reports.js` | – | – | – | [VERIFIZIERT] (ausgeführt) | BR-036–BR-039, BR-060 |
| F-026 | Fahrzeughistorie (Druck) | Chronologischer Bericht mit Kennzahlen, Abschreibung, Filtern | Dashboard → „Vehicle History“ | `ReportController.GetVehicleHistory`, `Views/Vehicle/Report/_VehicleHistory.cshtml` | – | – | – | [VERIFIZIERT] | BR-040 |
| F-027 | Anhänge-Export | ZIP aller Anhänge ausgewählter Typen | Dashboard → „Export Attachments“ | `ReportController.GetVehicleAttachments`, `FileHelper.MakeAttachmentsExport` | – | – | A | [VERIFIZIERT] | |
| F-028 | CSV-Import | Import je Record-Typ inkl. Musterdatei | Tab → „Import via CSV“ | `Controllers/Vehicle/ImportController.cs:14-216, 677-1168`, `MapProfile/ImportMappers.cs` | ImportModel | – | C | [VERIFIZIERT] | BR-052 |
| F-029 | CSV-Export | Export je Record-Typ mit Tag-/Datumsfilter | Tab → „Export“ | `ImportController.cs:217-676` | *ExportModel | – | C | [VERIFIZIERT] | Round-Trip-Probleme (08) |
| F-030 | Suche | Volltext über sichtbare Tabs, Tag-Suche (und/oder) | Suchfeld im Fahrzeug | `VehicleController.SearchRecords/SearchRecordsByTags` | – | – | – | [VERIFIZIERT] | |
| F-031 | Bulk-Operationen | Mehrfach bearbeiten, löschen, duplizieren (auch auf andere Fahrzeuge) | Tabellenauswahl → Kontextmenü | `VehicleController.cs:818-1587`, `GasController.SaveMultipleGasRecords`, `OdometerController.SaveMultipleOdometerRecords` | – | – | – | [VERIFIZIERT] | nicht transaktional |
| F-032 | Aufkleber/QR-Druck | Druckbare Sticker für Records | Auswahl → „Print“ | `VehicleController.PrintRecordStickers`, `Views/Vehicle/_Stickers.cshtml` | – | – | – | [VERIFIZIERT] | |
| F-033 | Extra-Fields | Root definiert Zusatzfelder je Record-Typ (Typ, Pflicht) | Einstellungen → „Extra Fields“ | `HomeController.GetExtraFieldsModal/UpdateExtraFields`, `StaticHelper.AddExtraFields` | RecordExtraField, ExtraField | T (lesen) | C | [VERIFIZIERT] | BR-049 |
| F-034 | Tags & Filter | Tags an allen Records; Filter in Tabellen, Berichten, Exporten, API | überall | Modelle, Views, `MethodParameter` | Tags | ✓ (Filter) | C | [VERIFIZIERT] | |
| F-035 | Lokale Benutzerkonten | Registrierung per Token, Login, Passwort-Reset, Kontoänderung | Login-Seite, Konto-Modal | `LoginController.cs`, `Logic/LoginLogic.cs`, `HomeController.cs:202-248` | UserData, Token | – | – | [VERIFIZIERT] (ausgeführt) | siehe 06 |
| F-036 | Root-User & Auth-Schalter | Auth aktivieren/deaktivieren, Root-Zugangsdaten | Einstellungen → „Enable Authentication“ | `LoginController.CreateLoginCreds/DestroyLoginCreds`, `LoginLogic.cs:503-549` | userConfig.json | – | – | [VERIFIZIERT] (ausgeführt) | Default ohne Auth |
| F-037 | OIDC-Login | Login über externen IdP, Registrierung, optional nur OIDC | Login → „Login via …“ | `LoginController.RemoteAuth`, `HomeController.ImportOpenIDConfiguration` | OpenIDConfig | – | – | [VERIFIZIERT] | siehe 06 |
| F-038 | Admin-Panel | Benutzer, Tokens, Admin-Flag, Passwort-Reset | Menü „Admin Panel“ | `Controllers/AdminController.cs`, `Views/Admin/*` | UserData, Token | – | – | [VERIFIZIERT] | |
| F-039 | Kollaboratoren | Fahrzeug mit Benutzern teilen (Vollzugriff) | Dashboard → Collaborators; Garage-Mehrfachauswahl | `ReportController.cs:174-200`, `VehicleController.cs:195-266`, `Logic/UserLogic.cs` | UserAccess | – | – | [VERIFIZIERT] | |
| F-040 | Haushalte | Benutzer erben Zugriff auf alle Fahrzeuge eines anderen mit Rechten View/Edit/Delete | Konto → „Household“ | `HomeController.cs:249-292`, `AdminController.cs:89-118`, `UserLogic.cs:203-298` | UserHousehold | – | – | [VERIFIZIERT] | |
| F-041 | API-Keys | Schlüssel mit Rechten je Benutzer | Konto → „API Keys“ | `HomeController.cs:293-315`, `UserLogic.CreateAPIKey`, `Filter/APIKeyFilter.cs` | APIKey | – | – | [VERIFIZIERT] (ausgeführt) | |
| F-042 | Öffentliche REST-API | CRUD für die meisten Record-Typen, Info, Backup, Cleanup | `/api`-Dokuseite | `Controllers/APIController.cs`, `Controllers/API/*.cs`, `wwwroot/defaults/api.json` | Export-Modelle | ✓ | – | [VERIFIZIERT] (ausgeführt) | siehe 05 |
| F-043 | Kiosk-Modus | Vollbild-Anzeige Fahrzeuge/Planer/Reminder, zyklisch | `/Kiosk` | `Controllers/KioskController.cs`, `Views/Kiosk/*`, `wwwroot/js/kiosk.js` | – | – | – | [VERIFIZIERT] | BR-048 |
| F-044 | Benutzereinstellungen | Einheiten, Darstellung, Tabs, Sprache, Theme, Automatiken | Menü „Settings“ | `HomeController.Settings/WriteToSettings`, `Helper/ConfigHelper.cs`, `wwwroot/js/settings.js` | UserConfig | – | B | [VERIFIZIERT] | |
| F-045 | Servereinstellungen | `/setup`: DB, SMTP, OIDC, Webhook, Locale, Notifications, Kestrel | `/setup` (Root) | `HomeController.Setup/WriteServerConfiguration`, `ConfigHelper.SaveServerConfig`, `wwwroot/js/serversettings.js` | ServerConfig | – | B | [VERIFIZIERT] | |
| F-046 | Übersetzungen | Download aus GitHub, Editor, Upload/Export | Einstellungen → Sprache | `HomeController.cs:325-534`, `Helper/TranslationHelper.cs`, `FilesController.HandleTranslationFileUpload` | JSON-Dateien | – | B | [VERIFIZIERT] | Schlüssel in `en_US.json` |
| F-047 | Themes | CSS-Themes hochladen und wählen | Einstellungen → Theme | `FilesController.HandleThemeFileUpload`, `SharedDataController.GetConfiguredTheme` | CSS-Dateien | – | B | [VERIFIZIERT] | |
| F-048 | Custom Widgets | Root-definiertes HTML/JS im Dashboard | Einstellungen → Widgets | `HomeController.cs:579-611`, `ReportController.GetAdditionalWidgets`, `Views/Vehicle/Report/_ReportWidgets.cshtml` | widgets.html | – | B | [VERIFIZIERT] | nur mit `LUBELOGGER_CUSTOM_WIDGETS` |
| F-049 | Webhooks | Ereignis je Datenänderung an URL/Discord | `/setup` | `Logic/Event/EventLogic.cs`, `Models/Shared/WebHookPayload.cs` | – | – | – | [VERIFIZIERT] | BR-054 |
| F-050 | WebSocket-Events | Live-Aktualisierung von UI/Kiosk | automatisch | `Logic/Event/EventHubLogic.cs`, `EventLogic.cs:29-37` | – | ✓ (`/api/ws`) | – | [VERIFIZIERT] | |
| F-051 | Automatisierte Ereignisse & Benachrichtigungen | Täglicher Job: Reminder-Mails, Zustandswechsel an E-Mail/externe Dienste, Backup-Mail, Gebühren, Cleanup | `/setup` → Notifications | `Logic/Event/AutomatedEventLogic.cs`, `Logic/Event/NotificationLogic.cs`, `Views/Home/_NotificationServiceConfig.cshtml` | NotificationConfig | – | – | [VERIFIZIERT] | BR-034, BR-055 |
| F-052 | E-Mail | SMTP-Versand (Tokens, Resets, Reminder, Backup) mit HTML-Template | automatisch/Admin | `Helper/MailHelper.cs`, `wwwroot/defaults/reminderemailtemplate.txt` | MailConfig | T (`reminders/send`) | – | [VERIFIZIERT] | |
| F-053 | Backup/Restore | ZIP-Sicherung und Wiederherstellung | Einstellungen → Backup | `FilesController.MakeBackup/RestoreBackup`, `FileHelper.cs`, `APIController.MakeBackup` | – | ✓ (Backup) | B | [VERIFIZIERT] (ausgeführt) | PG-Daten nicht enthalten |
| F-054 | LiteDB↔PostgreSQL-Migration | Datenbestand zwischen Backends übertragen | `/Migration` (Root) | `Controllers/MigrationController.cs`, `Views/Migration/Index.cshtml` | alle | – | LiteDB-Datei | [VERIFIZIERT] | |
| F-055 | Health/Info/Version | Betriebsstatus, Locale, Update-Check | `/health`, `/api/info`, `/api/version` | `APIController.cs:135-198`, `External/*/DBHealthCheck.cs` | ServerHealth | ✓ | – | [VERIFIZIERT] (ausgeführt) | |
| F-056 | Aufräumen | Temp leeren, unreferenzierte Dateien löschen | Einstellungen/API/Automatik | `APIController.CleanUp`, `FileHelper.ClearTempFolder/ClearUnlinked*` | – | ✓ | – | [VERIFIZIERT] | |
| F-057 | Demo-Wiederherstellung | Demo-Datensatz einspielen | `/api/demo/restore` | `APIController.RestoreDemo`, `wwwroot/defaults/demo_default.zip` | – | ✓ (undokumentiert) | – | [VERIFIZIERT] | nur für Demo-Instanz relevant [ABGELEITET] |
| F-058 | Tabellen-/UI-Präferenzen | Spaltenauswahl/-reihenfolge, Grid-Ansicht, Dialog-Verhalten | Tabellen-Kontextmenü | `VehicleController.SaveUserColumnPreferences`, `Views/Shared/_UserColumnPreferences.cshtml` | UserColumnPreference | – | – | [VERIFIZIERT] | |
| F-059 | Kultur/Locale | Serverweite Datums-/Zahlen-/Währungsformate, Vorschau | `/setup` | `Program.cs:23-33`, `HomeController.GetLocaleSample` | – | T (`/api/info`) | – | [VERIFIZIERT] | |
| F-060 | Sponsoren-Anzeige | Liste von Unterstützern aus GitHub | Menü „Sponsors“ | `HomeController.Sponsors` | – | – | – | [VERIFIZIERT] | produktfremd, entfällt [ABGELEITET] |
| F-061 | Fehlerseiten | 401/403/404/500-Seiten | automatisch | `Controllers/ErrorController.cs`, `Views/Error/*` | – | – | – | [VERIFIZIERT] | |

## Nicht vorhandene Funktionen (Abgleich mit Zielbild)

| Zielthema (Auftrag) | Befund im Ist | Status |
|---|---|---|
| Öl-Tracking (Stand, Nachfüllung, Verbrauch/1 000 km) | nicht vorhanden; nur als Service-/Notiz-Eintrag oder Extra-Field abbildbar | [VERIFIZIERT] (kein Record-Typ, `Enum/ImportMode.cs`) |
| Fahrten/Fahrtenbuch | nicht vorhanden | [VERIFIZIERT] |
| Wartungsdefinitionen mit Herstellerquelle | nur Reminder ohne Quellenverweis | [VERIFIZIERT] |
| Dokumentenverwaltung mit Typen / KI-Analyse | nicht vorhanden | [VERIFIZIERT] |
| Beweisfotos (Hash, EXIF) | nicht vorhanden | [VERIFIZIERT] |
| Audit-Log | nicht vorhanden | [VERIFIZIERT] |
| Push-Benachrichtigungen (mobil) | nur über konfigurierbare HTTP-Dienste (z. B. ntfy) | [VERIFIZIERT] |
| Native App / Offline | nicht vorhanden (PWA-Manifest `wwwroot/manifest.json`) | [VERIFIZIERT] |

## Anhang A – Vollständigkeits-Checkliste aller Controller-Aktionen

Automatisch aus dem Quellcode extrahiert (alle `public`-Methoden der Controller, 329 Aktionen, 35 Dateien). Spalte „Filter“: `Collab(P)` = `CollaboratorFilter` mit Mindestrecht P (`multi` = Liste `vehicleIds`), `StrictCollab` = nur direkte Kollaboratoren, `APIKey(P)`, `Root` = `[Authorize(Roles=IsRootUser)]`, `QueryParam` = `vehicleId` aus JSON-Body. Klassenebene: `APIController`, `HomeController`, `VehicleController`, `FilesController`, `KioskController` → `[Authorize]`; `AdminController` → Rolle `IsAdmin`; `MigrationController` → Rolle `IsRootUser`; `LoginController`, `ErrorController`, `SharedDataController` → ohne Klassen-Attribut. Aktionen ohne Filter prüfen Rechte (falls überhaupt) im Methodenrumpf. Status aller Zeilen: [VERIFIZIERT].

| Datei:Zeile | Aktion | HTTP | Pfad | Filter | Feature |
|---|---|---|---|---|---|
| `Controllers/API/EquipmentController.cs:12` | AllEquipmentRecords | Get | `/api/vehicle/equipmentrecords/all` | – | F-042, F-022 |
| `Controllers/API/EquipmentController.cs:51` | EquipmentRecords | Get | `/api/vehicle/equipmentrecords` | Collab(View) | F-042, F-022 |
| `Controllers/API/EquipmentController.cs:87` | AddEquipmentRecordJson | Post | `/api/vehicle/equipmentrecords/add` | Collab(Edit), APIKey(Edit), QueryParam | F-042, F-022 |
| `Controllers/API/EquipmentController.cs:92` | AddEquipmentRecord | Post | `/api/vehicle/equipmentrecords/add` | Collab(Edit), APIKey(Edit) | F-042, F-022 |
| `Controllers/API/EquipmentController.cs:138` | DeleteEquipmentRecord | Delete | `/api/vehicle/equipmentrecords/delete` | APIKey(Delete) | F-042, F-022 |
| `Controllers/API/EquipmentController.cs:174` | UpdateEquipmentRecordJson | Put | `/api/vehicle/equipmentrecords/update` | APIKey(Edit) | F-042, F-022 |
| `Controllers/API/EquipmentController.cs:178` | UpdateEquipmentRecord | Put | `/api/vehicle/equipmentrecords/update` | APIKey(Edit) | F-042, F-022 |
| `Controllers/API/GasController.cs:12` | AllGasRecords | Get | `/api/vehicle/gasrecords/all` | – | F-042, F-008 |
| `Controllers/API/GasController.cs:73` | GasRecords | Get | `/api/vehicle/gasrecords` | Collab(View) | F-042, F-008 |
| `Controllers/API/GasController.cs:135` | AddGasRecordJson | Post | `/api/vehicle/gasrecords/add` | Collab(Edit), APIKey(Edit), QueryParam | F-042, F-008 |
| `Controllers/API/GasController.cs:140` | AddGasRecord | Post | `/api/vehicle/gasrecords/add` | Collab(Edit), APIKey(Edit) | F-042, F-008 |
| `Controllers/API/GasController.cs:209` | DeleteGasRecord | Delete | `/api/vehicle/gasrecords/delete` | APIKey(Delete) | F-042, F-008 |
| `Controllers/API/GasController.cs:234` | UpdateGasRecordJson | Put | `/api/vehicle/gasrecords/update` | APIKey(Edit) | F-042, F-008 |
| `Controllers/API/GasController.cs:238` | UpdateGasRecord | Put | `/api/vehicle/gasrecords/update` | APIKey(Edit) | F-042, F-008 |
| `Controllers/API/NoteController.cs:12` | AllNotes | Get | `/api/vehicle/notes/all` | – | F-042, F-020 |
| `Controllers/API/NoteController.cs:59` | Notes | Get | `/api/vehicle/notes` | Collab(View) | F-042, F-020 |
| `Controllers/API/NoteController.cs:104` | AddNoteJson | Post | `/api/vehicle/notes/add` | Collab(Edit), APIKey(Edit), QueryParam | F-042, F-020 |
| `Controllers/API/NoteController.cs:109` | AddNote | Post | `/api/vehicle/notes/add` | Collab(Edit), APIKey(Edit) | F-042, F-020 |
| `Controllers/API/NoteController.cs:155` | DeleteNote | Delete | `/api/vehicle/notes/delete` | APIKey(Delete) | F-042, F-020 |
| `Controllers/API/NoteController.cs:180` | UpdateNoteJson | Put | `/api/vehicle/notes/update` | APIKey(Edit) | F-042, F-020 |
| `Controllers/API/NoteController.cs:184` | UpdateNote | Put | `/api/vehicle/notes/update` | APIKey(Edit) | F-042, F-020 |
| `Controllers/API/OdometerController.cs:13` | LastOdometer | Get | `/api/vehicle/odometerrecords/latest` | Collab(View) | F-042, F-003 |
| `Controllers/API/OdometerController.cs:28` | RecalculateDistance | Put | `/api/vehicle/odometerrecords/recalculate` | Collab(Edit), APIKey(Edit) | F-004 |
| `Controllers/API/OdometerController.cs:42` | AllOdometerRecords | Get | `/api/vehicle/odometerrecords/all` | – | F-042, F-003 |
| `Controllers/API/OdometerController.cs:86` | OdometerRecords | Get | `/api/vehicle/odometerrecords` | Collab(View) | F-042, F-003 |
| `Controllers/API/OdometerController.cs:133` | AddOdometerRecordJson | Post | `/api/vehicle/odometerrecords/add` | Collab(Edit), APIKey(Edit), QueryParam | F-042, F-003 |
| `Controllers/API/OdometerController.cs:138` | AddOdometerRecord | Post | `/api/vehicle/odometerrecords/add` | Collab(Edit), APIKey(Edit) | F-042, F-003 |
| `Controllers/API/OdometerController.cs:228` | DeleteOdometerRecord | Delete | `/api/vehicle/odometerrecords/delete` | APIKey(Delete) | F-042, F-003 |
| `Controllers/API/OdometerController.cs:253` | UpdateOdometerRecordJson | Put | `/api/vehicle/odometerrecords/update` | APIKey(Edit) | F-042, F-003 |
| `Controllers/API/OdometerController.cs:257` | UpdateOdometerRecord | Put | `/api/vehicle/odometerrecords/update` | APIKey(Edit) | F-042, F-003 |
| `Controllers/API/PlanController.cs:12` | AllPlanRecords | Get | `/api/vehicle/planrecords/all` | – | F-042, F-018 |
| `Controllers/API/PlanController.cs:65` | PlanRecords | Get | `/api/vehicle/planrecords` | Collab(View) | F-042, F-018 |
| `Controllers/API/PlanController.cs:116` | AddPlanRecordJson | Post | `/api/vehicle/planrecords/add` | Collab(Edit), APIKey(Edit), QueryParam | F-042, F-018 |
| `Controllers/API/PlanController.cs:121` | AddPlanRecord | Post | `/api/vehicle/planrecords/add` | Collab(Edit), APIKey(Edit) | F-042, F-018 |
| `Controllers/API/PlanController.cs:193` | DeletePlanRecord | Delete | `/api/vehicle/planrecords/delete` | APIKey(Delete) | F-042, F-018 |
| `Controllers/API/PlanController.cs:223` | UpdatePlanRecordJson | Put | `/api/vehicle/planrecords/update` | APIKey(Edit) | F-042, F-018 |
| `Controllers/API/PlanController.cs:227` | UpdatePlanRecord | Put | `/api/vehicle/planrecords/update` | APIKey(Edit) | F-042, F-018 |
| `Controllers/API/ReminderController.cs:12` | AllReminders | Get | `/api/vehicle/reminders/all` | – | F-042, F-015 |
| `Controllers/API/ReminderController.cs:56` | Reminders | Get | `/api/vehicle/reminders` | Collab(View) | F-042, F-015 |
| `Controllers/API/ReminderController.cs:97` | AddReminderRecordJson | Post | `/api/vehicle/reminders/add` | Collab(Edit), APIKey(Edit), QueryParam | F-042, F-015 |
| `Controllers/API/ReminderController.cs:102` | AddReminderRecord | Post | `/api/vehicle/reminders/add` | Collab(Edit), APIKey(Edit) | F-042, F-015 |
| `Controllers/API/ReminderController.cs:172` | UpdateReminderRecordJson | Put | `/api/vehicle/reminders/update` | APIKey(Edit) | F-042, F-015 |
| `Controllers/API/ReminderController.cs:176` | UpdateReminderRecord | Put | `/api/vehicle/reminders/update` | APIKey(Edit) | F-042, F-015 |
| `Controllers/API/ReminderController.cs:253` | DeleteReminderRecord | Delete | `/api/vehicle/reminders/delete` | APIKey(Delete) | F-042, F-015 |
| `Controllers/API/ReminderController.cs:276` | Calendar | Get | `/api/calendar` | – | F-017 |
| `Controllers/API/RepairController.cs:12` | AllRepairRecords | Get | `/api/vehicle/repairrecords/all` | – | F-042, F-011 |
| `Controllers/API/RepairController.cs:56` | RepairRecords | Get | `/api/vehicle/repairrecords` | Collab(View) | F-042, F-011 |
| `Controllers/API/RepairController.cs:98` | AddRepairRecordJson | Post | `/api/vehicle/repairrecords/add` | Collab(Edit), APIKey(Edit), QueryParam | F-042, F-011 |
| `Controllers/API/RepairController.cs:103` | AddRepairRecord | Post | `/api/vehicle/repairrecords/add` | Collab(Edit), APIKey(Edit) | F-042, F-011 |
| `Controllers/API/RepairController.cs:166` | DeleteRepairRecord | Delete | `/api/vehicle/repairrecords/delete` | APIKey(Delete) | F-042, F-011 |
| `Controllers/API/RepairController.cs:196` | UpdateRepairRecordJson | Put | `/api/vehicle/repairrecords/update` | APIKey(Edit) | F-042, F-011 |
| `Controllers/API/RepairController.cs:200` | UpdateRepairRecord | Put | `/api/vehicle/repairrecords/update` | APIKey(Edit) | F-042, F-011 |
| `Controllers/API/ServiceController.cs:12` | AllServiceRecords | Get | `/api/vehicle/servicerecords/all` | – | F-042, F-010 |
| `Controllers/API/ServiceController.cs:56` | ServiceRecords | Get | `/api/vehicle/servicerecords` | Collab(View) | F-042, F-010 |
| `Controllers/API/ServiceController.cs:98` | AddServiceRecordJson | Post | `/api/vehicle/servicerecords/add` | Collab(Edit), APIKey(Edit), QueryParam | F-042, F-010 |
| `Controllers/API/ServiceController.cs:103` | AddServiceRecord | Post | `/api/vehicle/servicerecords/add` | Collab(Edit), APIKey(Edit) | F-042, F-010 |
| `Controllers/API/ServiceController.cs:165` | DeleteServiceRecord | Delete | `/api/vehicle/servicerecords/delete` | APIKey(Delete) | F-042, F-010 |
| `Controllers/API/ServiceController.cs:195` | UpdateServiceRecordJson | Put | `/api/vehicle/servicerecords/update` | APIKey(Edit) | F-042, F-010 |
| `Controllers/API/ServiceController.cs:199` | UpdateServiceRecord | Put | `/api/vehicle/servicerecords/update` | APIKey(Edit) | F-042, F-010 |
| `Controllers/API/SupplyController.cs:12` | AllSupplyRecords | Get | `/api/vehicle/supplyrecords/all` | – | F-042, F-019 |
| `Controllers/API/SupplyController.cs:75` | SupplyRecords | Get | `/api/vehicle/supplyrecords` | Collab(View) | F-042, F-019 |
| `Controllers/API/SupplyController.cs:132` | AddSupplyRecordJson | Post | `/api/vehicle/supplyrecords/add` | Collab(Edit), APIKey(Edit), QueryParam | F-042, F-019 |
| `Controllers/API/SupplyController.cs:137` | AddSupplyRecord | Post | `/api/vehicle/supplyrecords/add` | Collab(Edit), APIKey(Edit) | F-042, F-019 |
| `Controllers/API/SupplyController.cs:189` | DeleteSupplyRecord | Delete | `/api/vehicle/supplyrecords/delete` | APIKey(Delete) | F-042, F-019 |
| `Controllers/API/SupplyController.cs:224` | UpdateSupplyRecordJson | Put | `/api/vehicle/supplyrecords/update` | APIKey(Edit) | F-042, F-019 |
| `Controllers/API/SupplyController.cs:228` | UpdateSupplyRecord | Put | `/api/vehicle/supplyrecords/update` | APIKey(Edit) | F-042, F-019 |
| `Controllers/API/TaxController.cs:12` | AllTaxRecords | Get | `/api/vehicle/taxrecords/all` | – | F-042, F-014 |
| `Controllers/API/TaxController.cs:56` | TaxRecords | Get | `/api/vehicle/taxrecords` | Collab(View) | F-042, F-014 |
| `Controllers/API/TaxController.cs:94` | CheckRecurringTaxRecords | Get | `/api/vehicle/taxrecords/check` | – | F-014 |
| `Controllers/API/TaxController.cs:134` | AddTaxRecordJson | Post | `/api/vehicle/taxrecords/add` | Collab(Edit), APIKey(Edit), QueryParam | F-042, F-014 |
| `Controllers/API/TaxController.cs:139` | AddTaxRecord | Post | `/api/vehicle/taxrecords/add` | Collab(Edit), APIKey(Edit) | F-042, F-014 |
| `Controllers/API/TaxController.cs:188` | DeleteTaxRecord | Delete | `/api/vehicle/taxrecords/delete` | APIKey(Delete) | F-042, F-014 |
| `Controllers/API/TaxController.cs:213` | UpdateTaxRecordJson | Put | `/api/vehicle/taxrecords/update` | APIKey(Edit) | F-042, F-014 |
| `Controllers/API/TaxController.cs:217` | UpdateTaxRecord | Put | `/api/vehicle/taxrecords/update` | APIKey(Edit) | F-042, F-014 |
| `Controllers/API/UpgradeController.cs:12` | AllUpgradeRecords | Get | `/api/vehicle/upgraderecords/all` | – | F-042, F-012 |
| `Controllers/API/UpgradeController.cs:56` | UpgradeRecords | Get | `/api/vehicle/upgraderecords` | Collab(View) | F-042, F-012 |
| `Controllers/API/UpgradeController.cs:98` | AddUpgradeRecordJson | Post | `/api/vehicle/upgraderecords/add` | Collab(Edit), APIKey(Edit), QueryParam | F-042, F-012 |
| `Controllers/API/UpgradeController.cs:103` | AddUpgradeRecord | Post | `/api/vehicle/upgraderecords/add` | Collab(Edit), APIKey(Edit) | F-042, F-012 |
| `Controllers/API/UpgradeController.cs:165` | DeleteUpgradeRecord | Delete | `/api/vehicle/upgraderecords/delete` | APIKey(Delete) | F-042, F-012 |
| `Controllers/API/UpgradeController.cs:195` | UpdateUpgradeRecordJson | Put | `/api/vehicle/upgraderecords/update` | APIKey(Edit) | F-042, F-012 |
| `Controllers/API/UpgradeController.cs:199` | UpdateUpgradeRecord | Put | `/api/vehicle/upgraderecords/update` | APIKey(Edit) | F-042, F-012 |
| `Controllers/APIController.cs:103` | Index | ANY | `/API/Index` | – | F-042 |
| `Controllers/APIController.cs:117` | WhoAmI | Get | `/api/whoami` | – | F-042 |
| `Controllers/APIController.cs:138` | GetServerHealth | Get | `/health` | Anonym | F-055 |
| `Controllers/APIController.cs:158` | GetServerInformation | Get | `/api/info` | – | F-055 |
| `Controllers/APIController.cs:173` | ServerVersion | Get | `/api/version` | – | F-055 |
| `Controllers/APIController.cs:201` | Vehicles | Get | `/api/vehicles` | – | F-001 |
| `Controllers/APIController.cs:220` | VehicleInfo | Get | `/api/vehicle/info` | – | F-002 |
| `Controllers/APIController.cs:256` | AdjustedOdometer | Get | `/api/vehicle/adjustedodometer` | Collab(View) | F-005 |
| `Controllers/APIController.cs:271` | AddVehicleJson | Post | `/api/vehicles/add` | – | F-001 |
| `Controllers/APIController.cs:274` | AddVehicle | Post | `/api/vehicles/add` | – | F-001 |
| `Controllers/APIController.cs:345` | DeleteVehicle | Delete | `/api/vehicles/delete` | APIKey(Delete) | F-001 |
| `Controllers/APIController.cs:369` | UpdateVehicleJson | Put | `/api/vehicles/update` | APIKey(Edit) | F-001 |
| `Controllers/APIController.cs:373` | UpdateVehicle | Put | `/api/vehicles/update` | APIKey(Edit) | F-001 |
| `Controllers/APIController.cs:452` | UploadDocument | Post | `/api/documents/upload` | – | F-023 |
| `Controllers/APIController.cs:485` | SendReminders | Get | `/api/vehicle/reminders/send` | Root | F-052 |
| `Controllers/APIController.cs:592` | GetExtraFields | Get | `/api/extrafields` | – | F-033 |
| `Controllers/APIController.cs:629` | MakeBackup | Get | `/api/makebackup` | Root | F-053 |
| `Controllers/APIController.cs:667` | GetTempFiles | Get | `/api/tempfiles` | Root | F-056 |
| `Controllers/APIController.cs:675` | CleanUp | Get | `/api/cleanup` | Root | F-056 |
| `Controllers/APIController.cs:706` | RestoreDemo | Get | `/api/demo/restore` | Root | F-057 |
| `Controllers/AdminController.cs:21` | Index | ANY | `/Admin/Index` | – | F-038 |
| `Controllers/AdminController.cs:30` | GetTokenPartialView | ANY | `/Admin/GetTokenPartialView` | – | F-038 |
| `Controllers/AdminController.cs:35` | GetUserPartialView | ANY | `/Admin/GetUserPartialView` | – | F-038 |
| `Controllers/AdminController.cs:40` | GenerateNewToken | ANY | `/Admin/GenerateNewToken` | – | F-038 |
| `Controllers/AdminController.cs:67` | DeleteToken | Post | `/Admin/DeleteToken` | – | F-038 |
| `Controllers/AdminController.cs:73` | DeleteUser | Post | `/Admin/DeleteUser` | – | F-038 |
| `Controllers/AdminController.cs:84` | UpdateUserAdminStatus | Post | `/Admin/UpdateUserAdminStatus` | – | F-038 |
| `Controllers/AdminController.cs:90` | GetUserHouseholdModal | Get | `/Admin/GetUserHouseholdModal` | – | F-040 |
| `Controllers/AdminController.cs:102` | RemoveUserFromHousehold | Post | `/Admin/RemoveUserFromHousehold` | – | F-040 |
| `Controllers/AdminController.cs:108` | AddUserToHousehold | Post | `/Admin/AddUserToHousehold` | – | F-040 |
| `Controllers/AdminController.cs:114` | ModifyUserHouseholdPermissions | Post | `/Admin/ModifyUserHouseholdPermissions` | – | F-040 |
| `Controllers/AdminController.cs:120` | RevokeUserPassword | Post | `/Admin/RevokeUserPassword` | – | F-038 |
| `Controllers/AdminController.cs:126` | ResetUserPassword | Post | `/Admin/ResetUserPassword` | – | F-038 |
| `Controllers/ErrorController.cs:9` | Index | ANY | `Error/{statusCode?}` | – | F-061 |
| `Controllers/FilesController.cs:26` | HandleFileUpload | Post | `/Files/HandleFileUpload` | – | F-023 |
| `Controllers/FilesController.cs:33` | HandleTranslationFileUpload | Post | `/Files/HandleTranslationFileUpload` | Root | F-046 |
| `Controllers/FilesController.cs:53` | HandleThemeFileUpload | Post | `/Files/HandleThemeFileUpload` | Root | F-047 |
| `Controllers/FilesController.cs:74` | HandleMultipleFileUpload | Post | `/Files/HandleMultipleFileUpload` | – | F-023 |
| `Controllers/FilesController.cs:86` | DeleteFiles | Post | `/Files/DeleteFiles` | Root | F-023 |
| `Controllers/FilesController.cs:93` | MakeBackup | Get | `/Files/MakeBackup` | Root | F-053 |
| `Controllers/FilesController.cs:100` | RestoreBackup | Post | `/Files/RestoreBackup` | Root | F-053 |
| `Controllers/FilesController.cs:119` | UploadCoordinates | ANY | `/Files/UploadCoordinates` | – | F-024 |
| `Controllers/FilesController.cs:132` | PreviewFile | ANY | `/Files/PreviewFile` | – | F-023 |
| `Controllers/FilesController.cs:142` | GetStaticFile | ANY | `/images/{fileName} /documents/{fileName} /translations/{fileName} /temp/{fileName}` | – | F-023 |
| `Controllers/HomeController.cs:65` | Index | ANY | `/Home/Index` | – | F-044 |
| `Controllers/HomeController.cs:69` | Garage | ANY | `/Home/Garage` | – | F-002 |
| `Controllers/HomeController.cs:125` | Calendar | ANY | `/Home/Calendar` | – | F-017 |
| `Controllers/HomeController.cs:135` | ViewCalendarReminder | ANY | `/Home/ViewCalendarReminder` | – | F-017 |
| `Controllers/HomeController.cs:141` | Settings | ANY | `/Home/Settings` | – | F-044 |
| `Controllers/HomeController.cs:154` | Sponsors | ANY | `/Home/Sponsors` | – | F-060 |
| `Controllers/HomeController.cs:169` | WriteToSettings | Post | `/Home/WriteToSettings` | – | F-044 |
| `Controllers/HomeController.cs:179` | GetExtraFieldsModal | ANY | `/Home/GetExtraFieldsModal` | Root | F-033 |
| `Controllers/HomeController.cs:189` | UpdateExtraFields | ANY | `/Home/UpdateExtraFields` | Root | F-033 |
| `Controllers/HomeController.cs:203` | GenerateTokenForUser | Post | `/Home/GenerateTokenForUser` | – | F-035 |
| `Controllers/HomeController.cs:223` | UpdateUserAccount | Post | `/Home/UpdateUserAccount` | – | F-035 |
| `Controllers/HomeController.cs:242` | GetUserAccountInformationModal | Get | `/Home/GetUserAccountInformationModal` | – | F-035 |
| `Controllers/HomeController.cs:250` | GetHouseholdModal | Get | `/Home/GetHouseholdModal` | – | F-040 |
| `Controllers/HomeController.cs:270` | RemoveUserFromHousehold | Post | `/Home/RemoveUserFromHousehold` | – | F-040 |
| `Controllers/HomeController.cs:276` | LeaveHousehold | Post | `/Home/LeaveHousehold` | – | F-040 |
| `Controllers/HomeController.cs:282` | ModifyUserHouseholdPermissions | Post | `/Home/ModifyUserHouseholdPermissions` | – | F-040 |
| `Controllers/HomeController.cs:288` | AddUserToHousehold | Post | `/Home/AddUserToHousehold` | – | F-040 |
| `Controllers/HomeController.cs:294` | GetUserAPIKeys | Get | `/Home/GetUserAPIKeys` | – | F-041 |
| `Controllers/HomeController.cs:300` | GetCreateApiKeyModal | Get | `/Home/GetCreateApiKeyModal` | – | F-041 |
| `Controllers/HomeController.cs:305` | CreateAPIKeyForUser | Post | `/Home/CreateAPIKeyForUser` | – | F-041 |
| `Controllers/HomeController.cs:311` | DeleteAPIKeyForUser | Post | `/Home/DeleteAPIKeyForUser` | – | F-041 |
| `Controllers/HomeController.cs:318` | GetRootAccountInformationModal | Get | `/Home/GetRootAccountInformationModal` | Root | F-036 |
| `Controllers/HomeController.cs:325` | GetTranslatorEditor | Get | `/Home/GetTranslatorEditor` | Root | F-046 |
| `Controllers/HomeController.cs:332` | SaveTranslation | Post | `/Home/SaveTranslation` | Root | F-046 |
| `Controllers/HomeController.cs:346` | ExportTranslation | Post | `/Home/ExportTranslation` | Root | F-046 |
| `Controllers/HomeController.cs:359` | GetAvailableTranslations | Get | `/Home/GetAvailableTranslations` | Root | F-046 |
| `Controllers/HomeController.cs:375` | DownloadTranslation | Get | `/Home/DownloadTranslation` | Root | F-046 |
| `Controllers/HomeController.cs:404` | DownloadAllTranslations | Get | `/Home/DownloadAllTranslations` | Root | F-046 |
| `Controllers/HomeController.cs:539` | GetVehicleSelector | ANY | `/Home/GetVehicleSelector` | – | F-031 |
| `Controllers/HomeController.cs:561` | GetVehicleSelectorOdometer | ANY | `/Home/GetVehicleSelectorOdometer` | – | F-031 |
| `Controllers/HomeController.cs:581` | GetCustomWidgetEditor | Get | `/Home/GetCustomWidgetEditor` | Root | F-048 |
| `Controllers/HomeController.cs:592` | SaveCustomWidgets | Post | `/Home/SaveCustomWidgets` | Root | F-048 |
| `Controllers/HomeController.cs:603` | DeleteCustomWidgets | Post | `/Home/DeleteCustomWidgets` | Root | F-048 |
| `Controllers/HomeController.cs:613` | GetLocaleSample | ANY | `/Home/GetLocaleSample` | Root | F-059 |
| `Controllers/HomeController.cs:633` | ImportOpenIDConfiguration | ANY | `/Home/ImportOpenIDConfiguration` | Root | F-037 |
| `Controllers/HomeController.cs:652` | Setup | ANY | `/setup` | Root | F-045 |
| `Controllers/HomeController.cs:691` | GetNotificationServiceConfigPartialView | Get | `/Home/GetNotificationServiceConfigPartialView` | Root | F-051 |
| `Controllers/HomeController.cs:697` | WriteServerConfiguration | Post | `/Home/WriteServerConfiguration` | Root | F-045 |
| `Controllers/HomeController.cs:704` | SendTestEmail | Post | `/Home/SendTestEmail` | Root | F-052 |
| `Controllers/HomeController.cs:711` | SendTestNotification | Post | `/Home/SendTestNotification` | Root | F-051 |
| `Controllers/HomeController.cs:742` | Error | ANY | `/Home/Error` | – | F-044 |
| `Controllers/KioskController.cs:36` | Index | ANY | `/Kiosk/Index` | – | F-043 |
| `Controllers/KioskController.cs:54` | KioskContent | Post | `/Kiosk/KioskContent` | – | F-043 |
| `Controllers/KioskController.cs:89` | GetKioskVehicleInfo | ANY | `/Kiosk/GetKioskVehicleInfo` | Collab(View) | F-043 |
| `Controllers/LoginController.cs:38` | Index | ANY | `/Login/Index` | – | F-035 |
| `Controllers/LoginController.cs:77` | Registration | ANY | `/Login/Registration` | – | F-035 |
| `Controllers/LoginController.cs:95` | ForgotPassword | ANY | `/Login/ForgotPassword` | – | F-035 |
| `Controllers/LoginController.cs:104` | ResetPassword | ANY | `/Login/ResetPassword` | – | F-035 |
| `Controllers/LoginController.cs:118` | GetRemoteLoginLink | ANY | `/Login/GetRemoteLoginLink` | – | F-037 |
| `Controllers/LoginController.cs:140` | RemoteAuth | ANY | `/Login/RemoteAuth` | – | F-037 |
| `Controllers/LoginController.cs:329` | RemoteAuthDebug | ANY | `/Login/RemoteAuthDebug` | – | F-037 |
| `Controllers/LoginController.cs:495` | Login | Post | `/Login/Login` | – | F-035 |
| `Controllers/LoginController.cs:546` | Register | Post | `/Login/Register` | – | F-035 |
| `Controllers/LoginController.cs:552` | RegisterOpenIdUser | Post | `/Login/RegisterOpenIdUser` | – | F-037 |
| `Controllers/LoginController.cs:573` | SendRegistrationToken | Post | `/Login/SendRegistrationToken` | – | F-035 |
| `Controllers/LoginController.cs:579` | RequestResetPassword | Post | `/Login/RequestResetPassword` | – | F-035 |
| `Controllers/LoginController.cs:585` | PerformPasswordReset | Post | `/Login/PerformPasswordReset` | – | F-035 |
| `Controllers/LoginController.cs:592` | CreateLoginCreds | Post | `/Login/CreateLoginCreds` | Root | F-036 |
| `Controllers/LoginController.cs:607` | DestroyLoginCreds | Post | `/Login/DestroyLoginCreds` | Root | F-036 |
| `Controllers/LoginController.cs:627` | LogOut | Post | `/Login/LogOut` | – | F-035 |
| `Controllers/MigrationController.cs:24` | Index | ANY | `/Migration/Index` | – | F-054 |
| `Controllers/MigrationController.cs:70` | Export | ANY | `/Migration/Export` | – | F-054 |
| `Controllers/MigrationController.cs:534` | Import | ANY | `/Migration/Import` | – | F-054 |
| `Controllers/SharedDataController.cs:19` | GetConfiguredTheme | ANY | `/css/theme.css` | Anonym | F-047 |
| `Controllers/Vehicle/EquipmentController.cs:12` | GetEquipmentRecordsByVehicleId | Get | `/Vehicle/GetEquipmentRecordsByVehicleId` | Collab(View) | F-022 |
| `Controllers/Vehicle/EquipmentController.cs:23` | SaveEquipmentRecordToVehicleId | Post | `/Vehicle/SaveEquipmentRecordToVehicleId` | – | F-022 |
| `Controllers/Vehicle/EquipmentController.cs:41` | GetAddEquipmentRecordPartialView | Get | `/Vehicle/GetAddEquipmentRecordPartialView` | – | F-022 |
| `Controllers/Vehicle/EquipmentController.cs:46` | GetEquipmentRecordForEditById | Get | `/Vehicle/GetEquipmentRecordForEditById` | – | F-022 |
| `Controllers/Vehicle/EquipmentController.cs:107` | DeleteEquipmentRecordById | Post | `/Vehicle/DeleteEquipmentRecordById` | – | F-022 |
| `Controllers/Vehicle/GasController.cs:12` | GetGasRecordsByVehicleId | Get | `/Vehicle/GetGasRecordsByVehicleId` | Collab(View) | F-008 |
| `Controllers/Vehicle/GasController.cs:36` | SaveGasRecordToVehicleId | Post | `/Vehicle/SaveGasRecordToVehicleId` | – | F-008 |
| `Controllers/Vehicle/GasController.cs:77` | GetAddGasRecordPartialView | Get | `/Vehicle/GetAddGasRecordPartialView` | Collab(View) | F-008 |
| `Controllers/Vehicle/GasController.cs:85` | GetGasRecordForEditById | Get | `/Vehicle/GetGasRecordForEditById` | – | F-008 |
| `Controllers/Vehicle/GasController.cs:143` | DeleteGasRecordById | Post | `/Vehicle/DeleteGasRecordById` | – | F-008 |
| `Controllers/Vehicle/GasController.cs:149` | SaveUserGasTabPreferences | Post | `/Vehicle/SaveUserGasTabPreferences` | – | F-009 |
| `Controllers/Vehicle/GasController.cs:158` | GetGasRecordsEditModal | Post | `/Vehicle/GetGasRecordsEditModal` | – | F-031 |
| `Controllers/Vehicle/GasController.cs:164` | SaveMultipleGasRecords | Post | `/Vehicle/SaveMultipleGasRecords` | – | F-031 |
| `Controllers/Vehicle/ImportController.cs:15` | GetBulkImportModalPartialView | Get | `/Vehicle/GetBulkImportModalPartialView` | – | F-028 |
| `Controllers/Vehicle/ImportController.cs:20` | GenerateCsvSample | Get | `/Vehicle/GenerateCsvSample` | – | F-028 |
| `Controllers/Vehicle/ImportController.cs:217` | GetCSVExportParameters | ANY | `/Vehicle/GetCSVExportParameters` | – | F-029 |
| `Controllers/Vehicle/ImportController.cs:223` | ExportFromVehicleToCsv | Post | `/Vehicle/ExportFromVehicleToCsv` | Collab(View) | F-029 |
| `Controllers/Vehicle/ImportController.cs:679` | ImportToVehicleIdFromCsv | Post | `/Vehicle/ImportToVehicleIdFromCsv` | Collab(Edit) | F-028 |
| `Controllers/Vehicle/InspectionController.cs:12` | GetInspectionRecordTemplatesByVehicleId | Get | `/Vehicle/GetInspectionRecordTemplatesByVehicleId` | Collab(View) | F-021 |
| `Controllers/Vehicle/InspectionController.cs:19` | GetInspectionRecordsByVehicleId | Get | `/Vehicle/GetInspectionRecordsByVehicleId` | Collab(View) | F-021 |
| `Controllers/Vehicle/InspectionController.cs:34` | GetAddInspectionRecordTemplatePartialView | Get | `/Vehicle/GetAddInspectionRecordTemplatePartialView` | – | F-021 |
| `Controllers/Vehicle/InspectionController.cs:39` | GetEditInspectionRecordTemplatePartialView | Get | `/Vehicle/GetEditInspectionRecordTemplatePartialView` | – | F-021 |
| `Controllers/Vehicle/InspectionController.cs:69` | GetAddInspectionRecordFieldPartialView | Get | `/Vehicle/GetAddInspectionRecordFieldPartialView` | – | F-021 |
| `Controllers/Vehicle/InspectionController.cs:73` | GetAddInspectionRecordFieldOptionsPartialView | ANY | `/Vehicle/GetAddInspectionRecordFieldOptionsPartialView` | – | F-021 |
| `Controllers/Vehicle/InspectionController.cs:81` | GetAddInspectionRecordFieldOptionPartialView | ANY | `/Vehicle/GetAddInspectionRecordFieldOptionPartialView` | – | F-021 |
| `Controllers/Vehicle/InspectionController.cs:86` | SaveInspectionRecordTemplateToVehicleId | Post | `/Vehicle/SaveInspectionRecordTemplateToVehicleId` | – | F-021 |
| `Controllers/Vehicle/InspectionController.cs:123` | DeleteInspectionRecordTemplateById | Post | `/Vehicle/DeleteInspectionRecordTemplateById` | – | F-021 |
| `Controllers/Vehicle/InspectionController.cs:129` | DeleteInspectionRecordById | Post | `/Vehicle/DeleteInspectionRecordById` | – | F-021 |
| `Controllers/Vehicle/InspectionController.cs:135` | GetAddInspectionRecordPartialView | Get | `/Vehicle/GetAddInspectionRecordPartialView` | – | F-021 |
| `Controllers/Vehicle/InspectionController.cs:167` | GetViewInspectionRecordPartialView | Get | `/Vehicle/GetViewInspectionRecordPartialView` | – | F-021 |
| `Controllers/Vehicle/InspectionController.cs:178` | SaveInspectionRecordToVehicleId | Post | `/Vehicle/SaveInspectionRecordToVehicleId` | – | F-021 |
| `Controllers/Vehicle/InspectionController.cs:256` | UpdateInspectionRecord | Post | `/Vehicle/UpdateInspectionRecord` | – | F-021 |
| `Controllers/Vehicle/NoteController.cs:12` | GetNotesByVehicleId | Get | `/Vehicle/GetNotesByVehicleId` | Collab(View) | F-020 |
| `Controllers/Vehicle/NoteController.cs:20` | GetPinnedNotesByVehicleId | Get | `/Vehicle/GetPinnedNotesByVehicleId` | Collab(View) | F-020 |
| `Controllers/Vehicle/NoteController.cs:27` | SaveNoteToVehicleId | Post | `/Vehicle/SaveNoteToVehicleId` | – | F-020 |
| `Controllers/Vehicle/NoteController.cs:44` | GetAddNotePartialView | Get | `/Vehicle/GetAddNotePartialView` | – | F-020 |
| `Controllers/Vehicle/NoteController.cs:50` | GetNoteForEditById | Get | `/Vehicle/GetNoteForEditById` | – | F-020 |
| `Controllers/Vehicle/NoteController.cs:77` | DeleteNoteById | Post | `/Vehicle/DeleteNoteById` | – | F-020 |
| `Controllers/Vehicle/NoteController.cs:83` | PinNotes | Post | `/Vehicle/PinNotes` | – | F-020 |
| `Controllers/Vehicle/OdometerController.cs:11` | ForceRecalculateDistanceByVehicleId | Post | `/Vehicle/ForceRecalculateDistanceByVehicleId` | – | F-004 |
| `Controllers/Vehicle/OdometerController.cs:24` | GetOdometerRecordsByVehicleId | Get | `/Vehicle/GetOdometerRecordsByVehicleId` | Collab(View) | F-003 |
| `Controllers/Vehicle/OdometerController.cs:44` | SaveOdometerRecordToVehicleId | Post | `/Vehicle/SaveOdometerRecordToVehicleId` | – | F-003 |
| `Controllers/Vehicle/OdometerController.cs:63` | GetAddOdometerRecordPartialView | Get | `/Vehicle/GetAddOdometerRecordPartialView` | Collab(View) | F-003 |
| `Controllers/Vehicle/OdometerController.cs:72` | GetOdometerRecordsEditModal | Post | `/Vehicle/GetOdometerRecordsEditModal` | – | F-031 |
| `Controllers/Vehicle/OdometerController.cs:87` | SaveMultipleOdometerRecords | Post | `/Vehicle/SaveMultipleOdometerRecords` | – | F-031 |
| `Controllers/Vehicle/OdometerController.cs:159` | GetOdometerRecordForEditById | Get | `/Vehicle/GetOdometerRecordForEditById` | – | F-003 |
| `Controllers/Vehicle/OdometerController.cs:205` | DeleteOdometerRecordById | Post | `/Vehicle/DeleteOdometerRecordById` | – | F-003 |
| `Controllers/Vehicle/OdometerController.cs:212` | DuplicateDistanceToOtherVehicles | Post | `/Vehicle/DuplicateDistanceToOtherVehicles` | Collab(Edit, multi) | F-007 |
| `Controllers/Vehicle/PlanController.cs:12` | GetPlanRecordsByVehicleId | Get | `/Vehicle/GetPlanRecordsByVehicleId` | Collab(View) | F-018 |
| `Controllers/Vehicle/PlanController.cs:18` | SavePlanRecordToVehicleId | Post | `/Vehicle/SavePlanRecordToVehicleId` | – | F-018 |
| `Controllers/Vehicle/PlanController.cs:53` | SavePlanRecordTemplateToVehicleId | Post | `/Vehicle/SavePlanRecordTemplateToVehicleId` | – | F-018 |
| `Controllers/Vehicle/PlanController.cs:72` | GetPlanRecordTemplatesForVehicleId | Get | `/Vehicle/GetPlanRecordTemplatesForVehicleId` | Collab(View) | F-018 |
| `Controllers/Vehicle/PlanController.cs:78` | DeletePlanRecordTemplateById | Post | `/Vehicle/DeletePlanRecordTemplateById` | – | F-018 |
| `Controllers/Vehicle/PlanController.cs:94` | OrderPlanSupplies | Get | `/Vehicle/OrderPlanSupplies` | – | F-018/F-019 |
| `Controllers/Vehicle/PlanController.cs:117` | ConvertPlanRecordTemplateToPlanRecord | Post | `/Vehicle/ConvertPlanRecordTemplateToPlanRecord` | – | F-018 |
| `Controllers/Vehicle/PlanController.cs:182` | ConvertPlanRecordToPlanRecordTemplate | Post | `/Vehicle/ConvertPlanRecordToPlanRecordTemplate` | – | F-018 |
| `Controllers/Vehicle/PlanController.cs:263` | GetAddPlanRecordPartialView | Get | `/Vehicle/GetAddPlanRecordPartialView` | – | F-018 |
| `Controllers/Vehicle/PlanController.cs:268` | GetAddPlanRecordPartialView | Post | `/Vehicle/GetAddPlanRecordPartialView` | – | F-018 |
| `Controllers/Vehicle/PlanController.cs:278` | UpdatePlanRecordProgress | Post | `/Vehicle/UpdatePlanRecordProgress` | – | F-018 |
| `Controllers/Vehicle/PlanController.cs:380` | GetPlanRecordTemplateForEditById | Get | `/Vehicle/GetPlanRecordTemplateForEditById` | – | F-018 |
| `Controllers/Vehicle/PlanController.cs:391` | GetPlanRecordForEditById | Get | `/Vehicle/GetPlanRecordForEditById` | – | F-018 |
| `Controllers/Vehicle/PlanController.cs:421` | DeletePlanRecordById | Post | `/Vehicle/DeletePlanRecordById` | – | F-018 |
| `Controllers/Vehicle/ReminderController.cs:50` | GetVehicleHaveUrgentOrPastDueReminders | Get | `/Vehicle/GetVehicleHaveUrgentOrPastDueReminders` | Collab(View) | F-015 |
| `Controllers/Vehicle/ReminderController.cs:57` | GetReminderRecordsByVehicleId | Get | `/Vehicle/GetReminderRecordsByVehicleId` | Collab(View) | F-015 |
| `Controllers/Vehicle/ReminderController.cs:65` | GetRecurringReminderRecordsByVehicleId | Get | `/Vehicle/GetRecurringReminderRecordsByVehicleId` | Collab(View) | F-015 |
| `Controllers/Vehicle/ReminderController.cs:73` | PushbackRecurringReminderRecord | Post | `/Vehicle/PushbackRecurringReminderRecord` | – | F-016 |
| `Controllers/Vehicle/ReminderController.cs:113` | SaveReminderRecordToVehicleId | Post | `/Vehicle/SaveReminderRecordToVehicleId` | – | F-015 |
| `Controllers/Vehicle/ReminderController.cs:128` | GetAddReminderRecordPartialView | Post | `/Vehicle/GetAddReminderRecordPartialView` | – | F-015 |
| `Controllers/Vehicle/ReminderController.cs:142` | GetReminderRecordForEditById | Get | `/Vehicle/GetReminderRecordForEditById` | – | F-015 |
| `Controllers/Vehicle/ReminderController.cs:192` | DeleteReminderRecordById | Post | `/Vehicle/DeleteReminderRecordById` | – | F-015 |
| `Controllers/Vehicle/RepairController.cs:12` | GetCollisionRecordsByVehicleId | Get | `/Vehicle/GetCollisionRecordsByVehicleId` | Collab(View) | F-011 |
| `Controllers/Vehicle/RepairController.cs:27` | SaveCollisionRecordToVehicleId | Post | `/Vehicle/SaveCollisionRecordToVehicleId` | – | F-011 |
| `Controllers/Vehicle/RepairController.cs:76` | GetAddCollisionRecordPartialView | Get | `/Vehicle/GetAddCollisionRecordPartialView` | – | F-011 |
| `Controllers/Vehicle/RepairController.cs:81` | GetCollisionRecordForEditById | Get | `/Vehicle/GetCollisionRecordForEditById` | – | F-011 |
| `Controllers/Vehicle/RepairController.cs:127` | DeleteCollisionRecordById | Post | `/Vehicle/DeleteCollisionRecordById` | – | F-011 |
| `Controllers/Vehicle/ReportController.cs:14` | GetReportPartialView | Get | `/Vehicle/GetReportPartialView` | Collab(View) | F-025 |
| `Controllers/Vehicle/ReportController.cs:176` | GetCollaboratorsForVehicle | Get | `/Vehicle/GetCollaboratorsForVehicle` | Collab(View) | F-039 |
| `Controllers/Vehicle/ReportController.cs:189` | AddCollaboratorsToVehicle | Post | `/Vehicle/AddCollaboratorsToVehicle` | StrictCollab | F-039 |
| `Controllers/Vehicle/ReportController.cs:196` | DeleteCollaboratorFromVehicle | Post | `/Vehicle/DeleteCollaboratorFromVehicle` | StrictCollab | F-039 |
| `Controllers/Vehicle/ReportController.cs:203` | GetSummaryForVehicle | Post | `/Vehicle/GetSummaryForVehicle` | Collab(View) | F-025 |
| `Controllers/Vehicle/ReportController.cs:259` | GetCostMakeUpForVehicle | Get | `/Vehicle/GetCostMakeUpForVehicle` | Collab(View) | F-025 |
| `Controllers/Vehicle/ReportController.cs:286` | GetCostTableForVehicle | Get | `/Vehicle/GetCostTableForVehicle` | Collab(View) | F-025 |
| `Controllers/Vehicle/ReportController.cs:324` | GetVehicleImageMap | ANY | `/Vehicle/GetVehicleImageMap` | Collab(View) | F-024 |
| `Controllers/Vehicle/ReportController.cs:340` | GetReminderMakeUpByVehicle | ANY | `/Vehicle/GetReminderMakeUpByVehicle` | Collab(View) | F-025 |
| `Controllers/Vehicle/ReportController.cs:354` | GetVehicleAttachments | Post | `/Vehicle/GetVehicleAttachments` | Collab(View) | F-027 |
| `Controllers/Vehicle/ReportController.cs:493` | GetReportParameters | ANY | `/Vehicle/GetReportParameters` | – | F-026 |
| `Controllers/Vehicle/ReportController.cs:517` | GetVehicleHistory | ANY | `/Vehicle/GetVehicleHistory` | Collab(View) | F-026 |
| `Controllers/Vehicle/ReportController.cs:687` | GetMonthMPGByVehicle | Post | `/Vehicle/GetMonthMPGByVehicle` | Collab(View) | F-025 |
| `Controllers/Vehicle/ReportController.cs:733` | GetCostByMonthByVehicle | Post | `/Vehicle/GetCostByMonthByVehicle` | Collab(View) | F-025 |
| `Controllers/Vehicle/ReportController.cs:776` | GetCostByMonthAndYearByVehicle | Post | `/Vehicle/GetCostByMonthAndYearByVehicle` | Collab(View) | F-025 |
| `Controllers/Vehicle/ReportController.cs:824` | GetAdditionalWidgets | Get | `/Vehicle/GetAdditionalWidgets` | – | F-048 |
| `Controllers/Vehicle/ReportController.cs:830` | GetImportModeSelector | Get | `/Vehicle/GetImportModeSelector` | – | F-025 |
| `Controllers/Vehicle/ServiceController.cs:12` | GetServiceRecordsByVehicleId | Get | `/Vehicle/GetServiceRecordsByVehicleId` | Collab(View) | F-010 |
| `Controllers/Vehicle/ServiceController.cs:27` | SaveServiceRecordToVehicleId | Post | `/Vehicle/SaveServiceRecordToVehicleId` | – | F-010 |
| `Controllers/Vehicle/ServiceController.cs:76` | GetAddServiceRecordPartialView | Get | `/Vehicle/GetAddServiceRecordPartialView` | – | F-010 |
| `Controllers/Vehicle/ServiceController.cs:81` | GetServiceRecordForEditById | Get | `/Vehicle/GetServiceRecordForEditById` | – | F-010 |
| `Controllers/Vehicle/ServiceController.cs:127` | DeleteServiceRecordById | Post | `/Vehicle/DeleteServiceRecordById` | – | F-010 |
| `Controllers/Vehicle/SupplyController.cs:84` | GetSupplyRecordsByVehicleId | Get | `/Vehicle/GetSupplyRecordsByVehicleId` | Collab(View) | F-019 |
| `Controllers/Vehicle/SupplyController.cs:99` | GetSupplyRecordsForPlanRecordTemplate | Get | `/Vehicle/GetSupplyRecordsForPlanRecordTemplate` | – | F-018 |
| `Controllers/Vehicle/SupplyController.cs:131` | GetSupplyRecordsForRecordsByVehicleId | Get | `/Vehicle/GetSupplyRecordsForRecordsByVehicleId` | Collab(View) | F-019 |
| `Controllers/Vehicle/SupplyController.cs:155` | SaveSupplyRecordToVehicleId | Post | `/Vehicle/SaveSupplyRecordToVehicleId` | – | F-019 |
| `Controllers/Vehicle/SupplyController.cs:179` | GetAddSupplyRecordPartialView | Get | `/Vehicle/GetAddSupplyRecordPartialView` | – | F-019 |
| `Controllers/Vehicle/SupplyController.cs:184` | GetSupplyRecordForEditById | Get | `/Vehicle/GetSupplyRecordForEditById` | – | F-019 |
| `Controllers/Vehicle/SupplyController.cs:246` | DeleteSupplyRecordById | Post | `/Vehicle/DeleteSupplyRecordById` | – | F-019 |
| `Controllers/Vehicle/TaxController.cs:12` | GetTaxRecordsByVehicleId | Get | `/Vehicle/GetTaxRecordsByVehicleId` | Collab(View) | F-014 |
| `Controllers/Vehicle/TaxController.cs:29` | CheckRecurringTaxRecords | Post | `/Vehicle/CheckRecurringTaxRecords` | Collab(View) | F-014 |
| `Controllers/Vehicle/TaxController.cs:42` | SaveTaxRecordToVehicleId | Post | `/Vehicle/SaveTaxRecordToVehicleId` | – | F-014 |
| `Controllers/Vehicle/TaxController.cs:68` | GetAddTaxRecordPartialView | Get | `/Vehicle/GetAddTaxRecordPartialView` | – | F-014 |
| `Controllers/Vehicle/TaxController.cs:73` | GetTaxRecordForEditById | Get | `/Vehicle/GetTaxRecordForEditById` | – | F-014 |
| `Controllers/Vehicle/TaxController.cs:116` | DeleteTaxRecordById | Post | `/Vehicle/DeleteTaxRecordById` | – | F-014 |
| `Controllers/Vehicle/UpgradeController.cs:12` | GetUpgradeRecordsByVehicleId | Get | `/Vehicle/GetUpgradeRecordsByVehicleId` | Collab(View) | F-012 |
| `Controllers/Vehicle/UpgradeController.cs:27` | SaveUpgradeRecordToVehicleId | Post | `/Vehicle/SaveUpgradeRecordToVehicleId` | – | F-012 |
| `Controllers/Vehicle/UpgradeController.cs:76` | GetAddUpgradeRecordPartialView | Get | `/Vehicle/GetAddUpgradeRecordPartialView` | – | F-012 |
| `Controllers/Vehicle/UpgradeController.cs:81` | GetUpgradeRecordForEditById | Get | `/Vehicle/GetUpgradeRecordForEditById` | – | F-012 |
| `Controllers/Vehicle/UpgradeController.cs:127` | DeleteUpgradeRecordById | Post | `/Vehicle/DeleteUpgradeRecordById` | – | F-012 |
| `Controllers/VehicleController.cs:113` | Index | Get | `/Vehicle/Index` | Collab(View) | F-001 |
| `Controllers/VehicleController.cs:119` | AddVehiclePartialView | Get | `/Vehicle/AddVehiclePartialView` | – | F-001 |
| `Controllers/VehicleController.cs:125` | GetEditVehiclePartialViewById | Get | `/Vehicle/GetEditVehiclePartialViewById` | Collab(View) | F-001 |
| `Controllers/VehicleController.cs:132` | SaveVehicle | Post | `/Vehicle/SaveVehicle` | – | F-001 |
| `Controllers/VehicleController.cs:168` | DeleteVehicle | Post | `/Vehicle/DeleteVehicle` | StrictCollab | F-001 |
| `Controllers/VehicleController.cs:180` | DeleteVehicles | Post | `/Vehicle/DeleteVehicles` | StrictCollab(multi) | F-001 |
| `Controllers/VehicleController.cs:197` | GetVehiclesCollaborators | Post | `/Vehicle/GetVehiclesCollaborators` | StrictCollab(multi) | F-039 |
| `Controllers/VehicleController.cs:229` | AddCollaboratorsToVehicles | Post | `/Vehicle/AddCollaboratorsToVehicles` | StrictCollab(multi) | F-039 |
| `Controllers/VehicleController.cs:249` | RemoveCollaboratorsFromVehicles | Post | `/Vehicle/RemoveCollaboratorsFromVehicles` | StrictCollab(multi) | F-039 |
| `Controllers/VehicleController.cs:270` | GetFilesPendingUpload | Post | `/Vehicle/GetFilesPendingUpload` | – | F-023 |
| `Controllers/VehicleController.cs:277` | SearchRecords | Post | `/Vehicle/SearchRecords` | Collab(View) | F-030 |
| `Controllers/VehicleController.cs:455` | SearchRecordsByTags | Post | `/Vehicle/SearchRecordsByTags` | Collab(View) | F-030 |
| `Controllers/VehicleController.cs:628` | CheckRecordExist | Post | `/Vehicle/CheckRecordExist` | Collab(View) | F-023 |
| `Controllers/VehicleController.cs:700` | GetMaxMileage | ANY | `/Vehicle/GetMaxMileage` | Collab(View) | F-003 |
| `Controllers/VehicleController.cs:705` | MoveRecord | ANY | `/Vehicle/MoveRecord` | – | F-013 |
| `Controllers/VehicleController.cs:758` | MoveRecords | ANY | `/Vehicle/MoveRecords` | – | F-013 |
| `Controllers/VehicleController.cs:818` | DeleteRecords | ANY | `/Vehicle/DeleteRecords` | – | F-031 |
| `Controllers/VehicleController.cs:868` | AdjustRecordsOdometer | Post | `/Vehicle/AdjustRecordsOdometer` | Collab(Edit) | F-005 |
| `Controllers/VehicleController.cs:926` | DuplicateRecords | Post | `/Vehicle/DuplicateRecords` | – | F-031 |
| `Controllers/VehicleController.cs:1095` | DuplicateRecordsToOtherVehicles | Post | `/Vehicle/DuplicateRecordsToOtherVehicles` | Collab(Edit, multi) | F-031 |
| `Controllers/VehicleController.cs:1312` | BulkCreateOdometerRecords | Post | `/Vehicle/BulkCreateOdometerRecords` | – | F-006 |
| `Controllers/VehicleController.cs:1400` | GetGenericRecordModal | Post | `/Vehicle/GetGenericRecordModal` | – | F-031 |
| `Controllers/VehicleController.cs:1406` | EditMultipleRecords | Post | `/Vehicle/EditMultipleRecords` | – | F-031 |
| `Controllers/VehicleController.cs:1588` | PrintRecordStickers | Post | `/Vehicle/PrintRecordStickers` | Collab(View) | F-032 |
| `Controllers/VehicleController.cs:1821` | SaveUserColumnPreferences | Post | `/Vehicle/SaveUserColumnPreferences` | – | F-058 |
## Anhang B – Checkliste Entitäten und Modelle

Alle 111 Dateien unter `Models/`; persistierte Entitäten sind in `03-domain-und-datenmodell.md` beschrieben, übrige Dateien sind Input-, View-, Export- oder Konfigurationsmodelle. Status: [VERIFIZIERT] (Dateiliste per Skript erzeugt, Inhalte gelesen).

| Datei | Rolle |
|---|---|
| `Models/API/APIDocumentation.cs` | API-Modell |
| `Models/API/APIKey.cs` | **persistierte Entität** |
| `Models/API/MethodParameter.cs` | API-Modell |
| `Models/API/ReleaseVersion.cs` | API-Modell |
| `Models/API/ServerHealth.cs` | API-Modell |
| `Models/API/ServerInformation.cs` | API-Modell |
| `Models/API/TypeConverter.cs` | API-Modell |
| `Models/API/VehicleInfo.cs` | API-Modell |
| `Models/Admin/AdminViewModel.cs` | View-Modell |
| `Models/Collision/CollisionRecord.cs` | **persistierte Entität** |
| `Models/Collision/CollisionRecordInput.cs` | Input-Modell |
| `Models/Configuration/MailConfig.cs` | Konfiguration |
| `Models/EquipmentRecord/EquipmentRecord.cs` | **persistierte Entität** |
| `Models/EquipmentRecord/EquipmentRecordInput.cs` | Input-Modell |
| `Models/EquipmentRecord/EquipmentRecordStickerViewModel.cs` | View-Modell |
| `Models/EquipmentRecord/EquipmentRecordViewModel.cs` | View-Modell |
| `Models/ErrorViewModel.cs` | View-Modell |
| `Models/GasRecord/GasRecord.cs` | **persistierte Entität** |
| `Models/GasRecord/GasRecordEditModel.cs` | Input-Modell |
| `Models/GasRecord/GasRecordInput.cs` | Input-Modell |
| `Models/GasRecord/GasRecordInputContainer.cs` | Input-Modell |
| `Models/GasRecord/GasRecordViewModel.cs` | View-Modell |
| `Models/GasRecord/GasRecordViewModelContainer.cs` | View-Modell |
| `Models/InspectionRecord/InspectionRecord.cs` | **persistierte Entität** |
| `Models/InspectionRecord/InspectionRecordInput.cs` | Input-Modell, **als Vorlage persistiert** |
| `Models/InspectionRecord/InspectionRecordTemplateField.cs` | **persistiert (Basis/eingebettet)** |
| `Models/Kiosk/KioskPlanViewModel.cs` | View-Modell |
| `Models/Kiosk/KioskReminderViewModel.cs` | View-Modell |
| `Models/Kiosk/KioskVehicleViewModel.cs` | View-Modell |
| `Models/Kiosk/KioskViewModel.cs` | View-Modell |
| `Models/Login/AuthCookie.cs` | View-/Hilfsmodell |
| `Models/Login/LoginModel.cs` | View-/Hilfsmodell |
| `Models/Login/OpenIDRegistrationModel.cs` | View-/Hilfsmodell |
| `Models/Login/Token.cs` | **persistierte Entität** |
| `Models/Note/Note.cs` | **persistierte Entität** |
| `Models/OIDC/OpenIDConfig.cs` | Konfiguration |
| `Models/OIDC/OpenIDProviderConfig.cs` | Konfiguration |
| `Models/OIDC/OpenIDResult.cs` | Konfiguration |
| `Models/OIDC/OpenIDUserInfo.cs` | Konfiguration |
| `Models/OdometerRecord/OdometerRecord.cs` | **persistierte Entität** |
| `Models/OdometerRecord/OdometerRecordEditModel.cs` | Input-Modell |
| `Models/OdometerRecord/OdometerRecordInput.cs` | Input-Modell |
| `Models/PlanRecord/PlanRecord.cs` | **persistierte Entität** |
| `Models/PlanRecord/PlanRecordInput.cs` | Input-Modell, **als Vorlage persistiert** |
| `Models/Reminder/CachedReminderRecord.cs` | View-/Hilfsmodell |
| `Models/Reminder/ReminderConfig.cs` | **persistiert (Basis/eingebettet)** |
| `Models/Reminder/ReminderRecord.cs` | **persistierte Entität** |
| `Models/Reminder/ReminderRecordInput.cs` | Input-Modell |
| `Models/Reminder/ReminderRecordInputViewModel.cs` | Input-Modell |
| `Models/Reminder/ReminderRecordViewModel.cs` | View-Modell |
| `Models/Report/CostDistanceTableForVehicle.cs` | Report-Modell |
| `Models/Report/CostForVehicleByMonth.cs` | Report-Modell |
| `Models/Report/CostMakeUpForVehicle.cs` | Report-Modell |
| `Models/Report/CostTableForVehicle.cs` | Report-Modell |
| `Models/Report/GenericReportModel.cs` | Report-Modell |
| `Models/Report/MPGForVehicleByMonth.cs` | Report-Modell |
| `Models/Report/ReminderMakeUpForVehicle.cs` | Report-Modell |
| `Models/Report/ReportHeader.cs` | Report-Modell |
| `Models/Report/ReportParameter.cs` | Report-Modell |
| `Models/Report/ReportViewModel.cs` | View-Modell |
| `Models/Report/VehicleHistoryViewModel.cs` | View-Modell |
| `Models/ServiceRecord/ServiceRecord.cs` | **persistierte Entität** |
| `Models/ServiceRecord/ServiceRecordInput.cs` | Input-Modell |
| `Models/Settings/KestrelAppConfig.cs` | Konfiguration |
| `Models/Settings/LocaleSample.cs` | Konfiguration |
| `Models/Settings/NotificationConfig.cs` | Konfiguration |
| `Models/Settings/ServerConfig.cs` | Konfiguration |
| `Models/Settings/ServerSettingsViewModel.cs` | View-Modell |
| `Models/Settings/SettingsViewModel.cs` | View-Modell |
| `Models/Settings/Sponsors.cs` | Konfiguration |
| `Models/Settings/Translations.cs` | Konfiguration |
| `Models/Settings/UserConfig.cs` | Konfiguration |
| `Models/Shared/CSVExportParameter.cs` | View-/Hilfsmodell |
| `Models/Shared/ExtraField.cs` | **persistiert (Basis/eingebettet)** |
| `Models/Shared/GenericRecord.cs` | **persistiert (Basis/eingebettet)** |
| `Models/Shared/GenericRecordEditModel.cs` | Input-Modell |
| `Models/Shared/ImportModel.cs` | Import-/Export-Modelle |
| `Models/Shared/OperationResponse.cs` | View-/Hilfsmodell |
| `Models/Shared/RecordExtraField.cs` | **persistierte Entität** |
| `Models/Shared/SearchResult.cs` | View-/Hilfsmodell |
| `Models/Shared/StickerViewModel.cs` | View-Modell |
| `Models/Shared/UploadedFiles.cs` | **persistiert (Basis/eingebettet)** |
| `Models/Shared/VehicleRecords.cs` | View-/Hilfsmodell |
| `Models/Shared/WebHookPayload.cs` | View-/Hilfsmodell |
| `Models/Supply/SupplyAvailability.cs` | View-/Hilfsmodell |
| `Models/Supply/SupplyRecord.cs` | **persistierte Entität** |
| `Models/Supply/SupplyRecordInput.cs` | Input-Modell |
| `Models/Supply/SupplyRequisitionHistory.cs` | View-/Hilfsmodell |
| `Models/Supply/SupplyStore.cs` | View-/Hilfsmodell |
| `Models/Supply/SupplyUsage.cs` | **persistiert (Basis/eingebettet)** |
| `Models/Supply/SupplyUsageHistory.cs` | **persistiert (Basis/eingebettet)** |
| `Models/Supply/SupplyUsageViewModel.cs` | View-Modell |
| `Models/TaxRecord/TaxRecord.cs` | **persistierte Entität** |
| `Models/TaxRecord/TaxRecordInput.cs` | Input-Modell |
| `Models/UpgradeRecord/UpgradeRecord.cs` | **persistierte Entität** |
| `Models/UpgradeRecord/UpgradeReportInput.cs` | Input-Modell |
| `Models/User/UserAccess.cs` | **persistierte Entität** |
| `Models/User/UserCollaborator.cs` | View-/Hilfsmodell |
| `Models/User/UserCollaboratorViewModel.cs` | View-Modell |
| `Models/User/UserColumnPreference.cs` | View-/Hilfsmodell |
| `Models/User/UserConfigData.cs` | **persistierte Entität** |
| `Models/User/UserData.cs` | **persistierte Entität** |
| `Models/User/UserDataViewModel.cs` | View-Modell |
| `Models/User/UserHousehold.cs` | **persistierte Entität** |
| `Models/User/UserHouseholdAdminViewModel.cs` | View-Modell |
| `Models/User/UserHouseholdUserViewModel.cs` | View-Modell |
| `Models/User/UserHouseholdViewModel.cs` | View-Modell |
| `Models/User/VehicleCollaboratorViewModel.cs` | View-Modell |
| `Models/Vehicle/Vehicle.cs` | **persistierte Entität** |
| `Models/Vehicle/VehicleImageMap.cs` | View-/Hilfsmodell |
| `Models/Vehicle/VehicleViewModel.cs` | View-Modell |

## Anhang C – Checkliste Logik-, Datenzugriffs- und Infrastrukturdateien

Alle Dateien unter `External/`, `Logic/`, `Helper/`, `Filter/`, `Middleware/`, `MapProfile/`, `Enum/` sowie `Program.cs`. Status: [VERIFIZIERT] (gelesen; die 44 Data-Access-Implementierungen folgen je Backend einem identischen Muster, stichprobenartig vollständig gelesen: Gas, UserRecord, UserAccess, ExtraField).

| Datei | Dokumentiert in |
|---|---|
| `Program.cs` | 01 |
| `Enum/APIMethodType.cs` | 03 |
| `Enum/AutomatedEvent.cs` | 03 |
| `Enum/DashboardMetric.cs` | 03 |
| `Enum/ExtraFieldType.cs` | 03 |
| `Enum/HouseholdPermission.cs` | 03 |
| `Enum/ImportMode.cs` | 03 |
| `Enum/InspectionFieldType.cs` | 03 |
| `Enum/KioskMode.cs` | 03 |
| `Enum/PlanPriority.cs` | 03 |
| `Enum/PlanProgress.cs` | 03 |
| `Enum/ReminderIntervalUnit.cs` | 03 |
| `Enum/ReminderMetric.cs` | 03 |
| `Enum/ReminderMileageInterval.cs` | 03 |
| `Enum/ReminderMonthInterval.cs` | 03 |
| `Enum/ReminderUrgency.cs` | 03 |
| `Enum/SkippedSetting.cs` | 03 |
| `Enum/TagFilter.cs` | 03 |
| `External/Implementations/Litedb/ApiKeyRecordDataAccess.cs` | 03, 08 |
| `External/Implementations/Litedb/CollisionRecordDataAccess.cs` | 03, 08 |
| `External/Implementations/Litedb/DBHealthCheck.cs` | 03, 08 |
| `External/Implementations/Litedb/EquipmentRecordDataAccess.cs` | 03, 08 |
| `External/Implementations/Litedb/ExtraFieldDataAccess.cs` | 03, 08 |
| `External/Implementations/Litedb/GasRecordDataAccess.cs` | 03, 08 |
| `External/Implementations/Litedb/InspectionRecordDataAccess.cs` | 03, 08 |
| `External/Implementations/Litedb/InspectionRecordTemplateDataAccess.cs` | 03, 08 |
| `External/Implementations/Litedb/NoteDataAccess.cs` | 03, 08 |
| `External/Implementations/Litedb/OdometerRecordDataAccess.cs` | 03, 08 |
| `External/Implementations/Litedb/PlanRecordDataAccess.cs` | 03, 08 |
| `External/Implementations/Litedb/PlanRecordTemplateDataAccess.cs` | 03, 08 |
| `External/Implementations/Litedb/ReminderRecordDataAccess.cs` | 03, 08 |
| `External/Implementations/Litedb/ServiceRecordDataAccess.cs` | 03, 08 |
| `External/Implementations/Litedb/SupplyRecordDataAccess.cs` | 03, 08 |
| `External/Implementations/Litedb/TaxRecordDataAccess.cs` | 03, 08 |
| `External/Implementations/Litedb/TokenRecordDataAccess.cs` | 03, 08 |
| `External/Implementations/Litedb/UpgradeRecordDataAccess.cs` | 03, 08 |
| `External/Implementations/Litedb/UserAccessDataAcces.cs` | 03, 08 |
| `External/Implementations/Litedb/UserConfigDataAccess.cs` | 03, 08 |
| `External/Implementations/Litedb/UserHouseholdDataAcces.cs` | 03, 08 |
| `External/Implementations/Litedb/UserRecordDataAccess.cs` | 03, 08 |
| `External/Implementations/Litedb/VehicleDataAccess.cs` | 03, 08 |
| `External/Implementations/Postgres/ApiKeyRecordDataAccess.cs` | 03, 08 |
| `External/Implementations/Postgres/CollisionRecordDataAccess.cs` | 03, 08 |
| `External/Implementations/Postgres/DBHealthCheck.cs` | 03, 08 |
| `External/Implementations/Postgres/EquipmentRecordDataAccess.cs` | 03, 08 |
| `External/Implementations/Postgres/ExtraFieldDataAccess.cs` | 03, 08 |
| `External/Implementations/Postgres/GasRecordDataAccess.cs` | 03, 08 |
| `External/Implementations/Postgres/InspectionRecordDataAccess.cs` | 03, 08 |
| `External/Implementations/Postgres/InspectionRecordTemplateDataAccess.cs` | 03, 08 |
| `External/Implementations/Postgres/NoteDataAccess.cs` | 03, 08 |
| `External/Implementations/Postgres/OdometerRecordDataAccess.cs` | 03, 08 |
| `External/Implementations/Postgres/PlanRecordDataAccess.cs` | 03, 08 |
| `External/Implementations/Postgres/PlanRecordTemplateDataAccess.cs` | 03, 08 |
| `External/Implementations/Postgres/ReminderRecordDataAccess.cs` | 03, 08 |
| `External/Implementations/Postgres/ServiceRecordDataAccess.cs` | 03, 08 |
| `External/Implementations/Postgres/SupplyRecordDataAccess.cs` | 03, 08 |
| `External/Implementations/Postgres/TaxRecordDataAccess.cs` | 03, 08 |
| `External/Implementations/Postgres/TokenRecordDataAccess.cs` | 03, 08 |
| `External/Implementations/Postgres/UpgradeRecordDataAccess.cs` | 03, 08 |
| `External/Implementations/Postgres/UserAccessDataAccess.cs` | 03, 08 |
| `External/Implementations/Postgres/UserConfigDataAccess.cs` | 03, 08 |
| `External/Implementations/Postgres/UserHouseholdDataAccess.cs` | 03, 08 |
| `External/Implementations/Postgres/UserRecordDataAccess.cs` | 03, 08 |
| `External/Implementations/Postgres/VehicleDataAccess.cs` | 03, 08 |
| `External/Interfaces/IApiKeyRecordDataAccess.cs` | 03, 08 |
| `External/Interfaces/ICollisionRecordDataAccess.cs` | 03, 08 |
| `External/Interfaces/IDBHealthCheck.cs` | 03, 08 |
| `External/Interfaces/IEquipmentRecordDataAccess.cs` | 03, 08 |
| `External/Interfaces/IExtraFieldDataAccess.cs` | 03, 08 |
| `External/Interfaces/IGasRecordDataAccess.cs` | 03, 08 |
| `External/Interfaces/IInspectionRecordDataAccess.cs` | 03, 08 |
| `External/Interfaces/IInspectionRecordTemplateDataAccess.cs` | 03, 08 |
| `External/Interfaces/INoteDataAccess.cs` | 03, 08 |
| `External/Interfaces/IOdometerRecordDataAccess.cs` | 03, 08 |
| `External/Interfaces/IPlanRecordDataAccess.cs` | 03, 08 |
| `External/Interfaces/IPlanRecordTemplateDataAccess.cs` | 03, 08 |
| `External/Interfaces/IReminderRecordDataAccess.cs` | 03, 08 |
| `External/Interfaces/IServiceRecordDataAccess.cs` | 03, 08 |
| `External/Interfaces/ISupplyRecordDataAccess.cs` | 03, 08 |
| `External/Interfaces/ITaxRecordDataAccess.cs` | 03, 08 |
| `External/Interfaces/ITokenRecordDataAccess.cs` | 03, 08 |
| `External/Interfaces/IUpgradeRecordDataAccess.cs` | 03, 08 |
| `External/Interfaces/IUserAccessDataAccess.cs` | 03, 08 |
| `External/Interfaces/IUserConfigDataAccess.cs` | 03, 08 |
| `External/Interfaces/IUserHouseholdDataAccess.cs` | 03, 08 |
| `External/Interfaces/IUserRecordDataAccess.cs` | 03, 08 |
| `External/Interfaces/IVehicleDataAccess.cs` | 03, 08 |
| `Filter/APIKeyFilter.cs` | 05, 06 |
| `Filter/CollaboratorFilter.cs` | 05, 06 |
| `Filter/QueryParamFilter.cs` | 05, 06 |
| `Filter/StrictCollaboratorFilter.cs` | 05, 06 |
| `Helper/ConfigHelper.cs` | 01, 04 |
| `Helper/EquipmentHelper.cs` | 04 |
| `Helper/FileHelper.cs` | 07, 08 |
| `Helper/GasHelper.cs` | 04 |
| `Helper/LiteDBHelper.cs` | 01 |
| `Helper/MailHelper.cs` | 02 (F-046/F-052) |
| `Helper/ReminderHelper.cs` | 04 |
| `Helper/ReportHelper.cs` | 04 |
| `Helper/StaticHelper.cs` | 01, 04 |
| `Helper/TranslationHelper.cs` | 02 (F-046/F-052) |
| `Logic/Event/AutomatedEventLogic.cs` | 04, 05 |
| `Logic/Event/EventHubLogic.cs` | 04, 05 |
| `Logic/Event/EventLogic.cs` | 04, 05 |
| `Logic/Event/NotificationLogic.cs` | 04, 05 |
| `Logic/LoginLogic.cs` | 06 |
| `Logic/OdometerLogic.cs` | 04 |
| `Logic/UserLogic.cs` | 06 |
| `Logic/VehicleLogic.cs` | 04 |
| `MapProfile/ImportMappers.cs` | 08 |
| `Middleware/Authen.cs` | 05, 06 |
| `Middleware/BufferBody.cs` | 05, 06 |

## Anhang D – Views und Client-Skripte

121 Razor-Views (`Views/**`) und 22 Skripte (`wwwroot/js/*.js`) wurden über die zugehörigen Controller-Aktionen erfasst; fachlich relevante Client-Logik (Tachokorrektur, Stückpreis, Einheitenumrechnung, Validierung) ist in `04-geschaeftsregeln.md` belegt (BR-007, BR-014, BR-022, BR-059). Status: [VERIFIZIERT] für die dort zitierten Stellen; eine zeilenweise Prüfung aller Views erfolgte nicht [ABGELEITET: Views enthalten Darstellung, Rechenlogik wurde per Suche nach Rechenoperationen/Faktoren lokalisiert].
