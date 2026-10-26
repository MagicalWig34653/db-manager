<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Unabhängiges Review Welle 4 (Reviewer B): W-P17, W-P27, W-P28, W-P29

Stand: 26.10.2026. Geprüft: `integration/wave4-backend` @ `6b74f3b` (API 0.17, 0.20, 0.21, 0.22). Branch `review/wave4-b`.
Nichts gemergt, kein Produktcode geändert. Reviewer hat nichts implementiert.

## Ergebnis

**Freigabe mit Auflagen. Kein Blocker.** Keine Schreib-/KILL-/SET-Pfade, keine Injektion, keine Secret-/SQL-Text-Lecks, kein
Teilzustand-Betrug gefunden. Zwei mittlere Befunde (F1 YAML, F2 Monitor-Fehlklassifizierung) sollten vor dem Vertragsfreeze
behoben oder im Vertrag als bekannte Einschränkung festgehalten werden; die übrigen sind niedrig/Information.

| Paket | Urteil |
| --- | --- |
| W-P17 Serververwaltung lesend (0.17) | frei; F3 (Über-Maskierung, Doku-Widerspruch) niedrig |
| W-P27 Monitor/Replikation (0.20) | frei mit Auflage F2 |
| W-P28 Exportformate (0.21) | frei mit Auflage F1 |
| W-P29 Partitionen/Präfix (0.22) | frei; keine Befunde aus Angriffen, nur Testlücken (T2) |

## Befunde nach Schwere

### F1 (Mittel, W-P28) YAML-Export ist für gängige YAML-Parser nicht lesbar bzw. verfälscht Daten
Das Format `yaml` erzeugt `json.Marshal`-Ausgabe. Go escaped U+0000..U+001F, `"`, `\`, `<`, `>`, `&`, U+2028/2029, aber nicht
U+007F, U+0080..U+009F, U+FFFE, U+FFFF. Diese Zeichen sind in YAML (auch 1.2, Produktion c-printable) im Strom unzulässig; U+0085 (NEL)
wird von libyaml/PyYAML/Psych als Zeilenumbruch gefaltet.
Reproduktion (echte Ausgabe, `go test ./internal/database -run TestReviewB28NastyExportDump` mit `DBM_REVIEW_DUMP=<dir>`, dann
`python3 -I scripts/review-wave4b-parsers.py <dir>`):
- PyYAML `safe_load`/CSafeLoader: `ReaderError: unacceptable character #x007f` (ebenso 0x90, 0x9f, 0xfffe, 0xffff).
- Ruby Psych `YAML.safe_load`: Fehler bei denselben Zeichen.
- U+0085 in einer Zelle: PyYAML und Psych liefern stumm `' '` statt NEL (Datenverfälschung ohne Fehler).
Der Vertrag (`contracts/EXTENDED-EXPORT-API.md`) sagt "Strikte JSON-Teilmenge von YAML 1.2" und schließt nur YAML 1.1 aus; die Zusage
ist für DEL/C1/Nichtzeichen falsch. Die vorhandenen unabhängigen Parser-Tests nutzen diese Zeichen nicht.
Empfehlung: im YAML-Pfad zusätzlich U+007F, U+0080..U+009F und U+FFFE/FFFF (und sicherheitshalber U+0085) als `\uXXXX` ausgeben (eigener Encoder
oder Nachbearbeitung der JSON-Zeichenkette; `\x85`/`\u0085` bleibt gültiges JSON und gültiges YAML-Doppelquote-Escape), Regressionstest mit PyYAML.
Dasselbe gilt für `codegen`/`json` nicht (kein YAML-Anspruch).

### F2 (Mittel, W-P27) MySQL/MariaDB: Anmeldefehler und Syntax-/Objektfehler werden als "Datenquelle nicht verfügbar" mit HTTP 200 gemeldet
`monitorUnavailable` behandelt MySQL-Fehler 1044, 1045 (Access denied), 1064 (Syntax), 1146 (Tabelle fehlt), 1142/1143/1227 usw. als
`state:"unavailable"`, Text: "Datenquelle ist nicht installiert, deaktiviert oder mit diesen Datenbankrechten nicht lesbar."
Reproduktion (`scripts/review-wave4b-live.py`, Test `TestReviewB17B27LiveLeaks`, Zeile "FINDING"):
- `monitor/sample` mit falschem Passwort gegen MySQL 8.4 und MariaDB 12.3: Antwort 200 `state=unavailable`, kein Fehler. Gegen PostgreSQL
  (28P01) dagegen Fehler `Datenbankoperation fehlgeschlagen`. Die alte API (`Query`) liefert für dasselbe falsche Passwort einen Fehler.
