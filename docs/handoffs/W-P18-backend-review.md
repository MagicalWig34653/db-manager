# Review W-P18 Backend: Benutzer und Rechte (API 0.15)

Reviewer: unabhängig (nicht Autor, Codex-Paket). Geprüft: `integration/wave3-backend` (f345adb), Merge 53ee62a, 26.10.2026.
Review-Branch: `review/wave3-a` (nur Tests und Review-Dokumente). Schwerpunkt: Passwörter, Selbstaussperrung, Rechte-Eskalation.

## Urteil

Freigabe mit Auflagen (accept with changes). Passwörter, Injektion und Selbstaussperrung sind sauber gelöst. Drei mittlere Befunde (M1 bis M3) betreffen
Systemkonten, stille No-op-Grants in PostgreSQL und fehlende Zusatzbestätigung für eskalierende Rechte; M4 ist ein serverseitiges Passwort-Log-Risiko in PostgreSQL.

## Ausgeführt

| Prüfung | Ergebnis |
| --- | --- |
| `gofmt -l .`, `go vet ./...`, `go test -race -count=1 ./...` | sauber, alle Pakete ok |
| alle `check-*-contract.py` | bestanden (accounts: v1 bis v14 byteidentisch, v15 additiv, strikt) |
| SHA-256 `contracts/api-v0.15.0.json` | `cd4681a2...dad1`, stimmt mit Integrationshandoff |
| `scripts/test-accounts.py` | bestanden (drei Live-Treiber, Kataloggegenprüfung, Antwortschemas), keine Container übrig |
| `scripts/check-docs.py` | 0 Fehler |
| eigene Livetests `TestReviewW3Accounts` (MySQL 8.4, MariaDB 12.3, PostgreSQL 18.6, je ein Wegwerf-Container nacheinander) | bestanden; Serverlogs und `docker inspect` nach dem Passwortmarker durchsucht |
| Mutationstest Wächter | wirksam (siehe unten) |

## Befunde

### M1 (mittel): Systemkonten sind löschbar
Beleg (Livetest): `drop` von `mysql.sys@localhost`, `mysql.session@localhost`, `mysql.infoschema@localhost` (MySQL 8.4) und `mariadb.sys@localhost` (MariaDB 12.3) wird geplant (`self=true`, getippter Name) und mit Bestätigung `completed`. Das zerstört interne Serverfunktionen (sys-Schema, Sitzungs-/Infoschema-Definer).
PostgreSQL: `pg_monitor`/`pg_read_all_data` werden vom Server abgelehnt (`dependency_blocks`), also nur durch den Server geschützt.
Fix: feste Sperrliste im Plan (MySQL/MariaDB: `mysql.*`/`mariadb.sys` mit Host `localhost`, optional `root`; PostgreSQL: `pg_*`, `postgres`) mit `invalid_account_request` oder `unsupported_action` bei drop/rename/lock/password/revoke.

### M2 (mittel): PostgreSQL meldet wirkungslose GRANTs als `completed`
Beleg (PostgreSQL 18): Rolle ohne Grant-Recht führt `GRANT SELECT ON rv_gt TO rv_u3` aus; PostgreSQL gibt nur eine Warnung aus ("no privileges were granted"), die API antwortet `completed`, `has_table_privilege('rv_u3','rv_gt','SELECT')` ist danach `false`. MySQL/MariaDB lehnen ab (`rejected`).
Fix: nach GRANT/REVOKE in derselben Transaktion per ACL-Abfrage (`has_*_privilege`/`aclexplode`) prüfen oder die Server-Notice auswerten und sonst `rejected/no_effect` melden.

### M3 (mittel): Eskalierende Rechte ohne Zusatzbestätigung und ohne Warnung
Beleg: `GRANT ... WITH GRANT OPTION` (alle Treiber) wird mit `destructive=false` und nur der Standardwarnung geplant; keine Warnung nennt GRANT OPTION. Auf MySQL/MariaDB werden `SUPER`, `CREATE USER`, `SET_USER_ID`, `SYSTEM_USER`, `SET_ANY_DEFINER`, `ROLE_ADMIN` global geplant (`destructive=false`) und verlangen nur Aktion und Plan-Hash. Die API prüft also nicht, ob der Aufrufer sie selbst halten darf (der Server entscheidet bei der Ausführung; Test: Benutzer ohne Recht bekommt schon beim Planen `permission_denied`, `AccountAccess` blendet für ihn alle Schreibaktionen aus).
Fix: `privileged=true` für GRANT OPTION, globale Administrationsrechte und `role_grant` mit Administrationsrollen; dafür getippter Kontoname und eine ausdrückliche Warnung.

