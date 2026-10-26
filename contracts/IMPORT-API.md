# SQL- und CSV-Import – API 0.18

Status: implementierter Reviewkandidat W-P12, **keine Vertragsfreigabe**, kein Webpin.
Endgültig zugewiesene Nummer 0.18.0, additiv auf API 0.12. Bestehende v1–v12 bleiben
unverändert; v13–v17 können durch andere Pakete vorhanden sein.
[OpenAPI](api-v0.18.0.json), [Fixtures](fixtures/import/README.md),
[Entscheidungsprotokoll](../docs/decisions/ADR-W-P12-import.md).

## Routen und Ablauf

Alle Routen verlangen denselben Bearer-Token wie die bestehende API. Der Token schützt
auch Job-IDs; IDs allein erteilen keine Berechtigung. Es gelten tatsächliche DB-Rechte.

| Route | Verhalten |
| --- | --- |
| `GET /api/v18/capabilities` | Formate, Treibergrenzen und Ressourcenlimits |
| `POST /api/v18/imports/plan` | Multipart-CSV-Vorschau, 200 ohne Schreibwirkung |
| `POST /api/v18/imports/execute` | Multipart, explizite Bestätigung, 202 mit Job-ID und Location |
| `GET /api/v18/imports/{id}` | Fortschritt sowie nach Abschluss `result`, 200 |
| `POST /api/v18/imports/{id}/cancel` | Abbruch anfordern, Status 200; weiterhin bis `finished` abfragen |

Multipart enthält exakt `metadata` (JSON, höchstens 1 MiB) und `file` (Bytes, höchstens
64 MiB). Dateiname und MIME-Typ der Datei werden nicht als Pfad oder Formatauswahl
verwendet. Keine Base64-Hülle, Serverdateiauswahl, temporären Dateien, ZIP-Extraktion
oder externen Importprogramme. Format und Kompression stehen ausdrücklich in Metadaten.
Upload vor DB-Ausführung vollständig lesen, Uploadfrist 60 Sekunden. ClassUpload erlaubt
65 MiB einschließlich Metadaten und Multipart-Rahmen; jedes Einzellimit gilt zusätzlich.
Andere Routen behalten 1 MiB. Fehlende/fremde Teile, JSON-Felder und explizite JSON-NULL-
Werte sind ungültig. Für CSV-NULL ist eine Zeichenfolge in `csv.null` vorgesehen.

Der Ausführungsjob überlebt das Ende der Uploadantwort. Abbruch des Browseruploads
verhindert einen Start; Abbruch nach 202 benötigt die Cancelroute. Ein verloren gegangenes
202 kann einen bereits gestarteten Job hinterlassen: niemals blind erneut senden.
Zwei Slots begrenzen zusammen aktive Uploads, Vorschauen und Ausführungen je API-Instanz.
Maximal 16 aktive/abgeschlossene Jobs; abgeschlossene Ergebnisse verfallen nach 300 s.
Danach oder nach Neustart 404 `import_not_found`. Keine Wiederaufnahme, automatische
Wiederholung, Exactly-once- oder Mehrinstanz-Jobgarantie; Status benötigt dieselbe Instanz.

**Einmalige Pläne (CSV).** Ein bestätigter Plan (`plan_sha256`) wird beim Start der Ausführung
serverseitig (im Speicher, 1 h) als verbraucht markiert, auch wenn das Ergebnis `outcome_unknown`
bleibt. Dieselbe Ausführungsanfrage ein zweites Mal liefert `plan_mismatch` ("bereits ausgeführt");
ein neuer Plan-Aufruf mit gleichem Inhalt macht den Hash wieder nutzbar (bewusster zweiter Import).
Nach Serverneustart ist der Schutz zurückgesetzt. Speicher (gemessen, 61-MiB-CSV, zwei Slots parallel, einschließlich nicht eingesammelter Garbage):
Plan etwa 320 MiB, Ausführung etwa 500 MiB Spitze (etwa 250 MiB je Slot: Datei, Entpackung/Dekodierung,
Vorabprüfung des Plans, Jobdaten). Für 64-MiB-Dateien ist ein Containerlimit von mindestens 1 GiB
vorzusehen; das Repository setzt kein Limit. Ledger-Grenzen: höchstens 4096 Einträge (bei Vollstand wird
zuerst Abgelaufenes, sonst der älteste entfernt, danach ist dessen Replay wieder möglich) und 1 h Lebensdauer.
Die Vorabprüfung der Execute-Route gibt einen Hash nie frei und lehnt verbrauchte Pläne synchron mit 409
`plan_mismatch` ab.

