# ADR-016 – Autorisierung: Rollen je Fahrzeug, zentrale Prüfung

- **Status:** akzeptiert · **Datum:** 2026-09-30 · **Entscheidung:** E-4

## Kontext
In der Altanwendung hatte jeder Mitnutzer Vollzugriff ohne Eigentümerbegriff, die Rechte wurden verstreut und teils am falschen Objekt geprüft. Dadurch konnten fremde Datensätze überschrieben werden (Phase 1, R-01 bis R-06, im Test bestätigt).

## Entscheidung
**Rollen je Fahrzeug**

| Aktion | Eigentümer | Bearbeiter | Leser |
|---|---|---|---|
| Fahrzeug und Einträge lesen | ✓ | ✓ | ✓ |
| Einträge anlegen/ändern/korrigieren | ✓ | ✓ | – |
| Einträge löschen (Soft-Delete mit Audit) | ✓ | ✓ | – |
| Fahrzeugstammdaten ändern | ✓ | ✓ | – |
| Freigaben verwalten | ✓ | – | – |
| Fahrzeug löschen oder übertragen | ✓ | – | – |

- Ein Fahrzeug hat **mindestens einen** Eigentümer. Mehrere Eigentümer sind erlaubt (nötig für die Migration nach E-4). Der letzte Eigentümer kann nicht entfernt werden.
- Freigaben erfolgen per **Einladung mit Annahme**, nie ohne Zustimmung des Empfängers.
- **Systemrolle Administrator:** verwaltet Konten und Einstellungen der Installation, hat **keinen** automatischen Zugriff auf Fahrzeugdaten. Ein Notfallzugriff ist nur explizit und mit Audit möglich.
- **Zentrale Prüfung:** Jeder Application Service prüft die Rolle am **geladenen** Objekt. Die Fahrzeug-ID eines Eintrags ist nach dem Anlegen unveränderlich (Verschieben = eigener Vorgang mit Rechten an beiden Fahrzeugen). Deny-by-default: Eine neue Aktion ohne Policy ist nicht aufrufbar.
- **Dateien** werden nur nach derselben Prüfung ausgeliefert (Details ADR-017).
- **API-Token-Scopes:** `vehicles:read`, `entries:write`, `entries:delete`, `sharing:manage`, `admin`. Wirksame Rechte = Schnittmenge aus Token-Scopes und den Rollen des Kontos. Die Prüfung gilt ausnahmslos für alle Endpunkte.

**Abbildung bei Migration (E-4):** LubeLogger-Kollaborator → Eigentümer; Haushalt mit nur „View“ → Leser auf allen Fahrzeugen des Parents; Haushalt mit „Edit“ oder „Delete“ → Bearbeiter; LubeLogger-Root → Administrator der Installation (plus Eigentümer der Fahrzeuge, auf die er Zugriff hatte, falls gewünscht; Abfrage im Import).

## Konsequenzen
- (+) Schließt die Klasse der Phase-1-Lücken R-01 bis R-06 konzeptionell.
- (+) Klare Rollen entsprechend Auftrag 5.3.
- (−) Haushalte als eigenes Konzept entfallen. Sie werden beim Import in Einzelfreigaben aufgelöst.

## Bezug
Phase 1 06, R-01 bis R-06, R-14; Auftrag 5.3, 6.2.
