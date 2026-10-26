# Lesender Monitor (API 0.20.0)

W-P27 ergänzt [API 0.20](api-v0.20.0.json) additiv zu API 0.12. V1 bis v12 bleiben
byteidentisch; Zwischenversionen werden nicht vorausgesetzt. Jede Route verlangt das
Bearer-Token und jede Messung die Verbindung mit tatsächlichen Datenbankrechten.
Keine Persistenz von Verbindungen, Messreihen oder SQL. Kein Ersatz für die
Serverstatus-/Variablen-/Prozesslistenfunktionen von W-P17 (API 0.15).

| Route unter `/api/v20/` | Inhalt |
| --- | --- |
| `GET capabilities` | Statische Grenzen und Einschränkungen pro Treiber und Aktion |
| `POST monitor/sample` | Genau eine Aufnahme ausgewählter numerischer Zähler |
| `POST monitor/analysis` | Aggregierte Abfragestatistik, ohne SQL-Texte |
| `POST monitor/advisor` | Aufnahme und deterministische Hinweise |
| `POST monitor/replication` | Ausschließlich lesender Replikationsstatus |
| `POST monitor/binary-logs` | Lesbare Binärlog-Kennungen, Größen und Verschlüsselungsstatus |
| `POST monitor/binary-events` | Ereignispositionen und geprüfte Typen; kein `Info`-Rohtext |
| `POST monitor/profiling` | Begrenztes EXPLAIN, ohne ANALYZE oder Abfrageausführung |
| `POST monitor/engines` | Engine-Fähigkeiten und numerische Details |

## Grenzen und Zustände

`limit`: 0 = 50, höchstens 200; `offset`: höchstens 10 000; `timeout_ms`: 0 =
10 000, höchstens 30 000 einschließlich Verbindungsaufbau. JSON-Anfrage höchstens
1 MiB, Antwort höchstens 8 MiB, einzelne Metadatenzelle höchstens 1 MiB. Listen
tragen `limit`, `offset`, `has_more`. `sampled_at` bezeichnet den Beginn der
Abfragen, keinen atomaren Snapshot; `duration_ms` misst den gesamten Aufruf.

`state=available` bedeutet erfolgreiche Abfrage, auch bei leerem Ergebnis.
`state=unavailable` bedeutet fehlende Tabellen-/Systemrechte (z. B. MySQL 1142/1143/1227, PostgreSQL 42501), deaktivierte oder fehlende Monitor-Quelle;
Anmeldefehler (MySQL 1044/1045, PostgreSQL 28xxx) und jeder Fehler einer vom Nutzer gelieferten
Profiling-Anweisung (fehlende Tabelle/Spalte, Syntax) sind echte Fehler (wie in der Query-API), nie `unavailable`;
`reason` erklärt die Einschränkung ohne Treiberrohfehler. Nicht unterstützte Aktionen
liefern 422 `unsupported_feature`. Ungültige Grenzen, SQL oder Logparameter:
400 `invalid_monitor`; ferner bestehende 400/401/408/413/415/422-Fehler.
Unbekannte JSON-Felder werden abgewiesen. DB-Rechte werden bei jedem Aufruf vom
Server durchgesetzt; PostgreSQL-Analyse/Replikation verlangen zusätzlich
`pg_read_all_stats` oder Superuser. Kein UI-Fähigkeitsflag ersetzt diese Prüfungen.

Zahlen einschließlich großer Zähler sind Dezimalstrings. Fehlende Felder bedeuten
unbekannt, niemals Null. Die Positivliste unterdrückt SQL, Verbindungsinformationen,
Passwörter, Host-/Dateipfade, Fehlertexte und frei konfigurierbare Namen. Unbekannte
Server-Enumwerte werden ausgelassen. Leere Datensätze sind deshalb möglich.

## Quellen und Treibergrenzen

MySQL/MariaDB: `sample`/`advisor` verwenden eine feste kleine Auswahl aus
`SHOW GLOBAL STATUS`. Der Client berechnet Raten aus zwei Aufnahmen; Neustarts und
Zählerrücksetzungen muss er selbst erkennen. Keine gespeicherten Vorwerte.
`analysis` liest `performance_schema.events_statements_summary_by_digest` mit
Digestkennung, Aufrufzahl, Gesamtsekunden und gesendeten Zeilen. Bei fehlender
Quelle oder leerer Statistik wird ausschließlich bei aktivem `slow_query_log` und
`log_output=TABLE` die gesamte lesbare `mysql.slow_log` aggregiert. Diese Quelle
liefert genau eine Summenzeile; kein General-Log und keine Dateizugriffe.
Aggregationen können bis zum Zeitlimit teuer sein. Kein Aktivieren von Logs.

