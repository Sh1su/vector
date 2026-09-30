# 06 – Authentifizierung, Rollen und Berechtigungen (Ist-Zustand)

Quellenbezug: `hargata/lubelog` @ `dd69e59` (v1.7.3).

## 1. Betriebsmodi

| Aussage | Quelle | Status |
|---|---|---|
| Authentifizierung ist **standardmäßig aus** (`EnableAuth: false`). Dann erhält jede Anfrage eine synthetische Identität „admin“ mit Id −1 und Rolle `IsRootUser` – ohne Login, auch über das Netzwerk | `appsettings.json`, `Middleware/Authen.cs:36-49` | [VERIFIZIERT] (ausgeführt) |
| Aktiviert wird sie, indem der (dann anonyme) Root-User Benutzername/Passwort setzt; diese werden als SHA-256-Hashes in `data/config/userConfig.json` gespeichert und `EnableAuth=true` gesetzt | `LoginController.CreateLoginCreds` (`LoginController.cs:590-604`), `LoginLogic.CreateRootUserCredentials` (`LoginLogic.cs:503-533`) | [VERIFIZIERT] (ausgeführt) |
| Beim erstmaligen Setzen wird `userConfig.json` mit Klassen-Defaults angelegt, wodurch Servereinstellungen wie `UseMPG` von `true` (appsettings) auf `false` springen | `LoginLogic.cs:521-530`, `Models/Settings/UserConfig.cs` | [VERIFIZIERT] (ausgeführt) |

## 2. Identitäten

| Identität | Beschreibung | Quelle | Status |
|---|---|---|---|
| **Root-User** | Kein DB-Datensatz; Id −1; Zugangsdaten nur als ungesalzene SHA-256-Hashes von Benutzername **und** Passwort in der Config-Datei; alternativ OIDC-Login, wenn `EnableRootUserOIDC` und E-Mail = `DefaultReminderEmail`. Hat alle Rechte (`IsRootUser` + `IsAdmin`). | `ConfigHelper.AuthenticateRootUser` (`ConfigHelper.cs:187-206`), `LoginLogic.GetRootUserData` (`LoginLogic.cs:556-566`) | [VERIFIZIERT] |
| **Benutzer** | `UserData` in DB: `UserName` (eindeutig per Prüfung, kein DB-Constraint), `EmailAddress` (eindeutig per Prüfung), `Password` (SHA-256-Hex ohne Salt; leer = nur OIDC), `IsAdmin` | `Models/User/UserData.cs`, `LoginLogic.RegisterNewUser` (`LoginLogic.cs:161-201`) | [VERIFIZIERT] (Hash ausgeführt nachgerechnet) |
| **Admin** | Benutzer mit `IsAdmin=true` | `AdminController` (`[Authorize(Roles = IsAdmin)]`) | [VERIFIZIERT] |
| **API-Key** | Gehört einem Benutzer (oder Root, `UserId = −1`), hat Name und Berechtigungsliste `View/Edit/Delete`; gespeichert wird nur SHA-256 des 32-stelligen Hex-Schlüssels | `UserLogic.CreateAPIKey` (`UserLogic.cs:299-323`) | [VERIFIZIERT] |

## 3. Anmeldeverfahren

