# 05 – Bestehende API (Ist-Zustand)

Quellenbezug: `hargata/lubelog` @ `dd69e59` (v1.7.3).

## 1. Schnittstellenarten

| # | Schnittstelle | Beschreibung | Quelle | Status |
|---|---|---|---|---|
| S-1 | **Öffentliche API** `/api/...` | JSON-Endpunkte in `APIController` (partial). Dokumentiert in `wwwroot/defaults/api.json`, im UI unter `/API` mit Testformular. | `Controllers/APIController.cs`, `Controllers/API/*.cs`, `Views/API/Index.cshtml` | [VERIFIZIERT] |
| S-2 | **Interne UI-Endpunkte** `/{Controller}/{Action}` | Konventionelles MVC-Routing; liefern überwiegend HTML-Partials, teils JSON (`OperationResponse`). Nicht versioniert, nicht dokumentiert, eng an die Razor-UI gekoppelt. Vollständige Liste: Anhang A in `02-feature-inventar.md`. | `Program.cs:166-168` | [VERIFIZIERT] |
| S-3 | **WebSocket** `/api/ws` (SignalR) | Push von Änderungsereignissen an UI/Kiosk. | `Program.cs:171`, `Logic/Event/EventHubLogic.cs` | [VERIFIZIERT] |
| S-4 | **Ausgehende Webhooks** | Ereignisse als HTTP-POST an eine global konfigurierte URL. | `Logic/Event/EventLogic.cs` | [VERIFIZIERT] |
| S-5 | **Ausgehende Benachrichtigungsdienste** | Frei konfigurierbare HTTP-POST-Templates (z. B. ntfy/Gotify) für Reminder-Zustandswechsel. | `NotificationLogic.SendNotificationToExternalServices` | [VERIFIZIERT] |

Eine OpenAPI-Spezifikation existiert nicht; `api.json` ist ein proprietäres Dokumentationsformat (Kategorie, Pfad, Methode, Query-Parameter, Beispiel-Bodies). [VERIFIZIERT]

## 2. Authentifizierung der API

| Aussage | Quelle | Status |
|---|---|---|
| Ist Authentifizierung deaktiviert (`EnableAuth=false`, Default), wird **jede** Anfrage als Root-User („admin“, Id −1) behandelt – die API ist dann ohne Zugangsdaten voll nutzbar | `Middleware/Authen.cs:36-49` | [VERIFIZIERT] (ausgeführt) |
| Verfahren bei aktivierter Authentifizierung, Priorität in dieser Reihenfolge: 1. HTTP Basic (`Authorization`), 2. Cookie `ACCESS_TOKEN`, 3. API-Key (`x-api-key`-Header oder Query-Parameter `apiKey`) | `Authen.cs:53-184` | [VERIFIZIERT] |
| HTTP Basic prüft Benutzername/Passwort bei jeder Anfrage (Root über Config-Hash, sonst DB); OIDC-Nutzer (leeres Passwort) können sich nicht per Basic anmelden | `Authen.cs:66-102`, `LoginLogic.ValidateUserCredentials` | [VERIFIZIERT] |
| API-Keys werden nur für Pfade `/api`, `/kiosk`, `/images`, `/documents`, `/temp` akzeptiert | `StaticHelper.GetPathAllowAPIKeyAuth` (`StaticHelper.cs:1118-1121`), `Authen.cs:156` | [VERIFIZIERT] |
| API-Key-Berechtigungen (`View`/`Edit`/`Delete`) werden nur an Endpunkten mit `APIKeyFilter` geprüft; Endpunkte ohne Filter (z. B. `POST /api/vehicles/add`, `POST /api/documents/upload`, `GET /api/vehicle/taxrecords/check`) sind mit einem reinen View-Key nutzbar | `Filter/APIKeyFilter.cs`, Attribute in `Controllers/API*` | [VERIFIZIERT] (ausgeführt: View-Key legt Fahrzeug an) |
| Ohne Authentifizierung antwortet die API mit HTTP 401 + `WWW-Authenticate: Basic`; bei fehlender Berechtigung 403 | `Authen.cs:187-217` | [VERIFIZIERT] |
| Fahrzeugbezogene Autorisierung über `CollaboratorFilter` (liest `vehicleId` aus Route/Query/Form; bei JSON-Body über `QueryParamFilter` aus dem Body) bzw. über explizite Prüfung des bestehenden Datensatzes bei Update/Delete | `Filter/CollaboratorFilter.cs`, `Filter/QueryParamFilter.cs`, z. B. `API/GasController.cs:262-270` | [VERIFIZIERT] |
| `QueryParamFilter.OnActionExecuting` ist `async void`; das Einlesen des Bodies kann zeitlich nach Beginn der Aktion abgeschlossen werden (Race) | `Filter/QueryParamFilter.cs:14-55` | [VERIFIZIERT] (Code), Auswirkung [ABGELEITET] |

