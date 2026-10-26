# Handoff W-P28 Backend: Erweiterte Textformate und Dump-Details

Stand: 25.10.2026. Implementierter Reviewkandidat auf `feat/w-p28-backend`,
Basis und unveränderter HEAD `c331c642fa4957747845458b079cffc32f6b1ff6`.
Arbeit ausschließlich im zugewiesenen Worktree `.worktrees/w-p28-backend`.
Keine Git-Schreibbefehle, kein Commit, keine Integration oder Veröffentlichung.
Lead übernimmt unabhängigen Review, Commit und Integration. Zentrale STATUS-/
ARBEITSPLAN-Dateien, Vertragsindex, Paritätsmatrix und Webassets bleiben beim Lead.

## Ergebnis und Dateien

- `internal/core/export.go`, `export_extended.go`, `export_extended_test.go`:
  additive Requestfelder, eigene v21-Normalisierung, strikte Dekodierung mit Erhaltung
  der TLS-Vorgabe, Optionspräsenz für v11 und UTF-8-Prüfung. Die bisherige
  `NormalizeExport`-Funktion und v11-HTTP-Oberfläche weisen neue Optionen ab.
- `internal/database/export.go`, `export_format.go`, `export_text.go`,
  `export_details.go`: zeilenweiser Export von XML, YAML, PHP-Array, LaTeX tabular/
  longtable, MediaWiki, Texy und CodeGen-Text. Vorhandene Limits, Slots, Kontext,
  Snapshots und Trailerpfade bleiben maßgeblich. Views als Tabellen, sichere
  Katalogkommentare, begrenzte Kompatibilität und Datenanweisungen, Katalogaliase.
- `internal/database/export_extended_test.go`: echte SQLite-Parser-/Werte-/
  Speicherklassen-Roundtrips, Sonderzeichen und doppelte Namen, Referenzbytes,
  Limits/Fehler/Abbruch/UTF-8, View-Restore, Aliase und Konfliktanweisungen;
  neue Livefälle für MySQL/MariaDB/PostgreSQL mit unabhängiger Restore-DB.
- `internal/httpapi/export.go`, `export_extended.go`, `export_extended_test.go`:
  authentisierte eigene v21-Routen, ehrliche Treiberfähigkeiten, MIME/Dateiendungen,
  GZip, Trailer, Schutz des alten Optionsumfangs. `features.go` genau ein neuer
  Eintrag, `features_test.go` ausschließlich Anzahl 12 → 13.
- [API 0.21](../../contracts/api-v0.21.0.json),
  [Verhaltensvertrag](../../contracts/EXTENDED-EXPORT-API.md),
  [synthetische Platzhalterfixtures](../../contracts/fixtures/extended-export/README.md),
  ergänzender Link in `contracts/EXPORT-API.md`. Alle v1–v12-Vertragsbytes und
  `export-format-v1.schema.json` unverändert.
- `scripts/check-extended-export-contract.py`: feste SHA256-Pins aller v1–v12,
  Additivität zu v12, Referenzen, Authentisierung, strikte neue Schemas und Fixtures;
  v13–v21 sind optional, werden nicht vorausgesetzt.
- `scripts/test-extended-export.py`: SQLite/HTTP mit Race, echte Fähigkeitsantwort
  gegen Schema; optional seriell `dbfix.py` für MySQL 8.4, MariaDB 12.3, PG 18.6.
  `.github/workflows/extended-export.yml`: vorbereitete CI einschließlich Livefällen.
- [ADR W-P28](../decisions/ADR-W-P28-text-export.md): begrenzte Formatprofile und
  Katalogziele als Vorschlag; Annahme durch Lead offen. Keine neue Abhängigkeit,
  eigener Code mit SPDX AGPL-3.0-only, keine Attribution-Zeilen.

## Fachliche Grenzen und Zuordnung

EXP-010/015..020 sind als eigene dokumentierte Textprofile implementiert, keine
Bytekompatibilität mit phpMyAdmin-Plugins. YAML ist JSON-kompatibles YAML 1.2;
PHP-Metadaten stehen als JSON-String, Zellarrays sind typisiert. CodeGen ist ein
nicht ausführbares JSON-Zeilenprofil, kein C#/NHibernate-Generator. Die sichtbaren
Formate erhalten Zelltypen und zeigen Steuerzeichen als Escapenotation. Keine
vollständige universelle Roundtrip- oder Rendering-Paritätsaussage.