- `profiling` mit `SELECT * FROM no_such_table_zz`: MySQL/MariaDB/PostgreSQL `unavailable` (irreführend: Nutzerfehler, kein fehlendes Feature);
  `SELECT nosuchcol FROM rv_probe`: MySQL/MariaDB Fehler, PostgreSQL `unavailable` (uneinheitlich).
Kein Sicherheitsproblem (keine Daten, kein Rohfehler), aber ein falsches Anmeldedatum sieht wie fehlende Monitor-Unterstützung aus und die
Engines verhalten sich verschieden. Empfehlung: 1044/1045 nicht schlucken (als Verbindungsfehler wie bei den übrigen APIs), 1064/1146/1054 beim Profiling als
`invalid_monitor`/Fehler, `unavailable` auf echte Quellen-/Rechtefehler (1142, 1143, 1227, 42501, 42P01 der Systemquellen) begrenzen.

### F3 (Niedrig, W-P17) `max_connections` und Status `Connections`/`Max_used_connections` werden immer maskiert
`adminSafeValue` maskiert jeden Namen mit Teilzeichenfolge `connection`. `contracts/SERVER-ADMIN-API.md` (Z. 55) führt `max_connections` aber als
sichtbare Variable. Test `TestReviewB17MaskingAdversarial` dokumentiert das Ist-Verhalten. Kein Leck (Über-Maskierung), nur Doku/Funktion
auseinander; ebenso `Key_*` (key), `*_file`-Zähler, `Ssl_*`-Zähler. Entweder Doku anpassen oder Allowlist-Namen vor der Sensitivprüfung zulassen.

### F4 (Niedrig/Information, W-P28) Klartext-`csv` ohne Formelschutz
`csv` gibt `=cmd|' /C calc'!A0`, `+1+1`, `-2+3`, `@SUM(1)` unverändert aus (auch Kopfzeile/Tabellennamen). Dokumentierte Entscheidung (W-P08-Vertrag);
`excel-csv` ist korrekt: jedes Feld inkl. Kopf, Marker und Trailer wird mit `'` präfixiert, Steuerzeichen werden durch U+FFFD ersetzt
(Test mit allen Trennzeichen und CRLF, Parse mit `encoding/csv`). Empfehlung: Web-Oberfläche muss bei `csv` warnen.

### F5 (Information) Nicht verifizierbar in dieser Umgebung
- LaTeX (`pdflatex` fehlt), MediaWiki und Texy: kein Renderer. Manuelle Analyse: LaTeX escaped `\ { } $ & # % _ ^ ~`, Steuerzeichen als Text; die erste Zelle jeder
  Datenzeile beginnt immer mit `\{` (JSON), daher kein `\\[`/`\\*`-Problem; Kopfzeile ist nicht betroffen. MediaWiki/Texy kodieren jedes Zeichen als `&#N;`.
- Last durch `analysis` (MySQL-Fallback `SELECT COUNT(*) FROM mysql.slow_log` ohne Begrenzung) und Abbruch serverseitiger Abfragen bei Timeout (30 s max) nicht unter Last gemessen.
- Konfigurierte Replikation (Replikas, Mehrquellen, WAL-Receiver) nicht live prüfbar (Fixtures sind Einzelserver); nur leere Listen und Rechte-`unavailable` gesehen.
- MySQL/MariaDB-Partitionswartung mit echtem Fehler in Ergebniszeilen (`partition maintenance reported failure`) nicht live ausgelöst.
- CI-Workflows nicht gelaufen. Zugehörige `*.real-*`-Fixtures unverändert vom Wächter akzeptiert.

### T (Testlücken, Folge der Mutationsproben)
Mutationen, die die vorhandenen Paket-Tests überleben (mit Reviewer-Tests getötet, soweit nicht anders vermerkt):
- T1 W-P17: Entfernen der Sensitiv-Namenssperre (`sensitive`), Wertlängenlimit 64, Scope-Validierung vor Verbindungsaufbau (SQL-Injektion über `SHOW <scope> VARIABLES` wäre möglich gewesen).
- T2 W-P29: `len([]rune(prefix))` -> `len(prefix)` (Mehrbyte-Präfix), Partitionsnamenvalidierung `opName`, `export_busy`-Zweig; `destructive := true` -> `false` wird nur durch den Live-Test
  `TestReviewB29LivePartitionAttack` getötet (SQLite kann keine Partitionen).