## Metadaten und Grenzen

| Feld | Bedeutung |
| --- | --- |
| `connection` | Wie bisher, TLS standardmäßig `verify-full`, SQLite ausschließlich verwaltete Kennung |
| `format` | `sql` oder `csv` |
| `compression` | Fehlend/`none` oder `gzip`; keine automatische Erkennung, ZIP/BZip2 abgewiesen |
| `max_bytes` | Fehlend/0: 64 MiB, sonst kleinere positive Grenze; gilt für entpackte **und** dekodierte Bytes |
| `max_rows` | CSV: fehlend/0: 100.000, sonst kleinere positive Grenze; SQL zählt stattdessen Anweisungen |
| `timeout_ms` | Fehlend/0: 60.000; maximal 300.000, kein unbegrenztes Warten |
| `mode` | `stop` als Vorgabe; SQL alternativ `continue`, CSV alternativ `skip` |
| `confirm` | Ausführung: `{"action":"import_sql"}` bzw. `{"action":"import_csv"}`; Vorschau ohne Bestätigung |
| `expected_plan_sha256` | CSV-Ausführung: exakter Hash der Vorschau; SQL ohne Plan/Hash |
| `delimiter` | SQL: initialer Trenner, Vorgabe `;`, gleiche Regeln wie SQL-Stapel |

GZip wird vollständig einschließlich Prüfsumme vor DB-Ausführung geprüft. Beide
Bytegrenzen gelten unabhängig von Kompressionsverhältnis und gzip-Mitgliedszahl.
Keine Extraktion von Dateinamen/Pfaden. UTF-8-BOM wird entfernt, NUL/ungültige Kodierung
abgewiesen. Maximal 1024 CSV-Spalten, 1 MiB je Feld, 4 MiB Feldinhalte je Zeile;
Vorschau maximal 20 gültige Zeilen mit zusammen 256 KiB und maximal 100 Fehlerpositionen.
`errors_truncated` meldet weitere Fehler. Maximal 10.000 SQL-Anweisungen, 1 MiB je
Anweisung. SQL-Ergebnisdaten werden nicht übernommen, nur begrenzte Statementstatus.
Ein Resultset mit mehr als einer Zeile stoppt konservativ mit `result_too_large`, weil
abgebrochenes RETURNING auch die Wirkung begrenzen kann. Kein Anspruch auf Abfrageexport.

## SQL

Ausdrücklich bestätigtes Nutzer-SQL mit echten DB-Rechten, kein Lesemodus und keine
SQL-Sandbox außer den bestehenden SQLite-VFS-/Authorizergrenzen. Lexik wie
[SQL-Stapel](SQL-BATCH-API.md): Strings, Kommentare, PostgreSQL-Dollar-Quotes und
DELIMITER-Direktiven. Routinen-/Triggerkörper mit inneren Semikolons benötigen eigenen
DELIMITER. Der Lexer validiert keine vollständige SQL-Grammatik. Mehrdeutige innere
BEGIN/END-Fragmente werden vor Ausführung abgewiesen. SQL-Dateien müssen zum Treiber passen.

Alle Anweisungen werden vor dem ersten Schreiben getrennt und auf Importrestriktionen
geprüft. LOAD (auch LOCAL INFILE), COPY, USE, ATTACH/DETACH, beliebige SET-Anweisungen,
LOCK/UNLOCK, SAVEPOINT/RELEASE/ROLLBACK/START/ABORT und leere Klassifikation sind gesperrt.
Erlaubt sind einfache ausgeglichene `BEGIN`/`COMMIT` ohne Verschachtelung und die festen
W-P11-Header für UTF-8/UTC/SQL-Modus. Kein Einschleusen beliebiger Sitzungskontrolle.
Verbindung wird ausschließlich für diesen Import verwendet und anschließend geschlossen.

