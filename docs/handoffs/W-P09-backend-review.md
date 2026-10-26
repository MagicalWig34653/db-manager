# Review W-P09 Backend: Indizes, Schlüssel, Fremdschlüssel (API 0.13)

Reviewer: unabhängig (nicht Autor, Codex-Paket). Geprüft: `integration/wave3-backend` (f345adb), Merge d8efbfc, Fixes 165534c, 26.10.2026.
Review-Branch: `review/wave3-b` (nur Tests und Review-Dokumente). Schwerpunkt: Bestätigung, Injektion, Datenverlust/Sperren, Treiberehrlichkeit, Systemobjekte.

## Urteil

Freigabe (accept), mit zwei niedrigen Auflagen (L1, L2) und Doku-Nachzug (L3). Keine Bestätigungsumgehung, keine Injektion, keine Lecks, kein Teilzustand gefunden.
Der PostgreSQL-Ausdrucksindex ist kein Freitext: nur Whitelist `lower`/`upper` auf Textspalten, Spalte aus dem Katalog, Prädikat nur `IS [NOT] NULL` (ADR-W-P09). Damit ist keine Plan-Pflicht-Sonderregel für Freitext nötig.

## Ausgeführt

| Prüfung | Ergebnis |
| --- | --- |
| `gofmt -l .`, `go vet ./...` | ohne Ausgabe |
| `go test -race -count=1 ./...` | alle Pakete ok (Last 50 bis 200) |
| alle `check-*-contract.py`, `check-docs.py` | bestanden (indexes: v1 bis v12 byteidentisch, v13 additiv, 60 Fixtures; 0 Docs-Fehler) |
| SHA-256 `contracts/api-v0.13.0.json` | `e05a2402...c3d7`, stimmt mit Handoff und WAVE3-INTEGRATION |
| `scripts/test-indexes.py` (SQLite, MySQL 8.4, MariaDB 12.3, PostgreSQL 18.6) | exit 0, keine Container übrig |
| eigene Tests `internal/database/wave3b_review_test.go` (`TestReviewW3BIndexes*`) auf SQLite und allen drei Netzwerkengines (eigene Wegwerf-Container nacheinander) | bestanden, Beobachtungen unten |
| Mutationstest Wächter | wirksam (siehe 6) |

## Befunde

### L1 (niedrig): Auto-Inkrement-Abhängigkeit und Datenverletzungs-FK ohne gezielte Vorwarnung
Beleg: MySQL/MariaDB `primary_drop` auf `id BIGINT AUTO_INCREMENT PRIMARY KEY` wird von der Engine abgelehnt, Ergebnis `rejected/database_error` ("Die Änderung wurde abgewiesen."); der Plan enthält nur die generischen Warnungen. Ebenso `foreign_key_create` auf verletzende Daten (Kind-Wert 99 ohne Elternzeile): Plan ohne Hinweis, Ausführung `rejected/database_error`, `StatementsExecuted:0`, kein Teilzustand.
Wirkung: sicher (Datenlage unverändert, keine Rohfehler), aber der Nutzer erfährt den Grund erst nicht. Fix: Plan prüft AUTO_INCREMENT-Spalte im PK (Warnung/Ablehnung `dependency_blocks`) und optional per `NOT EXISTS`-Zähler verwaiste FK-Werte (analog Duplikatzähler); Reason `invalid_value`/`dependency_blocks` statt `database_error`.

### L2 (niedrig): Warnungen ohne Rebuild-/Sperrhinweis je Aktion
Beleg: Pläne tragen drei feste Sammelwarnungen. `primary_set`/`primary_drop`/`index_alter` auf MySQL bauen die Tabelle ggf. neu auf (inplace/copy je Engine), PostgreSQL `CREATE INDEX` sperrt Schreibzugriffe (kein `CONCURRENTLY`). Das steht nur allgemein ("können die Tabelle sperren").
Fix: aktionsbezogener Satz je Treiber (PK-Änderung = Neuaufbau möglich; PG: Schreibsperre während Indexaufbau).

### L3 (niedrig, Doku): Handoff `W-P09-backend.md` ist veraltet
Nennt 27 Fixtures/zehn echte, "MySQL-/MariaDB-/PostgreSQL-Livebelege fehlen", Livesuite "nicht gestartet", einen fehlgeschlagenen Listener-Test. Nach Integration: 61 Fixtures, 43 echte (`real-*`), Vier-System-Suite exit 0. WAVE3-INTEGRATION.md ist korrekt; das Paket-Handoff sollte darauf verweisen oder den Prüfstand ersetzen.

