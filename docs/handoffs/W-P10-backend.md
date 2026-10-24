<!-- SPDX-License-Identifier: AGPL-3.0-only -->
# Handoff W-P10 Backend: Tabellenwartung

Stand: 24.10.2026. Backend implementiert im vorgegebenen Worktree
`.worktrees/w-p10-backend`, Branch `feat/w-p10-backend`, unveränderte Basis/HEAD
`5921a9dc1b813cc63b212fa5579f813b1595b761`. Änderungen liegen ausschließlich als
Dateien vor; kein Git-Schreibbefehl, Commit, Merge, Push oder Release ausgeführt.
Unabhängiger Review und echte Netzwerk-Engine-Abnahme bleiben offen. Keine
Behauptung vollständiger Parität oder einer Integrationsfreigabe.

## Ergebnis und Dateien

* `internal/core/maintenance.go`: strukturierte Anfragen/Antworten, optionales
  Engineinterface, statische ehrliche Fähigkeiten je Treiber und Aktion, Grenzen.
* `internal/database/maintenance.go`: Katalogauflösung, ausschließlich `dbplan`-Quoting,
  Vorschau und erneute Planprüfung je Eintrag, Enginefilter, Systemobjektsperre,
  Bestätigung, Zeitbudget, sichere Resultate und Dezimalprüfsumme als Zeichenfolge.
* `internal/database/maintenance_test.go`, `maintenance_execution_test.go`: SQLite,
  vorbereitete MySQL-/MariaDB-/PostgreSQL-Livefälle, Fehlbestätigungen, Sondernamen,
  Teilerfolg, Abbruch, Status-/Transportfehler, unterdrückte Diagnosen und Prüfsummen.
* `internal/httpapi/maintenance.go`, `maintenance_test.go`: drei authentifizierte
  Routen, striktes Decoding, Grenzen, Vorschau/Ausführung und negative HTTP-Fälle.
* `internal/httpapi/features.go`: genau eine neue Registrierung. Im Konfliktpunkt
  `features_test.go` ausschließlich Zähler 7 auf 8 geändert; Wartungsrouten prüft der
  eigene Test. Lead muss die Zählung bei gemeinsamer Integration zusammenführen.
* `internal/database/engine.go`: kleiner interner `openMode`-Wrapper für dedizierte
  Wartungsverbindungen. Bestehendes `open` behält den normalen Modus.
* `internal/sqlitesandbox/sandbox.go`, `maintenance_test.go`: `OpenMaintenance` erlaubt
  ausschließlich dort `integrity_check` mit Tabellenargument. Die normale Positivliste
  bleibt unverändert; ATTACH, VACUUM, VACUUM INTO, WAL und Dateizugriffe bleiben gesperrt.
  Diese kleine gemeinsame Änderung ist beim Review gesondert zu prüfen.
* `contracts/api-v0.11.0.json`, [Verhalten](../../contracts/MAINTENANCE-API.md),
  `contracts/fixtures/maintenance/` (32 JSON-Dateien plus README),
  `scripts/check-maintenance-contract.py`, `scripts/test-maintenance.py`,
  `.github/workflows/maintenance.yml`: Vertrag, Wächter, sequenzielle `dbfix`-Live-Suite
  und vorbereiteter Linux-amd64-CI-Job. Keine neuen Abhängigkeiten.

**API 0.11 ist vorläufig**; der Lead vergibt die endgültige Nummer. Additiv zum hier
vorhandenen v7, v1–v7 byteidentisch. Parallel entstehende v8–v10 sind absichtlich nicht
kopiert. Schema-SHA-256:
`a0c026dfaa72638c2980b076b59b9073715e83b7ab874fa697cc0432ae7fecac`.
Kein Web-Lock-Pin, keine Änderung am Vertragsindex, Webbuild oder zentralen Statusdateien.
Alle eingecheckten Wartungsfixtures sind ausdrücklich **handgeschriebene synthetische
Platzhalter**, einschließlich der nicht real berechneten Beispielhashes. Echte
versionierte Enginefixtures kommen nach dem separaten Live-Gate; temporäre echte
SQLiteantworten wurden bereits validiert.

