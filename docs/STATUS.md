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

## Veröffentlichung und Abnahme P0

Unabhängiger Veröffentlichungsreview abgeschlossen: keine offenen bestätigten Befunde,
Ergebnis im [Handoff](handoffs/P0-review.md). Ausschließlich 70 Hauptdateien und
je 19 Clientdateien aufgenommen; lokale Zusatzartefakte ausgeschlossen.

Alle drei Repos wurden öffentlich erstellt und `main` nach `origin` gepusht.
`gh repo view` bestätigt jeweils `PUBLIC` und Standardbranch `main`.
`git ls-remote origin refs/heads/main` stimmt jeweils mit dem lokalen Anfangscommit überein:

| Repo | Anfangscommit | Erfolgreiche GitHub-Dokumentations-CI |
| --- | --- | --- |
| [db-manager](https://github.com/MagicalWig34653/db-manager) | `5bf98b0` | [Lauf 39484043273](https://github.com/MagicalWig34653/db-manager/actions/runs/39484043273) |
| [db-manager-web](https://github.com/MagicalWig34653/db-manager-web) | `89e7fcf` | [Lauf 39484060992](https://github.com/MagicalWig34653/db-manager-web/actions/runs/39484060992) |
| [db-manager-apple](https://github.com/MagicalWig34653/db-manager-apple) | `f2344f0` | [Lauf 39484079203](https://github.com/MagicalWig34653/db-manager-apple/actions/runs/39484079203) |

Zusätzlich alle drei öffentlichen Repos mit `git clone --depth=1` in temporäre
Verzeichnisse geklont und dort `python3 scripts/check-docs.py` ausgeführt:
70/19/19 Dateien, jeweils 0 Fehler. Temporäre Checkouts entfernt.
Damit sind tatsächliche Installation, Anfangscommits, Veröffentlichung und Push-CI
für P0 nachgewiesen. Der Pull-Request-Pfad der CI wurde weiterhin nur statisch geprüft.
Die Abschlussdokumentation folgt in eigenen Commits; Anfangscommits oben bleiben Referenz.

## Grenzen

Kein freigegebener API-Vertrag, keine App, keine Produktabhängigkeiten oder Builds.
Keine Treiber-/Gerätetests, tatsächlichen iCloud-/Passkey-Prüfungen oder Migrationen.
Keine Produktdienste oder Hintergrundjobs gestartet; keine unintegrierte Workerarbeit.
Der Lead pflegt zentrale Dokumente und installierte Clientstatusdateien;
der unabhängige Reviewer arbeitete ausschließlich lesend.

## Frühere Umgebungsblockade

In der vorbereitenden Sitzung scheiterten Git-Initialisierung und Clientinstallation
an Schreibrechten, GitHub an DNS-/Netzwerkfehlern. Diese Blockade ist bei der jetzigen
Installation und Anmeldung nicht mehr reproduzierbar. Frühere Fehlermeldungen belegen
keinen aktuellen Anmeldefehler. Veröffentlichung ist separat nachzuweisen.

## Fortsetzung

P0 abgeschlossen. Als Nächstes Paket P1a gemäß [Arbeitsplan](ARBEITSPLAN.md):
phpMyAdmin-Referenz mit Primärquellen und prüfbarer Paritätsmatrix erheben; DB-/OS-Ziele und weitere
Architekturentscheidungen bleiben bis zu bestätigten Befunden offen.

Grundlagen: [Einrichtung](../EINRICHTUNG.md), [Projektplan](PROJEKTPLAN.md),
[Projektübergabe](../PROJEKTUEBERGABE.md).
