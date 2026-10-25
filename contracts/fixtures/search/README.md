# Synthetische W-P14-Fixtures

Alle Namen und Werte sind erfunden. `synthetic_p15` ist eine verwaltete SQLite-Kennung,
kein Dateipfad. Die 64 Nullen in `ReplacePlan.placeholder.json` und
`ReplaceExecuteRequest.placeholder.json` sind ausdrücklich **Hash-Platzhalter** und
kein ausführbarer Plan. Echte Hashes entstehen erst aus Anfrage, Treiber, Metadaten,
vollständigem Tabellenbestand und nativ berechneten Nachherwerten.

Dateinamen beginnen mit dem Schemanamen; `*.invalid.json` muss der Wächter ablehnen.
Echte SQLite-/Netzwerkergebnisse prüft `scripts/test-search.py` zusätzlich gegen
 denselben Vertrag. Fixtures enthalten keine Zugangsdaten.
