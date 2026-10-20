# Übergabe P4a – HTTP-Vertrag und Docker-Grundlage

Stand: 20.10.2026. Branch `feat/p4a-foundation`, Worktree `.worktrees/p4a`.
Architecture & Security Engineer übernahm den uncommitteten Grundlagenentwurf
des Leads und stellte Prüfung, Korrekturen, Vertrag und Übergabe fertig.
Status: Kandidat zur unabhängigen QA; keine selbst erteilte Produktfreigabe.

## Nachbesserung aus unabhängiger QA

QA-P4-01 (mittel): `encoding/json` akzeptierte ausdrücklich angegebene Nullwerte
als Go-Nullwert bzw. ließ vorbelegte Werte unverändert; unter anderem
`max_rows:null` und `connection.database:null` passten dadurch nicht zum
nicht-nullbaren Schema. Zudem akzeptierte der HTTP-Validator ein ausdrücklich
leeres `tls_mode`, obwohl die Schema-Enum das verbietet.

Korrektur: Nach begrenztem Einlesen/typisierter Dekodierung prüft ein JSON-
Tokenscan jede explizite Null einschließlich durch doppelte Schlüssel verdeckter
Werte und liefert 400 vor dem Engine-Aufruf. Fehlendes `tls_mode` erhält vor
Dekodierung den Standard `verify-full`; ein ausdrücklich leerer Wert wird nun
abgewiesen. Ausgelassene optionale Felder sowie Querygrenzen behalten ihre
Standards. Das Schema und sein oben dokumentierter Hash bleiben unverändert.

Regressionen prüfen Null in sämtlichen Connection-Skalaren über alle drei POST-
Operationen, Null in SQL/Querygrenzen, leeres TLS, versteckte Null bei doppeltem
Schlüssel, ausgelassene Felder, beide erlaubten TLS-Modi und den gültigen String
`"null"`. README/Einrichtung unterscheiden nun baubare HTTP-/Containergrundlage
und weiterhin fehlende Datenbank-Engine. Die ursprüngliche Aussage, es gebe noch
gar keinen Produktcode/Go-Build, ist entfernt.

Nachprüfung am 20.10.2026: `go test -race -count=1 ./...`, `go vet ./...`,
`python3 scripts/check-docs.py` (89 Dateien, 0 Fehler), Format- und Diffprüfung
erneut bestanden. Vollständigen Build/Compose/isolierten Swarm bestätigte die
unabhängige QA separat; dieser reine Validator-/Dokfix wiederholt Deploymenttests
gemäß QA-Abstimmung nicht. Enge unabhängige Prüfung des Folgecommits bleibt offen.

## Ergebnis

- [OpenAPI 0.1.0](../../contracts/api-v0.1.0.json) und
  [normativer Verhaltensvertrag](../../contracts/README.md) für Health,
  Fähigkeiten, Verbindungstest, Tabellenkatalog und SQL-Ausführung.
- [Go-Einstieg](../../cmd/db-manager/main.go), Kernmodelle, dateibasierte
  Tokenkonfiguration und HTTP-Rand mit begrenzten Anfragen, Zeitlimits und
  bereinigten Fehlern. Die Engine ist ausdrücklich ein Platzhalter mit leerer
  Treiberliste; es ist noch keine echte Datenbankabfrage implementiert.
- [Dockerfile](../../Dockerfile), [Compose](../../compose.yaml),
  [Swarm-Stack](../../deploy/swarm.yaml), [Betriebsanleitung](../BETRIEB.md),
  [Tokenwerkzeug](../../scripts/create-dev-token.py) und
  [Deploymentprüfung](../../scripts/test-deployment.py).
- [Backend-CI](../../.github/workflows/backend.yml) für Go und tatsächlichen
  Compose-/isolierten Swarm-Test, [Lizenzinventar](../LIZENZEN.md).

Vertragskandidat SHA-256:
`4b598dc69a1904685e67a717997412654440ac10f47c17f7a025ccd114e294be`.
Clients pinnen nach unabhängiger Abnahme außerdem den integrierten Herkunftscommit.
Bis dahin bleibt Version 0.1.0 ausdrücklich ein Kandidat.

## Wesentliche Korrekturen bei Übernahme

Fehlende/ungültige Connection-Objekte, Treiber, Ports, TLS-Modi und unsichere
SQLite-Datei-IDs werden vor Engine-Aufrufen abgewiesen. Leere Text-/Binärzellen
behalten `value:""`, NULL hat kein value. Standardkonformes Medientyp-Parsing
akzeptiert Parameter/Großschreibung; ein übergroßer JSON-Nachlauf liefert 413.
Die Tests decken diese Unterschiede und die tatsächliche Kontextdeadline ab.

Der frühere Swarmtest verwendete die nicht akzeptierte Nodekennung `self`.
Er liest nun die tatsächliche lokale Swarm-Node-ID aus. Ein zufällig benannter
separater Docker-in-Docker-Daemon isoliert den Test vom Host-Swarm. Aufgeräumt
wird ausschließlich der erfolgreich selbst erzeugte Testdaemon samt anonymen
Volumes; kein generisches Docker-Pruning oder Stoppen fremder Ressourcen.

