# ADR-025 – Dokumentenanalyse (RAG) mit pgvector

- **Status:** akzeptiert · **Datum:** 2026-09-30

## Kontext
Nutzer sollen Fragen an ihre Fahrzeugakte stellen können („Welche Öl-Spezifikation verlangt mein Motor?“, „Wann wurden laut Rechnungen die Bremsen gemacht?“). Jede Antwort nennt Dokument, Seite und Textstelle. Herstellervorgaben und durchgeführte Arbeiten werden unterschieden, und Dokumentinhalte gelten nie als Anweisung (Auftrag 6.12). Eine eigene Vektordatenbank ist ausgeschlossen (5.3).

## Entscheidung
**Pipeline** (Jobs, ADR-019; nur wenn der Assistent aktiv ist):
1. **Dateiprüfung:** wie ADR-017. Nur PDF und Bilder werden indexiert.
2. **Textextraktion:** Textebene des PDF je Seite (auch ohne KI für die Volltextsuche, DO-03).
3. **OCR nur bei Bedarf:** Seiten ohne verwertbare Textebene und Bilder. Lokal über eine OCR-Engine im optionalen Container oder über ein Vision-Modell des konfigurierten Anbieters (ADR-024, mit denselben Zustimmungsregeln).
4. **Strukturierung:** Seiten- und Abschnittsgrenzen (Überschriften), Tabellen als Text mit Zeilen.
5. **Chunking:** innerhalb einer Seite, ca. 400–800 Token mit 15 % Überlappung. Jeder Chunk trägt `document_id`, `page`, Zeichenbereich und `nature` (Vorgabe oder Nachweis, Documents §2.1).
6. **Embeddings:** Embedding-Adapter (ADR-024). Die Dimension ist je Index fest und wird mit Modellname und Version gespeichert. Ein Modellwechsel löst eine Neuindexierung aus.
7. **Index:** Tabelle `assistant.chunk_embedding` mit `vector(n)` (pgvector) und HNSW-Index, außerdem `tsvector` für die Stichwortsuche.
8. **Retrieval:** hybrid (Vektor- und Stichwortsuche, Ranglisten zusammengeführt), **immer gefiltert** auf Fahrzeuge, auf die der Nutzer Leserecht hat (ID-01). Der Filter steht in der SQL-Abfrage selbst, nicht nachgelagert.
9. **LLM:** bekommt die Top-Chunks als klar abgegrenzte, mit IDs versehene **Daten**-Blöcke; die Antwort muss Chunk-IDs zitieren. Der Server prüft, dass jedes Zitat auf einen gelieferten Chunk zeigt, und macht daraus Links auf Dokument und Seite. Nutzt der Anbieter eine eigene Zitierfunktion (bei Claude: Dokument-Blöcke mit Seitenangabe), wird sie auf dasselbe Format abgebildet.

**Regeln:**
- **Vorgabe oder Nachweis:** Chunks aus `specification` werden als „Herstellervorgabe“ gekennzeichnet, Chunks aus `record` als „Nachweis“. Der Systemprompt verbietet, eine Vorgabe als erledigte Wartung auszugeben. Die Antwort-UI zeigt die Kennzeichnung je Quelle. „Wann wurde X gemacht?“ beantworten zuerst die strukturierten Daten (Serviceeinträge über Werkzeuge, ADR-026), Dokumente nur ergänzend.
- **Schutz vor Prompt Injection:** Dokumenttext steht nie im Systemprompt. Er erscheint nur als zitierbarer Datenblock mit dem Hinweis, dass er keine Anweisungen enthält. Werkzeuge mit Schreibwirkung verlangen immer die Bestätigung des Nutzers (ADR-026), sodass eingeschleuster Text allein nichts ändern kann.
- **Keine Quelle, keine Behauptung:** Ohne passende Chunks lautet die Antwort „dazu finde ich nichts in deinen Dokumenten“, statt zu raten.
- **Löschen:** Wird ein Dokument gelöscht oder ersetzt, werden seine Chunks sofort entfernt.

## Konsequenzen
- (+) Keine zusätzliche Infrastruktur; Rechteprüfung im selben SQL wie die Suche.
- (−) HNSW-Indizes kosten Speicher. Für die erwartete Größe (Hunderte bis wenige Tausend Seiten je Installation) ist das unkritisch; es wird in Phase 3 gemessen.

## Bezug
Auftrag 6.12; ADR-017, ADR-019, ADR-024, ADR-026; Documents DO-03.
