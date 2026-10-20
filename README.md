# DB Manager

Geplante moderne Datenbankverwaltung: Go-Backend mit Svelte/shadcn-svelte-Weboberfläche
und eigenständiger Swift/SwiftUI-App für macOS und iPadOS. Ziel ist vollständige
phpMyAdmin-Parität und Unterstützung mehrerer Datenbanksysteme.

**Aktueller Zwischenstand:** Go-API 0.1.0 mit Tokenanmeldung, echter SQL-Ausführung,
Verbindungstest und Tabellen-/View-Katalog für SQLite, MySQL/MariaDB und PostgreSQL.
SQLite ist auf einen exklusiven lokalen Einzelinstanzbetrieb begrenzt. Compose startet
als UID 10001 mit schreibgeschütztem Root und beschreibbarem Datenvolume; SQLite-Daten
überstehen den Neustart. Der Webclient ist noch nicht in das Go-Image eingebunden.
Unabhängige Gesamt-/Releaseabnahme und vollständige phpMyAdmin-Parität stehen aus.
Swift arbeitet in seinem eigenen Repository; das Go-Backend bleibt dort optional.

- [Projektplan](docs/PROJEKTPLAN.md), [Konzept](docs/KONZEPT.md)
- [Status](docs/STATUS.md), [Arbeitsplan](docs/ARBEITSPLAN.md)
- [Dokumentationslandkarte](docs/README.md), [Einrichtung](EINRICHTUNG.md)
- [Betrieb mit Compose und Swarm](docs/BETRIEB.md), [Grundlagen-Handoff](docs/handoffs/P4a.md)
- [Agents](AGENTS.md), [Claude Code](CLAUDE.md)
- [Repositories](docs/REPOSITORIES.md), [Lizenz](LICENSE)

Eigener Projektcode: **AGPL-3.0-only**. Drittanbieter: [Lizenzinventar](docs/LIZENZEN.md).
