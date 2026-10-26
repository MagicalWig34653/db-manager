# W-P29 Backend: Partitionen und Tabellenpräfix-Stapel

Stand: 25.10.2026. Implementiert im zugewiesenen Worktree `.worktrees/w-p29-backend`,
Branch `feat/w-p29-backend`, Basis/HEAD unverändert
`c331c642fa4957747845458b079cffc32f6b1ff6`. Ausschließlich Dateien geschrieben;
keine Git-Schreibbefehle, kein Commit, Merge oder Push. Lead committet und integriert
nach unabhängigem Review. Zentrale Status-/Planungsdateien und Vertragsindex bleiben
beim Lead. Kein Webpin, Assetbuild oder Deployment.

## Ergebnis und Abnahmeumfang

Neue [API 0.21](../../contracts/PARTITIONS-API.md) unter `/api/v21/`, additiv auf
v12; Nummer durch Auftrag endgültig festgelegt. V1 bis v12 unverändert gepinnt,
v13 bis v20 keine Voraussetzung. Registrierung genau ein neuer Featureeintrag,
Featurezählung 13. Keine neuen Abhängigkeiten oder Änderungen an SQLite-Sandbox,
Treibern oder bestehender Objektoperationslogik. Eigener Code AGPL-3.0-only.

Partitionen: Katalogliste mit Methode, Ausdruck, Grenzen und Zeilenschätzungen;
MySQL/MariaDB definieren/neu partitionieren (RANGE/LIST/HASH/KEY), ADD/DROP,
REORGANIZE zum Teilen/Zusammenführen, COALESCE, REMOVE PARTITIONING,
ANALYZE/CHECK/OPTIMIZE/REBUILD/REPAIR nach Enginefähigkeit und TRUNCATE.
PostgreSQL deklarativ CREATE PARTITION OF, ATTACH/DETACH, DROP/TRUNCATE/ANALYZE
an ausgewählten Kindern. SQLite meldet ehrlich keine Partitionsunterstützung.

Präfixstapel: wörtliche Präfixsuche, DROP/TRUNCATE, Ersetzen/Entfernen/Hinzufügen
und Kopie mit Struktur und Daten; Zielnamen aus Präfix und Katalognamen abgeleitet.
Bis 100 Tabellen, vollständige W-P07-Pläne vor der ersten Änderung, ein Gesamt-Hash.
Die vorhandenen `buildOp`-Funktionen liefern SQL, Kindobjekte, Abhängigkeiten,
Zielprüfung und Warnungen. Keine duplizierten Kopier-/Umbenennungs-SQL-Generatoren.

Gemeinsam: Vorschau mit exakten Anweisungen, Warnungen (insbesondere Datenverlust
bei DROP/TRUNCATE und Sperren bei REORGANIZE), `dbplan.Check`, getippter
Katalogname beziehungsweise gemeinsames Katalogpräfix bei zerstörenden Aktionen.
Vollständige Neuplanung auf gepinnter Verbindung vor der Ausführung. Ergebnisse
`completed`, `rejected`, `outcome_unknown`; kein Retry. PostgreSQL/SQLite in einer
Transaktion, MySQL/MariaDB nach Teilwirkung konservativ unbekannt.
MySQL-Wartungsfehler in Ergebniszeilen werden ebenfalls erkannt; keine rohen
Treiberfehlermeldungen, SQLtexte oder Pfade in Fehlerantworten.

## Dateien / Besitz

