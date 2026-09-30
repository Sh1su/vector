# 30 – Migrationskonzept LubeLogger → Vectra

- **Status:** Entwurf (AP-7) · **Datum:** 2026-09-30
- **Grundlagen:** ADR-027 (Offline-Import), ADR-000 (Clean Room), E-4 bis E-6; Phase 1 `03-domain-und-datenmodell.md`, `08-import-export-und-migration.md` (MG-1 bis MG-15)
- **Clean Room:** Das Konzept nutzt nur **Fakten über das Fremdformat** (Collection-, Feld- und Enum-Namen der LubeLogger-Datenbank). Diese Namen sind im Import-Modul als Mapping-Konstanten zulässig (ADR-000). Die Implementierung ist eine Neuentwicklung.

## 1. Ziel und Umfang

Nutzer einer LubeLogger-Installation (v1.7.x) übernehmen ihre Daten vollständig und nachvollziehbar nach Vectra. „Vollständig“ heißt: Jede Quellzeile landet in einem Zielobjekt oder erscheint mit Grund im Prüfbericht. Nichts wird still verworfen.

**Nicht im Umfang:** Passwörter, API-Keys und offene Tokens (E-6); ein direkter Leser für LubeLogger-PostgreSQL (PostgreSQL-Nutzer erzeugen vorher mit der Exportfunktion ihrer Installation die Datenbankdatei, ADR-027); fortlaufende Synchronisation zwischen beiden Systemen.

## 2. Eingaben

| Eingabe | Pflicht | Verwendung |
|---|---|---|
| `cartracker.db` (LiteDB-Datenbankdatei) | ja | alle Fachdaten, Konten, Freigaben, Einstellungen |
| ZIP des Ordners `data/` bzw. LubeLogger-Backup-ZIP | empfohlen | Bilder und Dokumente (`images/`, `documents/`) |
| `config/userConfig.json` | optional | Server-Defaults für Einheiten und Sprache (Vorschlagswerte) |
| `config/serverConfig.json` | optional | Kultur-/Sprachvorgabe (Vorschlagswert) |

Aus den Konfigurationsdateien werden **nur** diese Schlüssel gelesen: Einheiten-Einstellungen, Sprache, Kultur. Alle anderen Inhalte, etwa Hashes, SMTP-Zugangsdaten oder OIDC-Geheimnisse, werden weder gelesen noch gespeichert.

**Umgang mit den Quelldateien:** Sie werden in einem privaten Importbereich des Storage abgelegt, der nicht über Web-Pfade erreichbar ist, und nie ausgeliefert. Nach Abschluss werden sie automatisch gelöscht (Default 7 Tage, sofort auf Wunsch). Die Quelldateien enthalten Passwort-Hashes der Altanwendung (Phase 1, R-07). Deshalb darf der Import nur ein Konto mit Web-Sitzung starten (OpenAPI `createImport`, nur `sessionCookie`).

## 3. Ablauf

```mermaid
flowchart LR
  U[Upload] --> A[Analyse<br/>schreibt keine Fachdaten]
  A --> P[Parameter bestätigen]
  P --> V[Vorschau<br/>Prüfbericht Entwurf]
  V --> I[Import<br/>je Fahrzeug eine Transaktion]
  I --> R[Prüfbericht final<br/>Einladungen versenden]
  R --> C[Quelldateien löschen]
  P -. Parameter ändern .-> V
```

