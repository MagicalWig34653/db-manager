# Handoff W-P27 Backend: lesender Monitor (API 0.20)

Stand: 25.10.2026. Branch `feat/w-p27-backend`, Basis `main c331c64`.
Nur Dateien geschrieben; kein Git-Schreibbefehl, kein Commit, keine Integration.
Unabhängiger Review und Leadcommit offen. Zentrale Status-/Planungsdateien bleiben
beim Lead. Alle Änderungen sind auf dieses Paket begrenzt.

## Ergebnis und Dateibesitz

- `internal/core/monitor.go`: Anfrage, Ergebnis, Capability-Vertrag, Interface.
- `internal/database/monitor*.go`: Einzelmessung, Analysequellen, feste Advisor-
  Regeln, Replikationsstatus, Binärlogliste/-ereignisse, begrenztes EXPLAIN,
  Engineinformationen; SQLite-Version und echte Tests, Netzwerk-Livetests.
- `internal/httpapi/monitor*.go`: authentifizierte `/api/v20/monitor/`-Routen,
  Fähigkeiten, sichere Fehler und HTTP-Tests. Registry: eine zusätzliche Zeile,
  Featurezahl im Test 13. W-P17 wird nicht dupliziert.
- `contracts/api-v0.20.0.json`: additiv zu v12, vorhandene Verträge unverändert;
  `contracts/MONITOR-API.md`, `contracts/fixtures/monitor/`,
  `scripts/check-monitor-contract.py`, `scripts/test-monitor.py` und eigener
  CI-Workflow `.github/workflows/monitor.yml`.
- [ADR W-P27](../decisions/ADR-W-P27-lesender-monitor.md): begrenzte Messdaten und
  Geheimnisschutz; Status Vorschlag, keine Leadentscheidung vorweggenommen.

Keine neuen Abhängigkeiten. Eigener Code AGPL-3.0-only; keine Attributionzeilen.
Fixtures unterscheiden ausdrücklich synthetische Platzhalter und echte SQLite-
Antworten. Keine produktiven Verbindungen/Logs, Passwörter oder Dateipfade in
Antworten/Artefakten. Keine Migration, keine persistente Messhistorie.

## Prüfstand

Geprüft am 25.10.2026 mit Go 1.27.1, darwin/arm64. Für alle Go-Befehle:
`GOCACHE=/tmp/dbmanager-w-p27-gocache GOPROXY=off GOSUMDB=off`.
Der erste Versuch mit dem Standardcache scheiterte an Sandbox-Schreibrechten;
der temporäre Cache benötigt weder Downloads noch Git-Schreibrechte.

| Befehl | Tatsächlicher Ausgang |
| --- | --- |
| `gofmt -l .` | bestanden, keine Ausgabe |
| `go vet ./...` | bestanden, auch nach letzten Produktänderungen |
| `go test -race -count=1 ./...` | ausgeführt, scheitert ausschließlich am bestehenden `TestMySQLPerConnectionLoggerSuppressesServerInput`: lokales TCP-Binding durch Sandbox verboten |
| `go test -race -count=1 ./... -skip '^TestMySQLPerConnectionLoggerSuppressesServerInput$'` | erster Lauf bestanden; abschließende Wiederholung: alle Pakete außer `sqlitesandbox` bestanden, bestehender `TestCrashRecovery` mit `child timeout` |
| `go test -race -count=1 ./internal/sqlitesandbox -run '^TestCrashRecovery$' -v` | bestanden (3,11 Sekunden Testlauf) |
| `go test -race -count=1 -p 1 ./... -skip '^TestMySQLPerConnectionLoggerSuppressesServerInput$'` | bestanden, alle Pakete; nur der eine gesperrte Listener-Test ausgeschlossen |
| `python3 scripts/test-monitor.py --sqlite-only --keep /tmp/w-p27-results` | bestanden; sechs gezielte Tests einschließlich HTTP/Redaktion/Paging/Advisor/PG-Planparser; zwei echte SQLite-Antworten gegen v20 validiert |
| `python3 scripts/test-monitor.py --keep /tmp/w-p27-live-results` | SQLite-Teil bestanden; erster Docker-Create scheitert an gesperrtem Socket, Netzwerkengines nicht gestartet |
| `python3 scripts/check-<paket>-contract.py` für alle zwölf vorhandenen Vertragswächter, jeweils einzeln | alle bestanden; Monitorwächter validiert 14 Fixtures, Additivität und feste v1-v12-Bytes |
| `PYTHONPYCACHEPREFIX=/tmp/dbmanager-w-p27-pycache python3 -m py_compile scripts/check-monitor-contract.py scripts/test-monitor.py` | bestanden |
| `python3 scripts/test-build-web.py` | 25 Tests bestanden |
| `python3 scripts/build-web.py --check` | eingebettete Assets und Provenienz unverändert bestätigt |
| `python3 scripts/check-docs.py` | bestanden, 877 Dateien, null Fehler |
| `git diff --check` | bestanden |
| `node scripts/check-pwa-embedded.mjs` | nicht ausführbar: `playwright-core` fehlt; keine Installation/Netzwerkfreigabe versucht |

