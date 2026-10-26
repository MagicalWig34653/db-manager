# Import-Fixtures (API 0.18)

`*.real-*.json` sind tatsächliche Antworten aus lokalen SQLite-/HTTP-Tests mit
synthetischen Wegwerfdaten. Erzeugung: `DBM_IMPORT_DUMP=contracts/fixtures/import`
mit `go test` oder `python3 scripts/test-import.py --keep contracts/fixtures/import`.
Die Dateinamen enthalten den geprüften Komponentennamen vor dem ersten Punkt.

`*.placeholder-{mysql,mariadb,postgres}.valid.json` sind ausdrücklich synthetische
Schema-Platzhalter und **keine** erfolgreichen Engineproben. Andere `valid`-/`invalid`-
Dateien sind synthetische Vertragsfälle. Invalid-Dateien müssen der Wächterprüfung
widersprechen. Keine echten Zugangsdaten oder produktiven Daten.

Der Vier-DB-Lauf verwendet `scripts/dbfix.py` und erzeugt zusätzliche tatsächliche
Antworten mit `--live --keep <Verzeichnis>`. Er prüft Nachlesen, Konflikte, Rechte,
W-P11-SQL-Restore und die Abweisung nichttransaktionaler Tabellen. Netzwerk- und
Dockerläufe bleiben bis zur dokumentierten Ausführung offen.
