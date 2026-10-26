# Erweiterter Export und Dump-Details – API 0.21

Implementierter Reviewkandidat W-P28, keine Freigabe oder Veröffentlichung. Der
[OpenAPI-Vertrag](api-v0.21.0.json) erweitert API 0.12 additiv um
`GET /api/v21/capabilities` und `POST /api/v21/export`. Bestehende Vertragsbytes,
insbesondere API 0.11 und das JSON-Formatschema 1, bleiben unverändert.

Authentisierung, Standardklasse (1 MiB), Dateiname, GZip, UTF-8/BOM, Exportplätze,
Verbindungsplätze, Abbruch, Fehlerbereinigung, Zeilen-/Byte-/Zeit-/Zell-/Metadatenlimits
und Trailer entsprechen [EXPORT-API](EXPORT-API.md). v11 weist auch explizite neue
Optionsfelder mit `false` oder leerem Array ab. Kein Serverdateiziel, keine neue
Bibliothek und kein zusätzlicher Prozess. Rechte bleiben die tatsächlichen DB-Rechte.

## Textformate und verbindlicher Abschluss

Die Daten werden zeilenweise ausgegeben. Speicher wächst mit begrenzter Zeilen- und
Metadatenbreite, nicht mit der Zeilenanzahl. Numerische Werte bleiben Dezimalstrings;
Binärzellen bleiben Base64. Namen werden als Daten behandelt, nie als Markup oder
Quellcode-Identifier. Ungültiges UTF-8 wird abgewiesen, nicht ersetzt. Datenbank-
Textwerte mit zulässigem UTF-8 einschließlich Steuerzeichen bleiben rekonstruierbar.
Die sichtbaren Tabellenformate zeigen jede Zelle als JSON-Zelle (`kind`, `value`),
damit NULL, leerer Text, Binär- und Zahlenwerte unterscheidbar bleiben.

| `format` | Darstellung und Escaping | Exakter Abschluss (mit LF) |
| --- | --- | --- |
| `xml` | XML 1.0, feste Elementnamen `export/table/columns/rows/row/cell`; DB/Schema/Name und Spalten als Felder; Zellen mit `kind/value`. XML-Escaping einschließlich CR; Felder mit XML-unzulässigen Steuerzeichen erhalten `encoding="base64"`. Diese Felder vor Verwendung dekodieren. Keine DTD, Entitäten oder dynamischen Elementnamen. | `<complete>true</complete></export>` |
| `yaml` | Strikte JSON-Teilmenge von YAML 1.2: Objekt mit `format`, `version`, `tables`, `complete`; Tabellen mit `database/table/columns/rows`. JSON-Stringescaping verhindert YAML-Tags, Aliase, implizite Typisierung und Schlüssel-Injektion. Doppelte Spaltennamen bleiben im geordneten Spaltenarray. Kein YAML-1.1-Versprechen. | `],"complete":true}` |
| `php-array` | `<?php return array(...)`; `tables` enthält `metadata` als JSON-String und `rows` als geordnete Arrays typisierter Zellen. Jeder Stringbyte wird als `\xHH` in einem doppelt zitierten Literal ausgegeben; auch `$`, Quotes, Backslash und Steuerzeichen. NULL hat `kind=null` und einen leeren `value`. Kein schließendes PHP-Tag. | `),'complete'=>true);` |
| `latex` | Fragment mit `tabular`, fester Spaltenzahl, gequoteten Überschriften; keine Dokumenthülle. Backslash, `{ } $ & # % _ ^ ~` werden escaped; Steuerzeichen als sichtbare `\uXXXX`-Notation. Unicode benötigt ein geeignetes LaTeX-System. | `% DBMANAGER EXPORT COMPLETE` |
| `latex-longtable` | Wie LaTeX, mit `longtable` statt `tabular`; Verbraucher lädt das Paket `longtable`. | `% DBMANAGER EXPORT COMPLETE` |
| `mediawiki` | Wiki-Tabelle, jede Zeichenposition der Namen und Werte als numerische HTML-Entität; Steuerzeichen zunächst sichtbare `\uXXXX`-Notation. Keine rohen Templates, Links, Tags oder Zeilentrenner aus Daten. | `<!-- DBMANAGER EXPORT COMPLETE -->` |
| `texy` | Texy-Tabelle mit `|`-Rahmen; dieselbe vollständige Entitätskodierung wie MediaWiki. | `<!-- DBMANAGER EXPORT COMPLETE -->` |
| `codegen` | Ausschließlich Textprofil: JSON-Zeilen mit Formatkopf, `table`-Metadatensatz, einzelnen `row`-Datensätzen und Abschluss. Keine ausführbaren Klassennamen, Sprachvorlagen oder C#/NHibernate-Generatorparität. | `{"complete":true}` |