## 3. Formate und Konventionen

| Aussage | Quelle | Status |
|---|---|---|
| **Keine Versionierung** im Pfad oder Header; Serverversion über `GET /api/version` | Routen, `APIController.cs:171-198` | [VERIFIZIERT] |
| **Keine Paginierung**; Listen werden vollständig geliefert | alle `GET`-Listen | [VERIFIZIERT] |
| Filter-Query-Parameter: `id`, `startDate`, `endDate` (inklusiv, `DateTime.TryParse`, kulturabhängig), `tags` (leerzeichengetrennt, **ODER**-Semantik), bei Kraftstoff `useMPG`, `useUKMPG`, bei Remindern `urgencies` | `Models/API/MethodParameter.cs`, z. B. `API/OdometerController.cs:100-116` | [VERIFIZIERT] |
| Antwort-Hülle für Schreiboperationen: `{ "success": bool, "message": string, "additionalData"?: object }`; neue IDs in `additionalData.recordId` bzw. `vehicleId` | `Models/Shared/OperationResponse.cs`, z. B. `API/GasController.cs:198` | [VERIFIZIERT] (ausgeführt) |
| HTTP-Statuscodes uneinheitlich: Validierungsfehler teils 400, teils 200 mit `success:false` (z. B. Reminder-Metrik-Validierung), Serverfehler 500 mit Exception-Text in `message` | `API/ReminderController.cs:122-144`, `API/GasController.cs:200-204` | [VERIFIZIERT] |
| JSON-Feldnamen camelCase (ASP.NET-Default) | Ausgabe (ausgeführt) | [VERIFIZIERT] |
| **Standardmodus:** Export-Modelle serialisieren alle Werte als **Strings** in Serverkultur (z. B. Datum `ToShortDateString()`, Dezimaltrennzeichen der Kultur) | `Models/Shared/ImportModel.cs` (Konverter `FromDateOptional` etc.), `Models/API/TypeConverter.cs` | [VERIFIZIERT] |
| **Invariant-Modus** (Header `culture-invariant` oder `LUBELOGGER_INVARIANT_API=true`): Zahlen/Booleans als JSON-Typen, Datum `yyyy-MM-dd` | `StaticHelper.GetInvariantOption`, `TypeConverter.cs` | [VERIFIZIERT] (ausgeführt) |
| Eingaben akzeptieren Zahlen als String oder JSON-Zahl, Datum als String (kulturabhängiges `DateTime.Parse`) oder Unix-Sekunden | `TypeConverter.cs` (`FromDateOptional.Read`), `API/GasController.cs:171` | [VERIFIZIERT] |
| Bodies werden sowohl als `application/json` (Aktionen `…Json`) als auch als Form-Daten akzeptiert (gleiche Route, zwei Aktionen) | z. B. `API/GasController.cs:129-140` | [VERIFIZIERT] |
| `PUT …/update` ersetzt alle Felder (auch `files`, `extraFields`, `tags`); fehlende Felder werden geleert – kein PATCH, keine Konflikterkennung (kein ETag/Version) | z. B. `API/GasController.cs:271-282` | [VERIFIZIERT] |
| `DELETE` erwartet `id` als Query-Parameter | Routen `…/delete` | [VERIFIZIERT] |
| Anhänge in API-Objekten sind frei setzbare `{name, location}`-Paare; `location` wird nicht validiert (beliebige URL oder fremder Dokumentpfad) | `API/*Controller.cs` (`input.Files`) | [VERIFIZIERT] |

