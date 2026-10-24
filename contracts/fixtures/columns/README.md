# Vertragsfixtures für Spaltenänderungen

`real-sqlite-*`: sechs echte API-/Engine-Antworten aus
`GOCACHE=/private/tmp/dbmanager-w-p08-gocache python3 -B scripts/test-columns.py --sqlite-only`,
24.10.2026, SQLite 3.53.4 (ncruces/go-sqlite3 0.35.6). Enthalten sind die statische
HTTP-Fähigkeitsantwort, zwei Vorschauen, zwei bestätigte Ergebnisse und eine
abgewiesene PK-Entfernung. Ausschließlich synthetische Wegwerfdaten.

`handwritten-*`: **handgeschriebene Vertragsplatzhalter**, keine Engine-Nachweise.
Das betrifft sämtliche Anfrage-/Negativfixtures sowie die drei Ergebnisplatzhalter
`completed`, `rejected`, `outcome_unknown`. Sie dienen dem Schemawächter.
MySQL-/MariaDB-/PostgreSQL-Antwortfixtures sind noch nicht erzeugt: Docker ist in
der Ausführungssandbox gesperrt. Sie dürfen erst nach echtem Lauf ergänzt werden.

`python3 -B scripts/test-columns.py --keep <temporäres-Verzeichnis>` startet über
`dbfix.py` jeweils nur einen eigenen Wegwerf-Container und prüft echte Antworten
gegen das v9-Schema. `--sqlite-only` benötigt weder Docker noch Netzwerk. Der
Wächter `python3 -B scripts/check-columns-contract.py` prüft alle JSON-Dateien;
`*.invalid.json` muss abgewiesen werden.