### M4 (mittel, serverseitig): Klartextpasswort im PostgreSQL-Serverlog bei fehlgeschlagener Anweisung
Beleg: `docker logs` des PostgreSQL-Containers enthält den Passwortmarker nach einem abgelehnten `CREATE ROLE ... PASSWORD '...'` (Standard `log_min_error_statement=error` protokolliert die fehlerhafte Anweisung samt Literal). MySQL/MariaDB: Marker nicht in den Logs (Standardkonfiguration).
Die API selbst gibt das Passwort nirgends aus (siehe unten); das Risiko liegt beim Server.
Fix: PostgreSQL-Passwort clientseitig als SCRAM-SHA-256-Prüfwert berechnen und als `PASSWORD 'SCRAM-SHA-256$...'` senden (Klartext erreicht den Server nicht), oder die Gefahr im Plan und Vertrag ausdrücklich warnen.

### L1 (niedrig): Maximale Passwortlänge 4096, MySQL lehnt längere als etwa 256 Zeichen mit generischem `database_error` ab
Fix: Grenze je Treiber setzen oder Grund `invalid_secret` melden.

### L2 (niedrig): Name-Obergrenze 32 gilt auch für MariaDB (erlaubt 80)
Beleg: 33 Zeichen langer Name wird mit `invalid_account_request` abgelehnt. Keine Sicherheitsfolge.

### L3 (niedrig): Wächter pinnt die eigene Vertragsdatei nicht
Beleg: Ein angehängtes Byte an `api-v0.15.0.json` besteht `check-accounts-contract.py` (der Programmwächter fängt es für v15 ab). Fix: SHA-Pin für v15 im Wächter (Integrationshandoff nennt `cd4681a2...`).

## Geprüft ohne Befund
1. Passwörter: Passwörter mit Quotes, Backslash, Unicode (Emoji) und 240 Zeichen (jeweils mit Marker) führen auf MySQL, MariaDB und PostgreSQL zu einem funktionierenden Login mit exakt diesem Passwort (Quoting korrekt, inkl. NO_BACKSLASH_ESCAPES und standard_conforming_strings); falsches Passwort wird abgelehnt, Fehlertext ohne Passwort. Leer, NUL, ungültiges UTF-8, 4097 Zeichen und fehlendes Secret werden abgelehnt. Plan und Plan-Hash enthalten nur `<protected-secret>`; Marker taucht in keinem Plan, Ergebnis, Fehler, Loginfehler, `docker inspect` und (MySQL/MariaDB) keinem Serverlog auf. Fehlerpfad ohne Recht (`permission_denied`): Meldung fest, ohne Passwort.
2. Injektion: Kontoname `x';DROP USER 'rv_victim'@'%';--` (MySQL/MariaDB) und `x"; DROP ROLE rv_victim; --` (PostgreSQL) wird als Konto angelegt, das Opfer bleibt bestehen. Grant mit Datenbankname `probe`; DROP USER ...`, Privileg `SELECT; DROP USER ...`, Tabellenname `t' OR '1'='1`, `ALL PRIVILEGES` und angehängtes SQL im Privilegtext werden abgelehnt. Datenbank-, Tabellen-, Spalten- und Routinenziele werden aus dem Katalog aufgelöst; Wildcard-Datenbanknamen (`_`, `%`, `\`) werden abgelehnt.
3. Selbstaussperrung: drop, password, lock, rename, revoke und role_revoke des eigenen Kontos (`root@%` bzw. `postgres`) sind `self=true` und verlangen `acknowledge_self`; ohne sie `confirmation_mismatch`, Konto unverändert. Gleicher Name bei anderem Host zählt als eigenes Konto (`root@localhost` `self=true`). `drop` und `revoke` verlangen die Zweitbestätigung stets.
4. Bestätigung: fehlender/großgeschriebener getippter Name, falsche Aktion, Nullhash und fehlende Zweitbestätigung bei `drop` lehnen ab, Konto bleibt. Zielname stammt aus dem Katalog (längengeprägtes Label `Länge:Name@Host`, daher keine Mehrdeutigkeit). MySQL-Hosts werden vom Server kleingeschrieben (Test), Auflösung erfolgt exakt binär.
5. Fähigkeiten: SQLite `unsupported_action`; PostgreSQL `flush` nicht verfügbar; Rollen/Sperren versionsabhängig geprüft; Rechte-Eskalation durch den Aufrufer: Benutzer ohne Rechte erhält `permission_denied` (MySQL, MariaDB schon beim Plan); die Anzeigehilfe `AccountAccess` blendet Schreibaktionen aus und kennzeichnet sich als „keine Autorisierung".
6. Auth/Methoden/Body-Limits: alle v15-Routen hinter `a.auth`, `decodeAccounts` verlangt je Aktion exakt die erlaubten Felder, 1-MiB-Limit über `decode`.