## 4. Endpunktkatalog der öffentlichen API

Legende Auth: *Login* = beliebiger angemeldeter Nutzer; *Collab(P)* = Zugriff auf Fahrzeug mit Recht P (`CollaboratorFilter`); *Rec(P)* = Recht P auf das Fahrzeug des bestehenden Datensatzes (Prüfung im Code); *Key(P)* = API-Key benötigt Recht P; *Root* = nur Root-User. „doc“ = in `api.json` dokumentiert.

### 4.1 Allgemein / System

| Methode | Pfad | Zweck | Auth | doc | Quelle | Status |
|---|---|---|---|---|---|---|
| GET | `/health` | Health-Check (DB) | anonym | – | `APIController.cs:135-155` | [VERIFIZIERT] |
| GET | `/api/info` | Locale, Währungssymbol, Dezimaltrenner, Datumsformat, Version | Login | ✓ | `APIController.cs:156-170` | [VERIFIZIERT] |
| GET | `/api/version?checkForUpdate` | aktuelle/neueste Version (GitHub-Abfrage) | Login | ✓ | `APIController.cs:171-198` | [VERIFIZIERT] |
| GET | `/api/whoami` | Benutzername, E-Mail, IsAdmin, IsRoot | Login | ✓ | `APIController.cs:115-134` | [VERIFIZIERT] |
| GET | `/api/extrafields` | Extra-Field-Definitionen je Record-Typ | Login | ✓ | `APIController.cs:590-625` | [VERIFIZIERT] |
| POST | `/api/documents/upload` | Multipart-Upload, legt Dateien direkt in `/documents/` ab, liefert `{name, location}` | Login (kein Key-Filter) | ✓ | `APIController.cs:450-481` | [VERIFIZIERT] (ausgeführt) |
| GET | `/api/calendar` | iCal (`text/calendar`) aller zugänglichen Reminder mit Datum | Login | ✓ | `API/ReminderController.cs:274-286` | [VERIFIZIERT] |
| GET | `/api/vehicle/reminders/send?id&tags&urgencies` | Reminder-E-Mails auslösen | Root | ✓ | `APIController.cs:482-589` | [VERIFIZIERT] |
| GET | `/api/makebackup?output=download|email` | Backup-ZIP erzeugen/herunterladen/mailen | Root | ✓ | `APIController.cs:626-663` | [VERIFIZIERT] |
| GET | `/api/tempfiles` | Liste temporärer Dateien | Root | ✓ | `APIController.cs:664-671` | [VERIFIZIERT] |
| GET | `/api/cleanup?deepClean` | Temp leeren, unreferenzierte Bilder/Dokumente löschen | Root | ✓ | `APIController.cs:672-702` | [VERIFIZIERT] |
| GET | `/api/demo/restore` | Demo-Datenbank wiederherstellen (überschreibt Daten!) | Root | – | `APIController.cs:703-710` | [VERIFIZIERT] |
| WS | `/api/ws` | SignalR-Hub, Methode `JoinGroup("kiosk"|"vehicleId_<id>")`, Event `ReceiveChangeForAllVehicles(payload)`; nur wenn `LUBELOGGER_WEB_SOCKET` | Login | – | `EventHubLogic.cs`, `EventLogic.cs:29-37` | [VERIFIZIERT] |

### 4.2 Fahrzeuge

