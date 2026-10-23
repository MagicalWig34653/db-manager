# Entwicklungsteam und Arbeitsmodell

Stand: 23.10.2026. Nutzerauftrag: professionelles Softwareentwicklungsteam mit
Agents. Root übernimmt ausschließlich Projektleitung und Koordination; beauftragte
Agents führen Implementierung, Dokumentation, Prüfungen und freigegebene Integration aus.

## Rollen und aktueller Einsatz

| Rolle | Verantwortung | Aktueller Einsatz |
| --- | --- | --- |
| Projektleitung / Lead | Ziele, Prioritäten, begrenzte Tickets, Dateibesitz, Abhängigkeiten, Reviewzuweisung und konkrete Integrationsfreigaben | Root; keine eigene Produkt-, Dokumentations- oder Releaseimplementierung |
| Architecture & Security | ADRs, Vertrauensgrenzen, gemeinsame Verträge, Tresor/Recovery und Sicherheitsreview | S1/S2a abgenommen; produktiver OS-Anchor und weitere Plattformgates separat offen |
| Backend & Platform | Go, DB-Adapter, Metadaten/Mutation/DDL, Compose/Swarm und reproduzierbare Builds | B4a implementiert und unabhängig geprüft; API0.6-Integration/Freeze noch eigener Schritt |
| Frontend & UX | Deutscher Webarbeitsplatz, Vertragsclients, Tastatur/Touch und Browserprüfungen | F6 integriert; F5 mit gezieltem Darstellungsfix und unabhängiger Nachprüfung |
| Apple Engineering | Eigenständiges Swift/SwiftUI, direkte DB-Verbindungen, Signing, Archive und Icon Composer | A-TF1-Vorbereitung mit registrierter Bundle-ID und lokalen signierten Exporten; CI-/Uploadgates getrennt |
| Site Engineering | GitHub Pages, Landingpage, kanonische Dokumentation, Roadmap und echte Screenshots | W1-Kandidat im unabhängigen Site-/Deploymentreview |
| QA | Unabhängige Source-/Vertrags-/Artefakt-/UI-/Integrationsprüfungen und reproduzierbare Findings | Fachlich geeignete Peers prüfen jeweils fremde Pakete; kein Autor nimmt sein eigenes Paket allein ab |
| Release & Integration | Freigegebene Commits/Reviewprovenienz übernehmen, kombinierte Checks, CI und autorisierte Veröffentlichung | Separat zugewiesener Releasekoordinator; kein ungeprüfter Merge oder Upload |
| Projektdokumentation | Aktuelle Hauptstände, Gates, Dateibesitz, Prüfstand und nächste Schritte konsistent halten | Exklusiver Dokumentations-Agent für Root-STATUS/ARBEITSPLAN; Clientstatus nur nach eigener Zuweisung |

Die Laufzeit bietet aktuell **31 gleichzeitige Plätze einschließlich Root**, also
bis zu 30 Worker. Das Team wird nach konkreten unabhängigen Paketen und benötigten
Fachrollen besetzt; die Kapazität ist keine vorgeschriebene Teamgröße. Keine festen
Anbieter-, Modell- oder Rollenquoten. Fachrolle und aktiver Agent-Slot bleiben
getrennt, qualifizierte Peers können Rollen zwischen abgegrenzten Paketen wechseln.
Zuständigkeiten und Übergaben müssen auch ohne dauerhafte Agentnamen fortsetzbar sein.

## Lokale Prüfressourcen

Schwere Prüfungen werden vor Start mit der Koordination abgestimmt und mit eigenem
Präfix/Jobhandle dokumentiert. Es laufen höchstens **zwei kleine DB-Prüfsuiten**
gleichzeitig. Ein DinD-/Swarm-Lauf erhält die Ressource exklusiv; es gibt keinen
zweiten parallelen DinD-Lauf. Weitere schwere Container-/Buildläufe werden für
diese Zeit zurückgestellt oder ausdrücklich anhand der verfügbaren Ressourcen
abgestimmt. Vor einem erneuten Lauf bestehende Prozesse und Ressourcen tatsächlich
prüfen; ein Beobachtungstimeout ist kein Prozessende.

Jede Suite verwendet eigene zufällige Container, Volumes, Netze und Scratchs.
Fremde Entwicklungscontainer und der Host-Swarm bleiben unverändert. Keine pauschalen
Prune-/Kill-/Löschaktionen. Der Eigentümer bereinigt ausschließlich seine bestätigten
Ressourcen und meldet Beendigung beziehungsweise verbleibende Jobs im Handoff.

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

Der Lead weist den Auftrag zu; der beauftragte Agent hält diese Angaben im
Arbeitsplan, Ticket oder Handoff fest. Fachliche Detailarbeit und Implementierung
erfolgen durch den zuständigen Engineer. Wesentliche Entscheidungen
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

Der Lead koordiniert die Vollständigkeitsprüfung und erteilt konkrete Freigaben.
Beauftragte Release- und Dokumentations-Agents integrieren beziehungsweise pflegen
die ihnen exklusiv zugewiesenen zentralen Dateien. Fertig im Workerbranch bedeutet noch nicht integriert, getestet
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
Freigabestand führt der beauftragte Dokumentations-Agent in [STATUS](STATUS.md); dieses Rollenmodell ist kein
Produkt- oder Releasefreigabenachweis.
