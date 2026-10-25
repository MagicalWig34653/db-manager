# W-P11 Backend Review (Export und SQL-Dump, API 0.11)

Stand 25.10.2026. Reviewer: unabhängig (nicht Autor). Geprüft: `integration/wave2-backend` (60592de), Merge 0ceeb59,
Vertrag `contracts/api-v0.11.0.json` SHA-256 `b3bcc38b44b020335151aead207e281f9c54bdc7463d6fb1ddd7a6cc22396620`.
Review-Worktree `review/wave2-b`; Integrationsbranch unverändert. Last Load 6 bis 230, Docker nacheinander, nur Wegwerf-Container
(aufgeräumt).

## Urteil

**Freigabe (Approve), keine Blocker.** Dump-Treue und Injektionsschutz sind in allen drei Dialekten belegt. Ein Hinweis mit
Web-Gate (Befund 1), sonst niedrige Befunde.

## Befunde

| # | Schwere | Befund | Beleg | Fix |
| --- | --- | --- | --- | --- |
| 1 | Mittel (Gate für das Web-Paket) | Abbruch oder Limit nach Streambeginn ist nur über HTTP-Trailer (`X-Export-Status: incomplete`, `X-Export-Error`) und das fehlende Schlussmarkenzeichen erkennbar; Antwort bleibt HTTP 200. Trailer sind in `fetch`/Browser-Download nicht lesbar, ein Download kann also als „fertig“ erscheinen. | Probe `max_rows`, `max_bytes`, `timeout_ms`: Status 200, Trailer vorhanden (curl `--raw`), Inhalt ohne `-- DBMANAGER EXPORT COMPLETE`. | Web muss die Schlussmarke prüfen (SQL: letzte Zeile, JSON: `"complete":true`, CSV: Kontrollzeile `complete`) und sonst „unvollständig“ melden; im Vertrag als Pflicht für Clients benennen. |
| 2 | Niedrig | MySQL-Dump setzt `sql_mode` nicht zurück. Auf einem Zielserver mit `NO_BACKSLASH_ESCAPES` werden `\\` in den von `SHOW CREATE TABLE` gelieferten Literalen (Default/Comment/ENUM) falsch gelesen, Restore bricht ab bzw. verfälscht Werte. Kein Injektionspfad: Quotes werden als `''` ausgegeben, Daten als Hex. | Restore des realen MySQL-8.4-Dumps mit `--init-command=SET sql_mode=...NO_BACKSLASH_ESCAPES`: Fehler 1265 in Zeile 24. | Header `SET SESSION sql_mode=REPLACE(@@sql_mode,'NO_BACKSLASH_ESCAPES','')` bzw. wie `mysqldump` (`SET @OLD_SQL_MODE`) und am Ende zurücksetzen. |
| 3 | Niedrig | `if_not_exists` macht nur `CREATE TABLE` idempotent; angehängte `CREATE INDEX` (SQLite/PostgreSQL) bleiben ohne `IF NOT EXISTS`. Zweiter Restore in dieselbe Datenbank scheitert am Index. | Code `exportDefinition`/`structure` (nicht ausgeführt). | Index-Anweisungen ebenfalls mit `IF NOT EXISTS` ausgeben oder dokumentieren. |
| 4 | Niedrig | Zeitüberschreitung meldet generisch `X-Export-Error: export_failed` statt eines Abbruchcodes (Byte-/Zeilenlimit haben eigene Codes). | Probe `timeout_ms:1`. | `safe(ctx, ...)`-Code durchreichen (`query_canceled`/`timeout`). |
| 5 | Niedrig | Doku stale: `W-P11-backend.md` nennt `api-v0.11.0.json` SHA `5855e974…` (Codex-Stand) und „API 0.12 vorläufig“; richtiges SHA und Nummer stehen nur im Welle-2-Handoff. Handoff meldet Docker/Livetests noch als „nicht ausgeführt“. | `shasum`, Handoff Zeilen 55, 139. | Handoff auf Integrationsstand aktualisieren. |
| 6 | Info | Keine Begrenzung gleichzeitiger Exporte; bis 300 s halten Snapshot und Verbindung (SQLite: exklusiv). Nur durch Token-Inhaber auslösbar, API-weit gleiches Muster. | `cmd/db-manager/main.go`, kein Limiter. | Optional Concurrency-Cap. |
| 7 | Info | Bekannte, dokumentierte Lücken: Views/Trigger/Routinen, PostgreSQL-Sequenzen, SQLite `sqlite_sequence`, NaN/Infinity (Export bricht ehrlich ab). | Handoff, Code. | Folgepakete. |

