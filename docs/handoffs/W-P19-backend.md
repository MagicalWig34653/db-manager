# Handoff W-P19 Backend: Views, Routinen, Trigger und Events

Stand: 25.10.2026. Dateistand auf `feat/w-p19-backend`, Worktree
`.worktrees/w-p19-backend`. Verifizierter HEAD `c51c149` (Merge von main
`c331c64`), ursprünglicher WIP `fc67231`. Die folgenden Nachbesserungen liegen
**uncommittet** vor. Ausschließlich Dateien in diesem Worktree bearbeitet;
keine Git-Schreibbefehle, keine Änderungen der Git-Identität. Lead committet und
integriert. Die bestehende Featurezählung **13** bleibt unverändert.

## Ergebnis und Dateibesitz

Das WIP-Gerüst wurde anhand des Auftrags, der tatsächlichen Quellen und der
Welle-2-Integration nachgeprüft und für die endgültige API **0.17** vervollständigt.
Kein Web-Lock-Pin oder formaler Vertragsfreeze behauptet. Zentrale STATUS-/Plan-/
Vertragsindexdateien und fremde Pakete bleiben unverändert.

Geändert gegenüber HEAD:

- `internal/core/programs.go`, `internal/httpapi/programs.go`,
  `internal/httpapi/programs_test.go`: Version 0.17 und sechs Routen unter `/api/v16/`.
- `internal/database/programs.go`: Ersetzungspläne vereinigen begrenzt und
  dedupliziert bestehende und geplante Abhängigkeiten; neue PostgreSQL-
  Triggerfunktionen stehen auch beim Ersetzen ausdrücklich im Plan.
- `internal/database/programs_dependencies.go`: PostgreSQL-Beziehungen legen
  keine Objekte aus Schemas ohne USAGE-Recht offen.
- `internal/database/programs_test.go`, `programs_live_test.go`: SQLite-Regression
  für neue Ersetzungsabhängigkeiten; Servergegenproben für Schema-USAGE,
  Triggerfunktionswechsel und angehängte zweite Anweisungen. Positives Eventdatum
  auf 2030 korrigiert (MySQL-8.4-Eventgrenze 2038); Events bleiben deaktiviert.
- Neu `contracts/api-v0.16.0.json`; vorläufige `api-v0.19.0.json` entfernt.
  V17 übernimmt alle v12-Pfade, Schemas und Authentifizierung unverändert und
  ergänzt ausschließlich W-P19. V1 bis v12 bleiben byteidentisch.
- `scripts/check-programs-contract.py`: Pins v1 bis v12, Additivität zu v12,
  v13 bis v16 optional vorhanden. Falls solche Dateien beim Merge vorliegen,
  verlangt der Wächter deren additive Übernahme; der Lead gleicht diese ab.
- `contracts/PROGRAMS-API.md`, `contracts/fixtures/programs/` (Versionsantwort
  und Eventbeispiele), `.github/workflows/programs.yml`: finale Nummer,
  gekennzeichnete Platzhalter und ehrliche Dialektgrenzen.
- Neu `docs/decisions/ADR-W-P19-programmobjekte.md`; dieses Handoff ersetzt die
  überholten WIP-Angaben zu Basis, Version und Prüfstand.

Vertrag SHA-256:
`d9474f21b906bc4c6d63cb7cfe776cd48ba3e3365a5fe88fcbc980b34a4cb9fe`.
Keine neuen Abhängigkeiten oder Lizenzinventaränderungen. Eigener Code mit
SPDX AGPL-3.0-only, keine Attribution-Zeilen.

Zum vollständigen Paket gehören außerdem die bereits im WIP vorhandenen Dateien
`internal/database/programs_{catalog,create,sqlite,mysql,postgres}.go`,
`scripts/test-programs.py` und die Registrierung in `internal/httpapi/features.go`.
Der Harness verwendet `scripts/dbfix.py`, führt Serverfixtures seriell aus und
validiert tatsächlich erzeugte Antwortdumps gegen v16.

## Verhalten und Sicherheitsgrenzen

Lesen: Listen je Schema/Art/Tabelle, Definition als Text, Parameter, Eigenschaften,
Definer/SQL SECURITY, Abhängigkeiten, Eventzeitplan/-status und Schedulerstatus.
Vollständige Definitionen und Parameter über Detailroute; Listen liefern verfügbare
Katalogfelder. PostgreSQL-Viewdefinitionen sind als abgeleitet gekennzeichnet;
SQLite-Triggerzeitpunkt/-ereignis stehen im Definitionstext (`properties={}`).
Definitionsexport ist keine vollständige Wiederherstellung von Rechten/Eigentümern.

