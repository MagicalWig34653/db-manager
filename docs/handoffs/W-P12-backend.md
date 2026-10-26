# Handoff W-P12 Backend: SQL-/CSV-Import (API 0.18)

Stand: 25.10.2026. Implementierter Reviewkandidat auf `feat/w-p12-backend`, Basis
`main` `c331c642fa4957747845458b079cffc32f6b1ff6`. Ausschließlich zugewiesener Worktree;
keine Git-Schreibbefehle, kein Commit, keine Integration oder Veröffentlichung.
Lead übernimmt Commit und unabhängige Abnahme einschließlich Sicherheitsreview.
Zentrale Status-/Planungsdateien, Vertragsindex und Webassets bleiben beim Lead.

## Ergebnis und Dateien

- `internal/core/import.go`: separates Engineinterface, Uploadmetadaten, CSV-Optionen,
  Plan-/Fehler-/Fortschritts-/Ergebnismodelle, feste Grenzen.
- `internal/database/import*.go`: begrenzte Entpackung/Zeichensatzkonvertierung, kompletter
  CSV-Prüflauf, gebundene Einzelzeilenstapel in Dateitransaktion, Savepoints, Stop/Skip,
  SQLite OR ABORT, Metadatensperre und erneute Planprüfung, Commit-/Rollbackunsicherheit.
  SQL auf gepinnter Verbindung, Stop/Continue, Statementstatus und Abbruch.
- `internal/httpapi/import*.go`: authentisierte `/api/v18/`-Routen für Fähigkeiten,
  Multipart-Plan/Execute und flüchtige Jobstatus/Cancel; begrenzte Speicherablage/Slots.
- `internal/sqlsplit/split.go`, `import_test.go`: zusätzlicher begrenzter, abbrechbarer Lexeraufruf
  für Importdateien; die alten 1-MiB-/100-Statementgrenzen des SQL-Stapels bleiben erhalten.
- `features.go`: genau ein neuer Featureeintrag und Uploadklasse auf 65 MiB einschließlich
  maximal 1 MiB Metadaten/Rahmen angehoben; Datei/Expansion jeweils separat maximal 64 MiB.
  `features_test.go`: Zählung 13, Prüfung der beiden unterschiedlichen Klassenlimits.
- [API 0.18](../../contracts/api-v0.18.0.json),
  [Verhalten](../../contracts/IMPORT-API.md), [Fixtures](../../contracts/fixtures/import/README.md),
  `scripts/check-import-contract.py`: v1–v12 gepinnt, v13–v17 optional, additiv auf v12,
  strikte neue Schemas und tatsächliche SQLite-Antworten sowie gekennzeichnete Platzhalter.
- `scripts/test-import.py`, `.github/workflows/import.yml`: SQLite ohne Docker,
  Vier-DB-Lauf ausdrücklich mit `--live`, gemeinsame Wegwerffixtures aus `dbfix.py`.
- [ADR W-P12](../decisions/ADR-W-P12-import.md): Entscheidungsvorschlag, Leadannahme offen.
  `go.mod`: bereits inventarisiertes `golang.org/x/text` 0.42.0 nun direkt verwendet;
  Version/Modulmenge und Originalbedingungen unverändert, keine neue Abhängigkeit.

Eigener Code SPDX AGPL-3.0-only, keine Attribution-Zeilen. Kein Schreiben in `.git`.
Neue Dateien sind ungetrackt, Bestandsänderungen ungestagt; Lead muss beides übernehmen.

## Verhalten und Grenzen

Browserupload, keine Importverzeichnisse, Multipart-Temporärdateien, Archivextraktion,
externen Programme oder LOAD DATA LOCAL INFILE. GZip mit harter Entpack-/Dekodiergrenze
und Prüfsumme; ZIP/BZip2 abgewiesen. Zwei aktive Slots für Upload/Plan/Ausführung, maximal
16 flüchtige Ergebnisse mit fünf Minuten Aufbewahrung; keine Resume-/Retrygarantie.
Status benötigt dieselbe API-Instanz. Credentials/Datei bleiben nur während des Jobs
im Speicher; Resultate enthalten feste Codes, keine Treiberrohtexte/SQL-/Serverpfade.

