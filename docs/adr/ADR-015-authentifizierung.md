# ADR-015 – Authentifizierung: OIDC und lokale Konten

- **Status:** akzeptiert · **Datum:** 2026-09-30 · **Entscheidung:** E-11

## Kontext
Self-Hosting muss ohne Identity-Provider möglich sein, soll aber vorhandene IdPs nutzen können. Die Altanwendung hatte deaktivierte Auth als Default, ungesalzene Hashes, kurze Tokens ohne Ablauf und unsichere Cookies (Phase 1, R-07 bis R-10).

## Entscheidung
- **Sicherer Default:** Es gibt keinen Betrieb ohne Anmeldung. Die Ersteinrichtung legt ein Administratorkonto an (Einmal-Setup-Token aus dem Server-Log oder per Umgebungsvariable).
- **Lokale Konten:** E-Mail + Passwort, Hash mit **Argon2id** (Parameter in der Konfiguration, OWASP-Empfehlung als Default), Passwort mindestens 12 Zeichen, Prüfung gegen eine Liste häufiger Passwörter, Rate-Limit und schrittweise Verzögerung bei Fehlversuchen. TOTP als zweiter Faktor ist optional (nach MVP).
- **OIDC:** Authorization Code + PKCE, Discovery, Validierung von Signatur, `iss`, `aud`, `exp`, `nonce` und `state`. Zuordnung über (`iss`, `sub`), nicht über die E-Mail. Eine Verknüpfung mit einem bestehenden lokalen Konto nur nach Anmeldung in diesem Konto.
- **Web:** serverseitige Sitzung, Cookie `HttpOnly; Secure; SameSite=Lax`, CSRF-Schutz für zustandsändernde Requests, Sitzungen widerrufbar, Rechte werden bei jedem Request aus der Datenbank gelesen.
- **Android:** OAuth2-Flow gegen Vectra (PKCE), kurzlebiges Access-Token (15 min) + rotierendes Refresh-Token, Speicherung im Android Keystore.
- **Persönliche API-Tokens:** zufällig ≥ 256 Bit, nur gehasht gespeichert, mit Scopes (ADR-016), Ablaufdatum und Übergabe nur per Header.
- **Einladungen/Reset-Links:** zufällig ≥ 128 Bit, gehasht gespeichert, einmalig verwendbar; Ablauf 24 h für Reset-Links, 7 Tage für Einladungen (präzisiert in AP-4, `10-domaene-identity.md` ID-04).

## Konsequenzen
- (+) Behebt alle Auth-Schwächen aus Phase 1 konzeptionell.
- (−) Mehr Implementierungsaufwand für lokale Konten; es wird eine etablierte Bibliothek genutzt (Auswahl in AP-2), keine Eigenentwicklung von Kryptografie.

## Bezug
Phase 1 06 §3–§5, R-07 bis R-11; Auftrag 6.2.