PostgreSQL: `sample`/`advisor` lesen `pg_stat_database` der verbundenen Datenbank.
`analysis` liest eine installierte `pg_stat_statements`-Extension aus ihrem
Katalogschema: `queryid`, `calls`, `total_exec_time` (Millisekunden), `rows`.
Die Extension wird weder installiert noch geladen. Fehlende Installation oder
fehlendes Preloading wird als nicht verfügbar gemeldet. Keine SQL-Texte.

Replikation: MySQL verwendet `SHOW REPLICA STATUS`, MariaDB wird über die
Serverversion erkannt und verwendet `SHOW ALL SLAVES STATUS`. Bei Syntaxabweichung
werden die Alternativen und zuletzt `SHOW SLAVE STATUS` versucht. Mehrere Kanäle erscheinen
als getrennte Datensätze ohne geheime/frei konfigurierte Kanalnamen.
Nur Laufzustände, Verzögerung, numerische Positionen und Fehlernummern.
PostgreSQL vereinigt `pg_stat_replication` und `pg_stat_wal_receiver`, mit
`role=sender|receiver`, PID und geprüften Zuständen. Keine `conninfo`, Slots,
Passwörter oder Fehlertexte. Leere Listen sind kein Beweis gesunder Replikation.

Binärlog: nur MySQL/MariaDB. Ereignisanfragen verlangen `log` aus der lesbaren
`SHOW BINARY LOGS`-Liste (maximal erste 10 200 Kennungen), sichere ASCII-Kennung
mit numerischer Endung und `position` zwischen 0 und 2^53−1. Keine Pfade.
Seitenauswahl per Offset und Limit; Reihenfolge ist die des Servers. Logrotation,
Zurücksetzen und parallele Last können Seiten verändern. Analyse sortiert nach
Gesamtzeit und Kennung, MySQL zusätzlich Schema, PostgreSQL zusätzlich
Benutzer-/Datenbank-ID und Top-Level-Merkmal.
Keine Snapshot-/Vollständigkeitszusage bei veränderlicher Statistik.

Profiling: ausschließlich einfache `SELECT` mit unquotierten ASCII-Spaltennamen,
optional einfacher Tabelle (`schema.table`), numerischem WHERE-Vergleich und LIMIT.
Auch `SELECT 1` ist erlaubt. Keine Funktionen, CTEs, Unterabfragen, Kommentare,
Zeichenketten, Sessionanweisungen oder mehrere Statements. SQL höchstens 16 KiB.
EXPLAIN läuft in einer als lesend eröffneten Transaktion und wird zurückgerollt.
MySQL liefert nur Plan-ID, Zugriffstyp, Zeilenschätzung und Filterprozentsatz;
PostgreSQL nur geprüfte Knotentypen, Kosten, Zeilen-/Breitenschätzungen.
Kein Plan-Rohtext, keine tatsächlichen Ausführungszeiten, kein `SET PROFILING`.
Datenbankfunktionen in serverseitigen Objekten und Optimiererarbeit unterliegen
weiter den Vertrauens- und Ressourcenregeln der verbundenen Datenbank.

Engines: MySQL/MariaDB lesen `SHOW ENGINES` ohne Kommentarfeld und
`SHOW ENGINE INNODB STATUS`. Nur die ersten 16 KiB des Status werden nach fünf
festen numerischen Zeilen durchsucht (History-List-Länge, Bufferpool, freie,
Datenbank- und geänderte Seiten). `truncated=true` kennzeichnet abgeschnittenen
Status; Rohtexte bleiben vollständig unterdrückt. Details stehen beim InnoDB-Eintrag
der ausgewählten Seite, falls vorhanden. Fehlende Statusrechte werden im Grund
benannt. PostgreSQL liefert nur die Anzahl der Table-Access-Methods.
SQLite bietet ausschließlich `sample`/`engines` mit tatsächlicher SQLite-Version,
keine Zähler, Pfade, Profilierung oder sonstige Serveradministration.

## Advisor und ausdrückliche Nichtziele

Eigene Regeln `w-p27/threads_running > 32`, `w-p27/slow_queries > 0`,
`w-p27/deadlocks > 0`, `w-p27/temp_files > 100`. Jede Antwort nennt Regelkennung,
Quellansicht, Messwert, Schwelle und deutschen Hinweis. Schwellen sind heuristisch,
keine Diagnose oder automatische Konfigurationsänderung; kumulative Zähler gelten
seit dem jeweiligen Server-/Statistikstart. Identische Eingangswerte ergeben
identische Hinweise in fester Reihenfolge. Nicht vorhandene Metriken lösen nichts aus.

E9 bleibt verbindlich: kein Start/Stop/Quellwechsel, kein Fehlerüberspringen,
keine Kontoanlage, Ressourcen-/TLS-Änderung, Logkonfiguration oder PURGE.
Die schreibenden Teile von ADMIN-012, PRIV-008 und NEU-ADMIN-010/011 bleiben offen.
Kein vollständiger phpMyAdmin-Paritätsnachweis.
