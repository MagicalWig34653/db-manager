# W-P29-Vertragsbeispiele

Alle JSON-Dateien in diesem Ordner sind **gekennzeichnete Platzhalter**, keine
beobachteten Datenbankantworten. `*.invalid.json` sind absichtlich ungültige
Schemafälle. Die Nullhashes sind nur Formbeispiele, keine ausführbaren Bestätigungen.

Echte Antworten erzeugt `python3 scripts/test-partitions.py --keep <Verzeichnis>`
mit der gemeinsamen Fixturebibliothek `scripts/dbfix.py`; `--sqlite-only` benötigt
kein Docker. Das Skript validiert die echten Dumps gegen API 0.21. Der
[Handoff](../../../docs/handoffs/W-P29-backend.md) trennt ausgeführte Prüfungen
von vorbereiteten Netzwerkfällen. Keine echten Fixtures ohne Laufnachweis übernehmen.