Vertrags-SHA256: `6f950582d9ff4d58a0b5cf186d5a5e19b64699564761165f1532f535b028d2fc`.
V13 bis v19 fehlen auf dieser Basis und werden nicht vorausgesetzt; falls der Lead
sie später hinzufügt, verlangt der Wächter deren feste Pins bei der Integration.

### Ausdrücklich nicht nachgewiesen

- Echte MySQL 8.4/MariaDB 12.3/PostgreSQL 18.6-Monitorantworten, deren Rechte-,
  Binärlog-/Ereignis-, Analyse- und EXPLAIN-Prüfungen: Docker-Socket gesperrt.
  Die Tests sind über `scripts/dbfix.py` geschrieben, aber nicht als grün belegt.
  Der fertige Harness aktiviert nur in eigenen Wegwerffixtures Binärlog,
  Performance-Schema/Slow-Log-Tabelle beziehungsweise PostgreSQL-Preloading;
  der Test prüft fehlende und anschließend installierte `pg_stat_statements`.
- Konfigurierte echte Replikation/Mehrquellen-/WAL-Receiver-Zustände, Last-,
  Logrotations- und reale Netzwerkabbruchfälle; der vorbereitete Live-Smoke-Test
  prüft nur Status ohne eingerichtete Replikation, Rechte und Kontextabbruch.
- Unveränderter Listener-Regressionstest in dieser Sandbox; vollständige Suite
  ohne Ausschluss bleibt deshalb offen. Crash-Recovery-Flakiness separat erfasst.
- PWA-Browserwächter wegen fehlendem `playwright-core`, echte Browser-/Compose-/
  Swarm-Läufe und der neue GitHub-CI-Workflow; keine Webproduktänderung.
- Unabhängiger Review, Integration und Commit durch den Lead.

`docker ps` und Prozessauflistung (`ps`) sind ebenfalls gesperrt. Beim abgewiesenen
Create registrierte `dbfix` bereits `dbmanager-w-p27-dcfc6af4-mysql`; der anschließende
Cleanupversuch wurde ebenfalls vom Socket abgewiesen. Es gibt keinen erfolgreichen
Create-/Startnachweis. Eine nachträgliche Docker-Auflistung/Cleanupbestätigung ist
hier nicht möglich; der Lead soll den eigenen Präfix auf einem erlaubten Host
prüfen. Keine fremden Ressourcen angefasst.

## Einschränkungen und nächste Schritte

Nur lesende Teilabdeckung der Inventarzeilen; schreibende ADMIN-012, PRIV-008,
NEU-ADMIN-010/011 bleiben offen. Kein General-Log-Rohtext, keine Dateilogs,
kein vollständiges Laufzeitprofil, nur eingeschränktes EXPLAIN. SQLite nur Version.
Binärlogrotation und veränderliche Statistiken verhindern eine Snapshotzusage.
Details: [Verhaltensvertrag](../../contracts/MONITOR-API.md).

Docker-Socket und lokale Port-Bindings sind im aktuellen Sandboxlauf gesperrt.
Echte MySQL-/MariaDB-/PostgreSQL-Abnahme, installierte Analyseextension,
konfigurierte Replikation und unabhängiger Review müssen auf geeignetem Host folgen.
Gitmetadaten nicht verändert; Commitmetadaten sind erst durch den Lead zu prüfen.
Nächster Schritt: unabhängiger Review und echte Liveprüfungen auf einem Host mit
Dockerzugriff, anschließend Leadcommit und Integration. Alle eigenen Prüfprozesse
sind beendet; keine laufenden Go-/Python-/Buildjobs. Unintegrierte Arbeit: sämtliche
oben genannten Dateien liegen uncommitted im zugewiesenen Worktree.