| Verfahren | Ablauf | Quelle | Status |
|---|---|---|---|
| Formular-Login | POST `/Login/Login` → Prüfung Root-Hash oder DB-Hash → Cookie `ACCESS_TOKEN` | `LoginController.cs:494-543` | [VERIFIZIERT] |
| HTTP Basic | Bei jeder Anfrage; für alle Pfade, nicht nur API | `Authen.cs:66-102` | [VERIFIZIERT] (ausgeführt) |
| API-Key | Header `x-api-key` oder Query `apiKey`; nur für `/api`, `/kiosk`, `/images`, `/documents`, `/temp` | `Authen.cs:156-183` | [VERIFIZIERT] (ausgeführt) |
| OIDC | Authorization-Code-Flow; optional `state`-Prüfung (`ValidateState`, Default aus) und PKCE (`UsePKCE`, Default aus); Token-Signatur nur geprüft, wenn `JwksURL` gesetzt, **ohne** Issuer-/Audience-Prüfung, ohne Nonce; Nutzerzuordnung ausschließlich über den `email`-Claim (kein `email_verified`); optional `DisableRegularLogin` | `LoginController.RemoteAuth` (`LoginController.cs:140-328`), `Models/OIDC/OpenIDConfig.cs` | [VERIFIZIERT] |
| OIDC-Registrierung | Unbekannte E-Mail → Registrierungsseite; Token optional automatisch erzeugt (`AutoGenerateTokens`), Registrierung abschaltbar | `LoginController.cs:282-295`, `LoginLogic.RegisterOpenIdUser` | [VERIFIZIERT] |
| Debug-Endpunkt | `/Login/RemoteAuthDebug` zeigt Details des OIDC-Austauschs (für Einrichtung) | `LoginController.cs:329-493` | [VERIFIZIERT] |

## 4. Sitzungen

| Aussage | Quelle | Status |
|---|---|---|
| Sitzung = zustandsloses Cookie `ACCESS_TOKEN`: per ASP.NET DataProtection verschlüsseltes JSON `{UserData (inkl. IsAdmin), ExpiresOn}` | `LoginController.cs:508-515`, `Models/Login/AuthCookie.cs` | [VERIFIZIERT] |
| Bei jeder Anfrage wird nur geprüft, ob der Benutzer noch existiert; Rollen (IsAdmin) stammen **aus dem Cookie** – Entzug von Adminrechten wirkt erst nach Ablauf (bis 90 Tage) | `Authen.cs:103-150`, `LoginLogic.CheckIfUserIsValid` | [VERIFIZIERT] |
| Keine serverseitige Sitzungsliste, kein Widerruf; Logout löscht nur das Cookie | `LoginController.LogOut` (`LoginController.cs:625-636`) | [VERIFIZIERT] |
| Cookie ohne `HttpOnly`, `Secure`, `SameSite`-Attribute | ausgeführt: `Set-Cookie: ACCESS_TOKEN=…; expires=…; path=/` | [VERIFIZIERT] (ausgeführt) |
| Kein CSRF-Schutz (keine Antiforgery-Tokens) | Suche nach `AntiForgery`/`ValidateAntiForgeryToken` ohne Treffer | [VERIFIZIERT] |
| Laufzeit: 1 Tag bzw. 1–90 Tage mit „angemeldet bleiben“ (BR-051) | `ConfigHelper.cs:114-133` | [VERIFIZIERT] |
| DataProtection-Schlüssel liegen in `/root/.aspnet/DataProtection-Keys` (eigenes Volume); ohne Persistenz werden alle Sitzungen beim Neustart ungültig | `docker-compose*.yml`, Startlog (ausgeführt) | [VERIFIZIERT] |

## 5. Registrierung, Token, Passwort

| Aussage | Quelle | Status |
|---|---|---|
| Registrierung nur mit Token, das an eine E-Mail-Adresse gebunden ist; Token = erste 8 Zeichen einer GUID (32 Bit), Klartext in DB, **ohne Ablaufdatum**, einmalig (wird bei Nutzung gelöscht) | `LoginLogic.NewToken` (`LoginLogic.cs:569-572`), `RegisterNewUser` | [VERIFIZIERT] |
| Token erzeugen: Admin (für beliebige Adressen, optional per E-Mail) oder – bei `LUBELOGGER_OPEN_REGISTRATION` – selbst per E-Mail; `DisableRegistration` sperrt die Registrierungsseite | `AdminController.GenerateNewToken`, `LoginLogic.SendRegistrationToken`, `LoginController.Registration` | [VERIFIZIERT] |
| Passwort-Reset per E-Mail: gleicher Token-Mechanismus; Antwort immer „erfolgreich“ (keine Benutzer-Enumeration) | `LoginLogic.RequestResetPassword` (`LoginLogic.cs:218-229`) | [VERIFIZIERT] |
| Kontoänderung (Name/E-Mail/Passwort) erfordert einen per E-Mail gesendeten Token | `LoginLogic.UpdateUserDetails` (`LoginLogic.cs:77-120`), `HomeController.GenerateTokenForUser` | [VERIFIZIERT] |
| Admin kann Passwort widerrufen (Konto wird OIDC-only) oder auf zufälligen Wert setzen | `AdminController.RevokeUserPassword/ResetUserPassword`, `LoginLogic.cs:349-384` | [VERIFIZIERT] |
| Keine Passwortrichtlinie, kein Rate-Limiting/Lockout; fehlgeschlagene Logins werden mit IP geloggt | `LoginController.cs:518-536` | [VERIFIZIERT] |
| Keine Zwei-Faktor-Authentifizierung | – (nicht vorhanden) | [VERIFIZIERT] |

