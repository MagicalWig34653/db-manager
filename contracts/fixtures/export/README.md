# Exportfixtures W-P11

Ausschließlich synthetische Beispiele, keine Live-DB-Aufzeichnungen.
`PLACEHOLDER_USER`, `PLACEHOLDER_NOT_A_SECRET` und `PLACEHOLDER_INVALID_PATH` sind
absichtlich markierte Platzhalter; sie sind keine verwendbaren Zugangsdaten/Pfade.
Namen mit `synthetic` bezeichnen Wegwerfobjekte.

`<Schema>.<Fall>.json` muss gültig sein; `*.invalid.json` muss abgelehnt werden.
Die ResultSnapshot-Fälle beschreiben das vorhandene lokale F6-Format; ExportDocument
beschreibt den getrennten Streamingexport. `sample.sql`, `sample.csv` und
`sample-excel.csv` illustrieren vollständige Dateirahmen, keine Produkt-Testaufzeichnung.
GZip wird im HTTP-Test tatsächlich erzeugt, dekomprimiert und auf Abschluss geprüft.

Prüfung: `python3 scripts/check-export-contract.py`. Der Wächter prüft unveränderte
Vertragsbytes bis API 0.7, Additivität, Referenzen, strikte neue Objekte, lokale
JSON-Schemabedingungen, gemeinsames Formatschema und negative Fixtures sowie CSV-/SQL-
Abschlüsse. Echte Produktbytes und SQLite-Restore prüft `python3 scripts/test-export.py`;
`--live` verwendet eigene `dbfix.py`-Fixtures für MySQL/MariaDB/PostgreSQL.
