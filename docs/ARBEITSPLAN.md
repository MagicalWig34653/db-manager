# Arbeitsplan

Stand: 20.10.2026. Lead: aktuelle koordinierende Session; kein dauerhafter Sessionbezug nötig.

| ID | Auftrag | Status | Abhängigkeiten / Abnahme |
| --- | --- | --- | --- |
| P0 | Drei Repo-Gerüste, Lizenz, Prozess, Prüfung und Installation | abgeschlossen: installiert, veröffentlicht, Push-CI grün | Drei öffentliche Repos auf main; unabhängiger Review bestanden; Remote-/CI-Nachweise in STATUS |
| P1 | phpMyAdmin-Referenz und vollständige Paritätsmatrix | P1a geprüft und integriert; 181 Fälle offen in Umsetzung | P0, Recherche mit Quellen und überprüfbaren Kriterien |
| P2 | Mac-/iPad-/Go-Treiberuntersuchung | P2a integriert; SQLite-Sandbox P2b in Arbeit | erste DB-/OS-Ziele festlegen; echte Plattformbefunde |
| P3 | Tresor, Recovery und verschlüsselter Sync | P3a in Arbeit | getrennte Machbarkeitsbefunde und dokumentiertes Bedrohungsmodell |

Lead pflegt zentrale Dokumente. Unabhängiger Gerüst- und Veröffentlichungsreview
abgeschlossen; bestätigte Befunde geschlossen, Ergebnis unter
[Handoff P0](handoffs/P0-review.md). Aktive Worker gemäß nachfolgendem Team-/Paketstand; keine produktiven Migrationen.

## Nächste begrenzte Pakete

Seit 20.10.2026 19:36 UTC in getrennten Worktrees aktiv. Keine Produktimplementierung daraus ableiten.
Für schreibende Worker legt der Lead jeweils eigenen Branch und Worktree an.
Ein anderer Agent prüft den Diff vor Integration; zentrale Statusdateien bleiben beim Lead.

| Paket | Ziel und erlaubte Dateien | Abnahme und Abhängigkeiten |
| --- | --- | --- |
| P1a | Im Hauptrepo auf Branch `docs/p1a-paritaet`: `docs/PARITAET.md`, `docs/TEAM.md` und `docs/handoffs/P1a.md`; phpMyAdmin-Referenz mit Primärquellen und Datum festlegen, Funktionsinventar erheben | Referenzversion nachvollziehbar; jede Funktion mit Sollverhalten, DB-/Plattformbezug und prüfbarem Abnahmekriterium; Vollständigkeitsgrenzen benannt; keine Implementierung oder stillschweigende Priorisierung |
| P2a | Im Hauptrepo auf Branch `docs/p2a-treiberplan`: `docs/handoffs/P2a.md`; getrennten Go-/Apple-Prüfplan und offene DB-/OS-Ziele erfassen | Vor praktischer Ausführung erste DB-/OS-Ziele klären; TLS, Auth, Typen, Abbruch, Streaming, SSH, Lizenz und echtes iPad abdecken; keine Treiberauswahl ohne Befunde |
| P3a | Im Hauptrepo auf Branch `docs/p3a-bedrohungsmodell`: `docs/handoffs/P3a.md`; Vertrauensgrenzen und Untersuchungsfälle für Tresor, Recovery und Sync beschreiben | Anmeldung und Entsperren getrennt; Geräteaufnahme, Verlust, Offlinebetrieb, Konflikte und Rotation berücksichtigt; Passkey-/PRF-Annahmen ausdrücklich offen; keine echten Zugangsdaten oder Kryptografieimplementierung |

Gemeinsamer Vertragsstand: API0.1.0 ist im Architecture-Paket noch Kandidat. P1a ist in Prüfung;
P2a/P3a sind bereits parallel bearbeitet und werden unabhängig geprüft. Resultate sind zunächst Vorschläge,
Architekturentscheidungen folgen nach Review als ADR. Laufende taskeigene Prüfjobs stehen bei den jeweiligen Paketübergaben; keine produktiven Dienste gestartet.

## Aktive autonome Umsetzung

Zeitgrenze: 21.10.2026 04:00 Europe/Berlin (02:00 UTC). Lead prüft sie vor neuen Paketen.
P1a: Worker `p1a_parity`, Worktree `.worktrees/p1a`, Branch `docs/p1a-paritaet`.
P2a: Worker `p2a_drivers`, Worktree `.worktrees/p2a`, Branch `docs/p2a-treiberplan`.
P3a: Worker `p3a_security`, Worktree `.worktrees/p3a`, Branch `docs/p3a-bedrohungsmodell`.
Dateibesitz gemäß Aufträgen; P2a zusätzlich `spikes/drivers/`, P3a zusätzlich
`docs/security/` und `spikes/security/`. Zentrale Dokumente ausschließlich Lead.
Nach Fertigstellung gegenseitige unabhängige Reviews, dann Integration durch Lead.
Architecture-/Security-Agent übernimmt gemeinsame Verträge und Go-/Compose-/Swarm-Grundlagen.
Lead koordiniert ausschließlich und implementiert keinen Produktcode.
Docker Compose und Docker Swarm sind verbindliche Ziele gemäß ADR-0003.