### Informativ
- `ExecuteIndexes` baut den Plan frisch und prüft Hash/Aktion/Name unter derselben Sitzung (PG/SQLite in Transaktion, MySQL ohne Transaktion, aber unmittelbar vor dem kombinierten ALTER). Verbleibendes Fenster auf MySQL ist klein und wird von der Engine selbst (Duplikat/Abhängigkeit) abgefangen: UNIQUE-Plan sauber, danach Duplikat eingefügt, Ausführung nicht `completed`, Zeilen unverändert (alle drei Engines bestätigt).
- Ausgang bei Commitverlust/Verbindungsabbruch läuft über den bereits begutachteten `runColumns`; hier nicht neu geprüft (kein Netzwerk-Fault-Test im Paket).
- Hash deckt Anweisungen ab, nicht Warnungen/Abhängigkeiten; Handoff sagt das ehrlich.

## Geprüft ohne Befund

1. Bestätigung: getippter Name stammt aus `p.Table.Name` (Katalog), Vergleich bytegenau. `üser tab`, `ÜSER TAB`, `Üser Tab ` (Leerzeichen), leer, Indexname als Ziel, falsche Aktion: alle `confirmation_mismatch`. Falscher/leerer Hash `plan_mismatch`. Case-variante Tabellenreferenz (`üSER tAB`) wird auf PostgreSQL/SQLite abgewiesen. Replay von `index_drop` nach erfolgreichem Drop: `object_not_found`; Replay von `index_create`: `object_exists`. Nicht-destruktive Aktionen (create) brauchen korrekt keinen Namen, aber Aktion und Hash.
2. Injektion: Namen mit Backtick, Anführungszeichen, `;DROP`, NUL, Spaltenname mit Quote, Direction `asc; DROP`, Expression `lower(v)); DROP...`, Präfixlänge -1 und 99999, ON-Aktion `cascade; DROP`, unbekannte Referenztabelle: alle als öffentliche Fehler abgewiesen. Existierende Namen kommen nur aus Katalogmetadaten, Quoting nur über `dbplan`. Aktionen aus Whitelist (`set_default` nur PostgreSQL). Nachbartabelle (`neighbor`) blieb in allen Läufen unverändert.
3. Datenverlust: PK-Drop mit FK-Abhängigkeit (PostgreSQL) und Index-Drop eines FK-tragenden Index (MySQL/MariaDB) enden `rejected/dependency_blocks`, Plan nennt "Fremdschlüssel betroffen". PK-Setzen über NULL-Spalte `rejected/invalid_value`. UNIQUE über Duplikate: Duplikatgruppen im Plan, Ausführung `rejected`, kein Teilzustand. Ausdrucks-UNIQUE (`lower(v)`) erkennt Groß/Klein-Duplikate. MyISAM/Nicht-InnoDB und Vererbung/Partitionen abgewiesen.
4. Fähigkeiten: SQLite nur CREATE/DROP INDEX (`unsupported_action` für PK/FK/alter), FULLTEXT/SPATIAL/Präfix nur MySQL-Familie, Ausdruck/Teilindex nur PostgreSQL; MySQL meldet `SPATIAL`, MariaDB `RTREE` (Test akzeptiert beides). Fehlermeldungen enthalten keine Treibertexte, SQL oder Pfade (Textscan in allen Fällen).
5. Systemobjekte: `sqlite_master`/`SQLITE_MASTER`, `mysql.user`, `MYSQL.user`, `sys`, `performance_schema`, `information_schema`, `pg_catalog`, `pg_toast`, `information_schema` (PG) für `index_create` und `primary_drop` abgewiesen; SQLite-Autoindex nicht löschbar; Indexname `sqlite_*` abgewiesen.
6. Vertrag/Wächter: v1 bis v12 byteidentisch (Wächter pinnt v7 bis v12 selbst, älter über overview/object-ops), v13 additiv. Mutationstest: Bytes von v12 verändert wird erkannt; v13-Bytes verändert wird vom Indexwächter nicht erkannt (er pinnt sich selbst nicht; v13 ist ab dem v14-Wächter gepinnt, dort erkannt), Entfernen einer Route, `security: []` auf v13-Route, unbekanntes Feld in echter Fixture: erkannt. Fixtures echt (`real-*`) und Platzhalter sind in der README getrennt. Authentifizierung `a.auth` auf allen drei Routen, GET capabilities/POST plan/alter, Body-Limit über `decode`.
7. Lizenz/Attribution: SPDX AGPL-3.0-only, keine Attribution-Zeilen, keine neuen Abhängigkeiten.

## Hinweis zu den eigenen Tests

`internal/database/wave3b_review_test.go` bleibt als Regression im Review-Branch (SQLite läuft immer, Livefälle über die dbfix-Umgebungsvariablen, sonst Skip).