1. **Upload** (`POST /imports`): Die Dateien werden gespeichert, dann wird ein Job „Analyse“ eingereiht (ADR-019).
2. **Analyse:** Datenbank lesen (Leser siehe §9), Datensätze zählen, Vorschlagswerte ableiten (§4), Befunde sammeln (§8). Ergebnis ist `ImportAnalysis`.
3. **Parameter bestätigen** (`PUT /imports/{id}/parameters`): Pflicht sind Quell-Zeitzone, Kultur, Währung und die Einheiten je Fahrzeug. Ohne bestätigte Parameter kein Import (ADR-027).
4. **Vorschau:** Der Import wird mit den Parametern **ohne Schreiben** durchgerechnet. Das Ergebnis ist der Entwurf des Prüfberichts mit allen Befunden, zum Beispiel Plausibilität und Einheitenzweifel.
5. **Import** (`POST /imports/{id}/run`): je Fahrzeug eine Transaktion (ADR-004), idempotent (§7). Konten und Freigaben werden zuerst in einer eigenen Transaktion angelegt.
6. **Abschluss:** Einladungen an die übernommenen Konten versenden (ID-04), Ereignis `import.completed` (ADR-020), Prüfbericht als Datei ablegen.

## 4. Parameter und Vorschlagswerte

| Parameter | Vorschlag aus | Wenn unklar |
|---|---|---|
| **Quell-Zeitzone** (MG-2) | Zeitzone des importierenden Kontos | Pflichtfeld; Hinweis, dass falsche Werte Daten um einen Tag verschieben |
| **Kultur** (MG-3) | `serverConfig.json`/Sprache; Probe: Die Kaufdatums-Strings werden mit `de-DE`, `en-US`, `en-GB` geparst, und es wird angezeigt, welche Kultur alle Werte fehlerfrei liest | Pflichtfeld, wenn mehrere Kulturen passen und Werte mehrdeutig sind (z. B. 01/05/2026) |
| **Währung** (MG-14) | Default-Währung des Kontos | Pflichtfeld |
| **Einheiten je Fahrzeug** (MG-1) | siehe §4.1 | Befund `UNIT_AMBIGUOUS`, Pflichtentscheidung |
| **Rechte** (E-4) | Regeln aus §5.8 | – |
| **Root-Konto** | – | E-Mail für das LubeLogger-Root-Konto (das in der Datenbank keinen Datensatz hat); ob es Eigentümer der Fahrzeuge wird |
| **Zurückgestellte Funktionen** (E-5) | als Notizen übernehmen: ja | – |
| **Unreferenzierte Dateien** | nicht übernehmen | – |

### 4.1 Einheiten je Fahrzeug
Die Distanzeinheit (km, mi oder h) und die Mengeneinheit (l, US gal, imp gal oder kWh) werden je Fahrzeug festgelegt:
1. Fahrzeug mit Motorstundenzähler → Distanzeinheit `h`.
2. Elektrofahrzeug → Menge `kWh`.
3. Sonst: Einheiten-Einstellungen **aller Nutzer mit Zugriff** auf das Fahrzeug (eigene Einstellung, sonst Server-Default):
   - Gallonen-Einstellung aus und UK-Einstellung aus → km / l
   - Gallonen-Einstellung an → mi / US gal
   - Gallonen-Einstellung an und UK-Einstellung an → mi / l (Mengen wurden in Litern gespeichert, Phase 1 BR-008)
   - UK-Einstellung an ohne Gallonen-Einstellung → mi / l
4. Liefern die Nutzer widersprüchliche Einstellungen, entsteht der Befund `UNIT_AMBIGUOUS`. Die Analyse zeigt dann Plausibilitätshilfen: den mittleren Verbrauch je Kandidat (z. B. „5,9 l/100 km“ gegenüber „39,9 mpg“) und den typischen Abstand zwischen Tankvorgängen.

## 5. Abbildung der Quelldaten

Spalte „Quelle“ nennt Collection und Felder der LubeLogger-Datenbank (Mapping-Konstanten). Alle Zieldatensätze erhalten `origin = import:lubelogger` und einen Eintrag in `import_source_map` (§7).

