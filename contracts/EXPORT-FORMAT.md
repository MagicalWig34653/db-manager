# Gemeinsames Exportformat 1

Status: vorläufiger C-Export1-/W-P11-Vertrag, noch unabhängig freizugeben. Gilt als
Formatvorgabe für Web und spätere Appleclients, keine Behauptung einer implementierten
Applefunktion. [Striktes JSON-Schema](export-format-v1.schema.json),
[API 0.12](EXPORT-API.md), [synthetische Fixtures](fixtures/export/README.md).
Formatversion und API-Version sind getrennte Versionsräume.

## Gemeinsame Zellen und lokaler Ergebnissnapshot

Zellen folgen dem unveränderten API-Cell-Modell: `kind` ist `null`, `text`, `integer`,
`decimal`, `float`, `boolean`, `binary` oder `timestamp`. NULL hat **kein** `value`, jede
andere Zelle hat einen Stringwert, auch der leere Text. Zahlen bleiben Zeichenketten,
Binärdaten sind Base64. Spalten und Zeilen sind geordnete Arrays; doppelte Spaltennamen
bleiben erhalten. Jede Zeile muss genau zur Anzahl der Spalten passen. Ein Parser muss
zusätzlich zum Schema diese tabellenübergreifende Längenbedingung prüfen.

Das bestehende F6-Profil `ResultSnapshot` wird unverändert beschrieben:
`schema_version: 1`, `scope`, `truncated`, `columns: [{name,type}]`, `rows`.
`scope` ist `{kind:"sql_result"}` oder
`{kind:"table_page",offset,page_size,has_more}`. Dies ist ausschließlich der geladene
Snapshot, höchstens 8 MiB, ohne weitere DB-Abfrage. `truncated` und `has_more` sind
keine Aussage über einen vollständigen Datenbankexport. Keine Tokens, SQL-Texte,
Verbindungen, Profilnamen oder Dateipfade in diesem Profil.

Die vorhandenen lokalen F6-CSV-Profile bleiben ebenfalls unverändert: `csv-raw` ohne
Formelschutz, `csv-spreadsheet` mit Apostrophpräfix je Feld; sie verweigern ungeeignete
Steuerzeichen und empfehlen JSON. Die neuen Streamprofile sind erkennbar getrennt und
ersetzen diese bestehenden Downloads nicht. Webassets und Weblock werden nicht geändert.

## JSON-Streamprofil

`ExportDocument` verwendet `format:"dbmanager-export"`, `version:1`,
`snapshot:"per_database"` (Abfragen: `read_only_best_effort`), `tables` und den **zuletzt geschriebenen** Wert
`complete:true`. Eine Tabelle enthält `database`, `table:{schema?,name}`,
`columns:[{name,database_type}]` und `rows`. Abfrageergebnisse heißen `query` und erhalten
keinen SQL-Text. DB-/Tabellennamen sind gewählte Datenreferenzen, keine Verbindungsdaten.

Ein abgebrochener Stream bleibt ohne gültigen Abschluss. Erst vollständiges JSON mit
`complete:true`, passendem Format, Version und Zeilenbreiten akzeptieren. Numerische
Texte dürfen nicht stillschweigend in JavaScript-Number konvertiert werden.
UTF-8 (optional BOM) ist die einzige unterstützte Kodierung. JSON erhält alle gültigen
Unicodezeichen einschließlich Steuerzeichen durch JSON-Escaping. Ein JSON-Leser muss
eine angebotene BOM vor der Dekodierung behandeln.

## CSV-Streamprofile

CSV/Excel-CSV exportieren eine Ergebnistabelle; SQL-Typinformationen gehören in JSON.
Jedes Feld wird mit `"` eingefasst, innere `"` werden verdoppelt. Trenner sind Komma,
Semikolon oder Tab; Zeilenenden LF oder CRLF. Innere CR/LF bleiben in den Ausgabebytes
unverändert. CSV-Leser normalisieren teils CRLF; für exakte Textbytes JSON verwenden.

Die erste Spalte heißt `_record`. Datenzeilen tragen hier `row`; der **letzte Satz**
trägt `complete` und ansonsten leere Felder. Er hat dieselbe Breite wie Header und
Daten. Ohne diesen Satz ist der Stream unvollständig. Diese Kontrollspalte und der
Abschlusssatz gehören zum Streamprofil und sind vor dem Datenimport zu entfernen.
Die anderen Headerfelder sind die geordneten Spaltennamen. Dadurch kann der Browser
Vollständigkeit ohne HTTP-Trailer feststellen. Leere Tabellen haben Header und Abschluss.

