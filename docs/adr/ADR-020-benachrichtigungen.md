# ADR-020 – Benachrichtigungen: E-Mail, UnifiedPush, optional FCM

- **Status:** akzeptiert · **Datum:** 2026-09-30 · **Entscheidung:** E-12

## Kontext
Fällige und überfällige Wartungen sollen per Push und E-Mail gemeldet werden (Auftrag 6.14). Firebase Cloud Messaging bedeutet eine Google-Abhängigkeit im Self-Hosting. In der Altanwendung lag die Deduplizierung nur im Speicher, daher kamen nach Neustarts Mehrfachmeldungen (Phase 1, D-17).

## Entscheidung
- **Kanäle:** E-Mail (SMTP mit erzwungenem TLS), **UnifiedPush** als Standard für Android (z. B. über einen selbst gehosteten ntfy-Server), **FCM optional** (per Konfiguration aktivierbar, Nutzer wählt), ausgehende **Webhooks** mit HMAC-Signatur und Retry über die Job-Queue.
- **Ereignisse:** Wartung demnächst fällig / fällig / überfällig, HU/TÜV, Einladung erhalten, Import abgeschlossen, Sicherheitsereignisse des Kontos.
- **Deduplizierung** persistent in der Datenbank: Schlüssel (Empfänger, Ereignistyp, Objekt-ID, Stufe). Dieselbe Stufe wird nicht erneut gemeldet, eine höhere Stufe schon.
- **Empfänger:** alle Nutzer mit Rolle am Fahrzeug, die den Kanal und das Ereignis nicht abbestellt haben. Die Einstellung liegt je Nutzer und Ereignistyp.
- **Zustellung** über Background Jobs (ADR-019) mit Wiederholung und Dead-Letter-Liste; ein Ausfall eines Kanals blockiert die anderen nicht.
- Inhalte werden über Vorlagen mit Escaping erzeugt (keine ungefilterten Nutzereingaben in HTML-Mails, Phase 1 R-13).

## Konsequenzen
- (+) Self-Hosting ohne Google möglich, FCM für Komfort verfügbar.
- (−) Nutzer von UnifiedPush brauchen eine Distributor-App. Die Einrichtung wird in der App geführt.

## Bezug
Auftrag 5.2 (Push), 6.14; Phase 1 BR-034, BR-054, D-17, R-13.
