# Projektplan DB Manager

Stand: 20.10.2026. Plan zur Umsetzung freigegeben; P0-Gerüste lokal installiert, Veröffentlichung in Vorbereitung. Keine Produktimplementierung.

## Grundlage und Verbindlichkeit

Fachliche Ausgangslage: [Projektübergabe](../PROJEKTUEBERGABE.md).
Diese beschreibt verbindliche Anforderungen und davon getrennte Vorschläge.
Der vorliegende Plan ergänzt die aktuelle Nutzerentscheidung:

- Swift-App im vorgesehenen lokalen Verzeichnis `Xcode/db-manager-apple`.
- Eigener Projektcode in allen Repositories unter `AGPL-3.0-only`.
- Repositories dürfen öffentlich auf GitHub stehen.
- Entwicklung mit Agents, Claude-Code-kompatibel, Repository als Projektgedächtnis.
- Sessions sind wegwerfbar; Entscheidungen, Umsetzung, Prüfungen und Übergaben dauerhaft dokumentieren.
- Zunächst planen. Repository-Bootstrap und Implementierung folgen als eigene Etappen.

Repositoryaufteilung, technische Detailentscheidungen und Reihenfolge unten sind Vorschläge.
Keine Entscheidung aus dem Timetrack-Projekt wird allein durch dessen Vorbild verbindlich.

## Repositoryaufteilung

| Vorgeschlagenes Repository | Inhalt und Verantwortung |
| --- | --- |
| `db-manager` | Go-Backend, zentrale Produktdokumentation, ADRs, API- und Austauschverträge, plattformübergreifende Testfälle, Betrieb und Integration |
| `db-manager-web` | Svelte/shadcn-svelte-Weboberfläche, Backend-Client, Frontendtests |
| `db-manager-apple` | Swift/SwiftUI-App für macOS und iPadOS, lokale Datenbankadapter, Tresor, Synchronisierung, optionaler Backend-Client |

Die drei lokalen Repos sind gemäß [Repositoryübersicht](REPOSITORIES.md) installiert.
GitHub-Owner und Web-Pfad sind geprüft; Veröffentlichungsstand siehe [Status](STATUS.md).
Öffentliche Dateien verwenden relative Pfade und Platzhalter statt privater Maschinenpfade.
Ein separates viertes Vertragsrepo ist zunächst nicht vorgesehen.

Das Hauptrepo besitzt die kanonischen gemeinsamen Verträge. Clients übernehmen versionierte
Vertragsstände mit Herkunft, Version und Prüfsumme. CI erkennt Abweichungen. Ein sauberer
Checkout eines Clients muss ohne zufällig vorhandene Geschwister-Repositories baubar sein.
Eine Kompatibilitätsmatrix dokumentiert unterstützte Backend-, Client- und Vertragsversionen.

## Architekturentwurf

Der Go-Teil wird als modularer Monolith geplant: Verbindungsverwaltung, Datenbankadapter,
Abfrageausführung, Metadaten, Import/Export, Tresor, Identitäten, Teams und Jobs.
Das Webfrontend nutzt eine versionierte Backend-API. Ein auslieferbares Backend mit
eingebettetem Webbuild sowie ein Container sind geplante Bereitstellungswege.
Framework, interne Persistenz und Deploymentdetails bleiben bis zu ihrer Bewertung offen.

Die Apple-App besitzt einen eigenständigen lokalen Kern. SwiftUI hängt an Anwendungsfällen
und Adapterverträgen; Datenbanktreiber, Tresor und Synchronisierung sind getrennte Module.
Direkte Verbindungen und Backend-Verbindungen erhalten klar erkennbare Herkunft und
unterschiedliche Fähigkeiten. Ein Backend-Ausfall darf lokale Verbindungen nicht blockieren.
Mehrere lokale und entfernte Verbindungen sollen gleichzeitig darstellbar sein.

Als Arbeitshypothese werden Swift- und Go-Adapter getrennt implementiert. Gemeinsame
Verhaltensspezifikationen, typisierte Austauschformate und dieselben Referenzfälle begrenzen
Abweichungen. Eine gemeinsame eingebettete Bibliothek wird im Treiberspike als Alternative
bewertet; die endgültige Entscheidung folgt erst danach als ADR.

