# Review W-P08 Backend: Spalten ändern (API 0.9)

Reviewer: unabhängig (nicht Autor, Codex-Paket). Geprüft: Integrationsbranch `integration/wave2-backend` (60592de), Merge 7ccdcc7, 25.10.2026.
Review-Branch: `review/wave2-a` (nur Tests und Review-Dokumente). Schwerpunkt: destruktive Pfade, Injektion, ehrliche Warnungen.

## Urteil

Freigabe mit Auflagen (accept with changes). Keine Bestätigungsumgehung, keine Injektion, keine Lecks gefunden. Zwei mittlere Befunde (M1, M2) betreffen
unvollständige Warnungen vor stillem Funktionsverlust; sie sollten vor dem Webpaket durch Warntexte/Planinhalt behoben werden.

## Ausgeführt

| Prüfung | Ergebnis |
| --- | --- |
| `gofmt -l .`, `go vet ./...` | ohne Ausgabe |
| `go test -race -count=1 ./...` | alle Pakete ok |
| alle elf `check-*-contract.py` | bestanden (columns: v1 bis v8 byteidentisch, v9 additiv, 44 Fixtures) |
| SHA-256 `contracts/api-v0.9.0.json` | `508b9951...e3f8`, stimmt mit Handoff |
| `scripts/test-columns.py` | bestanden, 21 echte Antworten gegen v9 validiert; keine Container übrig |
| `scripts/check-docs.py` | 0 Fehler |
| eigene Tests `wave2a_review_test.go` (SQLite, MySQL 8.4, MariaDB 12.3, PostgreSQL 18.6, je eigener Wegwerf-Container nacheinander), `types_review_test.go` (inkl. 15 s Fuzz) | bestanden |
| Mutationstest Wächter | wirksam (siehe 6) |

## Befunde

### M1 (mittel): MySQL/MariaDB `drop` einer von einer Sicht genutzten Spalte gelingt und macht die Sicht ungültig; Warntext sagt das Gegenteil
Beleg (Livetest MySQL 8.4 und MariaDB 12.3): Tabelle `rv_dat` mit Index `rv_dat_t` auf `t` und Sicht `rv_dat_v AS SELECT id, t`. `drop t` wurde bestätigt und lieferte `completed`; danach `SELECT COUNT(*) FROM rv_dat_v` ungültig (Sicht defekt), der Index ist stillschweigend entfallen.
Die Vorschau führt `view:probe.rv_dat_v` und `index:rv_dat_t` in `dependencies`, die Warnung lautet aber nur „Abhängigkeiten können die Änderung verhindern. Kein CASCADE." Das stimmt für PostgreSQL (nachweislich `rejected/dependency_blocks`) und Fremdschlüssel, nicht für Sichten und Indizes in MySQL/MariaDB/SQLite (SQLite lehnte im Test mit database_error ab).
Fix: im Planbau je Abhängigkeitsart treiberspezifische Warnungen erzeugen („Die Sicht X wird ungültig", „Der Index Y wird mitgelöscht oder verkürzt"), den Satz „können verhindern" nur für Fremdschlüssel/PostgreSQL-Sichten verwenden. Der Plan-Hash ändert sich dadurch nicht strukturell (nur Warnungen).

