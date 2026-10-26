# W-P19-Vertragsfixtures

Alle Objekte, Datenbanken, Benutzer, Passwörter und Körper sind synthetische
Platzhalter, keine Verbindungsdaten. Nullhashes sind ausdrücklich Platzhalter;
Clients müssen den tatsächlichen Hash aus der Planantwort übernehmen.
Dateien mit `.invalid.json` sind absichtlich ungültig.

`*.sqlite-observed.json` werden aus tatsächlichen lokalen SQLite-/HTTP-Antworten
übernommen. Ihre Namen sind synthetische Testnamen. MySQL-/MariaDB-/PostgreSQL-
Anfragen und mit `placeholder` gekennzeichnete Antworten sind Beispiele, keine
als beobachtet ausgegebenen Serverantworten.
Der Livetest erzeugt und validiert echte Antworten erst in einer Dockerumgebung.

Ergänzung Integration (Welle 3): Dateien `*.real-*.json` sind echte Antworten aus `scripts/test-programs.py --keep` gegen SQLite, MySQL 8.4,
MariaDB 12.3 und PostgreSQL 18.6 (TLS verify-full, Wegwerf-Container nacheinander), 26.10.2026; Platzhalter bleiben als Schema-Beispiele.
