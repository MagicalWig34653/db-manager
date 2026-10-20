# P0 – unabhängiger Planungsreview

Stand: 20.10.2026. Rolle: unabhängiger Prüfer; keine Implementierung übernommen.

## Umfang und Grenzen

Geprüft wurden [Projektplan](../PROJEKTPLAN.md), [Status](../STATUS.md),
[Projektübergabe](../../PROJEKTUEBERGABE.md) und die Agent-Einstiege sowie der
Entwicklungsablauf des Timetrack-Vorbilds. Der finale Bootstrap-Diff liegt bei diesem
Checkpoint noch nicht vor. Dies ist keine Abnahme angelegter Repositories, CI oder
Produktfunktionalität. Nach Bereitstellung des Gerüsts ist ein eigener Review erforderlich.

## Ergebnis des Plans

Die vorgeschlagenen drei Repositorygrenzen sind schlüssig. Zentrale versionierte Verträge
im Hauptrepo und eigenständig baubare Clients vermeiden eine unbeabsichtigte
Laufzeitabhängigkeit der Apple-App vom Go-Backend. Anforderungen, technische Vorschläge
und unbestätigte Entscheidungen sind ausreichend getrennt. P2/P3 prüfen wesentliche
Unsicherheiten vor produktiver Treiber-, Tresor- und Synchronisierungsimplementierung.

Der Plan berücksichtigt die erkannten Widersprüche des Vorbilds: ältere Codex-Quoten
gegenüber späterer Claude-only-Vorgabe sowie unterschiedlich strenge Worktree-Regeln.
Diese projektspezifischen Vorgaben werden nicht übernommen. Gemeinsame Regeln sollen
kanonisch bleiben; Claude-spezifische Dateien verweisen darauf.

## Empfehlungen für die Gerüstabnahme

- Jedes Repo enthält den vollständigen AGPLv3-Lizenztext und eine eindeutige
  `AGPL-3.0-only`-Kennzeichnung. Eigener Code und künftige Abhängigkeiten werden
  unterschieden; ein Lizenzinventar behauptet keine bereits geprüften Abhängigkeiten.
- README und Einrichtung nennen den tatsächlichen Gerüststand. Keine nicht vorhandenen
  Start-, Build-, Test- oder Produktfunktionen als ausführbar oder geprüft darstellen.
- `AGENTS.md`, `CLAUDE.md` und Startprompt führen eine neue Session zu Status, Arbeitsplan,
  maßgeblichen Anforderungen und dem nächsten begrenzten Auftrag. Keine Modellquoten,
  Pluginpflicht oder unbestätigte Claude-only-Vorgabe einschleusen.
- Gemeinsame Prozessregeln, Arbeitsauftrag und Dateibesitz sind eindeutig. Parallele
  Schreiber erhalten getrennte Branches/Worktrees; zentrale Statusdateien einen Besitzer.
- Hauptrepo und Clients kennzeichnen lokale und repoübergreifende Quellen. Vertragspinning
  und spätere Driftprüfung dürfen zunächst geplant sein, müssen dann ausdrücklich als
  noch nicht implementiert gelten. Platzhalter sind keine Kompatibilitätszusage.
- Dokumentationslinks und Dateipfade prüfen. Verweise auf noch nicht vorhandene Dokumente
  entweder vervollständigen oder klar als geplant kennzeichnen.
- Öffentlichen Diff auf Secrets, private absolute Pfade, Personenangaben, private
  Mailadressen, echte Zugangskonfiguration und übernommene Timetrack-Kennungen prüfen.
  Vorhandene öffentliche Git-Autoridentität prüfen, ohne sie ungefragt umzuschreiben.
- CI muss das tatsächliche Gerüst sinnvoll prüfen. Lokale Ausführung und noch nicht
  erfolgte GitHub-Ausführung getrennt dokumentieren; keine grüne CI ohne Lauf behaupten.
- Lokale Git-Initialisierung, Anfangscommit, Remote-Konfiguration und tatsächliche
  öffentliche Veröffentlichung jeweils separat nachweisen. Fehlende Zugriffsrechte
  blockieren nur den betroffenen Schritt, nicht die lokale Dokumentation.

## Prüfstand und nächster Schritt

Ausgeführt: lesender Planungsreview. Keine Produktbuilds oder Treibertests ausgeführt.
Keine abschließende Lizenztext-, Link-, Git-, Geheimnis- oder CI-Prüfung des Bootstrap-
Ergebnisses durchgeführt, weil dessen finaler Stand noch nicht vorliegt.

Nächster Schritt: finalen Bootstrap-Diff und dokumentierte Prüfungen unabhängig gegen
die obigen Kriterien prüfen; konkrete Befunde mit Datei, Auswirkung und Erledigungsstand
hier sichern. Architekturspikes bleiben nach P0 offen.

## Review des bereitgestellten Gerüsts

Nachfolgender Stand ersetzt die frühere Einschränkung zum noch nicht vorliegenden Gerüst.
Geprüft wurden die Einstiegstexte, Prozessregeln, Clientvorlagen, Einrichtung, Status,
Arbeitsplan, Repositoryzuordnung, Workflow und beide Python-Skripte. GitHub-Veröffentlichung
und reale Zielinstallation sind weiterhin nicht abgenommen.

