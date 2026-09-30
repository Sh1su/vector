# 10 – Offene Fragen

Jede Frage enthält Auswirkung, Optionen mit Vor-/Nachteilen und eine Empfehlung. Fragen, die **vor Phase 2** entschieden werden müssen, sind mit ★ markiert. Alle Empfehlungen sind [ABGELEITET]; zugrunde liegende Befunde sind in den verlinkten Dokumenten belegt.

## ★ Q-01 – Einheitenmodell
**Hintergrund:** Ist speichert Zahlen ohne Einheit; Interpretation per Nutzereinstellung (BR-059, D-04).
**Auswirkung:** Datenmodell aller Mess- und Mengenwerte, API, Migration.
| Option | Vorteile | Nachteile |
|---|---|---|
| A: Kanonische SI-Speicherung (m bzw. km, Liter, kWh), Umrechnung nur an den Rändern | eindeutige Berechnungen, einfache Aggregation | Rundungsfragen bei Rückanzeige in Meilen/Gallonen; Migration braucht Einheit je Fahrzeug |
| B: Einheit pro Fahrzeug (Distanz-, Volumeneinheit am Fahrzeug) | nah an Nutzererwartung, keine Rundung | Aggregation über Fahrzeuge erfordert Umrechnung |
| C: Einheit pro Wert | maximal flexibel, Originalwert bleibt erhalten | Komplexität in jeder Berechnung |
**Empfehlung:** A für die Speicherung **plus** Speicherung von Originalwert und Originaleinheit für manuell erfasste Messwerte (Beweisführung), Fahrzeug trägt Anzeige-Defaults.

## ★ Q-02 – Datum/Zeit-Semantik
**Hintergrund:** Ist speichert nur Datum ohne Zeitzone; LiteDB-Verschiebung bei TZ-Wechsel (M-6, D-03).
| Option | Vorteile | Nachteile |
|---|---|---|
| A: `timestamptz` für Ereignisse + optional lokales Datum | exakte Reihenfolge am selben Tag, Monotonieprüfung möglich | Migration muss Uhrzeit ergänzen (z. B. 12:00 lokal) |
| B: nur Kalenderdatum (`date`) wie Ist | einfache Migration | Reihenfolge mehrerer Einträge pro Tag unklar; widerspricht Auftrag 6.4 (Zeitstempel) |
**Empfehlung:** A; importierte Altwerte als Datum mit Kennzeichen „Uhrzeit unbekannt“. Für den Import ist die Zeitzone des Quellservers **abzufragen**.

## ★ Q-03 – Kilometerstand-Plausibilität und Tachotausch
**Hintergrund:** Ist ohne Prüfung (BR-021); Tachokorrektur ohne Gültigkeitszeitraum (BR-022).
| Option | Vorteile | Nachteile |
|---|---|---|
| A: harte Ablehnung nicht-monotoner Werte | maximale Datenqualität | blockiert Nacherfassung alter Belege, Tachotausch |
| B: Warnung mit expliziter Bestätigung + Vermerk | flexibel, nachvollziehbar | Nutzer können bestätigen „wegklicken“ |
| C: Tachotausch als eigenes Ereignis (neuer Zähler-Abschnitt mit Offset) | fachlich korrekt, Gesamtlaufleistung berechenbar | zusätzlicher Modellaufwand |
**Empfehlung:** B + C. Rückdatierte Einträge werden gegen Nachbarn im Zeitverlauf geprüft, nicht gegen das Maximum.

## ★ Q-04 – Rechtemodell und Migration der Freigaben
**Hintergrund:** Kollaborator = Vollzugriff ohne Eigentümer; Haushalte vererben Zugriff (06 §7).
| Option | Vorteile | Nachteile |
|---|---|---|
| A: Ersteller wird Eigentümer, alle weiteren Kollaboratoren Bearbeiter, Haushalt-View → Leser, Haushalt-Edit → Bearbeiter | automatisierbar | Ersteller ist im Ist **nicht gespeichert** → Heuristik (niedrigste Nutzer-ID?) nötig |
| B: Alle Kollaboratoren werden Eigentümer (Mehrfach-Eigentümer) | keine Rechte gehen verloren | weicht vom Zielmodell „ein Eigentümer“ ab |
| C: Interaktive Zuordnung beim Import | korrekt | Aufwand bei vielen Fahrzeugen |
**Empfehlung:** B als Default (keine Rechteverluste), optional C; Haushalte als explizite Fahrzeugfreigaben auflösen.

