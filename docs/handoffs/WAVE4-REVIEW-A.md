# Review A Welle 4: W-P12 Import (API 0.18) und W-P13 Mehrzeilen/Dateiwerte/FK-Lookup (API 0.19)

Unabhängiger Review (nichts implementiert, kein Produktcode geändert) des Stands
`integration/wave4-backend` `6b74f3b`. Review-Branch `review/wave4-a`. Dieser Bericht ist ein
Prüfergebnis, keine Annahme von ADR oder Vertrag; nichts wurde gemergt.

## Urteil

**Freigabe für W-P12 und W-P13 unter Auflagen. Kein Blocker.** Kein Fund erlaubt Datenverlust
ohne Bestätigung, Rechteausweitung, Sandbox-Ausbruch, Rohfehler-/Secret-Leck oder unbegrenzten
Ressourcenverbrauch. Plan, Hash, Bestätigung und Aktionsname werden für CSV-Import und Mehrzeilen
strikt erzwungen (Mutationsproben unten). Vor Web-Integration/Release sind F1 (Replay) und F2
(SQL-Import ohne Plan) zu entscheiden bzw. in Vertrag/UI zu dokumentieren; F3 bis F8 sind
Nacharbeiten. Es gibt keine Befunde der Schwere Hoch oder Kritisch.

## Ausgeführte Prüfungen (alle tatsächlich gelaufen)

| Prüfung | Ergebnis |
| --- | --- |
| `gofmt -l .`, `go vet ./...` | ohne Ausgabe/Fehler |
| `go test -race -count=1 ./...` | alle Pakete grün (inkl. eigener Angriffstests, Funde per Gate übersprungen) |
| alle 21 `scripts/check-*-contract.py`, `check-docs.py` | grün |
| `scripts/test-import.py --live` | grün: 15 echte Antworten gegen v18 (SQLite, MySQL 8.4, MariaDB 12.3, PostgreSQL 18.6, Wegwerfcontainer nacheinander) |
| `scripts/test-rows-bulk.py` | grün: 14 echte Antworten gegen v19 (vier Systeme) |
| `scripts/test-review-a-live.py` (neu, eigene Angriffstests) | auf allen vier Systemen grün bis auf die bewusst ausgeführten Fundnachweise (`DBM_REVIEW_A_FINDINGS=1`: F1, F3, F4, F2 schlagen auf allen Engines wie unten beschrieben an) |
| Mutationsproben | 38 Mutanten, siehe unten |

Eigene Tests: `internal/database/wave4_review_a_test.go` (SQLite immer, MySQL/MariaDB/PostgreSQL über
dbfix-Umgebung), `internal/httpapi/wave4_review_a_test.go`, `scripts/test-review-a-live.py`.
Tests, die einen offenen Fund belegen, heißen `*_Finding*` bzw. `PartialStateHonesty` und laufen
nur mit `DBM_REVIEW_A_FINDINGS=1`.

## Befunde

### F1 (Mittel) Bestätigter Plan ist wiederholbar: Import und Kopieren duplizieren Daten
Der Plan-Hash bindet Datei/Optionen/Ziel/Schema, ist aber nicht einmalig verwendbar und die Anfrage trägt
keinen Idempotenzschlüssel. Dieselbe bestätigte Execute-Anfrage fügt die CSV erneut ein (Tabelle ohne
Unique-Schlüssel). Bulk-Kopieren mit Auto-Schlüssel (Quellzeile unverändert) erzeugt bei Wiederholung
eine weitere Kopie. Praktisch: Netzwerk-Retry nach verlorenem 202 startet einen zweiten Job; zwei Slots
laufen parallel. Bulk-Löschen/-Update sind dagegen gesichert (Wiederholung ergibt `row_conflict`).
Reproduktion: `DBM_REVIEW_A_FINDINGS=1 go test ./internal/database -run 'TestReviewA_Finding(CSVImportReplay|BulkCopyReplay)'`
(SQLite; live auf MySQL, MariaDB, PostgreSQL identisch: Import 4 statt 2 Zeilen, Kopie 3 statt 2).
ADR W-P13 und IMPORT-API benennen "kein einmalig verwendbarer Plan" ausdrücklich; dennoch Lücke gegenüber
dem Soll "Replay". Empfehlung: serverseitig verbrauchte Plan-Hashes (TTL, im Speicher) ablehnen oder
Idempotenzschlüssel; mindestens UI-Schutz (Doppelklick/Retry) und Vertragshinweis.

