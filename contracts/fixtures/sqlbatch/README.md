# W-P15-Vertragsfixtures

Synthetische Beispiele, keine produktiven Daten oder realen Verbindungen.
`*_PLACEHOLDER` und Texte mit „PLATZHALTER“ sind ausdrücklich zu ersetzende
Vertragsbeispiele. Host, Benutzer und Passwort sind niemals nutzbare Zugangsdaten.
Dauer, Servercodes, Zeilenanzahlen und Pläne sind Beispielwerte, kein Livetestnachweis.

Dateiname: `<Schema>.<Fall>.json`, Ablehnungsfälle zusätzlich `.invalid.json`.
Der [Wächter](../../../scripts/check-sqlbatch-contract.py) prüft strikte Schemas,
additive Definitionen und den unveränderten Altvertrag. Echte SQLite-/Netzwerkantworten
werden von [test-sqlbatch.py](../../../scripts/test-sqlbatch.py) zusätzlich validiert.
Der [Verhaltensvertrag](../../SQL-BATCH-API.md) beschreibt die Grenzen.