## ★ Q-05 – Umfang: welche Ist-Features werden übernommen?
**Hintergrund:** 61 Features (02), mehrere Nischen (Imagemap F-024, Sticker F-032, Kiosk F-043, Widgets F-048, Themes F-047, Sponsoren F-060, Werkstattlager F-019, Planer F-018, Inspektionen F-021, Equipment F-022).
**Auswirkung:** Aufwand, Modulzuschnitt, Migrationsumfang.
**Optionen:** A alles übernehmen · B Kern (Fahrzeuge, Kilometer, Kraftstoff, Service/Reparatur/Upgrade, Gebühren/Kosten, Reminder→Wartung, Dokumente, Notizen, Import) im MVP, Rest bewerten · C nur Zielmodule laut Auftrag.
**Empfehlung:** B; für nicht übernommene Features wird beim Import mindestens die Information als Notiz/Dokument erhalten (kein Datenverlust).

## ★ Q-06 – Migrationsumfang und -modus
| Option | Vorteile | Nachteile |
|---|---|---|
| A: einmaliger Offline-Import aus LiteDB-Datei + `data/` | einfach, reproduzierbar | Installationen mit PG müssen zuerst exportieren |
| B: Direktzugriff auf LiteDB und PG | bequemer | zwei Leser |
| C: Import über die LubeLogger-API | kein Dateizugriff nötig | API unvollständig (Inspektionen, Vorlagen, Rechte fehlen) |
**Offen außerdem:** Sollen Passwort-Hashes (Legacy-Rehash), API-Keys, offene Tokens, Benutzereinstellungen übernommen werden?
**Empfehlung:** A (PG-Nutzer nutzen die vorhandene Exportfunktion, 08 §6); Passwörter nicht übernehmen, sondern Einladung/Reset per E-Mail oder OIDC; API-Keys nicht übernehmen; Einstellungen nur Einheiten/Sprache.

## Q-07 – Kompatibilität zur LubeLogger-API
**Auswirkung:** Drittanbieter-Integrationen (z. B. Home Assistant) würden ohne Kompatibilitätsschicht brechen; Nutzung ist unbekannt (05 §6).
**Optionen:** A keine Kompatibilität · B separater, abschaltbarer Kompatibilitäts-Adapter für die häufigsten Endpunkte (Fahrzeuge, Odometer, Gas, Reminder) · C volle Kompatibilität.
**Empfehlung:** A im MVP, B als späteres Modul nach Bedarfserhebung.

## ★ Q-08 – Umgang mit fehlerhaftem Ist-Verhalten (Bug-Kompatibilität)
**Hintergrund:** z. B. Schwellen-Leck BR-029, uneinheitliche Distanzdefinitionen BR-038/BR-060, ungewichteter Monatsdurchschnitt BR-012, Rundungsvarianten BR-022.
**Optionen:** A Ist exakt nachbilden · B korrigieren und Abweichung per ADR dokumentieren.
**Empfehlung:** B; Regressionstests führen beide Werte (Ist als Referenz, Soll als Assertion), damit Migrationsvergleiche erklärbar bleiben.

## ★ Q-09 – Kostenmodell: Teile/Arbeit, Kategorien, Währung
**Hintergrund:** Ist hat eine Gesamtsumme je Record, keine Währung (M-5).
**Optionen Migration:** A Altkosten als „Gesamt, nicht aufgeschlüsselt“ · B pauschal als Teile oder Arbeit.
**Optionen Währung:** A eine Währung je Installation · B je Fahrzeug · C je Betrag.
**Empfehlung:** A für Migration; Währung je Betrag speichern (ISO 4217), Default je Installation/Nutzer; keine automatische Umrechnung im MVP.

