# DB Manager

Moderne Datenbankverwaltung im Aufbau: Go-Backend mit Svelte/shadcn-svelte-Weboberfläche
und eigenständiger Swift/SwiftUI-App für macOS und iPadOS. Ziel ist vollständige
phpMyAdmin-Parität und Unterstützung mehrerer Datenbanksysteme.

**Aktueller Zwischenstand:** Go-API0.1.0 mit Tokenanmeldung, echter SQL-Ausführung,
Verbindungstest und Tabellen-/View-Katalog für SQLite, MySQL/MariaDB und PostgreSQL.
SQLite ist auf exklusiven lokalen Einzelinstanzbetrieb begrenzt. Compose/Swarm wurden
praktisch geprüft, einschließlich UID10001, schreibgeschütztem Root und Persistenz.
Der unabhängig geprüfte Webclient liegt mit festem Quellcommit und Lizenzhinweisen
als Go-Paket vor; Serververdrahtung und echter Web→Go→DB-Pfad folgen noch separat.

Ein eigenständiger Swift/SwiftUI-SQLite-Arbeitsplatz ohne Go ist im Apple-Repository
integriert, für macOS und den iPad-Simulator geprüft. Echte Geräte, Distribution,
vollständige Parität, produktiver Tresor/Sync/Teams und sichere native Netzwerkadapter
bleiben offen. Das Go-Backend bleibt für die Apple-App optional.


- [Projektplan](docs/PROJEKTPLAN.md), [Konzept](docs/KONZEPT.md)
- [Status](docs/STATUS.md), [Arbeitsplan](docs/ARBEITSPLAN.md)
- [Dokumentationslandkarte](docs/README.md), [Einrichtung](EINRICHTUNG.md)
- [Betrieb mit Compose und Swarm](docs/BETRIEB.md), [Grundlagen-Handoff](docs/handoffs/P4a.md)
- [Versionierter Webbuild und Go-Paket](docs/WEBBUILD.md), [F2a-Handoff](docs/handoffs/F2a.md)
- [Agents](AGENTS.md), [Claude Code](CLAUDE.md)
- [Repositories](docs/REPOSITORIES.md), [Lizenz](LICENSE)

Eigener Projektcode: **AGPL-3.0-only**. Drittanbieter: [Lizenzinventar](docs/LIZENZEN.md).