- T3 W-P27: Log-Name-Regexp mit Anführungszeichen, `ReadOnly: true` im Profiling-Tx (überlebt auch Reviewer-Tests; SQL-Grammatik verhindert Schreiben unabhängig davon), Längenlimit 256 in `monitorValue`.
- T4 W-P28: Zeichenbereich von XML (`0xfffd`; Go ersetzt unzulässige Zeichen stumm durch U+FFFD), Steuerzeichen-Ersetzung in `excel-csv`.
- Überlebende, die ich als gleichwertig/unbedeutend einstufe: `HasPrefix`-Nachfilter im Präfix (die SQL-Abfrage ist bereits wörtlich), `LOCATE(...)>=0`, `seen[part.Name]` (MySQL),
  `if failed` bei MySQL-Wartungsergebnissen (nur mit echtem Fehlerfall prüfbar).

## Was geprüft wurde und ohne Befund blieb

**W-P17.** Quelle `serveradmin.go`/`httpapi/serveradmin.go`: ausschließlich statische SELECT/SHOW-Texte, `Scope` nur aus {GLOBAL,SESSION}, Filter nur als Parameter (`LOCATE`/`strpos`), keine
`KILL`/`SET`/`FLUSH`/`pg_terminate_backend`-Zeichenfolge (Test `TestReviewB17NoWriteVerbs`); HTTP: genau `GET /api/v17/capabilities` und `POST status|variables|processes`;
PUT/PATCH/DELETE, GET auf POST-Routen und `kill`, `processes/kill`, `variables/set`, `flush` liefern kein 200 (404/405; `TestReviewB17B27NoWriteSurface`).
Maskierung: nur numerische Werte (Regex), Name ohne `password/secret/token/key/path/file/dir/ssl/tls/connection/credential`, für Variablen zusätzlich feste Allowlist;
adversariale Werte (`NaN`, `Inf`, `0x10`, `1-1`, 65 Ziffern, `.`) bleiben maskiert. Live auf MySQL 8.4/MariaDB 12.3/PostgreSQL 18.6 mit Root und eingeschränktem Konto: nie nichtnumerischer
Wert sichtbar; parallel laufende Abfrage `SELECT 'SECRETQ_xyz', SLEEP(4)` taucht in keiner Antwort auf; Passwörter/Pfade/`datadir`-Werte/Hosts fehlen. Prozessliste liefert ID, Benutzer, Datenbank, Sekunden, self
(Benutzernamen anderer Sitzungen sind sichtbar, soweit die Engine sie preisgibt; bewusst so dokumentiert).

**W-P27.** Profiling-Grammatik (`monitorSelect`): 30 feindliche Eingaben (Semikolon, Kommentare, Unterabfrage, Funktion, `INTO OUTFILE`, `FOR UPDATE`, UNION, Backticks, Unicode-Ziffern/Leerraum, NUL,
`EXPLAIN ANALYZE`, `WITH`) abgelehnt; nur `EXPLAIN` ohne Ausführung, read-only Transaktion. Binlog-Name: Regexp plus Abgleich mit `SHOW BINARY LOGS`; Quote/Pfad-Varianten abgelehnt.
Ausgabefelder sind pro Feld validiert (Zahl/Digest/Enum), kein Rohtext (Replikationsfehlertexte, Info-Spalte von Binlog-Ereignissen, InnoDB-Status, Planausdrücke werden unterdrückt).
Rechte: eingeschränktes Konto liefert ehrliches `unavailable` für analysis/replication/binary-*; PostgreSQL ohne `pg_read_all_stats` meldet es. Falsches Passwort auf PostgreSQL gibt sicheren Fehler (kein Passwort/Host im Text).
Alle Live-Ausgaben (root und eingeschränkt, drei Engines) enthalten weder Passwörter, Pfade, Hosts noch den Literaltext `SECRETQ_xyz`.

**W-P28.** Echte Ausgaben mit 56 feindlichen Zellwerten (Formel-Präfixe, C0/C1/DEL/Nichtzeichen/U+2028/BOM, NUL, `?>`, `${x}`, `</value>`, `<![CDATA[`, Marker-Texte, YAML-Tags/Anker, LaTeX-Befehle,
Wiki-Syntax, Zeilenumbrüche) und feindlichen Tabellen-/Spaltennamen: unabhängige Parser `php include`, `xml.etree`, `xmllint`, `json` (codegen) liefern identische Zellen wie der typisierte JSON-Export;
PyYAML/Psych siehe F1. Abschlussmarker: genau einmal, Daten können ihn nicht fälschen; fehlt bei Zeilenlimit, Bytelimit, Timeout, Writer-Fehler, Abbruch (alle acht Formate). Leere Datenbank, zwei Tabellen plus View
(`views_as_tables`), Query-Scope mit feindlichen Aliasen: gültige Dokumente (XML/YAML/PHP geparst). Streaming zeilenweise, höchstens 2 gleichzeitige Exporte (dritter `export_busy`, Slot danach frei).
Ungültiges UTF-8 wird abgelehnt statt ersetzt. `excel-csv`: siehe F4.