### 5.1 Fahrzeuge (`vehicles`)
| Quelle | Ziel | Regel |
|---|---|---|
| `Year`, `Make`, `Model`, `LicensePlate` | `model_year`, `make`, `model`, `license_plate` | `Year = 0` → leer |
| – | `display_name` | „Make Model“, sonst „Fahrzeug {Quell-ID}“ |
| `IsElectric`, `IsDiesel` | `energy_carriers` | Elektro → `[electricity]`, Diesel → `[diesel]`, sonst `[petrol]`. Sind beide gesetzt (Phase 1 D-19), entsteht der Befund `FUEL_TYPE_CONFLICT`; Vorschlag ist der Energieträger, zu dem die Tankdaten passen (Ladezustände vorhanden → Strom) |
| `UseHours` | `usage_meter = engine_hours` | |
| `OdometerOptional` | `odometer_required = false` | |
| `PurchaseDate`, `PurchasePrice`, `SoldDate`, `SoldPrice` | `purchase`, `sale`, `status = sold` | Datum-Strings mit der Kultur parsen (MG-3); nicht lesbar → Befund `DATE_UNPARSEABLE`, Feld leer |
| `ExtraFields` | `vin` (Feldname passt ohne Beachtung von Groß-/Kleinschreibung auf „VIN“, „FIN“, „Fahrgestellnummer“), sonst `custom_fields` + Definitionen (Vehicles §2.5) | Werte bleiben Text; Typ aus der Vorlage |
| `VehicleIdentifier` | Nutzereinstellung `vehicle_identifier` | |
| `ImageLocation` | Fahrzeugbild (primär) | Default-Bild wird ignoriert |
| `MapLocation` | Dokument `other` + Notiz „Imagemap (nicht unterstützt)“ | E-5 |
| `HasOdometerAdjustment`, `OdometerMultiplier`, `OdometerDifference` | Notiz + Befund `ODOMETER_ADJUSTMENT` | Rohwerte werden unverändert als Zählerwerte übernommen (OP-ODO-2); Zählerabschnitte legt der Nutzer bei Bedarf danach an |
| `Tags` | `tags` | |
| `DashboardMetrics` | – | entfällt (Befund `info`) |

### 5.2 Kilometerstände (`odometerrecords`)
| Quelle | Ziel | Regel |
|---|---|---|
| `Date`, `Mileage` | Messpunkt `source = import`, `time_precision = date_only` | Datum nach §6; `Mileage = 0` → kein Messpunkt, Befund `ODO_ZERO` |
| `InitialMileage` | – | wird nicht übernommen, Distanzen berechnet Vectra (ODO-05) |
| `Notes`, `Tags`, `Files` | Notiz des Messpunkts, Anhänge (Rolle `photo`) | |
| `EquipmentRecordId` | Hinweis in der Notiz | Ausstattung nicht im MVP |

**Automatisch erzeugte Einträge (MG-5):** Hat ein Kilometereintrag einen Anhang mit einem Verweis auf einen anderen Datensatz (`::<Typ>:<Id>`) und stimmen Datum und Stand mit diesem Datensatz überein, wird **kein** eigener Messpunkt angelegt. Der Messpunkt des Quelldatensatzes (Tanken, Service …) übernimmt diese Rolle; es entsteht der Befund `AUTO_ODOMETER_MERGED` (info). Stimmen die Werte nicht überein, werden beide übernommen, mit Befund `AUTO_ODOMETER_MISMATCH`.

### 5.3 Tankvorgänge (`gasrecords`)
| Quelle | Ziel | Regel |
|---|---|---|
| `Date` | `occurred_at`, `date_only` | §6 |
| `Mileage` | Messpunkt `source = fuel` | 0 → ohne Stand; ist `odometer_required`, wird es für das importierte Fahrzeug trotzdem auf `false` gesetzt, wenn solche Einträge vorkommen (Befund) |
| `Gallons` | `quantity` in der Mengeneinheit des Fahrzeugs (§4.1) | Menge ≤ 0 → nicht übernommen, Befund `FUEL_QUANTITY_INVALID` (error) |
| `Cost` | `cost` | §6.3 |
| `IsFillToFull`, `MissedFuelUp` | `fill_level`, `previous_missed` | |
| `StartingSoc`, `EndingSoc` | `soc_start_pct`, `soc_end_pct` | nur bei Strom; unveränderte Standardwerte 20/80 → Befund `SOC_DEFAULT_VALUES` (die Werte waren vermutlich nicht erfasst); Übernahme nach Parameter |
| `RequisitionHistory` | Notiz | Lager nicht im MVP |