| Dateien | Inhalt |
| --- | --- |
| `internal/core/partitions.go` | Typen, Engineinterface, Fähigkeiten, Limits |
| `internal/database/partitions.go`, `prefix.go` | Katalogauflösung, strukturierte Partitions-SQL-Erzeugung, W-P07-Stapelplanung, Bestätigung und Ausführung |
| `internal/database/partitions_test.go`, `partitions_live_test.go` | SQLite-/Negativ-/Outcomefälle und vorbereitete Netzwerk-Livetests |
| `internal/httpapi/partitions.go`, `partitions_test.go` | authentifizierte Routen, HTTP-Validierung und echte Antwortdumps |
| `internal/httpapi/features.go`, `features_test.go` | eine Registrierungszeile, Zählung 12 → 13 |
| `contracts/api-v0.21.0.json`, `PARTITIONS-API.md`, `fixtures/partitions/*` | additiver Vertrag, Verhalten, zwölf gekennzeichnete Platzhalter und Herkunft |
| `scripts/check-partitions-contract.py`, `test-partitions.py` | Byte-/Additivitäts-/Schemawächter und sequenzieller dbfix-Livetest mit SQLite-only-Modus |
| `.github/workflows/partitions.yml` | zukünftige CI: Go-Race/Vet, Wächter, vollständige Fixture-Suite |
| `docs/decisions/ADR-W-P29-partitionen.md` | begrenzte strukturierte Definitionen und Wiederverwendung der Objektplanung |
| `docs/handoffs/W-P29-backend.md` | Prüfstand und Fortsetzung |

Vertrag SHA-256: `99e32721fd07b9610b6fd5aa6db7c9de7f8e93c8e9ce7d9b4163d0d489a5cb52`.
Die eingecheckten JSON-Fixtures sind ausdrücklich Platzhalter. Echte lokale
Antwortdumps entstehen nur im temporären Prüfverzeichnis und wurden nicht als
Netzwerkbelege übernommen.

## Prüfungen am 25.10.2026

Go-Befehle verwenden `GOPROXY=off GOSUMDB=off` und einen beschreibbaren temporären
`GOCACHE`; vorhandene Abhängigkeiten genügen ohne Netzwerk.

| Befehl | Tatsächliches Ergebnis |
| --- | --- |
| `gofmt -l .` | bestanden, keine Ausgabe |
| `go vet ./...` | bestanden, keine Ausgabe |
| `go test -race -count=1 ./...` | ausgeführt; ausschließlich bestehender `TestMySQLPerConnectionLoggerSuppressesServerInput` scheitert: TCP-Binding in der Sandbox verboten. Übrige Pakete grün |
| `go test -race -count=1 ./... -skip '^TestMySQLPerConnectionLoggerSuppressesServerInput$'` | zweimal ausgeführt: Datenbank-/HTTP- und übrige Pakete grün, bestehender `TestCrashRecovery` erreicht im Gesamtbetrieb sein Fünf-Sekunden-Kindstartlimit; Netzwerkfälle ohne Fixturevariablen übersprungen |
| `go test -race -count=1 ./internal/sqlitesandbox -run '^TestCrashRecovery$'` | anschließend allein bestanden (2,077 Sekunden); keine Änderung am Bestandstest |
| `python3 -B scripts/test-partitions.py --sqlite-only --keep <temporäres Verzeichnis>` | Abschlusslauf bestanden: sieben W-P29-Tests mit Race, sechs echte SQLite-/HTTP-/Outcomefehler-Antworten gegen v21 validiert |
| alle `scripts/check-*-contract.py` | zwölf Wächter bestanden, einschließlich v21, Bestandsbytes und Additivität |
| `python3 -m py_compile scripts/test-partitions.py scripts/check-partitions-contract.py` | bestanden |
| `python3 -B scripts/check-docs.py` | bestanden: 875 Dateien, null Fehler |
| `git diff --check` | bestanden, ausschließlich lesend |
| `docker info --format '{{.ServerVersion}}'` | Zugriff auf Docker-Socket verweigert, keine Container gestartet |
| `git branch --show-current`, `git rev-parse HEAD`, `git status --short` | zugewiesener Branch/Basis bestätigt; vorher sauber, ausschließlich Paketdateien geändert |

