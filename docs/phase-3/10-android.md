# Android-App – Stand Iteration 2

- **Status:** Basis angelegt · **Datum:** 2026-09-30 · **Anlass:** Wunsch des Auftraggebers, die App parallel zur Web-App zu entwickeln
- **Grundlagen:** ADR-021 (Offline und Synchronisation), ADR-006 (Client-IDs), ADR-010 (Plausibilität), ADR-015 (Sitzung, CSRF); App-Entwürfe der Vorlage „Vectra Brand“ (AppBar, TabBar, Login, Dashboard, Kilometer, Fahrzeuge, Mehr)

## Aufbau (`android/`)

| Modul | Inhalt | Baut ohne Android-SDK |
|---|---|---|
| `:core` | Kotlin/JVM: DTOs nach `api/openapi.yaml`, `ApiClient` (OkHttp, Sitzungs-Cookie, CSRF-Header, Problem Details, Uhrabweichung aus `Date`), Outbox mit Zustandsautomat nach ADR-021, `VectraRepository` (Lesen mit Cache-Rückfall, Schreiben über die Outbox), UUIDv7, deutsche Formate | ja |
| `:app` | Android/Compose: Theme und Komponenten aus dem Design-System, Screens, Room (Outbox, Lesecache), WorkManager (`SyncWorker`), `MainViewModel` mit eigener Navigation | nein |
| `:uicheck` | nur Prüfung: übersetzt Theme, Komponenten und Screens aus `:app` mit Compose Multiplatform für die JVM und rendert sie als PNG | ja (Rendern braucht Google-Maven) |

Die Screens sind zustandslos und plattformneutral (`app/src/main/kotlin/app/vectra/android/{ui,feature}`). Android-spezifisch sind nur Ressourcen, Datenhaltung, Worker, ViewModel und Activity.

## Umgesetzt

- **Anmeldung** mit Server-Adresse, E-Mail und Passwort. Übergangsweise über das Sitzungs-Cookie mit CSRF-Token, bis `POST /auth/token` umgesetzt ist (dann API-Token im Keystore).
- **Übersicht** nach dem Entwurf „Dashboard“: Fahrzeugwahl, Status („bereit“ bzw. Einträge mit Bestätigungsbedarf), Kennzahl Kilometerstand, Schnellerfassung, zuletzt erfasste Messpunkte. Kennzahlen späterer Module (Service, Verbrauch, Öl) zeigen „–“, keine Beispielwerte.
- **Kilometerstand** nach dem Entwurf „Kilometer“: aktueller Stand mit Herkunft, Hinweise zu Befunden, Monatsbalken (ODO-05), Verlauf mit Zuwachs, bestätigte Abweichungen markiert.
- **Offline-Erfassung:** Jede Erfassung geht in die Outbox (client-erzeugte UUIDv7). `SyncWorker` sendet FIFO je Fahrzeug. Die Antworten werden nach der Tabelle in ADR-021 behandelt: 2xx entfernt den Eintrag, eine bestätigbare `422` setzt „braucht Bestätigung“ und hält spätere Einträge desselben Fahrzeugs an, `412` wird zum Konflikt, `403`/`404`/nicht bestätigbare `422` gelten als fehlgeschlagen, `5xx` und Netzfehler lösen Backoff aus, `401` bricht ab. Der Bestätigungsdialog verlangt eine Begründung (ADR-010).
- **Fahrzeuge**, **Mehr** (Kacheln, noch nicht umgesetzte Bereiche gedimmt), **Einstellungen** (Konto, Hell/Dunkel/System, Sync-Status, Warnung bei Uhrabweichung ≥ 2 min, Abmelden mit Hinweis auf nicht übertragene Einträge).
- **Design:** Farben hell/dunkel, Poppins/Inter (SIL OFL, auf Latin reduziert, `assets/licenses`), Icon-Set als ImageVectors (aus `web/src/components/Icon.tsx` erzeugt, ergänzt um more, shield, cloudoff), Markenlogos, adaptives App-Icon.
- **Sicherheit:** keine Sicherung von Cookies, Datenbank und Einstellungen (`allowBackup=false`, Extraktionsregeln), Klartext-HTTP nur für `10.0.2.2`/`localhost`.

## Iteration 2: Alltag und Fahrzeugakte