DUMP-003: INSERT/IGNORE/REPLACE für SQLite und MySQL/MariaDB; PG nur INSERT.
DUMP-006 bleibt begrenzt auf DEFINER-Auslassung durch Viewmaterialisierung und
`native`/`data-only`; keine Erhaltung/Ersetzung fremder DEFINER oder historischen
MySQL-/Cross-Dialektmodi. DUMP-007: zusätzliche sichere Katalognamenskommentare und
Aliase nur zu existierenden strukturgleichen Zielen für SQL-Datenexport; native
DDL-Kommentare bleiben erhalten. Keine Konfigurationsspeichermetadaten, freie
DDL-Aliasumschreibung oder Entfernung von Tabellen-/Spaltenkommentaren.
DUMP-008: Viewdaten als reguläre Tabellen ohne Viewlogik/Defaults/Schlüssel/Eigentümer;
SQLite ohne Typaffinität, Netzwerk nur skalare Typteilmenge. Bei Views wird der
Snapshot konservativ als Best-Effort gemeldet. Externe Quellen und nichttransaktionale
Viewquellen erhalten kein Transaktionsversprechen.

DUMP-001 bleibt bei Triggern/Routinen/Events offen, weil W-P19 in dieser Basis nicht
integriert ist. DEFINER-Objektdefinitionen, Rechte und Restoreabhängigkeiten dieser
Objektklassen sind kein implementierter Nachweis. DUMP-002/004/005 nutzen die
bestehenden begrenzten W-P11-Pfade. EXP-003/NEU-EXP-003 (Vorlagen), EXP-005/IMP-003
(Serververzeichnisse), EXP-011..014 (ODS/ODT/Word/PDF, E8) und IMP-009..011
(ODS/XML/MediaWiki-Import, W-P12) sind außerhalb dieses Auftrags, keine Paritätsabnahme.

Jeder Client muss Format und letzte strukturelle Schlussmarke prüfen; HTTP 200,
GZip-Abschluss und geschlossene Einzeltabelle reichen nicht. Trailer können im Browser
fehlen. Ein Textwert mit Markerinhalt ist kein gültiger Dokumentabschluss. Daten werden
nicht gesammelt und Quelldaten nicht verändert; Restore erst nach Abschlussprüfung.

## Prüfstand

Alle Go-Läufe verwenden einen beschreibbaren temporären Cache und
`GOPROXY=off GOSUMDB=off`; keine Downloads oder neuen Abhängigkeiten.

- `python3 scripts/test-extended-export.py`: bestanden mit Race; alle acht
  Textvarianten, XML-/YAML-Parser, Referenzbytes, SQLite-View-Restore und Katalogaliase,
  Konfliktvarianten, Dauer-/Byte-/Zeilenlimits, Abbruch, GZip, HTTP-Auth und Trailer.
  Tatsächliche Fähigkeitsantwort gegen v21-Schema validiert. Livefälle ohne Ports
  ausdrücklich übersprungen. Nach letzten Ergänzungen folgt der Abschlusslauf unten.
- Vollständiger Race-Lauf, Vet, sämtliche Vertragswächter und Dokumentationsprüfung:
  Abschlussresultate werden unten nachgetragen; keine Vorwegnahme grüner Ergebnisse.
- `docker ps --format '{{.Names}} {{.Status}}'`: Docker-Socketzugriff verweigert.
  Prozessübersicht `ps` ebenfalls durch Sandbox gesperrt; keine eigenen Dockerjobs
  gestartet. Eigene Go-Läufe werden über ihre Session bis zum Ende verfolgt.

## Nicht ausgeführt und nächste Schritte

Keine Docker-/Live-DB-Läufe, kein `scripts/test-extended-export.py --live`, keine
MySQL-/MariaDB-/PG-Roundtrips, Compose-/Swarm-/Browserprüfungen, kein externer PHP-/
LaTeX-/Wiki-/Texy-Renderer und kein unabhängiger Review oder öffentlicher CI-Lauf.
XML und die strikte YAML-Teilmenge haben unabhängige Parser; PHP/LaTeX/Wiki/Texy sind
lokal durch feste Referenzbytes und Injektionsfälle geprüft, kein Rendererbeweis.

