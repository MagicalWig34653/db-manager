# DB Manager

Moderne Datenbankverwaltung im Aufbau: Go-Backend mit Svelte/shadcn-svelte-Weboberfläche
und eigenständiger Swift/SwiftUI-App für macOS und iPadOS. Ziel ist vollständige
phpMyAdmin-Parität und Unterstützung mehrerer Datenbanksysteme.

**Aktueller Zwischenstand:** Go-API mit echter SQLite-/MySQL-/MariaDB-/PostgreSQL-
Ausführung, Tokenanmeldung, Katalog und strukturierter Tabellenansicht. Bestätigte
Einzelzeilen-Inserts/Updates/Deletes sind im Backend unabhängig geprüft; die passende
Web-Schreiboberfläche wird separat umgesetzt. Der eingebettete deutsche Webclient
bietet Verbindung, Tabellenmetadaten, Sortierung, Filter, Pagination, SQL und begrenzte
präzise Ergebnisse sowie Quellcode-/Originallizenzhinweise.

Docker Compose und Docker Swarm sind praktisch geprüft, einschließlich UID10001,
schreibgeschütztem Root und SQLitepersistenz nach Neustart/Taskersatz. SQLite ist auf
exklusiven lokalen Einzelinstanzbetrieb begrenzt. Reproduzierbare Webassets gehören
zum Go-Checkout; Node/npm werden zur Laufzeit nicht benötigt.

Die eigenständige Swift/SwiftUI-App unterstützt SQLite und direktes PostgreSQL ohne
Go mit verifiziertem TLS. Mac-/iPad-Simulator-Builds und echte synthetische Datenbanken
sind geprüft. Echte Geräte, Signing, Distribution, vollständige Parität, produktiver
Tresor/Sync/Teams und native MySQL-/MariaDB-Adapter bleiben offen. Konkrete Commit-/
CI-Nachweise und Grenzen stehen im [Status](docs/STATUS.md).


- [Projektplan](docs/PROJEKTPLAN.md), [Konzept](docs/KONZEPT.md)
- [Status](docs/STATUS.md), [Arbeitsplan](docs/ARBEITSPLAN.md)
- [Dokumentationslandkarte](docs/README.md), [Einrichtung](EINRICHTUNG.md)
- [Betrieb mit Compose und Swarm](docs/BETRIEB.md), [Grundlagen-Handoff](docs/handoffs/P4a.md)
- [Versionierter Webbuild und Go-Paket](docs/WEBBUILD.md), [F2a-Handoff](docs/handoffs/F2a.md)
- [Agents](AGENTS.md), [Claude Code](CLAUDE.md)
- [Repositories](docs/REPOSITORIES.md), [Lizenz](LICENSE)

Eigener Projektcode: **AGPL-3.0-only**. Drittanbieter: [Lizenzinventar](docs/LIZENZEN.md).
