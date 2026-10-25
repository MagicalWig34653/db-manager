# Handoff W-P15: SQL-Stapel, EXPLAIN und DML-Simulation

Stand: 24.10.2026. Implementierungskandidat auf `feat/w-p15-backend`, Basis/HEAD
`5921a9dc1b813cc63b212fa5579f813b1595b761`. Änderungen ausschließlich als Dateien im
zugewiesenen Worktree `.worktrees/w-p15-backend`; kein Git-Schreibbefehl, Commit,
Push, Merge oder Webpin. Lead übernimmt Commit und Integration.

## Ergebnis und Dateibesitz

- `internal/core/sqlbatch.go`: additive Request-/Result-/Warning-Typen, optionale
  `SQLBatchEngine`-Schnittstelle, vorläufige Vertragsversion 0.12.0.
- `internal/database/sqlbatch.go`, `sqlbatch_test.go`: eine Verbindung je Stapel,
  Stop/Fortsetzen, begrenzte Resultsets pro Statement, direkte Änderungszähler soweit
  verfügbar, Warncodes, feste Fehlercodes, Gesamtzeit-/Antwortgrenzen, Abbruch,
  EXPLAIN je Dialekt, SQLite-Simulation mit explizitem Rollback.
- `internal/httpapi/sqlbatch.go`, `sqlbatch_test.go`: authentifizierte neue
  `/api/v12/capabilities`, `/api/v12/sql/batch`, `/api/v12/sql/explain`,
  `/api/v12/sql/simulate`. Bestätigung `confirm: true` für jede Ausführung.
  `features.go`: genau ein angehängter Registrierungseintrag;
  `features_test.go`: ausschließlich Erwartungszahl 7 → 8.
- `internal/sqlsplit/split.go`, `split_test.go`, `testdata/fuzz/FuzzSplit/96e176437a9faebc`:
  eigenständiger begrenzter Splitter mit Dialekten, Delimiterdirektiven, Strings,
  Kommentaren, verschachtelten PostgreSQL-Kommentaren und Unicode-Dollar-Tags;
  Unit-/Fuzztests und gespeicherte Fuzzregression für ausschließlich leeren Whitespace.
- `contracts/api-v0.12.0.json`, `contracts/SQL-BATCH-API.md`,
  `contracts/fixtures/sqlbatch/*`: additive strikte Schemas, 13 synthetische positive/
  negative Fixtures, ausdrücklich gekennzeichnete Platzhalter und Verhaltensgrenzen.
- `scripts/check-sqlbatch-contract.py`, `scripts/test-sqlbatch.py`,
  `.github/workflows/sqlbatch.yml`: Wächter, dbfix-gestützte SQLite-/4-DB-Suite,
  Validierung tatsächlicher Ergebnisantworten, CI-Vorbereitung ohne Publish.
- [Paket-ADR](../decisions/ADR-W-P15-sql-werkzeuge.md) und dieser Handoff.

Unverändert: alte Vertragsdateien/API-Routen einschließlich v3/v4, `sqlitesandbox`,
Abhängigkeiten/Lizenzinventare, zentrale Status-/Plan-/Vertragsindexdateien, Weblock,
Webquellen und Assets. Eigener neuer Code mit SPDX AGPL-3.0-only.

## Vertragsstand und bewusste Grenzen

[API0.16](../../contracts/api-v0.12.0.json), SHA256:
`dd2d5e0ea82c7d5b1e30052d1278110d67af3acfbd0a314dab03645356c58573`.
Nummer vorläufig; keine Freigabe/Freeze/Pinnung des Webclients.
Der Wächter sichert auch API0.7 als unveränderte Basis mit SHA256
`c6fd8c91315e25185d949d309397c7c0d80cc67fcab87ae521e38068e7004d17`
und über den bestehenden Wächter die eingefrorenen API0.1 bis 0.6.

[Verhaltensvertrag](../../contracts/SQL-BATCH-API.md) ist maßgeblich:

- Nutzer-SQL mit tatsächlichen DB-Rechten und ausdrücklicher Zustimmung, keine
  strukturierte Planaktion und kein behaupteter SQL-Lesemodus. SQL-Modi müssen zu
  den dokumentierten Lexerannahmen passen; der Splitter ist kein Grammatikparser.
