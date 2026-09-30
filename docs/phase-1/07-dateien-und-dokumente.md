# 07 – Dateien, Dokumente und Bilder (Ist-Zustand)

Quellenbezug: `hargata/lubelog` @ `dd69e59` (v1.7.3).

## 1. Modell

| Aussage | Quelle | Status |
|---|---|---|
| Es gibt **keine** eigenständige Dokument-Entität. Dateien existieren nur als eingebettete Liste `Files: List<UploadedFiles>` in Records (Service, Repair, Upgrade, Gas, Tax, Odometer, Note, Supply, Plan, Plan-Vorlage, Inspection, Inspection-Vorlage, Equipment) | `Models/Shared/UploadedFiles.cs`, Modelle in `03-domain-und-datenmodell.md` | [VERIFIZIERT] |
| `UploadedFiles` = `Name` (Anzeigename, vom Nutzer änderbar), `Location` (Pfad oder URL), `IsPending` (nur UI-Zustand) | `Models/Shared/UploadedFiles.cs` | [VERIFIZIERT] |
| Drei Arten von `Location`: (a) hochgeladene Datei `/documents/{GUID}{.ext}`, (b) externe URL (beliebiger String, der nicht mit `/documents` oder `/temp` beginnt), (c) Record-Link `::{ImportMode}:{id}` | `StaticHelper.GetAttachmentIsLink/GetAttachmentIsRecord` (`StaticHelper.cs:785-828`) | [VERIFIZIERT] |
| Fahrzeugbild: `Vehicle.ImageLocation` = `/images/{GUID}{.ext}` oder `/defaults/noimage.png`; Imagemap: `Vehicle.MapLocation` = JSON in `/documents/` | `Models/Vehicle/Vehicle.cs`, `VehicleController.SaveVehicle` (`VehicleController.cs:144-146`) | [VERIFIZIERT] |
| Metadaten wie Größe, MIME-Typ, Hash, Upload-Zeitpunkt, Uploader, Dokumenttyp oder Seitenanzahl werden **nicht** gespeichert | `UploadedFiles` (3 Felder) | [VERIFIZIERT] |
| Die gleiche Datei kann von mehreren Records referenziert werden (z. B. Supply-Anhänge werden beim Verbrauch per Referenz kopiert; Inspektion ↔ Service-Kopie) | `SupplyController.GetSuppliesAttachments` (`SupplyController.cs:29-38`), `InspectionController.cs:204-207` | [VERIFIZIERT] |

## 2. Speicherort und Namensschema

| Ordner (unter `data/`) | Inhalt | Namensschema | Quelle | Status |
|---|---|---|---|---|
| `temp/` | Uploads vor dem Speichern des Records, CSV-Exporte, Attachment-ZIPs, Backups, Migrationsexporte | `{GUID}{.ext}`; Backups `db_backup_yyyy-MM-dd-HH-mm-ss.zip` | `FilesController.UploadFile` (`FilesController.cs:105-118`), `FileHelper.MakeBackup` (`FileHelper.cs:360-443`) | [VERIFIZIERT] |
| `documents/` | Anhänge (nach Speichern verschoben), API-Uploads (direkt), Imagemaps | `{GUID}{.ext}` – Originalname nur im Record | `FileHelper.MoveFileFromTemp` (`FileHelper.cs:444-466`), `APIController.UploadDocument` | [VERIFIZIERT] |
| `images/` | Fahrzeugbilder | `{GUID}{.ext}` | `VehicleController.cs:145` | [VERIFIZIERT] |
| `translations/`, `themes/` | Übersetzungs-JSON, CSS-Themes (Root) | Originalname | `FilesController.cs:31-71` | [VERIFIZIERT] |
| `config/` | `userConfig.json`, `serverConfig.json` | fest | `StaticHelper.cs:19-20` | [VERIFIZIERT] |
| `data/cartracker.db`, `data/widgets.html` | LiteDB-Datenbank, Custom Widgets | fest | `StaticHelper.cs:18, 23` | [VERIFIZIERT] |

