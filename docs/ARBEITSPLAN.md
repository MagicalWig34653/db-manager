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
| A2 – Apple Engineer (`p3a_security`) | Applerepo, `feat/apple-postgres`, `.worktrees/apple-postgres`, Basis `ce0aedb` | Direkte PostgreSQL-Verbindung ohne Go, TLS verpflichtend mit vollständiger Zertifikat-/Hostprüfung, flüchtige Credentials, Katalog/SQL und getaggte begrenzte Resultate. Besitz Produktmodule/SwiftUI, Package/Lock/Xcode/Entitlements, eigene Prüfungen/CI, Lizenzen/Originaltexte, README/Einrichtung/ADR/Handoff; keine zentralen Status-/Planungsdateien. SQLite erhalten. Echte TLS-Positiv-/Negativfälle, präzise Werte, tatsächlich beobachteter Abbruch und saubere Sessionwechsel; Mac-/iPad-Zielbuild, Gerätegrenzen offen. Kein MySQL/SSH/Vault/CloudKit/Teamumfang. |
| F3 – Frontend & UX (`p1a_parity`) | Webrepo, `feat/table-workbench`, `.worktrees/table-workbench`, Basis `ec8dcea` | Quellcode-/Lizenzverweise und strukturierter Tabellenarbeitsplatz mit freigegebenem gepinntem API0.2.0-Vertrag; Webprodukt/Build/Vertragskopien/Tests/Lizenzen/README/Einrichtung/Handoff. Keine zentralen Dateien, Mutation oder Credential-/SQL-Persistenz. Echte SQLite-/PostgreSQL-TLS-Browserproben und unveränderte strikte CSP/Sessiongrenzen. Root-Webpin folgt separat nach Webabnahme. |
| B2b – Backend & Platform (`p2a_drivers`) | Hauptrepo, `feat/table-edit`, `.worktrees/table-edit`, Basis `6009e1d` | Einzelzeilen-UPDATE/DELETE mit vollständigem typisiertem Erwartungssnapshot, DB-Sperre/Transaktion, expliziter Schreibbestätigung und ehrlichem unbekanntem Commitausgang. Nur bekannte transaktionale Basistabellen mit sicherem PK; Views/noPK/nullablePK/nichttransaktional konservativ sperren. Besitz neue DB/Core/HTTP-Mutationsdateien, enge SQLitehelper/Routingregistrierung, separater0.3-Kandidat, Prüfungen/CI/Doku/Handoff. Kein main/Docker/GoMod/Web und kein Insert/Batch/Schema; v1/v2 unverändert. Echte Paralleländerungs-/Abbruch-/Wert-/Injektions-/Commitfehlerproben und unabhängiger Vertragsfreeze vor Client. |

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
Die abgenommene B1-Engine meldet reale SQLite-/MySQL-/MariaDB-/PostgreSQL-Treiber.
API0.2.0 nach geschlossenem B2a-Review und erfolgreicher nativer Fix-CI für Clients
freigegeben: Herkunft `427444d7bcb8e30c9563ded2000ec3d5e19646c4`, Schema-SHA256
`c15a848f01bb779429fd68a2b6eb6bba1575d3e07a4c68e6796cf7840a661838`.
API0.1.0 und v1-Capabilities bleiben unverändert.

ADR0003: Compose und Swarm verbindlich. ADR0004: SQLite zunächst SingleReplica,
exklusives lokales Volume, eine aktive Verbindung jeDB, DELETE-Journal; kein WAL,
keine fremden SQLite-Prozesse. Kein HA-/Powerloss-/umfassendes Sandboxversprechen.
Native MySQLNIO-Netzwerkadapter bleiben wegen realem TLS-Downgrade gesperrt.

## Review- und Integrationsqueue

QA & Release (`p0_publication_review`) prüft fertig gemeldete Commits unabhängig;
Implementierer gehen bei benötigtem Slot vorübergehend idle. Findings zurück an den
Eigentümer, eng nachprüfen, erst danach Leadintegration und Gesamtprüfung.

1. B1, F1/F1b, A1 und F2a sind unabhängig abgenommen und integriert; jeweilige
   Grenzen gelten weiter, insbesondere keine vollständige Parität oder Tresorfreigabe.
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

[STATUS](STATUS.md) ist kanonisch für tatsächliche Ergebnisse. Am kombinierten
Hauptstand `6009e1d` sind Backend-/Container-, Engine-, Webpfad-, Tabellen- und
Dokumentations-CI erfolgreich; Nachweise dort verlinkt. Neuere Produktcommits separat
verifizieren. Reine Planungsupdates lösen keine unnötige Vollwiederholung aus.
Keine produktiven Migrationen oder Dienste gestartet. Aktive Prüfjobs sind in den
Workerübergaben zu erfassen; Taskcontainer nach Tests entfernen, Images/Buildcaches
nicht mit fremden Ressourcen verwechseln. Alte saubere Worktrees bleiben Provenienz.
Vor der Zeitgrenze alle Worker geordnet checkpointen, laufende eigene Jobs erfassen,
prüfbare Commits sichern, offene Funktionen/Tests und nächsten Schritt dokumentieren.

## Aktueller Review-/Clientcheckpoint

