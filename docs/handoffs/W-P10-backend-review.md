# Review W-P10 Backend: Tabellenwartung (API 0.10)

Reviewer: unabhängig (nicht Autor, Codex-Paket). Geprüft: Integrationsbranch `integration/wave2-backend` (60592de), Merge a56c9f5, 25.10.2026.
Review-Branch: `review/wave2-a` (nur Tests und Review-Dokumente). Schwerpunkt: Bestätigung, Systemtabellen, Sperren und Zeitlimits.

## Urteil

Freigabe (accept). Keine Bestätigungsumgehung, keine Injektion, keine Wartung auf Systemtabellen, keine rohen Diagnosen. Nur niedrige Hinweise.

## Ausgeführt

| Prüfung | Ergebnis |
| --- | --- |
| `gofmt -l .`, `go vet ./...`, `go test -race -count=1 ./...` | sauber, alle Pakete ok |
| alle elf `check-*-contract.py` | bestanden (maintenance: v1 bis v9 byteidentisch, v10 additiv, 83 Fixtures) |
| SHA-256 `contracts/api-v0.10.0.json` | `077a0e02...55cc5`, stimmt mit Handoff |
| `scripts/test-maintenance.py` | bestanden, echte Antworten SQLite/MySQL/MariaDB/PostgreSQL validiert; keine Container übrig |
| `scripts/check-docs.py` | 0 Fehler |
| eigene Tests `wave2a_review_test.go` (`TestReviewW2MaintenanceSQLite/Live`) je System in eigenem Wegwerf-Container nacheinander | bestanden |
| Mutationstest Wächter | wirksam |

## Befunde

### L1 (niedrig): Systemschema-Prüfung nur auf der angefragten Schreibweise
Beleg: `maintenancePreview` prüft `item.Table.Schema` (klein geschrieben) und den Namenspräfix `sqlite_` vor `describe`; das aufgelöste Schema (`d.Table.Schema`) wird nicht erneut geprüft (W-P08 tut das). Im Test sind alle Varianten abgewiesen (`mysql.user`, `MYSQL`, `information_schema`, `sys`, `performance_schema`, `pg_catalog.pg_class`, `PG_CATALOG`, `pg_toast`, leeres Schema mit `pg_class`/`pg_database`/`pg_authid`, SQLite `sqlite_master`/`SQLITE_schema`/`temp`), also heute kein Bypass. Fix (Tiefenverteidigung): dieselbe Prüfung nach `describe` auf `d.Table.Schema` wiederholen.

### L2 (niedrig): MySQL-Wartung ohne serverseitiges Zeitlimit
Beleg: Sitzung setzt `lock_wait_timeout`; für `OPTIMIZE`/`REPAIR`/`CHECKSUM` gibt es kein serverseitiges Statement-Limit. Ein Kontextabbruch schließt die Verbindung, der Server kann weiterarbeiten. Handoff und Plan-Warnung sagen das („Ein Abbruch garantiert keinen Rollback"). PostgreSQL setzt `statement_timeout` und `lock_timeout` auf der einzigen Verbindung (Pool = 1), SQLite begrenzt über den Kontext. Kein Fix nötig; optional `max_execution_time` für CHECKSUM/CHECK in MySQL (nur SELECT-artig wirksam).

### L3 (niedrig): CHECK/ANALYZE/CHECKSUM nicht „typed"
Beleg: Diese Aktionen scannen oder sperren Tabellen kurz, verlangen aber nur Aktion und frischen Hash, nicht den getippten Namen (`typed_confirmation=true` nur für optimize, repair, reindex und `vacuum full`). Vertretbar (nicht zerstörend), die Plan-Warnung nennt Sperren/Laufzeit. Webpaket sollte für `checksum` großer Tabellen den Hinweis hervorheben.

### Informativ
- Replay: Wartung ist idempotent; Wiederholung derselben Bestätigung führt die gleiche Wartung erneut aus (kein Datenverlust). Kein zusätzlicher Schutz nötig.
- Teilausführung: Stapel verarbeitet Elemente nacheinander, `partial` ist gesetzt; nach Ablauf des gemeinsamen Zeitlimits folgen `canceled`-Ergebnisse. Ehrlich dokumentiert.

## Geprüft ohne Befund
1. Bestätigung: für typed-Aktionen (optimize, reindex) ohne/falscher/großgeschriebener/mit Leerzeichen getippter Name, falsche Aktion, Nullhash, Großbuchstaben-Hash, leerer Hash: alle `rejected` mit `confirmation_mismatch`/`plan_mismatch`, Tabelle unverändert; korrekte Bestätigung `completed`. Nicht-typed (analyze) mit falscher Aktion oder falschem Hash abgelehnt. Zielname ist der Katalogname aus `describe`; Schreibweisen der Anfrage lösen nicht auf. Der HTTP-Pfad verlangt Hash und `confirm.action` je Element.
2. Injektion: Tabellenname mit Backtick, Quotes, `;DROP TABLE` und Schemaqualifizierung (MySQL, MariaDB, PostgreSQL, SQLite `"m""x'y;--"`): Plan, Bestätigung und Ausführung `completed`, Nachbartabelle unverändert, Ergebnis ohne Anweisungs- oder Diagnosetext. Es gibt keine freien Optionen; `full`/`analyze` nur bei `vacuum` (sonst abgewiesen).
3. Radius: Sichten, PostgreSQL-Partitionen/-Vererbung (`rv_pp`, `rv_pp1`, `rv_ip`, `rv_ic`), REPAIR auf InnoDB und alle Systemtabellen werden nicht geplant. Stapel auf 50 Elemente und 60 s begrenzt (51 Elemente und Zeitlimit > 60 s abgelehnt).
4. Fähigkeiten: SQLite nur check/analyze (VACUUM, reindex, optimize, repair `unsupported_action`; VACUUM unter ADR-0004 gesperrt), MySQL/MariaDB engineabhängig, PostgreSQL analyze/vacuum/reindex ohne Vererbung/Partitionen und mit Rechteprüfung. Neue SQLite-Verbindungsart `OpenMaintenance` erlaubt `integrity_check` nur mit Argument; die normale Abfrageverbindung verweigert `PRAGMA integrity_check` weiterhin (Test).
5. Fehlermeldungen: Ergebnis trägt feste Gründe; Serverdiagnosen von CHECK/REPAIR werden nie weitergegeben (`error`/`warning` ergibt `outcome_unknown`).
6. Verträge: SHA stimmt; v1 bis v9 byteidentisch, v10 additiv. Mutationstest: `outcome` auf ungültigen Wert, zusätzliches Feld (`raw_error`, `raw_sql`) in echten Fixtures, angehängtes Byte an v8 und entfernte Strikt-Eigenschaften in v9/v10 lassen den Wächter fehlschlagen. Auth: alle drei v10-Routen hinter Bearer (HTTP-Test 401), POST/GET wie im Vertrag, Body 1 MiB.
7. Doku: Handoff und Integrationshandoff stimmen mit dem Verhalten überein (Abbruch ohne Rollback-Garantie, Teilausführung, SQLite-Grenzen).
