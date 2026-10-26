# ADR W-P27: Lesende, zustandslose Monitordaten

Datum: 25.10.2026. Status: Implementierungsvorschlag auf `feat/w-p27-backend`;
unabhängiger Review und Leadintegration offen.

## Kontext

W-P27 verlangt Messwerte, Abfrageanalyse, Advisor, Replikations- und Binärlogstatus,
Profiling sowie Engine-Details. E9 beschränkt Replikation ausdrücklich auf Lesen.
Rohdaten aus Replikation, Logs und InnoDB-Status können Passwörter, SQL und
Dateipfade enthalten. W-P17 verwaltet bereits Serverstatus, Variablen und Prozesse.

## Umsetzungsvorschlag

API 0.20 ergänzt API 0.12 ohne Änderung bestehender Vertragsbytes. Je Aufruf genau
eine Messung, keine serverseitige Messhistorie; Raten berechnet der Client.
Datenquellen werden unter tatsächlichen Verbindungsrechten abgefragt, fehlende
Quellen/Rechte als nicht verfügbar gemeldet. PostgreSQL verlangt für globale
Abfrage- und Replikationsstatistik zusätzlich Statistikleserechte.

Antworten verwenden eine Positivliste numerischer und enumerierter Felder.
Abfrageanalyse liefert Kennungen und Aggregate ohne SQL-Texte. Ereignisse liefern
Positionen und Typen ohne Inhalt. InnoDB-Status wird auf 16 KiB begrenzt und nur
nach fünf numerischen Kennzahlen durchsucht; kein Rohtext verlässt den Adapter.
Advisor-Regeln sind eigene feste Heuristiken mit expliziter Quelle und Schwelle.

Profiling verwendet ausschließlich EXPLAIN ohne ANALYZE in einer lesenden
Transaktion. Eine kleine SELECT-Grammatik vermeidet freie Funktionsaufrufe,
Sitzungsbefehle und mehrere Anweisungen. Planantworten enthalten nur geprüfte
Struktur-/Schätzwerte. SQLite bleibt bei grundlegender Versionsinformation.
Details, Grenzen und Nichtziele: [Monitor-Vertrag](../../contracts/MONITOR-API.md).

## Folgen und Alternativen

Keine Konten-/TLS-/Ressourcenänderungen, Logaktivierung, Replikationssteuerung,
Fehlerüberspringen oder PURGE. Keine vollständige phpMyAdmin-Parität.
Unbeschränkte rohe Log-/Plan-/Statusausgabe wurde wegen möglicher Geheimnisse
verworfen. Freies SELECT würde auch Funktionen und externe Effekte zulassen;
die eingeschränkte Grammatik bedeutet bewusst weniger Analysekomfort.
Optimierer und serverseitige Objekte bleiben Teil der vertrauten Datenbank.

Keine neuen Abhängigkeiten oder Änderungen an SQLite-Sandbox, gemeinsamen
SQL-Werkzeugen, zentralem Status, Webbuild oder Vertragsindex.
