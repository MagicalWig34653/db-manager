# Review W-P14 Backend: Suche, Suchen und Ersetzen (API 0.14)

Reviewer: unabhängig (nicht Autor, Codex-Paket). Geprüft: `integration/wave3-backend` (f345adb), Merge d767727, Fixes 3847bdd, 26.10.2026.
Review-Branch: `review/wave3-b` (nur Tests und Review-Dokumente). Schwerpunkt: Musterescape, Regex/DoS, Ersetzen (Plan, Konflikt, Teilzustand), Vertrag.

## Urteil

Freigabe mit Auflagen (accept with changes). Keine Injektion, keine Bestätigungsumgehung, kein Teilzustand beim Ersetzen, kein Leck gefunden. Zwei mittlere Befunde: M1 (serverseitiger Abbruch von MySQL/MariaDB-Suchen fehlt) und M2 (Workflow ruft entferntes Skript auf; CI schlägt fehl). Die übrigen sind niedrig.

## Ausgeführt

| Prüfung | Ergebnis |
| --- | --- |
| `gofmt -l .`, `go vet ./...` | ohne Ausgabe |
| `go test -race -count=1 ./...` | alle Pakete ok |
| alle `check-*-contract.py` (inkl. `check-search-contract.py`), `check-docs.py` | bestanden (v1 bis v13 gepinnt, v14 additiv, 0 Docs-Fehler) |
| SHA-256 `contracts/api-v0.14.0.json` | `b57c3e5d...56c1e`, stimmt mit WAVE3-INTEGRATION; das Paket-Handoff nennt noch `fda8077a...` (siehe L4) |
| `scripts/test-search.py` (SQLite, MySQL 8.4, MariaDB 12.3, PostgreSQL 18.6) | exit 0, native Antworten gegen v14 validiert, keine Container übrig |
| eigene Tests `wave3b_review_test.go` (`TestReviewW3BSearch*`, `Replace*`, `Cancel*`, `ScanCancel*`) SQLite + drei Engines, eigene Wegwerf-Container nacheinander | bestanden bis auf Messungen M1 |
| Mutationstest Wächter | wirksam (siehe unten) |

## Befunde

### M1 (mittel): Clientseitiger Abbruch beendet die Serverabfrage auf MySQL/MariaDB nicht
Beleg: Suche über 800 000 Zeilen mit `timeout_ms=300` bricht nach 300 ms ab (Fehler an den Client). Probe von `information_schema.PROCESSLIST` alle 0,5 s: MariaDB Substring: Abfrage noch rund 2 s aktiv, MariaDB Regex: noch aktiv über die gesamte Beobachtung von 6 s; MySQL Substring noch rund 2 s. PostgreSQL: sofort beendet (0 aktive Abfragen). Es wird weder `max_execution_time` (MySQL), `max_statement_time` (MariaDB) noch `KILL QUERY` gesetzt; PostgreSQL profitiert vom Protokoll-Cancel.
Wirkung: Wiederholte teure Suchen (20 Tabellen, Regex auf großen Tabellen) stapeln serverseitige Volltabellenscans, obwohl die API Abbruch/Zeitlimit zusagt (SEARCH-API.md: "garantiert aber keinen..." nur teilweise ehrlich). DoS-Pfad für die Zieldatenbank.
Fix: pro Suche/Plan Sitzungsvariable setzen (`SET SESSION max_execution_time=<ms>` MySQL, `SET SESSION max_statement_time=<s>` MariaDB, `SET LOCAL statement_timeout` PostgreSQL) oder bei Context-Cancel `KILL QUERY <connection_id>` über zweite Verbindung; Test mit dem Abfragezähler wie in `TestReviewW3BScanCancelLive`.

### M2 (mittel): `.github/workflows/search.yml` ruft entferntes `scripts/test-search-guard.py` auf
Beleg: Zeile 35; die Datei existiert nach der v13-Neuaufbau nicht mehr (WAVE3-INTEGRATION: "entfernt"). CI-Lauf wäre rot.
Fix: Zeile entfernen; ggf. die Vorgängerpins im Wächter (bereits strikt) als Ersatz belassen.

### L1 (niedrig): Ersetzen in Tabellen mit `ON UPDATE`-Spalten oder Triggern endet immer `row_conflict` ohne Vorwarnung
Beleg: MySQL/MariaDB `r3 (id, v, upd TIMESTAMP ... ON UPDATE CURRENT_TIMESTAMP)`: Plan ok, Ausführung `rejected/row_conflict`, Daten unverändert. Die Nachprüfung vergleicht alle nicht generierten Zellen, die vom Update selbst geänderte Zeitspalte gilt als Fremdänderung. Sicher, aber irreführend (Grund klingt nach Nebenläufigkeit) und Ersetzen ist dort unbenutzbar.
Fix: Planphase erkennt `ON UPDATE`-Spalten (information_schema `EXTRA`) und weist `unsupported_search` mit eigenem Grund aus, oder die Nachprüfung toleriert sie dokumentiert.

### L2 (niedrig): Groß-/Kleinschreibung der Suche ist treiberabhängig und nur indirekt dokumentiert
Beleg: `strasse` findet `STRASSE` auf MySQL/MariaDB/SQLite (Kollation, SQLite nur ASCII), nicht auf PostgreSQL (`LIKE` case-sensitiv). Ersetzen mit `substring` ist überall case-sensitiv (Plan zeigt `affected_rows=0` für `KEEP`→`keep`-artige Treffer, ehrlich). SEARCH-API.md nennt es allgemein (Kollation). Fix: eine Tabelle je Treiber in der Doku; ggf. Option `case_sensitive`.

