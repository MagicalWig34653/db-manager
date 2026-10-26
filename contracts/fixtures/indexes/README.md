# Herkunft der W-P09-Vertragsfixtures

Dateien mit `placeholder` sind handgeschriebene Vertragsplatzhalter, keine echten
Engine-Antworten. Dateien mit `invalid` sind absichtlich ungültige Vertragsproben.
Insbesondere ist der Hash im PostgreSQL-Planplatzhalter kein berechneter Planhash.

Dateien mit `real-sqlite` stammen aus `scripts/test-indexes.py --sqlite-only` gegen
die echte eingebettete SQLite-Engine beziehungsweise authentifizierte HTTP-Handler.
MySQL-/MariaDB-/PostgreSQL-Livebelege fehlen, bis die Docker-Suite tatsächlich läuft.

Ergänzung Integration (Welle 3): Dateien `real-mysql-*`, `real-mariadb-*`, `real-postgres-*` (und weitere `real-sqlite-*`) sind echte Antworten aus
`scripts/test-indexes.py --keep` gegen MySQL 8.4, MariaDB 12.3, PostgreSQL 18.6 (TLS verify-full, Wegwerf-Container nacheinander), 26.10.2026.
Je System und Schema sind höchstens vier Beispiele abgelegt; Platzhalter bleiben als Schema-Beispiele.
