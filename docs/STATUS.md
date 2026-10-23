# Tatsächlicher Projektstand

Stand: 23.10.2026. Der Nutzer hat die autonome Umsetzung ausdrücklich verlängert:
bis zum vollständigen Abschluss oder heute 23.10.2026 10:00 Europe/Berlin (=08:00 UTC).
Lead koordiniert Reviews und Integration; Agents übernehmen Umsetzung, Dokumentation,
Prüfungen und freigegebene Integrationsschritte.
Docker Compose und Docker Swarm sind verbindlich (ADR-0003).

## Abgenommen und integriert

| Paket | Tatsächliches Ergebnis | Nachweis |
| --- | --- | --- |
| P0–P4a, P2b | Öffentliche Repos, Anforderungen/Paritätsreferenz, Treiber-/Securityspikes, HTTP/Auth-/Containergrundlage, eingeschränkte SQLite-VFS-Entscheidung | [P0](handoffs/P0-review.md), [P1a](handoffs/P1a.md), [P2a](handoffs/P2a.md), [P3a](handoffs/P3a.md), [P4a](handoffs/P4a.md), [P2b](handoffs/P2b.md) |
| B1 | Reale SQLite-/MySQL-/MariaDB-/PostgreSQL-Engine mit verifiziertem Netzwerk-TLS und begrenzten typisierten Ergebnissen; vier Reviewbefunde geschlossen | [B1](handoffs/B1.md) |
| A1/A2 | Eigenständiger nativer SwiftUI-/SQLite-/PostgreSQL-Arbeitsplatz ohne Go; verpflichtendes Netzwerk-TLS, präzise Werte und serverseitiger Abbruchpfad, Mac-/iPad-Simulator-Builds. Lifecycle-/Kalenderbefunde geschlossen | [Applerepo](https://github.com/MagicalWig34653/db-manager-apple), dort Handoff A1 |
| F1/F1b, F2a/F2b | Deutscher Web-SQL-Arbeitsplatz, reproduzierbare Offlineassets, echtes Browser→Go→DB-Produkt; Session-/CSP-Befunde geschlossen | [F2a](handoffs/F2a.md), [F2b](handoffs/F2b.md) |
| B2a | Strukturierte Tabellenmetadaten, Sortierung, gebundene AND-Filter und Pagination; Pflichtfeld-/Kollationsbefunde geschlossen | [B2a](handoffs/B2a.md) |
| B2b | Bestätigtes Einzelzeilen-UPDATE/DELETE mit typisiertem Erwartungssnapshot, echten Locks/Transaktionen und ehrlichem unbekanntem Commitausgang | [B2b](handoffs/B2b.md) |
| B2b-Schutzfixes | SQLite OR ABORT verhindert fremde REPLACE-Löschungen; PostgreSQL-Rewrite-Tabellen werden gesperrt. Beide nachträglichen Integritätsbefunde unabhängig geschlossen | [SQLite](handoffs/B2b-konfliktstrategie.md), [PostgreSQL](handoffs/B2b-rewrite-regeln.md) |
| B2c | Bestätigtes einzelnes INSERT mit getrennten Default-/NULL-Werten, konservativer PK-Ermittlung und tatsächlicher Rücklese nach Triggern | [B2c](handoffs/B2c.md) |
| F3/F3b | Strukturierte Web-Tabellenoberfläche und Quellen-/Originallizenzdialog, exakt reproduziert im Go-/Dockerprodukt eingebettet. Filterzustandsbefund QA-F3-01 geschlossen | [F3b](handoffs/F3b.md), [unabhängiger Review](handoffs/F3b-backend-review.md) |
| F4 / Webquelle | Einzelzeilen-Insert/Edit/Delete nach unabhängigem P0-Review im Webrepo integriert: Quelle `3a4a3fd`, Review `01933d5`, Merge `f28de33`, öffentliches Web-main `bf75748df9eb4422b084c20784d09ce931301d37`. Root-Einbettung F4b separat. | [Webquellen](https://github.com/MagicalWig34653/db-manager-web) |
| F4b | Unabhängig angenommene F4-Quelle exakt reproduziert im Go-/Dockerprodukt; neun echte Browserfälle, strukturierte CRUD-/409-/typisierte Persistenz in Compose und isoliertem Swarm. Produkt3770c4e/Handoff46e4bf1/Review533348a integriert als ffcf519/2ad78b2. | [F4b](handoffs/F4b.md), [unabhängiger Review](handoffs/F4b-backend-review.md) |
| S1 | Interoperables versioniertes Tresorformat mit Argon2id/AES-GCM/Recovery; ausschließlich Formatgrundlage, keine produktive Credentialpersistenz | [S1](handoffs/S1.md), [Format](security/VAULT-FORMAT-V1.md) |

Historische F3-Webquelle `6c6eb33c93eed355a10cd114f61ce27be7f3f55e`, Web-main `ffdd2a0`.
F3b-Produktquelle `639ba44`, Teststand `7500ed2`, Handoff `3f7a863`, unabhängiger
Review `e274068`; integriert als `0767969`/`22fdf78`/`cc175a5`/`5cb604a`.
Die ausführlichen historischen Befunde und Prüfbefehle bleiben in den Handoffs.

## Freigegebene Verträge

| Vertrag | Herkunft | SHA256 |
| --- | --- | --- |
| API0.1.0 | `80f203b3e0c7fa3f7f4aa5ec06825cb83bc0c5d0` | `4b598dc69a1904685e67a717997412654440ac10f47c17f7a025ccd114e294be` |
| API0.2.0 | `427444d7bcb8e30c9563ded2000ec3d5e19646c4` | `c15a848f01bb779429fd68a2b6eb6bba1575d3e07a4c68e6796cf7840a661838` |
| API0.3.0 | `946241d7f10f7444e9e5efebbe8b3a38a8120a9b` | `ec77c9739cda11df3f4cb9fe756a632e67840fdc0f6190deee1aa1d1333dc308` |
| API0.4.0 | `72855cf57ceb6620dead42856ee6c9d52d0612bd` | `4763ebf8632a9ef977cf7d3a6037299fc4dafde0a581fe10cb34d844f8356fbf` |
| Tresorformat1 | `1a4eb3c`/`03bd3f4`, ADR0005 | `b688656056fe2e23848ce17149bc817609c7369f230a3f49dd91f05db4d3f4dd` |

Die APIversionen sind getrennt; v1-Capabilities bleiben unverändert.
**Neue Schreibclients pinnen Backend `72855cf57ceb6620dead42856ee6c9d52d0612bd`**:
Dieser enthält beide v3-Schutzfixes. Die frühere v3-Implementierung ist für SQLite-
REPLACE- und PostgreSQL-Rewrite-Tabellen unsicher; identische Vertragsbytes beweisen
keinen gefixten Server. [Vertragsindex](../contracts/README.md).

## Tatsächlicher Prüfstand

- Hauptstand `03926cc`: alle sieben CIs erfolgreich: [Backend/Container](https://github.com/MagicalWig34653/db-manager/actions/runs/39503099272), [Engine](https://github.com/MagicalWig34653/db-manager/actions/runs/39503099334), [Tabellen](https://github.com/MagicalWig34653/db-manager/actions/runs/39503099117), [Mutationen](https://github.com/MagicalWig34653/db-manager/actions/runs/39503099112), [Inserts](https://github.com/MagicalWig34653/db-manager/actions/runs/39503099130), [Webpfad](https://github.com/MagicalWig34653/db-manager/actions/runs/39503099124), [Dokumentation](https://github.com/MagicalWig34653/db-manager/actions/runs/39503099293).
- F3b-Teststand `7500ed2`: alle sechs damaligen CIs erfolgreich, einschließlich [Engine](https://github.com/MagicalWig34653/db-manager/actions/runs/39502849585), [Backend](https://github.com/MagicalWig34653/db-manager/actions/runs/39502849685) und [Webpfad](https://github.com/MagicalWig34653/db-manager/actions/runs/39502849674). Unabhängig zweimal bytegleich öffentlich gebaut; fünf echte Browserfälle, Compose und eigener isolierter DinD-Swarm einschließlich SQLite-Neustart/Taskersatzpersistenz bestanden. Positiver PostgreSQL-TLS-Test geht Negativtest voraus; TCP-/Bootstrapbarriere schließt Readinessrace.
- Integrierter F3b-Hauptstand `5cb604a`: Go-Race/Vet, Manifest mit fünf Originalassets, acht Python-Buildprüfungen, Insertvertrag und Dokumentation266/0 bestanden. Alle sieben kombinierten Haupt-CIs an `f46cafe` inzwischen erfolgreich: Docs39504714727,
  Inserts39504714690, Backend39504714680, Tabellen39504714594,
  Mutationen39504714593, Engine39504714563, Web39504714562.
- S1 unabhängig: vier Go-/vier Swift-Tests, Race/Vet, 24 positive und 232 negative wechselseitige CLI-Fälle, 14 Originalquellvergleiche und Lizenzen bestanden.
- Tests verwendeten ausschließlich eigene Wegwerfressourcen. Host-Swarm blieb inaktiv, fremde Container unverändert. Keine eigenen Leadprüfjobs offen; Workerjobs stehen in ihren Handoffs. Originale generierte JS-/Lizenzbytes bleiben unverändert.

## Laufende Arbeit und unabhängige Gates

| Paket / Rolle | Repo, Branch / Worktree | Tatsächlicher Stand und Dateibesitz |
| --- | --- | --- |
| A2 / integriert | Apple-main `3c92f974aa6d1c3056b25e3bc9ec5b0d0cb9f161` (Produkt `7e97aae`), Provenienz `feat/apple-postgres` / `.worktrees/apple-postgres` | Quellen89b2683/abaa3dc/b72e62c als fdd533e/cd5d886/031b1e8 nach unabhängiger QA und exakter grüner nativer CI39503396485 integriert. Kalenderfix mit 1.008/4.032 Vektoren und drei echten TLS-PG-Fällen unabhängig geprüft; konkreter NIOssl-Pin ausdrücklich akzeptiert. Leadintegration Swift-/Kalender-/13Pins-/Originalcheckout-/Docs74/0 bestanden, sechs Livetests im Leadlauf ausdrücklich übersprungen. Kombinierte Apple-Main-CI39505094304 an 7e97aae erfolgreich. Unabhängiger A2-Review `0d96b86` als `4047180` aufgenommen; neuer Mainstand ändert nur Review/Status/Plan, Docs75/0 und Diffcheck bestanden. |
| F4b / Frontend & UX (`p1a_parity`) | Hauptrepo, `feat/row-web-runtime`, `.worktrees/row-web-runtime`, Basis `4044341` | Freigegebene F4-Quelle reproduzierbar im Rootprodukt einbetten. Besitz Assets/Manifest/Lock, Build-/Browser-/Deployskripte und enge Webhelper/CI, README/Einrichtung/Betrieb/Webbuild/Lizenzen/Handoff; kein DB/Core/HTTP/GoMod/API/STATUS/Plan. Produkt `3770c4e95ae4617a6b4858c87f1e69c963253416`, Handoff `46e4bf1`, unabhängiger Review `533348a72c5432c9f46b701dd70afcc4c1bdf944` ohne bestätigte Findings. Zwei bytegleiche öffentliche Builds, Embeddedbrowser9/9, vollständige Go-Race/Vet, neun Pythonprüfungen, echte Compose-/isolierte-Swarm-CRUD/409/DefaultNULLPrecisionBinary mit Persistenz und sauberer öffentlicher Go-/Imagecheckout ohne Node/npm/Siblings unabhängig bestanden. Alle sieben exakten CIs grün; Lead hat Agentintegration freigegeben, noch nicht integriert. |
| S2a / Apple & Security (`p3a_security`) | Apple, `feat/vault-core`, `.worktrees/vault-core`, Basis `7cbeb38` | Ausschließlich eigenständiges neues Paket Packages/DBManagerVault samt Prüfungen/Originaltexten, eigenen CI/Skripten, ADR-A003/Sicherheitsdokumenten/Handoff. Actor-Session, sichere lokale atomare Dateiablage, extern verankerte CAS-/Lebenszeit-AEAD-Reservierung und Recovery mit neuen Faktoren. Keine App-/Xcode-/Rootpackage-/A2-/zentralen Statusänderungen. Produktiver nicht zurückrollbarer Anchor bleibt gesondertes Gate; nur Testanker noch keine Produktfreigabe. |
| B3a / Backend & Platform (`p2a_drivers`) | Hauptrepo, `feat/schema-metadata`, `.worktrees/schema-metadata`, Basis `5cb604a` | Eingefrorener öffentlicher Prüfkandidat `77eccb7add990ad65fc6149103c548a40e9b7f5f`: lesende additive API0.5 für Index-/Constraintmetadaten; v1–v4 unverändert, keine DDL/SQL-Ausdruckstexte, explizite unknown-/CHECK-Lücken. Neue Core-/DB-/HTTPdateien, enge Route/Authorizer, eigene Tests/CI/Vertrag/Doku; zusätzlich exklusiv Root-STATUS/Plan. Vollständiges Go-Race/Vet, echte MySQL/MariaDB/PG-TLS-Proben, SQLite/HTTP/Auth/Limit-/Authorizerregressionen und Docs270/0 grün. Alle acht exakten nativen CIs grün; unabhängiger Peerreview durch Architecture & Security und 0.5-Freeze offen. |

Für dieses verlängerte Fenster hat der Lead ausschließlich die beiden zentralen
STATUS-/ARBEITSPLAN-Dateien des Hauptrepos an Backend & Platform zur Pflege übertragen;
keine parallelen Writer. Nach dem F4b-Reviewcheckpoint wird dieser Besitz für genau
den freigegebenen F4b-Integrationscheckpoint vorübergehend an Frontend & UX
übertragen; Backend & Platform schreibt bis erneuter Zuweisung nicht parallel.
Apple-STATUS/ARBEITSPLAN liegen exklusiv bei Architecture & Security.
Übrige zentrale Dateien benötigen eine gezielte Zuweisung. Jeder Writer hat
eigenen Worktree/Branch; unabhängiger Review vor Integration. Laufzeit weiterhin vier
Plätze einschließlich Lead, nachträglich konfigurierte höhere Limits hier nicht wirksam.

## Grenzen und Fortsetzung

Keine vollständige phpMyAdmin-Parität: [Referenzmatrix](PARITAET.md) enthält Ziele,
keinen pauschalen Implementierungsnachweis. Keine produktive Tresor-/Passkey-/CloudKit-/
Teamfreigabe, keine nativen MySQL-/MariaDB-Adapter wegen belegtem TLS-Downgrade.
Keine echten Geräte, Signing-/Sandboxdurchsetzung oder interaktive OS-UI abgenommen.
SQLite nach ADR0004: SingleReplica, exklusives lokales Volume, eine aktive Verbindung,
DELETE-Journal; kein WAL/NFS/fremder SQLiteprozess/HA-/Powerlossversprechen.
Abbruch bedeutet keine garantierte Rücknahme, unbekannter Commit kein automatischer
Retry; Trigger-/externe Nebenwirkungen, ABA und Exactly-once bleiben begrenzt.

Drei öffentliche Repos `db-manager`, `db-manager-web`, `db-manager-apple` unter
https://github.com/MagicalWig34653, main mit origin-Tracking. Git funktioniert;
Go1.27.1, Node26.5.0/npm11.17.0, Swift6.4/Xcode27.1 und Docker/Colima verfügbar.
Nächste konkrete Schritte: neue Go-/Apple-Haupt-CIs kontrollieren,
kombinierte F4b-Main-CIs kontrollieren; B3a bleibt bis unabhängigem P3-Review und separatem0.5-Freeze unintegriert. [Arbeitsplan](ARBEITSPLAN.md),
[Team](TEAM.md), [Einrichtung](../EINRICHTUNG.md).

## Paketcheckpoint 23.10.2026 – B3a eingefroren

B3a-Vertrags-SHA256 `b5225339511d80fe4f52f09118fdad2060d8a92f31d44b73dd89fe9f76178c95`,
öffentlicher Produktcommit `77eccb7add990ad65fc6149103c548a40e9b7f5f`. Keine
Integration oder 0.5-Clientfreigabe vor unabhängiger QA und exakter grüner CI.
Gestartete Jobs: Struktur39901687640, Engine39901687425, Backend39901687412,
Tabellen39901687408, Mutationen39901687543, Insert39901687391, Web39901687396,
Docs39901687478. Lokale eigene B3a-Container/Volumes/Prüfjobs sind entfernt/beendet.
[Öffentlicher Handoff](https://github.com/MagicalWig34653/db-manager/blob/77eccb7add990ad65fc6149103c548a40e9b7f5f/docs/handoffs/B3a.md).

Web-main `bf75748df9eb4422b084c20784d09ce931301d37`: integrierte 26 Units,
Typecheck, vier Verträge/Standalone, Build und Docs112/0 bestanden. Die neuen
Mainjobs39901222539/39901222403 und Docs39901222564 sind inzwischen erfolgreich. S2a ist inzwischen separat fachlich angenommen, Appleintegration und realer Produktanker bleiben eigene Gates;
aus dem dokumentarischen Apple-Mainupdate folgt keine Tresor-Coreintegration.

## Unabhängiger F4b-Abschlusscheckpoint

Review `533348a72c5432c9f46b701dd70afcc4c1bdf944` gibt ausschließlich das unveränderte
F4b-Paket zur Integration frei. [Öffentlicher Review](https://github.com/MagicalWig34653/db-manager/blob/533348a72c5432c9f46b701dd70afcc4c1bdf944/docs/handoffs/F4b-backend-review.md).
Produkt3770c4e, Handoff46e4bf1 und Review sind getrennte Provenienz. Exakte CIs
Docs39901535040, Web39901535086, Backend39901535009, Engine39901535020,
Tabellen39901535055, Mutationen39901535031 und Insert39901535030 alle success.
Eigene lokale Prüfjobs/Ressourcen entfernt, Hostswarm inactive, Fremdcontainer unverändert.

Auch alle acht oben genannten B3a-CIs an77eccb7 sind inzwischen success;
B3a bleibt vor unabhängigem Review und separatem0.5-Freeze unintegriert.
Kein S2a-/Produktanker-/Tresorfreigabeschluss. Nächster Schritt ist ausschließlich
die durch Lead freigegebene F4b-Agentintegration samt Gesamtcheck und MainCI.

## F4b-Agentintegration – 23.10.2026

Die konkrete Leadfreigabe wurde in eigenem `release/f4b`-Worktree ausgeführt:
Produktmerge `ffcf5195c79973a3b90a76fc79b5d82183a13453`, unabhängiger Reviewmerge
`2ad78b2fe604d40d69b54fdfa552d693644c1a5d`. Quelle3770c4e, Handoff46e4bf1 und
Review533348a bleiben in main-Ancestry. Webpin unverändert
`3a4a3fd0a345f8c127758ac5eadc4e6feb8d21ec`; Manifest, fünf Assets, Dockerprovenienz
und Lock bytegleich zum unabhängig geprüften Produkt. Kein Neubau/Produktbytefix.

Kombinierte lokale Integration: `go test -race ./...`, `go vet ./...`, Offline-
Manifestprüfung, neun Python-Buildtests, v2-/v3-/v4-Vertragsprüfungen und Dokumentation
259/0 bestanden. Kein API0.5-Vertrag im integrierten Baum; B3a wird nicht mitgenommen.
Web-mainbf75748 hat inzwischen alle drei neuen Main-CIs erfolgreich abgeschlossen:
Frontend39901222539, echteIntegration39901222403, Dokumentation39901222564.
Die sieben neuen kombinierten Root-Main-CIs werden mit diesem Checkpoint gestartet;
Ergebnis/IDs folgen als reiner Dokumentationsnachtrag. Noch keine Behauptung, diese
neue Mainserie sei bereits abgeschlossen.

Für diesen Checkpoint besitzt Frontend & UX exklusiv RootSTATUS/ARBEITSPLAN; P2
schreibt nicht parallel. Frist weiterhin heute23.10.2026 10:00Europe/Berlin=08:00UTC.
B3a bleibt trotz acht grüner Kandidaten-CIs vor unabhängigem P3-Review und gesonderter
Freigabe unintegriert; kein0.5-Verbrauch. S2a ist separat fachlich angenommen;
Appleintegration übernimmt P3, daraus folgt keine produktive Tresor-/Anchorfreigabe.
Keine eigenen laufenden Integrations-Testjobs, keine Containeränderung in diesem
Mergecheck. Nächster konkreter Schritt: kombinierte Main-CIs beobachten und Lead
melden; danach nur gezielt beauftragtes Folgepaket.