### 5.4 Service, Reparatur, Nachrüstung (`servicerecords`, `collisionrecords`, `upgraderecords`)
| Quelle | Ziel | Regel |
|---|---|---|
| Collection | `kind` = `maintenance` / `repair` / `upgrade` | |
| `Description` | `title` (max. 200 Zeichen), vollständiger Text in `description`, falls länger | |
| `Mileage` | Messpunkt `source = service` | 0 → ohne Stand |
| `Cost` | **eine** Kostenposition `other`, Bezeichnung „Gesamtbetrag (Import)“ (ADR-029) | `Cost = 0` → Position mit 0; die Altanwendung unterscheidet nicht zwischen „kostenlos“ und „unbekannt“, dazu gibt es den Befund `COST_ZERO_AMBIGUOUS` (info) |
| `RequisitionHistory` | `parts`: Name = Beschreibung, Teilenummer, Menge | Kosten der Teile als Hinweis in der Notiz, keine zweite Kostenposition (sonst doppelte Kosten) |
| `Notes`, `Tags`, `Files`, `ExtraFields` | Notiz (Zusatzfelder als „Name: Wert“), Schlagwörter, Anhänge (Rolle `invoice`) | |

### 5.5 Gebühren (`taxrecords`)
| Quelle | Ziel | Regel |
|---|---|---|
| jeder Datensatz | Kosteneintrag `category = tax` | Kategorie im Parameterdialog global änderbar (z. B. „fee“) |
| `IsRecurring = true` | zusätzlich Kostenplan (CO-02) | Intervall aus `RecurringInterval` bzw. `CustomMonthInterval` + Einheit; `first_due_on = Date + Intervall`. Intervall 0 → kein Plan, Befund `RECURRING_INTERVAL_INVALID` (Phase 1 D-11) |

Einträge, die LubeLogger selbst fortgeschrieben hat (MG-11), sind echte Kosteneinträge und werden normal übernommen. Befund `TAX_AUTOGENERATED_POSSIBLE` (info), wenn mehrere Einträge mit gleicher Beschreibung und gleichem Betrag im Intervallabstand liegen.

### 5.6 Reminder (`reminderrecords`) → Wartungsdefinitionen
| Quelle | Ziel | Regel |
|---|---|---|
| `Description` | `title` | Kategorie per Stichwort: HU, TÜV, MOT, AU, Inspection → `legal_inspection`; sonst `other` |
| `Metric` (`Date`, `Odometer`, `Both`) | Auslöser Zeit / Distanz / beide | bei „nur Kilometer“ wird das (zufällige) Datum verworfen, und umgekehrt |
| `IsRecurring = false` | `schedule_mode = once`, `due_date_once` / `due_odometer_once` | |
| `IsRecurring = true`, `FixedIntervals = false` | `from_last_completion` | |
| `IsRecurring = true`, `FixedIntervals = true` | `fixed_grid` | |
| Intervall-Enums bzw. Custom-Werte | `interval_months` / `interval_days` / `interval_distance` | Distanz-Enums sind einheitenlos und werden in der Distanzeinheit des Fahrzeugs gelesen (Phase 1 T-12) |
| `Date`, `Mileage` (aktuelle Fälligkeit) | `anchor_date = Date − Intervall`, `anchor_odometer = Mileage − Intervall` | Damit ergibt MA-02/MA-03 genau die importierte Fälligkeit. Bei Monatsintervallen gilt die Monatsende-Regel; entsteht dadurch ein anderes Datum, wird `interval_days` statt Monaten gesetzt und der Befund `ANCHOR_ADJUSTED` erzeugt |
| `UseCustomThresholds`, `CustomThresholds` | `thresholds` | Distanzschwellen in der Fahrzeugeinheit |
| `Notes`, `Tags` | `note`, `tags` | |

