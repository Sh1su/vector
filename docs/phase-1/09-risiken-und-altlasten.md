# 09 – Risiken, technische Schulden und Auffälligkeiten

Quellenbezug: `hargata/lubelog` @ `dd69e59` (v1.7.3). Die Befunde dienen ausschließlich dazu, Fehler **nicht** in die Neuimplementierung zu übernehmen und Migrationsrisiken zu erkennen. Sie sind bewusst auf Ursache, Auswirkung und Gegenmaßnahme beschränkt.

**Hinweis zur Weitergabe:** Die Sicherheitsbefunde R-01 bis R-06 fallen nach `SECURITY.md` des Projekts unter „Records data being accessed and modified by … unauthorized users“. Eine vertrauliche Meldung an die Maintainer (Kontakt laut `SECURITY.md`) wird empfohlen; diese Entscheidung liegt beim Auftraggeber (Q-12).

Schweregrad: **hoch** = Datenintegrität/Vertraulichkeit mehrerer Nutzer betroffen; **mittel** = eingeschränkte Wirkung oder Voraussetzungen; **niedrig** = Qualitäts-/Wartbarkeitsthema.

## 1. Sicherheitsbefunde

| ID | Befund | Auswirkung | Quelle | Schwere | Status | Konsequenz für Neuimplementierung |
|---|---|---|---|---|---|---|
| R-01 | Beim Speichern bestehender Records über die UI-Endpunkte (`Save…ToVehicleId`, `SaveVehicle`, `SaveNoteToVehicleId` u. a.) wird nur das Recht am **übermittelten** Fahrzeug geprüft, nicht, zu welchem Fahrzeug der bestehende Datensatz gehört | Ein Nutzer mit Schreibrecht auf irgendein Fahrzeug kann Datensätze anderer Nutzer verändern/übernehmen | z. B. `Controllers/Vehicle/GasController.cs:36-42`, LiteDB `Upsert` (`Litedb/GasRecordDataAccess.cs:37-44`) | hoch | [VERIFIZIERT] (ausgeführt mit zwei Testnutzern) | Autorisierung immer am geladenen Bestandsobjekt; Fahrzeug-ID eines Datensatzes unveränderlich oder explizit geprüft |
| R-02 | `AdjustRecordsOdometer` prüft das Recht am Fahrzeug aus dem Parameter, nicht an den Datensätzen | wie R-01, auf Kilometerstände beschränkt | `VehicleController.cs:866-924` | hoch | [VERIFIZIERT] (Code) | wie R-01 |
| R-03 | Dateien unter `/documents`, `/images`, `/temp` sind für jeden angemeldeten Nutzer abrufbar; Schutz nur durch schwer erratbare Namen – mit Ausnahme der Backups, deren Namen einen Zeitstempel tragen | Offenlegung fremder Dokumente; Backups enthalten Datenbank und Konfiguration mit Hashes/Geheimnissen | `FilesController.cs:137-159`, `FileHelper.MakeBackup` | hoch | [VERIFIZIERT] (ausgeführt) | Zugriff nur über berechtigungsgeprüfte oder kurzlebig signierte URLs; Backups nie im Web-Pfad |
| R-04 | Keine serverseitige Prüfung von Dateityp/-größe; Auslieferung inline mit aus der Endung abgeleitetem Content-Type | aktive Inhalte in Uploads werden im Anwendungskontext ausgeliefert (Stored XSS), Speicher-Erschöpfung möglich | `FilesController.cs:105-118, 148-156`, `Program.cs:136-145` | hoch | [VERIFIZIERT] (ausgeführt) | Allowlist + Inhaltsprüfung serverseitig, `Content-Disposition: attachment`, `nosniff`, Größenlimits |
| R-05 | API-Key-Rechte werden nur an Endpunkten mit `APIKeyFilter` geprüft; mehrere schreibende Endpunkte haben keinen (u. a. Fahrzeug anlegen, Dokument-Upload, Gebühren-Fortschreibung per GET) | Nur-Lese-Schlüssel können Daten erzeugen | `APIController.cs:268-274, 450-452`, `API/TaxController.cs:92-94` | mittel | [VERIFIZIERT] (ausgeführt) | Scopes zentral und standardmäßig verweigernd (deny by default) |
| R-06 | Lesezugriffe ohne Rechteprüfung: `HomeController.ViewCalendarReminder` lädt Reminder per ID ohne Fahrzeugprüfung | Einsicht in fremde Reminder | `HomeController.cs:135-140` | mittel | [VERIFIZIERT] (Code) | einheitliche Autorisierungsschicht in den Application Services |
| R-07 | Passwörter und Root-Zugangsdaten als ungesalzenes SHA-256; Registrierungs-/Reset-Tokens mit 32 Bit Entropie, im Klartext gespeichert, ohne Ablauf | erleichterte Offline-Angriffe bei Datenabfluss; langlebige Tokens | `StaticHelper.GetHash` (`StaticHelper.cs:1023-1037`), `LoginLogic.NewToken` | hoch | [VERIFIZIERT] (Hash ausgeführt nachgerechnet) | Argon2id/bcrypt; Tokens ≥128 Bit, gehasht, mit Ablauf |
| R-08 | Sitzungscookie ohne `HttpOnly`/`Secure`/`SameSite`; kein CSRF-Schutz; Rollen im Cookie eingefroren, kein Widerruf | Sitzungsdiebstahl in Kombination mit R-04; Rechteentzug wirkt verzögert | `LoginController.cs:515`, `Authen.cs:103-150` | hoch | [VERIFIZIERT] (ausgeführt) | serverseitige Sessions/kurzlebige Tokens, sichere Cookie-Attribute, CSRF-Schutz für Cookie-Auth |
| R-09 | Authentifizierung standardmäßig deaktiviert; dann ist jeder Netzwerkzugriff Root | ungeschützte Installationen | `appsettings.json`, `Authen.cs:36-49` | hoch | [VERIFIZIERT] (ausgeführt) | sicherer Default: Ersteinrichtung erzwingt Admin-Konto |
| R-10 | OIDC: `state`-Prüfung und PKCE optional (Default aus), Signaturprüfung nur mit JWKS-URL, keine Issuer-/Audience-/Nonce-Prüfung, Zuordnung nur über E-Mail-Claim; Weiterleitungsziel aus Cookie ungeprüft | schwächere Absicherung des Login-Flows, Account-Zuordnung über E-Mail | `LoginController.cs:140-328`, `Models/OIDC/OpenIDConfig.cs` | mittel | [VERIFIZIERT] (Code) | Standardbibliothek (OIDC-Client mit Discovery), Zuordnung über `iss`+`sub` |
| R-11 | API-Key auch als Query-Parameter zulässig | Schlüssel landen in Logs/Verläufen | `Authen.cs:57-61` | niedrig | [VERIFIZIERT] | nur Header |
| R-12 | Pfadbehandlung in `MoveFileFromTemp` ohne Normalisierung/Prüfung; `DeleteFile` prüft ohne Normalisierung | potenziell Dateioperationen außerhalb des vorgesehenen Ordners | `FileHelper.cs:444-490` | mittel | [VERIFIZIERT] (Code), Ausnutzbarkeit [ABGELEITET] | Objekt-Storage-Abstraktion mit Schlüsseln statt Pfaden |
| R-13 | HTML-E-Mails setzen Nutzereingaben unescaped ein; SMTP mit `SecureSocketOptions.Auto` | HTML-Injection in Mails; ggf. unverschlüsselter Versand | `Helper/MailHelper.cs:174-186, 226, 266` | niedrig | [VERIFIZIERT] (Code) | Template-Engine mit Escaping, TLS erzwingen |
| R-14 | Admins können sich gegenseitig entrechten/löschen; Haushalts- und Kollaborator-Zuordnung ohne Zustimmung des Betroffenen | Aussperren, ungewollte Freigaben | `AdminController.cs:73-88`, `UserLogic.cs:63-84, 235-272` | niedrig | [VERIFIZIERT] | Einladungen mit Annahme; Schutz des letzten Admins |

