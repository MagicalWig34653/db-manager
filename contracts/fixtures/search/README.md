# Synthetische W-P14-Fixtures

Alle Namen und Werte sind erfunden. `synthetic_p14` ist eine verwaltete SQLite-Kennung,
kein Dateipfad. Die 64 Nullen in `ReplacePlan.placeholder.json` und
`ReplaceExecuteRequest.placeholder.json` sind ausdrücklich **Hash-Platzhalter** und
kein ausführbarer Plan. Echte Hashes entstehen erst aus Anfrage, Treiber, Metadaten,
vollständigem Tabellenbestand und nativ berechneten Nachherwerten.

Dateinamen beginnen mit dem Schemanamen; `*.invalid.json` muss der Wächter ablehnen.
Echte SQLite-/Netzwerkergebnisse prüft `scripts/test-search.py` zusätzlich gegen
 denselben Vertrag. Fixtures enthalten keine Zugangsdaten.

Ergänzung Integration (Welle 3): Dateien `*.real-*.json` sind echte Antworten aus `scripts/test-search.py --keep` gegen SQLite, MySQL 8.4,
MariaDB 12.3 und PostgreSQL 18.6 (TLS verify-full, Wegwerf-Container nacheinander), 26.10.2026; Platzhalter bleiben als Schema-Beispiele.
