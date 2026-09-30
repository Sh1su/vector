# 00 – Zusammenfassung Phase 1 (Reverse Engineering LubeLogger)

**Referenzstand:** `hargata/lubelog`, Commit `dd69e59b276e79b96bb459d8b1eda1501316086d` = Tag **v1.7.3** (2026-09-12). Lizenz **MIT** (Code/Logik nutzbar mit Copyright-Hinweis; Name und Logos werden nicht übernommen). Ergänzend lief das offizielle Image v1.7.3 lokal (LiteDB) zur Verifikation. Statusmarker: [VERIFIZIERT] · [ABGELEITET] · [UNKLAR].

## Überblick

- LubeLogger ist ein ASP.NET-Core-10-MVC-Monolith mit serverseitig gerenderter UI (Razor + jQuery), ~27 000 Zeilen C#, **ohne automatisierte Tests**. [VERIFIZIERT]
- **Zwei Persistenz-Backends:** eingebettete LiteDB (`data/cartracker.db`, Default) oder PostgreSQL, dort fast ausschließlich JSONB-Dokumente je Record in 22 Tabellen ohne Fremdschlüssel/Migrationen. Dateien liegen im Dateisystem unter `data/`. [VERIFIZIERT]
- **61 Ist-Features** (F-001 – F-061), u. a. Fahrzeuge, Kilometerstände, Tankvorgänge mit Verbrauch (inkl. EV), Service/Reparatur/Upgrade, Steuern/Gebühren (wiederkehrend), Reminder, Planer (Kanban), Teilelager, Inspektionen, Ausstattung, Berichte, CSV-Import/-Export, Backup, REST-API, Webhooks, Benachrichtigungen, OIDC. [VERIFIZIERT]
- **Nicht vorhanden** sind die Zielthemen Öl-Tracking, Fahrten, Wartungsdefinitionen mit Herstellerquelle, typisierte Dokumente, Beweisfotos, Audit-Log und native/offline Clients. [VERIFIZIERT]
- Erfasst: 35 Controller-Dateien mit 329 Aktionen, 20 persistierte Entitäten, alle öffentlichen API-Endpunkte (Checklisten in `02`, Anhang A–D), **60 Geschäftsregeln** mit Quelle und Regressionstest (`04`).

## Wichtigste fachliche Erkenntnisse

1. **Verbrauch** wird nur bei Vollbetankung mit Kilometerstand berechnet; Teilbetankungen werden akkumuliert, „Missed Fuel-Up“ setzt zurück; Durchschnitt gewichtet über ausgewählte Einträge; EV-Verbrauch aus Ladezuständen (BR-001–BR-011). [VERIFIZIERT, teils ausgeführt]
2. **Einheiten werden nicht gespeichert.** Distanz, Menge und Währung sind reine Interpretation von Nutzer-/Servereinstellungen; im Modus „UK MPG“ sind Mengen Liter, Distanzen Meilen (BR-059). [VERIFIZIERT]
3. **Kilometerstand** hat keine Plausibilitätsprüfung; „aktueller Stand“ = Maximum über fünf Record-Typen; Tachokorrektur existiert (Multiplikator/Offset) mit drei unterschiedlichen Rundungen und ohne Gültigkeitszeitraum (BR-015–BR-022). [VERIFIZIERT, ausgeführt]
4. **Reminder** unterstützen Datum/km/beides (was zuerst eintritt), wiederkehrende Intervalle und Schwellen; Erledigung per „Pushback“ ohne gespeicherte Verknüpfung (BR-026–BR-033). [VERIFIZIERT]
5. **Rechte:** Kollaborator = Vollzugriff ohne Eigentümerbegriff; Haushalte vererben Zugriff mit View/Edit/Delete; Root-User ohne DB-Datensatz; Authentifizierung ist standardmäßig aus (`06`). [VERIFIZIERT, ausgeführt]
6. **Datumswerte** in LiteDB verschieben sich bei Zeitzonenwechsel des Servers um einen Tag. [VERIFIZIERT, ausgeführt]

## Größte Risiken

| Risiko | Beleg |
|---|---|
| **Sicherheitslücken der Altanwendung** (Überschreiben fremder Datensätze, Dateizugriff ohne Rechteprüfung inkl. Backups, fehlende Upload-Prüfung, ungesalzene Hashes, unsichere Sitzungscookies) – dürfen nicht übernommen werden; Altdaten können durch sie bereits verfälscht sein | `09` R-01–R-10 (mehrere ausgeführt) |
| **Migration**: Einheiten, Quell-Zeitzone und Kultur (String-Daten) sind aus den Daten allein nicht bestimmbar | `08` MG-1–MG-3 |
| **Fehlerhafte Ist-Regeln** (Reminder-Schwellen-Leck, widersprüchliche Distanz-/Durchschnittsdefinitionen, verlustbehafteter CSV-Round-Trip bei UK/EV) | `09` D-02, D-06, D-12 |
| **Scope**: viele Nischenfunktionen ohne klare Priorität | `02`, Q-05 |
| Backups bei PostgreSQL enthalten keine Fachdaten | `08` §5 |

## Wichtigste offene Fragen (vor Phase 2 zu klären)

- **Q-01** Einheitenmodell (Empfehlung: SI kanonisch + Originalwert/-einheit)
- **Q-02** Zeitmodell (Empfehlung: `timestamptz`, Import mit abgefragter Quell-Zeitzone)
- **Q-03** Kilometer-Plausibilität und Tachotausch (Empfehlung: Warnen + Bestätigen, Tachotausch als Ereignis)
- **Q-04** Rechte-Mapping der Freigaben (Empfehlung: Kollaboratoren → Eigentümer, Haushalt → Leser/Bearbeiter)
- **Q-05** MVP-Umfang der Ist-Features (Empfehlung: Kern übernehmen, Nischen zurückstellen, Daten beim Import erhalten)
- **Q-06** Migrationsmodus/-umfang (Empfehlung: Offline-Import aus LiteDB-Datei, keine Passwort-Übernahme)
- **Q-08** Ist-Fehler korrigieren statt nachbilden (per ADR)
- **Q-09** Kosten/Währung, **Q-12** Meldung der Sicherheitsbefunde an die Maintainer, **Q-17** Produktname

## Dokumente

`01` Repository · `02` Feature-Inventar + Vollständigkeitsanhänge · `03` Domain/Datenmodell + ER-Diagramm · `04` Geschäftsregeln · `05` bestehende API · `06` Auth/Berechtigungen · `07` Dateien · `08` Import/Export/Migration · `09` Risiken · `10` offene Fragen · `11` Vorschlag Phase 2 (31 ADRs, Module, Reihenfolge, 36–54 PT).
