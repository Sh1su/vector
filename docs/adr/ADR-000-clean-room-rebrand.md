# ADR-000 – Clean-Room-Neuentwicklung als Rebrand (keine Übernahme von LubeLogger-Code)

- **Status:** akzeptiert
- **Datum:** 2026-09-30
- **Entscheidung durch:** Auftraggeber (Workshop Phase 2)
- **Gilt für:** alle Phasen, alle Repositories und Artefakte von Vectra

## Kontext

Vectra ist ein Rebrand und eine eigenständige Neuentwicklung. LubeLogger (MIT-Lizenz) diente in Phase 1 ausschließlich als **fachliche Referenz**. Der Auftraggeber hat entschieden, dass **keine Code-Snippets von LubeLogger** verwendet werden. Die MIT-Lizenz würde eine Übernahme mit Copyright-Hinweis zwar erlauben, sie ist aber ausdrücklich nicht gewünscht.

## Entscheidung

1. **Kein Code, keine Texte, keine Assets.** In Vectra wird nichts aus dem LubeLogger-Repository kopiert, übersetzt oder umgeschrieben. Das betrifft Quellcode (C#, JavaScript, Razor, SQL), CSS, UI-Texte und Übersetzungsschlüssel (`en_US.json`), E-Mail-Vorlagen, API-Dokumentation (`api.json`), Logos, Icons, Screenshots, Demo-Daten sowie Klassen-, Methoden- und Variablennamen als Vorlage.
2. **Übernommen wird nur fachliches Verhalten**, und zwar ausschließlich über die Spezifikationen in `docs/phase-1` und `docs/phase-2`. Diese beschreiben Regeln in eigenen Worten und mathematischer Notation. Dort zitierte Code-Ausdrücke dienen nur als Beleg und werden nicht in die Implementierung übernommen.
3. **Trennung von Analyse und Umsetzung.** Ab Phase 3 arbeitet die Implementierung nur mit den Vectra-Spezifikationen (Regelkatalog, OpenAPI, Domänenmodelle). Das LubeLogger-Repository ist dabei keine Arbeitsgrundlage. Fehlt einer Spezifikation etwas, wird sie ergänzt, und zwar in Form von Verhalten, nicht von Code.
4. **Eigene Architektur und eigene Begriffe.** Datenmodell, API, UI-Struktur und Benennung werden neu entworfen, siehe die folgenden ADRs. Übereinstimmungen dürfen sich nur aus der Fachdomäne ergeben (z. B. „Kilometerstand“, „Tankvorgang“).
5. **Ausnahme Interoperabilität.** Für den Import von LubeLogger-Daten (ADR-027) muss Vectra deren **Datenformat lesen**, also Tabellennamen, Feldnamen, CSV-Spaltennamen und Enum-Werte. Diese Fakten dürfen im Import-Modul als Mapping-Konstanten stehen. Der Import-Code wird trotzdem neu geschrieben und ist auf dieses Modul beschränkt.
6. **Name und Marke.** Das Produkt heißt **Vectra**. „LubeLogger“ erscheint nur beschreibend, z. B. im Menüpunkt „Import aus LubeLogger“, niemals als Produkt- oder Markenbestandteil.

## Umsetzung und Kontrolle

- Pull-Request-Vorlage mit Pflicht-Checkbox: „Enthält keinen Code, Text oder Asset aus LubeLogger.“
- Stichprobenprüfung vor jedem Release: Abgleich neuer Dateien gegen das LubeLogger-Repository mit einem Ähnlichkeitswerkzeug, z. B. einem Token-Vergleich. Ausgenommen sind das Import-Modul (nur Mapping-Konstanten) und generierter Code.
- Abhängigkeiten nur aus eigenen Entscheidungen (ADR-003 ff.), nicht deshalb, weil LubeLogger sie nutzt.
- Übersetzungen und UI-Texte werden neu verfasst (Vectra-Tonalität laut Markengrundlagen).

## Konsequenzen

- (+) Keine lizenzrechtliche Abhängigkeit, eigenständige Marke, freie Wahl der Architektur.
- (+) Keine Übernahme der in Phase 1 gefundenen Sicherheits- und Datenfehler durch Copy & Paste.
- (−) Fachliche Randfälle müssen vollständig spezifiziert sein, weil nicht „im Zweifel nachgesehen“ wird. Deshalb bekommt der Soll-Regelkatalog in Phase 2 ein hohes Gewicht.
- (−) Die Kompatibilität zur LubeLogger-API entfällt standardmäßig (siehe Q-07, nicht MVP).

## Bezug

Auftrag Abschnitt 1 und 2 (Lizenz beachten, eigener Name), `docs/phase-1/01-repository-ueberblick.md` §2, Workshop-Entscheidungen `docs/phase-2/00-entscheidungslog.md`.
