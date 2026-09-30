# 01 – Repository-Überblick LubeLogger

> **Quellenbezug für alle Phase-1-Dokumente:** `hargata/lubelog`, Commit `dd69e59b276e79b96bb459d8b1eda1501316086d` („Merge pull request #1461 from hargata/Hargata/173“), identisch mit Release-Tag **v1.7.3**, Datum **2026-09-12 08:52:01 −0600**. Pfadangaben beziehen sich auf das Wurzelverzeichnis dieses Repositorys; Zeilennummern auf diesen Commit.
>
> **Statusmarkierung:** [VERIFIZIERT] = im Code gelesen oder durch Ausführung bestätigt · [ABGELEITET] = Schlussfolgerung mit Begründung · [UNKLAR] = offen, siehe `10-offene-fragen.md`.
>
> **Ausführung:** Für Verifikationen lief das offizielle Image `hargata/lubelogger:v1.7.3` (Docker Hub) lokal mit LiteDB-Backend. Wo eine Aussage durch Ausführung bestätigt wurde, steht „(ausgeführt)“.

## 1. Stand und Versionierung

| Aussage | Quelle | Status |
|---|---|---|
| Analysierter Commit `dd69e59…` ist Tag `v1.7.3` | `git rev-list -n1 v1.7.3` | [VERIFIZIERT] |
| Die Anwendung meldet sich als Version `1.7.3` | `Helper/StaticHelper.cs:17` (`VersionNumber`), `/health` (ausgeführt) | [VERIFIZIERT] |
| Historie: 2 106 Commits, erster Commit 2023-12-31 | `git log` | [VERIFIZIERT] |
| Es gibt keine automatisierten Tests (kein Testprojekt, keine Testframeworks) | Suche nach `*test*`, xUnit/NUnit/MSTest ohne Treffer | [VERIFIZIERT] |

## 2. Lizenz und Bedeutung für die Neuimplementierung

| Aussage | Quelle | Status |
|---|---|---|
| Lizenz ist **MIT**, Copyright „(c) 2024 Hargata Softworks“ | `LICENSE` | [VERIFIZIERT] |
| MIT erlaubt Nutzung, Veränderung und Weitergabe von Code, Datenstrukturen und Texten, auch kommerziell, sofern bei Übernahme „substantieller Teile“ Copyright-Hinweis und Lizenztext beigefügt werden | `LICENSE` | [VERIFIZIERT] (Lizenztext), Auslegung [ABGELEITET] |
| Die Lizenz gewährt **keine Markenrechte**. Name „LubeLogger“, Logos (`wwwroot/defaults/lubelogger_*.png`, `hargata_logo*.png`) und Website-Texte werden nicht übernommen; das neue Produkt trägt einen eigenen Namen | Auftrag Abschnitt 2; MIT regelt nur Urheberrecht | [ABGELEITET] |
| Wird keine Zeile Code übernommen und nur Fachlogik nachgebaut, entsteht keine Pflicht zur Attribution. Übernahme z. B. der CSV-Header-Aliase oder der Übersetzungsschlüssel (`wwwroot/defaults/en_US.json`) sollte vorsorglich mit MIT-Hinweis erfolgen | Auslegung | [ABGELEITET] – Empfehlung: `THIRD_PARTY_NOTICES` ab Phase 2 führen |
| Mitgelieferte Drittbibliotheken (Bootstrap, jQuery, Chart.js, SignalR-Client, Masonry, QRCode-Generator, Drawdown, bootstrap-datepicker, bootstrap-tagsinput) haben eigene Lizenzen und sind für die Neuimplementierung irrelevant | `wwwroot/lib/*`, `README.md` | [VERIFIZIERT] (Vorhandensein) |

## 3. Tech-Stack

| Bereich | Technologie | Quelle | Status |
|---|---|---|---|
| Laufzeit | .NET 10 (`net10.0`), ASP.NET Core MVC mit Razor-Views (Server-Rendering) | `CarCareTracker.csproj`, `Program.cs:41` | [VERIFIZIERT] |
| NuGet-Abhängigkeiten | CsvHelper 33.1.0, LiteDB 5.0.17, MailKit 4.17.0, Microsoft.IdentityModel.JsonWebTokens 8.22.0, Npgsql 9.0.5 | `CarCareTracker.csproj` | [VERIFIZIERT] |
| Frontend | Razor-Views (121 `.cshtml`, ~12 300 Zeilen), jQuery-basiertes JS (~9 300 Zeilen in `wwwroot/js`), Bootstrap, Chart.js, SweetAlert2 | `Views/`, `wwwroot/js/`, `wwwroot/lib/` | [VERIFIZIERT] |
| Echtzeit | SignalR-Hub unter `/api/ws` | `Program.cs:171`, `Logic/Event/EventHubLogic.cs` | [VERIFIZIERT] |
| Hintergrundjobs | `BackgroundService` mit Minuten-Timer, nur wenn `LUBELOGGER_AUTO_EVENTS=true` | `Program.cs:117-120`, `Logic/Event/AutomatedEventLogic.cs` | [VERIFIZIERT] |
| Authentifizierung | Eigenes `AuthenticationHandler` („AuthN“): Cookie (DataProtection-verschlüsselt), HTTP Basic, API-Key; optional OIDC (Authorization Code) | `Middleware/Authen.cs`, `Controllers/LoginController.cs` | [VERIFIZIERT] |
| Umfang C# | 27 222 Zeilen in 241 Dateien | `wc -l` | [VERIFIZIERT] |

## 4. Projektstruktur

| Verzeichnis | Inhalt | Status |
|---|---|---|
| `Controllers/` | MVC-Controller: `HomeController`, `VehicleController` (partial, verteilt auf `Controllers/Vehicle/*.cs`), `APIController` (partial, verteilt auf `Controllers/API/*.cs`), `LoginController`, `AdminController`, `FilesController`, `KioskController`, `MigrationController`, `SharedDataController`, `ErrorController` | [VERIFIZIERT] |
| `Models/` | Persistenzmodelle, Input-/View-Modelle, Export-Modelle, Konfigurationsmodelle | [VERIFIZIERT] |
| `Enum/` | Fachliche Aufzählungen (`ImportMode`, `ReminderMetric`, `ReminderUrgency`, `PlanProgress`, …) | [VERIFIZIERT] |
| `External/Interfaces`, `External/Implementations/Litedb`, `External/Implementations/Postgres` | Data-Access-Schicht: je Entität ein Interface und zwei Implementierungen | [VERIFIZIERT] |
| `Logic/` | `VehicleLogic`, `OdometerLogic`, `UserLogic`, `LoginLogic`, `Event/*` (Webhook, SignalR, Notifications, Scheduler) | [VERIFIZIERT] |
| `Helper/` | `GasHelper` (Verbrauch), `ReminderHelper` (Fälligkeit), `ReportHelper`, `ConfigHelper`, `FileHelper`, `MailHelper`, `TranslationHelper`, `StaticHelper` | [VERIFIZIERT] |
| `Filter/` | Autorisierungsfilter `CollaboratorFilter`, `StrictCollaboratorFilter`, `APIKeyFilter`, `QueryParamFilter` | [VERIFIZIERT] |
| `Middleware/` | `Authen` (Authentifizierung), `BufferBody` | [VERIFIZIERT] |
| `MapProfile/` | CsvHelper-Mapping für CSV-Import (Header-Aliase) | [VERIFIZIERT] |
| `wwwroot/defaults/` | `api.json` (API-Dokumentation), `en_US.json` (Übersetzungsschlüssel), `reminderemailtemplate.txt`, `demo_default.zip`, Logos | [VERIFIZIERT] |
| `docs/` | Statische Website/Screenshots, Konfigurator (`docs/configure/configurator.html`) – keine technische Doku | [VERIFIZIERT] |

**Architekturmerkmal:** Geschäftslogik liegt verteilt in Controllern (z. B. Reminder-Pushback, Supply-Abbuchung, Plan→Record-Konvertierung), Helpern und teils im Browser-JavaScript (Tachokorrektur bei Neuanlage, Stückpreis→Gesamtkosten, Einheitenumrechnung der Anzeige). [VERIFIZIERT] – Belege in `04-geschaeftsregeln.md` (BR-014, BR-022, BR-059).

## 5. Konfiguration

| Aussage | Quelle | Status |
|---|---|---|
| Konfigurationsquellen: `appsettings.json`, `data/config/userConfig.json` (Root-/Server-Benutzereinstellungen), `data/config/serverConfig.json` (Servereinstellungen), Umgebungsvariablen, optional Key-per-File-Secrets aus `LUBELOGGER_SECRETS_PATH`; `reloadOnChange: true` | `Program.cs:16-21`, `StaticHelper.cs:19-20` | [VERIFIZIERT] |
| Wichtige Schlüssel: `POSTGRES_CONNECTION`, `EnableAuth`, `UserNameHash`/`UserPasswordHash` (Root), `LUBELOGGER_ALLOWED_FILE_EXTENSIONS`, `LUBELOGGER_WEBHOOK`, `LUBELOGGER_DOMAIN`, `LUBELOGGER_AUTO_EVENTS`, `LUBELOGGER_WEB_SOCKET`, `LUBELOGGER_INVARIANT_API`, `LUBELOGGER_COOKIE_LIFESPAN`, `LUBELOGGER_LOCALE_OVERRIDE`, `LUBELOGGER_LOCALE_DT_OVERRIDE`, `LUBELOGGER_OPEN_REGISTRATION`, `LUBELOGGER_CUSTOM_WIDGETS`, `MailConfig`, `OpenIDConfig`, `ReminderUrgencyConfig`, `NotificationConfig`, `Kestrel` | `Models/Settings/ServerConfig.cs`, `Helper/ConfigHelper.cs` | [VERIFIZIERT] |
| Die Servereinstellungen sind über die Web-Oberfläche `/setup` durch den Root-User schreibbar (inkl. Postgres-Connection-String, SMTP-Passwort, OIDC-Client-Secret, Kestrel-Zertifikat) | `HomeController.cs:652-703`, `ConfigHelper.SaveServerConfig` | [VERIFIZIERT] |
| Kultur/Locale bestimmt Datums-, Zahlen- und Währungsformate serverweit; im Docker-Image ohne Override ist die Kultur **invariant** (Währung `¤`, Datum `MM/dd/yyyy`) | `Program.cs:23-33`, `/api/info` (ausgeführt) | [VERIFIZIERT] |

## 6. Persistenz-Backends

Es existieren **zwei** vollständige Persistenz-Backends mit identischen Interfaces.

| Aussage | Quelle | Status |
|---|---|---|
| Auswahl: Ist `POSTGRES_CONNECTION` gesetzt, werden alle `PG*DataAccess`-Implementierungen registriert, sonst die LiteDB-Implementierungen | `Program.cs:47-97` | [VERIFIZIERT] |
| LiteDB wird **immer** zusätzlich registriert (für Backup/Restore und Migration) | `Program.cs:43-44` | [VERIFIZIERT] |
| **LiteDB:** eingebettete Dokumentdatenbank, Datei `data/cartracker.db`; eine Collection je Entität; Int32-Autoincrement-IDs; Composite-IDs als eingebettete Objekte bei `useraccessrecords` und `userhouseholdrecords`; `extrafields` nutzt `ImportMode` als ID | `StaticHelper.cs:18`, `External/Implementations/Litedb/*.cs`, `Models/Shared/RecordExtraField.cs` (`[BsonId(false)]`) | [VERIFIZIERT] |
| **PostgreSQL:** Schema `app`, 22 Tabellen. Fast alle Fachtabellen haben nur `id INT IDENTITY`, `vehicleId INT` und `data JSONB` (vollständiges Objekt als JSON); relational ausmodelliert sind nur `userrecords`, `tokenrecords`, `useraccessrecords` und teilweise `apikeyrecords`, `userhouseholdrecords` | `External/Implementations/Postgres/*.cs` (jeweils `initCMD`), `MigrationController.cs:34-68` | [VERIFIZIERT] |
| Tabellen werden beim Start per `CREATE TABLE IF NOT EXISTS` angelegt; es gibt **keine Migrationen, keine Fremdschlüssel, keine Indizes** außer Primärschlüsseln | ebd. | [VERIFIZIERT] |
| Bei PostgreSQL ist `vehicleId` doppelt gespeichert (Spalte und JSON); ein Update schreibt nur das JSON, nicht die Spalte | `PGGasRecordDataAccess.SaveGasRecordToVehicle` (`Postgres/GasRecordDataAccess.cs:123-133`) und analoge Klassen | [VERIFIZIERT] |
| Keine Transaktionen: Inserts erfolgen in zwei Schritten (INSERT mit `{}` + UPDATE mit JSON), Mehrfachoperationen sind nicht atomar | `Postgres/GasRecordDataAccess.cs:100-121` | [VERIFIZIERT] |
| Details zur Struktur und Migration: `03-domain-und-datenmodell.md`, `08-import-export-und-migration.md` | – | – |

## 7. Dateiablage (Kurzüberblick)

| Aussage | Quelle | Status |
|---|---|---|
| Alle Nutzdateien liegen im Dateisystem unter `data/`: `images/` (Fahrzeugbilder), `documents/` (Anhänge), `temp/` (Uploads vor Zuordnung, Exporte, Backups), `translations/`, `themes/`, `config/`, `widgets.html` | `StaticHelper.InitMessage` (`StaticHelper.cs:341-376`) | [VERIFIZIERT] |
| Details: `07-dateien-und-dokumente.md` | – | – |

## 8. Deployment

| Aussage | Quelle | Status |
|---|---|---|
| Multi-Stage-Dockerfile (SDK 10 → aspnet 10), Port 8080, Start `./CarCareTracker` | `Dockerfile` | [VERIFIZIERT] |
| Images: `ghcr.io/hargata/lubelogger`, `hargata/lubelogger` (amd64, arm64), gebaut per GitHub Actions bei Push auf `main` (Tag `edge`) und bei Releases | `.github/workflows/build-and-push-image.yml` | [VERIFIZIERT] |
| Compose-Varianten: nur App (LiteDB), App + PostgreSQL 18, App hinter Traefik | `docker-compose.yml`, `docker-compose.postgresql.yml`, `docker-compose.traefik.yml` | [VERIFIZIERT] |
| Persistente Volumes: `/App/data` und `/root/.aspnet/DataProtection-Keys` (Schlüssel für Login-Cookies) | Compose-Dateien | [VERIFIZIERT] |
| Beispiel-Compose enthält Klartext-Standardpasswort `lubepass` für PostgreSQL | `docker-compose.postgresql.yml` | [VERIFIZIERT] |
| Laut README gibt es zusätzlich ein Windows-Standalone-Executable und ein Community-Helm-Chart | `README.md` | [VERIFIZIERT] (Doku-Aussage), Existenz im Code nicht prüfbar → [UNKLAR], irrelevant für Phase 2 |
| Beim Start werden Legacy-Dateien aus `wwwroot/{images,documents,translations,temp}` nach `data/` verschoben (Migration aus älteren Versionen) | `StaticHelper.CheckMigration` (`StaticHelper.cs:378-433`) | [VERIFIZIERT] |
| Externe Netzwerkzugriffe zur Laufzeit: GitHub (Release-Check, Übersetzungen, Sponsoren), konfigurierte Webhooks/Notification-Dienste, SMTP, OIDC-Provider | `StaticHelper.cs:29-32`, `EventLogic`, `NotificationLogic`, `MailHelper`, `LoginController.RemoteAuth` | [VERIFIZIERT] |

## 9. Widersprüche Doku ↔ Code (Auswahl)

| Doku-Aussage | Code-Befund | Status |
|---|---|---|
| `CONTRIBUTING.md`/`SECURITY.md`: „Records data being accessed and modified by … unauthorized users“ gilt als Sicherheitslücke | Solcher Zugriff ist über `Save*ToVehicleId` möglich (siehe `09-risiken-und-altlasten.md`, R-01) | [VERIFIZIERT] (ausgeführt) |
| `api.json` kategorisiert `/api/vehicle/taxrecords/check` als „Admin“ | Endpunkt hat keine Rollenprüfung; jeder angemeldete Nutzer kann ihn für seine Fahrzeuge auslösen (GET mit Schreibwirkung) | [VERIFIZIERT] (`Controllers/API/TaxController.cs:92-127`) |
| `appsettings.json` setzt `UseMPG: true` als Default | Beim erstmaligen Aktivieren der Authentifizierung wird `userConfig.json` mit Klassen-Defaults (`UseMPG=false`, `EnableCsvImports=false` …) neu geschrieben | [VERIFIZIERT] (ausgeführt; `LoginLogic.CreateRootUserCredentials`, `LoginLogic.cs:521-530`) |