## 6. Rollenmodell

| Rolle | Rechte | Quelle | Status |
|---|---|---|---|
| Root | Alles: alle Fahrzeuge (Filter werden übersprungen), Server-Setup, Backup/Restore, Migration, Extra-Fields, Übersetzungen, Themes, Widgets, Cleanup, Reminder-Mails | `Filter/CollaboratorFilter.cs:282`, `[Authorize(Roles = IsRootUser)]` an diversen Aktionen | [VERIFIZIERT] |
| Admin | Admin-Panel: alle Benutzer sehen/löschen, Adminrechte vergeben/entziehen, Tokens erzeugen/sehen/löschen, Haushalte beliebiger Benutzer verwalten, Passwörter zurücksetzen. **Kein** Zugriff auf fremde Fahrzeuge | `Controllers/AdminController.cs`; `CollaboratorFilter` prüft nur Root | [VERIFIZIERT] |
| Benutzer | Eigene und geteilte Fahrzeuge; eigene Einstellungen, API-Keys, Haushalt | `HomeController.cs:250-315` | [VERIFIZIERT] |

Admins können sich gegenseitig die Adminrechte entziehen oder löschen; es gibt keine Schutzregel „letzter Admin“. [VERIFIZIERT: `AdminController.cs:73-88` ohne Prüfung]

## 7. Fahrzeugfreigabe

### 7.1 Kollaboratoren (`UserAccess`)
| Aussage | Quelle | Status |
|---|---|---|
| Ein Kollaborator hat **vollen** Zugriff auf das Fahrzeug (Lesen, Schreiben, Löschen von Records, Fahrzeug löschen, weitere Kollaboratoren hinzufügen/entfernen). Es gibt keine abgestuften Rollen und keinen Eigentümer | `UserLogic.UserCanDirectlyEditVehicle` (`UserLogic.cs:180-192`), `StrictCollaboratorFilter` an `DeleteVehicle`, `AddCollaboratorsToVehicle` | [VERIFIZIERT] |
| Wer ein Fahrzeug anlegt, wird Kollaborator | `VehicleController.SaveVehicle` (`VehicleController.cs:149-151`), `APIController.AddVehicle` | [VERIFIZIERT] |
| Kollaboratoren werden per Benutzername hinzugefügt, ohne Zustimmung des Hinzugefügten | `UserLogic.AddCollaboratorToVehicle` | [VERIFIZIERT] |
| Ein Kollaborator kann den ursprünglichen Ersteller entfernen | `UserLogic.DeleteCollaboratorFromVehicle` ohne Einschränkung | [VERIFIZIERT] |

