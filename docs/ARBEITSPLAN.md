# Arbeitsplan

Stand: 20.10.2026, autonome Umsetzung. Zeitgrenze 21.10.2026 04:00 Europe/Berlin
(02:00 UTC) oder vorher vollständiger Projektabschluss. [Team und Gates](TEAM.md).
Lead: ausschließlich Projektleitung, Koordination, Revieworganisation, Integration
und zentrale Statuspflege. Produktcode schreiben die Agents.

## Abgenommene Grundlagen

| Paket | Ergebnis | Integration / unabhängiger Review |
| --- | --- | --- |
| P0 | Drei öffentliche Repos, AGPL-3.0-only, Prozess und Dokumentations-CI | abgeschlossen, [Review](handoffs/P0-review.md) |
| P1a | Referenz phpMyAdmin5.2.3, 181 prüfbare Fälle, Teammodell | `83388e3`/`109cd3e`, QA und Backendpeer; [Handoff](handoffs/P1a.md) |
| P2a | Go-/Swift-/iPad-Simulator-Treiberspikes, TLS-Downgradebefund | `0017043`/`d3da4b7`, QA-P2-01 geschlossen; [Handoff](handoffs/P2a.md) |
| P3a | Bedrohungsmodell, Tresor-/Sync-Stufen, Sicherheitsprüffälle/Spikes | `b836dbf`, QA; keine produktive Tresorfreigabe; [Handoff](handoffs/P3a.md) |
| P4a | Authentifizierte Go-HTTP-Grundlage, API0.1.0, Docker Compose und Swarm | `bb37904`/`80f203b`, QA-P4-01 geschlossen, unabhängige reale Deploytests; [Handoff](handoffs/P4a.md) |
| P2b | ncruces/Authorizer/Root-VFS-Untersuchung | `7d2ed07`, unabhängige Mac-/Linuxarm64-Suite, ADR0004 nur eingeschränkt für B1 akzeptiert; [Handoff](handoffs/P2b.md) |

## Aktive Tickets und Dateibesitz

Alle schreibenden Worker haben eigene Branches und Worktrees. Kein paralleles
Schreiben in denselben Dateien; gemeinsame Verträge vor abhängiger Arbeit einfrieren.
Maximal drei aktive Worker, QA wird nach fertig gemeldeten Paketen eingeplant.

| Ticket / Rolle | Repo / Branch / Worktree | Ziel, Besitz und Abnahme |
| --- | --- | --- |
| B1 – Backend & Platform (`p2a_drivers`) | Hauptrepo, `feat/backend-engine`, `.worktrees/backend-engine` | Reale SQLite-/MySQL-/MariaDB-/PostgreSQL-Engine. Besitz `internal/database/`, `internal/sqlitesandbox/`, Modul-/Lizenzdateien, begrenzte main-/Dockerverdrahtung, eigene Engineprüfungen/CI, Betrieb und Handoff. Kein HTTP/Core/Schema/Web ohne Abstimmung. Echte DB-Tests, Cancel/Lifecycle, sichere Fehler, 1MiB Rohzelle/8MiB tatsächliches JSON, Docker-UID/Persistenz sowie nativer Linuxamd64-Lauf sind Gates. ADR0004-Grenzen verpflichtend. |
| F1 – Frontend & UX (`p1a_parity`) | Webrepo, `feat/web-workbench`, `.worktrees/web-workbench` | Deutsche Svelte/shadcn-svelte-Workbench mit Connection/Catalog/SQL/Ergebnissen. Besitz Produkt-/Build-/Testdateien, eigenes Lizenzinventar, Einrichtung, Handoff. Exakte freigegebene API binden; Credentials nur im Speicher; Unit-/Komponenten-/Browserprüfungen und sauberer unabhängiger Build. Echte B1-Integration folgt separat. |
| A1 – Apple Engineer (`p3a_security`, Doppelrolle Architecture/Security) | Applerepo, `feat/apple-workbench`, `.worktrees/apple-workbench` | Eigenständiger SystemSQLite3-/SwiftUI-Arbeitsplatz auf macOS/iPadOS ohne Go. Besitz Swift-Kern, UI, Tests/Projekt/CI, Einrichtung/Lizenzen/Handoff. Mac-Tests und sauberer Mac-/iPad-Simulatorbuild, Grenzen/Cancel/Dateizugriff und tatsächlicher UI-Smoke. Kein echtes iPad oder produktiver Tresor behaupten. |