CSV nur vorhandene transaktionale Basistabellen, explizite Spaltenzuordnung und portable
Typteilmenge. Optionen für Separator/Quote/Escape/Charset/Header/NULL, maximal 100.000
Zeilen, 1024 Spalten, 1 MiB je Feld und 4 MiB Feldinhalte je Zeile. Vorschau maximal
20 gültige Zeilen/256 KiB und 100 Fehlerpositionen mit Kürzungsflag. Der Plan bindet
Datei, Optionen, Zielidentität ohne Passwort und beobachtete Metadaten. HTTP prüft vor
202 und Engine erneut unter Sperre. Kein CREATE, UPSERT oder implizites REPLACE.
Einzelzeilen-Inserts bilden begrenzte Stapel in einer Gesamttransaktion; PostgreSQL-
Savepoints erlauben Skip nach Constraintfehlern. MySQL erzwingt STRICT_ALL_TABLES und
weist Serverwarnungen als möglichen verlustbehafteten Import zurück. Unbestätigter
Commit/Rollback bleibt `outcome_unknown`; kein automatischer Wiederholungsversuch.

SQL maximal 10.000 Statements/1 MiB je Statement und 64 MiB Datei, 60 s Vorgabe/300 s
Maximum. Gleicher Lexer wie W-P15, eigene Grenzen ohne Lockerung des alten Vertrags.
Einfache BEGIN/COMMIT-Rahmen und feste W-P11-Sitzungsheader, keine beliebige Sitzungs-
steuerung. W-P11-Schlussmarke bei erkanntem Exportkopf erforderlich. Tabellendumps
mit optionalem Transaktionsrahmen getestet; Export mit disable_fk, USE-Serverdumps und
Statements über 1 MiB nicht unterstützt. CSV-Exportprofil mit Kontrollspalte/Endzeile
verlangt explizite Umwandlung, keine stillschweigende Formaterkennung. Keine universelle
Backup-, Dialekt-, Routinen-, Trigger- oder Formatparität. MySQL-/MariaDB-DDL kann
implizit committen; Statementabschluss beweist keinen Commit der ganzen Datei.
Externe Trigger-/Sequenzeffekte sind auch bei CSV nicht allgemein rücknehmbar.

## Prüfstand

Go mit `GOCACHE=/private/tmp/dbm-wp12-go-cache` und `GOPROXY=off`, keine Moduldownloads.
Alle Daten synthetisch, verwaltete kurzlebige SQLite-Datenbanken. Prüfungen lokal:

| Befehl | Tatsächlicher Ausgang |
| --- | --- |
| `go test -race -count=1 ./...` | Ausgeführt: bestehender `TestMySQLPerConnectionLoggerSuppressesServerInput` scheitert am gesperrten TCP-Listener; `TestCrashRecovery` mit Childtimeout. Übrige Pakete bestanden. |
| `go test -race -count=1 ./internal/sqlitesandbox -run '^TestCrashRecovery$'` | Gezielte Wiederholung bestanden (1,747 s). |
| `go test -race -count=1 ./... -skip '^TestMySQLPerConnectionLoggerSuppressesServerInput$'` | Bestanden: alle Pakete einschließlich Crash-Recovery (Datenbank 32,677 s). Nach abschließender Lexer-Abbrucherweiterung erneuter Lauf unten. |
| `go vet ./...` | Bestanden, auch nach den ergänzten Live-/Typ-/Triggerfällen. |
| `gofmt -l .` | Ohne Ausgabe nach gofmt auf allen geänderten Dateien. |
| `python3 scripts/test-import.py` | Bestanden: 6 tatsächliche Antworten gegen v18 validiert; 25 Fixtures insgesamt (6 echte, 19 synthetische positive/negative Fälle einschließlich 3 gekennzeichneter Netzwerkplatzhalter). |
| `DBM_IMPORT_STRESS=1 go test -race -count=1 ./internal/httpapi -run '^TestImportHTTPMaxUpload$' -v` | Bestanden: exakt 64 MiB akzeptiert, 64 MiB + 1 Byte mit 413 abgewiesen (3,001 s). |
| `DBM_IMPORT_STRESS=1 go test -race -count=1 ./internal/database -run '^TestSQLiteImportMaxFile$' -v` | Bestanden: 64-MiB-SQL-Datei mit 129 Statements, 128 Zeilen direkt nachgelesen (11,600 s). |
| Alle zwölf `scripts/check-*-contract.py` | Bestanden einschließlich neuem Importwächter. |
| `python3 scripts/check-docs.py`, `git diff --check` | Bestanden: 893 Dateien, 0 Fehler; Diff ohne Whitespacefehler. |

