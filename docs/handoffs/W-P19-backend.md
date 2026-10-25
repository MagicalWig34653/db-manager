# Handoff W-P19 Backend: Views, Routinen, Trigger und Events

Stand: 24.10.2026. Implementierung als uncommittete Dateien auf
`feat/w-p19-backend`, Basis/HEAD unverändert
`b74d10e6ff50ed49dccb0484ee3f8d2f0ca5b656`, Worktree `.worktrees/w-p19-backend`.
Keine Git-Schreibbefehle, kein Commit, Push oder Merge. Lead übernimmt Sicherung,
unabhängiges Review und Integration. API **0.19 ist vorläufig**, kein Web-Lock-Pin,
keine formale Vertragsfreigabe oder vollständige Paritätsabnahme behauptet.

## Ergebnis und Dateibesitz

- `internal/core/programs.go`: eigene Typen, Grenzen, optionales Engine-Interface,
  Fähigkeiten je Treiber und ausdrücklich ausgeschlossene Aktionen.
- `internal/database/programs.go`: Validierung, Schema-/Zielauflösung,
  Plan/Bestätigung, transaktionale bzw. nicht atomare Ausführung, sichere
  Ergebnismeldungen und Antwortgrenzen.
- `internal/database/programs_catalog.go`, `programs_sqlite.go`,
  `programs_mysql.go`, `programs_postgres.go`: Listen/Detaildefinitionen,
  Parameter, Eigenschaften, Definer, SQL SECURITY, Events und Schedulerstatus.
- `internal/database/programs_create.go`: strukturierte DDL-Erzeugung pro Dialekt,
  Parameter-/Typauswahl, Triggerfunktion, Zeitplan, Definer-/Eigentümerwechsel.
- `internal/database/programs_dependencies.go`: Abhängigkeiten mit Richtung,
  Gewissheit und ehrlicher Begrenzung der Analyse.
- `internal/database/programs_test.go`, `programs_live_test.go`: SQLite-,
  Dialekt-, Negativ- und dbfix-gestützte Serverdatenbanktests.
- `internal/httpapi/programs.go`, `programs_test.go`: sechs geschützte Routen
  unter `/api/v19`, HTTP- und Integrationsgegenproben. `features.go` erhält genau
  einen Registrierungseintrag; in `features_test.go` ausschließlich Zähler 8 → 9.
- `contracts/api-v0.19.0.json`, `contracts/PROGRAMS-API.md`,
  `contracts/fixtures/programs/`: additiver Vertrag mit strikten neuen Schemas,
  24 Fixtures; reale lokale Antworten und klar gekennzeichnete synthetische
  Platzhalter einschließlich Fehler-/Negativfällen.
- `scripts/check-programs-contract.py`, `scripts/test-programs.py`,
  `.github/workflows/programs.yml`: Wächter, serielles dbfix-Harness, eigener
  CI-Entwurf. Dieses Handoff dokumentiert den Paketstand.

Vertrags-SHA-256:
`21aacd6acb5b1711e4ba2e091d0b9be350764abc7a152f067fcc66af94de98eb`.
V1 bis V8 sind byteidentisch; alle bisherigen Pfade, Schemas und
Authentifizierungsschemata sind im neuen Vertrag unverändert übernommen.
Keine neuen Abhängigkeiten oder Lizenzinventaränderungen. Eigener Code mit
SPDX AGPL-3.0-only, keine Attribution-Zeilen. Zentrale Status-/Plan-/Vertragsindex-
und Webdateien bleiben im Besitz des Leads und wurden nicht geändert.

## Verhalten und Sicherheitsgrenzen

Lesen: Listen je Schema/Art/Tabelle, vollständige Definition per Detailroute,
Parameter und Eigenschaften, Definer/Sicherheitskontext, Eventzeitplan/-status,
Scheduler ausschließlich lesend. Definitionen sind als Text exportierbar.
PostgreSQL-Views werden aus Katalogdaten abgeleitet und entsprechend markiert;
ein Definitionsexport verspricht keine Wiederherstellung sämtlicher Rechte.

Schreiben: CREATE, REPLACE, DROP von Views, Routinen, Triggern und Events gemäß
Treiberfähigkeit. Jede Aktion benötigt einen neu berechneten Plan-Hash und
Bestätigung; DROP und jedes REPLACE zusätzlich den exakt getippten Katalognamen.
Freie Körper sind ausdrücklich Nutzer-SQL gemäß E5. Statements werden vollständig
angezeigt, nicht an Semikolons gesplittet; Compound-Körper gehen als ein Statement
über die Treiber-API. MySQL verwendet MultiStatements=false, SQLite/PostgreSQL
Prepare zur Abweisung zusätzlicher Statements. DELIMITER wird nicht interpretiert.
Ein Plan ist kein Syntaxnachweis; Syntaxprüfung erfolgt beim Anwenden im Server.

