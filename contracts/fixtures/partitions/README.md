# W-P29-Vertragsbeispiele

Dateien `*.real-<system>.json` sind unveränderte Antworten der Livetests (MySQL 8.4,
MariaDB 12.3, PostgreSQL 18.6, SQLite; Lauf vom 26.10.2026, `scripts/test-partitions.py`);
`PrefixStructureResult.real-*` zeigt den Teilfehlerfall (MySQL/MariaDB: `applied`/`unknown`,
PostgreSQL/SQLite: Rollback). Alle übrigen JSON-Dateien sind **gekennzeichnete Platzhalter**, keine
beobachteten Datenbankantworten. `*.invalid.json` sind absichtlich ungültige
Schemafälle. Die Nullhashes sind nur Formbeispiele, keine ausführbaren Bestätigungen.

Echte Antworten erzeugt `python3 scripts/test-partitions.py --keep <Verzeichnis>`
mit der gemeinsamen Fixturebibliothek `scripts/dbfix.py`; `--sqlite-only` benötigt
kein Docker. Das Skript validiert die echten Dumps gegen API 0.22. Der
[Handoff](../../../docs/handoffs/W-P29-backend.md) trennt ausgeführte Prüfungen
von vorbereiteten Netzwerkfällen. Keine echten Fixtures ohne Laufnachweis übernehmen. SQLite kennt keine Partitionen (422), daher gibt es dafür keine Partitionsantworten.
