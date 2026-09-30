# Phase 2 – Entscheidungslog (Workshop 30.09.2026)

Grundlage: offene Fragen aus `docs/phase-1/10-offene-fragen.md`. Alle Entscheidungen hat der Auftraggeber getroffen, jeweils zugunsten der vorgeschlagenen Empfehlung. Die ADRs unter `docs/adr/` setzen sie um.

| ID | Frage | Entscheidung | Umgesetzt in |
|---|---|---|---|
| E-0 | Umgang mit LubeLogger | **Rebrand als Clean-Room-Neuentwicklung. Kein Code, kein Text, kein Asset von LubeLogger.** Nur fachliches Verhalten über eigene Spezifikationen. | ADR-000 |
| E-9 | Produktname (Q-17) | **Vectra** (laut Markengrundlagen „Vectra Brand“) | ADR-000 |
| E-1 | Einheitenmodell (Q-01) | Intern SI-Einheiten (km, l, kWh); zusätzlich Originalwert und Originaleinheit jeder Eingabe | ADR-007 |
| E-2 | Zeitmodell (Q-02) | Zeitstempel mit Zeitzone; Altdaten mit Kennzeichen „Uhrzeit unbekannt“; Import fragt die Zeitzone des Quellservers ab | ADR-008 |
| E-3 | Kilometer-Plausibilität (Q-03) | Unplausible Werte → Warnung mit Pflicht-Bestätigung und Audit-Vermerk; Tachotausch als eigenes Ereignis | ADR-009, ADR-010 |
| E-4 | Rechte bei Migration (Q-04) | Alle LubeLogger-Kollaboratoren eines Fahrzeugs werden Eigentümer; Haushalt „View“ → Leser, „Edit“/„Delete“ → Bearbeiter | ADR-016, ADR-027 |
| E-5 | MVP-Umfang (Q-05) | Nur Kernmodule. Nicht im MVP: Aufgabenplaner, Teilelager, Inspektionen, Ausstattung, Kiosk, Widgets, Themes, Imagemap, Sticker. Deren Altdaten bleiben beim Import als Notiz bzw. Dokument erhalten. | Modulzuschnitt `01-arbeitsplan.md`, ADR-027 |
| E-6 | Migration (Q-06) | Offline-Import aus LubeLogger-Datenbankdatei + Dateiordner mit Prüfbericht; **keine** Übernahme von Passwörtern und API-Keys; Nutzer werden per E-Mail eingeladen | ADR-027 |
| E-7 | Fehler der Altanwendung (Q-08) | Fachlich korrekt neu spezifizieren; jede Abweichung von LubeLogger wird im Regelkatalog dokumentiert | ADR-031 |
| E-8 | Kosten/Währung (Q-09) | Währung je Betrag (ISO 4217), Default je Nutzer; Service-Kosten getrennt nach Teile, Arbeit, Sonstiges; Altkosten als Gesamtsumme | ADR-029 |
| E-10 | Meldung an LubeLogger (Q-12) | Ja; neutraler Meldetext wird vorbereitet (`90-entwurf-meldung-lubelogger.md`), Versand durch den Auftraggeber | – |
| E-11 | Anmeldung (Q-15) | OIDC **und** lokale Konten | ADR-015 |
| E-12 | Push (Q-14) | UnifiedPush als Standard (selbst hostbar), FCM optional | ADR-020 |
| E-13 | Ölstand-Skala (Q-11) | Intern 0–100 % zwischen Min und Max am Peilstab; Eingabe auch in Stufen (Min, ¼, ½, ¾, Max); optional Foto | Domänenmodell Oil (Arbeitspaket AP-4) |

## Noch offen (blockieren den Start von Phase 2 nicht)

| ID | Frage | Geplant |
|---|---|---|
| Q-07 | Kompatibilitätsschicht zur LubeLogger-API | nicht MVP; Bedarf nach Pilotphase erheben |
| Q-10 | Genaue Abbildung der Record-Typen auf Zielmodule | **erledigt** in `30-migrationskonzept.md` §5 (AP-7) |
| Q-13 | Fahrtenbuch mit Unveränderbarkeit | Datenmodell Fahrten wird append-only mit Korrekturbuchungen entworfen (AP-4) |
| Q-16 | Übersetzungen | durch E-0 entschieden: Texte werden neu verfasst |
| Q-18 | Technische Restunsicherheiten | Spikes in AP-9; anonymisierte Beispielexporte von Pilotnutzern erbeten |
| E-14 (Auftrag 5.3) | Ressourcenbudgets als Abnahmekriterien | Bestätigung ausstehend; bis dahin gelten die Zielwerte aus dem Auftrag |
