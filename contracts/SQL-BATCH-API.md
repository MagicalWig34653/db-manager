# SQL-Stapel, EXPLAIN und DML-Simulation

W-P15-Backendkandidat, API **0.12.0** (bei der Welle-2-Integration endgültig vergeben), kein freigegebener Webpin.
[Schema](api-v0.12.0.json), [Fixtures](fixtures/sqlbatch/README.md),
[Prüfstand](../docs/handoffs/W-P15-backend.md).
Bestehende API-Definitionen und Dateien 0.1 bis 0.7 bleiben unverändert.

## Routen und Zustimmung

Alle Routen erfordern Bearer-Authentifizierung und liefern `Cache-Control: no-store`.
`GET /api/v12/capabilities` nennt Grenzen, Adapterverfügbarkeit und Fähigkeiten je
Treiber (`mysql` umfasst MariaDB). POST-Routen:

| Route | Verhalten |
| --- | --- |
| `/api/v12/sql/batch` | Stapel mit Ergebnissen je Anweisung |
| `/api/v12/sql/explain` | Genau eine Anweisung, serverseitiges EXPLAIN-Präfix |
| `/api/v12/sql/simulate` | Genau eine SQLite-DML-Anweisung in Rollback-Transaktion |

Gemeinsamer Request `SQLBatchRequest`: `connection`, `sql`, ausdrücklich `confirm: true`;
optional `delimiter`, `mode` (`stop`, Standard; oder `continue`), `max_rows`, `timeout_ms`.
Die Zustimmung ist wie die bestehende Schreibzustimmung ein boolesches Pflichtfeld;
alle neuen Ausführungen verlangen sie, auch SELECT und EXPLAIN. Der Client muss vor
Bestätigung den vollständigen Nutzer-SQL-Text und dessen Kontext anzeigen. Es gibt keine
Umdeutung zu strukturiertem SQL oder einen serverseitigen Schutz vor destruktivem Nutzer-SQL.
Dies verwendet die tatsächlichen Rechte der Verbindung, einschließlich Routinen und
Serverfunktionen. Ein Lesemodus wird nicht versprochen. E5/E6/E9 erweitern diese Rechte
nicht; strukturierte Planaktionen anderer Pakete behalten ihre eigenen Bestätigungen.

Unbestätigte/ungültige Grenzen, Modi, leere Stapel und lexikalische Fehler: HTTP400
`invalid_sqlbatch`, vor Öffnen/Ausführung. Nicht unterstützte Aktionen: HTTP422
`sql_not_supported` oder `simulation_not_supported`. Fehlende Adapter: HTTP422
`unsupported_feature`. Authentifizierung, Medientyp und Größenprüfung erfolgen wie bisher.
Unbekannte JSON-Felder und explizite Nullwerte werden abgewiesen. Anfrage maximal 1 MiB;
die SQL-Bytegrenze wird zusätzlich im Core geprüft.

## Trennung und Stapelsemantik

`internal/sqlsplit` trennt höchstens 100 Statements aus höchstens 1 MiB UTF-8-SQL.
Standardtrenner `;`; ein eigener Trenner hat 1 bis 16 ASCII-Satzzeichen aus
`;!%&*+,-./:<=>?@^|~$`, ohne Präfix `--` oder `/*`. Leere Trennerwahl bedeutet Standard.
`DELIMITER <trenner>` ist eine eigene Zeile zwischen Anweisungen; die Direktive wird
nicht an die DB gesendet. Für SQLite-Triggerkörper mit inneren Semikolons muss ebenfalls
ein eigener Trenner gesetzt werden. Keine implizite SQL-Grammatikerkennung für BEGIN/END.

Beachtet werden einfache/doppelte Quotes mit Verdoppelung, MySQL-/SQLite-Backticks,
SQLite-Klammeridentifier, Block-/Zeilenkommentare, MySQL-`#` und dessen Whitespace-Regel
nach `--`, verschachtelte PostgreSQL-Blockkommentare, Dollar-Quoting einschließlich
Unicode-Tags und PostgreSQL-E-Strings. MySQL nimmt den normalen Backslash-Escape-Modus,
PostgreSQL normale standardkonforme Strings an. Abweichende serverseitige SQL-Modi sind
nicht automatisch ermittelt. Eine getrennte PostgreSQL-Anweisung wird zusätzlich
vorbereitet; MySQL bleibt ohne `MultiStatements`, SQLite verwendet native Prepare/Tail-
Prüfung. Der Splitter ist kein Linter oder Sicherheitsparser. Lexerfehler verhindern
den gesamten Stapel vor dessen erstem Effekt. Lexikalisch gültige, aber syntaktisch
falsche SQL-Anweisungen scheitern erst an ihrer Position in der DB.