## 2. Datenintegrität und fachliche Fehler

| ID | Befund | Auswirkung | Quelle | Status |
|---|---|---|---|---|
| D-01 | Keine Plausibilitätsprüfung von Kilometerständen; „aktueller Stand“ = Maximum über Typen | ein Tippfehler verfälscht dauerhaft Reminder-Fälligkeit, Kosten/km und Verbrauch | BR-015, BR-021 | [VERIFIZIERT] (ausgeführt) |
| D-02 | Reminder-Schwellen „lecken“ auf nachfolgende Reminder | falsche Dringlichkeit, falsche Benachrichtigungen | BR-029 | [VERIFIZIERT] (ausgeführt) |
| D-03 | Datumswerte in LiteDB verschieben sich bei Zeitzonenwechsel des Servers | falsche Daten nach Umzug/Containerwechsel | M-6 in 03 | [VERIFIZIERT] (ausgeführt) |
| D-04 | Einheiten nicht gespeichert; Nutzer mit unterschiedlichen Einstellungen interpretieren Daten unterschiedlich; Einstellungswechsel konvertiert nichts | falsche Werte ohne Hinweis | BR-059 | [VERIFIZIERT] |
| D-05 | Aktivieren der Authentifizierung setzt Server-Defaults (u. a. `UseMPG`) zurück | Einheitenanzeige springt | 01 §9 | [VERIFIZIERT] (ausgeführt) |
| D-06 | CSV-Export für UK- und EV-Kraftstoffdaten exportiert berechnete statt gespeicherte Mengen | Datenverfälschung beim Re-Import | 08 §4 | [VERIFIZIERT] (Code) |
| D-07 | Keine Transaktionen; Mehrschritt-Operationen (Move, Bulk, Plan→Done, Supply-Abbuchung, PG-Insert) können halb ausgeführt enden | Inkonsistenzen | 01 §6, BR-042/044/057 | [VERIFIZIERT] |
| D-08 | Bei PostgreSQL wird die Spalte `vehicleId` bei Updates nicht aktualisiert | Abweichung Spalte ↔ JSON | `Postgres/*DataAccess.cs` | [VERIFIZIERT] |
| D-09 | Reminder-Pushback ohne gespeicherte Verknüpfung, auch bei fehlgeschlagenem Speichern; mehrfache Auslösung beim Bearbeiten | Reminder werden zu weit fortgeschrieben, nicht nachvollziehbar | BR-033 | [VERIFIZIERT] |
| D-10 | Plan „Done“ ohne Idempotenz; Inspektion erzeugt Service-Kopie | Duplikate, Kosten potenziell doppelt in Auswertungen, falls beide gezählt würden | BR-044, BR-047 | [VERIFIZIERT] (Code) / Duplikat bei Mehrfach-Done [ABGELEITET] |
| D-11 | Recurring Tax mit Intervall 0 (Client-Validierung umgangen) → Endlosschleife | Datenflut | BR-035 | [ABGELEITET] |
| D-12 | Unterschiedliche Distanzdefinitionen (Σ Odometer vs. max−min), gewichteter vs. ungewichteter Durchschnitt | widersprüchliche Kennzahlen im selben Dashboard | BR-012, BR-038, BR-060 | [VERIFIZIERT] (ausgeführt) |
| D-13 | Tachokorrektur mit drei Rundungsvarianten, nur bei UI-Neuanlage automatisch | uneinheitliche Stände | BR-022 | [VERIFIZIERT] |
| D-14 | Extra-Field-Werte gehen verloren, wenn die Vorlage geleert wird | stiller Datenverlust | BR-049 | [VERIFIZIERT] (Code) |
| D-15 | Löschungen hart, ohne Audit; Dateien bleiben verwaist | keine Nachvollziehbarkeit | 03 §6, 07 §6 | [VERIFIZIERT] |
| D-16 | Backup enthält bei PostgreSQL keine Fachdaten | trügerische Sicherheit | 08 §5 | [VERIFIZIERT] (Code) |
| D-17 | Benachrichtigungs-Deduplizierung nur im Speicher | Mehrfachversand nach Neustart | BR-034 | [VERIFIZIERT] |
| D-18 | Inspektions-Action-Items tragen falschen Herkunftstext („Fuel Record“) | irreführende Notizen | BR-047 | [VERIFIZIERT] |
| D-19 | API-Update der Kraftstoffart setzt `IsDiesel`/`IsElectric` nie zurück | falscher Fahrzeugtyp | `APIController.cs:425-433` | [VERIFIZIERT] |

