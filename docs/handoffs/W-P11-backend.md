# Handoff W-P11 Backend: Export und SQL-Dump

Stand: 24.10.2026. Implementierter Reviewkandidat, **keine Selbstfreigabe, Integration
oder Veröffentlichung**. Branch `feat/w-p11-backend`, Basis `main` `5921a9d`;
Arbeit ausschließlich im zugewiesenen Worktree `.worktrees/w-p11-backend`.
Keine Git-Schreibbefehle ausgeführt; Produkt-/Test-/Vertragsdateien sind uncommittet.
Lead übernimmt Review, Commit und spätere Integration. Keine zentrale Status-/Planungsdatei,
kein Vertragsindex, keine Webassets und kein Weblock verändert.

## Ergebnis und Dateibesitz

- `internal/core/export.go`, `export_test.go`: separates Engineinterface, Requestmodell,
  feste Obergrenzen und gemeinsame Optionsvalidierung vor I/O.
- `internal/database/export.go`, `export_rows.go`, `export_format.go`: begrenzte Metadaten,
  zeilenweiser Stream, SQLite-Speicherklassen, SQL-/JSON-/CSV-/Excel-CSV-Encoder,
  begrenzte INSERT-Stapel, Snapshots und garantierte Verbindungsfreigabe.
- `internal/database/export_test.go`, `export_live_test.go`: echter SQLite-Restore,
  unabhängige CSV-/JSON-Leser, Sonderwerte/Identifiers, Optionen, Byte-/Zeilen-/Zeit-/
  Statementgrenzen, Abbruch; vorbereitete Livefälle für MySQL/MariaDB/PostgreSQL.
- `internal/httpapi/export.go`, `export_test.go`: authentisierte v11-Routen in
  Standardklasse, verzögerter Streambeginn, Dateinamenprüfung, GZip, eigener
  Schreibzeitrahmen und deklarierte Abschluss-/Fehlertrailer.
- Registrierung: eine zusätzliche Zeile in `internal/httpapi/features.go`;
  Konfliktstelle `features_test.go` nur Featurezahl 7 → 8. Eigene Routentests im Exporttest.
- Kleine notwendige Erweiterung in `internal/database/overview_postgres.go`:
  `postgresDefinition` akzeptiert das bestehende `metadataQuerier` statt nur `*sql.DB`.
  Dadurch kann der Export denselben Katalogrenderer auf seiner gepinnten Verbindung
  im Snapshot verwenden. Keine SQL-/Antwortänderung des bestehenden v7-Pfads.
- Verträge: [API 0.11](../../contracts/api-v0.11.0.json),
  [Verhalten](../../contracts/EXPORT-API.md), [gemeinsames Format](../../contracts/EXPORT-FORMAT.md),
  [Formatschema](../../contracts/export-format-v1.schema.json),
  [Fixtures](../../contracts/fixtures/export/README.md).
- `scripts/check-export-contract.py`, `scripts/test-export.py`,
  `.github/workflows/export.yml`: Byte-/Additivitäts-/Schemawächter, Dockerfreier
  SQLite-Lauf, optionaler Vier-DB-Lauf mit `dbfix.py`, CI als Reviewkandidat.
- [ADR W-P11](../decisions/ADR-W-P11-exportstream.md): vorläufige Entscheidung zum
  Formatabschluss; Nummer/Annahme durch Lead offen. Keine neuen Abhängigkeiten.
  Eigener Code mit SPDX AGPL-3.0-only; keine Attribution-Zeilen.

## Verhalten und bewusst sichtbare Grenzen

Download ausschließlich zum Browser; keine Exportpfade, temporären Dumpdateien,
`mysqldump`-/`pg_dump`-Aufrufe oder automatischen Retries. SQL als Standard mit CREATE
und vollständigen/erweiterten INSERTs, optional Struktur/Daten, DROP, IF NOT EXISTS,
BEGIN/COMMIT und treiberspezifischem FK-Rahmen. GZip, geprüfter Dateiname, UTF-8/BOM.
Auswahl von Tabellen, ganzer DB, ausdrücklich gewählten DBs oder lesendem Abfrageergebnis.
CSV genau eine Tabelle; SQL-Abfrageexport und SQL-Serverexport außerhalb MySQL abgewiesen.

Standard: 64 MiB unkomprimiert, 1.000.000 Zeilen, 60 Sekunden. Obergrenzen:
1 GiB, 10.000.000 Zeilen, 300 Sekunden. Zusätzlich Zell-/Zeilen-/Metadaten-/INSERTgrenzen.
Zeilen werden niemals als vollständige Tabelle gesammelt. Alle Formate haben einen
positiven Abschluss; Limits oder Fehler dürfen keinen gültigen Abschluss erzeugen.
CSV-Stream hat deshalb eine Kontrollspalte/Endzeile. Alte lokale F6-Profile bleiben
unverändert; gemeinsamer Vertrag beschreibt beide Profile getrennt. Schema-/Dateiversion
und API-Version bleiben getrennt. API 0.11 ist vorläufig und nicht im Weblock gepinnt.