Für die ersten vier bisherigen Formate gilt der bisherige Abschluss. `structure`
ist für alle neuen Formate falsch, `data` wahr. Tabellen-, DB-, expliziter Server-
und lesender Abfrageumfang sind möglich; CSV bleibt bei genau einer Tabelle.
Neue Formate werden mit `.xml`, `.yaml`, `.php`, `.tex` beziehungsweise `.txt` und
passendem MIME-Typ heruntergeladen. PHP wird als `text/plain` geliefert.

Clients MÜSSEN das vollständige Format und die letzte strukturelle Schlussmarke
prüfen, im Browser insbesondere unabhängig von Trailern. Ein gleichlautender Text
in einer Zelle ist kein Abschluss. HTTP 200, eine geschlossene GZip-Datei, eine
beendete Tabelle oder ein geschlossenes XML-Element allein beweisen keinen Erfolg.
Bei Limits/Abbruch/Fehlern fehlt der Abschluss; HTTP-Trailer melden `incomplete` und
einen festen öffentlichen Fehlercode. Kein Restore vor vollständiger Prüfung.

## SQL-Dump-Details

| Feld | Unterstützter Umfang |
| --- | --- |
| `views_as_tables` | Explizit gewählte oder bei DB-/Serverumfang entdeckte Views werden mit Ergebnisdaten als reguläre Tabellen ausgegeben, ohne Viewdefinition, Schlüssel, Defaults, generierte Ausdrücke oder Eigentümer. PostgreSQL umfasst auch materialisierte Views. SQLite erzeugt Spalten ohne Typaffinität, um Speicherklassen zu bewahren. MySQL/PG akzeptieren nur eine feste skalare Typteilmenge; Domains, Arrays, ENUM/SET und unbekannte Typen werden abgewiesen. |
| `definer` | Nur `omit`. Es werden keine DEFINER-tragenden Objektdefinitionen exportiert; materialisierte Views enthalten keine DEFINER-Angabe. `preserve`/Ersetzung fremder Konten sind nicht implementiert und werden abgewiesen. Kein Regex-Umschreiben fremder Objekt-DDL. |
| `compatibility` | `native` (Vorgabe) erhält den Quelldialekt. `data-only` setzt `structure=false` und erzeugt Daten für vorhandene Zieltabelle. Widerspruch mit explizitem `structure=true`, `drop` oder `if_not_exists` wird abgewiesen. Historische MySQL-Modi und Cross-Dialektübersetzung sind nicht implementiert. |
| `comments` | Ergänzt Katalognamen als sichere einzeilige `-- Catalog table:`-Kommentare; Vorgabe falsch. Steuer-/Zeilentrennzeichen werden durch Leerzeichen ersetzt. Kommentare in nativer Tabellen-DDL bleiben wie bisher erhalten, ihre Entfernung wird nicht zugesagt. Keine Host-, Konto-, Pfad-, Zeit- oder internen Konfigurationsmetadaten. |
| `aliases` | Nur SQL-Datenexport (`structure=false` oder `data-only`) und explizite Tabellenauswahl. Liste aus `source` und `target` (je Schema/Name); beide werden im Katalog aufgelöst, das Ziel muss eine existierende reguläre Tabelle mit gleichen geordneten Namen/Typen/Generierungsmerkmalen sein. Quellen müssen ausgewählt sein; doppelte Quellen/Ziele oder widersprüchliche Auflösungen werden abgewiesen. dbplan zitiert Katalogbezeichner. Keine freie DDL-/Constraint-/Spaltenumschreibung. |
| `data_statement` | `insert` (Vorgabe); SQLite zusätzlich `insert-ignore` → `INSERT OR IGNORE`, `replace` → `REPLACE`; MySQL/MariaDB `INSERT IGNORE`/`REPLACE`. PostgreSQL weist beide Varianten ab. Diese Optionen verändern ausschließlich den späteren Restore; REPLACE kann abhängige Löschungen/Trigger auslösen und IGNORE Daten verwerfen. Kein automatischer Retry. |

Für Views meldet der HTTP-Header konservativ `read-only-best-effort`: MySQL-Views
können nichttransaktionale Quellen, PG-Views externe Quellen/Funktionen enthalten.
Bei üblichen Tabellen bleiben die bisherigen Snapshotregeln bestehen. Der Export
verändert die Quelldaten nicht. Ein View-Dump wird in eine frische Zieldatenbank
restauriert; `DROP TABLE` entfernt keine bereits vorhandene View gleichen Namens.

Trigger, Routinen und Events sind mangels integrierter W-P19-Anbindung ausdrücklich
offen. Deren DEFINER, Rechte, Reihenfolge und Restore sind nicht zugesagt. Keine
vollständige Backup- oder phpMyAdmin-Paritätsaussage. Exportvorlagen (EXP-003 /
NEU-EXP-003), serverseitige Verzeichnisse (EXP-005 / IMP-003), Office/PDF
(EXP-011..014; E8) und ODS/XML/MediaWiki-Import (IMP-009..011; W-P12) gehören nicht
zu diesem Backendpaket. Diese Grenzen sind keine implementierte Nichtziel-Parität.

Prüfstand und Reviewgate: [Handoff W-P28](../docs/handoffs/W-P28-backend.md).