## Teamorganisation nach Nutzerpräzisierung

Lead: Projektleitung, Priorisierung, Dateibesitz, unabhängige Reviews, Integration und
zentrale Statuspflege; keine eigene Implementierung. Produktarbeit ausschließlich Agents.

| Rolle | Besetzung / nächster Auftrag | Verantwortung |
| --- | --- | --- |
| Architecture & Security | `p3a_security`, P4a nach abgeschlossenem P3a | Verträge, Sicherheitsgrenzen, technische Entscheidungen, Infrastrukturgrundlage |
| Backend & Platform | `p2a_drivers`, P2a, anschließend Backendadapter | Go, DB-Adapter, Integrationsnachweise, Containerbetrieb |
| Frontend & UX | `p1a_parity`, P1a, anschließend Webclient | Parität, deutschsprachige Bedienabläufe, Svelte/shadcn-svelte, Frontendtests |
| Apple Engineer | `p3a_security` nach P4a, separate Tickets/Worktrees | Eigenständiger Swift-/SwiftUI-Kern, macOS/iPadOS, native Tests |
| QA & Release | `p0_publication_review`, unabhängige Reviews aktiv | Fremdreview, Abnahme-/Regressionstests, CI, Releasebefunde |

Maximal drei gleichzeitig arbeitende Worker. Feste fachliche Rollen werden
nacheinander eingeplant, ohne zusätzliche parallele Kapazität zu behaupten.
Tickets nennen Ziel/Nichtziele, Dateien, Branch/Worktree, Vertragsstand und Abnahme.
Kein Selbstaudit als Freigabe; P1a/P2a/P3a und P4a erhalten Fremdreview vor Integration.
P3a-Commit `d25d660` durch QA unabhängig geprüft und als `b836dbf` integriert;
keine produktive Tresorfreigabe. P1a inklusive Folgecommit `f2721e4` durch QA freigegeben und integriert. P2a inklusive Folgecommit `557efc9` nach geschlossenem QA-P2-01 freigegeben
und als `0017043`/`d3da4b7` integriert.
P4a im Worktree `.worktrees/p4a` / Branch `feat/p4a-foundation` wurde vom Lead an
Architecture & Security übergeben. Begonnene Dateien sind noch nicht integriert.
Letzter Lead-Test: Compose authentifizierte API/Healthcheck/nonroot/read-only bestanden;
isolierter Swarmtest scheiterte am Node-ID-Aufruf. Worker übernimmt Korrektur und
vollständigen Nachweis sowie Aufräumen ausschließlich taskeigener Ressourcen.

## Aktuelle Umsetzungstickets

- P2b: durch QA als Untersuchung geprüft und als `7d2ed07` integriert; Dateien:
  `spikes/sqlite-sandbox/`, `docs/handoffs/P2b.md` und ADR-0004-Vorschlag.
  Eingeschränkte Wiederverwendung gemäß ADR0004 akzeptiert; keine umfassende Sandbox-/Durabilitätsfreigabe.
- P4a: abgenommen und integriert als `bb37904`/`80f203b`; API0.1.0 freigegeben.
  Compose und isolierter Swarm durch Worker und QA tatsächlich grün.
- F1: Frontend & UX im Webrepo-Worktree `.worktrees/web-workbench`,
  Branch `feat/web-workbench`. API0.1.0 ist inzwischen freigegeben und die exakte Bindung erlaubt. Keine Credentialpersistenz.
- A1: anschließend Architecture-/Apple-Engineer im Apple-Worktree
  `.worktrees/apple-workbench`, Branch `feat/apple-workbench`: eigenständiger
  lokaler SQLite-/SwiftUI-Arbeitsplatz; Netzwerkadapter erst nach Sicherheitsnachweis.

## B1 und aktuelle Integrationsreihenfolge

Backend & Platform implementiert B1 im Worktree `.worktrees/backend-engine`,
Branch `feat/backend-engine`, Ausgangspunkt `7d2ed07`. Besitz: `internal/database/`,
`internal/sqlitesandbox/`, Go-Moduldateien, begrenzte main-/Dockerverdrahtung, eigene
Engineprüfskripte/CI, Lizenzinventar, Betrieb und Handoff B1. Kein HTTP-/Core-/Schema-
oder Webcode ohne vorherige Abstimmung. ADR0004-Grenzen und eigene B1-Nachweise
verpflichtend. Frontend F1 und Apple A1 laufen parallel in ihren Clientworktrees.
QA prüft jeweils nach fertig gemeldeten Commits; bei Slotknappheit Implementierer idle.
Die Backend-/Container-CI am Hauptstand `9f1880f` ist tatsächlich erfolgreich:
[GitHub-Lauf](https://github.com/MagicalWig34653/db-manager/actions/runs/39487622613).
