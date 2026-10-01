# ADR-032 – MCP-Server, Claude als Chat-Anbieter und Websuche für Herstellerangaben

- **Status:** akzeptiert · **Datum:** 2026-10-01 · **Ergänzt:** ADR-024, ADR-026

## Kontext
Der Auftraggeber möchte mit Claude (Sonnet 4.5) über Fahrzeugdaten sprechen, per Sprache (z. B. Wispr Flow) Fahrten und Kilometerstände eintragen und für Modelle ohne hinterlegtes Wartungsbuch (z. B. Skoda Octavia) die Herstellerintervalle nachladen. Dazu soll Claude über einen MCP-Server mit Vectra verbunden werden.

## Entscheidung
- **Ein Werkzeugkasten, zwei Zugänge** (`backend/internal/assistant`): Die Werkzeuge aus ADR-026 (lesen: Fahrzeuge, Überblick, Fahrten, Wartung, Service, Kosten, Wartungsbücher; schreiben: Kilometerstand, Fahrt starten/beenden/nachtragen, Kosten, Service, Wartung erledigen, Wartungsbuch übernehmen, Wartungsplan anlegen) rufen die öffentliche API **intern** mit den Rechten des Nutzers auf. Rechte, Validierung, Plausibilität (ADR-010) und Audit sind dadurch identisch mit einem direkten API-Aufruf.
- **MCP-Server** `POST /api/v1/mcp` (Model Context Protocol, Transport Streamable HTTP, zustandslos, JSON-Antworten, Protokollversionen 2025-06-18/2025-03-26/2024-11-05). Anmeldung mit **persönlichem API-Token** (`Authorization: Bearer vct_…`, nur SHA-256 gespeichert, Ablauf ≤ 1 Jahr, Scopes `vehicles:read`/`entries:write`, optional auf Fahrzeuge beschränkt). Bearer-Tokens gelten vorerst **nur** für `/mcp`. Externe Clients (Claude Desktop, Claude Code) lassen jeden Werkzeugaufruf vom Nutzer freigeben; deshalb führen schreibende Werkzeuge über MCP direkt aus. Plausibilitätsbefunde gehen als Werkzeugfehler zurück; bestätigen lassen sie sich nur mit `confirm_anomalies` und einer Begründung des Nutzers.
- **Chat in Web und App** über die Assistant-Endpunkte der Spezifikation (Unterhaltungen, Nachrichten als Server-Sent Events, Vorschläge bestätigen/verwerfen). Anbieter `anthropic` mit dem offiziellen Go-SDK, Modell per `VECTRA_ASSISTANT_MODEL` (Vorgabe in `.env.example`: `claude-sonnet-4-5`). Schreibende Werkzeuge erzeugen wie in ADR-026 **Vorschläge**, die der Nutzer mit einem Klick bestätigt; bei Befunden fragt die Oberfläche nach einer Begründung.
- **Abweichung von ADR-026 („kein Internetzugriff“):** Der Chat darf die serverseitige **Websuche** von Anthropic nutzen (`web_search_20250305`, höchstens 5 Suchen je Anfrage, abschaltbar mit `VECTRA_ASSISTANT_WEB_SEARCH=false`), ausschließlich für öffentliche Herstellerangaben wie Wartungsintervalle. Der Systemprompt verbietet persönliche Daten (Kennzeichen, FIN, Namen, Orte) in Suchanfragen. Ergebnisse werden nie direkt gespeichert, sondern als Vorschlag `create_maintenance_plan` mit Quellenangabe und dem Hinweis „Richtwerte, mit dem Serviceheft abgleichen“.
- **Spracheingabe:** Diktier-Apps wie Wispr Flow schreiben in jedes Textfeld; zusätzlich bieten Web (Web Speech API, wo der Browser sie hat) und Android (Spracherkennung des Systems) eine Mikrofontaste. Vectra speichert keine Audiodaten.
- **Datenschutz (ADR-024):** Zustimmung je Nutzer vor der ersten Anfrage an Anthropic; dauerhafte Kennzeichnung „Antworten erzeugt von Anthropic (Claude)“; Metadaten-Protokoll je Anfrage (`assistant.request_log`, ohne Inhalte); Unterhaltungen werden nach 30 Tagen ohne Aktivität gelöscht (`VECTRA_ASSISTANT_RETENTION_DAYS`); Tageslimit je Nutzer (`VECTRA_ASSISTANT_DAILY_LIMIT`, Default 200 Modellanfragen).

## Konsequenzen
- (+) Claude Desktop/Code und der eingebaute Chat nutzen dieselben, getesteten Werkzeuge.
- (+) Wartungspläne für beliebige Modelle ohne gepflegte Vorlage.
- (−) Websuche-Ergebnisse können falsch oder unvollständig sein; deshalb immer Vorschlag mit Quelle, nie automatisch gespeichert.
- (−) Der MCP-Server ist nur nutzbar, wenn der Client Vectra erreicht (eigene Domain/VPN). Der MCP-Connector der Claude-API selbst wird nicht verwendet, weil er eine öffentlich erreichbare Adresse voraussetzt.

## Bezug
ADR-010, ADR-015, ADR-016, ADR-024, ADR-026; `backend/internal/assistant`, `backend/internal/server/assistant_test.go`.