Adapter melden Fähigkeiten explizit: beispielsweise Transaktionen, Abbruch, Streaming,
Schemaänderungen, Rechteverwaltung und datenbankspezifische Objekte. Unbekannt oder
unsupported darf nicht als erfolgreich ausgeführt erscheinen. SQL-Werte, NULL, Binärdaten,
große Zahlen, Zeitwerte und Fehler brauchen definierte, verlustarme Darstellungen.
Der Apple-Treiberspike umfasst auch SSH-Tunnel, App-Hintergrundverhalten und einen Lauf
auf einem echten iPad; Simulatorergebnisse allein belegen die Geräteunterstützung nicht.

## Umfang und Priorisierung

Vollständige phpMyAdmin-Parität bleibt das Produktziel. Ein erster nutzbarer Stand ist
ein Zwischenziel und darf nicht als erreichte Parität bezeichnet werden.

Vorgeschlagene Reihenfolge: MySQL/MariaDB zuerst; PostgreSQL und SQLite danach.
Die Priorisierung ist noch nicht bestätigt. Vor der Umsetzung wird eine konkrete
phpMyAdmin-Referenzversion mit Quelle und Datum festgehalten und eine Paritätsmatrix erstellt.
Jeder Eintrag enthält Referenzfunktion, Sollverhalten, Datenbanksystem, Plattform,
Priorität, Abnahmetest und tatsächlichen Erfüllungsstand.

Die Matrix umfasst auch SQL-Stapel und mehrere Ergebnisse, Benutzer und Rechte,
Routinen/Trigger/Events, Wartung, Import-/Exportoptionen, Designer, Transformationen,
Änderungsverfolgung, Einstellungen und Übersetzungen. Funktionen werden hinsichtlich
Anwendbarkeit je Plattform bewertet; Abweichungen brauchen eine ausdrückliche Entscheidung.

## Etappen und Abnahme

| Etappe | Ergebnis | Abnahme / Voraussetzung für Fortsetzung |
| --- | --- | --- |
| P0 – Grundlagen | Repository-, Lizenz-, Dokumentations- und Agent-Gerüste; Anforderungen und offene Entscheidungen | Frische Session findet Ziel, Stand, Auftrag und nächsten Schritt allein aus Dateien; drei Repo-Grenzen dokumentiert |
| P1 – Paritätsplanung | Referenzversion, Funktionsmatrix, Bedienabläufe und unterstützte DB-/OS-Ziele | Anforderungen sind überprüfbar; Zwischenziel und vollständige Parität getrennt |
| P2 – Treiber-Spikes | Wegwerfprototypen für direkte Verbindungen auf Mac und iPad sowie Go-Adapter | TLS, Authentifizierung, Typen, Abbruch, große Ergebnisse und Plattform-/Lizenzgrenzen praktisch geprüft; Architektur-ADR |
| P3 – Tresor und Sync-Spikes | Bedrohungsmodell, Masterpasswort, Passkey-Untersuchung, Wiederherstellung und verschlüsselter iCloud-Prototyp | Neues Gerät, Verlust eines Entsperrwegs, Offlinebetrieb und Konflikte geprüft; keine Passkey-PRF-Annahme ohne Nachweis |
| P4 – Durchgehender Produktpfad | Verbindung → Objektbaum → begrenzte Tabellenansicht → SQL → Ergebnis auf Web und Apple | Web über Go und Apple direkt gegen echte Wegwerf-Datenbank getestet; Apple funktioniert ohne Go |
| P5 – Nutzbarer persönlicher Client | Bearbeiten mit Konflikterkennung, Schemafunktionen, gespeicherte Abfragen, Import/Export, Tresor und Sync | Datenintegrität, große Datenmengen, Unterbrechungen und Wiederanlauf geprüft; lokale Nutzung ohne iCloud bleibt möglich |
| P6 – Teamfunktionen | Backend-Konten, Rollen, explizite Freigaben, persönliche/geteilte Zugangsdaten, Jobs und Audit | Rechteentzug, Kontentrennung, Schlüsselwechsel und Jobberechtigungen getestet; persönliche Verbindungen bleiben ohne Freigabe privat |
| P7 – Vollständige Parität und Release | Übrige Matrixfunktionen, weitere DB-Systeme, Bedienbarkeit, Distribution und Betrieb | Alle zugesagten Matrixeinträge belegt; Lizenzen, Installationswege, Updates, Migrationen und Plattformtests dokumentiert |

P1 und die beiden Untersuchungsstränge P2/P3 können nach P0 parallel arbeiten.
Produktive Geheimnisspeicherung und Synchronisierung beginnen erst nach Auswertung von P3.
P4 hängt vom Treiberentscheid ab und kann bis dahin ausschließlich flüchtige Zugangsdaten nutzen.
Keine Zeitversprechen vor Auswertung der Spikes und Zerlegung der Paritätsmatrix.

