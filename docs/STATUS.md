# Tatsächlicher Projektstand

Stand: 20.10.2026, 22:00 UTC. Autonome Umsetzung bis zum vollständigen Abschluss
oder spätestens 21.10.2026 04:00 Europe/Berlin (=02:00 UTC).
Lead koordiniert, organisiert unabhängige Reviews und integriert; Agents implementieren.
Docker Compose und Docker Swarm sind verbindlich (ADR-0003).

## Abgenommen und integriert

| Paket | Tatsächliches Ergebnis | Nachweis |
| --- | --- | --- |
| P0–P4a, P2b | Öffentliche Repos, Anforderungen/Paritätsreferenz, Treiber-/Securityspikes, HTTP/Auth-/Containergrundlage, eingeschränkte SQLite-VFS-Entscheidung | [P0](handoffs/P0-review.md), [P1a](handoffs/P1a.md), [P2a](handoffs/P2a.md), [P3a](handoffs/P3a.md), [P4a](handoffs/P4a.md), [P2b](handoffs/P2b.md) |
| B1 | Reale SQLite-/MySQL-/MariaDB-/PostgreSQL-Engine mit verifiziertem Netzwerk-TLS und begrenzten typisierten Ergebnissen; vier Reviewbefunde geschlossen | [B1](handoffs/B1.md) |
| A1 | Eigenständiger nativer SwiftUI-/SQLite-Arbeitsplatz ohne Go; zehn Tests und Mac-/iPad-Simulator-Build, Dateiwechselfix geprüft | [Applerepo](https://github.com/MagicalWig34653/db-manager-apple), dort Handoff A1 |
| F1/F1b, F2a/F2b | Deutscher Web-SQL-Arbeitsplatz, reproduzierbare Offlineassets, echtes Browser→Go→DB-Produkt; Session-/CSP-Befunde geschlossen | [F2a](handoffs/F2a.md), [F2b](handoffs/F2b.md) |
| B2a | Strukturierte Tabellenmetadaten, Sortierung, gebundene AND-Filter und Pagination; Pflichtfeld-/Kollationsbefunde geschlossen | [B2a](handoffs/B2a.md) |
| B2b | Bestätigtes Einzelzeilen-UPDATE/DELETE mit typisiertem Erwartungssnapshot, echten Locks/Transaktionen und ehrlichem unbekanntem Commitausgang | [B2b](handoffs/B2b.md) |
| B2b-Schutzfixes | SQLite OR ABORT verhindert fremde REPLACE-Löschungen; PostgreSQL-Rewrite-Tabellen werden gesperrt. Beide nachträglichen Integritätsbefunde unabhängig geschlossen | [SQLite](handoffs/B2b-konfliktstrategie.md), [PostgreSQL](handoffs/B2b-rewrite-regeln.md) |
| B2c | Bestätigtes einzelnes INSERT mit getrennten Default-/NULL-Werten, konservativer PK-Ermittlung und tatsächlicher Rücklese nach Triggern | [B2c](handoffs/B2c.md) |
| F3/F3b | Strukturierte Web-Tabellenoberfläche und Quellen-/Originallizenzdialog, exakt reproduziert im Go-/Dockerprodukt eingebettet. Filterzustandsbefund QA-F3-01 geschlossen | [F3b](handoffs/F3b.md), [unabhängiger Review](handoffs/F3b-backend-review.md) |
| S1 | Interoperables versioniertes Tresorformat mit Argon2id/AES-GCM/Recovery; ausschließlich Formatgrundlage, keine produktive Credentialpersistenz | [S1](handoffs/S1.md), [Format](security/VAULT-FORMAT-V1.md) |

F3-Webquelle `6c6eb33c93eed355a10cd114f61ce27be7f3f55e`, Web-main `ffdd2a0`.
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
- Integrierter F3b-Hauptstand `5cb604a`: Go-Race/Vet, Manifest mit fünf Originalassets, acht Python-Buildprüfungen, Insertvertrag und Dokumentation266/0 bestanden. Neue kombinierte Haupt-CIs nach Veröffentlichung noch abzuwarten.
- S1 unabhängig: vier Go-/vier Swift-Tests, Race/Vet, 24 positive und 232 negative wechselseitige CLI-Fälle, 14 Originalquellvergleiche und Lizenzen bestanden.
- Tests verwendeten ausschließlich eigene Wegwerfressourcen. Host-Swarm blieb inaktiv, fremde Container unverändert. Keine eigenen Leadprüfjobs offen; Workerjobs stehen in ihren Handoffs. Originale generierte JS-/Lizenzbytes bleiben unverändert.

## Laufende Arbeit und unabhängige Gates

| Paket / Rolle | Repo, Branch / Worktree | Tatsächlicher Stand und Dateibesitz |
| --- | --- | --- |
| A2 / Apple & Security | Apple, `feat/apple-postgres`, `.worktrees/apple-postgres` | Direkter PostgreSQL-Pfad ohne Go; Code `89b2683`/`abaa3dc`, Kalenderfix `b72e62c`. Unabhängiger Review einschließlich konkretem unveröffentlichtem NIOssl-Revisionspin bestanden; QA-A2-01 geschlossen. 1.008 Datums-/4.032 Timestampvektoren und drei echte TLS-PG-Kalenderfälle unabhängig geprüft. Vor Integration fehlt exakter erfolgreicher nativer CI39503396485; Docs-CI erfolgreich. Kein offener Produktbefund. |
| F4 / Frontend & UX (`p1a_parity`) | Web, `feat/row-workbench`, `.worktrees/row-workbench`, Basis `ffdd2a0` | Einzelzeilen-Insert/Edit/Delete, bestätigte Aktionen, frische Eignung und vollständige Snapshots, ehrlicher Konflikt-/Unknown-Ausgang. Besitz Webprodukt/Vertragskopien/Build/Tests/Lizenzen/Einrichtung/Handoff. Keine zentralen Statusdateien. v4-Fähigkeiten und v3-Eignung verpflichtend, Referenzbackend72855cf. Unabhängige QA und separater Root-Webpin folgen. |
| S2a / Apple & Security (`p3a_security`) | Apple, `feat/vault-core`, `.worktrees/vault-core`, Basis `7cbeb38` | Ausschließlich eigenständiges neues Paket Packages/DBManagerVault samt Prüfungen/Originaltexten, eigenen CI/Skripten, ADR-A003/Sicherheitsdokumenten/Handoff. Actor-Session, sichere lokale atomare Dateiablage, extern verankerte CAS-/Lebenszeit-AEAD-Reservierung und Recovery mit neuen Faktoren. Keine App-/Xcode-/Rootpackage-/A2-/zentralen Statusänderungen. Produktiver nicht zurückrollbarer Anchor bleibt gesondertes Gate; nur Testanker noch keine Produktfreigabe. |
| B3a / Backend & Platform | Hauptrepo, neuer eigener Worktree/Branch wird vorgeschlagen | Begrenzte lesende Index-/Constraintmetadaten vorbereiten. Erst Scope-/Vertragsentscheidung, keine DDL; noch kein Implementierungsnachweis. |

Zentrale STATUS-/ARBEITSPLAN-Dateien aller Repos gehören dem Lead. Jeder Writer hat
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
Nächste konkrete Schritte: neue Haupt-CIs kontrollieren, A2 nach exakter CI integrieren,
F4 und S2a unabhängig prüfen, B3a zuschneiden. [Arbeitsplan](ARBEITSPLAN.md),
[Team](TEAM.md), [Einrichtung](../EINRICHTUNG.md).