Tests belegen unter anderem: SQL Stop/Continue und Fehlerpositionen, W-P11-Restore,
Schlussmarkenabweisung vor Schreiben, explizite Zustimmung, lange Uploads bei unverändertem
Standardlimit, GZip-Bombe, malformed CSV, NULL/Leerwert, Quotes/Zeilenumbrüche, Latin-1,
Planänderungen (Datei/Optionen/Schema), Fehlerkürzung, Typteilmenge, Skip/Stop bei
Eindeutigkeits-/CHECKfehlern, bestehendes SQLite REPLACE bleibt ohne Löschung, Abbruch
nach einer eingefügten Zeile mit Rollback, SQL-Zeitlimit und Rest nicht ausgeführt,
unbekannter Commit bei deferred FK sowie Trigger-IGNORE ohne falschen Insertzähler.

## Nicht ausgeführt, Jobs und Übergabe

`docker ps --format '{{.Names}} {{.Status}}'`: Zugriff auf Docker-Socket verweigert.
Daher kein `scripts/test-import.py --live`: keine echten W-P12-Läufe gegen MySQL 8.4,
MariaDB 12.3 und PostgreSQL 18.6 behauptet. Deren Platzhalter sind keine Nachweise.
Geschriebene Livefälle: Direktnachlesen, synthetischer Fremddump, echter W-P11-Restore,
Skip/Savepoints, Dateischema-/Hashwechsel, SELECT-Rolle ohne INSERT, MyISAM-Abweisung,
Routinen mit Delimiter/Dollar-Quotes und SQL-Zeitlimit mit unangetastetem Rest.
Echte Verbindungsverluste/Sperrkonflikte, maximal große gleichzeitige reale DB-Importe,
Browserintegration, Compose/Swarm und öffentliche CI nicht ausgeführt. Der Listener-
Regressionstest benötigt einen Host mit erlaubtem TCP-Bind. Kein unabhängiger Review;
Sicherheitsreview und Leadintegration bleiben erforderliche Gates.

Keine eigenen Container, externen Dienste oder Hintergrundserver gestartet. Kurzlebige
Go-Prüfjobs werden vor Abschluss ausgewertet; Endstand dazu unten. Nächster konkreter
Schritt des Leads: gesamten uncommitteten Diff einschließlich neuer Dateien unabhängig
reviewen, Vier-DB-Lauf/Listenerprüfung auf geeignetem Host ausführen, Vertrag/ADR abnehmen,
Featurezählung mit parallel integrierten Paketen abgleichen und erst dann committen.
Zentrale STATUS-/ARBEITSPLAN-/Vertragsindex-/Paritätsänderungen sind Leadbesitz.

## Zuordnung zum Inventar