Die Dringlichkeit wird nicht übernommen; sie wird berechnet (MA-04). Eine Fälligkeit, die in LubeLogger „überfällig“ war, ist es in Vectra auch.

### 5.7 Notizen und zurückgestellte Funktionen
| Quelle | Ziel |
|---|---|
| `notes` | Notiz: `Description` → `title`, `NoteText` → `body`, `Pinned` → `pinned` |
| `supplyrecords` (auch Werkstattlager mit `VehicleId = 0`) | Notiz je Fahrzeug bzw. beim importierenden Konto unter „Werkstattlager“ als eine Notiz mit Tabelle, dazu JSON-Anhang (NO-03) |
| `planrecords`, `planrecordtemplates` | Notiz je Plan (Status, Priorität, Kosten), JSON-Anhang. Kosten werden **nicht** als Kosten gebucht, weil erledigte Pläne bereits Serviceeinträge erzeugt haben (MG-11) |
| `inspectionrecords`, `inspectionrecordtemplates` | Notiz je Inspektion mit Ergebnissen, JSON-Anhang. Kosten werden nicht gebucht, weil LubeLogger eine Service-Kopie erzeugt hat (MG-11); Befund `INSPECTION_COST_VIA_SERVICE` |
| `equipmentrecords` | Notiz „Ausstattung“ mit Liste und Montagestatus |
| `extrafields` (Vorlagen) | Fahrzeug-Vorlage → Zusatzfeld-Definitionen; andere Vorlagen → nicht übernommen (info) |

### 5.8 Konten und Rechte
| Quelle | Ziel | Regel |
|---|---|---|
| `userrecords` | Konto `status = invited` | `EmailAddress` Pflicht; fehlt sie oder ist sie doppelt → Befund `ACCOUNT_EMAIL_MISSING` bzw. `ACCOUNT_EMAIL_DUPLICATE`, Zuordnung im Parameterdialog. Existiert die E-Mail in Vectra bereits, wird das vorhandene Konto verwendet |
| `IsAdmin` | `is_admin` | nur wenn im Parameterdialog bestätigt (Default: nein, weil Vectra-Admins Installationsrechte bekommen) |
| `Password`, `apikeyrecords`, `tokenrecords` | – | nicht übernommen (E-6), gezählt im Bericht |
| `useraccessrecords` | Mitgliedschaft `owner` | E-4 |
| `userhouseholdrecords` | für jedes Fahrzeug, auf das der Parent Zugriff hat: `viewer` (nur View) bzw. `editor` (Edit oder Delete) | Bei mehreren Wegen gilt die höchste Rolle |
| Root | Konto aus Parameter `root_account_email`; `is_admin = true`; Eigentümer aller Fahrzeuge nur, wenn `root_gets_ownership` | |
| `userconfigrecords` | Nutzereinstellungen: Einheiten → `display_units`, Sprache → `de`/`en` (andere → `en`) | übrige Einstellungen entfallen |

Hat ein Fahrzeug nach diesen Regeln keinen Eigentümer (z. B. nur Root hatte Zugriff und `root_gets_ownership = false`), wird das importierende Konto Eigentümer (I-ID-1), mit Befund `OWNER_ASSIGNED_TO_IMPORTER`.

### 5.9 Dateien (MG-10)
| Quelle (`Files[].Location`) | Ziel |
|---|---|
| `/documents/<name>` bzw. `/images/<name>` | Datei aus dem ZIP übernehmen (ADR-017), SHA-256 berechnen, `upload_time_origin = import`, `original_name` = `Files[].Name`; Verknüpfung mit Rolle je Zieltyp (Tanken `receipt`, Service `invoice`, Kosten `receipt`, Messpunkt `photo`, Notiz `attachment`) |
| gleiche Datei mehrfach referenziert | eine Datei, mehrere Verknüpfungen (DO-02) |
| externe URL (`http…`) | Text „Externer Link: <URL>“ in der Notiz des Zieldatensatzes. Vectra kennt im MVP keine Link-Dokumente; ADR-027 Punkt 6 wird entsprechend präzisiert |
| `::<Typ>:<Id>` | nicht übernommen, nur für §5.2 ausgewertet |
| Datei fehlt im ZIP | Befund `FILE_MISSING` (warning) |
| Datei nicht referenziert | nur mit Parameter `import_unreferenced_files`; dann als Dokument `other` am Fahrzeug, wenn der Ordner das zuordnen lässt, sonst Befund |
| Inhaltstyp nicht erlaubt (ADR-017) | nicht übernommen, Befund `FILE_TYPE_REJECTED` |

