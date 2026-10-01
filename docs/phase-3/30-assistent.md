# Phase 3 – Assistent mit Claude und MCP-Server

- **Status:** umgesetzt · **Datum:** 2026-10-01 · **Entscheidung:** ADR-032 (ergänzt ADR-024, ADR-026)
- **Auftrag:** Claude (Sonnet 4.5) über einen MCP-Server mit Vectra verbinden, Wartungsbücher für weitere Modelle (z. B. Skoda Octavia) nachladen, Fahrtenbuch und Kilometerstände per Chat/Sprache (Wispr Flow) eintragen.

## Einrichten

```env
# deploy/.env
VECTRA_ASSISTANT_PROVIDER=anthropic
VECTRA_ASSISTANT_MODEL=claude-sonnet-4-5
ANTHROPIC_API_KEY=sk-ant-…
# optional
VECTRA_ASSISTANT_WEB_SEARCH=true      # Websuche für Herstellerangaben
VECTRA_ASSISTANT_DAILY_LIMIT=200      # Modellanfragen je Nutzer und Tag
VECTRA_ASSISTANT_RETENTION_DAYS=30    # Unterhaltungen ohne Aktivität löschen
```

Das Modell ist frei wählbar; `claude-sonnet-4-5` ist die gewünschte Vorgabe. Neuere Modelle (z. B. `claude-sonnet-5-5`) funktionieren ohne Codeänderung.

## Chat in Web und App

- Web: Seitenleiste → **Assistent**. App: Übersicht → „Mit dem Assistenten sprechen“ oder Mehr → Assistent.
- Beim ersten Mal stimmt jeder Nutzer der Übertragung an Anthropic zu (ADR-024).
- **Sprache:** Mikrofontaste (Web: Web Speech API in Chrome/Edge/Safari; App: Spracherkennung von Android). **Wispr Flow** und andere Diktier-Apps schreiben direkt in das Eingabefeld – Enter sendet.
- **Einträge:** Der Assistent speichert nie selbst. Er legt Vorschläge an („Fahrt starten bei 52.340 km · Geschäftlich · Kundentermin Müller“), die du mit **Bestätigen** speicherst oder verwirfst. Meldet Vectra einen unplausiblen Wert (ADR-010), fragt die Karte nach einer Begründung.
- Beispiele:
  - „Ich fahre jetzt los, Kilometerstand 52.340, Kundentermin bei Müller“ → Fahrt starten (Geschäftlich, Zweck)
  - „Bin angekommen, 52.398“ → laufende Fahrt beenden
  - „Gestern 38 km privat gefahren, von 52.200 bis 52.238“ → Fahrt nachtragen
  - „Parken am Flughafen 24 Euro“ → Kosten
  - „Lade die Wartungsintervalle für meinen Skoda Octavia nach“ → hinterlegtes Wartungsbuch oder Websuche nach Herstellerangaben → Vorschlag „Wartungsplan anlegen“ mit Quelle
  - „Was ist als Nächstes fällig?“ → liest die Wartungen

## MCP-Server für Claude Desktop, Claude Code & Co.

- Adresse: `https://<deine-domain>/api/v1/mcp` (Streamable HTTP, JSON).
- Token: Web → Einstellungen → **KI-Zugang (MCP)** → Token erstellen (Lesen oder Lesen + Schreiben, optional nur ein Fahrzeug, 30 Tage bis 1 Jahr). Das Token wird nur einmal angezeigt.
- Claude Code: `claude mcp add --transport http vectra https://<domain>/api/v1/mcp --header "Authorization: Bearer vct_…"`
- Andere Clients mit HTTP-Transport: `{"mcpServers":{"vectra":{"type":"http","url":"https://<domain>/api/v1/mcp","headers":{"Authorization":"Bearer vct_…"}}}}`
- Über MCP führen schreibende Werkzeuge direkt aus; der Client fragt vor jedem Aufruf nach. Unplausible Werte kommen als Werkzeugfehler zurück und lassen sich nur mit `confirm_anomalies` und einer Begründung bestätigen.
- Der Server muss für den Client erreichbar sein (eigene Domain oder VPN).

## Werkzeuge

| Lesen | Schreiben (Chat: Vorschlag, MCP: direkt) |
|---|---|
| `list_vehicles`, `get_vehicle_overview`, `list_trips`, `list_maintenance`, `list_service_entries`, `get_cost_summary`, `list_maintenance_books` | `record_odometer`, `start_trip`, `finish_trip`, `record_trip`, `add_cost`, `add_service_entry`, `complete_maintenance`, `apply_maintenance_book`, `create_maintenance_plan` |

Im Chat zusätzlich die Websuche von Anthropic (`web_search_20250305`, höchstens 5 Suchen je Anfrage).

## Prüfung

- `server/assistant_test.go`: MCP mit Token (Initialisierung, Werkzeugliste, Kilometerstand mit Befund und Bestätigung, Fahrt starten/beenden, Wartungsplan, Nur-Lese-Token, Fahrzeugbeschränkung, Widerruf, CSRF) und Chat gegen eine nachgebildete Messages-API (Zustimmung, Stream, Vorschlag, Bestätigung, Befund mit Begründung, Verlauf, fremde Vorschläge).
- Android `ApiClientTest`: Server-Sent Events (status, proposal, message, error).
- Sichtprüfung im Browser mit nachgebildeter Messages-API (Zustimmung → Diktat → Vorschlag → Gespeichert; Token erstellen und MCP-Aufruf). Ein Test gegen die echte Claude-API braucht einen API-Schlüssel und ist nicht Teil der CI.

## Offen

- OAuth für den MCP-Server (für Clients, die nur OAuth anbieten, z. B. benutzerdefinierte Konnektoren in Claude Desktop/claude.ai).
- Dokumentensuche (RAG, ADR-025) als Werkzeug `search_documents`.
- Kraftstoff- und Ölwerkzeuge, sobald die Module umgesetzt sind.
