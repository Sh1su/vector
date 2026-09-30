# ADR-011 – Audit und Änderungsverfolgung

- **Status:** akzeptiert · **Datum:** 2026-09-30

## Kontext
In der Altanwendung wird hart gelöscht und überschrieben, ohne Protokoll (Phase 1, D-15). Vectra verlangt nachvollziehbare Korrekturen, insbesondere bei Kilometerständen, Beweisfotos und Fahrten (Auftrag 6.4, 6.7; ADR-009, ADR-010).

## Optionen
- **A: Append-only Audit-Tabelle** plus fachliche Korrekturmodelle, wo die Historie Teil der Fachlichkeit ist.
- B: Event Sourcing für alle Module – vollständige Historie, aber hoher Aufwand und schwer abfragbar.

## Entscheidung
Option A.
- **Tabelle `audit.events`** (nur `INSERT`, Rechte der Anwendungsrolle entsprechend eingeschränkt): `id`, `occurred_at` (Serverzeit), `actor_account_id`, `actor_kind` ∈ {`user`, `api_token`, `assistant`, `import`, `system`}, `action` (z. B. `fuel_fill.updated`), `vehicle_id`, `object_type`, `object_id`, `changes` (JSONB: Feld → alt/neu), `reason` (Pflicht bei Plausibilitätsbestätigung, ADR-010), `request_id`.
- Geschrieben wird in derselben Transaktion wie die Änderung (ADR-004), aus dem Application Service – nicht per Trigger, damit Akteur und Grund bekannt sind.
- **Fachliche Korrekturen statt Überschreiben** gelten für: Kilometer-Messpunkte (ADR-009), Fahrten (append-only mit Korrekturbuchung, Q-13) und Beweisfotos (Original unveränderlich, ADR-018). Alle anderen Einträge dürfen geändert werden; die Änderung wird auditiert.
- **Löschen** ist Soft-Delete mit Audit-Eintrag; Wiederherstellen innerhalb der Aufbewahrungsfrist (Default 30 Tage) ist möglich.
- **Sichtbarkeit:** Eigentümer und Bearbeiter sehen die Historie ihrer Fahrzeuge; Administratoren sehen konto- und installationsbezogene Ereignisse, aber keine Fahrzeuginhalte (ADR-016).
- Sensible Werte (Passwörter, Tokens) erscheinen nie im Audit.

## Konsequenzen
- (+) Jede Änderung ist nachvollziehbar, inklusive Assistent und Import.
- (−) Wachsende Tabelle; Partitionierung nach Monat ist vorgesehen, Aufbewahrung konfigurierbar.

## Bezug
Auftrag 6.4, 6.7; Phase 1 D-15; ADR-009, ADR-010, ADR-016, ADR-018.