## 6. Umrechnungen

### 6.1 Datum (MG-2, MG-3)
LiteDB speichert fachliche Datumswerte als UTC-Zeitpunkte, die aus lokaler Mitternacht des Quellservers entstanden sind. Umrechnung: UTC-Zeitpunkt → lokale Zeit in der **Quell-Zeitzone** → Kalenderdatum → `occurred_at` = 12:00 lokal an diesem Datum, `time_zone` = Quell-Zeitzone, `time_precision = date_only` (ADR-008). Liegt die lokale Uhrzeit nach der Umrechnung nicht auf 00:00, entsteht der Befund `DATE_NOT_MIDNIGHT` (Hinweis auf eine falsche Quell-Zeitzone). Die Analyse zählt diese Fälle je Kandidaten-Zeitzone und schlägt die Zone mit den wenigsten Befunden vor.

### 6.2 Zahlen
- Distanzen: `value = round_half_even(Mileage × Faktor)` (km → m, mi → m, h → s).
- Mengen: `Gallons × Faktor` der Fahrzeugeinheit (ADR-007, exakte Faktoren).
- Zusatzfelder bleiben Text.

### 6.3 Geld
`amount_minor = round_half_up(Cost × 10^Stellen(Währung))`. Hat der Quellwert mehr Nachkommastellen als die Währung, entsteht der Befund `AMOUNT_ROUNDED` (info) mit Alt- und Neuwert.

## 7. Idempotenz und Wiederholbarkeit

- Tabelle `import.source_map(import_source_id, source_collection, source_id, target_type, target_id)`. `import_source_id` ist der SHA-256 der Datenbankdatei plus die ID der Quellinstallation, falls vorhanden.
- Ein erneuter Import derselben Datei überspringt Datensätze, die bereits abgebildet sind. Ein abgebrochener Import wird fortgesetzt; das geht, weil jedes Fahrzeug eine Transaktion ist.
- Eine **neuere** Datei derselben Installation (anderer Hash, gleiche Installations-ID) ergänzt nur neue Datensätze. Geänderte Altdatensätze werden als Befund `SOURCE_CHANGED_AFTER_IMPORT` gemeldet und nicht überschrieben.
- Importierte Fahrzeuge lassen sich als Ganzes zurücknehmen (Soft-Delete, ADR-011), solange keine neuen Einträge in Vectra hinzugekommen sind.

## 8. Plausibilität und Prüfbericht

Der Import prüft wie jeder andere Weg (ADR-010), bricht aber nicht ab:
- Rückläufige oder springende Kilometerstände (ODO-03) werden als `confirmed_anomaly` mit Begründung „Import“ übernommen (ADR-027). Befund `ODO_BACKWARDS` bzw. `ODO_JUMP` mit Datum und Werten.
- Zeitpunkt in der Zukunft (ODO-02) → Datensatz nicht übernommen, Befund `DATE_IN_FUTURE` (error).
- Tankmenge über Tankinhalt (FU-08) entfällt, weil der Tankinhalt unbekannt ist.

### 8.1 Befundkatalog

