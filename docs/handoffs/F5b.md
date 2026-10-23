# F5b – eingebettete Strukturansicht und Ergebnisexport

Stand: 23.10.2026, Produktprüfkandidat, **keine Selbstfreigabe oder Integration**.
Eigener Branch `feat/schema-web-runtime`, Worktree `.worktrees/schema-web-runtime`;
nach Leadfreigabe auf `1b02ea30447f6ff100ef00a24b7e174bb8618cd0` gebracht. Dieser
Backendstand enthält unabhängig akzeptiertes B4a/API0.6; die Webquelle konsumiert
weiterhin ausschließlich API1–5.

## Herkunft und Besitz

Exakter unabhängig angenommener öffentlicher Webpin
`8ef738d22a1164291e2c8183d80e72d1f043c0f8`, unabhängiger F5-Fixreview
`469653dd6df9264f700275f045dc2b500dc2b1fc`, drei exakte Quell-CIs erfolgreich laut
Leadfreigabe. Quelle enthält angenommenes F6 und den geschlossenen F5-Befund zur
Darstellung bekannter leerer Bezeichner. Keine freigegebenen Webproduktbytes geändert.

Besitz nur Webassets/Manifest/Lock, enge Build-/Runtime-/Deploymentprüfungen, ein
Docker-Quelllabel und zugehörige Dokumentation. Keine DB-/Core-/HTTP-/GoMod-/API-,
Site-, Apple- oder zentralen STATUS-/ARBEITSPLAN-Änderungen. Kein neues Produktmodul
oder npm-Lockfile; Original-AGPL-/Drittanbietertexte bleiben bytegleich.

## Tatsächlich bestanden

- Zwei unabhängige frische öffentliche Quellcheckouts über `scripts/build-web.py`:
  fünf Originalassets und Manifest bytegleich. Lock/Manifest/Quellkopien binden
  API1–5 einschließlich Struktur-SHA256
  `b5225339511d80fe4f52f09118fdad2060d8a92f31d44b73dd89fe9f76178c95`.
  Node26.5.0/npm11.17.0, npm-Lockhash unverändert. Exakter gebauter SourceURL,
  Originalnotices und keine Sourcemaps/privaten Pfade geprüft. Die bekannte Vite-
  Chunkwarnung bei 958 kB ist kein Buildfehler; fünf Artefakte bleiben der Output.
- Offline-Manifestprüfung und neun Python-Buildprüfungen einschließlich zusätzlicher
  fehlender/falscher Strukturpins bestanden. `go test -race -count=1 ./...` und
  `go vet ./...` vollständig bestanden; kein neuer Live-Vier-Treiberbeweis daraus.
- `python3 scripts/test-web-integration.py`: **13 echte Chromiumfälle in 19,9 s**
  bestanden. CLI und tatsächliches eingebettetes Composeprodukt, SQLite sowie
  PostgreSQL mit verify-full; zuerst positive TLS-Probe, dann falscher Hostname.
  Unveränderte Bootstrap-/TCP-Barriere reproduziert Socket-only-Readiness und lehnt
  sie ab. Kein Viteproxy oder Ersatz für Datenbankantworten.
- F5: tatsächliche Expression-/Partial-/DESC-/Include-/FK-/Check-/Coverage-Metadaten,
  explizite SQLite-CHECK-Lücke und rowid-Hilfsterm, eindeutig zitierter bekannter
  leerer Bezeichner, Viewgrenze und verspätete echte Antwort nach Tokenwechsel.
  Nur der explizite v5-HTTP404-Kompatibilitätsfall verwendet eine synthetische
  Statusantwort; reguläre Strukturwerte kommen aus den echten Datenbanken.
- F6: tatsächliche JSON-/CSV-Downloadbytes, geordnete doppelte Namen, Präzision,
  NULL/Leer/Binary, bewusste CSV-Varianten, echte Steuerzeichenabwehr vor Blob/URL,
  SQL-Trunkierung/Page-Scope/Offset/has_more und keine neue DB-Anfrage pro Export.
  Bereits gestarteter Download bleibt trotz Cleanup intakt. Keine Credentials,
  SQL-/Profilfelder im Export. JSON erhält Steuerzeichen unverändert.
- Bisherige CRUD-/409-/PK-/Generated-/Default-/Snapshot-/Doppelklickfälle und echter
  Commit vor verloren gegangener Antwort ohne Retry bestanden; separate DB-Abfrage
  bestätigt genau eine gespeicherte Zeile. SQL/Cancel/DB-Freigabe, F3-Pagingzustand,
  Source-/Originalnotice-Dialog und CSP-/Sessiongrenzen unverändert grün.
- `python3 scripts/test-deployment.py`: Compose und eigener isolierter Docker29.5.2-
  DinD-Swarm bestanden Originalassets/CSP/Auth/Health/UID10001/read-only-Root und
  echte v4-INSERT/v3-UPDATE/DELETE/409/Default/NULL/Präzision/Binary. SQLitezeile nach
  Compose-Neustart bzw. Swarm-Taskersatz byte-/typgleich. Neue echte v5-Metadaten
  eines benannten partiellen DESC-Indexes/PK/Coverage sind vor/nach Ersatz exakt
  gleich. Der erste Lauf prüfte nur PK/Coverage; nach Ergänzung des nichtleeren
  Indexfalls wurde ausschließlich die Deploymentprüfung nochmals bestanden.