Schreiben: CREATE, REPLACE und DROP gemäß Treiberfähigkeit. Jeder Körper ist
Nutzer-SQL nach E5 mit exakten Statements, neu berechnetem Hash und Bestätigung.
DROP und jedes REPLACE verlangen zusätzlich den getippten Katalognamen.
Compound-Körper gehen als ein Statement über die Treiber-API, ohne Client-Splitting
oder DELIMITER. MySQL MultiStatements=false; SQLite/PostgreSQL Prepare weist
zusätzliche Statements ab. Kein eigener Parser und kein Syntaxnachweis durch Plan.
Namen vorhandener Objekte/Tabellen/Funktionen werden aus dem Katalog aufgelöst,
exakt verglichen und dialektgerecht gequotet. Neue Namen und Typoptionen sind
validiert. Explizite Definer-/Eigentümeraktionen tragen Warnungen, keine privilegierte
Ersatzverbindung. Öffentliche Fehler enthalten keine rohen Treiberfehler oder Pfade.

SQLite/PostgreSQL: transaktionale DDL-Gruppe; SQLite-View-/Triggerersetzung ist
DROP/CREATE ohne Tabellen-Neuaufbau. MySQL/MariaDB: teilweise DROP/CREATE mit
impliziten Commits; Fehler nach DROP meldet `outcome_unknown`, kein automatisches
Wiederherstellen. Keine CASCADE-Aktionen. Abschluss und Katalogbeobachtung sind
getrennt (`observation=unavailable`). Hash ist keine Einmal-Nonce; Clients sperren
nach Abschluss/Unsicherheit und verlangen neue Vorschau/Bestätigung. Das Fenster
zwischen Katalogprüfung und namensbasierter Ausführung bleibt ausdrücklich bestehen.

Abhängigkeiten: PostgreSQL exakt, aber nicht vollständig; Stringkörper/dynamisches
SQL können Verweise verbergen. SQLite/MySQL/MariaDB: gekennzeichnete Texttreffer
innerhalb eines Schemas, höchstens 1000 Kandidaten/100 Beziehungen mit Kürzungsflag.
Ersetzungspläne vereinigen Ist- und erkennbare neue Beziehungen; alte ausgehende
Verweise dürfen enthalten bleiben. Neue PostgreSQL-DDL hat noch keine vollständigen
Katalogbeziehungen. Kein garantierter späterer Katalogzustand.

Bewusste Paketgrenzen:

- PMA-PROG-001..004/006/007 und OBJ-014 werden backendseitig im beschriebenen
  Umfang bedient; Servernachweise sind teilweise offen, kein voller Paritätsclaim.
- **Keine Routinenaufrufe/-tests**, keine Aufrufroute; PMA-PROG-005 bleibt offen.
- **Scheduler nur lesend**. Livetests legen deaktivierte zukünftige Events an und
  kontrollieren unveränderten Scheduler; kein tatsächlicher Eventlauf.
- SQLite: Views/Trigger; keine Routinen/Events und kein Tabellen-Neuaufbau.
  PostgreSQL: Funktionen/Prozeduren und separate Triggerfunktionen; keine Events.
- PostgreSQL strukturierte Routinen: SQL/PLpgSQL, feste Typen und IN/OUT/INOUT;
  spezialisierte/variadische/Tabellen-/unbenannte Signaturen lesbar/löschbar,
  aber nicht vollständig strukturiert ersetzbar; Überladung bleibt erhalten.
- Kein eigener Linter (NEU-PROG-002), keine portable Eventfehlerliste/Rohlogs
  (NEU-PROG-003: `event_errors_state=unavailable`, Fähigkeit unsupported).

## Review und Prüfstand

Unabhängiger zweiter Agent führte ausschließlich einen statischen, lesenden Review
von Core/HTTP/Datenbankcode und der Nachbesserung durch; keine parallelen Schreib-
oder Testjobs. Befunde Schema-USAGE und geplante Triggerfunktion wurden behoben
und nachgelesen. Eventdatum und SQLite-Metadatengrenze anschließend korrigiert bzw.
dokumentiert und abschließend statisch nachgeprüft. Keine verbleibenden konkreten
Codebefunde gemeldet. Dies ersetzt keine unabhängige
Server-/Laufzeitabnahme oder Leadfreigabe.

