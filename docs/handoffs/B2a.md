# B2a – Tabellenmetadaten und strukturiertes Lesen

Stand: 20.10.2026. Implementierer: Backend & Platform. Branch `feat/table-browse`,
Worktree `.worktrees/table-browse`, Basis `c9cac01`. Noch nicht unabhängig abgenommen,
integriert oder für Clientbindung eingefroren. Kein Mutationsendpunkt.

## Ergebnis und Vertrag

Neue authentifizierte Routen: `GET /api/v2/capabilities`,
`POST /api/v2/tables/describe`, `POST /api/v2/tables/browse`.
[Vertragskandidat 0.2.0](../../contracts/api-v0.2.0.json), SHA256
`c15a848f01bb779429fd68a2b6eb6bba1575d3e07a4c68e6796cf7840a661838`.
[Vertragsbeschreibung](../../contracts/TABLE-API.md) erklärt Felder und Grenzen.
API0.1.0, alle alten Schemadefinitionen und Routen sowie die v1-Capabilities bleiben
unverändert; dies prüft `scripts/check-table-contract.py`. Der unveränderte
0.1.0-Schemahash ist
`4b598dc69a1904685e67a717997412654440ac10f47c17f7a025ccd114e294be`.
Frontendowner hat die additive v2-Trennung bestätigt. Keine Clientbindung vor QA-Freeze.

Describe liefert kanonische Tabellen-/Schemanamen, Spalten, servernative Typen und
Defaults, Nullable, PK-Reihenfolge, Generated, Tabellenart und bekannte
Transaktionseigenschaft. MySQL InnoDB wird von MyISAM/MEMORY/CSV/ARCHIVE getrennt;
unbekannte Engines bleiben `unknown`. Alle `read_only`-Felder sind bewusst `true`:
Das bezeichnet die aktuell angebotene strukturierte Produktfähigkeit, keine Aussage
über DB-Privilegien und keine Schreibsperre für den bestehenden freien SQL-Endpunkt.
Views, Foreign-/virtuelle Tabellen und fehlende PK werden nicht als bearbeitbar ausgegeben.

Browse prüft Identifier gegen frisch gelesene Metadaten, quotet sie treibergerecht
und bindet Filter-, LIMIT- und OFFSET-Werte als Parameter. Unterstützt werden AND-
verknüpfte `eq`, `is_null`, `is_not_null`, höchstens 16 Filter/Sortierspalten,
Seiten bis 1000 Zeilen, Offset bis 1000000 und Timeout bis 60000ms. Standard:
100 Zeilen/10000ms. Es gibt kein implizites COUNT. Ein zusätzlicher Zeilenabruf
ermittelt `has_more`, ohne die Extrazeile ins Ausgabebudget einzurechnen.
Nichtnullable PK-Spalten ergänzen jede Sortierung zur eindeutigen Ordnung;
fehlende/nullable PK melden `stable_order=false`. SQLite INTEGER-Rowidalias wird
von nullable INTEGER PRIMARY KEY DESC bzw. Text-PK getrennt.
Runtime-Speicherklassen, NULL/Leerstring/Leerblob und Präzisionszellen bleiben erhalten.

## Besitz und Änderungen

- Neue Core-Verträge `internal/core/table.go`, additive HTTP-Dateien und enge
  Routenregistrierung; vorhandenes Engine-Interface und API0.1.0 unverändert.
- `internal/database/table_metadata.go`, `table_browse.go`, Regressionen;
  SQLite-Nativeabfrage um geprüfte Parameterbindung erweitert.
- Eigene Tabellen-CI, `scripts/test-table-browse.py`, Vertragsprüfung und Dokumentation.
- Keine Änderungen an main, Modulen, Docker/Compose/Swarm, Webwrapper oder Vaultschema.
  Keine neuen Abhängigkeiten und keine Migration bestehender Produktdaten.

## Tatsächliche Prüfungen

Lokal am 20.10.2026 erfolgreich, einschließlich der letzten Änderungen:

- `go test -race -count=1 ./...`
- `go vet ./...`
- `python3 scripts/check-table-contract.py`
- `python3 scripts/test-table-browse.py`: echte TLS-Verbindungen gegen eigene
  MySQL8.4-, MariaDB12.3- und PostgreSQL18.6-Wegwerfcontainer. Sonderzeichen in
  Tabellen-/Spaltennamen, zusammengesetzte PK, Defaults/Generated, NULL/Leerblob,
  Binary/Decimal, Paging/Sortierung, gebundene Injektionswerte, fehlende PK,
  Views und nichttransaktionale MySQL-Engine geprüft. Echte langsame Views
  brechen mit `context.DeadlineExceeded` innerhalb zwei Sekunden ab;
  übergroße Netzwerkzellen werden als `result_too_large` abgewiesen.