Eine physische Verbindung und ein gemeinsames Zeitbudget je Request. Keine atomare
Stapeltransaktion. Explizite Transaktions-/Sitzungskontrolle (`BEGIN`, `START`, `COMMIT`,
`END`, `ROLLBACK`, `SAVEPOINT`, `RELEASE`, `SET`) und unbekannte/exekutive Kommentarpräfixe
werden vor dem gesamten Stapel abgewiesen. Routinen und Funktionen können weiterhin
Nebenwirkungen und eigene Transaktionen haben; diese Regel ist keine SQL-Sandbox.

Jeder Eintrag enthält 1-basierten `index`, `outcome`, `results`, `warnings`, `duration_ms`,
optional `rows_affected`, `error_code`, `server_code`. SQL wird nicht zurückgespiegelt.

- `completed`: Adapter hat die Ausführung und gelesenen Ergebnisse bestätigt; kein
  Exactly-once-/dauerhafter Commitnachweis für beliebige DB-Programme.
- `outcome_unknown`: Ausführungs-/Ergebnisfehler oder abgeschnittene Ergebnisse; vorherige
  oder partielle Effekte können bestehen. Kein automatischer Wiederholungsversuch.
- `not_executed`: Diese Anweisung wurde vom Adapter nicht gestartet.

HTTP200 kann Statementfehler enthalten. `stop` lässt nach dem ersten solchen Fehler
alle folgenden Anweisungen unangetastet; `continue` versucht nach DB-Fehlern weitere
Anweisungen. Abbruch, Zeit-/Größenlimit und abgeschnittene Resultsets stoppen immer.
Eine unterbrochene HTTP-Antwort macht alle nicht erhaltenen Ergebnisse unsicher.
Abbruch des HTTP-Kontexts gibt keine Rollbackzusage.

Zeilen verwenden den bestehenden präzisen Cell-Codec; SQLite behält native Speicherklassen.
Höchstens 16 Resultsets je Anweisung, 1.000 Zeilen standardmäßig / höchstens 10.000 je
Resultset, Standard 10 s / höchstens 60 s für den gesamten Stapel, 1 MiB je Zelle und
8 MiB tatsächliche JSON-Antwort einschließlich Framing. `0` bedeutet den jeweiligen
Standard. Bei aggregierter Überschreitung wird das aktuelle Ergebnis verworfen,
`result_too_large` gemeldet und der Rest nicht gestartet; frühere Ergebnisse bleiben.
Ein Zeilenlimit liefert `truncated: true` und `outcome_unknown`; weitere Sets können fehlen.
Die vier globalen Engineslots und sämtliche SQLite-VFS-/Authorizergrenzen bleiben bestehen.
Wire-Pufferung und DB-seitige Arbeiten können Ressourcen schon vor der Ausgabeprüfung verbrauchen.

`rows_affected` ist eine Dezimalzeichenfolge, wenn der Adapter einen belastbaren direkten
DML-Zähler hat. SQLite liefert `Changes()` bei INSERT/UPDATE/DELETE/REPLACE. Netzwerk-Exec
liefert `RowsAffected`; bei RETURNING, WITH oder CALL im Zeilenpfad fehlt der Zähler.
Fehlen bedeutet unbekannt, niemals null betroffene Zeilen. Trigger-/Kaskadenzeilen sind
nicht mitgezählt; MySQL zählt geänderte, nicht unbedingt alle passenden Zeilen.
`CALL` kann mehrere Sets zurückgeben. PostgreSQL-Stapel werden als einzelne vorbereitete
Kommandos ausgeführt; kein Mehrfachkommando im Simple-Query-Protokoll.

MySQL/MariaDB: bis 100 `SHOW WARNINGS`-Codes auf derselben Verbindung, feste deutsche
Meldung statt Rohtext. Das ist eine begrenzte Liste; Rohtexte könnten SQL, Werte und
Pfade enthalten. PostgreSQL-Hinweise sind nicht erfasst (`notices_unavailable`); SQLite
besitzt hier keinen Serverwarnungskanal. Fehlercodes sind öffentliche Codes, optional
numerischer MySQL-/SQLite-Code oder validierter PostgreSQL-SQLSTATE. Keine rohen
Treiberfehlermeldungen, DSNs, Credentials oder Pfade.