- Dokumentationsprüfung 294/0, eigene Quellen/Doku/Tests diff-sauber. Originale
  generierte JS-/Lizenzbytes werden nicht zum Bereinigen fremden Leerraums verändert.

## Ressourcen und Grenzen

Host vor Beginn: 48 GiB RAM, memory_pressure 57 % verfügbar, 44 GiB Disk frei;
Docker-VM mit 7,737 GiB und vier vorhandene fremde Container unter 0,9 GiB. Lead koordinierte
beendete Backend-/Frontendreviewfixtures vor Freigabe. Nur eine eigene PG-Suite bzw.
ein eigener isolierter DinD-Lauf gleichzeitig, keine parallelen Deploymentläufe.

Eigene Browser-/CLI-/Compose-/PG-/DinD-Prozesse, Container, Volumes, Netze, Images und
kurzlebige Credentialdateien durch Skripte entfernt. Host-Swarm vorher/nachher
`inactive`; fremde Container und ursprüngliche TestFlightarchive unverändert.
Keine HA-/Multinode-/Powerloss-/Exactly-once-/öffentliche Produktreleasegarantie,
keine nativen oder MySQL-/MariaDB-Browsernachweise aus zwei geprüften Engines.
SQLite weiterhin eingeschränkt nach ADR0004.

Offen bei diesem Produktcheckpoint: frischer öffentlicher Rootclone ohne
Host-Node/npm/Geschwister für CLI-/Imagebau, exakte öffentliche Kandidaten-CIs und
unabhängiger F5b-Review. Produktcode erst nach diesen Nachweisen integrieren; keine
Selbstfreigabe. Nächster Schritt: unveränderten Kandidaten veröffentlichen, frischen
Clone prüfen und vollständige Evidenz als reinen Handoffnachtrag sichern.

## Öffentlicher Abschlusscheckpoint

Produkt-/Testkandidat `0dc9557c6a4d7ecf58aedc40987c64526aae416d` ist öffentlich auf
`feat/schema-web-runtime` eingefroren. Neue Prüfungen verändern keine Produktbytes.
Frischer öffentlicher Rootclone exakt dieses SHA ohne Geschwisterrepository:

- CLI mit `CGO_ENABLED=0 GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOENV=off` auf
  überprüft Node-/npm-freiem PATH gebaut. Go1.27.1 und vorhandener lokaler Modulcache;
  kein behaupteter cachefreier Build. Frischer Clone blieb sauber.
- Reguläres Dockerimage aus demselben frischen Clone gebaut; normale öffentliche
  gepinnte Go-Moduldownloads zulässig, kein behaupteter netzloser Imagebau. Image
  enthält originalen Root-AGPL-Text und alle vorhandenen Go-Lizenztexte, Weblock und
  Manifest mit exakten Hashes, richtige OCI-/Webquelllabels und kein Node/npm.
- Eigenes Cleanimage, öffentlicher Clone, CLI und Toolverzeichnis entfernt. Keine
  ursprünglichen TestFlightarchive berührt. Autoritative Ressourcenabfrage zeigt
  ausschließlich vier fremde Container, keine eigenen F5b-Container/Images und
  Host-Swarm `inactive`.
- Originale generierte neue JS-Datei `index-Ay2aZLYy.js` enthält auf Zeile 1 bereits
  trailing whitespace. Zwei öffentliche Builds/Manifest bestätigen Originalbytes;
  kein Handbereinigen dieses Fremd-/Buildartefakts. Eigene Quellen/Doku/Tests sind
  diff-sauber; keine neue Lizenzdateiveränderung.

Exakte öffentliche CIs am Produktcheckpoint:

| Prüfung | Run | Tatsächlicher Stand bei Nachtrag |
| --- | --- | --- |
| Dokumentation | [39910176520](https://github.com/MagicalWig34653/db-manager/actions/runs/39910176520) | success |
| Tabellenanlage | [39910176514](https://github.com/MagicalWig34653/db-manager/actions/runs/39910176514) | success |
| Insert | [39910176523](https://github.com/MagicalWig34653/db-manager/actions/runs/39910176523) | success |
| Update/Delete | [39910176612](https://github.com/MagicalWig34653/db-manager/actions/runs/39910176612) | success |
| Tabellenansicht | [39910176571](https://github.com/MagicalWig34653/db-manager/actions/runs/39910176571) | success |
| Strukturmetadaten | [39910176656](https://github.com/MagicalWig34653/db-manager/actions/runs/39910176656) | success |
| Engine | [39910176605](https://github.com/MagicalWig34653/db-manager/actions/runs/39910176605) | success |
| Backend/Compose/Swarm | [39910176639](https://github.com/MagicalWig34653/db-manager/actions/runs/39910176639) | success |
| Web/Reproduktion/Browser | [39910176583](https://github.com/MagicalWig34653/db-manager/actions/runs/39910176583) | success |

Der unabhängige F5b-Reviewer hat den eingefrorenen Code übernommen und den freien
Dockerheavytestslot nach tatsächlicher Autorenbereinigung bestätigt bekommen.
Keine eigenen lokalen Prüfjobs/Server/DBfixtures/Images oder privaten Credentialdateien
mehr aktiv. Weitere Quelländerungen nur nach konkretem Reviewbefund. Der abschließende
autoritative CI-Check bestätigt alle neun exakten Produkt-CIs erfolgreich; Lead
entscheidet nach dem unabhängigen Review über Integration.
Dieser reine Handoffnachtrag startet mit `[skip ci]` keine unveränderte neue Prüfserie.