- **Wartung** (Tab): Fälligkeiten nach Dringlichkeit (Level, Datum bzw. Rest-km, geschätztes Datum); Antippen markiert die Wartung als erledigt (mit `Idempotency-Key`).
- **Servicehistorie** (über Wartung oder Mehr): Werkstattbesuche mit Summen; neuer Eintrag mit Art, Kilometerstand, Teile/Arbeit/Sonstiges und abgehakten Wartungen.
- **Kosten** (Tab): Jahressumme, je km und je Tag, Balken je Kategorie, anstehende Kosten aus Plänen („Bezahlt“ bestätigt das Vorkommen), sonstige Kosten erfassen.
- **Fahrten** (Tab): laufende Fahrt oder Monatssumme mit Anteilen je Kategorie; Fahrt starten (Startstand vorbelegt) und beenden (`If-Match`); Lücken zwischen Fahrten werden angezeigt.
- **Dokumente** (Mehr, Schnellerfassung „Beleg“): Fahrzeugakte mit Vorschaubildern; Foto oder PDF über die Systemauswahl hochladen, danach Titel und Typ.
- **Fahrzeugfoto:** auf der Fahrzeugkarte „Foto hinzufügen/ändern“; Übersicht und Karte zeigen die bereinigte Vorschau (ohne EXIF, ADR-017).
- **Übersicht:** Kacheln „nächste Wartung“ und „Kosten dieses Jahr“, Schnellerfassung Fahrt/Wartung/Kosten/Beleg.
- **Befunde:** Die Dialoge zeigen Plausibilitätsbefunde (ADR-010) und erlauben „Trotzdem speichern“ mit Begründung.
- **Online-Schreiben:** Diese Erfassungen gehen direkt an den Server; ohne Verbindung meldet der Dialog das. Die Outbox deckt weiterhin nur Kilometerstände ab. Gelesen wird mit Cache-Rückfall (offline zuletzt geladener Stand).
- `:core`: DTOs in `model/Modules.kt`, neue Endpunkte im `ApiClient` (inkl. Multipart-Upload und Vorschau), `MoneyFormat` (kleinste Einheit, ADR-029).

## Angemeldet bleiben, Synchronisierung, Rückfragen

- **Angemeldet bleiben:** Die gespeicherten Cookies werden beim Start sofort geladen (vorher erst bei der ersten Anfrage, weshalb die App nach einem Neustart zur Anmeldung sprang). Die App meldet sich mit `client_kind=android` an: Sitzung bis 365 Tage, Leerlauf 90 Tage. Erst eine abgelaufene oder beendete Sitzung führt zur Anmeldung zurück; die Outbox bleibt dabei erhalten.
- **Synchronisierung bei Verbindung:** `VectraApp` beobachtet das Netz (`registerDefaultNetworkCallback`). Wird die Verbindung wieder hergestellt, startet sofort ein Sync (`SyncWorker.syncNow`) und die Anzeige lädt neu; ein Hinweis am unteren Rand meldet „Wieder online“ bzw. „Offline“.
- **Rückfragen als Hinweis-Box:** Hat die Synchronisierung eine Rückfrage (Befund nach ADR-010, Konflikt, abgelehnter Eintrag), öffnet sich der Dialog von selbst. Der Nutzer antwortet: *Der Wert stimmt so* (mit Begründung), *Wert korrigieren* (neuer Stand, neue ID, wird neu geprüft – `Outbox.amendValue`), *Verwerfen* oder *Später entscheiden*.
- **Wartungsbuch übernehmen:** auf dem Wartungs-Tab; Modell wird nach Marke/Modell vorgewählt.

## Assistent

- Übersicht → „Mit dem Assistenten sprechen“ bzw. Mehr → Assistent: Chat mit Claude (ADR-032), Antworten als Stream mit Zwischenschritten.
- Mikrofon: Spracherkennung von Android (`RecognizerIntent`, Deutsch); Diktier-Tastaturen wie Wispr Flow funktionieren im Eingabefeld.
- Vorschläge erscheinen als Karten mit **Bestätigen/Verwerfen**; bei Befunden mit Begründungsfeld. Nur online.

## Prüfung

| Prüfung | Ergebnis in dieser Umgebung |
|---|---|
| `:core:test`: 21 Tests (Outbox-Zustände, Bestätigung, Wiederholung mit gleicher ID, Cookie/CSRF, Problem Details, Cache-Rückfall, Monatsgrenzen in lokaler Zone, UUIDv7, Formate) | grün |
| Integrationstest gegen das laufende Backend: Fahrzeug anlegen, Stand senden, P1-Befund → „braucht Bestätigung“, Wiederholung liefert `200` statt eines Duplikats, bestätigen → `201`, Status `confirmed_anomaly`, Fahrzeug löschen | grün |
| `:uicheck:compileKotlin` (alle Screens, Theme, Komponenten, Icons) | grün |
| `:app:assembleDebug`, `:app:lintDebug` | lokal nicht ausführbar (`dl.google.com`/`maven.google.com` gesperrt); in der CI grün (Job „Android“, Artefakt APK) |
| Rendern der Screens als PNG | nur in der CI (Artefakt „android“) |

## Offen

- Tachofoto mit SHA-256 (ADR-018) als Beleg am Messpunkt; Kamera-Aufnahme direkt aus der App (bisher Systemauswahl).
- Offline-Erfassung (Outbox) für Wartung, Service, Kosten und Fahrten.
- Änderungsfeed `GET /sync/changes` (Backend noch 501); bis dahin lädt die App bei jedem Aufruf neu und nutzt den Cache nur offline.
- API-Token statt Sitzungs-Cookie; Organisations-Login (OIDC) aus dem Login-Entwurf.
- Konfliktdialog mit beiden Fassungen, sobald es PATCH-Operationen in der Outbox gibt (Messpunkte werden nur angelegt).
- Instrumentierte Tests (Room, Worker) und Screenshot-Vergleich in der CI.