**W-P29.** Präfix wird nie als LIKE benutzt (`substr`/`BINARY LEFT`/`left`, Treffer zusätzlich per `HasPrefix`). Live auf allen drei Engines: `w_`/`w%` treffen nur wörtlich; `%` allein findet nichts;
`remove` auf Tabelle gleich Präfix (leerer Zielname) und Umbenennen auf bestehende Namen/Ketten (`r`+`add r`) werden im Plan abgelehnt, ohne Änderung; Präfix `q'"`;--ä` droppt exakt 2 Tabellen;
10 Bestätigungs-/Hash-Varianten (leerer Name, falsche Aktion, Leerzeichen, Großschreibung, Hash groß/leer/Leerzeichen, anderer Präfix, Systemschema) abgelehnt, keine Tabelle geändert; veralteter Plan -> `plan_mismatch`;
Zielname > 64 Zeichen (MySQL) im Plan abgelehnt; zwei parallele Ausführungen desselben Plans: genau eine `completed`. Partitionen (Live): leerer getippter Name, Partitionsname statt Tabellenname, falsche Aktion,
Null-Hash, Hash eines anderen Plans abgelehnt; feindliche Partitionsnamen (Backtick, `"`, `%`, Leerzeichen) nicht im Katalog -> abgelehnt; feindliche Grenzwerte (`30) , PARTITION px ...`) werden als Textliteral
`'...'` eingesetzt, der Server lehnt sie ab, Schema unverändert; selektives TRUNCATE lässt die andere Partition unberührt; zwei parallele DROP-Ausführungen: eine completed. Statische Prüfung der Grenzen mit 17 feindlichen Werten.
MySQL-Sitzung setzt `NO_BACKSLASH_ESCAPES` (Literal-Verdopplung des `'` ist damit ausreichend). Teilzustand je Tabelle (`applied`/`not_applied`/`unknown`) ist konservativ: die fehlgeschlagene Tabelle gilt als `unknown`.

## Ausgeführte Prüfungen

| Prüfung | Ergebnis |
| --- | --- |
| `gofmt -l .`, `go vet ./...` | leer / ok |
| `go test -race -count=1 ./...` (inkl. Reviewer-Tests) | alle Pakete ok |
| 21 `scripts/check-*-contract.py`, `check-docs.py` | alle ok; 0 Fehler |
| `test-server-admin.py` | ok: 12/12/11 Antworten (MySQL 8.4 / MariaDB 12.3 / PostgreSQL 18.6) gegen 0.17 |
| `test-monitor.py` | ok: 24 Antworten (SQLite, MySQL, MariaDB, PostgreSQL) |
| `test-extended-export.py --live` | ok; 24 Dateien durch unabhängige Parser; alle drei Engines + SQLite |
| `test-partitions.py` | ok: 19 Antworten; SQLite, dann je eigener Wegwerf-Container |
| `scripts/review-wave4b-live.py` | eigene Live-Angriffe: Präfix, Partitionen, Lecks/Rechte; alle Assertions grün (F2 als Log-Zeile) |
| `scripts/review-wave4b-parsers.py` | PyYAML und Psych schlagen fehl (F1); php/xml/xmllint/codegen ok |
| `scripts/review-wave4b-mutations.py` | 39 Mutationen, Ergebnis siehe T1..T4 |

Alle Container (eigene `dbmanager-rev-b-*`, Fixtures der Skripte) wurden entfernt; fremde Container (`dbmanager-rev-a-*`, `dbmanager-wp15-*`) nicht angefasst.

## Neue Dateien (Reviewer)

- `internal/database/review_wave4b_test.go`, `internal/httpapi/review_wave4b_test.go`: Angriffstests (SQLite immer, Live nur mit `DBM_TEST_*`-Variablen).
- `scripts/review-wave4b-parsers.py`, `scripts/review-wave4b-live.py`, `scripts/review-wave4b-mutations.py`: Hilfsskripte (Mutationsskript stellt jede Datei per `git checkout` wieder her).

## Nächste Schritte

1. F1 und F2 beheben oder im Vertrag als Einschränkung eintragen; danach Wiederholung der Live-Skripte.
2. F3: Doku oder Maskierungsreihenfolge angleichen.
3. Reviewer-Tests in den Welle-4-Integrationsbranch übernehmen (nach Entscheidung des Leads).
4. Offen aus F5: LaTeX-Kompilierung, Replikations-/Lastfälle auf geeignetem Host.
