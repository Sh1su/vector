# ADR-012 – Optimistisches Locking und Idempotenz

- **Status:** akzeptiert · **Datum:** 2026-09-30

## Kontext
Mehrere Nutzer und die Android-App (offline, Auftrag 6.16) ändern dieselben Fahrzeuge. Die Altanwendung kennt weder Konflikterkennung noch Wiederholungsschutz; Aktionen wie „Plan erledigt“ erzeugen bei Wiederholung Duplikate (Phase 1, D-09, D-10).

## Entscheidung
- **Versionierung:** Jede änderbare Ressource hat eine ganzzahlige `version`, die bei jeder Änderung steigt. Die API liefert sie als `ETag` (stark, z. B. `"7"`).
- **Änderungen** (`PATCH`, `PUT`, `DELETE`) verlangen `If-Match`. Fehlt der Header → `428 Precondition Required`; passt er nicht → `412 Precondition Failed` mit aktuellem Stand im Fehlerdokument, damit Clients einen Konfliktdialog zeigen können (ADR-021).
- **Anlegen** (`POST`) erfolgt idempotent auf zwei Wegen:
  - Client-erzeugte UUIDv7 (ADR-006): ein erneutes Anlegen derselben ID mit identischem Inhalt liefert `200` mit der bestehenden Ressource, mit abweichendem Inhalt `409`.
  - Header `Idempotency-Key` für Aktionen ohne eigene ID (z. B. „Wartung als erledigt markieren“): Ergebnis wird 24 h gespeichert, Wiederholung liefert dieselbe Antwort.
- **Folgewirkungen** (z. B. Fälligkeit einer Wartung nach Serviceeintrag fortschreiben) werden als explizite, gespeicherte Verknüpfung modelliert und nur einmal angewendet (korrigiert D-09/D-10).
- Serverseitige Nebenläufigkeit innerhalb einer Transaktion: `SELECT … FOR UPDATE` auf dem Aggregat (z. B. Fahrzeug bei Tachotausch).

## Konsequenzen
- (+) Keine verlorenen Updates, sichere Wiederholungen bei schlechtem Netz.
- (−) Clients müssen ETags mitführen; die generierten Clients (ADR-014) kapseln das.

## Bezug
Auftrag 6.16, 6.17; Phase 1 D-07, D-09, D-10; ADR-006, ADR-021.
