# Handoff W-P13 Backend – API 0.19

Stand: 25.10.2026. Branch `feat/w-p13-backend`, Basis `main` `c331c64`.
Implementierungskandidat im zugewiesenen Worktree, noch kein unabhängiger Review,
Commit, Merge, Vertragsfreeze oder Webintegration. Keine Git-Schreibbefehle.
Zentrale STATUS-/Plan-/Vertragsindexdateien bleiben beim Lead.

## Ergebnis und Dateibesitz

- `internal/core/rows_bulk.go`, `internal/database/rows_bulk*.go`,
  `internal/httpapi/rows_bulk*.go`: gemeinsame Auswahltransaktion für Update markierter
  Spalten, Delete und Kopieren/Duplizieren; Plan mit Schlüsseln, Anzahl, Warnungen,
  Zeilenstabilitätsmerkmalen und Hash über Ziel/Metadaten/Werte. Vollständige Snapshots,
  Konflikte vor Schreibbeginn, gesicherte v3/v4-Eignung, maximal 100 Zeilen.
- Eng begrenzte Extraktion der vorhandenen Transaktionsblöcke in `mutation.go` und
  `insert.go`: dieselben Schutzprüfungen werden ohne Zwischencommit wiederverwendet.
  Einzelzeilenrouten behalten Validierung und Commitverhalten. `features.go` erhält
  genau einen Eintrag, `features_test.go` zählt 13.
- Begrenzte Multipart-Dateiuploads in Binärspalten mit Einzelzeilenbestätigung und
  Snapshotkonflikten; sichere gestreamte Downloads, Attachment/no-store/nosniff.
  Sichere PNG-Neukodierung aus PNG/JPEG/GIF für Bildvorschau. Kein Inline-Rendern.
- Fremdschlüssel-Lookup aus Metadaten, gebundene Suche, Werte und Anzeigespalte,
  stabile PK-Ordnung, Paging und Rechte der Verbindung. Einfache eindeutige FK-Ziele;
  unbekannte, implizite und zusammengesetzte Ziele werden erklärt gesperrt.
- `contracts/api-v0.19.0.json` additiv auf v12, `contracts/ROWS-BULK-API.md`,
  `contracts/fixtures/rows-bulk/`, eigener Vertragswächter und dbfix-Livetest,
  `.github/workflows/rows-bulk.yml`. Alle alten Vertragsbytes unverändert;
  v13–v18 sind optional. Keine neuen Module oder Abhängigkeiten.
- [ADR-Vorschlag](../decisions/ADR-W-P13-auswahltransaktion.md) hält die
  gemeinsame Transaktion, Datenplanhashes und Datei-Speichergrenzen fest.

## Prüfstand

Alle folgenden Prüfungen wurden am 25.10.2026 lokal tatsächlich ausgeführt.
Go verwendete einen eigenen Buildcache im Worktree und GOPROXY=off, weil die Sandbox
den normalen Go-Cache sperrt. Der eigene Cache wurde vor Übergabe entfernt.

| Prüfung | Ergebnis |
| --- | --- |
| gofmt auf allen geänderten Go-Dateien, abschließend gofmt -l | formatiert; abschließend keine Ausgabe |
| go vet ./... | bestanden, auch nach letzten Produktänderungen |
| go test -race -count=1 ./... | ausgeführt; bestehender TestMySQLPerConnectionLoggerSuppressesServerInput scheitert an einem sandboxbedingt gesperrten lokalen TCP-Listener; zusätzlich initialer Timeout in TestCrashRecovery unter Last |
| go test -race -count=1 ./... -skip '^TestMySQLPerConnectionLoggerSuppressesServerInput$' | alle Pakete bestanden, einschließlich TestCrashRecovery; nur der gesperrte Listener-Test ausgenommen, DB-Livetests ohne Fixturevariablen übersprungen |
| python3 -B scripts/test-rows-bulk.py --sqlite-only --keep contracts/fixtures/rows-bulk | nach letzten Produkt-/Teständerungen bestanden; echte SQLite-/HTTP-Racetests, fünf reale Antworten gegen Schema validiert |
| alle zwölf scripts/check-*contract.py | bestanden; v1–v12 eingefroren, v19 additiv, Referenzen und Fixtures geprüft |
| python3 -B scripts/check-rows-bulk-contract.py | 19 Fixtures bestanden: 14 klar gekennzeichnete synthetische/negative Beispiele und fünf echte SQLite-/HTTP-Antworten |
| python3 -B scripts/check-docs.py | bestanden nach Cacheentfernung: 884 Dateien, 0 Fehler |
| git diff --check | bestanden, ausschließlich lesender Git-Befehl |
| python3 -B scripts/test-rows-bulk.py | SQLite-/HTTP-Teil bestanden; MySQL-Fixturestart am gesperrten Docker-Socket gescheitert, anschließend auch dbfix-Cleanupzugriff verweigert |
| docker info | Zugriff auf lokalen Docker-Socket verweigert; keine DB-Containerprobe möglich |