| Methode | Pfad | Zweck | Auth | doc | Quelle | Status |
|---|---|---|---|---|---|---|
| GET | `/api/vehicles` | alle zugänglichen Fahrzeuge (vollständiges `Vehicle`-Objekt) | Login | ✓ | `APIController.cs:199-216` | [VERIFIZIERT] |
| GET | `/api/vehicle/info?vehicleId` | Kennzahlen je Fahrzeug (Kosten/Anzahl je Typ, Reminder-Zähler, nächster Reminder, Planer-Zähler, letzter km-Stand) | Login; bei `vehicleId` View-Recht | ✓ | `APIController.cs:218-252`, `VehicleLogic.GetVehicleInfo` | [VERIFIZIERT] (ausgeführt) |
| GET | `/api/vehicle/adjustedodometer?vehicleId&odometer` | Tachokorrektur anwenden (BR-022) | Collab(View) | ✓ | `APIController.cs:253-267` | [VERIFIZIERT] |
| POST | `/api/vehicles/add` | Fahrzeug anlegen (`year, make, model, licensePlate, identifier, fuelType ∈ {Gasoline, Diesel, Electric}, useEngineHours, odometerOptional, tags, extraFields`); Ersteller wird Kollaborator | Login (**kein** Key-Filter) | ✓ | `APIController.cs:268-341` | [VERIFIZIERT] (ausgeführt) |
| PUT | `/api/vehicles/update` | Fahrzeug ändern (Kauf-/Verkaufsdaten, Bild, Tachokorrektur nicht änderbar; Kraftstoffart wird nie zurückgesetzt) | Rec(Edit), Key(Edit) | ✓ | `APIController.cs:365-449` | [VERIFIZIERT] |
| DELETE | `/api/vehicles/delete?id` | Fahrzeug inkl. aller Records löschen | direkter Kollaborator, Key(Delete) | ✓ | `APIController.cs:342-364` | [VERIFIZIERT] |

### 4.3 Record-Endpunkte (einheitliches Muster)

Für jeden Typ gibt es: `GET …/all` (alle zugänglichen Fahrzeuge, Login), `GET …?vehicleId` (Collab(View)), `POST …/add?vehicleId` (Collab(Edit), Key(Edit)), `PUT …/update` (Rec(Edit), Key(Edit)), `DELETE …/delete?id` (Rec(Delete), Key(Delete)). [VERIFIZIERT: Attribute in `Controllers/API/*.cs`, siehe Anhang A in `02-feature-inventar.md`]

| Typ | Basis-Pfad | Pflichtfelder bei add | Besonderheiten | Quelle | Status |
|---|---|---|---|---|---|
| Odometer | `/api/vehicle/odometerrecords` | date, odometer | zusätzlich `GET …/latest` (BR-015), `PUT …/recalculate` (BR-019); `initialOdometer` leer/0 → BR-017; `equipmentRecordId` (Leerzeichen-Liste) wird gegen Fahrzeug validiert; `autoIncludeEquipment` | `API/OdometerController.cs` | [VERIFIZIERT] |
| Gas | `/api/vehicle/gasrecords` | date, odometer, fuelConsumed, cost, isFillToFull, missedFuelUp | GET liefert berechnete `fuelEconomy` je nach `useMPG`/`useUKMPG` (Default **false**) und die umgerechnete Menge (BR-008/009) | `API/GasController.cs` | [VERIFIZIERT] (ausgeführt) |
| Service | `/api/vehicle/servicerecords` | date, odometer, description, cost | Auto-Odometer bei `EnableAutoOdometerInsert` | `API/ServiceController.cs` | [VERIFIZIERT] |
| Repair | `/api/vehicle/repairrecords` | wie Service | | `API/RepairController.cs` | [VERIFIZIERT] |
| Upgrade | `/api/vehicle/upgraderecords` | wie Service | | `API/UpgradeController.cs` | [VERIFIZIERT] |
| Tax | `/api/vehicle/taxrecords` | date, description, cost | Wiederkehr-Felder über API nicht setzbar [VERIFIZIERT: `TaxRecordExportModel` ohne diese Felder]; `GET …/check` erzeugt fällige Folgeeinträge (BR-035) **per GET** | `API/TaxController.cs` | [VERIFIZIERT] |
| Supply | `/api/vehicle/supplyrecords` | date, partQuantity, description, cost | `vehicleId=0` = Werkstattlager (nur wenn aktiviert) | `API/SupplyController.cs` | [VERIFIZIERT] |
| Plan | `/api/vehicle/planrecords` | description, type (nur Service/Repair/Upgrade), priority, progress (≠ Done), cost | Status „Done“ per API nicht setzbar (keine Konvertierung in Wartungseintrag); keine Supply-/Reminder-Verknüpfung über API [VERIFIZIERT: `PlanRecordExportModel`] | `API/PlanController.cs` | [VERIFIZIERT] |
| Reminder | `/api/vehicle/reminders` | description, metric (+ dueDate/dueOdometer je Metrik) | GET liefert berechnete `urgency`, `dueDays`, `dueDistance`; Wiederkehr, Intervalle, eigene Schwellen über API **nicht** setzbar | `API/ReminderController.cs` | [VERIFIZIERT] (ausgeführt) |
| Equipment | `/api/vehicle/equipmentrecords` | description, isEquipped | GET-Variante liefert `distanceTraveled` (BR-024) | `API/EquipmentController.cs` | [VERIFIZIERT] |
| Note | `/api/vehicle/notes` | description, noteText | | `API/NoteController.cs` | [VERIFIZIERT] |