## Tatsächlicher Prüfstand

Toolchains: Go 1.27.1 Darwin/arm64; Docker Engine 29.5.2,
Docker Compose 5.3.1; isolierter Linux-Swarm-Testdaemon `docker:29.5.2-dind`.

| Befehl/Prüfung | Ausgang am 20.10.2026 |
| --- | --- |
| `go test -race -count=1 ./...` | bestanden in `internal/config`, `internal/core`, `internal/httpapi` |
| `go vet ./...` | bestanden |
| `go build -o /tmp/db-manager-p4a-build ./cmd/db-manager` | bestanden; temporäre Binärdatei nicht im Repo |
| `gofmt -l cmd internal` | keine Ausgabe nach Formatierung |
| `python3 scripts/check-docs.py` | abschließend 89 Dateien, 0 Fehler |
| `git diff --check` | bestanden |
| `python3 scripts/test-deployment.py --image db-manager:p4a --skip-build` | bestanden: Compose und isolierter Swarm mit vorherigem Grundlagenimage |
| `python3 scripts/test-deployment.py --image db-manager:p4a` | abschließender Build und echter Compose-/Swarm-Durchlauf mit überarbeitetem Quellstand bestanden, Exit 0; eigene Ressourcen anschließend entfernt |

Compose prüft HTTP-Health, 401 ohne Token, authentisierte Fähigkeiten,
Docker-Healthcheck, UID/GID 10001 und schreibgeschütztes Root-Dateisystem.
Swarm prüft echte Initialisierung, Secret, Placementlabel, einen konvergierten
Task, Routing-Mesh-HTTP, authentisierten Fähigkeitenaufruf und Task-Healthcheck.
Das ist ein echter isolierter Ein-Node-Swarm; kein Multi-Node-/HA-/Rollback-
oder langfristiger Persistenznachweis. CI wurde lokal statisch geprüft, läuft
erst nach Veröffentlichung/Integration tatsächlich bei GitHub.

## Vertrauens- und Implementierungsgrenzen

Ein persönliches administratives Bearer-Token, kein Benutzer-/Team-/Mandantenmodell.
Keine Credentialpersistenz, kein Tresor, kein iCloud, keine echten Geheimnisse
im Repository. Externe Bereitstellung benötigt HTTPS und Betreiber-Zugriffsregeln.
Swarm veröffentlicht über Routing Mesh; Compose bindet standardmäßig Loopback.
Die Go-Ausführung wird später übermittelte DB-Geheimnisse im Klartext benötigen.

Image-Tags sind noch nicht per Digest fixiert; vollständige Laufzeit-SBOM und
Release-Lizenzhinweise sind offen. Noch keine Netzwerk-Treiber oder deren
Zielnetzrichtlinie. Die normativen Ergebnisbytegrenzen sind bei Backendintegration
zu implementieren und real zu prüfen; die leere Engine beweist sie nicht.

SQLite-Vertrag: sichere Kennung wird zu `<kennung>.sqlite` unter einem fest vom
Betreiber gewählten Root. Pfad-/URI-/Symlink- und SQL-Dateizugriffsschutz benötigen
einen belastbaren Adapter-/Authorizer-/VFS-Nachweis. Ein naiver SQL-Präfixfilter
gilt nicht als Schutz. Ohne diesen Nachweis SQLite-Fähigkeit nicht freigeben.

## Übergabepunkte und nächste Schritte

Backendworker darf nach Leadfreigabe eigenes Engineverzeichnis, Adapter/Tests und
`go.mod`/`go.sum` bearbeiten sowie `pendingEngine` in `cmd/db-manager/main.go`
ersetzen. `core.Engine` bleibt `Test/Query/Catalog/Drivers`; `core.Connection`,
getaggte Stringzellen und Fehlercodes bilden den Vertrag. Vorhandene
`config.Config.DataDir`/`DBM_DATA_DIR` ist der Root-Übergabepunkt (Container `/data`,
direkt `./data`). Enginekonstruktion muss sicher fehlschlagen, wenn Initialisierung
oder Root-Prüfung scheitert. Neue Go-Abhängigkeiten auch im Docker-Build berücksichtigen.

Lead veranlasst unabhängige QA vor Integration und Vertragsfreigabe; Architektur-
Agent wechselt erst danach geordnet in das separate Apple-Paket. Keine weiteren
P4a-Dateiänderungen parallel zum Backendworker. Bisherige manuelle P4a-Ressourcen
`dbmanager-p4a-app-1` und `dbmanager-p4a-swarm` wurden nach Namens-/Image-/Projekt-
Labelprüfung entfernt; fremde P2a-/Timetrack-/Datenbankcontainer unangetastet.
Lokale ignorierte Tokenwerte werden nicht ausgegeben oder eingecheckt.