Alle Tickets: technische Bezeichner/Code/Kommentare/Tests Englisch, Dokumentation/UI
Deutsch. Keine echten Zugangsdaten, keine privaten Pfade oder produktiven SQL-Daten.
Keine fremden Docker-/Simulatorressourcen verändern. Worker dokumentieren genaue
Prüfbefehle, Grenzen, laufende Jobs und nächste Schritte im jeweiligen Handoff.
Zentrale STATUS-/ARBEITSPLAN-Dateien aller Repos gehören dem Lead.

## Gemeinsamer Vertrag und Betriebsentscheidungen

API0.1.0 ist nach unabhängigem QA-Review ausdrücklich als Clientvertrag freigegeben.
Herkunft `80f203b3e0c7fa3f7f4aa5ec06825cb83bc0c5d0`, SHA256
`4b598dc69a1904685e67a717997412654440ac10f47c17f7a025ccd114e294be`.
Clients pinnen Kopie und Provenienz; keine zufällige Geschwisterrepo-Abhängigkeit.
Das bestehende Foundation-Backend meldet bis B1 bewusst eine leere Treiberliste.

ADR0003: Compose und Swarm verbindlich. ADR0004: SQLite zunächst SingleReplica,
exklusives lokales Volume, eine aktive Verbindung jeDB, DELETE-Journal; kein WAL,
keine fremden SQLite-Prozesse. Kein HA-/Powerloss-/umfassendes Sandboxversprechen.
Native MySQLNIO-Netzwerkadapter bleiben wegen realem TLS-Downgrade gesperrt.

## Review- und Integrationsqueue

QA & Release (`p0_publication_review`) prüft fertig gemeldete Commits unabhängig;
Implementierer gehen bei benötigtem Slot vorübergehend idle. Findings zurück an den
Eigentümer, eng nachprüfen, erst danach Leadintegration und Gesamtprüfung.

1. B1, F1 und A1 jeweils getrennt abnehmen, nicht von den Spikefreigaben ableiten.
2. Durchgehenden echten Web→Go→Wegwerf-DB-Pfad prüfen; Webbuild reproduzierbar und
   mit Herkunft in das Go-/Containerprodukt einbinden. Compose und Swarm erneut mit
   tatsächlicher Produktengine testen.
3. Nativen eigenständigen SQLite-Pfad praktisch prüfen und Vertragswertfälle vergleichen.
4. Weitere Produktpakete aus Paritätsmatrix nach Nutzen/Abhängigkeit zuschneiden:
   Datenbearbeitung, Schemafunktionen, Import/Export, Verlauf/Favoriten und sichere
   Netzwerkadapter. Jeweils eigenes Ticket, Dateibesitz und Abnahme.
5. Produktiven Tresor, Recovery, Passkeys, CloudKit-Sync und Teams erst nach den
   gesonderten P3-Gates umsetzen/freigeben. Offene Betreiber-/Gerätegrenzen dokumentieren.

## Tatsächlicher Prüfstand und Übergabe

[STATUS](STATUS.md) ist kanonisch für tatsächliche Ergebnisse. Am Hauptstand `9f1880f`
sind [Backend-/Container-CI](https://github.com/MagicalWig34653/db-manager/actions/runs/39487622613)
und Dokumentations-CI erfolgreich. Neuere Commits separat verifizieren.
Keine produktiven Migrationen oder Dienste gestartet. Aktive Prüfjobs sind in den
Workerübergaben zu erfassen; Taskcontainer nach Tests entfernen, Images/Buildcaches
nicht mit fremden Ressourcen verwechseln. Alte saubere Worktrees bleiben Provenienz.
Vor der Zeitgrenze alle Worker geordnet checkpointen, laufende eigene Jobs erfassen,
prüfbare Commits sichern, offene Funktionen/Tests und nächsten Schritt dokumentieren.