## Sicherheits- und Datenmodell als Planungsaufgabe

- Anmeldung, Tresorentsperrung und tatsächliche Datenbankberechtigung sind getrennte Vorgänge.
- Der Prozess, der die DB-Verbindung ausführt, benötigt dort Zugang zu den Klartextgeheimnissen.
- Ein normaler Passkey-Login liefert keinen Tresorschlüssel. PRF-Verfügbarkeit und der
  eigenständige Apple-Betrieb sind offene Machbarkeitsfragen. Masterpasswort bleibt erforderlich,
  wenn kein nutzbarer Passkey-Entsperrweg vorhanden ist.
- Passwortableitung, Schlüsselhierarchie, Metadaten, Sperren und Recovery werden vor Speicherung
  echter Geheimnisse spezifiziert und geprüft; keine eigene Kryptoprimitive entwickeln.
- iCloud-Sync transportiert zusätzlich verschlüsselte persönliche Geheimnisse. Schlüsselaufnahme,
  Rotation und konkurrierende Änderungen sind Teil des Designs, kein späterer Zusatz.
- Persönliche und geteilte Bereiche haben getrennte Freigaberegeln. Entfernte Freigaben löschen
  kein bereits bekanntes Geheimnis; Zugangsdatenrotation muss im Modell berücksichtigt werden.
- SQL ist bewusst mächtig. API-Autorisierung und DB-Rechte werden gemeinsam getestet;
  frei konfigurierbare Backend-Verbindungen benötigen eine definierte Netzwerkzugriffsrichtlinie.
- Tests verwenden isolierte Wegwerf-Datenbanken und synthetische Daten. Zugangsdaten,
  Abfrageergebnisse und produktive SQL-Verläufe gehören nicht in öffentliche Logs oder Repos.

## Agent-Arbeitsweise

Vorbild sind die Dokumentations- und Übergaberegeln von Timetrack, nicht dessen Fachlichkeit,
historische Modellquoten, Pluginabhängigkeiten oder projektspezifische Ausnahmen.
Die dortigen Dateien widersprechen sich bei älteren Codex-Quoten und späterer Claude-only-Regel.
Für DB Manager wird deshalb eine einzige kanonische Prozessbeschreibung gepflegt.

Ein Lead koordiniert Arbeitspakete und Integration. Implementierer bearbeiten abgegrenzte Pakete;
ein anderer Agent prüft Anforderungen und Diff unabhängig. Jeder schreibende Agent verwendet
einen eigenen Branch/Worktree. Gemeinsame Verträge werden vor abhängiger Arbeit versioniert.
Nur der Lead bearbeitet zentrale Status-/Planungsdateien während paralleler Arbeit.
Dokumentationsarbeit und unabhängige Reviews sind ebenfalls konkrete Arbeitspakete.

Jeder Auftrag nennt Ziel, Nichtziele, Repo, Branch/Worktree, Dateibesitz, Vertragsstand,
Abhängigkeiten, Abnahmekriterien und erlaubte Prüfungen. Das Ergebnis enthält Dateien/Commits,
ausgeführte Prüfungen, Einschränkungen, Reviewbefunde und nächsten Schritt.
Session- oder Agent-IDs sind Zusatzinformationen und keine Voraussetzung zur Fortsetzung.

`AGENTS.md` ist der gemeinsame Einstieg. `CLAUDE.md` verweist auf dieselben Regeln und
ergänzt nur Claude-spezifische Hinweise. Ein `CLAUDE_STARTPROMPT.md` gibt den wiederverwendbaren
Startauftrag. Optionale Claude-Subagent-Definitionen dürfen keine eigenen Projektwahrheiten
enthalten. Der Ablauf muss auch ohne Plugins und persönliche Agent-Memory funktionieren.
Keine festen Modellquoten oder Einschränkung auf einen Anbieter ohne Nutzerentscheidung.

Vorgeschlagene Sprachregel: Dokumentation und Oberfläche Deutsch; Code, Kommentare,
Tests und technische Bezeichner Englisch; Nutzertexte in Lokalisierungsressourcen.

## Dauerhafte Dokumentation