Prüfumgebung: Go `1.27.1 darwin/arm64`, vorhandene Modulquellen, alle Go-Aufrufe mit
`GOCACHE=/tmp/dbmanager-w-p19-go-cache GOPROXY=off`; keine Netzwerkdownloads.

| Befehl | Tatsächlicher Ausgang |
| --- | --- |
| `gofmt -l .` | bestanden, keine Ausgabe |
| `go vet ./...` | bestanden, keine Ausgabe |
| `go test -race -count=1 ./...` | vollständiger Lauf wegen bestehendem `TestMySQLPerConnectionLoggerSuppressesServerInput` rot: `listen tcp 127.0.0.1:0: bind: operation not permitted`; Serverfixtures ohne Umgebungsvariablen übersprungen |
| `go test -race -count=1 -p 1 ./... -skip '^TestMySQLPerConnectionLoggerSuppressesServerInput$'` | bestanden, alle Pakete einschließlich neuer SQLite-Regression; ausschließlich der gesperrte Listener-Test ausgeschlossen, Server-Livetests ohne Fixtures übersprungen |
| `python3 -B scripts/test-programs.py --sqlite-only` | bestanden: SQLite, Dialektpläne, Ersetzungsabhängigkeiten und HTTP; neun echte Antwortdumps gegen v16 validiert |
| Alle `scripts/check-*-contract.py` | zwölf Wächter bestanden; W-P19 mit 24 Fixtures |
| `python3 -B scripts/check-docs.py` | bestanden: 888 Dateien, null Fehler |
| `git diff --check` (nur lesend) | bestanden |
| `docker ps --format '{{.Names}} {{.Status}}'` | Docker-API-Zugriff verweigert; keine Fixtures gestartet |

Ein Zwischenlauf nach Ergänzung der Livetests scheiterte beim Kompilieren an einem
erneuten `:=`; korrigiert zu `=`, danach Endläufe wie oben. Historische Prüfungen
des alten Handoffs werden nicht als Nachweis dieses Dateistands fortgeschrieben.

SQLite/HTTP abgedeckt: Views vor/nach Ersetzen, Sondernamen mit Quotes/Backticks/
Punkten/Semikolons/SQL-Text, Nachbartabelle und Basisdaten unverändert, Compound-
Trigger mit Auditwirkung, gezieltes Löschen, Rollback nach fehlgeschlagenem CREATE,
zusätzliche Anweisung/ATTACH abgewiesen, Schema-/Ziel-/Typ-/Zeitplanvalidierung,
Größenlimit, unsupported Fälle, Hash-/Körper-/Aktions-/Namensabweichungen,
Bearer/Methoden/geschachtelte unbekannte Felder und sichere Fehlerantworten.

## Nicht ausgeführt und Fortsetzung

Nicht ausgeführt: vollständige `python3 -B scripts/test-programs.py` gegen MySQL
8.4, MariaDB 12.3 und PostgreSQL 18.6 mit TLS verify-full. Dockerzugriff gesperrt;
`TestLivePrograms/{mysql,maria,postgres}` ohne Fixturevariablen übersprungen.
Die geschriebenen Tests prüfen Metadaten/Definitionen/Definer, IN/INOUT, Ersetzung
ohne Routinenaufruf, Triggerwirkung/Funktionswechsel, Abhängigkeiten/USAGE/RESTRICT,
angehängte Anweisungen, deaktivierte Events/Schedulerkonstanz, fehlende Rechte und
nicht atomaren MySQL-Teilabschluss. Diese Servernachweise bleiben **offen**;
synthetische Fixtures ersetzen sie nicht.

Keine neuen Compose-/Swarm-/Browser-/Web-/Linux-/CI-/Deploymentläufe. Kein Eventlauf
und kein Routinenaufruf (außerhalb Auftrag). Vollständigen Go-Lauf einschließlich
Listener-Regression auf einem Host mit erlaubtem Socket-Binding nachholen.

Keine eigenen Container oder Dienste gestartet. Temporäre Caches/Antwortdumps sind
keine Integrationsartefakte. Alle eigenen Prüfprozesse sind beendet; keine eigenen
laufenden Jobs oder unintegrierten anderen Worktrees. Hostjobs sind durch den
gesperrten Dockerzugriff nicht vollständig beobachtbar.
Nächster konkreter Schritt: Lead sichert den Dateistand per Commit, gleicht v13–v16
bei Integration ab und lässt die vollständige serielle dbfix-Serie sowie unabhängige
Laufzeitabnahme durchführen; danach zentrale Vertrags-/Statuspflege und Freigabe.
