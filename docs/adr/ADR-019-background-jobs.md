# ADR-019 – Background Jobs mit River

- **Status:** akzeptiert · **Datum:** 2026-09-30

## Kontext
Vectra braucht asynchrone und zeitgesteuerte Arbeit: Benachrichtigungen, Fälligkeitsprüfung, Vorschaubilder, Import, Aufräumen, optional Embeddings. In der Altanwendung laufen solche Prüfungen beim Seitenaufruf oder mit Zustand nur im Speicher; nach einem Neustart kommt es zu Mehrfachmeldungen (Phase 1, D-17, T-06). Der Auftrag gibt River vor (5.2).

## Optionen
- **A: River** – PostgreSQL-basierte Queue, transaktionales Einreihen, keine zusätzliche Infrastruktur.
- B: Eigener Scheduler – weniger Abhängigkeiten, aber Wiederholung, Sperren und Dead-Letter wären Eigenbau.
- C: Redis-basierte Queue – zusätzlicher Dienst, widerspricht dem Ressourcenbudget.

## Entscheidung
Option A. Worker laufen im selben Prozess wie die API (ADR-001); ein Betriebsmodus `--role=worker` erlaubt das Trennen, ist aber nicht nötig.
- **Transaktionales Einreihen:** Jobs werden in derselben Transaktion wie die auslösende Änderung eingereiht (ADR-004). Kein Job ohne Änderung, keine Änderung ohne Job.
- **Periodische Jobs:** Fälligkeitsprüfung Wartung (stündlich), Aufräumen Soft-Deletes und Idempotenzschlüssel (täglich), Storage-Konsistenz (wöchentlich). Periodische Jobs nutzen eindeutige Schlüssel je Zeitfenster, damit mehrere Instanzen sie nicht doppelt ausführen.
- **Idempotente Handler:** Jeder Job darf mehrfach laufen. Fachliche Deduplizierung liegt persistent in der Datenbank (z. B. Benachrichtigungen, ADR-020), nicht im Speicher.
- **Fehlerbehandlung:** exponentielles Backoff, maximal 10 Versuche (konfigurierbar je Job-Art), danach Status *discarded*; Admins sehen fehlgeschlagene Jobs in der Verwaltung und können sie erneut anstoßen.
- **Ressourcen:** begrenzte Parallelität (Default 2 Worker, Import 1), damit das Budget von 1 vCPU / 1 GB gehalten wird.
- **Keine Fachlogik im Job:** Jobs rufen Application Services auf, mit Akteur `system` im Audit (ADR-011).

## Konsequenzen
- (+) Zuverlässig nach Neustarts, keine Zusatzdienste.
- (−) Last auf PostgreSQL; für die erwartete Größe (Selbsthoster, wenige Fahrzeuge) unkritisch, wird im Spike (AP-9) gemessen.

## Bezug
Auftrag 5.2, 6.14; Phase 1 D-17, T-06; ADR-001, ADR-004, ADR-011, ADR-020.
