# DB Manager

Geplante moderne Datenbankverwaltung: Go-Backend mit Svelte/shadcn-svelte-Weboberfläche
und eigenständiger Swift/SwiftUI-App für macOS und iPadOS. Ziel ist vollständige
phpMyAdmin-Parität und Unterstützung mehrerer Datenbanksysteme.

**Aktueller Zwischenstand:** Go-HTTP-Grundlage mit versioniertem API-Vertragskandidaten,
Tokenanmeldung sowie geprüfter Docker-Compose-/Swarm-Bereitstellung vorhanden.
Die Datenbank-Engine ist noch ein Platzhalter; es gibt noch keine nutzbare Datenbank-App.
Swift soll direkt mit Datenbanken arbeiten können; das Go-Backend bleibt dort optional.

- [Projektplan](docs/PROJEKTPLAN.md), [Konzept](docs/KONZEPT.md)
- [Status](docs/STATUS.md), [Arbeitsplan](docs/ARBEITSPLAN.md)
- [Dokumentationslandkarte](docs/README.md), [Einrichtung](EINRICHTUNG.md)
- [Betrieb mit Compose und Swarm](docs/BETRIEB.md), [Grundlagen-Handoff](docs/handoffs/P4a.md)
- [Agents](AGENTS.md), [Claude Code](CLAUDE.md)
- [Repositories](docs/REPOSITORIES.md), [Lizenz](LICENSE)

Eigener Projektcode: **AGPL-3.0-only**. Drittanbieter: [Lizenzinventar](docs/LIZENZEN.md).
