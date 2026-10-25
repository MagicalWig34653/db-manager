<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Handoff W-P17 Backend: Serververwaltung

Stand: 24.10.2026. Implementiert als **uncommitteter Reviewkandidat** im zugewiesenen
Worktree `.worktrees/w-p17-backend`, Branch `feat/w-p17-backend`, unveränderte Basis
`b74d10e6ff50ed49dccb0484ee3f8d2f0ca5b656`. Keine Git-Schreibbefehle ausgeführt;
Commit und Integration übernimmt der Lead. Unabhängiger Review, Liveabnahme und
Vertragsfreeze bleiben offen. Keine Vollparität und keine Veröffentlichung behauptet.

## Ergebnis und Dateibesitz

- `internal/core/serveradmin.go`: optionale Engine-Schnittstelle, lesende und
  schreibende Typen, ehrliche Fähigkeiten für SQLite/MySQL inklusive MariaDB/PostgreSQL.
- `internal/database/serveradmin.go`: Zeit-/Zeilen-/Bytegrenzen, Namenfilter,
  Pagination, numerische Statuszähler, maskierte Variablen, Prozessmetadaten ohne
  SQL/Host/Freitext; HMAC-Identitäten und Metadatenauflösung für bestätigte Einzelaktionen.
- `internal/httpapi/serveradmin.go`: acht authentifizierte v17-Routen,
  getrennte strikte Kill-/Variablen-Anfrageformen, vorhandene Fehlerkonventionen.
  `features.go`: genau eine Registrierung angehängt; `features_test.go`: nur
  erwartete Anzahl 8 auf 9 geändert, sonst unverändert.
- `internal/database/serveradmin_test.go`, `serveradmin_live_test.go` und
  `internal/httpapi/serveradmin_test.go`: SQLite, HTTP, Metadaten-Testtreiber,
  Maskierung, Whitelist, Hash-/Bestätigungsfehler, fremde/veränderte Identität,
  Selbstschutz, Verbindungszielbindung, Ablehnung/Unsicherheit, kein Retry,
  Vorabbruch und PostgreSQL-Signalantwort `false`.
- [Vertrag](../../contracts/api-v0.17.0.json),
  [Verhalten und Grenzen](../../contracts/SERVER-ADMIN-API.md),
  [27 synthetische Platzhalter-Fixtures](../../contracts/fixtures/server-admin/README.md).
  SHA-256 des vorläufigen Vertrags:
  `6f45a9545825969597e0e03580e74a1ec086901ab2c90dce4de640bb04a8618a`.
  V1 bis v8 byteidentisch, bisherige v8-Routen/Schemata unverändert übernommen.
- `scripts/check-server-admin-contract.py`: eingefrorene Bytes v1–v8,
  Additivität, lokale Referenzen, strikte neue Objektschemata, Muster und Fixtures.
  `scripts/test-server-admin.py`: vorhandene `dbfix.Fixtures`, je ein kurzlebiger
  Container nacheinander; echte Antwortschemas werden aus temporären Dumps geprüft.
  `.github/workflows/server-admin.yml`: eigener CI-Kandidat mit bestehenden Go-/Checkoutmustern.
- Keine Abhängigkeit ergänzt, keine Inventar-/Lizenzänderung erforderlich.
  Kein Fremdcode übernommen. Zentrale Status-/Plan-/Vertragsindexdateien, Web-Lock
  und Assets unverändert.

## Sicherheits- und Umfangsentscheidungen

Stufe 1 liest MySQL/MariaDB über SHOW GLOBAL STATUS, SHOW SESSION/GLOBAL VARIABLES
und information_schema.PROCESSLIST. PostgreSQL verwendet pg_stat_database,
pg_settings (SESSION) und pg_stat_activity; SQLite ausschließlich sqlite_version().
Nur bekannte unkritische numerische Variablenwerte sind lesbar, unbekannte und
sensible Werte bleiben `null/masked`. Freie SQL-Texte, Fehlertexte, Passwörter und
Dateipfade werden nicht ausgegeben. Details und genaue Grenzen stehen im Vertrag.

Stufe 2 bietet ausschließlich `kill` (query/connection) und `set_variable`.
Dynamische Whitelist: MySQL/MariaDB tmp_table_size/max_heap_table_size
(SESSION/GLOBAL); PostgreSQL statement_timeout/lock_timeout (SESSION). Vorschau
und Ausführung lösen das Ziel erneut aus Metadaten auf. dbplan.Check verlangt
Hash, Aktion und getipptes Ziel ID:Benutzer bzw. SCOPE:Variablenname. Wert, Scope,
Verbindungsziel und bisheriger Wert bzw. Prozessidentität sind im Hash gebunden.
Rechte prüft der Datenbankserver. Eigene Verwaltungssitzung/Systemprozesse,
Auth-/Rechte-/Verbindungsvariablen sind geschützt. Kein Selbst-Kill oder Abschalten
der eigenen Zugangsmöglichkeiten über die Whitelist.

HMAC-Token sind backendprozessgebunden; Neustart/anderer Prozess erfordert neue
Prozessliste/Vorschau. SESSION endet nach dieser API-Anfrage, Rücklesen erfolgt
vorher auf derselben Verbindung; keine persistente Benutzersitzung behauptet.
GLOBAL wirkt auf neue Sitzungen, ohne SET PERSIST. PostgreSQL GLOBAL wird explizit
abgewiesen. Prozesssignalbestätigung ist kein Beweis des Prozessabschlusses.
Verbindungs-/Zeitverlust nach Senden ergibt outcome_unknown, kein automatischer Retry.
Zwischen erneuter Metadatenprüfung und Signal bleibt ein Rennen; keine ABA-/Replay-/
Exactly-once-Garantie. Clients müssen Wiederholung nach completed/outcome_unknown sperren.