- Höchstens 100 Statements/1 MiB SQL, 16 Sets je Statement, 1.000/10.000 Zeilen je
  Set, 10/60 Sekunden insgesamt, 1 MiB je Zelle und 8 MiB tatsächliche Gesamtantwort.
  Aktuelle zu große Ergebnisse werden verworfen, frühere Ergebnisse bleiben erhalten.
- Keine atomare Stapeltransaktion. Explizite Transaktions-/Sitzungssteuerung wird
  vorab abgewiesen; Routinen können trotzdem eigene Effekte haben. Fehler/Abbruch/
  begrenzte Ergebnisse sind konservativ `outcome_unknown`, Rest `not_executed` bei
  Stop oder Ressourcenabbruch. Kein automatischer Retry und keine Exactly-once-Zusage.
- `rows_affected` fehlt, wenn kein belastbarer Adapterzähler vorliegt (insbesondere
  Netzwerk-RETURNING/WITH/CALL). Fehlen bedeutet unbekannt. MySQL/MariaDB geben höchstens
  100 Warncodes mit festen Texten aus; PostgreSQL-Hinweise sind ausdrücklich nicht erfasst.
- EXPLAIN ohne automatisches ANALYZE. Keine universelle Nebenwirkungsfreiheit von
  DB-Planung/Funktionen behauptet; Zustimmung auch dort erforderlich.
- Simulation unterstützt einfache SQLite-INSERT/UPDATE/DELETE ohne RETURNING,
  WITH und virtuelle Tabellen: echte DML-Ausführung, direkte Zählung, danach Rollback.
  Lokale Triggeränderungen rollen mit zurück; kein späterer Commitplan. Fehlerhafter
  oder nicht bestätigter Rollback wird nicht als bestätigter Erfolg ausgegeben.
- MySQL/MariaDB/PostgreSQL weisen freie Simulation vor DB-Zugriff ausdrücklich ab:
  allgemeine Transaktionssicherheit lässt sich bei unbekannten Engines, Triggern,
  Sequenzen, Rewrite-/FDW-/externen Effekten nicht beweisen. Keine universelle
  Rücknahmegarantie und kein ungesicherter Ausführungsfallback.

Das Backendpaket ersetzt weder W-P16s Editor/Konsole noch einen vollständigen Linter.
PMA-SQL-002/003/004/006/013 haben begrenzte Backendnachweise; Netzwerk-Livenachweise,
PMA-NEU-SQL-003 über die reine Lexerprüfung hinaus und PMA-NEU-NAV-003s Weboberfläche
bleiben separate Gates. Keine vollständige phpMyAdmin-Parität behauptet.

## Tatsächliche Prüfungen

Alle Go-Befehle mit `GOCACHE=/tmp/dbmanager-w-p15-go-cache`, weil der normale
Benutzer-Cache durch die Sandbox gesperrt ist. Keine Abhängigkeiten geändert.

| Befehl | Ausgang |
| --- | --- |
| `gofmt -l internal/core/sqlbatch.go internal/database/sqlbatch*.go internal/httpapi/sqlbatch*.go internal/httpapi/features*.go internal/sqlsplit` | keine Ausgabe |
| `go vet ./...` | bestanden |
| `go test -race -count=1 ./...` | ausgeführt; bestehender `TestMySQLPerConnectionLoggerSuppressesServerInput` scheitert ausschließlich an verbotenem `listen tcp 127.0.0.1:0`; übrige Pakete erfolgreich |
| `go test -race -count=1 ./... -skip '^TestMySQLPerConnectionLoggerSuppressesServerInput$'` | bestanden; genau dieser eine bestehende Loopbacktest ausgenommen; Netzwerkfixturetests übersprungen mangels Ports |
| `go test -race -count=1 ./internal/database ./internal/httpapi ./internal/sqlsplit -run 'TestSQLBatch\|TestSQLiteSQLBatch\|TestUnicode\|TestKeyword\|TestSplit\|FuzzSplit'` | bestanden; echte SQLite-/HTTP-/Parserfälle |
| `go test ./internal/sqlsplit -run '^$' -fuzz FuzzSplit -fuzztime=10s -parallel=2` | bestanden nach Korrektur des gefundenen Whitespacefalls; letzter Lauf 318.018 Ausführungen, Regression gespeichert |
| `python3 scripts/test-sqlbatch.py --sqlite-only` | bestanden; vier echte SQLite-Stapel-/EXPLAIN-/Simulations-/HTTP-Capability-Antworten gegen v12 validiert; Netzwerkports im Kindprozess entfernt |
| `python3 scripts/check-create-contract.py`, `check-insert-contract.py`, `check-mutation-contract.py`, `check-overview-contract.py`, `check-sqlbatch-contract.py`, `check-structure-contract.py`, `check-table-contract.py` (je unter `scripts/`) | alle bestanden; v12 additiv, alte Hashes unverändert, 13 v12-Fixtures geprüft |
| `python3 scripts/check-docs.py` | bestanden: 506 Dateien, 0 Fehler |
| `git diff --check` | bestanden (read-only); neue Dateien zusätzlich auf nachgestellten Whitespace geprüft |