Die Ausführung liefert je Statement Index ab 1, direkte Änderungszahl soweit verfügbar,
Dauer, feste Fehlercodes und `completed`, `outcome_unknown`, `not_executed` oder `rolled_back`
(erfolgreich ausgeführt, aber durch ein abschließendes ROLLBACK auf SQLite/PostgreSQL mit
transaktionalem DDL wieder zurückgenommen).

**SQL-Import führt beliebiges SQL aus.** Es gibt bewusst keinen Plan, keinen Hash und keinen getippten
Namen; die Bestätigung `import_sql` genügt. Die erste Warnung der Antwort sagt das ausdrücklich und
zählt die Anweisungsklassen der validierten Datei (DDL, DROP/TRUNCATE, DELETE, UPDATE, INSERT,
sonstige). Entscheidung: SQL-Dateien (Dumps) sind nicht sinnvoll planbar; die UI muss die Warnung
vor der Ausführung anzeigen und die Datei benennen lassen.
SQL-/Treiberrohtexte, Ergebniszellen und Serverpfade erscheinen nicht im Bericht.
`stop` lässt den Rest unangetastet, `continue` versucht weitere Anweisungen bis zum Limit
oder Abbruch. Innerhalb eines fehlgeschlagenen PostgreSQL-Transaktionsrahmens bleiben
Folgeanweisungen gegebenenfalls ebenfalls fehlerhaft. Erfolgreiche Anweisungen können
bei späterem Rollback wieder entfallen; ein Statementabschluss ist keine Commitzusage.
Gesamtausgang bei jedem Fehler/Abbruch konservativ `outcome_unknown`, mit Teilergebnissen.
MySQL/MariaDB-DDL kann implizit committen, auch zwischen BEGIN und COMMIT. Nichttransaktionale
Tabellen, Routinen, Trigger und externe Effekte verhindern eine universelle Rücknahme.

W-P11-SQL-Dumps werden am erkennbaren Exportkopf erkannt und müssen mit
`-- DBMANAGER EXPORT COMPLETE` enden. Fehlende Schlussmarke verhindert jede Ausführung.
Fremde SQL-Dateien benötigen diese Marke nicht. Gleichdialektige Tabellendumps mit
normalen Sitzungsheadern und optionalem Transaktionsrahmen werden unterstützt; maximal
1 MiB je Statement. Export mit `disable_fk`, Serverdump mit USE und größere Einzelstatements
sind in dieser Stufe nicht importierbar. SQLite-FK-Pragmas bleiben durch die Sandbox
beschränkt. Kein vollständiger Backup-Roundtrip für sämtliche Objektklassen.

## CSV und Planbestätigung

`table` muss eine vorhandene reguläre transaktionale Tabelle bezeichnen; keine automatische
CREATE-Strukturerkennung. MySQL/MariaDB nur InnoDB; PostgreSQL ohne Regeln, Partitionierung,
Vererbung/Fremdtabellen; SQLite ohne virtuelle Tabellen. Generierte Spalten sind gesperrt,
Identity-Spalten dürfen zugeordnet werden, soweit die DB explizite Werte akzeptiert.
Nicht zugeordnete Spalten folgen ausschließlich den echten Defaults/Constraints der DB.

| `csv`-Option | Bedeutung |
| --- | --- |
| `columns` | Pflicht: geordnete eindeutige Zielspalten; jedes CSV-Feld entspricht genau einer Zielspalte |
| `separator` | Genau ein Unicodezeichen, Vorgabe Komma; kein CR/LF/NUL/Backslash oder Quotezeichen |
| `quote` | Genau ein Unicodezeichen, Vorgabe doppeltes Anführungszeichen; kein CR/LF/NUL/Backslash |
| `escape` | Vorgabe `double`: Quote verdoppeln; alternativ `backslash`: Quote/Backslash mit Backslash schützen |
| `charset` | Vorgabe UTF-8; alternativ `iso-8859-1` oder `windows-1252` |
| `header` | Wahr: erste Zeile überspringen, gleiche Feldzahl erforderlich; Zuordnung bleibt explizit |
| `null` | Optionaler exakter NULL-Text; ohne Option sind auch Leerfelder Text. Einfassung ändert die NULL-Erkennung nicht |

