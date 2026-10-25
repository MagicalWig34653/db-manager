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

Ergänzung Integration (Welle 2): die Dateien `real-mysql-*`, `real-mariadb-*` und `real-postgres-*` stammen aus
einem echten Lauf von `scripts/test-columns.py` gegen MySQL 8.4, MariaDB 12.3 und PostgreSQL 18.6
(TLS verify-full, Wegwerf-Container nacheinander), 25.10.2026. Die handgeschriebenen Ergebnisplatzhalter
bleiben als Schema-Beispiele erhalten; Nachweise sind die `real-*`-Dateien.