### M2 (mittel/niedrig): PostgreSQL-Typänderung einer `serial`-Spalte entfernt den `nextval`-Standardwert ohne konkreten Hinweis
Beleg (Livetest PostgreSQL 18): `change s` (SERIAL, nicht PK) auf `int64 NOT NULL`, Standardwert `none`: `completed`; `column_default` danach `<none>`, Sequenz bleibt verwaist. Folgende Inserts ohne Wert scheitern an NOT NULL. Die Warnung nennt nur „vollständige neue Definition ersetzt die Attribute"; der entfallende Standardwert ist nur an `DROP DEFAULT` in den Anweisungen erkennbar. Gleiche Klasse in MySQL/MariaDB (Default wird ersetzt). Identity-Spalten werden abgewiesen (`unsupported_action`), PK-Serial schlug im Test fehl (rejected, unverändert).
Fix: bei `change`/`reorder` den aktuellen Standardwert aus `describe` in die Warnung aufnehmen („Standardwert nextval(...) entfällt") und bei `nextval` den Hinweis auf die verwaiste Sequenz ergänzen; alternativ Serial-Spalten ablehnen.

### L1 (niedrig): `rename` ohne Abhängigkeitsprüfung und Warnung
Beleg: Plan für `rename t -> t2` hat `destructive=false`, `dependencies=[]` und nur die zwei Standardwarnungen. In MySQL/MariaDB werden Sichten dadurch ungültig, in allen Treibern brechen Anwendungsabfragen. Fix: `columnDependencies` auch für `rename` aufrufen und „Sichten und Programme, die den alten Namen verwenden, brechen" warnen. Bestätigung ohne getippten Namen ist vertretbar, solange die Warnung steht.

### L2 (niedrig): Planen von NOT-NULL-Änderungen zählt per Volltabellenscan
Beleg: `SELECT COUNT(*) ... WHERE col IS NULL` im Planpfad (Handoff nennt es). Begrenzt durch Zeitlimit (Standard 30 s, höchstens 60 s) und Sitzungs-Sperrzeit; kein Bypass. Fix (optional): `LIMIT 1`-Existenzprüfung (`EXISTS`) statt Zählung, Warnung „enthält NULL-Werte" ohne Zahl.

### L3 (niedrig): Namensprüfung erlaubt Rand-Leerzeichen und Steuerzeichen in neuen Spaltennamen
Beleg: `createName` prüft nur Länge (63 Byte), UTF-8 und NUL. Die Engine lehnt unbrauchbare Namen mit generischem `database_error` ab (MySQL: Namen mit Endleerzeichen); keine Injektion (Bezeichner immer über `dbplan.QuoteIdentifier`). Fix: gleiche Regeln wie `opName` in W-P07 (Rand-Leerzeichen und Steuerzeichen früh mit `invalid_columns` ablehnen).

### L4 (niedrig, Doku): Handoff `W-P08-backend.md` ist veraltet
Beleg: nennt „29 Fixtures, 23 Platzhalter", „Live-Skript nicht ausgeführt", „noch kein Review"; der Integrationsstand hat 44 Fixtures mit echten Engine-Antworten und Livetests. `WAVE2-INTEGRATION.md` stellt das richtig; das Paket-Handoff sollte einen Verweis oder Nachtrag erhalten.

### Informativ
- Replay: `drop`, `add` und `rename` scheitern bei Wiederholung harmlos (`column_not_found`, `column_exists`/ungültig); `change` wiederholt dieselbe Definition (idempotent). Das Handoff verspricht keinen Replay-Schutz und sagt es ehrlich.
- TOCTOU: Anweisungen sind namensbasiert; Plan wird in der Ausführungssitzung (PostgreSQL in der Transaktion) neu berechnet. Das Fenster ist klein und im Handoff nicht versprochen; kein Befund über die W-P07-Grenze hinaus.

## Geprüft ohne Befund
1. Bestätigung: ohne/falscher/großgeschriebener/mit Leerzeichen getippter Name, Name einer anderen Tabelle, Spaltenname als Ziel, falsche Aktion, Plan einer anderen Tabelle, Nullhash, Großbuchstaben- und kurzer Hash: alle abgelehnt (`confirmation_mismatch`/`plan_mismatch`), Spalte unverändert (SQLite, MySQL, MariaDB, PostgreSQL). Zielname ist `d.Table.Name` aus dem Katalog; abweichende Schreibweisen der Anfrage lösen nicht auf. Erlaubte Variante dann `completed`, Nachbartabelle unberührt. Replay von `drop` und `add` wie oben.
2. Injektion: 8 bis 9 Angriffsnamen je Treiber (Backtick, Quotes, `;DROP`, Unicode, Backslash, 63 Byte) für add und rename; Standardwert mit Quotes/Backslash/Semikolon round-trip exakt; Kommentar mit Quotes/Backslash/SQL (MySQL, PostgreSQL) exakt; Kollation nur aus dem Katalog (4 Injektionsversuche abgelehnt); ENUM-Werte mit `'`, Backtick, `"`, `;`, `\` und Standardwert außerhalb der Liste (MySQL/MariaDB); Datums-Standardwerte mit Quote/Backslash/Überlänge nicht ausgeführt; Positionsspalte mit Injektion und ungültiger Modus abgelehnt. Nachbartabelle unversehrt. `FuzzTypeSQLEnum` (3 Mio. Läufe): Ausgabe parsebar, kein früher Literalabschluss, Werte unverändert.
3. Zerstörungsradius und Datenverlust: Verkürzen von Text und unmögliche Casts (`'not a number'` nach BIGINT, PostgreSQL mit `USING`) enden `rejected`, Daten unverändert; NOT NULL über NULL-Zeilen warnt mit Anzahl und wird abgelehnt; Primärschlüssel-Drop unter Fremdschlüssel wird abgelehnt, Kindtabelle unverändert; PostgreSQL-Drop unter Sicht `dependency_blocks`; Dependencies enthalten Index, Constraint, Sicht je Treiber. Systemtabellen (`mysql.user`, `pg_catalog.pg_class`), Sichten, MyISAM, PostgreSQL-Vererbung und generierte Spalten (`unsupported_action`/`invalid_columns`) abgewiesen.
4. Fähigkeiten und Meldungen: SQLite kein change/reorder (Neuaufbau nicht unterstützt), PostgreSQL kein reorder; Ergebnisse tragen feste Gründe, im Test keine Tabellen-/SQL-Texte in Meldungen.
5. Typkatalog `internal/dbplan/types.go`: Grenzen (text 1..1024, decimal 1..38 / scale ≤ precision, enum ≤ 256, set ≤ 64, Werte ≤ 255 Byte, kein NUL/ungültiges UTF-8/Komma in SET/Endleerzeichen/Duplikate), Kataloge je Treiber (ENUM/SET nur MySQL/MariaDB), irrelevante Parameter und Kinds wie `"text VARCHAR(1)"` abgelehnt (`TestTypeSQLBoundaries`). Die MariaDB-Abweichung (`COLUMN_DEFAULT='NULL'` als Text) betrifft nur die Testabfrage (Integrationshandoff Korrektur 1), nicht den Produktcode. PostgreSQL `USING "col"::TYPE` wird nur aus dem festen Katalog erzeugt.
6. Verträge: SHA stimmt; v1 bis v8 byteidentisch (Wächter), v9 additiv und strikt. Mutationstest: zusätzliches Feld in einem echten Fixture, angehängtes Byte an v8, entfernte Strikt-Eigenschaft in v9 lassen den Wächter fehlschlagen (`AssertionError`). Auth: alle drei v9-Routen hinter Bearer (HTTP-Test 401), POST/GET wie im Vertrag, Body 1 MiB (413/400 im HTTP-Test).
