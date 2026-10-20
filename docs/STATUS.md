# Tatsächlicher Projektstand

Stand: 20.10.2026.

## Vorhanden

Hauptprojekt mit Dokumentation, vollständigem AGPLv3-Lizenztext und eindeutiger
`AGPL-3.0-only`-Festlegung, Agent-Einstiegen, Konzept, Architekturentwurf, Etappen,
Paritätsvorbereitung und zwei ADRs. Sicherheits-/Beitragshinweise, Issue-/PR-Vorlagen
und Dokumentations-CI sind vorhanden. Keine Produktimplementierung.

Drei eigenständige lokale Git-Repositories sind auf `main` initialisiert:
`GoLand/db-manager`, `PhpstormProjects/db-manager-web` und `Xcode/db-manager-apple`,
jeweils relativ zum Projektworkspace. Die 38 Clientdateien wurden aus
`bootstrap/repos/` installiert. Diese Vorlagen bleiben historische Installationsvorlagen;
weitere Clientarbeit erfolgt in den Zielrepos.

## Tatsächlich geprüft

Am 20.10.2026 in dieser Sitzung:

- Git 2.54.0 verfügbar; temporäre Initialisierung erfolgreich und Testverzeichnis entfernt.
- `python3 scripts/bootstrap-repos.py --workspace '<workspace>'`: Plan für drei Repos,
  keine Konflikte mit vorhandenen Clientdateien.
- Derselbe Befehl mit `--apply`: drei Repositories tatsächlich initialisiert,
  38 Clientdateien installiert; alle drei Dokumentationsprüfungen bestanden.
- `python3 scripts/check-docs.py`: Hauptprojekt sowie beide installierten Clients
  bestanden; zusätzlich beide Clientvorlagen bestanden (jeweils 0 Fehler).
- `gh auth status` und `gh api user --jq .login`: aktives Konto `MagicalWig34653`
  bestätigt. Bestehende Git-Identität verwendet GitHub-noreply-Adresse; unverändert.
- Abfrage aller drei vorgesehenen GitHub-Repos: vor Veröffentlichung nicht vorhanden.
- Vorheriger unabhängiger [Gerüstreview](handoffs/P0-review.md): bestätigte Befunde geschlossen;
  Installations-/Commitablauf und Wiederholung in isolierten Kopien geprüft.

## Noch offen

Aktueller Checkpoint: noch keine Commits, Remotes, Pushes oder GitHub-CI-Läufe.
Unabhängiger lesender Review des tatsächlichen Veröffentlichungsstands läuft.
Anschließend Anfangscommits erstellen, öffentliche Repositories unter
`MagicalWig34653` anlegen, pushen und Remote-/CI-Ergebnisse hier dokumentieren.

Kein freigegebener API-Vertrag, keine App, keine Produktabhängigkeiten oder Builds.
Keine Treiber-/Gerätetests, tatsächlichen iCloud-/Passkey-Prüfungen oder Migrationen.
Keine Produktdienste oder Hintergrundjobs gestartet; keine unintegrierte Workerarbeit.
Der Lead besitzt Status, Arbeitsplan, Einrichtung, Repositoryübersicht, Projektplan und
installierte Clientstatusdateien; der Reviewer arbeitet ausschließlich lesend.

## Frühere Umgebungsblockade

In der vorbereitenden Sitzung scheiterten Git-Initialisierung und Clientinstallation
an Schreibrechten, GitHub an DNS-/Netzwerkfehlern. Diese Blockade ist bei der jetzigen
Installation und Anmeldung nicht mehr reproduzierbar. Frühere Fehlermeldungen belegen
keinen aktuellen Anmeldefehler. Veröffentlichung ist separat nachzuweisen.

## Fortsetzung

P0 mit Review, Anfangscommits, Veröffentlichung und tatsächlichen CI-Ergebnissen abschließen.
Danach P1/P2/P3 als begrenzte Pakete konkretisieren; DB-/OS-Ziele und weitere
Architekturentscheidungen bleiben bis zu bestätigten Befunden offen.

Grundlagen: [Einrichtung](../EINRICHTUNG.md), [Projektplan](PROJEKTPLAN.md),
[Projektübergabe](../PROJEKTUEBERGABE.md).
