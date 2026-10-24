# Handoff W-P06 Backend: Katalog, Übersicht, Definition, Serverinfo (API 0.7)

Status: umgesetzt auf Branch `feat/w-p06-backend` (Basis main `02a30db`), nicht gemergt, unabhängiger Review ausstehend.
Der Vertrag ist nicht im Web-Lock gepinnt. Der Vertragsindex `contracts/README.md` ist unverändert (Lead).

## Ergebnis

- Vertrag: `contracts/api-v0.7.0.json` (SHA-256 `c6fd8c91315e25185d949d309397c7c0d80cc67fcab87ae521e38068e7004d17`,
  additiv zu v6, v1 bis v6 byteidentisch), Verhaltensvertrag `contracts/OVERVIEW-API.md`, Fixtures
  `contracts/fixtures/overview/` (32 Dateien: echte Engine-Antworten von SQLite, MySQL 8.4, MariaDB 12.3,
  PostgreSQL 18.6, gültige und ungültige Anfragen, absichtlich ungültige Antworten), Wächter
  `scripts/check-overview-contract.py` (Bytes v1 bis v6, Additivität, Referenzen, Fixtures mit eigenem kleinen
  Schema-Validator; `validate()` ist importierbar).
- Routen `/api/v7/{capabilities,databases,objects,definition,server-info}`: `internal/httpapi/overview.go`,
  Registrierung als letzter Eintrag in `featureList`. Statuscodes: 400 `invalid_overview`, 404 `object_not_found`, sonst wie bisher.
- Core: `internal/core/overview.go` (Typen, Grenzen, statische Treiberfähigkeiten über `dbplan.Capabilities`).
- Datenbank: `internal/database/overview.go` (Validierung, Zeit-/Größenlimits, exakte Zählung) und je Dialekt
  `overview_{sqlite,mysql,postgres}.go`. MariaDB läuft als `mysql`, `variant=mariadb` in server-info.
- Schätzung vs. exakt: `rows.state` estimated/exact/unknown; `estimated` in der Datenbankliste. Exakte Zählung nur mit
  `exact_rows`, höchstens 100 Objekte, eigenes Zeitbudget, Teilergebnis mit `exact_incomplete`.
- Sicherheit: nur sichtbare Objekte (information_schema bzw. PostgreSQL mit derselben Rechteprüfung), Werte gebunden,
  Identifier für SHOW CREATE/COUNT aus dem Katalogtreffer und `dbplan.QualifiedName`; kein Passwort, keine Pfade;
  Listen auf 1000 begrenzt, Definition 1 MiB, Antwort 8 MiB.
- Kein `dbplan.Check` nötig (nur lesend).

## Prüfungen (Worktree `.worktrees/w-p06-backend`, 24.10.2026, Host-Load 150 bis 190)

| Befehl | Ausgang |
| --- | --- |
| `gofmt -l .`, `go vet ./...` | ohne Ausgabe |
| `go test -race -count=1 ./...` | alle Pakete ok |
| `python3 scripts/check-{table,mutation,insert,structure,create}-contract.py` | je "Frozen ... passed" |
| `python3 scripts/check-overview-contract.py` | Frozen v1-v6 bytes, additive v7 ..., 32 fixtures passed |
| `python3 scripts/test-overview.py` | MySQL 8.4, MariaDB 12.3, PostgreSQL 18.6 (TLS verify-full) und SQLite: `TestSQLiteOverview`, `TestLiveOverview/{mysql,maria,postgres}`, `TestOverviewValidation` PASS; 20 echte Antworten gegen das v7-Schema validiert; danach keine Container mit dem Präfix |
| `python3 scripts/check-docs.py` | siehe Bericht |

Abgedeckt je System (gegen Direktabfrage: `SHOW CREATE`, `sqlite_schema`, `VERSION()`/`SHOW server_version`): leeres Schema,
27 Objekte mit Paging (10er-Seiten, keine Duplikate), Tabelle vs. View, Sondername (`we"ird \`na'me;DROP TABLE x`),
Schätzung vs. exakt (3 Zeilen), ungültige Grenzen, nicht gefundene Namen und Injektionsversuche, Verbindungsecho ohne
Passwort, Rolle ohne Rechte (leere Liste, 404, Serverinfo ok), falsches Passwort ohne Echo.

## Einschränkungen und offen

- SQLite: Datenbankgröße nicht verfügbar (die Sandbox verbietet PRAGMA page_count; `internal/sqlitesandbox` bewusst nicht geändert).
- PostgreSQL: Objektzahl nur für die verbundene Datenbank; Definition ist rekonstruiert (`derived=true`) und lässt Rechte, Kommentare,
  Trigger, Storage-Parameter und Partitionen aus.
- `information_schema`-Statistiken (MySQL/MariaDB) können veraltet sein (`information_schema_stats_expiry`); daher als Schätzung markiert.
- Eine Zählung läuft ohne `KILL` nur bis zum Kontext-Abbruch; der Server kann sie kurz weiterführen (Verbindung wird geschlossen).
- `internal/httpapi/features_test.go` zählt die Features (jetzt 7) und listet v7-Routen; jedes Folgepaket ändert diese Zahl, Merge-Konflikt vom Lead zu lösen.
- Review (Sicherheitsfokus: Informationslecks, teure Abfragen) und Webpaket W-P06-web stehen aus.