PMA-ADMIN-002..005 und PMA-NEU-ADMIN-001 sind implementierter Teilumfang, noch ohne
Liveabnahme. PostgreSQL hat keine gleichwertigen Zähler je MySQL-Anweisungstyp.
PMA-NEU-ADMIN-005 (FLUSH) bleibt ausdrücklich unsupported: Der konkrete Auftrag
begrenzt die Schreibstufe auf KILL und Variablen. Kein Web-/Monitor-/Advisor-/
Replikationsumfang und kein Anspruch vollständiger Matrixerfüllung.

## Tatsächliche Prüfungen

Go-Befehle mit `GOCACHE=/private/tmp/dbmanager-w-p17-gocache GOPROXY=off`.
Der erste Versuch mit Standardcache scheiterte an dessen Sandboxzugriff; der
separate temporäre Cache funktioniert ohne Moduldownloads. Prüfungen lokal unter
macOS arm64, keine Linux-/Containerabnahme daraus ableiten.

| Befehl | Tatsächlicher Ausgang |
| --- | --- |
| `gofmt -l .` | Keine Ausgabe. |
| `go vet ./...` | Bestanden. |
| `go test -race -count=1 ./...` | **Nicht vollständig grün:** ausschließlich bestehender Test `TestMySQLPerConnectionLoggerSuppressesServerInput` scheitert beim lokalen TCP-listen mit `bind: operation not permitted`; alle anderen Pakete grün. Live-Untertests ohne Fixturevariablen übersprungen. |
| `go test -race -count=1 ./... -skip '^TestMySQLPerConnectionLoggerSuppressesServerInput$'` | Alle Pakete bestanden; exakt der durch Sandbox gesperrte Bestandsfall ausgeschlossen. Live-Untertests weiterhin nicht ausgeführt. |
| `go test -race -count=1 ./internal/database ./internal/httpapi -run 'TestSQLiteServerAdmin\|TestServerAdmin\|TestLiveServerAdmin' -v` | Sämtliche lokalen W-P17-/SQLite-/HTTP-/Fehlerfälle bestanden. Live-Untertests mysql/maria/postgres explizit SKIP: disposable fixture required. |
| `python3 -B scripts/check-{table,mutation,insert,structure,create,overview,object-ops,server-admin}-contract.py` (acht Skripte einzeln) | Alle bestanden; neuer Wächter: 27 Fixtures, Referenzen/strikte Schemata, v1–v8 unverändert und v17 additiv. |
| `python3 -B scripts/test-server-admin.py` | Lokaler Paket-/SQLite-Teil mit Race bestanden; anschließend Abbruch bei `docker ps`: Zugriff auf Docker-API-Socket verweigert. Keine Container angelegt, keine Liveantwort-Dumps erzeugt. |
| Python-`ast.parse` beider neuen Skripte | Syntaxprüfung bestanden. |
| `python3 -B scripts/check-docs.py` | 615 Dateien geprüft, null Fehler. |
| `git diff --check` | Keine Ausgabe. Nur lesende Git-Befehle verwendet. |

Die vollständige Suite wurde zuerst ohne Ausnahme tatsächlich versucht, die
Sandboxblockade nicht als bestandener Test gewertet. Zieltests und Vet wurden nach
Folgekorrekturen wiederholt; abschließender Gesamtlauf mit obiger gezielter Ausnahme.

## Geschriebene, hier nicht ausgeführte Livefälle

`TestLiveServerAdmin/{mysql,maria,postgres}` über `scripts/test-server-admin.py`:
Status/Messzeitpunkt, MySQL-Com_select-Zuwachs durch synthetische Last,
Variablenfilter/Paging, SESSION setzen und auf derselben Sitzung nachlesen,
unveränderter Default einer neuen Sitzung, MySQL GLOBAL setzen/unabhängig
nachlesen/zurückstellen, nicht freigegebene Variable, falsche Bestätigung/Hash,
lange Fixtureabfrage erkennen/abbrechen und dieselbe Verbindung später beenden,
unbeteiligte Kontrollverbindung weiter nutzbar, fehlendes GLOBAL-Recht (MySQL/MariaDB)
bzw. fehlendes Signalrecht auf Superuserprozess (PostgreSQL), falsches Passwort ohne Echo.
Echte Antworten würden gegen v17 geprüft, keine synthetischen Fixtures als Livebeleg verwendet.

**Offen:** Alle genannten Livefälle auf MySQL 8.4, MariaDB 12.3 und PostgreSQL 18.6,
TLS-/Rechte-/Prozess-/GLOBAL-Abnahme, CI-Workflowausführung und unabhängiger Review.
Docker-Inventar und Start waren durch die Sandbox blockiert; `ps` zur hostweiten
Jobansicht ebenfalls verweigert. Keine eigenen Container oder schweren Buildjobs
angelegt. Keine Compose-/Swarm-/Browser-/Webbuildprüfungen für dieses Backendpaket.

## Fortsetzung

Dateien verbleiben ausschließlich im zugewiesenen Worktree, kein Commit/Index/Ref
verändert. Keine laufenden eigenen Prüfjobs oder unintegrierten Fremdarbeiten.
Nächster Schritt: Lead prüft Diff und erstellt den Commit; unabhängiger Reviewer
prüft den konkreten SHA und führt das Live-Skript auf einem Host mit Docker-/Netzzugriff
aus. Erst danach Vertragsfreeze/Nummernvergabe und Integrationsentscheidung.