| IDs | Tatsächlich implementierter Teil / Grenze |
| --- | --- |
| PMA-IMP-001, PMA-NEU-IMP-001 | Browserdatei in expliziten DB-/Tabellenkontext; keine Serverdateiauswahl |
| PMA-IMP-002, PMA-NEU-IMP-004 | Backend akzeptiert Dateiauswahl/Drag-and-drop als Upload; Browserbedienung ist Webpaket |
| PMA-IMP-004 | GZip; ZIP/BZip2 bleiben abgewiesen |
| PMA-IMP-005 | Same-Dialekt-SQL über gemeinsamen Lexer, Statementstatus, keine allgemeine Objekt-/Backup-Parität |
| PMA-IMP-006, PMA-IMP-014 | Explizite CSV-Optionen, Typprüfung, Planbestätigung, feste Zieltabellen; keine vollständige Typ-/Pluginparität |
| PMA-IMP-007 | Keine automatische Tabellenanlage in dieser ausdrücklich begrenzten Stufe |
| PMA-IMP-008, PMA-IMP-003 | LOAD DATA LOCAL INFILE und Serververzeichnis bewusst ausgeschlossen |
| PMA-IMP-013, PMA-NEU-IMP-002/003 | Pollfortschritt/Cancel ab vollständig gelesenem Upload; Uploadfortschritt browserseitig, keine Resumezusage |
| PMA-NEU-ADMIN-008 | Kein serverweiter USE-/Mehrdatenbankimport in dieser Stufe |

## Abschlusscheckpoint

Endgültiger Vertrag SHA-256:
`f31875c9be8a86533766d454fe1e5f4cf055557b8ff35ae2e261a1f79abc9b27`.
Getrennte strikte Metadatenschemas für Plan und Ausführen prüfen auch fehlende
Bestätigung, fehlenden CSV-Hash und unpassende Bestätigungsaktion; neue negative Fixtures.

Nach den letzten Produktänderungen vollständig bestanden:
`DBM_IMPORT_STRESS=1 DBM_IMPORT_DUMP=contracts/fixtures/import go test -race -count=1 -p 1 ./... -skip '^TestMySQLPerConnectionLoggerSuppressesServerInput$'`
(mit den oben genannten Cache-/Offlinevariablen). Alle Pakete grün, einschließlich
64-MiB-HTTP-/Enginefällen und Crash-Recovery: Datenbank 26,156 s, HTTP 3,864 s,
SQLite-Sandbox 3,222 s, SQL-Lexer 15,595 s. Die drei Netzwerk-Livefälle werden mangels
Fixtureports übersprungen. Keine Änderung des gesperrten Listener-Tests.

Der Lexer besitzt zusätzlich einen Context-aufrufbaren Importpfad; Abbruch wirkt auch
beim Trennen großer Dateien. Charsetkonvertierung liest höchstens die Dekodiergrenze
plus ein Byte, statt zuerst eine unbegrenzt vergrößerte UTF-8-Kopie anzulegen.
CSV behandelt einen mit automatischem Rollback konkurrierenden ErrTxDone konservativ
als unbekannt, bis eine eigene Rollbackbestätigung vorliegt.

Alle zwölf Vertragswächter erneut grün. Zusätzlicher Fuzzlauf:
`go test ./internal/sqlsplit -run '^$' -fuzz FuzzSplit -fuzztime=10s -parallel=2`
bestanden (11,290 s, 42.957 Ausführungen, kein Absturz). Cache ausschließlich im
oben genannten Scratchverzeichnis. Kein neuer Fehlerkorpus im Repository.

Abschließend `go vet ./...`, `gofmt -l .`, Python-Syntaxprüfung der beiden neuen
Skripte, Vertragswächter, `scripts/check-docs.py` (893 Dateien, 0 Fehler) und
`git diff --check` bestanden. Ergänzter gezielter Race-Test mit zitiertem Tabellen-/
Spaltennamen und SQL-Text als gebundenem Zellwert: `TestSQLiteImportQuotedTargetsAndBoundValues`;
Nachbartabelle bleibt unverändert; Test bestanden (Race, 1,537 s). Keine eigenen
Prüfjobs, Container oder Hintergrundserver verbleiben.
