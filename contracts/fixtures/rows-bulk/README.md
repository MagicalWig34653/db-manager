# W-P13-Vertragsfixtures

`*-placeholder.json` sind ausdrücklich synthetische Beispiele ohne Datenbanknachweis.
`*.invalid.json` verletzen absichtlich das zugehörige Schema.
`*.real-TestSQLite*.json` und `*.real-http.json` stammen aus tatsächlichen lokalen
SQLite-/HTTP-Tests; keine Netzwerk-Treiberbelege daraus ableiten. Die synthetischen
Tabellen und Werte enthalten keine echten Verbindungsdaten.

Reproduktion: `python3 scripts/test-rows-bulk.py --sqlite-only --keep contracts/fixtures/rows-bulk`.
Für einen frischen Verzeichnisinhalt alternativ ein leeres Verzeichnis im Worktree
an `--keep` übergeben. `python3 scripts/check-rows-bulk-contract.py` prüft Schema,
Additivität und eingefrorene Verträge bis API 0.12. Die Nummern 0.13 bis 0.18 sind
optional; vorhandene Dateien werden nicht verändert.

Ohne `--sqlite-only` startet `scripts/dbfix.py` nacheinander eigene MySQL-8.4-,
MariaDB-12.3- und PostgreSQL-18.6-Fixtures mit TLS verify-full. Die dabei aufgezeichneten
Antworten werden gegen denselben Vertrag validiert. Netzwerk-Fixtures sind erst nach
einem tatsächlich erfolgreichen Lauf als Nachweis zu übernehmen.
