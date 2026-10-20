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
Treiber im dokumentierten eingeschränkten Betrieb. Go-Webpaket `15b339e`/`c9cac01`
und reale Serververdrahtung F2b `df0b291`/`c7e1a1d` sind integriert. Echter
Browser→Go→SQLite/PostgreSQL-TLS-Pfad sowie Compose-/Swarm-Persistenz unabhängig
geprüft. Der native SQLite-Client ist
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
zentraler Webstatuscommit `ec8dcea`. Reale B1-Integration über F2b abgenommen.

## Laufende Arbeit und Dateibesitz

F2b und B2a sind abgenommen und integriert; API0.2.0 ist für Clients freigegeben.
Frontend implementiert F3, Backend B2b, Apple-Engineer A2.
Die laufende Sitzung bietet weiterhin vier Plätze einschließlich
Lead. Nachträglich lokal konfigurierte höhere Limits sind hier nicht wirksam geworden.

| Paket / Rolle | Branch / Worktree | Tatsächlicher Auftrag |
| --- | --- | --- |
| A2 / Apple Engineer | Applerepo, `feat/apple-postgres`, `.worktrees/apple-postgres`, Basis `ce0aedb` | Direkter PostgreSQL-Pfad ohne Go, verpflichtend verifiziertes TLS, flüchtige Credentials, Katalog/einzelne SQL-Anweisung und begrenzte getaggte Ergebnisse. Besitz Apple-Produktmodule/SwiftUI, Package/Lock/Xcode, eigene Prüfungen/CI/Lizenzen/ADR/Einrichtung/Handoff; zentrale Status-/Planungsdateien ausgenommen. SQLitebetrieb erhalten; keine native MySQL-Freigabe. |
| F3 / Frontend & UX | Webrepo, `feat/table-workbench`, `.worktrees/table-workbench`, Basis `ec8dcea` | Sichtbare Quellcode-/Lizenzverweise und strukturierte Tabellenmetadaten/-ansicht mit gepinntem API0.2-Vertrag, Sortierung, AND-Filtern und Pagination; echte SQLite-/PostgreSQL-TLS-Browserproben. Besitz Webprodukt/Build/Vertragskopien/Tests/Lizenzen/README/Einrichtung/Handoff; keine zentralen Status-/Planungsdateien. Kein Mutation-/Persistenzumfang. |
| B2b / Backend & Platform | `feat/table-edit`, `.worktrees/table-edit`, Basis `6009e1d` | Einzelzeilen-UPDATE/DELETE mit DB-Transaktion, gesperrtem aktuellem Datensatz und exakt typisiertem vollständigem Erwartungssnapshot; nur belegte PK-/Transaktionseignung. Neuer separater v3-Vertragskandidat, v1/v2 unverändert; keine Inserts/Massenänderung/Schemafunktionen. Besitz neue DB-/Core-/HTTP-Mutationsdateien, enge Helper/Routingregistrierung, Vertrag/Doku, eigene Prüfungen/CI/Handoff. |

S1 nach unabhängiger QA ohne bestätigte Befunde als Formatgrundlage integriert:
`7969f63`/`bc5dd4c`, Herkunft `1a4eb3c`/`03bd3f4`. Vier Go-/vier Swift-Tests,
Race/Vet, 24 positive und 232 negative wechselseitige CLI-Fälle sowie Originalquellen-/
Lizenzprüfung unabhängig bestätigt. ADR0005 akzeptiert; Schema-SHA256
`b688656056fe2e23848ce17149bc817609c7369f230a3f49dd91f05db4d3f4dd`.
Keine produktive Credentialpersistenz. [Abnahme und Grenzen](handoffs/S1.md).

B2a `62f1297`/`573e392` plus Fix `3b5a54c` nach unabhängiger Fixnachprüfung und
erfolgreicher nativer Fix-CI integriert als `cb07ebb`/`19673b3`/`427444d`.
API0.2.0 für Clientbindung freigegeben, Herkunft
`427444d7bcb8e30c9563ded2000ec3d5e19646c4`, Schema-SHA256
`c15a848f01bb779429fd68a2b6eb6bba1575d3e07a4c68e6796cf7840a661838`.
Pflichtfeld-/Kollationsbefunde geschlossen. [Abnahme/CI](handoffs/B2a.md).
Integrierte Go-Race/Vet-, Vertrags- und Dokumentationsprüfung bestanden.

