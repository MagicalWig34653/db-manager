# W-P15 Backend Review (SQL-Stapel, EXPLAIN, DML-Simulation, API 0.12)

Stand 25.10.2026. Reviewer: unabhängig (nicht Autor). Geprüft: `integration/wave2-backend` (60592de), Merge c562a61,
Vertrag `contracts/api-v0.12.0.json` SHA-256 `7f0446edccdc368a202e4eaae5689d4b44d641e34bdf77ec8314de9f3e3f2dcd`.
Review-Worktree `review/wave2-b`; Integrationsbranch unverändert. Docker nur Wegwerf, nacheinander, aufgeräumt.

## Urteil

**Freigabe (Approve), keine Blocker.** Kein Statement-Schmuggel nachweisbar, Simulation rollt wirklich zurück, Sandbox nicht
aufgeweicht. Hinweise unten (niedrig) betreffen Nutzbarkeit und Dokumentation.

## Befunde

| # | Schwere | Befund | Beleg | Fix |
| --- | --- | --- | --- | --- |
| 1 | Niedrig | Der Splitter trennt an `;` auch innerhalb `BEGIN … END` (SQLite-Trigger, PostgreSQL `BEGIN ATOMIC`); der Rest `END` fällt in die Sperrliste, der Stapel wird vor jeder Ausführung mit 422 `sql_not_supported` abgelehnt (fail-closed). Abhilfe ist `DELIMITER //` (funktioniert auch für SQLite/PostgreSQL, belegt). | Probe SQLite: `CREATE TRIGGER … BEGIN …; END;` → 422; mit `DELIMITER //` → ausgeführt. | In Vertrag/UI nennen (Hinweis auf DELIMITER) oder `BEGIN…END` für SQLite-`CREATE TRIGGER` im Splitter erkennen. |
| 2 | Niedrig | Es gibt nur eine globale Zustimmung (`confirm:true`) je Anfrage, keinen Lesemodus und keine Klassifikation je Statement; `EXPLAIN ANALYZE <DML>` über die **Batch**-Route führt das Statement aus (PostgreSQL: `DELETE` löschte 2 Zeilen). Das entspricht dem ADR (Nutzer-SQL mit echten Rechten und Zustimmung, nicht schwächer als `/api/v1/query`, das auf SQLite ebenfalls schreibt), aber der Capability-Wert `explain_analyze:false` kann missverstanden werden. | Live PostgreSQL: `EXPLAIN ANALYZE DELETE FROM t` über `/sql/batch`, danach Zähler 0. Die `explain`-Route lehnt `ANALYZE`, `FORMAT`, `(…)`, `/*!` ab und hat nie Seiteneffekte (belegt, siehe unten). | Vertragstext: „`explain_analyze` gilt nur für die Route `explain`; der Stapel führt jedes Statement aus.“ UI: Schreibwarnung unabhängig vom Statementtyp. |
| 3 | Niedrig | Nutzbarkeit MySQL/MariaDB: `ANALYZE/OPTIMIZE/CHECK/REPAIR TABLE` werden als Exec behandelt, die Ergebniszeilen (Status) gehen verloren (`rows_affected: 0`); führendes `(SELECT …)` und `/*!…*/` werden abgelehnt. Fehler in sicherer Richtung. | Live MySQL 8.4: `ANALYZE TABLE t` → completed ohne Resultset. | Schlüsselwörter zur Liste der Zeilenlieferanten ergänzen. |
| 4 | Niedrig | Sperrliste (BEGIN/START/COMMIT/END/ROLLBACK/SAVEPOINT/RELEASE/SET) kennt `ABORT` (PostgreSQL), `LOCK TABLES`, `PREPARE TRANSACTION` nicht. Wirkungslos, weil jede Anfrage eine eigene Verbindung nutzt und im Autocommit endet; als „ergonomische Einschränkung“ dokumentiert. | Live PostgreSQL `ABORT` completed (No-op), MySQL `LOCK TABLES` completed. | Optional ergänzen. |
| 5 | Niedrig | Simulation, die in das Zeitlimit läuft, meldet `outcome_unknown` + `rollback_unknown`, obwohl SQLite beim Interrupt zurückrollt (Daten unverändert, belegt). Ehrlich, aber konservativ. | Probe mit 3 s-Limit. | Info. |
| 6 | Niedrig | Doku stale: `W-P15-backend.md` und ADR nennen „API0.16“ und SHA `dd2d5e0e…`; richtig sind 0.12 und `7f0446ed…` (nur im Welle-2-Handoff). | Handoff Z. 39 bis 40, ADR Z. 16. | Handoff und ADR korrigieren. |
| 7 | Info | `json.Marshal(out)` je Statement (Reserveprüfung) ist O(n²) bis 100 Statements × 8 MiB; durch Grenzen beschränkt. PostgreSQL-Hinweise werden nicht erfasst (dokumentiert). | Code. | Optional Größe inkrementell zählen. |