### L3 (niedrig): Plan nimmt auf MySQL/MariaDB Zeilensperren (`FOR UPDATE`)
Beleg: `replacePrepare` liest immer mit `FOR UPDATE` (PostgreSQL unterscheidet SHARE/SHARE ROW EXCLUSIVE). "Plan ist reine Vorschau" gilt für Daten, nicht für Sperren bis zu 1001 Zeilen; ein Nutzer ohne UPDATE-Recht kann nicht planen. Fix: Doku-Satz oder `LOCK IN SHARE MODE`/Plain-Select für den Plan (Hash bleibt, Ausführung prüft erneut unter Sperre).

### L4 (niedrig, Doku): Handoff `W-P14-backend.md` und Workflow-Kommentar sind veraltet
Handoff nennt SHA `fda8077a...`, "v13 optional/abwesend", Zähler 13, `test-search-guard.py`, "keine Livelaufnachweise". Maßgeblich sind WAVE3-INTEGRATION.md und `b57c3e5d...`. Fix: Prüfstandtabelle ersetzen oder Verweis oben.

### Informativ
- Regex: MySQL 8.4 beendet pathologische Muster per ICU-Zeitlimit mit Fehler (redigiert), MariaDB (PCRE) und PostgreSQL liefern bei den geprüften Mustern (`^(a+)+$`, `(a|aa)+$b`, Rückverweis-Muster) in unter 110 ms ohne Explosion. Ungültige Regex liefert `database_error` mit festem Text, kein Treibertext. Kein Befund, aber Zeitlimit allein reicht ohne M1 nicht.
- Rückverweise im Ersetzungstext sind treiberspezifisch (`$1` MySQL 8.4, `\1` MariaDB/PostgreSQL); in Doku genannt, Test belegt `<123>`.
- Mehrwortsuche begrenzt auf 32 Wörter/32 Spalten/20 Tabellen; Treffer in SQL auf 256 Zeichen gekürzt (5000-Zeichen-Wert → Vorschau ≤ 256); Binär-, Integer-, unbekannte Spalten als Suchziel abgewiesen.

## Geprüft ohne Befund

1. Muster: LIKE immer `ESCAPE '!'` mit Escape von `!`, `%`, `_` (Werte `100%`, `a_b`, `a!b`, `a!!b`, `\`, `\_`, `back\slash` auf allen vier Systemen mit erwarteten Trefferzahlen). MySQL/MariaDB mit global gesetztem `NO_BACKSLASH_ESCAPES` (neue Sitzung): gleiche Zahlen, kein Bruch. Werte nur gebunden, Bezeichner nur aus Katalog (`SUBSTR`-/`typeof`-Hüllen).
2. Ersetzen: PK-, Nicht-Text- und unbekannte Spalte werden nicht geplant; generierte Spalte nicht beschreibbar, von ihr abhängige Spalte ersetzbar (Folgewerte werden akzeptiert). UNIQUE-Kollision (`a`→`b`) endet `rejected/database_error`, Tabellenzustand byteidentisch zum Vorher (vollständiger Rollback, SQLite/MySQL/MariaDB/PostgreSQL). Zwischen Plan und Ausführung: Änderung einer fremden Spalte → `plan_mismatch`, eingefügte Phantomzeile → `plan_mismatch`, Replay desselben Plans nach Erfolg → Fehler, nicht erneut angewendet. Falsche Bestätigung (Tabellenname groß/klein, Trefferzahl+1, Aktion `update`) abgewiesen. NULL-Zeilen unberührt. Ersetzungstext `'); DROP TABLE r1; -- \1 $1 %` wird in `substring`/`exact` literal gespeichert (gebunden), Tabelle bleibt. Limit 100/1000 Zeilen des gesamten Bestands (`replace_limit`, 422 statt Teilersetzung). Commitfehler → `outcome_unknown` (SQLite-Test).
3. HTTP: `a.auth` auf allen fünf Routen, strikte Dekodierung, fehlende/NULL-Felder 400, 409/422/404 abgebildet; keine Roh-SQL-/Treibertexte in Fehlern.
4. Vertrag: v1 bis v13 byteidentisch gepinnt, v14 additiv (inkl. v13-Pfade/Schemas übernommen). Mutationstest: v13 Byte ändern, Route aus v14 entfernen, `additionalProperties:true` in neuem Schema, unbekanntes Feld in gültiger Fixture: jeweils erkannt. Fixtures: 47 Dateien, davon 21 `real-*` (SQLite, MySQL, MariaDB, PostgreSQL), Platzhalter (Hash aus Nullen) in der README gekennzeichnet.

## Hinweis zu den eigenen Tests

`internal/database/wave3b_review_test.go` (gemeinsam mit W-P09) ist Regression; Live-Teile werden ohne dbfix-Umgebung übersprungen. Die Messtests `TestReviewW3BCancelLive` und `TestReviewW3BScanCancelLive` (Tabelle mit 800 000 Zeilen) sind absichtlich schwer und nur für Reviewläufe gedacht.
