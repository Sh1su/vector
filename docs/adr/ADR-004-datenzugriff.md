# ADR-004 – Datenzugriff: PostgreSQL mit pgx und sqlc

- **Status:** akzeptiert · **Datum:** 2026-09-30

## Kontext
Die Altanwendung speichert Dokumente als JSON ohne Schema, Fremdschlüssel oder Transaktionen und pflegt zwei Persistenz-Backends parallel (Phase 1, T-03, D-07, D-08). Priorität von Vectra ist Datenintegrität (Auftrag 2).

## Optionen
- **A: pgx + sqlc** – handgeschriebenes SQL, typsicher generierte Go-Funktionen, kein ORM-Laufzeitaufwand.
- B: GORM – schnell, aber implizites Verhalten und Reflexion.
- C: ent – schemagetrieben, schwerer und eigenes Modell.

## Entscheidung
Option A. **Nur PostgreSQL** (≥ 16) wird unterstützt; ein eingebettetes zweites Backend gibt es nicht.
- **Relationales Schema** mit Fremdschlüsseln, `NOT NULL`, `CHECK`-Constraints (z. B. `value_m >= 0`, Währungscode `^[A-Z]{3}$`) und eindeutigen Indizes. JSONB nur für echte Freiform-Daten (Zusatzfelder, EXIF).
- **Ein Schema je Modul** (z. B. `odometer`, `fuel`), Zugriff nur über die sqlc-Queries des eigenen Moduls (ADR-001). Modulübergreifende Fremdschlüssel sind erlaubt, wenn sie nur auf stabile Kern-IDs zeigen (Fahrzeug, Konto, Messpunkt).
- **Transaktionen:** Jeder Application-Service-Aufruf läuft in genau einer Transaktion (Unit of Work); Audit-Eintrag (ADR-011), Outbox-Ereignis und Job-Einreihung (ADR-019) werden in derselben Transaktion geschrieben.
- **Soft-Delete** über `deleted_at` für Fachdaten, gefiltert in allen Standard-Queries; endgültige Löschung nur über einen Aufräum-Job mit Aufbewahrungsfrist.
- **Paginierung** immer in SQL, Aggregationen (Verbrauch, Kosten) als SQL-Abfragen statt „alles laden und rechnen“ (korrigiert T-10).
- **pgvector** nur im optionalen Assistant-Modul (ADR-025).

## Konsequenzen
- (+) Integrität durch die Datenbank abgesichert, Mehrschrittoperationen atomar.
- (+) Kein ORM im Speicher; SQL ist reviewbar.
- (−) Mehr SQL-Handarbeit; sqlc-Generierung wird Teil der CI.

## Bezug
Auftrag 2, 5.2; Phase 1 T-03, T-10, D-07, D-08, D-15; ADR-001, ADR-005.