Nächster Schritt: Lead übernimmt den vollständigen uncommitteten Diff einschließlich
neuer Dateien, lässt Vertrag/ADR/Backend unabhängig reviewen und führt die vorbereiteten
Vier-DB-Läufe auf geeignetem Host aus. Insbesondere die begrenzten DEFINER-/Kommentar-/
Kompatibilitäts-/CodeGen-Profile vor Freigabe fachlich abnehmen. Offene Teilparität
nicht als vollständige phpMyAdmin-Parität in die zentralen Dokumente übernehmen.

## Abschlusscheckpoint und exakte Ergebnisse

Prüfdatum 25.10.2026. Für die Go-Befehle:
`GOCACHE=/private/tmp/dbm-wp28-cache GOPROXY=off GOSUMDB=off`.
Der Cache liegt außerhalb der Produktdateien und enthält keine DB-Fixtures.

| Befehl | Tatsächlicher Ausgang |
| --- | --- |
| `gofmt -l .` | ohne Ausgabe |
| `go vet ./...` | bestanden, einschließlich Abschlusswiederholung |
| `go test -race -count=1 ./...` | ausgeführt; bestehender `TestMySQLPerConnectionLoggerSuppressesServerInput` scheitert an `listen tcp … bind: operation not permitted`; übrige Pakete bestanden |
| `go test -race -count=1 ./... -skip '^TestMySQLPerConnectionLoggerSuppressesServerInput$'` | Datenbank 72,637 s, HTTP 5,466 s und übrige Pakete bestanden; bestehender `TestCrashRecovery` scheitert am Kindprozess-Starttimeout; deshalb kein vollständig grüner Gesamtlauf behauptet |
| `go test -race -count=1 ./internal/sqlitesandbox -run '^TestCrashRecovery$'` | gezielte Wiederholung bestanden, 6,680 s |
| `python3 scripts/test-extended-export.py` | abschließend bestanden mit Race; alle neuen Format-/Dump-/HTTP-Tests, Typallowlist und echte v21-Fähigkeitsantwort gegen Schema; drei Livefälle ausdrücklich übersprungen |
| `go test -race -count=1 ./internal/core -run '^TestExtendedExportOptions$'` | bestanden, 1,246 s; nach zusätzlicher UTF-8-Namensvalidierung |
| alle `scripts/check-*-contract.py` mittels sequenziellem Python-Subprozessaufruf | zwölf Wächter bestanden, einschließlich v21 mit 17 Platzhalterfixtures; v1–v12-Pins und bestehendes Formatschema unverändert |
| `python3 scripts/check-docs.py` | 880 Dateien, 0 Fehler; Schlussprüfung nach diesem Checkpoint unten |
| `git diff --check` | bestanden; neue Dateien zusätzlich auf nachgestellte Leerzeichen geprüft |
| `docker ps --format '{{.Names}} {{.Status}}'` | Socketzugriff verweigert; keine eigenen Container gestartet |

Frühe lokale Befunde wurden korrigiert und nachgeprüft: v11-Normalisierung bleibt
separat strikt; der neue Requestdecoder erhält die TLS-Vorgabe; Alias-Duplikate nach
Katalogauflösung werden abgewiesen; die View-Typallowlist besitzt echte Klammerescapes.
Der SQLite-Materialisierungsvergleich prüft Zellwerte und `typeof`, nicht bewusst
entfernte Deklarationstypen. Das ersetzt keinen unabhängigen Review.

Uncommittete Arbeit liegt vollständig in diesem Worktree. Keine unintegrierte Arbeit
an anderer Stelle, keine eigenen Server oder Dockerressourcen. Die gestarteten
Testsessions werden bis Prozessende verfolgt; Abschlusszustand unten. Kein Gitindex
oder Ref wurde verändert. Nächster konkreter Schritt bleibt Leadreview des vollständigen
Diffs und Liveprüfung auf einem Host mit Docker-/Listenerzugriff.

Abschlusszustand: Alle eigenen Testsessions beendet, keine laufenden eigenen Jobs.
Schlussprüfung: `gofmt -l .` ohne Ausgabe, `git diff --check` bestanden und
`python3 scripts/check-docs.py` mit 880 Dateien/0 Fehlern. Keine Freigabe vorweggenommen.