## Geprüft, ohne Befund

- **Treue (Round-Trip):** SQLite: Tabelle mit Namen aus `"`, `` ` ``, `]`, Newline, `;DROP TABLE victim;--`, Spalte mit Newline/`--`/Quote, Default mit eingebettetem `DROP`, Werte (NULL, leer, `\N`, `base64:`, Newline+`;`, `*/`, `$$`, Emoji, Backslash, Steuerzeichen), Blob, INTEGER-Extreme, REAL (1e300, 5e-324, 1.79e308), Index, Mehrzeilen-INSERT und Einzel-INSERT, GZip: Restore per unabhängigem `sqlite3.executescript` in DB mit `victim`; Zeilen **und `typeof` je Zelle** gleich, Indexe gleich, `victim` unversehrt. PostgreSQL 18.6 und MySQL 8.4 (Docker): Restore per `psql -f` bzw. `mysql <` (volle Mehrfachanweisungs-Semantik, also strenger als der Go-Treiber) in Datenbank mit `victim`: keine fremde Ausführung, Daten gleich (Dezimal 40,20, bytea/blob, timestamptz, double, jsonb/json, NULL, `\N`).
- **Injektion:** Werte (SQLite `''`, MySQL `CONVERT(X'..' USING utf8mb4)`, PostgreSQL `E'..'` mit `\\` und `''`, NUL abgewiesen), Namen (`quoteIdentifier` mit Verdopplung), Header fest (keine Pfade/Hosts/Zugangsdaten/Zeitstempel), Dateiname per Regex `^[A-Za-z0-9][A-Za-z0-9_.-]{0,119}$`: `a"b`, CRLF, `../x`, `.h`, Leerzeichen, `;`, 121 Zeichen, Umlaut alle 400; Header-Injektion nicht möglich. Dialekt-Mismatch, unbekannte Felder, Format-Mismatch: 400.
- **Streaming/Limits:** 2 000 000 Zeilen (~230 MB roh, GZip 11 MB): Server-RSS 18 auf 29 MB (flach). Client-Abbruch: Folgeanfrage sofort, keine hängende Verbindung. Byte-, Zeilen- und Zeitlimit liefern abgeschlossene Zeilen plus `incomplete`-Trailer (kein halbes Statement). SQLite-`zeroblob(900000000)` als Abfrageexport: 0,1 s, RSS 15 MB (kein Speicherstoß). Gzip nur Ausgabe (keine Eingabekompression, keine Bombe).
- **Abfrageexport SQLite:** `load_extension`, `ATTACH`, `PRAGMA writable_schema`/`journal_mode`/`query_only`, `readfile`, `VACUUM INTO`, Mehrfachanweisung, DML, `WITH … DELETE` abgewiesen; Daten unverändert, keine Dateien angelegt. Lesende `pragma_*`-Tabellenfunktionen/`sqlite_master` wie bei `/api/v1/query` erlaubt.
- **Auth/Methoden:** GET auf Export 405, ohne/falsches Token 401 (auch Capabilities), falscher Content-Type 415, `Cache-Control: no-store`.
- **Konsistenz:** Header/JSON nennen ehrlich „per Datenbank“ bzw. „read-only best effort“; MySQL nur InnoDB (REPEATABLE READ/CONSISTENT SNAPSHOT), PostgreSQL REPEATABLE READ READ ONLY.
- **Vertrag:** Vorgänger v1 bis v10 byteidentisch gepinnt, v11 additiv, Schemas strikt; Mutationstests des Wächters (je Exit 1): Byte in v0.10 geändert (`check-export` und `check-sqlbatch`), `additionalProperties` in v0.11 gelockert, Fixture mit unbekanntem Feld. Echte Engine-Fixtures (`real-*.sql/csv`) vorhanden.

## Ausgeführte Läufe

`gofmt -l .` leer; `go vet ./...` ok; `go test -race -count=1 ./...` alle Pakete ok; alle elf Wächter `check-*-contract.py` ok;
`check-docs.py` 0 Fehler; `python3 scripts/test-export.py --live` Exit 0 (SQLite, MySQL, MariaDB, PostgreSQL, Schema- und CSV-Prüfung);
eigene Proben und Docker-Läufe wie oben (Skripte im Scratchpad, nicht eingecheckt).