Ein zusätzlicher Race-Gesamtlauf parallel zum Fuzzing scheiterte im bestehenden
`TestCrashRecovery` an `child timeout`; die unveränderte Sandbox wurde nicht angepasst.
Der abschließende serielle Gesamtlauf mit genau einer Loopbackausnahme war grün,
einschließlich `TestCrashRecovery`; keine Sandbox-/Timeoutänderung vorgenommen.

SQLitefälle: Fehler an Position 3 mit Erfolg 1/2 und nicht gestartetem Rest;
Fortsetzung nach Fehler, Strings/Delimiter, betroffene Zeilen, EXPLAIN, unabhängiges
Nachlesen nach Simulation, Audittrigger-Rollback, fehlgeschlagene und abgebrochene
schreibende Simulation, Zell-/aggregierte JSON-/Zeilenlimits, Zeit-/Contextabbruch,
Connectionfreigabe, Sandbox-Ablehnungen ATTACH/VACUUM/PRAGMA/Extension, Fehler ohne
SQL-/Pfadleck, unbestätigte und unzulässige JSON-Anfragen, unveränderte v1-Batchablehnung.

Die zusätzliche Live-Suite ist geschrieben und kompiliert: MySQL/MariaDB/PostgreSQL
über `dbfix.py`, echte TLS-Fixtures, Stop/Fortsetzen/Änderungszähler/EXPLAIN/
Nichtunterstützung der Simulation, Nachlesen, Zeilenlimit, Deadline, MySQL-/MariaDB-
Warnungen und DELIMITER-Prozedur mit mehreren CALL-Sets, PostgreSQL-Dollar-Routine.

## Nicht ausführbar, Review und Übergabe

`docker info --format '{{.ServerVersion}}'` scheiterte am gesperrten Docker-Socket.
Daher **nicht ausgeführt**: `python3 scripts/test-sqlbatch.py` mit Netzwerkfixtures,
MySQL8.4-/MariaDB12.3-/PostgreSQL18.6-Livetests, Container-/Compose-/Swarm-Läufe,
Linux-amd64-/CI-Ausführung und Deploy-/Webintegration. Keine Dockerressource angelegt.
Der bestehende MySQL-Logger-Loopbacktest bleibt wegen des verbotenen TCP-Bindings
ungeprüft. Kein grüner uneingeschränkter Gesamtlauf oder grüner Netzwerk-Livetest behauptet.

Unabhängiger separater Read-only-Quellreview einschließlich Nachprüfung durchgeführt.
Gemeldete Befunde zu Unicode-Dollar-Tags, dialektabhängigen Kommentaren vor Transaktions-
kontrolle, RETURNING-Erkennung/Zählergrenzen, Rollbacktext, geerbten Fixtureports und
überschriebenen Ergebnisdumps korrigiert und nachgeprüft; keine weiteren Befunde gemeldet.
Dieser Quellreview ersetzt keine unabhängigen Netzwerk-/CI-/Integrationsgates.

Angeforderter `docs/handoffs/W-P07-backend.md` fehlt auf dieser Basis; keine fremde
Version erfunden. W-P12s `internal/sqlsplit` existierte ebenfalls noch nicht: Erstellung
ist durch den konkreten W-P15-Auftrag einschließlich Parser/Fuzz autorisiert, muss vor
Integration mit W-P12 abgestimmt werden. Shared-Dateien nur minimal verändert.

Nächster konkreter Schritt des Leads: uncommittierte Dateien prüfen/übernehmen,
vorläufige Nummer und gemeinsamen Splitter abgleichen, auf einem Host mit Docker und
Loopbackrecht die oben offenen Läufe ausführen, danach Review-/Integrationsgate und
separater Commit/Push. STATUS/ARBEITSPLAN/Vertragsindex und Webpin bleiben beim Lead.
Keine eigenen laufenden Jobs oder DB-Ressourcen verbleiben nach Abschluss.
