# ADR-027 – Migration aus LubeLogger: Offline-Import mit Prüfbericht

- **Status:** akzeptiert · **Datum:** 2026-09-30 · **Entscheidung:** E-4, E-5, E-6

## Kontext
Bestehende LubeLogger-Nutzer sollen wechseln können. Einheiten, Quell-Zeitzone und Datumsformate lassen sich aus den Daten allein nicht bestimmen (Phase 1, MG-1 bis MG-15). LubeLogger bietet bei PostgreSQL-Installationen bereits einen Export in die eingebettete Datenbankdatei.

## Entscheidung
1. **Eingabe:** LubeLogger-Datenbankdatei (`cartracker.db`) plus Ordner `data/` (Bilder, Dokumente, Konfiguration). PostgreSQL-Nutzer erzeugen die Datei vorher mit der Exportfunktion ihrer LubeLogger-Installation. Ein direkter PostgreSQL-Leser ist nicht im MVP.
2. **Ablauf in drei Schritten:**
   1. *Analyse* (schreibt nichts): Einlesen, Anomalien und verwaiste Referenzen erkennen, Vorschlagswerte ableiten.
   2. *Parameter bestätigen:* Quell-Zeitzone, Kultur (Datums-/Zahlenformat), Einheiten je Fahrzeug, Währung, Rechte-Mapping und Behandlung zurückgestellter Funktionen.
   3. *Import:* je Fahrzeug in einer Transaktion, idempotent über eine Tabelle `import_source_map` (Quelle, Alt-Tabelle, Alt-ID → UUID). Jeder Datensatz erhält die Herkunft `import:lubelogger`.
3. **Prüfbericht** als HTML/PDF und im UI: rückläufige Kilometerstände (werden als `confirmed_anomaly` mit Vermerk „Import“ übernommen, ADR-010), abweichende Fahrzeug-IDs (Phase 1 D-08), fehlende Dateien, doppelte Einträge durch Automatiken (MG-11), Extra-Felder ohne Ziel.
4. **Nicht übernommen:** Passwörter, API-Keys, offene Tokens (E-6). Konten werden anhand der E-Mail angelegt, die Nutzer erhalten eine Einladung.
5. **Zurückgestellte Funktionen (E-5):** Aufgabenplaner-, Teilelager-, Inspektions- und Ausstattungsdaten sowie Notizen werden als Notizen bzw. Dokumente am Fahrzeug abgelegt (strukturierte Kopie als JSON-Anhang), damit nichts verloren geht.
6. **Dateien:** Jede referenzierte Datei wird ins Vectra-Storage kopiert, der SHA-256 wird berechnet und `upload_time_origin = import` gesetzt. Externe Links werden als Link-Dokumente übernommen, unreferenzierte Dateien nur auf Wunsch.
7. **Clean-Room (ADR-000):** Das Import-Modul enthält nur Fakten über das Fremdformat (Tabellen-, Feld- und Enum-Namen) als Mapping-Konstanten. Die Implementierung ist Neuentwicklung. Das Lesen des LiteDB-Formats in Go wird in einem Spike geprüft (AP-9).

## Konsequenzen
- (+) Nachvollziehbare, wiederholbare Migration; keine schwachen Alt-Hashes in Vectra.
- (−) Nutzer müssen Parameter bestätigen. Das ist bewusst so, weil ein falscher Default Daten verfälschen würde.

## Bezug
Phase 1 08 §7, MG-1 bis MG-15, D-03, D-08; Entscheidungen E-4 bis E-6.
