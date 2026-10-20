# ADR-0002: Drei Repositories und AGPL-3.0-only

Status: akzeptierter Bootstrap-Plan nach „Dann los“ am 20.10.2026.

Kontext: Go/Web und eigenständige Apple-App haben getrennte Toolchains und Laufzeiten.
Entscheidung: db-manager, db-manager-web und db-manager-apple; gemeinsame Dokumentation
und Verträge im Hauptrepo. Eigener Code AGPL-3.0-only, öffentliche Veröffentlichung erlaubt.
Alternativen: Monorepo oder separates Vertragsrepo; vorerst kein zusätzlicher Verwaltungsaufwand.
Folgen: Vertragspinning und Integrationsstand dokumentieren. Drittanbieterlizenzen inventarisieren.
Keine Entscheidung über endgültige Treiber, Verschlüsselungs- oder Synchronisierungsbibliotheken.