## Verhalten und Abnahmezuordnung

PMA-MAINT-001 bis 006: CHECK, ANALYZE, OPTIMIZE, REPAIR, CHECKSUM und Stapelverarbeitung.
Jeder Eintrag erhält eine eigene Vorschau und Bestätigung; eine Ablehnung verhindert
nicht die folgenden Einträge. Keine globale Transaktion oder pauschale Rollbackzusage.
Die Ergebnisliste bleibt vollständig und geordnet, `partial` nennt gemischte Erfolge.

* SQLite: Tabellen-Integritätsprüfung und ANALYZE. VACUUM bleibt gemäß bestehender
  ADR-0004 gesperrt; keine Erweiterung der Dateisandbox. Das ist eine sichtbare
  Nichtunterstützung, kein behaupteter VACUUM-Nachweis.
* MySQL/MariaDB: CHECK/ANALYZE/OPTIMIZE/CHECKSUM konservativ auf InnoDB, MyISAM, Aria;
  REPAIR nur MyISAM, Aria, ARCHIVE, CSV. Nicht installierte/andere Engines abgewiesen.
  Engine-Statuszeilen werden ausgewertet, rohe Texte nie ausgegeben.
* PostgreSQL ab 17: ANALYZE, VACUUM mit FULL/ANALYZE, REINDEX TABLE. Vorab Eigentümer/
  MAINTAIN prüfen, damit fehlendes Recht nicht als stiller Erfolg erscheint. Keine
  partitionierten Tabellen, Partitionen, Vererbung oder Fremdtabellen; verhindert
  ungenannte Kindwirkungen aus dem W-P07-Review.

Alle Aktionen verlangen Planhash und Aktionsbestätigung; OPTIMIZE, REPAIR, REINDEX
und VACUUM FULL zusätzlich den getippten Katalognamen. Nach Dispatch konservativer
`outcome_unknown`, soweit kein ausdrücklich sicherer Rechte-/Objektfehler vorliegt.
Keine freien SQL-Fragmente, keine Systemtabellen als Ziel, keine Rohfehler, Hostpfade
oder Geheimnisse in Antworten. Namensgebundene Planung bleibt zustandslos und besitzt
ein Austauschfenster; Clientpflicht zu neuem Plan/Bestätigung und keinem Retry steht
im Vertrag. Warnungen enthalten Laufzeit-, Sperr-, Speicher- und Abbruchrisiken.

Budget: Standard 10 s, maximal 60 s für den gesamten Stapel; 1 bis 50 Einträge.
PostgreSQL zusätzlich serverseitiges Statement-/Locklimit, MySQL Metadatensperrlimit;
MySQL-Wartung kann nach Kontextabbruch serverseitig weiterlaufen. Keine allgemeine
serverseitige Abbruchgarantie für diese MySQL-Befehle behauptet.

## Tatsächlich ausgeführte Prüfungen

Go-Umgebung für alle nachfolgenden erfolgreichen Prüfungen:
`GOCACHE="$PWD/.build/go-cache" GOTMPDIR="$PWD/.build/tmp" TMPDIR="$PWD/.build/tmp"`.
Das vermeidet Schreibzugriffe außerhalb des Worktrees. Ein erster Aufruf ohne diese
Umgebung scheiterte bereits beim Zugriff auf den gesperrten Standard-Go-Cache.