Nicht über die API verfügbar: Inspektionen, Planvorlagen, Inspektionsvorlagen, Kollaboratoren/Haushalte, Benutzerverwaltung, Einstellungen, Import/Export, Berichte (außer `vehicle/info`). [VERIFIZIERT: keine entsprechenden Routen]

### 4.4 Beispiel (ausgeführt)

```http
POST /api/vehicle/gasrecords/add?vehicleId=1
Content-Type: application/json

{"date":"2026-02-01","odometer":"2000","fuelConsumed":"30","cost":"50","isFillToFull":"true","missedFuelUp":"false"}
→ 200 {"success":true,"message":"Gas Record Added","additionalData":{"recordId":3}}

GET /api/vehicle/gasrecords?vehicleId=1&useMPG=true   (Header culture-invariant: 1)
→ [{"vehicleId":1,"id":3,"date":"2026-02-01","odometer":2000,"fuelConsumed":30,"cost":50,
    "fuelEconomy":20,"isFillToFull":true,"missedFuelUp":false,…}]
```

## 5. Ausgehende Formate

| Format | Inhalt | Quelle | Status |
|---|---|---|---|
| Webhook-Payload | `{type:"gasrecord.add"|"…​.api"|"vehicle.delete"|"bulk.move"…, timestamp (UTC ISO 8601), data:{user, description, odometer, cost …}, vehicleId, username, action (Klartextsatz)}` | `Models/Shared/WebHookPayload.cs` | [VERIFIZIERT] |
| Discord-Payload | `{username:"LubeLogger", avatar_url, content:action}` bei URL-Präfix `discord://` | `WebHookPayload.cs` (`DiscordWebHook`), `EventLogic.cs:50-54` | [VERIFIZIERT] |
| Notification-Template | Platzhalter `{vehicleId} {title} {message} {priority} {link} {domain}` in URL, Headern und Body; Prioritäts-Mapping je Dringlichkeit | `NotificationLogic.cs:333-376` | [VERIFIZIERT] |
| iCal | siehe BR-053 | `StaticHelper.RemindersToCalendar` | [VERIFIZIERT] |

## 6. Bewertung für die Neuimplementierung

- Die öffentliche API deckt nur einen Teil der Funktionen ab; die Web-UI nutzt fast ausschließlich die internen Endpunkte (S-2). Eine Kompatibilitätsschicht müsste daher nur S-1 berücksichtigen. [ABGELEITET]
- Clients Dritter (z. B. Home-Assistant-Integrationen, Kurzbefehle) hängen vermutlich an Pfaden, String-Werten und `x-api-key`; Umfang der Nutzung ist unbekannt. [UNKLAR] → Frage Q-07.
