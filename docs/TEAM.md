# Entwicklungsteam und Arbeitsmodell

Stand: 20.10.2026. Nutzerauftrag: professionelles Softwareentwicklungsteam mit Agents;
Lead ausschließlich Projektleitung und Koordination, Umsetzung durch beauftragte Engineers.
Dieser Teamplan weist tatsächliche Zuständigkeiten und bewusst besetzte Doppelrollen aus.

## Rollen und Besetzung

| Rolle | Zuständigkeit | Tatsächliche Besetzung / Aktivierung |
| --- | --- | --- |
| Projektleitung / Lead | Ziele, Tickets, Prioritäten, Dateibesitz, Verträge zur Abstimmung, Agent-Koordination, zentrale Status-/Planpflege, Reviewbeauftragung und Integration | Hauptagent; koordiniert, implementiert keine Produktfunktionen |
| Architecture & Security Engineer | Architekturvorschläge/ADRs, Vertrauensgrenzen, Tresor/Recovery/Sync, API-Verträge, Sicherheitsreview | P3-Worker (`p3a_security`); feste Fachrolle, zusätzlich nach P4a-Fertigstellung Apple Engineer; konkrete Folgeaufträge durch Lead |
| Backend & Platform Engineer | Go, DB-Adapter, serverseitige Tests, Compose/Swarm, Betriebs-/Buildpfade | P2-Worker; feste Fachrolle, praktische Plattformbefunde bleiben Abnahmevoraussetzung |
| Frontend & UX Engineer | Svelte/shadcn-svelte, deutsche Bedienabläufe, API-Integration, Tastatur/Touch, Frontendprüfungen | P1a-Worker (`p1a_parity`); feste Fachrolle, Webclient nach geprüftem API-Freeze und eigenem Client-Worktree |
| Apple Engineer | Swift/SwiftUI, direkte DB-Adapter, macOS/iPadOS, iCloud, eigenständiger Betrieb ohne Go | P3-Worker (`p3a_security`) in offengelegter Doppelrolle; Apple-Implementierung nach P4a-Fertigstellung im eigenen Apple-Repo-Worktree |
| QA & Release Engineer | Unabhängige Anforderungs-/Diffreviews, Integrations-/Regressionstests, Nachweismatrix, Releaseprüfung | `p0_publication_review`; feste unabhängige QA-/Release-Rolle, prüft aktuell P1a/P3a und zentrale Dokumentation |

Fachrolle und aktiver Agent-Slot sind verschieden. Maximal drei Worker arbeiten gleichzeitig;
weitere zugewiesene Agents bleiben bis zur Aktivierung ihrer Tickets idle. Der Lead koordiniert
diese Aktivierung. Ein weiterer dedizierter Apple-Agent konnte wegen des Threadlimits nicht
gestartet werden; deshalb wird die Apple-Rolle im kleinen Team durch Architecture & Security
übernommen. Architektur-/Security- und Apple-Implementierung erfolgen in abgegrenzten Tickets
nacheinander. Fachliche Rollenzuweisung allein belegt keine abgeschlossene Prüfung.
Dauerhafte Namen/Modellquoten sind keine Voraussetzung; dokumentierte Zuständigkeiten
und Übergaben müssen einen Agentwechsel ermöglichen.

Unabhängige Reviews können durch einen fachlich geeigneten anderen Engineer erfolgen.
Der Autor darf sein eigenes Paket prüfen, aber weder sein eigener unabhängiger Reviewer
noch die einzige Grundlage für die Abnahme sein. Bei Rollenwechsel auf dasselbe Thema
bleibt diese Trennung bestehen. Ein separater QA-Agent ist erforderlich, wenn unter den
vorhandenen Engineers niemand unabhängig und geeignet prüfen kann.

## Tickets vor Beginn: Definition of Ready

Ein ausführbares Ticket enthält:

1. Stabile ID, konkretes Nutzer-/Technikziel und prüfbare Abnahmekriterien; zugehörige
   Matrix-/Anforderungs-IDs und aktueller tatsächlicher Stand.
2. Repo, eigener Branch/Worktree, expliziter Dateibesitz, Nichtziele und Abhängigkeiten.
   Zwei schreibende Worker bearbeiten keine gemeinsame Datei. Gemeinsame Änderungen
   werden vorab vertraglich abgestimmt oder in einem eigenen Ticket nacheinander umgesetzt.
3. Zuständigen Engineer und einen vom Autor unabhängigen Reviewer; der Reviewer kann
   bei Übergabe zugewiesen werden, muss aber vor Integration tatsächlich geprüft haben.
4. Akzeptierten Vertragsstand oder ausdrücklich begrenztes Untersuchungsziel. Offene
   Treiber-/Kryptografie-/Plattformfragen werden durch Spikes geklärt; eine Vermutung
   wird nicht als freigegebener Produktvertrag behandelt.
5. Benötigte Fixtures, Testumgebung und passende Prüfungen. Fehlende Hardware oder
   Dienste sind als Begrenzung benannt und nicht als bestandener Nachweis verplant.