Nur derselbe SQL-Dialekt, UTF-8, reguläre Tabellen und dokumentierte Typteilmenge.
Views/Routinen/Trigger/Events und weitere Kodierungen sind nicht implementierte
Objektklassen-Parität. SQL-DUMP-001/NEU-DUMP-001 sowie freie CSV-Einfassungs-/NULLprofile
bleiben teilweise abgedeckt; W-P19/W-P28 sind Folgepakete. Kein universeller Roundtrip.
PostgreSQL-Katalog-DDL lässt unter anderem Sequenzen, Sequenzstände, Trigger, Regeln,
Partitionierung, Rechte und Storageparameter aus; FK-/Sequenzabhängigkeiten können
Restorevorbereitung verlangen. Kein vollständiges Backupversprechen.

SQLite: ADR-0004 unverändert, lokal/exklusiv/eine Verbindung/DELETE-Journal.
FK-PRAGMA im optionalen Dumprahmen ist für externe SQLite-Clients; die bestehende
Sandbox-Query-API erlaubt dieses PRAGMA nicht. MySQL/MariaDB: nur InnoDB, READ ONLY /
REPEATABLE READ / CONSISTENT SNAPSHOT. PostgreSQL: READ ONLY / REPEATABLE READ, UTC.
Kein Snapshot über DB-Grenzen; freie Abfragen melden Read-only-Best-Effort, weil
externe/nichttransaktionale Datenquellen nicht universell erkennbar sind.

## Prüfstand

Alle Go-Befehle mit `GOCACHE=/private/tmp/dbm-wp11-go-cache`, weil der normale
Hostcache in der Sandbox nicht beschreibbar ist. Der erste unkonfigurierte Go-Aufruf
scheiterte am gesperrten Cache; anschließend kein Netzwerkdownload erforderlich.

| Befehl | Tatsächlicher Ausgang |
| --- | --- |
| `gofmt -l` auf eigenen/geänderten Go-Dateien | ohne Ausgabe, auch `gofmt -l .` auf dem gesamten Worktree |
| `go vet ./...` | bestanden; abschließende Wiederholung nach letzten Produktänderungen bestanden |
| `go test -race -count=1 ./...` | ausgeführt; bestehender `TestMySQLPerConnectionLoggerSuppressesServerInput` scheitert am von der Sandbox verbotenen TCP-Listener. Übrige Pakete im ersten Gesamtlauf grün |
| `go test -race -count=1 ./... -skip '^TestMySQLPerConnectionLoggerSuppressesServerInput$'` | Datenbank-/HTTP-/Coreprüfungen grün; erster gefilterter Lauf mit Timeout im bestehenden `TestCrashRecovery`; gezielte Wiederholung bestanden (2,397 s), abschließender gefilterter Gesamtlauf in allen Paketen bestanden |
| `python3 scripts/test-export.py` | SQLite-Restore (Einzel-/Mehrzeilen-INSERT), Schema-/Wertevergleich, JSON-/CSV-/HTTP-/GZip-Prüfungen und Schemaabgleich realer Antworten bestanden; abschließend auch Scope-, Midstream-Abbruch-, Dauer-, Einzelstatement- und Batchgrenzen grün (Datenbank 2,086 s, HTTP 1,407 s, jeweils Race) |
| `python3 scripts/check-export-contract.py` | unveränderte v1–v7-Bytes, additive v11-Pfade/Schemata, Referenzen, gemeinsames Schema, 18 JSONfixtures und 3 Streamfixtures bestanden |
| übrige `scripts/check-*-contract.py` | alle sechs bestehenden Wächter bestanden; sieben einschließlich Export |
| `python3 scripts/check-docs.py` | 518 Dateien, 0 Fehler vor Abschlusscheckpoint; Endprüfung danach ebenfalls erforderlich und unten erfasst |
| `git diff --check` | bestanden |

Die SQLite-Nachweise verwenden ausschließlich synthetische Daten: große Integer,
präziser Dezimaltext, leere Werte/NULL, Binärwerte (auch leer), NUL/Backslash/Quote/Unicode,
Zeittexte, besondere Identifier, rekonstruierte Tabellen und eigenständige Indizes.
Netzwerk-Livefälle sind geschrieben, aber **nicht ausgeführt**; deren Erfolg wird
hier ausdrücklich nicht behauptet.

## Nicht ausgeführt und nächste Schritte

Dockerprobe `docker ps --format '{{.Names}} {{.Status}}'`: Zugriff auf den Docker-Socket
verweigert. Deshalb kein `scripts/test-export.py --live`, keine realen MySQL-8.4-/
MariaDB-12.3-/PostgreSQL-18.6-Roundtrips, keine Live-Snapshot-/Rechte-/MyISAMproben und
keine Compose-/Swarmtests. Livefälle verwenden `scripts/dbfix.py`, frische Zieldatenbank,
Direktabfrage zum Vergleich, unabhängigen konkurrierenden Writer, UTC-Ausgabe,
fehlende Rechte/falsches Passwort und nichttransaktionale Tabellen. Auch vorhandene
andere Livefälle ohne Fixtureports werden vom Go-Test als übersprungen gemeldet.
Keine fremden Container oder Jobs geändert; keine eigenen Container gestartet.

