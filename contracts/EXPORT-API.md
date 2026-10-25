# Export und SQL-Dump – vorläufige API 0.12

Status: Implementierter Reviewkandidat W-P11, **keine Vertragsfreigabe**, kein Webpin.
Die Nummer 0.12.0 ist vorläufig. Bestehende Routen und Schemata bis 0.7 bleiben unverändert;
0.8 bis 0.11 sind unabhängige Pakete. Vertrag: [OpenAPI](api-v0.12.0.json),
[Dateiformat und Treuegrenzen](EXPORT-FORMAT.md), [Fixtures](fixtures/export/README.md).

## Routen und Authentisierung

- `GET /api/v12/capabilities`: Verfügbarkeit des Engineinterfaces, Formate, Kodierungen,
  Snapshotgrenzen je Treiber und Ressourcenobergrenzen. Dies ist keine Rechtezusage:
  ausschließlich tatsächliche Datenbankrechte entscheiden über jeden Zugriff.
- `POST /api/v12/export`: JSON-Anfrage der Standardklasse (höchstens 1 MiB), Bearer-Token,
  browserseitiger Download. Keine Serververzeichnisse, externen Dump-Prozesse oder Jobs.

Vor dem ersten Ausgabebyte werden Optionen, Verbindung und die Metadaten der aktuellen
Datenbank geprüft. Ungültige Optionen: 400 `invalid_export`; fehlendes Engineinterface:
422 `unsupported_feature`; weitere Verbindungs-/Datenbank-/Zeitfehler wie im Bestand.
Ab Streambeginn bleibt der Status 200: Bei Fehlern fehlen Formatabschluss und
Erfolgstrailer. Ein 200 allein ist **kein** erfolgreicher Export.
`X-Export-Status: complete|incomplete` und gegebenenfalls `X-Export-Error` sind deklarierte
HTTP-Trailer mit festen öffentlichen Codes. Bei Verbindungsabbruch können auch Trailer
fehlen. Keine rohe DB-Fehlermeldung oder Zelleninhalte werden ausgegeben oder protokolliert.

Browser können Trailer nicht zuverlässig lesen. Deshalb muss ein Client außerdem den
Abschluss im [Dateiformat](EXPORT-FORMAT.md) prüfen. Ein teilweise gespeicherter Download
ist keine Sicherung. SQL-Restore erst nach Prüfung des vollständigen Downloads beginnen;
insbesondere MySQL-DDL kann auch mit Transaktionsrahmen implizit committen.

## Umfang und Optionen

`connection` entspricht dem bisherigen flüchtigen Verbindungsobjekt; TLS ist standardmäßig
`verify-full`. Sichere SQLite-Kennungen bleiben verpflichtend. Nicht ausgewählte
Datenbanken werden niemals automatisch entdeckt oder exportiert.

| Feld | Vorgabe / Verhalten |
| --- | --- |
| `scope` | `tables` (Vorgabe, explizite `tables` erforderlich), `database` (reguläre Tabellen der verbundenen DB), `server` (explizite `databases`, höchstens 20), `query` (genau eine lesende `sql`-Abfrage) |
| `format` | `sql` (Vorgabe), `csv`, `excel-csv`, `json` |
| `tables` | Höchstens 100 eindeutige Schema-/Tabellennamen, anhand der tatsächlichen Metadaten aufgelöst und danach zitiert |
| `structure`, `data` | SQL standardmäßig beide wahr; andere Formate nur Daten. Mindestens eines wahr |
| `drop`, `if_not_exists` | Optionale SQL-DDL-Zusätze, standardmäßig falsch |
| `extended` | Gebündelte INSERTs; sonst ein vollständiges INSERT mit Spaltenliste je Zeile |
| `max_statement_bytes` | 0/fehlend: 1 MiB; explizit 1024 bis 4 MiB; einschließlich INSERT-Kopf und Abschluss. Auch eine einzelne zu große Zeile bricht mit `export_statement_limit` ab |
| `dialect` | Vorgabe: Quelltreiber; nur derselbe Dialekt unterstützt. Keine automatische Typ-/Schemaübersetzung |
| `charset` | `utf-8` (Vorgabe), `utf-8-bom`; andere Kodierungen werden abgewiesen |
| `filename` | Optionaler ASCII-Basisname bis 120 Zeichen, ohne Verzeichnisse oder Steuerzeichen; sonst `export.sql`, `.csv` bzw. `.json` |
| `gzip` | GZip-Datei mit zusätzlichem `.gz`; kein HTTP Content-Encoding, kein temporäres Serverdateiziel |
| `delimiter`, `crlf` | CSV: Komma, Semikolon oder Tab, doppeltes Anführungszeichen als feste Einfassung und Verdopplung als Escape; LF oder CRLF. Excelprofil: Semikolon als Standard, CRLF, BOM |
| `transaction` | Optionaler BEGIN-/COMMIT-Rahmen im SQL-Dump; keine DDL-Atomaritätszusage |
| `disable_fk` | SQL MySQL/MariaDB: ursprünglichen Sitzungswert sichern/wiederherstellen; SQLite: OFF vor BEGIN, ON nach COMMIT. PostgreSQL wird abgewiesen; kein Bypass nichtdeferrierbarer Constraints |
| `max_rows` | Gesamtzahl über alle ausgewählten Datenbanken und Tabellen; Vorgabe 1.000.000, maximal 10.000.000 |
| `max_bytes` | Unkomprimierte Ausgabe einschließlich Rahmen/BOM/Abschluss; Vorgabe 64 MiB, maximal 1 GiB. GZip kann zusätzliche Container-/Flushbytes benötigen |
| `timeout_ms` | Gesamter Vorgang einschließlich Öffnen/Metadaten/Download: Vorgabe 60.000, maximal 300.000 ms |