## 3. Technische Schulden und Architektur

| ID | Befund | Quelle | Status |
|---|---|---|---|
| T-01 | Keine automatisierten Tests | Repository | [VERIFIZIERT] |
| T-02 | Fachlogik verteilt über Controller, Helper und Browser-JavaScript; Controller mit bis zu 1 848 Zeilen | `VehicleController.cs`, `wwwroot/js/*` | [VERIFIZIERT] |
| T-03 | Zwei Persistenz-Backends mit duplizierter, nahezu identischer Implementierung (44 Klassen); PG ohne Schema/Migrationen, Daten als JSONB | `External/Implementations/*` | [VERIFIZIERT] |
| T-04 | Input-Modelle werden persistiert; Datumswerte teils als Strings; Zahlen (Tachokorrektur) als Strings | 03 M-8, M-10 | [VERIFIZIERT] |
| T-05 | Kulturabhängiges Parsen/Formatieren in API, CSV und Persistenz | 05 §3, 08 | [VERIFIZIERT] |
| T-06 | Schreibende GET-Endpunkte (`taxrecords/check`, `PinNotes`, Auto-Refresh von Remindern beim Laden) | `API/TaxController.cs:92`, `NoteController.cs:83`, `ReminderController.cs:17-47` | [VERIFIZIERT] |
| T-07 | `async void` in Filter und Event-Publishing | `QueryParamFilter.cs:14`, `EventLogic.cs:26` | [VERIFIZIERT] |
| T-08 | Singleton-Datenzugriff auf eine LiteDB-Instanz; Konfigurationsdateien werden von mehreren Stellen direkt geschrieben | `Program.cs:44`, `ConfigHelper`, `LoginLogic` | [VERIFIZIERT] |
| T-09 | Uneinheitliche Fehlerbehandlung (200 mit `success:false` vs. 4xx/5xx; Exception-Texte an Clients) | 05 §3 | [VERIFIZIERT] |
| T-10 | Keine Paginierung; Berechnungen laden stets alle Records eines Fahrzeugs (mehrfach je Request) | z. B. `VehicleLogic.GetMaxMileage` | [VERIFIZIERT] |
| T-11 | Laufzeitabhängigkeiten zu GitHub (Übersetzungen, Sponsoren, Versionscheck) | `StaticHelper.cs:29-32` | [VERIFIZIERT] |
| T-12 | Enum-Namen in Meilen (`FiveThousandMiles`) obwohl einheitenlos verwendet | `Enum/ReminderMileageInterval.cs` | [VERIFIZIERT] |

## 4. Top-Risiken für das Projekt (Neuimplementierung)

1. **Migration der Einheiten und Datumswerte** (D-03, D-04, MG-1–MG-3): ohne Quell-Zeitzone, -Kultur und Einheitenwahl je Fahrzeug ist ein korrekter Import nicht möglich. [ABGELEITET]
2. **Übernahme fehlerhafter Regeln** (D-01, D-02, D-12): Regressionstests müssen zwischen „Ist-Verhalten dokumentieren“ und „Soll-Verhalten“ unterscheiden; Abweichungen per ADR. [ABGELEITET]
3. **Rechtemodell-Mapping** (Kollaborator ohne Eigentümer, Haushalte): automatische Abbildung auf Eigentümer/Bearbeiter/Leser ist mehrdeutig. [ABGELEITET]
4. **Scope-Umfang**: 61 Ist-Features, davon mehrere Nischenfunktionen (Imagemap, Sticker, Kiosk, Widgets, Themes); ohne KEEP/DEPRECATE-Entscheidung droht Aufwandsexplosion. [ABGELEITET]
