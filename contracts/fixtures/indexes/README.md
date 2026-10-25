# Herkunft der W-P09-Vertragsfixtures

Dateien mit `placeholder` sind handgeschriebene Vertragsplatzhalter, keine echten
Engine-Antworten. Dateien mit `invalid` sind absichtlich ungültige Vertragsproben.
Insbesondere ist der Hash im PostgreSQL-Planplatzhalter kein berechneter Planhash.

Dateien mit `real-sqlite` stammen aus `scripts/test-indexes.py --sqlite-only` gegen
die echte eingebettete SQLite-Engine beziehungsweise authentifizierte HTTP-Handler.
MySQL-/MariaDB-/PostgreSQL-Livebelege fehlen, bis die Docker-Suite tatsächlich läuft.