### F2 (Mittel, dokumentierte Designentscheidung) SQL-Import hat keinen Plan/Hash/getippten Namen
`format=sql` verlangt nur `confirm.action=import_sql`; `ExpectedPlanSHA256` ist dort verboten, ein SQL-Plan
existiert nicht. `DELETE FROM t; DROP TABLE t` läuft damit nach der einfachen Bestätigung durch
(`TestReviewA_FindingSQLImportDestructiveWithoutTypedName`). IMPORT-API dokumentiert dies als "ausdrücklich
bestätigtes Nutzer-SQL mit echten DB-Rechten" (wie W-P15, "kein Sandbox"), widerspricht aber dem strikten
Plan->Bestätigen->Ausführen mit getipptem Namen bei Destruktivem. Empfehlung: Dateihash in der Bestätigung
und UI-Pflicht zu Warnung/Anweisungsliste, oder Erkennung destruktiver Anweisungen mit getipptem Zielnamen.
Die Sperrliste (ATTACH, VACUUM INTO, load_extension, writable_schema, SET, START, USE, LOAD, COPY u. a.) hält:
SQLite lässt nur genau eine Anweisung zu (Tail-Prüfung), `VACUUM INTO`/`load_extension`/`PRAGMA
writable_schema` scheitern in der Sandbox, kein Host-Dateizugriff; MySQL ohne MultiStatements, PostgreSQL
Prepare-Pfad.

### F3 (Niedrig) Einzelstatus "completed" bleibt nach späterem Rollback stehen
Datei `CREATE TABLE a; BEGIN; INSERT; CREATE TABLE b; INSERT (Fehler); COMMIT`: Gesamtergebnis ist
ehrlich `outcome_unknown`/`partial_or_unknown`, aber die Statements 3 und 4 stehen auf `completed`, obwohl
SQLite und PostgreSQL (transaktionales DDL) sie durch das abschließende `ROLLBACK` zurückgenommen haben
(Zeilen 0). MySQL/MariaDB: Zeile bleibt durch implizites DDL-Commit (Status stimmt). Reproduktion:
`TestReviewA_SQLImportPartialStateHonesty`. Empfehlung: Statements seit dem letzten BEGIN/COMMIT bei
Rollback auf `outcome_unknown` setzen und im Vertrag klarstellen.

### F4 (Niedrig) Datei ersetzen nahe am Limit schlägt mit `invalid_bulk` fehl
Vertrag: Dateiwert bis 512 KiB. Wird ein vorhandener 400-KiB-Wert durch 400 KiB ersetzt, überschreitet
die Anfrage (alter Snapshot + neue Bytes, beide Base64) das 1-MiB-Budget in `bulkValidate`; Antwort 400
`invalid_bulk` statt `file_too_large`/Hinweis. Handoff nennt die Grenze, aber der Fehlercode führt irre.
Reproduktion: `TestReviewA_FindingFileReplaceNearLimit` (alle vier Systeme).

### F5 (Niedrig) Speicherverstärkung beim Import
Eine 62-MiB-CSV-Plananfrage hebt den Heap um etwa 310 MiB über die Ausgangslage (Spitze 375 MiB bei 64 MiB
Basis; Lesepuffer, entpackte Kopie, Dekodierkopie, String-Kopie). Zwei Slots ergeben mehr als 600 MiB
Spitze; ein Containerlimit ist im Repo nicht gesetzt. Grenzen selbst halten (Gzip-Bombe 600 MiB, Multi-Member
1,2 GiB, Zeichensatzexpansion 40 auf 80 MiB werden begrenzt; Heap bleibt unter 400 MiB). Empfehlung:
Speichergrenze dokumentieren/Compose-Limit, `io.ReadAll` durch vorallokierte Größe ersetzen, Strings nicht
kopieren.

### F6 (Niedrig/Info) Abbruch einer CSV-Importausführung je Engine uneinheitlich
Abbruch nach 500 Zeilen: SQLite, MySQL, MariaDB `rejected`/`query_canceled`, `inserted=0`. PostgreSQL
`outcome_unknown`/`query_canceled` mit `inserted=500`, obwohl 0 Zeilen persistent sind (konservative
`ErrTxDone`-Behandlung). Ehrlich-unsicher, aber `inserted` irreführend und Engines unterschiedlich.

### F7 (Info) Mehrzeilen-Löschen bis 100 Zeilen mit boolescher Bestätigung
Kein getippter Name; konsistent mit Einzelzeilen-Löschen (v4) und durch Planhash/Stabilitätsmerkmale/
Zeilenabgleich gestützt. Für das UI getippte Anzahl oder Tabellenname empfohlen.

### F8 (Info) Weitere Beobachtungen
- Job-IDs (128 Bit Zufall) und Status/Abbruch sind nicht an einen Aufrufer gebunden; Einzeltokenbetrieb.
- Jobs laufen mit `context.Background()` ohne Shutdown-Kopplung (Prozessende rollt zurück).
- Kontextprüfung in CSV-/Lexerschleife nur bei `pos % 4096 == 0`; kann bei sprungweisem Vorrücken übersprungen
  werden (Abbruch folgt spätestens am Satzende/Zeilenende).
- Ergebnisablage 16 Einträge, 5 Minuten lazy Expiry: 16 fertige Ergebnisse sperren neue Starts (Test belegt
  Refus Nr. 17).
- FK-Lookup: kein gemeinsamer Schemasnapshot, Offset bis 1.000.000 kann teure Scans auslösen (Timeout bis 60 s).

