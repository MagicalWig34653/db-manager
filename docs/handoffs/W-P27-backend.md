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

## Prüfstand (Fertigstellung 26.10.2026)

Basis: Merge von `main 3a7e1f1` (API 0.13-0.16) in `feat/w-p27-backend`; Konflikte in
`features.go`/`features_test.go` mit beiden Seiten aufgelöst (17 Features).
`contracts/api-v0.20.0.json` wurde auf v16 als Basis umgestellt (kumulativ, additiv
nur Monitor-Routen/-Schemas); der Wächter pinnt v1-v16 fest, v17-v19 sind optional
(noch nicht gemergt; bei Integration Pins ergänzen, sonst scheitert der Wächter bewusst).

| Befehl | Ausgang |
| --- | --- |
| `gofmt -l .`, `go vet ./...` | sauber |
| `go test -race -count=1 ./...` | alle Pakete bestanden |
| `python3 scripts/test-monitor.py` (Wegwerf-Container nacheinander: MySQL 8.4, MariaDB 12.3, PostgreSQL 18.6, plus SQLite) | bestanden; 24 echte Ergebnisse gegen v20 validiert; Container entfernt |
| alle `scripts/check-*-contract.py` | bestanden |
| `python3 scripts/check-docs.py` | 0 Fehler |

Ehrlich zur Replikation: Die Fixtures sind Einzelserver ohne konfigurierte Replikation;
Live-Replikationsantworten sind daher leere Statuslisten (Fähigkeits-/Rechteabfrage
funktioniert). Konfigurierte Replikas, Mehrquellen, WAL-Receiver, Logrotation,
Lastfälle sind nicht live nachgewiesen. Die Fixtures unter `contracts/fixtures/monitor/`
unterscheiden Platzhalter und echte SQLite-Antworten. Binärlog-/Analyse-/Profiling-
Antworten wurden live auf allen drei Netzwerkengines erzeugt und geprüft; keine
Secrets in den Ergebnissen gefunden. Nur lesend; keine START/STOP-REPLICA-Aktionen.

Nicht ausgeführt: `node scripts/check-pwa-embedded.mjs` (playwright-core fehlt),
neuer GitHub-Workflow `monitor.yml` nicht in CI gelaufen, unabhängiger Review offen.

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