NULL ist `\N`; leere Zeichenkette bleibt leer. Binärdaten sind `base64:<Base64>`.
Textwerte, die mit `\` oder `base64:` anfangen, erhalten einen zusätzlichen Backslash.
Beim Lesen zuerst den Escape-Backslash entfernen, andernfalls NULL-/Binärmarker erkennen.
Alle anderen Werte sind Zeichenketten; CSV bewahrt keine eigenständigen Zahlen-/Zeittypen.

`csv` lässt Formeln absichtlich als Rohtext stehen; nur in geeigneten Programmen öffnen.
`excel-csv` setzt vor **jedes** Header-/Datenfeld einschließlich Kontrollwerten ein
Apostroph, nutzt standardmäßig Semikolon, UTF-8-BOM und CRLF. Ungeeignete Steuerzeichen
werden durch U+FFFD ersetzt; dieses Komfortprofil ist ausdrücklich nicht verlustfrei.
Das Präfix ist keine Zusage für jede Tabellenkalkulation oder deren Importoptionen.
Vor der Markerinterpretation genau dieses eine Profilpräfix entfernen. Leere Felder
im Abschlusssatz bleiben leer. Für unveränderte Werte das JSON-Profil verwenden.

## SQL-Streamprofil

Ein fester Header nennt ausschließlich Format und technische Grenzen; keine Zeitpunkte,
Hosts, Benutzer, Passwörter, Tokens oder Serverpfade. Reguläre Tabellen werden mit
CREATE und wahlweise DROP/IF NOT EXISTS ausgegeben, anschließend vollständige INSERTs
mit expliziten Spaltennamen oder begrenzte Mehrzeilen-INSERTs. Generierte Spalten werden
nicht eingefügt; Identityspalten werden erhalten (PostgreSQL `OVERRIDING SYSTEM VALUE`).
Die maximale INSERT-Länge umfasst Kopf, Werte und Abschluss; zu große Einzelzeilen
werden nicht geteilt oder stillschweigend gekürzt.

SQL-NULL ist `NULL`, numerische Werte werden ausschließlich nach Prüfung einer festen
Zahlengrammatik geschrieben. Ganze Floatwerte erhalten `.0`, damit SQLite die
Speicherklasse behält. Nichtendliche Zahlen und unbekannte Zellarten werden abgewiesen.
Binärwerte sind Hexliterale, bei PostgreSQL `decode(...,'hex')`. Text wird dialektgenau
zitiert: Quoteverdopplung, PostgreSQL E-Strings mit Backslash-Escaping, MySQL
`CONVERT(X'...' USING utf8mb4)` unabhängig von NO_BACKSLASH_ESCAPES, SQLite bei NUL
`CAST(X'...' AS TEXT)`. Ungültiges UTF-8 in SQLite-TEXT wird abgewiesen.

Netzwerksitzungen lesen TIMESTAMP in UTC; PostgreSQL-Zeitwerte mit Zeitzone werden in
UTC ausgegeben. Naive DATE/TIME/TIMESTAMP-Werte erhalten keine erfundene Zeitzone.
SQLite-TEXT bleibt TEXT und wird nicht als Zeitwert umgedeutet. Dezimalwerte werden
nicht durch Float64 umgerechnet; SQLite bewahrt seine tatsächliche Speicherklasse,
keine Präzision, die bereits durch SQLite-Affinität verloren ging.

Der vollständige Dump endet exakt mit der separaten Zeile
`-- DBMANAGER EXPORT COMPLETE` und einem Zeilenumbruch, erst nach COMMIT/FK-Rücksetzung.
Ein Vorkommen im Dateninhalt ist kein Abschluss. Vor dem Ausführen Abschluss und
Transportende prüfen. GZip muss zusätzlich einschließlich Prüfsumme vollständig lesbar
sein. Bei Limits/Fehlern wird kein erfolgreicher Abschluss nachgeschoben.

Dies ist kein universelles Backupprogramm. Die [Snapshot-/Treuegrenzen je Treiber](EXPORT-API.md)
sind Teil dieses Formats. Fremdschlüssel, Abhängigkeiten, Sequenzen, Trigger, Views,
Routinen, Rechte, Collations und unterschiedliche Serverversionen können zusätzliche
Wiederherstellungsarbeit verlangen. Die konkrete belegte Teilmenge steht im
[Handoff](../docs/handoffs/W-P11-backend.md).