- A1 nach QA inklusive UI-Lifecyclefix und erfolgreicher Feature-/Main-CI im Apple-main
  integriert, Statuscommit `ce0aedb`. Echte Geräte/Signing/interaktive OS-UI bleiben offen.
- F1/F1b nach QA inklusive Session-/Zeilenzahlfix und CSP-tauglichen Validatoren
  freigegeben. Quelle `4e5abee` unverändert in Web-main, Statuscommit `ec8dcea`;
  öffentliche Frontend-/Dokumentations-CI erfolgreich.
- B1: alle vier Befunde im Fix `4ecd15f` unabhängig geschlossen; integrierte
  Go-Race/Vet-Prüfung und native öffentliche CIs erfolgreich. Produktcode im Hauptrepo
  `87caed8`/`063209e`, Handoff `4056c99`, zentraler Abnahmecheckpoint `520ca42`.
- F2a `42a0dec` plus `d236076` nach unabhängiger öffentlicher Reproduktion als
  `15b339e`/`c9cac01` integriert. Assetbytes/Manifest/Notices exakt; strikte CSP,
  MIME/Cache/Traversal/API-Passthrough geprüft. Noch keine Serververdrahtung.
- S1 nach unabhängiger QA als Formatgrundlage integriert (`7969f63`/`bc5dd4c`),
  ADR0005 akzeptiert; weiterhin keine produktive Credentialpersistenz.
  B2b-Mutation folgt separat
  mit Konflikt-/Transaktionsabnahme; kein vorgezogener Verbraucher des B2a-Kandidaten.

Die aktive Sitzung bietet vier gleichzeitige Plätze einschließlich Lead. Die lokale
nachträgliche Konfiguration höherer Limits wird von den verfügbaren Laufzeitwerkzeugen
nicht neu geladen. Kein Neustart oder Umgehungsprozess; drei Implementierer und
unabhängige QA wechseln sich geordnet ab.

## Folgepakete und Voraussetzungen

| Paket | Voraussetzung | Geplanter begrenzter Umfang und Abnahme |
| --- | --- | --- |
| F3 – Tabellenarbeitsplatz (beauftragt, siehe oben) | B2a-Vertrag unabhängig freigegeben; F2b integriert | Gepinnter v2-Client, strukturierte Metadaten/Tabellenansicht mit Seitennavigation, Sortierung und parametrisierten Filtern; eindeutige Anzeige instabiler Reihenfolge, unbekannter Eigenschaften und fehlender Schreibfunktion. Echte Browserproben gegen SQLite und PostgreSQL; keine Mutation oder heimliche SQL-Fragmentbildung. Neue Quellcode-/Lizenzverweise im Webclient und anschließend neu reproduzierter Go-Webpin. |
| B2b – Datenbearbeitung (beauftragt, siehe oben) | B2a abgenommen; eigener neuer Vertragskandidat | Zunächst klar begrenzte einzelne Zeilen mit sicherem Primärschlüssel; validierte Metadaten, parametrisierte Werte, Transaktions-/Konfliktregeln und ausdrückliche Behandlung unsicherer Tabellen. Kein Erfolg bei unbekanntem Commitausgang; konkurrierende Änderungen und Abbruch praktisch prüfen. Neuer Vertrag vor Frontendverbrauch separat abnehmen. |
| A2 – Native Netzwerkverbindung (beauftragt, siehe oben) | Geprüfter P2a-Treiberpfad | Zunächst PostgreSQL direkt ohne Go, flüchtige Zugangsdaten, verpflichtend verifiziertes TLS und unverfälschte Werttypen; Verbindungswechsel/Abbruch/Fehlerfreigabe sowie lokaler SQLitebetrieb erhalten. Keine native MySQL-/MariaDB-Freigabe aus dem bestehenden Downgradebefund ableiten. Mac-/iPad-Zielbuild und echte synthetische TLS-Datenbankproben; Gerätegrenzen ausdrücklich offen. |
| S2 – Optionale Tresorpersistenz | S1-Format/ADR unabhängig abgenommen; getrenntes Produktdesign | Atomare Speicherung, Recovery-Rückleseablauf, gesperrte/entsperrte Zustände, Dateischutz, Aktualisierungs-/Rotationszähler und Abbruch-/Crashverhalten. Produktplattform und Dateibesitz vor Beginn festlegen; keine Speicherung echter Geheimnisse allein aufgrund erfolgreicher Interopproben. CloudKit/Passkeys/Teams bleiben eigene Pakete. |

Diese Tabelle bereitet Folgearbeit vor und ist keine Implementierung oder Freigabe.
Konkrete Branches, Dateibesitz und Reviewer werden erst bei Beauftragung zugewiesen.

B2a `62f1297`/`573e392`/`3b5a54c` unabhängig abgenommen und als
`cb07ebb`/`19673b3`/`427444d` integriert; beide Befunde geschlossen, vier Fix-CIs
erfolgreich. Engine nach einmaligem DockerHub-HTTP502-Jobneustart erfolgreich.
F2b `f2c92ee`/`c9ced80` unabhängig abgenommen und als `df0b291`/`c7e1a1d` integriert.
S1-Originalworktree bleibt sauber, keine dortigen Jobs. QA hat den Formatreview beendet.