## Q-10 – Abbildung der Ist-Record-Typen auf Zielmodule
| Ist | Vorschlag Ziel | Offen |
|---|---|---|
| Service / Repair / Upgrade | ServiceHistory mit Kategorie (Wartung, Reparatur, Upgrade) | Kategorienliste |
| Gas | Fuel | EV-Ladevorgänge als eigener Typ? |
| Odometer | Odometer mit Herkunft | Herkunft „Import“ + ursprünglicher Auto-Insert-Typ |
| Tax | Costs (Kategorie Steuer/Versicherung/Gebühr) mit Wiederholung | Wiederholungslogik übernehmen? |
| Reminder | Maintenance (Definition + Fälligkeit) | Reminder ohne Intervall = einmalige Aufgabe |
| Note | Notes/Documents | – |
| Plan, Inspection, Supply, Equipment | offen (Q-05) | – |
**Empfehlung:** wie Tabelle, Bestätigung durch Auftraggeber.

## Q-11 – Ölstand-Skala (aus Auftrag 6.6)
**Optionen:** A Prozent zwischen Min und Max · B Stufen (Min, ¼, ½, ¾, Max) · C Millimeter am Peilstab.
**Empfehlung:** A intern (0–100 %), UI erlaubt Stufen-Eingabe; Messung immer mit optionalem Foto. Im Ist gibt es keinen Anknüpfungspunkt (02, „Nicht vorhandene Funktionen“).

## Q-12 – Meldung der Sicherheitsbefunde an die LubeLogger-Maintainer
**Hintergrund:** R-01 bis R-06 fallen laut `SECURITY.md` unter meldepflichtige Lücken.
**Optionen:** A vertrauliche Meldung per E-Mail an den in `SECURITY.md` genannten Kontakt · B keine Meldung.
**Empfehlung:** A (Nutzer der Altanwendung – potenziell auch künftige Migrationskunden – sind betroffen). Entscheidung und Durchführung beim Auftraggeber.

## Q-13 – Steuerlich anerkanntes Fahrtenbuch
Kein MVP-Ziel (Auftrag 6.8). Frage: Soll das Datenmodell der Fahrten von Beginn an Unveränderbarkeit (Append-only + Korrekturbuchungen) vorsehen?
**Empfehlung:** Ja, Append-only mit Korrekturvermerk kostet wenig und hält die Option offen.

## Q-14 – Push-Kanal
**Optionen:** A FCM (verbreitet, Google-Abhängigkeit) · B UnifiedPush/ntfy (selbst hostbar) · C beides.
**Empfehlung:** C mit UnifiedPush als Default für Self-Hosting; Ist kennt nur generische HTTP-Dienste (F-051), was UnifiedPush/ntfy-kompatibel ist.

## Q-15 – Authentifizierung im MVP
**Optionen:** A nur OIDC · B OIDC + lokale Konten · C nur lokale Konten.
**Empfehlung:** B (Self-Hosting ohne IdP muss möglich sein); lokale Konten mit Argon2id; OIDC-Zuordnung über `iss`+`sub`.

## Q-16 – Wiederverwendung von Texten/Übersetzungen
`wwwroot/defaults/en_US.json` enthält alle UI-Texte; Community-Übersetzungen liegen extern (GitHub).
**Optionen:** A nicht übernehmen · B Begriffe als Glossar nutzen (MIT-Hinweis).
**Empfehlung:** A für Texte (eigene Produktsprache), Glossar nur zur Abdeckungskontrolle. Lizenzfrage der externen Übersetzungen ist [UNKLAR] (nicht im Repository).

## Q-17 – Produktname
Der neue Name ist festzulegen (Auftrag 2). Betrifft Repository, Paketnamen, Android-App-ID. **Empfehlung:** vor Phase 2 festlegen.

## Q-18 – Technische Restunsicherheiten (in Phase 2 per Spike klären)
| # | Frage | Status |
|---|---|---|
| a | Serialisiert die PG-Variante `DateTime` exakt ohne Offset und ist sie damit zeitzonenstabil (M-7)? | [UNKLAR] – nicht ausgeführt |
| b | Existieren in realen Installationen Records mit `Mileage` = 0 und Tachokorrektur gemischt (Häufigkeit)? | [UNKLAR] – nur mit Echtdaten klärbar |
| c | Welche Extra-Field-Namen werden in der Praxis genutzt (VIN, Reifen, Ölsorte …)? | [UNKLAR] – Echtdaten nötig |
| d | Nutzung der Windows-Standalone-Variante (README) und deren Datenpfade | [UNKLAR] |
**Empfehlung:** anonymisierte Beispielexporte von Pilotnutzern anfordern.