Gemeinsame Verträge benötigen vor abhängiger Produktarbeit unabhängige Abnahme.
Kein paralleles Schreiben derselben Dateien. Zentrale Status-/Planungsdateien gehören
dem Lead. Alte saubere Worktrees bleiben als Provenienz erhalten.

Aktive Produktänderungen sind noch nicht integriert. Prüfjobs liegen bei den
jeweiligen Workern; keine produktiven Dienste oder Migrationen. Keine echten
Zugangsdaten speichern, keine fremden Docker-/Simulatorressourcen verändern.

## Repositories und Fortsetzung

Alle drei Repos öffentlich unter `https://github.com/MagicalWig34653/<name>`:
`db-manager`, `db-manager-web`, `db-manager-apple`, jeweils `main` mit origin-Tracking.
Git-/GitHub-Schreibblockaden der vorbereitenden Sitzung sind behoben.
Go1.27.1, Node26.5.0, Swift6.4/Xcode27.1 und Docker/Colima tatsächlich verfügbar.

Nächste Pakete: Frontend F3 und separate Datenmutation B2b auf dem
freigegebenen B2a-Vertrag. S1-Format ist abgenommen; produktive Persistenz S2
bleibt separat. A2 erweitert Apple um den direkten PostgreSQL-Pfad.
Offen bleiben insbesondere vollständige Parität, produktiver Tresor/Recovery/Sync,
Passkey-Entsperren, Teamkonten/Rechte, native sichere Netzwerkadapter und echte Geräte.
Planung ist kein Funktionsnachweis. Konkrete Aufträge: [Arbeitsplan](ARBEITSPLAN.md),
Team: [TEAM](TEAM.md), Einrichtung: [EINRICHTUNG](../EINRICHTUNG.md).

Hauptstand `21bd46a`: [Backend-/Container-CI](https://github.com/MagicalWig34653/db-manager/actions/runs/39493600835),
[Engine-CI](https://github.com/MagicalWig34653/db-manager/actions/runs/39493600757) und
[Dokumentations-CI](https://github.com/MagicalWig34653/db-manager/actions/runs/39493600707)
tatsächlich erfolgreich. Neuere Commits separat prüfen.

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

F2a nach unabhängiger öffentlicher Reproduktion abgenommen und integriert:
Assetbytes/Manifest/Notices exakt, strikte CSP, MIME/Cache/Traversal/API-Passthrough
geprüft. Originaler generierter JS-Leerraum unverändert; eigene Quellen diff-sauber.
Integrierte Go-Race/Vet-, Offline- und Pythonprüfungen bestanden. B2a-Backendticket
für strukturierte Tabellenmetadaten/-ansicht im Worktree `.worktrees/table-browse`,
Branch `feat/table-browse` beauftragt, neuer Vertrag erst nach eigener QA freigeben.

## F2b-Abnahmecheckpoint

`f2c92ee`/`c9ced80` nach unabhängigem Review ohne bestätigte Befunde integriert
als `df0b291`/`c7e1a1d`. QA wiederholte sauberen Archivbuild, Go-Race/Vet,
Manifest-/sechs Pythonprüfungen, echten Chromium→Go→SQLite/PostgreSQL-TLS-Pfad
und Compose-Neustart-/isolierte-Swarm-Taskersatzpersistenz. DB-Arbeit vor Abbruch
und tatsächliche Freigabe danach beobachtet; Typen/Auth/Tokenwechsel/CSP bestätigt.
Image ohne Node/npm, Original-Lizenzen/Assets/Provenienz geprüft. Integrierte
Go-Race/Vet-, Manifest-/Python- und Dokumentationsprüfung ebenfalls bestanden.
Feature-CIs [Webpfad](https://github.com/MagicalWig34653/db-manager/actions/runs/39495523677),
[Backend/Container](https://github.com/MagicalWig34653/db-manager/actions/runs/39495523685),
[Engine](https://github.com/MagicalWig34653/db-manager/actions/runs/39495523709),
[Dokumentation](https://github.com/MagicalWig34653/db-manager/actions/runs/39495523691)
alle erfolgreich. Keine eigenen Jobs/Ressourcen verblieben, Host-Swarm unverändert.
Keine Paritäts-/HA-/Vault-/Distributionsfreigabe. [Handoff](handoffs/F2b.md).