Lokale Nachweise: SQLite-Nichtunterstützung von Partitionierung; Literalquoting
von Grenzen mit SQL-Text; doppelte und fremde Grenzfelder; wörtliche Präfixe mit
`%`/`_`; Kopierdaten, Vorschau sämtlicher Aktionen; falscher Hash/Name;
Metadatendrift erweitert den Stapel und verwirft alten Hash; 101 Tabellen werden
abgewiesen; Fremdschlüsselfehler rollt vorherige Stapeländerung zurück;
HTTP-Authentifizierung, null/fremde Felder und sichere Fehlercodes.
Ergänzt: ausgeführte Präfix-Ersetzen/Hinzufügen/Entfernen/Leeren-Folge mit direkter
Katalog-/Zeilenprüfung und konservative Teilfehler-/Verbindungsverlust-Injektion.

## Nicht ausgeführt / Grenzen

- Echte MySQL-8.4-/MariaDB-12.3-/PostgreSQL-18.6-Läufe wegen gesperrtem Docker-Socket.
  Das Skript verwendet `scripts/dbfix.py`, ein System nach dem anderen, eigene
  Wegwerfressourcen und nachfolgenden Leerkontrollnachweis. Geschriebene Fälle:
  Partitionierung, Grenzen, isolierter Datenverlust, Aufteilen/Zusammenführen,
  HASH/KEY mit ADD/COALESCE, LIST, Wartung, InnoDB-REPAIR-Ablehnung,
  CREATE PARTITION OF, ATTACH/DETACH, fehlende Rechte, falscher Name/Hash,
  SQLtext als unbekannter Katalogname, Sondernamen, unbeteiligte Nachbartabelle
  und Präfixaktionen. Diese Fälle sind vorbereitet und kompiliert, kein Livebeleg.
- Unabhängiges Review, CI, Compose/Swarm, Web-/Browserintegration und reale
  Netzwerkunterbrechungen/Sperrkonflikte nicht ausgeführt. Ein TCP-basierter
  Bestandsregressionstest bleibt durch Sandbox verhindert. Im Gesamtbetrieb
  erreichte außerdem der unveränderte Crash-Recovery-Test zweimal sein
  Kindstartzeitlimit; allein und im ersten ungefilterten Lauf war er grün. Kein
  vollständig grüner Gesamtracelauf behauptet.
- Definitionen sind auf eine Katalogspalte und skalare Datenwerte begrenzt.
  Keine freien SQL-Ausdrücke, mehrspaltigen Definitionen, MySQL-Subpartitionen,
  Defaultpartitionen oder automatische Migration bestehender PostgreSQL-Tabellen.
  Vorhandene Ausdrücke werden angezeigt. Server/Engine entscheiden über Typen,
  Schlüssel, Grenzüberlappungen und weitere Rechtebedingungen. REPAIR für eine
  tatsächlich unterstützende MariaDB-Engine ist noch nicht live belegt.
- `observation=null`: Ausführung bestätigt Serverabschluss, keinen nachgelesenen
  Nachzustand. Wartungsfehlerzeilen führen konservativ zu unbekanntem Ausgang;
  rohe Serversätze werden nicht veröffentlicht. Plan-Hash ist kein Replay-/ABA-
  Schutz, Metadatenabfrage und DDL sind kein atomarer Schemasnapshot. Trigger und
  externe Effekte bleiben entsprechend dem W-P07-Vertrag begrenzt.
- Keine vollständige PMA-Parität oder Produktfreigabe. ADR ist Paketvorschlag;
  zentrale Doku-/Vertragsindexpflege bleibt beim Lead.

## Fortsetzung

Nach Abschluss der protokollierten lokalen Läufe bleiben keine eigenen Jobs
oder Container aktiv. Arbeit ist vollständig uncommittet im Paketworktree.
Nächster Schritt des Leads: Diff prüfen und committen, unabhängigen Review
vergeben, `python3 scripts/test-partitions.py --keep <Prüfverzeichnis>` auf einem
Docker-/TCP-fähigen Host ausführen, Befunde beheben lassen und erst nach Abnahme
integrieren. Keine Veröffentlichung oder Webpinnung aus diesem Handoff ableiten.