- SQLite: zusätzlich nullable PK-/Rowid-Sonderfälle, rekursive langsame View,
  übergroße Zellen und fünf große Blobzeilen plus Lookahead, sodass die tatsächliche
  JSON-Seitengröße und nicht eine reine Rohbytesumme maßgeblich bleibt.
- HTTP: Authentifizierung, unveränderte v1-Capabilities, v2-Features, tatsächliches
  Describe/Browse und Ablehnung unbekannter Felder, explizitem NULL und ungültigen Limits.

Neue Linuxamd64-CI muss nach Push dieses Commits laufen; vor deren tatsächlichem
Ergebnis wird kein Erfolg behauptet. Unabhängige Vertrags-/Security-/Funktionsprüfung
bleibt erforderlich. Vorhandene B1-Durabilitäts-/Containerproben wurden für diese
begrenzte Tabellenänderung nicht erneut als Gesamtdeployment ausgeführt.

## Grenzen, Ressourcen und nächster Schritt

Metadatenlesen und Seitenabruf sind keine gemeinsame Snapshottransaktion.
Parallele DDL kann zum sicheren Abfragefehler führen; parallele DML kann zwischen
Offsetseiten Zeilen verschieben. Eindeutige Ordnung ist keine Snapshotgarantie.
DB-eigene Vergleiche/Collations und Typkonvertierungen bestimmen Filtersemantik.
Nicht jede getaggte Wertart ist mit jedem Spaltentyp vergleichbar; DB-Fehler bleiben
redigiert. SELECT auf Views kann DB-seitige Funktionswirkungen haben; keine
Nebenwirkungsfreiheit oder vollständige DoS-Abwehr zugesichert. Ein Abbruch ist keine
Rollbackzusage. Die bestehenden B1-Limits, global vier aktiven DB-Anfragen, Host-/TLS-
Policy und ADR0004-Betriebsgrenzen gelten unverändert.

Die eigenen Testcontainer verwenden ausschließlich `dbmanager-b2a-<Zufall>` und
Label `dbmanager.task=B2a`, dynamische Loopbackports und temporäre öffentliche
Testzertifikate. Alle eigenen Testcontainer/-volumes wurden durch das Skript entfernt;
keine fremden Ressourcen angehalten. Keine lokalen Prüfjobs mehr aktiv.
CI-IDs und Commit werden nach Push als Übergabecheckpoint ergänzt.
Nächster Schritt: unabhängiger Review und nativer Linuxamd64-Nachweis; anschließend
Leadentscheidung über Integration und gesonderten 0.2.0-Freeze vor Clientarbeit.

## Commit- und CI-Checkpoint

Produktcommit `62f129755a06922da6108c294e324b9171c268c4` auf
`origin/feat/table-browse` gepusht. Der reine Übergabecheckpoint folgt lokal und
ändert den geprüften Produkt-/Schemaquellstand nicht. Zum Übergabezeitpunkt stehen
folgende Jobs am Produktcommit in der Queue; dies ist noch kein Erfolgsnachweis:

- [Tabellenmetadaten und Lesen, native Linuxamd64](https://github.com/MagicalWig34653/db-manager/actions/runs/39495448336)
- [Datenbankengine](https://github.com/MagicalWig34653/db-manager/actions/runs/39495448305)
- [Backend und Container](https://github.com/MagicalWig34653/db-manager/actions/runs/39495448282)
- [Dokumentation](https://github.com/MagicalWig34653/db-manager/actions/runs/39495448176)

Keine weiteren lokalen Jobs oder eigenen Dockerressourcen. Worker ist für den
unabhängigen Review idle; Jobausgänge kontrolliert die koordinierende/prüfende Rolle.

## Nacharbeit nach unabhängigem Erstreview

Die ursprünglichen vier CI-Jobs am `62f1297` sind inzwischen tatsächlich erfolgreich.
Der unabhängige Architecture-/Securityreview fand zwei mittlere Vertragsfehler;
noch kein 0.2.0-Freeze oder Integrationsnachweis.

- **B2a-01, Pflichtfeldpräsenz:** JSON ohne `FilterValue.value` wurde als Leerstring
  interpretiert. Auch fehlendes Filter-/Sortier-`column` konnte eine vorhandene leere
  Spaltenbezeichnung treffen. Die v2-HTTP-Grenze verwendet nun Präsenzpointer für
  Filterwert-Art/-Wert und Spaltenbezeichner; fehlende Felder werden mit HTTP400
  abgewiesen. Explizites JSON-NULL bleibt durch die bestehende Tokenprüfung gesperrt.
  Explizite `value:""` für Text/Blob sowie `column:""` bleiben gültig. Alle übrigen
  neuen Pflichtfelder wurden geprüft: fehlender Tabellenname, Filteroperator und
  Sortierrichtung ergeben bereits ungültige Werte und werden abgewiesen.
- **B2a-02, SQLite-PK-Kollation:** Ein PK mit Indexkollation BINARY bei Spaltenkollation
  NOCASE wurde fälschlich bereits durch das normale Spalten-ORDER-BY eindeutig geordnet.
  SQLite hängt jetzt alle PK-Spalten mit festem `COLLATE BINARY ASC` als letzten
  Tiebreak an, einschließlich schon explizit sortierter PK-Spalten. Der Ansatz wurde
  mit dem unabhängigen Reviewer abgestimmt; keine API-kontrollierte Kollation eingeführt.

Regressionsprüfungen: fehlender bzw. NULL-Filterwert für sämtliche sieben zulässigen
Nicht-NULL-Arten; fehlende Kind-/Filterspalten-/Sortierspaltenfelder; echte leere
Text-/Blobwerte und leerer Spaltenidentifier. SQLite-Test mit NOCASE-Spalte und
BINARY-PK, Werten `a`/`A`, zwei Einzelseiten bei Default-, asc- und desc-Sortierung.
`go test -race -count=1 ./...`, `go vet ./...`, Vertragsprüfung und Diffprüfung
nach diesen Fixes erfolgreich. Kein neues Deployment oder lokale Netzwerkcontainer
für die eng begrenzten HTTP-/SQLite-Fixes notwendig. Schemahash bleibt unverändert.
Fixcommit wird gepusht und benötigt eigene native CI sowie unabhängigen engen Nachreview.
Keine lokalen Prüfjobs oder eigenen Container aktiv; anschließend Worker idle.

## Unabhängige Abnahme und Vertragsfreeze

Architecture-/Securitypeer prüfte ein eigenes sauberes Archiv mit Go-Race/Vet,
Vertragsprüfung und echten TLS-Tabellenproben gegen MySQL/MariaDB/PostgreSQL.
Zwei mittlere Befunde blockierten zunächst: fehlende Pflichtfeldpräsenz bei Werten/
Spalten und unbelegte SQLite-Ordnungeindeutigkeit bei abweichender PK-Kollation.
Fix `3b5a54c` schloss beide; unabhängig neu geschriebene Repros und enge
Regressionen bestätigten fehlend/NULL gegenüber echten Leerwerten/-bezeichnern
sowie Kollation mit expliziter DESC-Sortierung über mehrere Seiten. Keine weiteren
bestätigten Befunde. Keine zusätzlichen vollständigen Deploymentproben erforderlich.

Alle vier Fix-CIs erfolgreich: [Tabellen](https://github.com/MagicalWig34653/db-manager/actions/runs/39496042145),
[Engine](https://github.com/MagicalWig34653/db-manager/actions/runs/39496042204),
[Backend/Container](https://github.com/MagicalWig34653/db-manager/actions/runs/39496041978),
[Dokumentation](https://github.com/MagicalWig34653/db-manager/actions/runs/39496042202).
Engine zunächst beim MariaDB-Imageabruf mit DockerHub-HTTP502 gescheitert;
einmaliger gezielter Jobneustart erfolgreich, kein Produktfehler daraus abgeleitet.
Lead integrierte `cb07ebb`/`19673b3`/`427444d`; integrierte Go-Race/Vet-, Vertrags-
und Dokumentationsprüfung bestanden. API0.2.0 ausdrücklich für Clientbindung
freigegeben, Herkunft `427444d7bcb8e30c9563ded2000ec3d5e19646c4`, Schema unverändert.
Keine Mutation, Snapshot- oder umfassende Paritätsfreigabe. Keine eigenen Jobs offen.
