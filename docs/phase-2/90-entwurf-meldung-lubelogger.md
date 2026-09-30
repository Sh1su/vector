# Entwurf: vertrauliche Meldung an die LubeLogger-Maintainer

- **Status:** Entwurf · **Datum:** 2026-09-30 · **Entscheidung:** E-10 (Q-12: „Ja, Text vorbereiten“)
- **Versand:** durch den Projektinhaber, **nicht** durch Claude. Empfohlener Kanal: privater Sicherheitsmeldeweg des Projekts (GitHub „Report a vulnerability“ / Security Advisory), **kein** öffentliches Issue.
- **Inhalt:** rein beschreibend (Befund, Auswirkung, betroffener Bereich, Empfehlung). Bewusst **ohne** Reproduktionsschritte oder Angriffsanleitungen; Details werden erst auf Nachfrage der Maintainer über den privaten Kanal geteilt.
- **Grundlage:** Phase 1, `docs/phase-1/09-risiken-und-altlasten.md` §1 (R-01 bis R-14), geprüfte Version v1.7.3.

Vor dem Versand prüfen:
- [ ] Befunde gegen die dann aktuelle LubeLogger-Version gegenprüfen (evtl. bereits behoben).
- [ ] Kontaktweg im Repository (SECURITY.md / Advisory) bestätigen.
- [ ] Offenlegungsfrist festlegen (Vorschlag: 90 Tage).

---

## Text (Englisch)

**Subject:** Private security report – access control, file handling and authentication findings in LubeLogger v1.7.3

Hello,

while reviewing LubeLogger v1.7.3 for a data-migration project, we noticed several security-relevant issues. We are reporting them privately so they can be addressed before any public discussion. We have not published any of this and will not do so before an agreed disclosure date (we suggest 90 days, and are happy to adjust).

We intentionally describe the findings at a high level. We can share further technical details privately if that helps.

**High severity**

1. **Record ownership not verified on update.** When an existing record is saved, permission is checked against the vehicle submitted with the request, not against the vehicle the stored record belongs to. A user with write access to any vehicle may be able to modify records of vehicles they do not have access to. The odometer bulk-adjust action has a similar pattern. *Suggestion:* authorize against the loaded record and treat a record's vehicle as immutable (or check both vehicles explicitly).
2. **Uploaded files and backups reachable by any signed-in user.** Files in the documents, images and temp locations are protected only by hard-to-guess names; backup archives use predictable, time-based names and contain the database and configuration. *Suggestion:* serve files only after a permission check (or via short-lived signed URLs) and keep backups outside any web-served path.
3. **No server-side validation of uploaded files.** File type and size are not validated on the server, and files are served inline with a content type derived from the extension. This allows active content to be served in the application's origin. *Suggestion:* server-side allow-list and content checks, size limits, `Content-Disposition: attachment` and `X-Content-Type-Options: nosniff`.
4. **Password and token storage.** Passwords and root credentials are stored as unsalted SHA-256; registration/reset tokens have low entropy, are stored in plain text and do not expire. *Suggestion:* Argon2id or bcrypt; tokens with ≥128 bits of entropy, stored hashed, with expiry.
5. **Session cookie hardening.** The session cookie lacks `HttpOnly`, `Secure` and `SameSite`; there is no CSRF protection, and roles are captured in the cookie so revocation is delayed. *Suggestion:* secure cookie attributes, CSRF protection, server-side sessions or short-lived tokens.
6. **Authentication disabled by default.** A fresh installation grants full administrative access to anyone who can reach it over the network. *Suggestion:* require creating an admin account on first start.

**Medium severity**

7. **API key scopes not enforced on all endpoints.** Some write endpoints do not apply the API key permission filter, so read-only keys can create data; one write action is reachable via GET. *Suggestion:* central deny-by-default scope enforcement.
8. **Missing permission check on a read endpoint** for calendar reminders. *Suggestion:* a single authorization layer used by all handlers.
9. **OIDC hardening.** `state` and PKCE are optional and off by default; issuer, audience and nonce are not validated; accounts are linked by e-mail claim; the post-login redirect target is not validated. *Suggestion:* use a standard OIDC client with discovery and link accounts by `iss` + `sub`.
10. **File path handling** in the temp-to-storage move and delete helpers does not normalize paths. *Suggestion:* normalize and confine to the storage root, or use storage keys instead of paths.

**Low severity**

11. API keys are accepted as a query parameter (they end up in logs); headers only would be safer.
12. User input is inserted unescaped into HTML e-mails; SMTP uses opportunistic rather than required TLS.
13. Admins can demote or delete each other (including the last admin), and users can be added to households/collaborations without their consent.

Thank you for maintaining LubeLogger. Please let us know how you would like to proceed and whether you would like to coordinate a disclosure date or credit.

Kind regards,
&lt;Name&gt;