| Code | Schwere | Bedeutung | Handlung |
|---|---|---|---|
| `UNIT_AMBIGUOUS` | error (vor Import) | Einheiten nicht eindeutig | Parameter setzen |
| `DATE_NOT_MIDNIGHT` | warning | Quell-Zeitzone vermutlich falsch | Zeitzone prüfen |
| `DATE_UNPARSEABLE` | warning | Datums-String mit gewählter Kultur nicht lesbar | Kultur prüfen oder Feld nachtragen |
| `DATE_IN_FUTURE` | error | Datensatz liegt in der Zukunft | nicht übernommen |
| `ODO_BACKWARDS`, `ODO_JUMP` | warning | Anomalie übernommen und markiert | nach Import prüfen/korrigieren |
| `ODO_ZERO` | info | Kilometereintrag mit 0 übersprungen | – |
| `ODOMETER_ADJUSTMENT` | warning | Tachokorrektur war aktiv | ggf. Zählerabschnitt anlegen |
| `AUTO_ODOMETER_MERGED` / `_MISMATCH` | info / warning | automatischer Kilometereintrag | – / prüfen |
| `FUEL_QUANTITY_INVALID` | error | Menge ≤ 0 | nicht übernommen |
| `SOC_DEFAULT_VALUES` | info | Ladezustände vermutlich nicht erfasst | – |
| `FUEL_TYPE_CONFLICT` | warning | Diesel- und Elektro-Kennzeichen gleichzeitig | Energieträger bestätigen |
| `COST_ZERO_AMBIGUOUS` | info | 0 € = kostenlos oder unbekannt | ggf. „Kosten unbekannt“ setzen |
| `AMOUNT_ROUNDED` | info | Betrag gerundet | – |
| `RECURRING_INTERVAL_INVALID` | warning | Intervall 0 | Plan manuell anlegen |
| `TAX_AUTOGENERATED_POSSIBLE` | info | möglicherweise automatisch fortgeschrieben | prüfen |
| `ANCHOR_ADJUSTED` | info | Wartungsintervall auf Tage umgestellt | – |
| `INSPECTION_COST_VIA_SERVICE` | info | Kosten nur über Serviceeintrag | – |
| `VEHICLE_ID_MISMATCH` | warning | Spalten- und JSON-Fahrzeug-ID weichen ab (MG-15; nur bei aus PostgreSQL exportierten Dateien sichtbar, wenn die Exportfunktion die Abweichung überträgt) | JSON-Wert maßgeblich |
| `ORPHAN_REFERENCE` | warning | Verweis auf nicht vorhandenen Datensatz (MG-6) | Verweis entfällt |
| `FILE_MISSING`, `FILE_TYPE_REJECTED` | warning | Datei fehlt / nicht erlaubt | ggf. nachreichen |
| `ACCOUNT_EMAIL_MISSING` / `_DUPLICATE` | error (vor Import) | Konto nicht zuordenbar | Parameter |
| `OWNER_ASSIGNED_TO_IMPORTER` | info | Fahrzeug ohne Eigentümer | – |
| `SECRETS_NOT_IMPORTED` | info | Anzahl nicht übernommener Passwörter, Keys, Tokens | – |
| `SOURCE_CHANGED_AFTER_IMPORT` | warning | Quelldatensatz geändert | manuell nachziehen |
| `EXTRA_FIELD_UNMAPPED` | info | Zusatzfeld in Notiz übernommen | – |

### 8.2 Kennzahlen-Hinweis (ADR-031)
Der Bericht enthält einen festen Abschnitt „Warum Zahlen abweichen können“: Durchschnittsverbrauch (BR-011), Monatswerte (BR-012), Distanzen und Kosten je km (BR-038, BR-060), Fälligkeiten am Fälligkeitstag (BR-026). Zu jedem Punkt gibt es ein Beispiel aus `20-regelkatalog-soll.md`.

## 9. Lesen der LiteDB-Datei

Vectra ist in Go geschrieben, LiteDB ist ein .NET-Format. Optionen, die in Spike S-4 bewertet werden (`40-spikes.md`):
- **A:** eigener, schreibgeschützter Leser für das Dateiformat in Go.
- **B:** kleines Hilfswerkzeug auf Basis der LiteDB-Bibliothek (MIT-Lizenz, nicht LubeLogger), das die Datei nach JSON exportiert. Es läuft als optionaler Container nur während der Analyse.
- **C:** Nutzer exportieren vorher mit LubeLogger selbst (nur CSV je Typ möglich, verlustbehaftet; Phase 1 08 §4) → ungeeignet.

