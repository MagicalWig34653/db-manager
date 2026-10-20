# Tatsächlicher Projektstand

Stand: 20.10.2026, laufende autonome Umsetzung. Zeitgrenze: 21.10.2026 04:00
Europe/Berlin (=02:00 UTC), sofern das Gesamtprojekt nicht vorher vollständig abgenommen ist.
Lead koordiniert, reviewed und integriert ausschließlich; Agents implementieren.
Docker Compose und Docker Swarm sind verbindliche Betriebsziele (ADR-0003).

## Abgenommen und integriert

| Paket | Tatsächliches Ergebnis | Nachweis |
| --- | --- | --- |
| P0 | Drei öffentliche Git-Repos auf main, Lizenz/Prozess/CI, frische Checkouts geprüft | [P0-Review](handoffs/P0-review.md) |
| P1a | phpMyAdmin5.2.3-Referenz, 181 prüfbare PMA-Fälle, 6 eigene Produktziele; alle Implementierungsstände noch offen | [Paritätsmatrix](PARITAET.md), [Handoff](handoffs/P1a.md) |
| P2a | Go-/Swift-Treiberspikes, echte Wegwerf-DBs, native iPad-Simulatorproben; QA-Abbruchtestbefund korrigiert | [Handoff](handoffs/P2a.md), [Inventar](../spikes/drivers/LIZENZEN.md) |
| P3a | Bedrohungsmodell, Tresor-/Sync-Stufen, 36 Prüf-/Untersuchungsfälle, AEAD- und PRF-Compile-Spikes | [Handoff](handoffs/P3a.md), [Sicherheitsentwurf](security/TRESOR-UND-SYNC.md) |
| P4a | Baubare Go-HTTP-Grundlage mit Authentifizierung, Limits, sicheren Fehlern, API0.1.0 sowie Compose-/Swarm-Konfiguration | [Handoff](handoffs/P4a.md), [Betrieb](BETRIEB.md) |

Alle Pakete erhielten unabhängiges Review. P4a integriert als `bb37904` und `80f203b`.
API0.1.0 nach geschlossenem QA-P4-01 ausdrücklich als Clientvertrag freigegeben:
[OpenAPI](../contracts/api-v0.1.0.json), Herkunft `80f203b3e0c7fa3f7f4aa5ec06825cb83bc0c5d0`,
SHA256 `4b598dc69a1904685e67a717997412654440ac10f47c17f7a025ccd114e294be`.
B1 ist nach unabhängiger QA als reale SQLite-/MySQL-/MariaDB-/PostgreSQL-Engine
integriert (`87caed8`/`063209e`, Handoffcheckpoint `4056c99`). Der Server meldet diese
Treiber im dokumentierten eingeschränkten Betrieb. Webeinbettung und tatsächlicher
Browser→Go→DB-Pfad fehlen noch. Der native SQLite-Client ist
inzwischen als begrenztes eigenständiges Entwicklungsinkrement integriert;
keine vollständige Datenbankverwaltung oder Parität.

## Tatsächliche Prüfungen

- Go-Racetests, `go vet`, Format-/Diffchecks und Dokumentationsprüfungen für P4a
  durch Implementierer und QA bestanden; nach Integration `go test ./...` bestanden.
- Image tatsächlich gebaut; Compose sowie echter Swarm auf eigenem isolierten
  DinD-Daemon durch Implementierer und QA geprüft: Secretübergabe, Authentifizierung,
  Healthchecks, Swarm-Placement und Routing Mesh. Eigene Testressourcen entfernt;
  fremde Container und Host-Swarm unverändert. Kein HA-/Multinode-Nachweis.
- P2a Go: MySQL/MariaDB/PostgreSQL/SQLite gegen Wegwerfziele; Swift und iPad-Simulator:
  SQLite/MariaDB/PostgreSQL dokumentiert. QA wiederholte SQLite und Abbruchklassifikation,
  keine Live-Netzwerkproben. Kein echtes iPad getestet.
- Native MySQLNIO-TLS-Konfiguration kann tatsächlich unverschlüsselt verbinden;
  Downgrade reproduziert, native MySQL-/MariaDB-Produktadapter bleiben gesperrt.
- P3a-Spikes belegen AEAD-Manipulationsschutz und SDK-PRF-Verfügbarkeit; kein
  praktischer Provider-/Geräte-/Recovery-/CloudKit-Nachweis oder produktiver Tresor.

## Native App und Clientstand

