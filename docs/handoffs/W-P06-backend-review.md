# Review W-P06 Backend: Katalog, Übersicht, Definition, Serverinfo (API 0.7)

Reviewer: unabhängig (nicht Autor). Geprüft: `feat/w-p06-backend` (9f44a7c, Basis main 02a30db), 24.10.2026. Schwerpunkt Sicherheit.
Review-Branch: `review/w-p06-backend` (nur Tests und dieses Dokument).

## Urteil

Freigabe (accept). Keine blockierenden oder mittleren Befunde. Zwei niedrige und zwei informative Hinweise.

## Ausgeführt

| Prüfung | Ergebnis |
| --- | --- |
| `gofmt -l .`, `go vet ./...` | ohne Ausgabe |
| `go test -race -count=1 ./...` | alle Pakete ok |
| fünf Vertragswächter + `check-overview-contract.py` | bestanden (v1 bis v6 byteidentisch, v7 additiv, 32 Fixtures) |
| SHA-256 `contracts/api-v0.7.0.json` | `c6fd8c91315e25185d949d309397c7c0d80cc67fcab87ae521e38068e7004d17`, stimmt mit Handoff |
| `scripts/test-overview.py` | bestanden: 20 echte Antworten (SQLite, MySQL 8.4, MariaDB 12.3, PostgreSQL 18.6) gegen das v7-Schema validiert, danach keine Container |
| eigene Livetests (siehe unten) auf SQLite, MySQL, MariaDB, PostgreSQL | bestanden |
| `scripts/check-docs.py` | 0 Fehler |

Docker-Suiten nacheinander, jeweils eigene Wegwerf-Container (Präfix `dbmanager-w-p06*`), danach keine Reste.

## Befunde

### (1) Injektion: kein Befund
Beleg: Alle Bezeichner in `SHOW CREATE`, `COUNT(*)` und der PostgreSQL-Rekonstruktion kommen aus der Katalogzeile bzw. aus `dbplan.QualifiedName`/`QuoteIdentifier`
(zuvor im W-P01-Review gegen `database.quoteIdentifier` gefuzzt); Anfragewerte laufen nur als gebundene Parameter (`?`, `$n`).
Eigene Livetests `internal/database/overview_attack_review_test.go` mit 15 Angriffsnamen je System (Backtick, doppelte Backticks, Anführungszeichen,
`; DROP TABLE`, Unicode, Zeilenumbruch, Backslash, `%_`, maximale Namenslänge: 64 MySQL, 63 PostgreSQL, 300 SQLite): Objektliste mit exakter Zählung (2 Zeilen je Tabelle),
Definition, Varianten mit angehängtem Quote/Semikolon werden `object_not_found`, Schema- und Namens-Injektionen (inkl. NUL, > 1024 Zeichen) abgelehnt, Fixture-Tabelle unversehrt.

### (2) Informationslecks: kein Befund
- Sichtbarkeit: MySQL/MariaDB über `information_schema` (Rechteregeln des Servers), PostgreSQL mit `pg_has_role`/`has_table_privilege`/`has_any_column_privilege`; Livetest mit rechtelosem Benutzer: leere Liste, 404, Serverinfo ok.
- Fehler: nur `safe()`-Meldungen; Test prüft, dass `SELECT`, `information_schema`, `pg_class`, `sqlite_schema`, Passwort und `/tmp` nicht in Fehlern stehen; falsches Passwort wird nicht gespiegelt.
- Serverinfo: Verbindungsecho ohne Passwort; Version, Variante, Session-Variablen, Zeichensätze, Engines/Plugins/Extensions, Uptime; keine Konfigurationsgeheimnisse, keine Pfade.
- SQLite: nur `main.sqlite_schema` mit `name NOT GLOB 'sqlite_*'`; Definition für `sqlite_*` und Schema `temp` abgelehnt; keine PRAGMAs (Sandbox unverändert).

### (3) Teure Abfragen: kein Befund
Exakte Zählung nur mit `exact_rows`, nur Basistabellen, Seite höchstens 100 (Standard bei `limit=0` ebenfalls 100), ein gemeinsames Zeitbudget (`timeout_ms`, höchstens 30 s), nach dem ersten Timeout Rest `unknown` und `exact_incomplete`; Abbruch des Requests liefert 408.
Eigener Livetest `TestReviewExactCountBudget` (PostgreSQL, 12 Mio. Zeilen): 40 ms Budget endet nach rund 52 ms, `exact_incomplete=true`, nie als exakt ausgegeben; 200 ms Context-Abbruch schlägt fehl; danach 0 aktive serverseitige COUNT-Abfragen.
Listen auf 1000, Offset 1 000 000, Definition 1 MiB, Antwort 8 MiB, Zellgrößen begrenzt; Katalogseite hat eigenes 10-s-Budget.

### (4) Korrektheit je Treiber: kein Befund
Bestehende Livetests gegen Direktabfragen (SHOW CREATE, sqlite_schema, VERSION()) bestanden; Tabelle vs. View, Schätzung vs. exakt, Paging ohne Duplikate, leeres Schema, Sondernamen, PostgreSQL `derived=true` mit Hinweis. Eigene Tests ergänzen Unicode und Grenzlängen.

### (5) Vertrag: kein Befund
SHA stimmt; alle neuen v7-Schemas haben `additionalProperties:false` (die ohne Angabe sind ausschließlich eingefrorene v1-bis-v6-Schemas); Pfade v1 bis v6 unverändert (Wächter); `contracts/README.md` unverändert; echte Fixtures durch `test-overview.py` regeneriert und validiert.

### (6) Auth, Methoden, Limits: kein Befund
Alle fünf v7-Routen hinter `a.auth` (Test in `features_test.go`: 401 ohne Token); GET nur Capabilities, übrige POST; `decode` mit 1 MiB, `DisallowUnknownFields`, ein JSON-Objekt; `validConnection` vor Engine-Zugriff.

### Hinweise
- L1 (niedrig): v7-Pfade im Vertrag nennen 400/404/408 nur im Text der `default`-Antwort, nicht als eigene Statusschlüssel. Fix: bei nächster Vertragsrevision `400`, `404`, `408` ergänzen (nicht jetzt, Vertrag ist gehasht).
- L2 (niedrig): MySQL `SYSTEM VIEW` (information_schema-Sichten) wird als `view` gelistet; ein `SHOW CREATE VIEW` darauf kann serverseitig scheitern und ergibt dann generisches `database_error` statt Definition. Ungefährlich, nur Nutzerführung; Web-Paket sollte `system`-Schemas nicht als Definitionsziel anbieten.
- I1 (informativ): PostgreSQL kürzt gebundene `name`-Parameter auf 63 Byte, daher löst ein überlanger Name (64 Byte) auf das 63-Byte-Objekt auf. Antwort trägt den echten Katalognamen, keine Injektion, nicht ausnutzbar. Eigener Test überspringt diese Variante bewusst.
- I2 (informativ): Wie im Handoff genannt läuft eine MySQL-Zählung nach Abbruch serverseitig ggf. kurz weiter (PostgreSQL bricht nachweislich ab).

## Ergänzte Tests (Review-Branch)
`internal/database/overview_attack_review_test.go`: `TestReviewAttackSQLite`, `TestReviewAttackLive` (mysql, maria, postgres), `TestReviewExactCountBudget` (PostgreSQL, überspringt ohne Fixture).
