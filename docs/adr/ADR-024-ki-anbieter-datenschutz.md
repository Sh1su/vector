# ADR-024 – KI-Anbieter-Abstraktion und Datenschutz

- **Status:** akzeptiert · **Datum:** 2026-09-30

## Kontext
Der Assistent ist optional und vollständig abschaltbar; ohne ihn ist Vectra voll nutzbar (Auftrag 5.3). Sprachmodell (LLM) und Embedding-Anbieter sollen austauschbar sein, lokal (z. B. Ollama) oder in der Cloud. Der Nutzer wird informiert, wenn Daten an externe Dienste gehen (6.12). Fahrzeug- und Dokumentdaten sind personenbezogen: Standort, Kennzeichen, FIN, Rechnungen mit Namen und Adressen.

## Entscheidung
- **Modul Assistant** mit zwei Go-Interfaces, **Chat** (Nachrichten, Werkzeugaufrufe, Streaming) und **Embedding** (Text → Vektor). Implementierungen (Adapter):
  - `ollama` (lokal, Default-Vorschlag);
  - `openai_compatible` (jede Schnittstelle dieses Formats, z. B. lokal mit llama.cpp oder vLLM, oder ein Cloud-Dienst);
  - `anthropic` (Claude über das offizielle Go-SDK; nur Chat, weil Embeddings von einem anderen Adapter kommen).
  Chat- und Embedding-Anbieter werden **getrennt** konfiguriert. Eine lokale Einbettung ist also auch bei einem Cloud-Chat möglich.
- **Standard: aus.** Ein Admin aktiviert den Assistenten in den Installationseinstellungen (`assistant_enabled`) und konfiguriert die Anbieter. Nutzer müssen ihn zusätzlich für sich einschalten.
- **Kennzeichnung externer Übertragung:** Jeder Anbieter hat die Eigenschaft `external: true/false`, die der Admin bestätigt und die nicht automatisch erkannt wird. Bei `external: true`:
  - Einmalige, ausdrückliche Zustimmung je Nutzer mit Nennung des Anbieters, bevor die erste Anfrage ihn erreicht.
  - Dauerhafte Kennzeichnung in der Oberfläche („Antworten werden von <Anbieter> erzeugt“).
  - Dokumente werden nur an einen externen Embedding-Anbieter übertragen, wenn der Nutzer das je Fahrzeug erlaubt. Sonst wird lokal indexiert oder gar nicht.
- **Datensparsamkeit:** An das Modell gehen nur Daten, die ein Werkzeug für die aktuelle Frage liefert (ADR-026), keine Voll-Exporte. EXIF-Standorte, E-Mail-Adressen anderer Mitglieder und Geheimnisse sind nie Teil des Kontexts.
- **Protokoll:** Metadaten jeder Anfrage (Zeit, Nutzer, Anbieter, Modell, Token-Zahlen, aufgerufene Werkzeuge) werden gespeichert. Die Inhalte von Unterhaltungen werden je Nutzer 30 Tage aufbewahrt (konfigurierbar, auch 0) und sind vom Nutzer löschbar.
- **Robustheit:** Timeouts, begrenzte Wiederholungen, Limits je Nutzer und Tag. Ablehnungen des Modells (z. B. Stopp-Grund „refusal“ bei Claude) werden als verständliche Meldung behandelt, nicht als Fehler 500.
- **Modellwahl:** Der Admin konfiguriert sie. Vectra liefert keine fest eingebauten Modellnamen aus; die Doku nennt geprüfte Kombinationen.

## Konsequenzen
- (+) Betrieb komplett ohne Cloud möglich; Cloud nur mit informierter Zustimmung.
- (−) Die Qualität hängt stark vom gewählten Modell ab. Kleine lokale Modelle beherrschen Werkzeugaufrufe teils unzuverlässig; deshalb validiert der Server jede Werkzeugeingabe (ADR-026).
- (−) Lokale KI sprengt das 1-GB-Budget; sie ist ausdrücklich nicht Teil des Basis-Stacks (ADR-030).

## Bezug
Auftrag 5.3, 6.12, 6.13; ADR-025, ADR-026, ADR-030.