| Befehl | Tatsächlicher Ausgang |
| --- | --- |
| `gofmt -l .` | Bestanden, nach Ende aller Gojobs keine Ausgabe. Zwischenzeitliche Meldungen betrafen ausschließlich generierte `_testmain.go` in ignorierten Buildartefakten. |
| `go vet ./...` | Bestanden, keine Ausgabe, nach Umstellung auf dedizierte Wartungsverbindung. |
| `go test -race -count=1 ./...` | Ausgeführt, nicht vollständig grün: bestehender `TestMySQLPerConnectionLoggerSuppressesServerInput` kann in der Sandbox keinen TCP-Listener binden (`operation not permitted`). Im letzten ungekürzten Lauf zusätzlich `TestCrashRecovery: child timeout`; separater Nachlauf unten. |
| `go test -race -count=1 ./... -skip '^TestMySQLPerConnectionLoggerSuppressesServerInput$'` | Alle Pakete bestanden, einschließlich `TestCrashRecovery` und sämtlicher bestehenden SQLite-Sandboxtests. Livefälle mangels Fixtureumgebung übersprungen. |
| `go test -race -count=1 ./internal/database ./internal/httpapi ./internal/sqlitesandbox -run Maintenance` | Bestanden; echte SQLitewartung, HTTP, Sandbox, Ergebnis-/Abbruchsimulationen. Netzwerk-Livefälle übersprungen. |
| `go test -race -count=1 ./internal/sqlitesandbox -run '^TestCrashRecovery$' -v` | Bestanden im isolierten Nachlauf (Test 3,54 s); der Timeout des ungekürzten Gesamtlaufs ist damit nicht reproduziert. |
| Sechs bestehende `check-{table,mutation,insert,structure,create,overview}-contract.py` | Bestanden; eingefrorene Altbytes unverändert. |
| `python3 scripts/check-maintenance-contract.py` | Bestanden; additive Routen/Schemas, lokale Referenzen, strikte neue Objekte und 32 positive/negative Platzhalter. |
| Schemawächter auf `.build/maintenance-dumps/*.json` | 10 echte SQLiteantworten gegen den neuen Vertrag validiert; nur lokale ignorierte Prüfartefakte. |
| Python-AST-Prüfung beider neuen Skripte | Bestanden. |
| `docker info --format '{{.ServerVersion}}'` | Socketzugriff verweigert; keine Container gestartet. |
| `python3 scripts/check-docs.py`, `git diff --check` | Bestanden; Dokumentationsprüfung 522 Dateien, 0 Fehler; Diffprüfung ohne Ausgabe. |

Während der Implementierung deckte die vollständige Suite eine zunächst zu breite
`integrity_check`-Freigabe auf. Sie wurde durch den dedizierten Wartungsmodus behoben;
die bestehenden Negativtests wurden nicht abgeschwächt. Die erste SQLite-Abbruchprobe
führte außerdem zur ausdrücklichen Kontextprüfung vor dem Öffnen. Keine offenen
bekannten Produktfehler aus diesen lokalen Befunden.

## Ausdrücklich nicht ausgeführt und nächste Schritte

Nicht ausgeführt: `python3 scripts/test-maintenance.py`, echte Wartung auf MySQL 8.4,
MariaDB 12.3 und PostgreSQL 18.6, einschließlich echter Rechte-, REPAIR-/OPTIMIZE-,
VACUUM-FULL-/REINDEX- und PostgreSQL-Sperrentests. Ursache: Docker-Socket in dieser
Sandbox unzugänglich; kein Eskalationsversuch. Tests sind geschrieben und verwenden
`scripts/dbfix.py`, ein eigenes DB-System nach dem anderen, eigene kurzlebige Ressourcen.
Echte Enginefixturedateien sind noch nicht hergestellt. Der neue CI-Workflow ist nur
vorbereitet, nicht ausgeführt. Keine Compose-/Swarm-/Web-/Browser-/Deployprüfung.
Der lokale TCP-Loggersicherheitstest muss außerhalb dieser Sandbox nachgeholt werden.

Unabhängiger Review/Vertragsfreeze und anschließende Leadintegration sind nicht Teil
einer bereits erfolgten Freigabe. Nächster konkreter Schritt: Lead prüft die Dateien,
vergibt die API-Nummer endgültig und lässt den unabhängigen Review sowie die vorbereitete
Vier-System-Suite in einer Docker-fähigen Umgebung ausführen. Erst danach echte
Fixtures sichern und gegebenenfalls Webvertrag/Pin separat integrieren.

Alle eigenen Prüfjobs beendet; keine laufenden eigenen Jobs. Keine Container,
Server oder fremden Ressourcen angelegt. Kein Wissen für die Fortsetzung nur im Chat;
die ignorierten `.build`-Caches/Logs sind entbehrliche lokale Prüfartefakte.