Die Dateiendung stammt ungeprüft aus dem vom Client gesendeten Dateinamen (`Path.GetExtension(file.FileName)`). [VERIFIZIERT: `FilesController.cs:111`]

## 3. Upload-Ablauf (Web-UI)

1. Datei wird per `POST /Files/HandleFileUpload` bzw. `HandleMultipleFileUpload` nach `data/temp/` geschrieben; Antwort `{name, location:"/temp/…", isPending:true}`. [VERIFIZIERT: `FilesController.cs:25-83`]
2. Beim Speichern eines Records verschiebt der Server jede `Location`, die mit `/temp/` beginnt, nach `/documents/` (`MoveFileFromTemp`). [VERIFIZIERT: z. B. `ServiceController.cs:35`]
3. Nicht gespeicherte Uploads bleiben in `temp/` bis zur Bereinigung (manuell, API `cleanup`, Automatik `CleanTempFile`). [VERIFIZIERT: `FileHelper.ClearTempFolder`, `NotificationLogic.cs:68-99`]
4. Optional verkleinert der Browser Fahrzeugbilder vor dem Upload (Hermite-Resize, Einstellung `LUBELOGGER_RESIZE_THUMBNAIL`); Anhänge werden nicht verändert. [VERIFIZIERT: `wwwroot/js/shared.js:282-307`]

Über die API werden Dateien per `POST /api/documents/upload` direkt in `documents/` geschrieben und anschließend im Record-JSON referenziert. [VERIFIZIERT: `APIController.cs:450-481`]

## 4. Prüfungen und Grenzen

| Aussage | Quelle | Status |
|---|---|---|
| **Keine Größenbegrenzung:** `MaxRequestBodySize`, `MultipartBodyLengthLimit`, `ValueLengthLimit` auf `int.MaxValue` | `Program.cs:136-145` | [VERIFIZIERT] |
| **Keine serverseitige Typprüfung:** `LUBELOGGER_ALLOWED_FILE_EXTENSIONS` (Default `.png,.jpg,.jpeg,.pdf,.xls,.xlsx,.docx`) wird nur als HTML-`accept`-Attribut im Browser verwendet | `ConfigHelper.GetAllowedFileUploadExtensions`, `Views/Vehicle/_FileUploader.cshtml:11`; kein Aufruf in `FilesController`/`APIController` | [VERIFIZIERT] (ausgeführt: `.html` akzeptiert) |
| Einzige serverseitige Endungsprüfung: Theme-Upload muss `.css` sein | `FilesController.cs:55-59` | [VERIFIZIERT] |
| Keine Inhaltsprüfung (Magic Bytes), kein Virenscan | – | [VERIFIZIERT] |

## 5. Auslieferung und Zugriffsschutz

| Aussage | Quelle | Status |
|---|---|---|
| Auslieferung über `GET /images/{f}`, `/documents/{f}`, `/translations/{f}`, `/temp/{f}` (`FilesController.GetStaticFile`) | `FilesController.cs:137-159` | [VERIFIZIERT] |
| Einziger Schutz: angemeldet (Cookie, Basic oder API-Key). **Keine Prüfung**, ob der Nutzer Zugriff auf das Fahrzeug/Record hat; Schutz beruht auf der Unerratbarkeit der GUID | ebd.; `StaticHelper.GetPathAllowAPIKeyAuth` | [VERIFIZIERT] (ausgeführt: fremde Datei abrufbar) |
| Bei deaktivierter Authentifizierung sind alle Dateien öffentlich | `Authen.cs:36-49` | [VERIFIZIERT] |
| Backups liegen mit **erratbarem Zeitstempel-Namen** in `temp/` und sind für jeden angemeldeten Nutzer herunterladbar; sie enthalten die komplette LiteDB (alle Benutzer, Passwort-Hashes, Tokens) und die Config-Dateien (Root-Hashes, SMTP-/OIDC-Geheimnisse) | `FileHelper.MakeBackup`, `FilesController.GetStaticFile` | [VERIFIZIERT] (ausgeführt als Nicht-Root-Nutzer) |
| Content-Type wird aus der Endung abgeleitet (`FileExtensionContentTypeProvider`), `inline` ohne `Content-Disposition: attachment` und ohne `X-Content-Type-Options` → hochgeladenes HTML/SVG wird im Kontext der Anwendung ausgeführt (Stored XSS) | `FilesController.cs:148-156` | [VERIFIZIERT] (ausgeführt: `Content-Type: text/html`) |
| `Cache-Control: no-store` für Dateien | `[ResponseCache(... NoStore = true)]` `FilesController.cs:141` | [VERIFIZIERT] |
| Pfadprüfung gegen Traversal in `GetFullFilePath`/`RenameFile` (Prefix-Vergleich mit Content-/Data-Pfad); **keine** Prüfung in `MoveFileFromTemp` (`/temp/../…` möglich) und nur schwache in `DeleteFile` (Vergleich ohne Normalisierung) | `FileHelper.cs:86-138, 444-490` | [VERIFIZIERT] (Code); Ausnutzbarkeit von `MoveFileFromTemp` [ABGELEITET] |