## EXPLAIN und Simulation

EXPLAIN akzeptiert nur SELECT, WITH, VALUES, INSERT, UPDATE, DELETE als führendes
Schlüsselwort; die DB entscheidet die konkrete Syntaxunterstützung. SQLite erhält
`EXPLAIN QUERY PLAN`, MySQL/MariaDB/PostgreSQL `EXPLAIN`. Es gibt kein `analyze`-Feld;
`EXPLAIN ANALYZE` als Eingabe wird hier abgewiesen. Wer solches Nutzer-SQL über den
bestätigten Stapel ausführt, nimmt dessen tatsächliche Ausführung bewusst in Kauf.
Auch einfache Planung kann DB-Funktionen/Planungshooks berühren; keine allgemeine
Nebenwirkungsfreiheit. Die Antwort enthält die dialektspezifischen Zeilen ohne
Umdeutung in einen universellen Plan.

Simulation ist eine echte, begrenzte DML-Ausführung, kein statisches Versprechen.
Nur SQLite INSERT/UPDATE/DELETE ohne RETURNING, WITH, virtuelle Tabellen und mehrere
Anweisungen werden unterstützt. Lokale Transaktion, direkte Zeilenzählung, expliziter
Rollback vor Antwort. Erfolg heißt `rolled_back` mit optionalem Zähler, fehlgeschlagene
DML bei bestätigtem Rollback ebenfalls `rolled_back` ohne Zähler mit Fehlerwarnung.
Bei nicht bestätigtem Rollback heißt der Ausgang `outcome_unknown`. Rollback wird auch
bei Fehler/Abbruch versucht, niemals Commit. Lokale Triggeränderungen werden mitgerollt;
keine Zählung ihrer indirekten Effekte. Vorschau und spätere Ausführung sind getrennte
Zeitpunkte und können verschiedene Ergebnisse liefern. Keine externen SQLite-Funktionen
oder virtuelle Speicher werden für diese Funktion freigeschaltet.

MySQL/MariaDB/PostgreSQL melden Simulation derzeit `unsupported` und weisen sie vor
DB-Zugriff ab. Freies DML kann nichttransaktionale Engines, Trigger, Rewrite-Regeln,
FDWs/externe Funktionen oder nicht rücknehmbare Sequenz-/Auto-ID-Effekte berühren.
Auch Rollback ist keine universelle Rücknahme externer Effekte. Eine künftig eng
metadatengeprüfte Simulation ist eine separate Vertragserweiterung; kein stiller
Fallback auf tatsächliche ungesicherte Ausführung.

## Hinweise und Grenzen (Review Welle 2)

* **Zustimmung:** `confirm` gilt global für die Anfrage; es gibt keinen Lesemodus und keine Klassifikation je Anweisung. Der Stapel führt jede Anweisung
  mit den echten Rechten der Verbindung aus, auch `EXPLAIN ANALYZE <DML>` (PostgreSQL führt das `DELETE` aus). Das Capability-Feld `explain_analyze=false`
  gilt nur für die Route `explain`, die `ANALYZE`, `FORMAT`, Klammerformen und `/*!` ablehnt und nie Nebenwirkungen hat. Clients zeigen die Schreibwarnung
  unabhängig vom Anweisungstyp.
* **`BEGIN ... END`:** Der Splitter trennt an jedem `;`. SQLite-Trigger und PostgreSQL `BEGIN ATOMIC` mit inneren Semikolons werden deshalb vor jeder Ausführung
  mit 422 `sql_not_supported` abgelehnt (fail-closed). Abhilfe: `DELIMITER //` ... `//` ... `DELIMITER ;` (auch für SQLite und PostgreSQL).
* **Sitzungskontrolle:** `BEGIN`, `START`, `COMMIT`, `END`, `ROLLBACK`, `ABORT`, `SAVEPOINT`, `RELEASE`, `SET`, `LOCK`, `UNLOCK` werden abgelehnt (jede Anfrage hat eine eigene Verbindung im Autocommit).
* **MySQL/MariaDB:** `ANALYZE`, `OPTIMIZE`, `CHECK`, `REPAIR` und `CHECKSUM TABLE` liefern ihre Ergebniszeilen.
* **Simulation:** Läuft die Simulation in das Zeitlimit, ist die Transaktion zurückgerollt (`rolled_back`); `outcome_unknown` mit `rollback_unknown`
  bleibt für einen tatsächlich nicht bestätigten Rollback.