### 7.2 Haushalte (`UserHousehold`)
| Aussage | Quelle | Status |
|---|---|---|
| Ein Benutzer (Parent) kann andere Benutzer (Child) per Benutzername in seinen Haushalt aufnehmen – ohne Zustimmung des Childs; Root kann keinen Haushalt haben | `UserLogic.AddUserToHousehold` (`UserLogic.cs:235-272`), `HomeController.AddUserToHousehold` | [VERIFIZIERT] |
| Children sehen alle Fahrzeuge, auf die der Parent **direkt** Kollaborator ist (eine Ebene, nicht transitiv) | `UserLogic.FilterUserVehicles` (`UserLogic.cs:123-154`), `UserCanEditVehicle` (`UserLogic.cs:155-179`) | [VERIFIZIERT] |
| Rechte je Haushaltsmitglied: `View` immer, zusätzlich `Edit` und/oder `Delete` je nach Konfiguration durch den Parent | `UserLogic.cs:171-175`, `UpdateUserHousehold` | [VERIFIZIERT] |
| Children können keine Kollaboratoren verwalten und das Fahrzeug nicht löschen (Strict-Filter) | `StrictCollaboratorFilter` | [VERIFIZIERT] |
| Children können den Haushalt verlassen; zirkuläre Beziehungen werden nur direkt (A↔B) verhindert | `HomeController.LeaveHousehold`, `UserLogic.cs:258-263` | [VERIFIZIERT] |
| Reminder-E-Mails gehen nur an direkte Kollaboratoren, nicht an Haushaltsmitglieder | `NotificationLogic.cs:172-182, 286-296` | [VERIFIZIERT] |

### 7.3 Werkstattlager (Shop Supplies)
Supplies mit `VehicleId = 0` sind – wenn `EnableShopSupplies` aktiv ist – für **alle** angemeldeten Benutzer les- und schreibbar. [VERIFIZIERT: `SupplyController.cs:157-168`, `CollaboratorFilter.cs:305-316`]

## 8. Berechtigungsmatrix (Ist)

| Aktion | Root | Admin (ohne Freigabe) | Kollaborator | Haushalt View | Haushalt Edit | Haushalt Delete | API-Key View-only |
|---|---|---|---|---|---|---|---|
| Fahrzeug/Records lesen | ✓ | – | ✓ | ✓ | ✓ | ✓ | ✓ |
| Record anlegen/ändern | ✓ | – | ✓ | – | ✓ | – | – (ausgeführt: „Access Denied“) |
| Record löschen | ✓ | – | ✓ | – | – | ✓ | – |
| Fahrzeug anlegen | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | **✓** (API ohne Key-Filter) |
| Fahrzeug bearbeiten | ✓ | – | ✓ | – | ✓ | – | – |
| Fahrzeug löschen | ✓ | – | ✓ | – | – | – | – |
| Kollaboratoren verwalten | ✓ | – | ✓ | – | – | – | – |
| Wiederkehrende Gebühren fortschreiben (`taxrecords/check`) | ✓ | – | ✓ | **✓** | ✓ | ✓ | **✓** |
| Benutzer verwalten | ✓ | ✓ | – | – | – | – | – |
| Server-Setup, Backup, Migration | ✓ | – | – | – | – | – | – |

Quelle: Filter-Attribute (Anhang A, `02-feature-inventar.md`), `UserLogic.UserCanEditVehicle`, `APIKeyFilter`. Status: [VERIFIZIERT] für Code-Pfade; Einträge „✓“ unter „API-Key View-only“ für Fahrzeug anlegen [VERIFIZIERT] (ausgeführt).

**Abweichung Soll/Ist:** Die Matrix beschreibt die beabsichtigte Wirkung der Filter. Durch die in `09-risiken-und-altlasten.md` beschriebenen Lücken (R-01 bis R-05) sind tatsächlich weitergehende Zugriffe möglich (z. B. Überschreiben fremder Records). [VERIFIZIERT] (ausgeführt)

## 9. Abbildung auf das Zielbild (Hinweis für Phase 2)

| Ist | Möglicher Zielbegriff | Status |
|---|---|---|
| Kollaborator (Vollzugriff) | Eigentümer bzw. Bearbeiter mit Verwaltungsrecht | [ABGELEITET] – Ist kennt keinen Eigentümer; Migrationsregel nötig (Q-04) |
| Haushalt + `View` | Leser (auf alle Fahrzeuge des Parents) | [ABGELEITET] |
| Haushalt + `Edit` (+`Delete`) | Bearbeiter | [ABGELEITET] |
| Root | System-Administrator (Installation) | [ABGELEITET] |
| Admin | Benutzerverwaltung ohne Datenzugriff | [ABGELEITET] |
| API-Key-Berechtigungen | Token-Scopes | [ABGELEITET] |
