# Handoff W-P07 Backend: Datenbank- und Tabellenoperationen (API 0.8)

Status: umgesetzt auf Branch `feat/w-p07-backend` (Basis main `5921a9d`), nicht gemergt, unabhängiger Review ausstehend.
Der Vertrag ist nicht im Web-Lock gepinnt. Der Vertragsindex `contracts/README.md` ist unverändert (Lead).

## Ergebnis

- Vertrag: `contracts/api-v0.8.0.json` (SHA-256 `74aa39ed06311f2c45afc274a2412a0ac30912e3a5683ec96b812fd473a28264`,
  additiv zu v7, v1 bis v7 byteidentisch), Verhaltensvertrag `contracts/OBJECT-OPS-API.md`, Fixtures
  `contracts/fixtures/object-ops/` (80 Dateien: echte Engine- und HTTP-Antworten von SQLite, MySQL 8.4, MariaDB 12.3,
  PostgreSQL 18.6, gültige und ungültige Anfragen, absichtlich ungültige Antworten), Wächter
  `scripts/check-object-ops-contract.py` (Bytes v1 bis v7, Additivität, Plan/Ausführen-Schemaregeln, Referenzen,
  Fixtures mit eigenem kleinen Validator; `validate()` ist importierbar).
- Routen `/api/v8/`: `capabilities`, `tables/dependencies` sowie je Aktion `<pfad>/plan` und `<pfad>` für `databases/create`,
  `databases/drop`, `databases/collation`, `tables/rename`, `tables/copy`, `tables/truncate`, `tables/drop`, `tables/options`
  (`internal/httpapi/objectops.go`, Registrierung als letzter Eintrag in `featureList`). Statuscodes: 400
  `invalid_object_op`/`confirmation_mismatch`, 404 `object_not_found`, 409 `object_exists`/`plan_mismatch`, 422 `unsupported_action`.
- Core: `internal/core/objectops.go` (Typen, Grenzen, statische Fähigkeitstabelle über `dbplan.Capabilities`).
  Datenbank: `internal/database/objectops.go` (Validierung, Sitzung, Prüfung, Ausführung, Fehlerklassifikation),
  `objectops_plan.go` (Planbau je Aktion und Dialekt), `objectops_{mysql,postgres,sqlite}.go` (Katalogabfragen,
  Abhängigkeiten, Optionen), `objectops_observe.go` (Rücklesen).
- Muster: Zielobjekte werden aus dem Katalog aufgelöst (`describe`, `information_schema`, `pg_database`); `dbplan.Check` erhält nur
  diesen Katalognamen als Zielname (exakte Schreibweise verlangt, sonst `object_not_found`). Zerstörend mit getipptem Namen:
  `database_drop`, `table_rename`, `table_truncate`, `table_drop`. Verbindungsdatenbank (MySQL/MariaDB) verlangt
  `confirm.acknowledge_current_database`. Kein CASCADE, keine Abschaltung der Fremdschlüsselprüfung, kein freies SQL.
- Ausgang: `completed` (PostgreSQL/SQLite Tabellen-DDL mit Rücklesen in derselben Transaktion vor dem Commit; sonst getrennt
  beobachtet), `rejected` (mit Grundcode aus fester Liste, nie rohe Treiberfehler), `outcome_unknown` ohne Retry (Verbindungs-/
  Zeitverlust, Commit-Fehler, MySQL-Mehrschrittfolge nach wirksamer erster Anweisung).
- Sitzung: MySQL/MariaDB `NO_BACKSLASH_ESCAPES` und `STRICT_ALL_TABLES` (Literale in der Vorschau sind exakt die ausgeführten),
  `lock_wait_timeout=10`; PostgreSQL `standard_conforming_strings=on`, `lock_timeout=10s`.

## Prüfungen (Worktree `.worktrees/w-p07-backend`, 24.10.2026, Host-Last 20 bis 60)

| Befehl | Ausgang |
| --- | --- |
| `gofmt -l .`, `go vet ./...` | ohne Ausgabe |
| `go test -race -count=1 ./...` | alle Pakete ok |
| `python3 scripts/check-{table,mutation,insert,structure,create,overview}-contract.py` | je "Frozen ... passed" |
| `python3 scripts/check-object-ops-contract.py` | Frozen v1-v7 bytes, additive v8 ..., 80 fixtures passed |
| `python3 scripts/test-object-ops.py` | SQLite + HTTP-Test, dann nacheinander je ein Wegwerf-Container (Präfix `dbmanager-w-p07-`): MySQL 8.4, MariaDB 12.3, PostgreSQL 18.6 (TLS verify-full), je `TestLiveObjectOps/<system>` PASS; 44 echte Antworten gegen das v8-Schema validiert; danach keine Container mit dem Präfix |
| `python3 scripts/check-docs.py` | 573 files checked; 0 errors |

