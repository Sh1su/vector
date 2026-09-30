# ADR-008 – Zeitmodell: Zeitstempel mit Zeitzone

- **Status:** akzeptiert · **Datum:** 2026-09-30 · **Entscheidung:** E-2

## Kontext
Die Altanwendung speichert nur ein Datum ohne Zeitzone; in der eingebetteten Datenbank verschiebt es sich beim Zeitzonenwechsel des Servers um einen Tag (Phase 1, M-6, D-03, im Test bestätigt). Kilometerstände sollen zeitlich monoton geprüft werden (Auftrag 6.4); dafür reicht ein Datum nicht aus, wenn mehrere Einträge am selben Tag liegen.

## Optionen
- **A: `timestamptz` + Zeitzone des Ereignisorts**
- B: Nur Kalenderdatum – einfach, aber die Reihenfolge am selben Tag ist unklar.

## Entscheidung
Option A.
- Fachliche Ereignisse (Tanken, Kilometerstand, Ölmessung, Service, Fahrtbeginn/-ende) speichern `occurred_at` (`timestamptz`, UTC) und `time_zone` (IANA-Name, z. B. `Europe/Berlin`), in der das Ereignis erfasst wurde.
- `time_precision` ∈ {`exact`, `date_only`}. Bei `date_only` (Altdaten, Belege ohne Uhrzeit) gilt 12:00 Uhr lokal als Ordnungszeitpunkt. Die UI zeigt dann nur das Datum.
- Serverzeitpunkte (`created_at`, `updated_at`, Upload-Zeit) werden immer vom Server gesetzt. Vom Client gemeldete Aufnahmezeiten (EXIF) werden separat gespeichert und als clientseitig gekennzeichnet (Auftrag 6.7).
- Fälligkeiten nach Datum (Wartung, HU) sind **Kalenderdaten** (`date`) in der Zeitzone des Fahrzeughalters. Sie gelten bis zum Ende dieses Tages als nicht überfällig. Das korrigiert BR-026 der Altanwendung.
- Die API liefert und akzeptiert ausschließlich ISO 8601 mit Offset (kulturunabhängig).

## Konsequenzen
- (+) Eindeutige Reihenfolge, keine Verschiebung durch Serverumzug.
- (−) Beim Import ist die Zeitzone des LubeLogger-Servers anzugeben (ADR-027).

## Bezug
Phase 1 M-6, M-7, M-8, BR-026, MG-2, MG-3; Auftrag 6.4, 6.7.