0 bei Ressourcenfeldern verwendet die Vorgabe. Grenzen sind pro Anfrage innerhalb
fester Serverobergrenzen einstellbar. Zusätzlich gelten höchstens 1024 Spalten,
1 MiB pro Zelle, 4 MiB Zellwerte je Zeile, 1 MiB Definition je Tabelle und 8 MiB
angesammelte Definitionen/Abfragen je Datenbank. Kein 8-MiB-Limit für die Daten insgesamt.
Eine zusätzliche Zeile über dem Zeilenlimit löst `export_row_limit` aus; keine stille
Kürzung. Exakt am Limit vollständig gelesene Daten bleiben vollständig.

CSV ist auf genau eine Ergebnistabelle begrenzt. JSON erhält mehrere Tabellen samt
DB-Zuordnung. Abfrageexport ist nur CSV/Excel-CSV/JSON: Metadaten werden vorbereitet
(SQLite ohne Ausführung, Netzwerk mit gekapseltem LIMIT 0), Daten anschließend
zeilenweise gelesen. Der Abfrageexport meldet ausdrücklich `read_only_best_effort` (Header:
`read-only-best-effort`), da freie Abfragen auch nichttransaktionale oder externe
Datenquellen/Funktionen lesen können. Die Read-only-Transaktion ist die Schreibgrenze; freie Abfragen
laufen mit den tatsächlichen Verbindungsrechten, kein SQL-Präfix als Sicherheitsfilter.
SQLite verlangt zusätzlich ein natives ReadOnly-Statement und verweigert Batches.
Zeilenauswahl/Projektion/Sortierung sind im Abfrageexport möglich; Tabellenexport
umfasst alle speicherbaren Spalten und verwendet keine zugesagte Zeilenreihenfolge.

## Snapshot und unterstützte Teilmenge

SQLite: `query_only`, eine lokale Transaktion, eine exklusiv geöffnete verwaltete Datei;
alle Grenzen von [ADR-0004](../docs/decisions/ADR-0004-sqlite-adapter.md) bleiben bestehen.
Der Sandboxadapter erlaubt `PRAGMA foreign_keys` nicht; ein Dump mit FK-Schalter ist
für einen externen SQLite-Restore gedacht, nicht automatisch über die bestehende Query-API
importierbar. Keine WAL-/Mehrprozess-/HA-/Powerlosszusage.

MySQL/MariaDB: UTF-8-Verbindung, UTC-Sitzung, REPEATABLE READ, CONSISTENT SNAPSHOT und
READ ONLY. Nur InnoDB-Tabellen; MyISAM/MEMORY/Views werden abgewiesen. Keine globale
Sperre und kein serverweiter konsistenter Snapshot. Tabellenzugriff hält die regulären
Metadatensperren bis Transaktionsende. Der Serverexport ist SQL nur hier möglich
(CREATE DATABASE/USE); normale Tabellendumps nutzen unqualifizierte Zielnamen, damit sie
in einer frischen anders benannten DB wiederhergestellt werden können. SQL-Tabellenauswahl
muss aus der verbundenen DB stammen.

PostgreSQL: eine REPEATABLE READ READ ONLY-Transaktion mit UTC und ISO-Datumsdarstellung.
Partitionierte/vererbte Tabellen werden abgewiesen. Die Katalogrekonstruktion entspricht API 0.7: Spalten, Constraints und eigenständige Indizes,
aber keine vollständige Sicherung. Rechte, Eigentümer, Sequenzen/Sequenzstände, Trigger,
Regeln, Partitionierung und Storageparameter sind kein zugesagter Roundtrip.
FK-Abhängigkeiten und vorhandene Sequenzen können Vorbereitung oder angepasste Reihenfolge
beim Restore verlangen. SQL-Serverexport wird abgewiesen, JSON kann ausdrücklich gewählte
DBs nacheinander lesen. Nie einen atomaren Snapshot über DB-Grenzen ableiten.

Views/Routinen/Trigger/Events, zusätzliche Kompressionen und Cross-Dialektübersetzung sind
keine implementierte Objektklassen-Parität; W-P19/W-P28 bleiben zuständig. Insbesondere
PMA-DUMP-001 und PMA-NEU-DUMP-001 sind damit nur teilweise abgedeckt. Vollständige
phpMyAdmin-Parität und universelle Wiederherstellbarkeit werden nicht behauptet.