| Datei im Hauptrepo | Kanonischer Inhalt |
| --- | --- |
| `docs/KONZEPT.md` | Bestätigte Anforderungen, Vorschläge und offene Fragen klar getrennt |
| `docs/ARCHITEKTUR.md` | Module, Ausführungswege, Verträge und Vertrauensgrenzen |
| `docs/PARITAET.md` | Referenz und vollständige Funktions-/Abnahmematrix |
| `docs/UMSETZUNG.md` | Etappen und Abnahmekriterien |
| `docs/STATUS.md` | Tatsächlicher Stand, geprüfte Ergebnisse und Einschränkungen |
| `docs/ARBEITSPLAN.md` | Paketstatus, Zuständigkeiten, Abhängigkeiten und nächste Schritte |
| `docs/ENTWICKLUNGSABLAUF.md` | Einheitliche Agent-, Review- und Dokumentationsregeln |
| `docs/decisions/` | ADRs mit Status, Gründen, Alternativen und Folgen |
| `docs/handoffs/` | Kompakte Ergebnisse einzelner Pakete und Übergaben |
| `docs/REPOSITORIES.md` | Repo-Zuordnung, Vertragsversionen und Integrationsregeln |
| `docs/LIZENZEN.md` | Abhängigkeiten, Lizenzen und Prüfstand |
| `EINRICHTUNG.md` | Tatsächliche, reproduzierbar geprüfte Start-/Build-/Testbefehle |

Jedes Clientrepo besitzt eigene Einrichtung, Status, Arbeitsplan und Handoffs für seinen
Code sowie Verweise auf versionierte gemeinsame Spezifikationen. Keine ungepflegten Vollkopien.
Eine neue Session liest Einstieg, Status, Arbeitsplan und relevante ADRs und verifiziert dann
Git-Zustand und Dateien. Alte Einträge „in Arbeit“ werden vor neuer Delegation geprüft.

Zu jedem integrierten Paket gehören Code, passende Prüfungen, unabhängiges Review und
aktualisierte Dokumentation. Nicht ausgeführte Prüfungen bleiben ausdrücklich offen.
Vor Kontextwechsel werden Änderungen, Branch/Commit, Testergebnisse, Probleme, laufende Jobs
und nächster konkreter Schritt gesichert. Wichtige Zwischenstände regelmäßig committen,
sobald Repositories initialisiert sind. Keine vollständigen Chats als Ersatz für Dokumentation.

## Repository-Bootstrap nach der Planungsphase

1. Namen, GitHub-Owner, lokalen Web-Pfad und vorhandene Git-Autoridentität prüfen.
2. Drei Repositories initialisieren; keine fremden Änderungen oder vorhandene Daten überschreiben.
3. Vollständigen AGPLv3-Lizenztext, eindeutige `AGPL-3.0-only`-Kennzeichnung, README,
   Agent-Einstiege, Startprompt, Dokumentationslandkarte und erste ADRs anlegen.
4. Ignore-Regeln, Beispielkonfiguration ohne Secrets, Beitrags-/Sicherheitshinweise und
   Issue-/PR-Vorlagen ergänzen; Dependency-Inventar ab erster Abhängigkeit führen.
5. Minimale CI für Dokumentationskonsistenz und Lizenzkennzeichnung; Codeprüfungen erst
   mit realen Projektskeletten hinzufügen. Geplante Befehle nicht als geprüft ausgeben.
6. Konsistenten Anfangsstand committen und öffentliche GitHub-Repos anlegen/verknüpfen.
   GitHub-Berechtigungen und Schreibzugriff auf die lokalen Zielordner praktisch prüfen.
7. P1/P2/P3 als begrenzte Agent-Aufträge starten und Ergebnisse vor Architekturfestlegung reviewen.

AGPL-3.0-only gilt für den eigenen Projektcode. Drittanbieter behalten ihre Lizenzen;
die Auswahl wird auf Kompatibilität und Verteilung geprüft. Lizenz-Ausnahmen aus Timetrack
werden nicht übernommen. Öffentliche Freigabe ersetzt keine Prüfung auf Secrets/private Daten.

## Offene Entscheidungen

Für den Bootstrap geklärt: GitHub-Owner, Web-Pfad und endgültige Reponamen.
Vor Treiberauswahl: erste Datenbanksysteme und unterstützte Server-/Apple-OS-Versionen.
Vor Festlegung von Tresor und Sync: Entsperrwege, Recovery, Geräteaufnahme und Konfliktmodell.
Vor Teams: Rollen, Freigaben, Jobberechtigungen und Schlüsselrotation.
Vor Distribution: App-Verteilungswege, Signierung und CloudKit-Umgebungen.
iPhone ist weiterhin eine mögliche spätere Erweiterung, kein zugesagtes Erstziel.

Aktueller nächster Schritt: P0 nach unabhängiger Prüfung committen und veröffentlichen; siehe STATUS und EINRICHTUNG. Technische Machbarkeit
der Produktfunktionen wurde bisher nicht behauptet oder getestet.