Positiv: Claude-Einstieg verweist auf gemeinsame Regeln; keine Anbieterquote übernommen.
Lizenztexte und `AGPL-3.0-only`-Kennzeichnungen vorhanden. Installation lehnt abweichende
Zieldateien und Symlinkziele ab. Bestehende Git-Identität wird nicht umgeschrieben.
Workflow verlangt nur lesenden Repositoryzugriff. Dokumentationsprüfung umfasst relative
Links, erforderliche Dateien, Lizenzkennzeichnung und ausgewählte private Datenmarker.
Sie ersetzt keine vollständige Secretprüfung aller Dateiformate.

### Ausgeführte Prüfungen

Am 20.10.2026:

- `python3 scripts/check-docs.py`: 73 Dateien, 0 Fehler.
- `python3 bootstrap/repos/db-manager-web/scripts/check-docs.py`: 19 Dateien, 0 Fehler.
- `python3 bootstrap/repos/db-manager-apple/scripts/check-docs.py`: 19 Dateien, 0 Fehler.
- Isolierte temporäre Kopie des Hauptprojekts: `--apply` mit temporärem Workspace
  erfolgreich; drei Git-Repos initialisiert, alle Dokumentationsprüfungen erfolgreich.
  Danach `--apply --commit` fehlgeschlagen, bevor ein Commit angelegt wurde.
  Temporäre Kopie anschließend entfernt; reale Zielordner und Git-Konfiguration unverändert.

### Befunde

| ID | Befund und Auswirkung | Stand |
| --- | --- | --- |
| R1 | `scripts/bootstrap-repos.py` lehnt bereits installierte, noch nicht committete Gerüste als geänderte Repos ab. Der in `EINRICHTUNG.md` nahegelegte Ablauf aus erst `--apply`, später `--apply --commit --publish` funktioniert dadurch nicht ohne manuellen Eingriff. Reproduziert in isolierter Kopie. | Offen; Lead informiert |
| R2 | Zusätzliche vorhandene Zieldateien werden bei erstmaliger Installation nicht inventarisiert/abgelehnt. `git add -- .` kann diese beim anschließenden Commit übernehmen; Prüfung scannt private Datenmarker nur in Markdown. Fremde Dateien oder private Daten anderer Formate könnten damit veröffentlicht werden. Vorhandene Zusatzdateien ausdrücklich prüfen oder ablehnen und zu committen bestimmte Dateien begrenzen. | Offen; statisch bestätigt, Lead informiert |
| R3 | `docs/STATUS.md` beschreibt noch den reinen Planstand und behauptet, CI sei nicht angelegt. Workflow und Vorlagen sind inzwischen vorhanden; Vorbereitung, lokale Prüfung und tatsächliche GitHub-Ausführung getrennt erfassen. | Offen; Lead aktualisiert zentrale Dokumente |

Kein GitHub-Workflow ausgeführt. Die tatsächliche Erreichbarkeit des Event-Commits beim
Workflow-Fetch, insbesondere für Pull Requests aus Forks, ist nicht praktisch nachgewiesen;
daraus wird ohne Reproduktion kein bestätigter Fehler abgeleitet.

Abnahmeempfehlung: R1/R2 vor Verwendung des Veröffentlichungswegs lösen oder diesen
ausdrücklich durch einen geprüften manuellen Ablauf ersetzen; R3 vor Übergabe aktualisieren.
Danach Korrekturen erneut prüfen und den verbleibenden Zielpfad-/GitHub-Blocker ehrlich
als offen führen. Produkt-, Treiber-, Plattform- und Kryptografieprüfungen bleiben unberührt offen.

## Nachprüfung der Korrekturen

Stand: 20.10.2026, nach Einführung von `bootstrap/manifest.json`, expliziter Pfadauswahl
beim Staging, Schutz unbekannter Clientdateien und Behandlung noch nicht committeter Repos.
Dieser Abschnitt ist für die abschließende Befundbewertung maßgeblich.

- **R1 geschlossen:** Mit einer frischen isolierten Kopie des Hauptprojekts in einem
  temporären Workspace nacheinander `python3 scripts/bootstrap-repos.py --workspace
  '<temporary-workspace>' --apply`, danach denselben Befehl mit `--apply --commit` und
  anschließend nochmals mit `--apply --commit` ausgeführt. Alle drei Läufe Exit 0.
  Anschließend in allen drei temporären Repos `git status --porcelain` leer und
  `git rev-list --count HEAD` genau 1. Kein zusätzlicher Commit beim Wiederholungslauf.
- **R2 geschlossen:** Staging verwendet explizite Manifest-/Templatepfade. In einer
  weiteren frischen temporären Kopie vor Installation eine zusätzliche synthetische
  Binärdatei im Webziel angelegt. `--apply` verweigert Installation mit Exit 1 und
  `Unknown existing file`. Inhalt unverändert, kein Hauptrepo-Git angelegt und keine
  Client-README installiert. Das ist ein Schutz vor unbemerkter Übernahme fremder Dateien,
  kein allgemeiner Nachweis, dass sämtliche zulässigen Dateien frei von Secrets sind.
