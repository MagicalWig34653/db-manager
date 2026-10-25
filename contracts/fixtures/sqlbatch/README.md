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

Ergänzung Integration (Welle 2): die Dateien `*.real-*.json` sind echte Antworten aus `scripts/test-sqlbatch.py --keep`
gegen SQLite, MySQL 8.4, MariaDB 12.3 und PostgreSQL 18.6 (TLS verify-full, Wegwerf-Container nacheinander), 25.10.2026.
Die `*-placeholder.json`-Dateien bleiben als Schema-Beispiele; Nachweise sind die `real-*`-Dateien.