**Entscheidung nach Spike S-4: Option A.** Ein Go-Leser mit ca. 250 Zeilen hat eine echte LubeLogger-v1.7.3-Datei exakt gelesen, auch Dezimalwerte, Unicode, mehrseitige Dokumente und Löschungen. Voraussetzung ist eine leere Log-Datei `cartracker-log.db`, also ein regulär beendetes LubeLogger. Sonst lehnt die Analyse mit Hinweis ab. Enum-Werte stehen als Namen in der Datei (Erratum zu MG-13); das Mapping verwendet Namen.

## 10. CSV-Import (allgemein)

Für Daten aus anderen Quellen (z. B. Tankapps) gibt es einen CSV-Import für Tankvorgänge, Kilometerstände, Serviceeinträge und Kosten. Er korrigiert die Schwächen aus Phase 1 (BR-052):
- Spaltenzuordnung im Dialog, mit Vorschlägen über Spaltennamen (eigene Alias-Liste).
- Datumsformat, Dezimaltrennzeichen, Einheiten und Währung werden **ausdrücklich** gewählt; es gibt kein Raten anhand der Serverkultur.
- Zeilen ohne Datum oder mit ungültigen Werten sind Fehler mit Zeilennummer. Sie werden nicht übernommen und nicht als „importiert“ gezählt.
- Duplikaterkennung: gleiches Fahrzeug, gleiches Datum, gleicher Stand, gleiche Menge bzw. gleicher Betrag → Befund, Übernahme nur nach Bestätigung.
- Vorschau mit Plausibilitätsbefunden vor dem Schreiben, danach dasselbe Import-Modell (Job, Transaktion, Prüfbericht).

## 11. Export aus Vectra

Gegenstück und Datenportabilität (korrigiert Phase 1 D-06):
- **JSON-Export je Konto** (MVP): alle Fahrzeuge mit Rolle `owner`, alle Einträge mit kanonischen Werten **und** Originaleingaben, Dateien als ZIP. Das Format ist versioniert und wird in der API-Spezifikation beschrieben (Ergänzung zu AP-6 bei der Umsetzung).
- **CSV je Modul** (nach MVP): exportiert gespeicherte Werte mit Einheitenspalte, keine berechneten Werte, Zahlen im gewählten Format.

## 12. Tests

- **Golden-File-Tests:** synthetische LubeLogger-Datenbanken, erzeugt mit einer lokal laufenden LubeLogger-Instanz (wie in Phase 1). Sie decken alle Collections, beide Einheitenmodi, UK-Gallonen, Elektrofahrzeuge, Motorstunden, Haushalte, Tachokorrektur, Automatik-Duplikate und fehlende Dateien ab. Erwartet werden: Zielobjekte (als JSON-Schnappschuss) und Prüfbericht.
- **Zeitzonen-Matrix:** dieselbe Datei mit Quell-Zeitzone UTC, `Europe/Berlin` und `Pacific/Honolulu`; nur die richtige ergibt 0 × `DATE_NOT_MIDNIGHT`.
- **Idempotenz:** Import zweimal → keine zusätzlichen Datensätze; Abbruch nach Fahrzeug 2 von 3 → Fortsetzung vollständig.
- **Anonymisierte Pilotdaten** (Q-18): Sobald Pilotnutzer Daten bereitstellen, werden sie als weitere Golden Files aufgenommen.

## 13. Auswirkungen auf offene Fragen und ADRs

- **Q-10** (Abbildung der Record-Typen auf Zielmodule) ist mit §5 beantwortet.
- **ADR-027 Punkt 6:** Externe Links werden als Text in der Notiz übernommen, nicht als Link-Dokumente (Documents hat im MVP keinen Link-Typ). ADR-027 ist entsprechend präzisiert.