## Geprüft, ohne Befund

- **Splitter (`internal/sqlsplit`):** Neuer Differenztest gegen SQLites eigenen Tokenizer (Prepare-Schleife): 100 606 vergleichbare Eingaben (zufällige Token-Folgen und Fragmente mit Strings, `''`, `"`, `[]`, Backticks, `--`, `/* */`, `\` in SQLite-Strings, Unicode, Newlines): **0 Fälle, in denen der Splitter akzeptiert, aber anders zählt**; 93 strengere Ablehnungen (z. B. unterminierte `/*`, die SQLite duldet) sind sicher. Als `internal/sqlsplit/sqlite_differential_test.go` im Review-Branch abgelegt (0,5 s). PostgreSQL live: `'a\'` (Standard-String), `E'a\'; SELECT 2;--'`, Dollar-Quoting, verschachtelte Kommentare werden wie vom Server gelesen; MySQL live: `\'`, `#`-Kommentare, Backtick-Fall (Splitter schluckt mehr, Server lehnt Mehrfachanweisung ab). **Kein Schmuggel möglich**, weil jeder Treiber einzelne Statements erzwingt: SQLite `Prepare`-Rest muss leer sein, MySQL `MultiStatements=false`, PostgreSQL erweitertes Protokoll. Fehlparsing bei abweichendem Modus (`NO_BACKSLASH_ESCAPES`, `ANSI_QUOTES`, `standard_conforming_strings=off`) kann höchstens die Aufteilung des eigenen SQL ändern; im Vertrag als Lexerannahme genannt.
- **Modi:** `stop` bricht nach Fehler ab (Rest `not_executed`), `continue` setzt fort; Fehler konservativ `outcome_unknown`; Zustimmung: ohne `confirm`/`confirm:false`/unbekanntes Feld 400, ohne Token 401, für Batch, Explain und Simulate. 101 Statements, `max_rows` 10001, `timeout_ms` 60001: 400. Zeitlimit: `query_canceled`. 10 000-Zeilen-Grenze: `truncated`, Stapel stoppt konservativ.
- **EXPLAIN:** nur `SELECT/WITH/VALUES/INSERT/UPDATE/DELETE`, genau ein Statement; Präfix fest. SQLite, PostgreSQL und MySQL live: `EXPLAIN DELETE/UPDATE/INSERT` ohne Wirkung (Zeilenzahl unverändert), PostgreSQL `nextval` per EXPLAIN verändert die Sequenz nicht; `ANALYZE`, `PRAGMA`, `EXPLAIN …`, `(SELECT …)`, `/*!…*/` abgelehnt.
- **Simulation:** nur SQLite, nur einzelnes `INSERT/UPDATE/DELETE` ohne `RETURNING`, `WITH`, `REPLACE`, Mehrfachanweisung, `ATTACH`, `DROP`. DML wird tatsächlich ausgeführt (`rows_affected` 6, Trigger feuert) und danach zurückgerollt: Zeilenzahl der Tabelle und der Audit-Tabelle sowie `sqlite_sequence` danach unverändert, auch nach Fehlern und Zeitlimit; keine Dateien (`ATTACH`-Ziel) angelegt. MySQL/MariaDB/PostgreSQL: 422 `simulation_not_supported` vor DB-Zugriff.
- **SQLite-Sandbox nicht aufgeweicht:** über die Batch-Route abgelehnt (Auth-Fehler 23 aus der Sandbox): `ATTACH`, `DETACH`, `VACUUM`, `VACUUM INTO`, `PRAGMA writable_schema/journal_mode/foreign_keys/user_version/schema_version`, `load_extension`, `writefile`, `pragma_function_list`, `CREATE VIRTUAL TABLE`; keine Dateien entstanden. Erlaubt wie bisher: Positivliste-PRAGMAs, `CREATE TEMP TABLE`, `DROP TABLE` (Nutzerrechte; eine gelöschte Audit-Tabelle ließ den Trigger später fehlschlagen, erwartetes Verhalten).
- **Fehler/Limits:** nur feste Codes (`database_error`, `server_code`), keine rohen Treibertexte oder Pfade; Warnungen MySQL nur Code ohne Rohtext.
- **Vertrag:** v1 bis v11 byteidentisch gepinnt (v7 bis v11 explizit im Wächter, v1 bis v6 über Overview-Wächter), v12 additiv und strikt; Mutationstests (je Exit 1): `/api/v11/export` aus v12 entfernt, `additionalProperties` von `SQLBatchRequest` gelockert, Fixture mit unbekanntem Feld, Byte in v0.10 geändert. Echte Engine-Fixtures (`real-*`) vorhanden.

## Ausgeführte Läufe

`gofmt -l .` leer; `go vet ./...` ok; `go test -race -count=1 ./...` alle Pakete ok (inkl. `sqlsplit`, `sqlitesandbox`); elf Wächter ok; `check-docs.py` 0 Fehler;
`python3 scripts/test-sqlbatch.py` Exit 0 (SQLite, MySQL, MariaDB, PostgreSQL, 10 echte Antworten gegen v12 validiert); eigene SQLite-HTTP-Proben und Live-Proben (PostgreSQL 18.6, MySQL 8.4) wie oben.

## Nachreview (Autorenstand 675adcc, in `review/wave2-b` gemergt, 25.10.2026)

Geprüft: `git diff 60592de..origin/integration/wave2-backend` für W-P15; Vertrag unverändert (SHA `7f0446ed…`).

**Urteil: Freigabe.** Befunde 1 bis 6 sind behoben bzw. dokumentiert, keine neuen Befunde.

| Prüfung | Ergebnis |
| --- | --- |
| (e) `DELIMITER //` für `BEGIN…END` | SQLite live: `CREATE TRIGGER … BEGIN …; END//` mit Direktive läuft (Trigger feuert); ohne Direktive weiter 422 (fail-closed, im Vertrag dokumentiert). |
| `ABORT`, `LOCK`, `UNLOCK` | SQLite, PostgreSQL und MySQL: 422 `sql_not_supported` vor jeder Ausführung. |
| MySQL `ANALYZE/OPTIMIZE/CHECK/REPAIR/CHECKSUM TABLE` | Live MySQL 8.4: Ergebniszeilen werden geliefert (Msg_type/Msg_text bzw. Prüfsumme). Die Zustimmung bleibt unberührt: ohne `confirm` und mit `confirm:false` 400; nur die Wahl Query statt Exec ändert sich. |
| Simulation | SQLite: normale Simulation `rolled_back` (`rows_affected` 2, Trigger-Tabelle danach unverändert); Simulation mit Zeitlimit `rolled_back` mit Warnung `query_canceled`; Daten danach unverändert. `rollback_unknown` bleibt für einen tatsächlich nicht bestätigten Rollback. |
| Splitter-Differenztest | `go test -race ./internal/sqlsplit` grün (inkl. `TestSplitMatchesSQLiteTokenizer`). |
| Läufe | `gofmt` leer, `go vet`, `go test -race -count=1 ./...` ok, elf Wächter ok, `check-docs` 0 Fehler, `test-sqlbatch.py` Exit 0 (vier Systeme). |
