# Handoff Integration Welle 3 (Backend)

Branch `integration/wave3-backend` (Basis main `c331c64`), nicht gemergt. Codex-Pakete ohne Docker gebaut; hier mit echten Engine-Fixtures und Livetests
(MySQL 8.4, MariaDB 12.3, PostgreSQL 18.6 verify-full, SQLite; Container nacheinander) nachgeprüft.

## Reihenfolge, Nummern

| Paket | Quelle | API | Vertrag SHA-256 |
| --- | --- | --- | --- |
| W-P09 Indizes/Schlüssel | `84f9d7e` | 0.13 (unverändert) | `e05a2402a8e6ec6a1901afeac03969e34d4fa1821e50a5c8675a6e20f5c1d3d7` |
| W-P14 Suche/Ersetzen | `655015b` | 0.14 (auf v13 neu aufgebaut) | `b57c3e5dcf403c3e16d8564c68d0220453625b25f6f206fdc128d2e249656c1e` |
| W-P18 Benutzer/Rechte | `18d38f5` | 0.16 -> **0.15** | `cd4681a200045c2fdd5ddf9f1bdc2a356c998b30e640495121925ca8a609dad1` |
| W-P19 Programmobjekte | `cac1709` | 0.17 -> **0.16** | `d90ac3fda8295a4b8a7095c72eb459c6433fce90f729d7a6b247760d86de7bf3` |
| W-P17 Serververwaltung | `d55b567` | **nicht integriert** | WIP-Stand enthält die schreibende Stufe (API 0.17); der Codex-Lauf zur Reduktion auf die lesende Stufe war nicht fertig (origin unverändert). |

Verträge wurden jeweils auf dem Vorgänger neu aufgebaut; die Wächter pinnen v1 bis v(N-1) strikt (keine optionalen Sonderfälle; `test-search-guard.py` entfernt).
Konflikte nur in `features.go`/`features_test.go` (Zählung jetzt 16).

## Korrekturen durch Livetests
1. W-P09: MySQL 8.4 meldet Methode `SPATIAL`, MariaDB `RTREE`; Test akzeptiert beides.
2. W-P18: MariaDB 12.3 liefert `JSON_VALUE(... account_locked)` als `1` statt `true` (Sperrstatus wurde als entsperrt gelesen) – Produktcode korrigiert; MySQL lehnt `CREATE ROLE`/`CREATE FUNCTION` im Prepared-Protokoll ab (Test nutzt Textprotokoll); PostgreSQL: `DROP ROLE` braucht vorheriges Entziehen aller Rechte (Test), Grant-Option-Revoke entzieht nur die Option (Test).
3. W-P19: MySQL speichert Backticks in Sichttexten verdoppelt – Abhängigkeitserkennung (`textMentions`) beachtet das (Produktcode); Testsicht qualifiziert.
4. W-P19: Fixture-Inhalte `contract_version`/Pfade auf 0.16/v16 umgestellt.
5. Skripte `test-search.py`, `test-accounts.py`, `test-programs.py` erhielten `--keep`/Dumpverzeichnis; `test-accounts.py` prüft, dass das Testpasswort in keiner Antwort vorkommt.
6. Echte Fixtures `*.real-*` für indexes, search, accounts, programs ergänzt.

## Prüfstand
gofmt, go vet, `go test -race -count=1 ./...` (alle Pakete, inkl. Listener-/Recovery-Tests; einmal flaky unter Last 700: `TestSQLiteSQLBatchLimitsAndCancellation`, isoliert und im Wiederholungslauf grün), Vertragswächter indexes/search/accounts/programs, `test-indexes.py`, `test-search.py`, `test-accounts.py`, `test-programs.py` (vier Systeme), `check-docs.py`.

## Sicherheitsblick
W-P18: Passwort nur bei Ausführung, Plan-Hash enthält Platzhalter `<protected-secret>`; Livelauf bestätigt, dass das Passwort in keiner Antwort steht; Selbstaussperrung wird abgewiesen; Bezeichner/Literale gequotet. W-P19: Plan + Bestätigung via `dbplan.Check` mit Katalogname; keine Ausführung von Routinen. W-P09/W-P14: `dbplan.Check` mit Katalogname; Suche mit ESCAPE '!'; Ersetzen nie in PK-/generierten Spalten. Keine Codeänderung darüber hinaus nötig.

## Offen
Unabhängige Reviews aller vier Pakete; W-P17 (lesende Stufe) folgt; Vertragsindex/STATUS/PARITAET beim Lead; `check-*`-Wächter der älteren Pakete (z. B. Wartung/Export) pinnen v13+ nicht (nur ihre Vorgänger).