Der Lead hält diese Angaben im Arbeitsplan, Ticket oder Handoff fest. Fachliche Detailarbeit
und Implementierung erfolgen durch den beauftragten Engineer. Wesentliche Entscheidungen
werden als ADR vorgeschlagen und nach Review mit Status und Begründung festgehalten.

## Übergabe und Definition of Done

Ein Paket ist integrationsbereit, wenn sein konkreter Umfang umgesetzt beziehungsweise
sein Untersuchungsziel belastbar beantwortet ist und Folgendes vorliegt:

- Passende Prüfungen mit Datum, exakten Befehlen, Ausgang und tatsächlicher Umgebung;
  nicht ausgeführte Prüfungen bleiben explizit offen. Selbstprüfung ersetzt kein Review.
- Aktualisierte Dokumentation und `docs/handoffs/<paket>.md` mit Dateien, Branch/Commit,
  Befunden, offenen Grenzen, Daten-/Migrationsfolgen, laufenden Jobs und nächstem Schritt.
- Eigener fokussierter Commit mit bestehender Autoridentität; keine fremden Änderungen,
  Secrets, privaten Pfade oder produktiven Daten. AGPL-3.0-only für eigenen Code,
  Drittanbieterabhängigkeiten mit erhaltenen Lizenzen inventarisiert.
- Unabhängiger Review mit dokumentiertem Prüfumfang und Ergebnis; bestätigte Befunde
  behoben und nachgeprüft oder ausdrücklich als ungelöste Integrationsblocker geführt.

Der Lead prüft die Vollständigkeit dieser Nachweise, integriert und pflegt zentrale
Status-/Planungsdateien. Fertig im Workerbranch bedeutet noch nicht integriert, getestet
im Gesamtstand oder veröffentlicht. Bei notwendiger Nacharbeit wird dasselbe Paket wieder
geöffnet oder ein klar abgegrenztes Folgeticket mit Abhängigkeit angelegt.

## Integrationsgates

1. **Verträge:** API-/Datentyp-/Fähigkeitsänderungen vor abhängigen Clientänderungen
   reviewen. Hauptrepo ist kanonisch; Clients pinnen nachvollziehbaren Vertragsstand.
2. **Review:** Unabhängige Prüfung vor Integration, keine Zusammenführung eines Pakets
   mit ungelösten bestätigten wesentlichen Befunden. Reviewer schreibt keine fachfremden
   Produktänderungen in den Branch des Autors.
3. **Gesamtprüfung:** Nach Integration relevante kombinierte Checks, Build und CI prüfen.
   Clients müssen aus sauberem Checkout ohne zufällige Geschwisterdateien baubar sein.
4. **Nachweise:** STATUS/ARBEITSPLAN und Handoffs bilden tatsächliche Commits, Prüfungen,
   unintegrierte Arbeit und laufende Jobs ab. Geplante Eigenschaften bleiben als Plan markiert.

## Releasegates

Der QA-/Releaseverantwortliche legt für einen konkreten Releasekandidaten zusammen mit den
Fachrollen eine prüfbare Freigabeliste vor; der Lead koordiniert deren Abschluss.

- Releaseumfang und verbleibende Matrixlücken explizit ausweisen. Vollständige Parität
  erst behaupten, wenn alle zugesagten Funktionen samt Varianten tatsächlich belegt sind.
- Go/Web mit Docker Compose **und** Docker Swarm praktisch installieren, Gesundheit,
  Konfiguration, persistente Daten, Update und Wiederanlauf prüfen. Ein statisch gültiges
  YAML oder erfolgreicher Imagebuild belegt noch keinen lauffähigen Deploymentpfad.
- Apple separat auf macOS und echtem iPad prüfen; lokale Funktionen müssen ohne Go
  nutzbar sein. Simulator-/Go-Tests ersetzen keine Geräte-/Swift-Nachweise.
- Datenintegrität, Fehler-/Abbruchfälle, Kontentrennung, Geheimnisschutz, Recovery und
  Migrationen entsprechend zugesagtem Umfang prüfen. Kein Geheimnisspeicher auf Basis
  unvalidierter PRF-/Kryptografieannahmen veröffentlichen.
- Lizenzen, Abhängigkeitsinventar, Build-/Distributionsartefakte, Installationsanleitung
  und bekannte Einschränkungen auf den tatsächlichen Kandidaten beziehen. Offene
  Releaseblocker verhindern die Behauptung eines vollständig freigegebenen Produkts.

Ein Nutzungszwischenstand darf mit ehrlich begrenztem Funktionsumfang veröffentlicht werden;
er ersetzt weder die Endzielabnahme noch fehlende Plattformprüfungen. Den tatsächlichen Implementierungs- und
Freigabestand führt der Lead in [STATUS](STATUS.md); dieses Rollenmodell ist kein
Produkt- oder Releasefreigabenachweis.