SQLite/PostgreSQL: DDL-Gruppe transaktional. SQLite-View-/Triggerersetzung ist
DROP/CREATE ohne Tabellen-Neuaufbau. MySQL/MariaDB: Ersetzen von Routine, Trigger
oder Event ist eine offen angezeigte DROP/CREATE-Folge mit impliziten Commits;
ein Fehler nach DROP ergibt `outcome_unknown`, ohne automatisches Wiederherstellen.
Keine CASCADE-Aktionen. Abschluss und anschließende Katalogbeobachtung bleiben
getrennt (`observation=unavailable`). Der zustandslose Hash ist keine Einmal-Nonce;
Clients müssen nach Abschluss/Unsicherheit sperren und neu bestätigen lassen.
Das bekannte Fenster zwischen Katalogprüfung und namensbasierter Ausführung bleibt.

Namen bestehender Objekte und referenzierter Tabellen/Funktionen kommen aus dem
Katalog, werden exakt verglichen und dialektgerecht gequotet. PostgreSQL-Routinen
werden anhand der Katalogidentität unterschieden; Trigger zusätzlich anhand der
Tabelle. Neue Namen und strukturierte Typen/Optionen werden eng validiert. Explizite
Definer-/DEFINER- und Eigentümeraktionen tragen Warnungen und verwenden sichtbare
Katalogbenutzer/-rollen. Keine privilegierte Ersatzverbindung und keine rohen
Treiberfehler, Verbindungsdaten oder Hostpfade in Fehlermeldungen.

Bewusste Paketgrenzen aus dem Auftrag:

- **Kein Aufruf/Test von Routinen**, auch nicht in den Livetests; PMA-PROG-005
  bleibt offen für ein späteres Paket nach E5/E9. Keine Aufrufroute.
- **Keine Scheduleränderung**, auch nicht zum Testen. Event-Livetests legen
  ausschließlich deaktivierte Events mit zukünftigem Beginn an und kontrollieren
  unveränderten Schedulerstatus. Kein Nachweis eines tatsächlichen Eventlaufs.
- **Keine SQLite-Routinen/Events**, keine PostgreSQL-Events, kein SQLite-
  Tabellen-Neuaufbau. Fähigkeiten/Fehler machen dies explizit sichtbar.
- Keine portable Eventfehlerliste oder Rohlogs: `event_errors_state=unavailable`,
  Fähigkeit `unsupported`. Kein eigener Syntax-Linter und kein Parserversprechen.
- PostgreSQL-Struktureingaben bieten SQL/PLpgSQL, feste Typen und IN/OUT/INOUT;
  spezialisierte/variadische/Tabellen-/unbenannte Signaturen sind lesbar/löschbar,
  aber nicht vollständig strukturiert ersetzbar. Ersetzen erhält die Überladung.
- Katalogabhängigkeiten in PostgreSQL sind exakt, aber nicht vollständig;
  Stringkörper/dynamisches SQL können Verweise verbergen. SQLite/MySQL/MariaDB
  liefern textuelle Hinweise innerhalb eines Schemas, höchstens 1000 Kandidaten
  und 100 Beziehungen, mit Kennzeichnung einer Kürzung. Bei neuer PostgreSQL-DDL
  können noch keine Katalogbeziehungen der geplanten Definition existieren.

## Tatsächlich ausgeführte Prüfungen

Alle Go-Aufrufe nach dem ersten Cachefehler mit
`GOCACHE=/tmp/dbmanager-w-p19-go-cache GOPROXY=off`; vorhandene Modulquellen,
Go `1.27.1 darwin/arm64`, keine Netzwerkdownloads. Die Sandbox verweigerte den
Standardcache; der temporäre Cache löst diese Umgebungsgrenze ohne Rechteänderung.