A1 im Applerepo nach unabhängiger QA und erfolgreicher Apple-/Dokumentations-CI
integriert: `8ea3c7c`/`ec3bbe9`/`4404f91`, Statuscheckpoint `ce0aedb`.
Zehn Swift-Tests sowie Mac-/iPad-Simulator-Build belegt; UI-Dateiwechselbefund geschlossen.
[Native Quellen und Status](https://github.com/MagicalWig34653/db-manager-apple).
Signing/Sandboxdurchsetzung, interaktive OS-UI und echte Geräte bleiben offen.

F1/F1b-Webclient nach QA inklusive Session-/Zeilenzahlfix und CSP-tauglichen
Standalone-Validatoren freigegeben. Produktquelle `4e5abee` im Webrepo;
öffentliche Frontend-/Dokumentations-CI erfolgreich; per FastForward integriert,
zentraler Webstatuscommit `ec8dcea`. Reale B1-Integration fehlt.

## Laufende Arbeit und Dateibesitz

- Backend & Platform: P2b in `.worktrees/p2b`, Branch `spikes/p2b-sqlite-sandbox`;
  ncruces/Authorizer/VFS nach unabhängigem QA-Review als Untersuchung integriert
  (`7d2ed07`). ADR0004 eingeschränkt für B1 akzeptiert, keine Produktfreigabe.
  B1 jetzt in `.worktrees/backend-engine`, Branch `feat/backend-engine`: reale Go-Engine.
- Frontend & UX: F1 im Webrepo `.worktrees/web-workbench`, Branch `feat/web-workbench`;
  Svelte/shadcn-svelte-Workbench, jetzt Bindung an freigegebenen Vertrag erlaubt.
- Architecture/Security: S1 im Hauptrepo `.worktrees/vault-format`, Branch
  `feat/vault-format`; interoperables synthetisch geprüftes Vaultformat/KDF/Recovery.
  Noch keine produktive Credentialpersistenz. A1-Worktree bleibt saubere Provenienz.
- QA & Release: eigene Rolle, Reviews vor Integration. Maximal drei aktive Worker.
  Alte P1a/P2a/P3a/P4a-Worktrees bleiben als saubere Provenienz erhalten.

Aktive Produktänderungen liegen in diesen Worktrees und sind noch nicht integriert.
Prüfjobs liegen bei den jeweiligen Workern; keine produktiven Dienste oder Migrationen.
Keine echten Zugangsdaten speichern, keine fremden Docker-/Simulatorressourcen verändern.

## Repositories und Fortsetzung

Alle drei Repos öffentlich unter `https://github.com/MagicalWig34653/<name>`:
`db-manager`, `db-manager-web`, `db-manager-apple`, jeweils `main` mit origin-Tracking.
Git-/GitHub-Schreibblockaden der vorbereitenden Sitzung sind behoben.
Go1.27.1, Node26.5.0, Swift6.4/Xcode27.1 und Docker/Colima tatsächlich verfügbar.

Nächste Integration: F2a-Go-Webpaket nach eigener QA, dann F2b-Verdrahtung und echter Browserpfad. F1 und A1 getrennt
fertigstellen/reviewen, danach echten durchgehenden Web-/Docker-/Apple-Pfad prüfen.
Offen bleiben insbesondere vollständige Parität, produktiver Tresor/Recovery/Sync,
Passkey-Entsperren, Teamkonten/Rechte, native sichere Netzwerkadapter und echte Geräte.
Planung ist kein Funktionsnachweis. Konkrete Aufträge: [Arbeitsplan](ARBEITSPLAN.md),
Team: [TEAM](TEAM.md), Einrichtung: [EINRICHTUNG](../EINRICHTUNG.md).

Hauptstand `9f1880f`: [Backend-/Container-CI](https://github.com/MagicalWig34653/db-manager/actions/runs/39487622613)
und [Dokumentations-CI](https://github.com/MagicalWig34653/db-manager/actions/runs/39487622656)
tatsächlich erfolgreich. Neuere reine Status-/Untersuchungscommits separat prüfen.

## B1-Abnahmecheckpoint

Produktcode `fc1d68f` plus Fix `4ecd15f` und Handoffcheckpoints wurden nach
unabhängigen funktionalen und Securityreviews integriert. Alle vier Befunde
(SQLite-Typsemantik hoch, Metadata-IP-/Loggerpolicy mittel, Catalogfilter niedrig)
geschlossen. Kein konkretes Passwortleck/Metadata-HTTP-Exfiltration behauptet.
Go-Race/Vet nach Integration bestanden. Native Linuxamd64 und Container-CIs am Fix:
[Engine](https://github.com/MagicalWig34653/db-manager/actions/runs/39492062555),
[Backend/Container](https://github.com/MagicalWig34653/db-manager/actions/runs/39492062646),
[Dokumentation](https://github.com/MagicalWig34653/db-manager/actions/runs/39492062458),
alle erfolgreich. Exakte JSON-Grenze berücksichtigt Escaping/Base64/HTTP-Newline.
Keine umfassende Sandbox-/Powerloss-/HA-/Web-/Vault-/Team-/Paritätsfreigabe.