Keine Browser-/Webintegration, kein Assetneubau, keine öffentliche CI-Ausführung,
kein unabhängiger Review in dieser Workeraufgabe. Lead muss den uncommitteten Diff
reviewen, Vertrag/ADR bestätigen, den Vier-DB-Lauf und den gesperrten Listenertest auf
einem geeigneten Host ausführen und erst danach committen/integrieren. Zentralen Status,
Vertragsindex und Paritätsmatrix erst nach unabhängiger Abnahme aktualisieren.

## Zuordnung der Paketanforderungen

| Inventar-ID | Implementierter Teil / Grenze |
| --- | --- |
| PMA-EXP-001, PMA-NEU-EXP-002, PMA-NEU-ADMIN-007 | Tabellen/DB/ausgewählte DBs/lesende Abfrage; Zeilenauswahl über Abfrage, Mengenlimits; kein serverweiter Snapshot |
| PMA-EXP-002 | dokumentierte sichere Vorgaben und angepasste Requests; UI ist eigenes Webpaket |
| PMA-EXP-004, PMA-NEU-EXP-004 | Downloadname, GZip, UTF-8/BOM; andere Zeichensätze/Kompressionen abgewiesen |
| PMA-EXP-006, PMA-DUMP-001 | CREATE/INSERT und Indizes regulärer Tabellen, DROP/IF NOT EXISTS; übrige Objektklassen offen |
| PMA-EXP-007, PMA-EXP-008 | getrennte CSV-/Excelprofile, Trenner/Zeilenenden, eindeutige NULL-/Binärmarker; Einfassung/Escape fest, frei konfigurierbare Profile offen |
| PMA-EXP-009 | typisierte JSON-Zellen, DB-/Tabellenzuordnung, geordnete doppelte Namen |
| PMA-DUMP-002 | vollständige oder erweiterte INSERTs mit konfigurierbarer Anweisungslänge |
| PMA-DUMP-004 | Hex-Binärliterale, exakte Dezimalstrings und UTC-Netzwerksitzungen; SQLite erhält Speicherklassen/Zeittexte |
| PMA-DUMP-005 | optionaler Transaktions-/FK-Rahmen mit ausdrücklich dokumentierten Treibergrenzen |
| PMA-NEU-EXP-001 | begrenzter laufender Download statt persistenter Sitzung/Polljob; Kontext und Schreibdeadline |
| PMA-NEU-DUMP-001 | nur Quell-/Zieldialekt identisch; Cross-Dialekt-Konvertierung abgewiesen |

Die Zuordnung ist ein Teilnachweis des Backendkandidaten, keine Freigabe oder Änderung
der zentralen Paritätsmatrix. Unabhängiger Review bleibt ein Leadgate.

## Abschlusscheckpoint

Verifizierter unveränderter HEAD: `5921a9dc1b813cc63b212fa5579f813b1595b761`,
Branch `feat/w-p11-backend`. SHA256 der abzugebenden Vertragsdateien:

| Datei | SHA256 |
| --- | --- |
| `contracts/api-v0.11.0.json` | `5855e97408029bba128ec47659e58758d622e24aedd7b6001eb294aa91ba02fe` |
| `contracts/export-format-v1.schema.json` | `f21a9e7d97237b91a4d191b4ac9926677c9565c4073ca00f54964eb21d57267b` |

Abschließend `gofmt -l .` ohne Ausgabe, `go vet ./...` bestanden, alle sieben
Vertragswächter bestanden. Der letzte Gesamtlauf mit ausschließlich dem gesperrten
Listenertest ausgeschlossen war vollständig grün (Datenbank 25,287 s; Sandbox 4,529 s).
Danach nur zusätzliche Batchgrenz-/GZip-Paritätstests und Schemafixtures; diese sind
im letzten `scripts/test-export.py` mit Race und echten SQLite-/HTTP-Bytes bestanden.
`python3 scripts/check-docs.py`: 518 Dateien, 0 Fehler. `git diff --check` bestanden;
zusätzlich eigene neue Textdateien auf nachgestellten Leerraum geprüft.

Eigene lokale Testprozesse sind beendet; keine Server, DBs oder Container laufen.
Ein temporärer Go-Buildcache bleibt außerhalb der Produktdateien; keine
Credentialdateien oder DB-Fixtures bleiben zurück. Kein Gitcommit erzeugt, kein
Webpin verändert. Der Handoff W-P07-backend und ein Verzeichnis `docs/contracts`
waren in dieser Basis nicht vorhanden; vorhandene Verträge unter `contracts/`,
W-P01-/W-P06-/F5b-Handoffs, PWA-Plan, Inventare und ADR-0004 wurden gelesen.

Nächster konkreter Schritt für den Lead: gesamten uncommitteten W-P11-Diff einschließlich
aller neuen Dateien übernehmen und unabhängig reviewen; die oben ausdrücklich offenen
Live-/Listener-/Browsergates nachholen. Keine Aussage über erfolgreichen unabhängigen
Review, tatsächliche CI oder umfassende Matrixabnahme aus diesem Workercheckpoint ableiten.