CRLF/LF/CR und eingebettete Zeilenumbrüche/Trenner in eingefassten Feldern werden gelesen.
Keine stillschweigende Reparatur fehlender Quotes oder falscher Feldzahl. Fehlerpositionen
verwenden Datensatznummer ab 1 **ohne Kopfzeile**, Spalte ab 1 (0 für ganze Zeile/Constraint).
Unterstützte Typteilmenge: ganze Zahlen, endliche Gleitkommazahlen, Dezimalzahlen,
Boolean `true`/`false`, Zeichen-/Texttypen. Andere Typen, etwa Binär-/Datum-/Geodatentypen,
werden als `unsupported_type` gemeldet. Typprüfung ist konservativ; Constraints,
serverseitige Genauigkeit und Trigger werden erst bei der Ausführung verbindlich geprüft.

Die Vorschau prüft alle Zeilen ohne INSERT und liefert parametrisierte Anweisung,
Warnungen, betroffene Objekte und `plan_sha256`. Ein zweiter Kommentar in der Anweisungsliste
bindet den SHA-256 über Datei, normalisierte Optionen, Zielverbindung ohne Passwort und
beobachtete Metadaten. Keine Werte werden in INSERT-SQL interpoliert. Hashabweichung bei
Datei-/Optionen-/Schemawechsel: 409 vor 202; die Engine prüft unter Sperre erneut.
Hash ist kein persistiertes Ticket, keine Berechtigungszusage und schützt nicht gegen
unbeobachtete Trigger-/Constraintänderungen oder ABA.

Ausführen: einzelne gebundene INSERTs als begrenzte Zeilenstapel in einer Dateitransaktion,
Savepoint pro gültiger Zeile. `stop` mit Typfehlern schreibt nichts; DB-Fehler rollen alle
direkten Zieländerungen zurück. `skip` überspringt Typ-/Constraint-/Eindeutigkeitsfehler
und committet nur die übrigen Zeilen. Keine REPLACE-/UPSERT-Löschung, SQLite explizit
INSERT OR ABORT. `completed` nach bestätigtem Commit, `rejected` nach bestätigter Ablehnung/
Rollback, `outcome_unknown` bei unbestätigtem Commit/Rollback. `inserted` ist bei Unknown
nur der Versuchszähler, keine Zusage persistierter Zeilen. Abbruch/Fehler vor dem COMMIT (auch
auf PostgreSQL nach Kontextabbruch) ergibt auf allen Engines `rejected` mit `inserted=0`. Keine automatische Wiederholung.
Externe Trigger-/Sequenzeffekte sind auch nach Rollback nicht allgemein rücknehmbar.

CSV aus W-P11 besitzt zusätzlich `_record` und eine `complete`-Kontrollzeile sowie
maskierte Sonderwerte. Diese Datei ist kein gewöhnliches CSV-Eingabeprofil: Client muss
Abschluss prüfen und das Exportprofil ausdrücklich in gewöhnliches CSV umwandeln.
W-P12 rät weder Kontrollspalten noch Binärpräfixe automatisch. Erweiterte Profile bleiben
W-P28 vorbehalten; kein stilles Verschlucken von Kontroll-/Nutzzeilen.

## Fehler und Betriebsnachweise

Vor Start: 400 `invalid_import`/`confirmation_mismatch`, 409 `plan_mismatch`, 413
`request_too_large`/`import_too_large`, 415, 422 bei nicht verfügbaren Tabellen/Typen/
Aktionen oder Engine, 503 `import_busy`. Nach 202 stehen Enginefehler im Jobresultat als
fester `reason` und `outcome`; der HTTP-Erfolg bedeutet ausschließlich gestarteten Job.
Abbruchanforderung setzt `cancel_requested`, ist keine Rollback- oder Abschlussbestätigung.

SQLite-Tests und echte Antworten sind im [Handoff](../docs/handoffs/W-P12-backend.md)
belegt. MySQL-/MariaDB-/PostgreSQL-Platzhalter sind ausdrücklich keine Livebelege.
Maximalupload-/Parallelbelastung, echte Verbindungsabbrüche, unabhängiger Sicherheitsreview,
Webintegration, Compose/Swarm und Mehrinstanzrouting sind eigene noch offene Gates.