| Befehl | Tatsächlicher Ausgang |
| --- | --- |
| `gofmt -l .` | keine Ausgabe |
| `go vet ./...` mit obiger Umgebung | bestanden, keine Ausgabe |
| `go test -race -count=1 ./...` | **nicht vollständig grün**: vorhandener `TestMySQLPerConnectionLoggerSuppressesServerInput` scheitert am gesperrten `listen tcp 127.0.0.1:0: bind: operation not permitted`; übrige Pakete grün, Server-Livetests ohne Fixturevariablen übersprungen |
| `go test -race -count=1 ./... -skip '^TestMySQLPerConnectionLoggerSuppressesServerInput$'` | erster Kontrolllauf: Bestands-`TestCrashRecovery` mit `child timeout` (5-Sekunden-Grenze); kein neuer W-P19-Fehler |
| `go test -race -count=1 -p 1 ./... -skip '^TestMySQLPerConnectionLoggerSuppressesServerInput$'` | serieller Kontrolllauf bestanden, einschließlich Crash-Recovery; der eine Netzwerk-Regressionstest bleibt ausdrücklich ausgeschlossen |
| `python3 -B scripts/test-programs.py --sqlite-only` mit obiger Go-Umgebung | bestanden: SQLite, Dialektpläne und HTTP; sieben echte Antwortdumps gegen v19 validiert |
| `python3 -B scripts/check-table-contract.py` | bestanden |
| `python3 -B scripts/check-mutation-contract.py` | bestanden |
| `python3 -B scripts/check-insert-contract.py` | bestanden |
| `python3 -B scripts/check-structure-contract.py` | bestanden |
| `python3 -B scripts/check-create-contract.py` | bestanden |
| `python3 -B scripts/check-overview-contract.py` | bestanden, 32 Fixtures |
| `python3 -B scripts/check-object-ops-contract.py` | bestanden, 80 Fixtures |
| `python3 -B scripts/check-programs-contract.py` | bestanden, additive v19-Schemas und 24 Fixtures |
| `python3 -B scripts/check-docs.py` | bestanden, Endlauf einschließlich Handoff: 617 Dateien, null Fehler |
| `git diff --check` (nur lesend) | bestanden |
| `docker ps --format '{{.Names}} {{.Status}}'` | Zugriff auf Docker-API verweigert; keine Fixture gestartet |
| Prozessübersicht mit `ps` | Sandbox verweigert; fremde Hostjobs nicht prüfbar |

SQLite/HTTP tatsächlich abgedeckt: Viewwerte vor/nach Änderung; Sondernamen mit
Quotes, Backticks, Punkten, Semikolons und SQL-Text; Nachbartabelle und Basisdaten
bleiben; Trigger mit mehreren inneren Statements und Auditwirkung; Ersetzen und
gezieltes Löschen; Rollback eines DROP bei fehlerhaftem CREATE; angehängte zweite
Anweisung einschließlich ATTACH sicher abgewiesen; fehlendes Ziel/Schema,
unpassende Eingaben/Typen/Zeitpläne, Größenlimit, unsupported Routinen/Events,
abweichender Hash/Körper, falsche Aktion und getippter Name; Bearer-/Methodenprüfung,
fehlende Bestätigung, unbekannte geschachtelte Felder, Fehler ohne SQL-Spiegelung.
Dialektgegenproben sichern Quoting von PostgreSQL-Triggernamen mit Punkt und Quote,
Compound-Routinenkörper als ein Statement, MySQL-DROP/CREATE und Typinjektionsabwehr.

## Nicht ausgeführt und nächster Schritt

Nicht ausgeführt: `scripts/test-programs.py` als vollständige Docker-Serie gegen
MySQL 8.4, MariaDB 12.3 und PostgreSQL 18.6 mit TLS verify-full. Der Dockerzugriff
ist gesperrt. `TestLivePrograms/mysql`, `/maria`, `/postgres` wurden im allgemeinen
Go-Lauf übersprungen. Die geschriebenen Tests prüfen unter anderem IN/INOUT-
Metadaten, native Definition, Definer, Routinenersetzung ohne Aufruf, Triggerwirkung,
Abhängigkeiten/RESTRICT, deaktivierte Events, unveränderten Scheduler, Rechtefehler
und nicht atomaren MySQL-Teilabschluss. Diese Servernachweise sind **offen** und
wurden nicht durch synthetische Fixtures ersetzt.

Keine neuen Compose-/Swarm-, Browser-/Web-/Einbettungs-, Linux-/CI- oder Deployment-
Läufe. Kein tatsächlicher Schedulerlauf und kein Routinenaufruf (absichtlich außerhalb
des freigegebenen Umfangs). Vollständigen Go-Lauf einschließlich Netzwerk-Regression
auf einem Host mit erlaubtem Socket-Binding nachholen. Unabhängiger Review und
formaler Vertragsfreeze stehen aus; keine eigene Freigabeentscheidung des Autors.

Keine eigenen Container oder Hintergrunddienste gestartet. Temporäre Go-Caches und
Antwortdumps liegen außerhalb des Repos; keine Artefakte zur Integration erforderlich.
Alle eigenen gestarteten Prüfprozesse sind beendet; keine eigenen Prüfjobs laufen.
Nächster konkreter Schritt für den Lead: Dateien prüfen/sichern, unabhängigen Review
und vollständige dbfix-Serie in geeigneter Umgebung durchführen; erst anschließend
Vertragsindex/zentralen Status aktualisieren und Integration freigeben. Keine offenen
Produktänderungen oder Git-Schreibaktionen durch diesen Worker geplant.