## Positiv belegt (Angriffstests, SQLite und, wo engineabhängig, alle vier Systeme)
- Upload: 65-MiB-Grenze, unbegrenzter Chunked-Body wird bei etwa 67 MiB mit 413 beendet; doppelte/zusätzliche/
  fehlende Parts, Nicht-Multipart, Metadaten über 1 MiB, unbekannte Felder werden mit 4xx abgewiesen ohne
  Leck; Slots (2) werden bei Fehlern wieder freigegeben, dritter paralleler Upload 503.
- Zeichensatz/Format: NUL, ungültiges UTF-8, UTF-16-BOM, UTF-8-BOM-Entfernung, CR-only, Quote-Fehler,
  1-MiB-Zelle exakt am Limit, 100.001 Zeilen, Separator-/Quote-Validierung; SQL-Injektion als gebundener Wert
  bleibt literal; Zielnamen/Spalten mit Injektionsversuchen und Systemschemata werden abgewiesen.
- Plan/Bestätigung: fehlende/falsche Aktion, fehlender oder großgeschriebener Hash, anderer Inhalt, Tabelle,
  Modus, Spalten, Timeout und Schemadrift werden abgelehnt; abgelehnte Anfragen schreiben nichts.
- Zeitlimit/Abbruch: SQL-Import mit 1,5 s Limit endet nach 1,5 s auf allen Engines, Rest `not_executed`;
  Abbruch per API markiert `cancel_requested`, kein `completed`.
- Fehlertexte: falsches Passwort/fehlende Tabelle liefert keine Passwörter, Hosts, Tabellennamen oder
  Servertexte, nur feste Codes.
- Bulk: Auswahl ohne PK, Duplikate, Phantomzeilen, Stale-Plan, gefälschter Hash, fehlendes `confirm`,
  UNIQUE-Konflikt (kein Teilzustand), PK-Änderung, Wiederholung von Delete, veralteter Kopier-Snapshot.
- Dateiwerte: nur Binärspalten außerhalb des PK, Injektionsname, Übergröße (513 KiB, 2 MiB), 512 KiB exakt,
  Stale-Snapshot; Download immer `application/octet-stream`, Attachment, nosniff, CSP sandbox, no-store;
  Vorschau nur PNG-Neukodierung, SVG/HTML/Header-Bomben/9-MP-PNG abgewiesen.
- FK-Lookup: `%`/`_`/`'`/`!`-Suche literal, Anzeigespalte-Injektion, Seitenlimits, 257-Byte-Suche,
  Nicht-FK-Spalte, zusammengesetzter FK abgewiesen.
- Alle v19-Routen authentifiziert; JSON-Routen weisen falsche Content-Types ab.

## Mutationsproben (38 Mutanten)
Je Paket Mutation im Produktcode, Prüfung mit der Paket-Testmenge. Getötet: 30 von 38 (8 überlebt, davon 5 äquivalent bzw. durch Folgeprüfung abgedeckt, 3 Testlücken: M17, Plan mit Confirm, Metadaten-zuerst).

Paket `internal/database` (Import): getötet: Gzip-Grenze (M01, erst nach Ergänzung der Heap-Assertion), NUL-, UTF-8-
Prüfung, Engine-Plan-Check, `OR ABORT` zu `OR REPLACE`, Stop bei Validierungsfehler, Rows-affected-Prüfung,
Sperrliste, Schlussmarke, Stop-Modus, Separator=Quote, Zeilenlimit. Bulk/Dateien/FK: Stabilitätsmerkmal,
Snapshot-Vergleich (erst nach neuem Test `BulkCopyStaleSnapshot`), Duplikatprüfung, Plan-Hash, Confirm-Flag
(erst nach `BulkExecuteNeedsConfirmFlag`), Nachherprüfung, LIKE-Escape, Binärtyp-Beschränkung, Pixelgrenze
(erst nach `PreviewLargeValidPNG`), Upload-Limit, Seitengrößenlimit. Überlebt: Metadaten-Neulesen nach
Sperren (M17; nur mit gleichzeitigem DDL testbar, ungeprüft), Vorschau-Formatprüfung und FK-Composite
(äquivalente Mutanten: nur png/jpeg/gif-Decoder registriert, Spaltenzahl wird vorher geprüft).

Paket `internal/httpapi`: getötet: Slot-Freigabe, MaxBytesReader, Plan-Check, Aktionsprüfung, Duplikat-Part,
Jobablage-Limit (erst nach `ImportJobStoreBounded`), Attachment-Header. Überlebt: Part-Limit und
Dateilimit im HTTP-Rahmen (Engine/Folgeprüfung fängt es; nur anderer Fehlercode), nosniff (äquivalent, globale
Middleware setzt ihn ebenfalls), Plan mit Confirm, Metadaten-zuerst.

## Offen/nicht geprüft
Echte Verbindungsverluste während Commit, Sperrkonflikte, gleichzeitiges DDL (M17), maximale parallele reale
DB-Importe, Browser-/Compose-/Swarm-Pfade, native Linux-CI. Die Prüfung der `mutationSession`-Extraktion
(`rowsBulkInsert`/`rowsBulkMutate`) erfolgte per Diff: Einzelzeilenverhalten unverändert.