## 6. Löschen und Aufräumen

| Aussage | Quelle | Status |
|---|---|---|
| Löschen eines Records oder Fahrzeugs löscht **keine** Dateien | `Delete…WithChecks`, `VehicleLogic.DeleteVehicleRecords` | [VERIFIZIERT] |
| „Deep Clean“ löscht Dateien in `images/` bzw. `documents/`, die in keinem Fahrzeug/Record (inkl. Vorlagen, Werkstattlager) referenziert sind; nur wenn mindestens eine Referenz existiert | `FileHelper.ClearUnlinkedThumbnails/Documents` (`FileHelper.cs:528-563`), `VehicleLogic.GetVehicleDocuments` (`VehicleLogic.cs:591-621`), `APIController.CleanUp` | [VERIFIZIERT] |
| Per API hochgeladene, noch nicht verknüpfte Dokumente werden beim Deep Clean gelöscht | ebd. | [ABGELEITET] |
| Keine Audit-Spur für Löschungen oder Ersetzungen | – | [VERIFIZIERT] |

## 7. Export von Anhängen

`POST /Vehicle/GetVehicleAttachments` erzeugt ein ZIP in `temp/` mit Dateinamen `{laufendeNr}_{DataType}_{yyyy-MM-dd}_{Name}{.ext}`; externe Links werden in `link_attachments.csv` (`DataType, Date, Name, Location`) aufgeführt. [VERIFIZIERT: `FileHelper.MakeAttachmentsExport` (`FileHelper.cs:309-359`), `ReportController.cs:352-492`]

## 8. Bezug zu den Zielanforderungen (Auftrag 6.7, 6.11)

| Zielanforderung | Ist | Status |
|---|---|---|
| Original unverändert + SHA-256 | Original unverändert gespeichert, kein Hash | [VERIFIZIERT] |
| Upload-Zeit serverseitig | nicht gespeichert (nur Dateisystem-mtime) | [VERIFIZIERT] |
| EXIF separat, Standort nur opt-in | keine EXIF-Verarbeitung; EXIF bleibt in der Datei und wird unverändert ausgeliefert (inkl. GPS) | [VERIFIZIERT] (keine Verarbeitung im Code) / Auslieferung inkl. EXIF [ABGELEITET] |
| Vorschaubilder getrennt | keine serverseitigen Ableitungen; Vorschau im Browser aus Original | [VERIFIZIERT: `Views/Files/_AttachmentPreview.cshtml`, `FilesController.PreviewFile`] |
| Zugriff nur nach Berechtigungsprüfung | nur Authentifizierung | [VERIFIZIERT] |
| Dokumenttypen (Handbuch, Rechnung, HU-Bericht …) | nicht vorhanden; nur freier Name | [VERIFIZIERT] |
| Binärdaten nicht in der DB | erfüllt (Dateisystem) | [VERIFIZIERT] |

Für die Migration relevant: Zuordnung Datei ↔ Record ist nur über den String `Location` in den eingebetteten `Files`-Listen rekonstruierbar; der ursprüngliche Dateiname steht nur in `Name` (vom Nutzer änderbar, ggf. ohne Endung). [VERIFIZIERT]