- **R3 geschlossen:** Hauptstatus beschreibt jetzt Gerüste, vorbereitete CI, tatsächliche
  Grenzen der Prüfungen und blockierte reale Zielinstallation/Veröffentlichung getrennt.
  Die isolierten Reviewer-Commits sind Testartefakte und keine Initialisierung der realen Repos.
- CI verwendet für Pull Requests nun ausdrücklich `refs/pull/<number>/merge`.
  Statisch nachvollziehbar; GitHub-Ausführung weiterhin nicht erfolgt.

Die temporären Git-Prozesse verwendeten ausschließlich pro Prozess gesetzte synthetische
Autor-/Committerdaten mit `example.invalid`; Signierung und Hooks waren ebenfalls nur für
die Testprozesse deaktiviert. Keine reale Git-Identität oder Konfiguration geändert.
Temporäre Workspaces nach den Assertions entfernt. Keine Netzwerk- oder Publishaktion.

Dokumentationsprüfungen erneut ausgeführt: Hauptprojekt 74 Dateien/0 Fehler, Webvorlage
19 Dateien/0 Fehler, Applevorlage 19 Dateien/0 Fehler. Der Handoff wurde danach um diese
Ergebnisse ergänzt; ein abschließender Dokumentationslauf folgt durch den Lead.

Ergebnis: Kein offener bestätigter Befund aus diesem Gerüstreview. Das vorbereitete Gerüst
und der lokal isoliert geprüfte Installations-/Commitablauf sind für P0 nachvollziehbar.
Reale Zielinstallation, tatsächliche öffentliche Git-Autoridentität, Remote-Erstellung,
Pushes und GitHub-CI bleiben ausdrücklich offen; P0 nicht als vollständig veröffentlicht
ausgeben. Produkt- und Plattformmachbarkeit wurden durch diesen Review nicht geprüft.

Nachtrag zur letzten Lead-Korrektur: Manifestvalidierung vor Mutation sowie allgemeiner
formulierter Clientstatus lesend geprüft. Danach beide isolierten Szenarien erneut gegen
diesen Stand ausgeführt: Erstinstallation, Commit und Wiederholung erfolgreich mit genau
einem Commit und sauberem Zustand je Repo; unbekannte Clientdatei vor Installation
abgelehnt. Haupt-Dokumentationsprüfung erneut 74 Dateien/0 Fehler vor diesem Nachtrag.


## Veröffentlichungsreview der tatsächlichen Installation

Stand: 20.10.2026. Unabhängiger lesender Reviewer; Ergebnis durch Lead festgehalten.
Geprüfter Umfang: 70 Manifestdateien im Hauptrepo und je 19 explizite Clientdateien
in den tatsächlich installierten Zielrepos. Keine offenen bestätigten Befunde;
Veröffentlichung dieses Umfangs freigegeben.

- Alle fünf `python3 <repo>/scripts/check-docs.py`-Läufe bestanden: Hauptprojekt,
  beide installierten Clients und beide Vorlagen, jeweils 0 Fehler. Dateizählungen
  enthalten lokale Zusatzartefakte und sind kein veröffentlichter Dateiumfang.
- Veröffentlichungspfade auf private Pfade, Schlüssel-/Tokenmarker, verdächtige
  Zugangsdatenzuweisungen und Mailadressen geprüft; keine ungeklärten Treffer.
  Scanner-Ausdrücke im Prüfcode als Fehlalarme verifiziert. Kein vollständiger Secret-Scanner.
- AGPL-Lizenztexte bytegleich, `AGPL-3.0-only` eindeutig; keine Produktabhängigkeiten.
- Bestehende Git-Autoridentität mit GitHub-noreply-Adresse unverändert.
- CI statisch mit ausschließlich `contents: read`, ohne Drittanbieteractions und
  getrennten Fetchpfaden für Push/PR geprüft; tatsächliche Ausführung noch offen.
- Status, Arbeitsplan, Repositoryübersicht, Einrichtung und installierte Clientstatus
  unterscheiden Installation und noch offene Veröffentlichung konsistent.
- Bestehende geschlossene Bootstrap-Befunde R1–R3 nachvollzogen.

Reviewer schrieb keine Dateien und führte keine Veröffentlichung aus. Lead übernimmt
nur explizite Manifest-/Templatepfade. Lokale Zusatzartefakte wie `.DS_Store` oder
Reviewdatenbanken bleiben ausgeschlossen. Nach Clientstatusänderungen erfolgt die
Veröffentlichung mit expliziten Git-/gh-Befehlen, nicht erneutem Bootstrap.

Abschließende Indexprüfung durch Lead: exakt 70/19/19 Pfade und
`git diff --cached --check` in allen drei Repos erfolgreich. Nächster Schritt:
Anfangscommits, öffentliche Remote-Erstellung, Push sowie tatsächlichen CI-Stand prüfen.