Belegt: atomare Ablehnung beim zweiten Constraint-/Snapshotkonflikt, Hash- und
Zeilenstabilitätsabweichung, bestätigte Updates/Deletes/Kopien, neue explizite und
Auto-Schlüssel, doppelte Auswahl, 101-Zeilen-Sperre, PK-Zielsperre, No-op ohne Trigger,
spätere Triggeränderung an einer früheren Ergebniszeile mit vollständigem Rollback,
Composite-PK mit SQLite-Kollationsabweichung, No-PK-Ablehnung, Kontextabbruch,
Dateibytes/Leerblob/512-KiB-Grenze/Übergröße/Textspaltensperre/Snapshotkonflikt,
sichere PNG-Neukodierung und SVG-Sperre, FK-Paging und literal behandelte Suchzeichen.
Ein echter verzögerter SQLite-FK-Commitfehler belegt outcome_unknown für alle Zeilen;
der HTTP-Test bestätigt 503 ohne als erfolgreich ausgegebene Nachherzeile.

Die gemeinsame Live-Fallserie für MySQL/MariaDB/PostgreSQL ist geschrieben und über
scripts/dbfix.py angebunden: Update-/Delete-/Kopierablauf, vollständiger Rollback,
explizite und Auto-Schlüssel, präzise Binärwerte, Dateioperationen, FK-Suche/Paging,
Nichtunterstützung ohne PK sowie Abbruch. Tatsächliche Läufe dieser drei TLS-Systeme
bleiben offen. Kein Docker-Daemonzugriff war möglich; kein erfolgreicher Containerstart
oder erfolgreicher Cleanup wird behauptet. Keine Browser-, Deployment- oder native
Linux-CI-Prüfung in diesem Backend-Worktree ausgeführt.

Vertragssumme SHA-256:
511f245df89464183ce0c49d92a1c922d25f0d0f524684c61894b8ca5dda35a3.

## Grenzen und Abnahme

Die explizite Paketvorgabe verlangt Binär-/BLOB-Dateiwerte; TEXT-Uploads und ENUM/SET-
Editorlisten aus dem allgemeineren PWA-Plan sind nicht Bestandteil dieses Vertrags.
Die Weboberfläche wird in diesem Backendpaket nicht geändert. Teilerfolg ist durch
atomare Ablehnung ersetzt; jede Zeile trägt den tatsächlichen Transaktionsausgang.
Kein Erfolg wird für lokal zurückgerollte Zeilen gemeldet.

Dateiwerte maximal 512 KiB, gesamte Anfrage weiterhin 1 MiB. Alte volle Snapshots und
neue Bytes können gemeinsam früher an dieses Limit gelangen. DB-Treiber binden volle
Werte und lesen begrenzte Snapshots; HTTP-Streaming ist keine unbeschränkte DB-BLOB-
Streaminggarantie. Keine Serverdateien. Lookup ist kein gemeinsamer Schemasnapshot
über Katalog- und Zeilenabfrage. ABA, externe Triggerwirkungen, Sequenzlücken und
unbekannte Commits bleiben wie in B2b/B2c begrenzt. Keine automatischen Retries.

Offen vor Integration: unabhängiges Review, echte TLS-Läufe gegen MySQL 8.4,
MariaDB 12.3 und PostgreSQL 18.6, native Linux-CI. Keine Selbstfreigabe.
Lead committet und integriert erst nach Abnahme. Keine eigenen Prüfjobs oder temporären Buildcache-Dateien verbleiben.
Nächster konkreter Schritt: unabhängigen Review des vollständigen uncommitteten Diffs
und echte TLS-Läufe in einer Umgebung mit Dockerzugriff ausführen; anschließend
Commit und Integration durch den Lead.