Abgedeckt je System (unabhängiges Nachlesen über `information_schema`/`pg_class`/`sqlite_schema` und Direktabfragen):
Validierung (fremdes/fehlendes Feld, unbekanntes Objekt, Injektionsnamen mit Quote/Backtick/Semikolon, Sicht statt Tabelle, Systemobjekt,
Namen zu lang/mit Steuerzeichen/NUL), Abhängigkeiten (Fremdschlüssel und Sicht), Plan-Stabilität (gleicher Hash), falsche Aktion/falscher
oder fehlender getippter Name/veralteter oder ungültiger Hash, Fremdschlüssel verhindert Löschen/Leeren (Tabelle bleibt unverändert),
Löschen mit Sondernamen (Nachbartabellen bleiben), Leeren mit Zählerfolge (MySQL 1, PostgreSQL 4 bzw. 1 mit `restart_identity`, SQLite 4),
Umbenennen mit Sonderzeichen (Fremdschlüssel folgt), Verschieben und Kopieren in ein anderes Schema, Kopie `structure`/`data`/`both` mit
Zeilenzahl und Prüfsumme, Tabellenoptionen (Engine, Kommentar mit Quote/Backslash/SQL-Text, Kollation, Auto-Inkrement; ungültige Engine/
Kollation, Injektion), Datenbank anlegen mit Kollation und Sondernamen, doppelt/Systemname/falscher Name, Löschen nur der Zieldatenbank,
Verbindungsdatenbank mit Zweitbestätigung, Fremdschlüssel aus anderer Datenbank verhindert das Löschen (MySQL/MariaDB), Rechte (SELECT-Rolle:
jede Aktion `rejected/permission_denied`, Fixture unverändert, keine Namen/Passwörter in Meldungen), unbekannter Ausgang (getrennte
Verbindung, SQLite) und abgebrochener Kontext (nichts gesendet).

## Einschränkungen und offen

- Nicht umgesetzt (im Plan als Matrixzeilen genannt, im Backendumfang des Plans nicht aufgeführt): `OBJ-004` Datenbank kopieren/umbenennen
  und der Teil von `OBJ-005`, der bestehende Tabellen mit umstellt (dafür `table_options` mit `collation_mode=convert` je Tabelle).
  Datenbank kopieren/umbenennen braucht einen mehrstufigen Plan über alle Objekte; Entscheidung des Leads, ob eigenes Paket.
- SQLite: keine Datenbankaktionen (verwaltete Kennung), keine Tabellenoptionen, kein Verschieben; Kopie vereinfacht (`CREATE TABLE AS`).
- PostgreSQL: `database_collation` nicht möglich; Kollation bei Anlage nur libc-Locales; Inhalt einer anderen Datenbank nicht einsehbar;
  Identity-/serial-Zähler einer Kopie werden nicht nachgeführt (Warnung).
- Sichten werden in MySQL/MariaDB/SQLite am Quelltext erkannt (`certainty=textual`, nur mit dem Recht SHOW VIEW lesbar); Trigger nicht
  ausgewertet. Die Sichtsuche scannt alle Sichten des Servers und ist durch das Zeitlimit begrenzt.
- MySQL/MariaDB: Mehrschrittfolgen (Kopie `both`) sind nicht atomar; ein Fehlschlag nach der ersten Anweisung wird als `outcome_unknown`
  mit `statements_executed` gemeldet, die angelegte leere Kopie bleibt bestehen.
- `plan_sha256` deckt Treiber und Anweisungen ab, nicht Warnungen und Abhängigkeiten (Hinweis aus dem W-P01-Review).
- `internal/httpapi/features_test.go`: nur die Zählung (8) und der Negativpfad (`/api/v9/capabilities`) geändert; jedes Folgepaket
  ändert diese Zahl, Merge-Konflikt vom Lead zu lösen.
- Review (Sicherheitsfokus: destruktive Pfade, Bestätigungsumgehung, Injektion) und Webpaket W-P07-web stehen aus. Ein Sperrentest
  (Zeitüberschreitung `lock_timeout` bei fremder Sperre) wurde nicht ausgeführt.
