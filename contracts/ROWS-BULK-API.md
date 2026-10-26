# Mehrzeilen, Dateiwerte und Fremdschlüssel-Auswahl – API 0.19

Vertragskandidat: [api-v0.19.0.json](api-v0.19.0.json), additiv zum unveränderten
API-Stand 0.12. Die Nummern 0.13 bis 0.18 werden von anderen Paketen vergeben.
Alle sechs Routen unter `/api/v19/` verlangen Bearer-Authentifizierung. Verbindungen
stehen ausschließlich im Requestbody; Antworten enthalten keine Verbindungsdaten,
SQL-Anweisungen, Treiberfehler oder Dateipfade. Unbekannte Felder, fehlende
Pflichtfelder und JSON-null werden abgewiesen; echte Leerwerte bleiben gültig.

## Auswahloperationen

`POST rows/plan` nimmt `connection`, `table`, `action` (`update`, `delete`, `copy`),
`rows` und gegebenenfalls `changes`. Jede ausgewählte Zeile enthält `expected`
mit vollständigen, geordneten `columns`/`values` gemäß v3. Auswahl über Filter oder
freie SQL-Fragmente ist ausgeschlossen. `changes` setzt dieselben markierten
Nicht-PK-/Nicht-Generated-Spalten aller ausgewählten Zeilen; ausgelassene Spalten
bleiben erhalten. Keine doppelten Zeilen oder Zielspalten.

Der Plan prüft jede aktuelle Zeile unter Sperre gegen das vollständige Abbild.
Er liefert die kanonische Tabelle, Aktion, `row_count`, Warnungen und je Zeile den
Primärschlüssel (`key`) sowie `stability_sha256`. Das Stabilitätsmerkmal ist ein
SHA-256 über Tabellenmetadaten und vollständigen kanonischen Zeilenzustand mit
Domänentrennung. Der Planhash bindet zusätzlich Aktion, Auswahlreihenfolge,
Änderungs-/Kopierwerte und Verbindungsziel (ohne Passwort). Er ist kein Geheimnis,
kein Berechtigungsnachweis und kein dauerhafter Einmalbeleg. Die Plantransaktion
endet ohne Schreibwirkung; zwischen Vorschau und Ausführung bestehen keine Locks.

`POST rows/execute` verlangt dieselben Eingaben, `confirm:true`,
`expected_plan_sha256` und das vom Plan gelieferte `stability_sha256` jeder Zeile.
Der Server prüft den gesamten Plan erneut in derselben Transaktion wie alle
Mutationen. Alle Zeilen werden vor der ersten Änderung gesperrt und geprüft.
Ein Konflikt oder eine Constraintverletzung weist den gesamten Batch zurück.
Die Ergebnisse enthalten je Auswahlindex `completed`, `rejected` oder
`outcome_unknown`; es gibt keinen als Erfolg ausgegebenen Teilcommit.

Kopieren dupliziert Zeilen in derselben Tabelle. `copy_values` je Zeile überschreibt
gezielt die aus dem Snapshot übernommenen Werte. Ein expliziter neuer Wert jeder
PK-Spalte ist erforderlich, außer bei dem nach v4 belegten einzelnen Auto-PK
(SQLite INTEGER-Rowidalias, MySQL/MariaDB AUTO_INCREMENT, PostgreSQL Identity/Serial).
Dieser wird standardmäßig ausgelassen. Generated-Spalten werden neu berechnet.
Keine REPLACE-, IGNORE- oder UPSERT-Semantik. SQLite verwendet `OR ABORT`.
Nachherzeilen werden wie bei v4 mit tatsächlicher Schlüsselidentität zurückgelesen.
Vor Commit werden alle zurückgegebenen Zeilen nochmals gegen spätere Triggerwirkungen
geprüft; eine Abweichung führt zum Rollback.

Nur transaktionale Basistabellen mit stabilem, nichtnullablem Primärschlüssel und
v3/v4-Eignung sind zugelassen. Views, virtuelle/Foreign-Tabellen, PostgreSQL-Vererbung,
Partitionen und Rewrite-Regeln sowie nichttransaktionale MySQL-Engines werden gesperrt.
SQLite nutzt BEGIN IMMEDIATE mit aktivierter FK-Prüfung, Netzwerkadapter Read Committed
mit FOR UPDATE. Schlüsselwerte werden gebunden; neue Identifier werden ausschließlich
nach Metadatenprüfung über `dbplan` gequotet. Die wiederverwendeten v3/v4-Helfer behalten
ihr bisheriges bytegleiches Quoting und ihre Schutzregeln.

HTTP 200 bedeutet `completed`, 400 ungültige Auswahl/Bestätigung, 409 Zeilen-/Plan-
Konflikt, 422 sonstige sichere Ablehnung. Execute-Ablehnungen enthalten `RowsBulkResult`
mit `outcome:rejected`, festem `reason` und durchgehend abgelehnten Zeilen, keine
zurückgerollten Nachherabbilder. Nach versuchtem Commit ist jeder Commitfehler
HTTP 503 mit `outcome_unknown` für alle Zeilen. Authentifizierungs-, Decoder- und
fehlende Featurefehler verwenden das bestehende `Error`-Format.

Maximal 100 Zeilen, 1024 Spalten je Abbild/Änderungsliste, 1 MiB Requestbody,
8 MiB JSON-Antwort einschließlich abschließender Newline. Standardzeitlimit 10 s,
höchstens 60 s. Auch No-op-Updates prüfen alle Snapshots. Kein automatischer Retry,
kein Exactly-once, keine Erkennung von ABA oder physisch unrepräsentierten Änderungen.
Trigger/Kaskaden und externe Effekte sowie Sequenz-/Auto-ID-Lücken bleiben Grenzen
wie in v3/v4. Die Antwort ist kein Nachweis eines dauerhaft unveränderten Zustands.

## Dateiwerte

`POST values/upload`: `multipart/form-data`, zuerst genau ein Teil `metadata`
(JSON gemäß `FileValueRequest`), danach genau ein Teil `file`. Dateiname und gelieferter
Content-Type werden ignoriert. Metadaten tragen Verbindung, Tabelle, vollständiges
`expected`, `column`, `confirm:true` und optional `timeout_ms`. Kein weiterer Teil;
die komplette begrenzte Hülle wird vor einem Schreibversuch geprüft. Die Mutation
erhält v3-Konflikterkennung und Ergebnisformat `MutationResult` (`committed`),
einschließlich HTTP 409 und unbekanntem Commit (503 Error). Kein Plan für diesen
bereits durch Einzelzeilenabbild und Bestätigung geschützten Schreibpfad.

Unterstützt werden beschreibbare BLOB/BINARY/VARBINARY/BYTEA-Spalten außerhalb des PK.
TEXT-Dateiupload ist in diesem Paket nicht freigegeben. Maximal 512 KiB Binärinhalt,
einschließlich leerem BLOB; NULL bleibt davon verschieden. Der Body bleibt insgesamt
auf 1 MiB begrenzt. Alte große Abbildwerte und neue Dateiwerte können gemeinsam die
Grenze erreichen, selbst wenn die Datei allein unter dem Maximum liegt.

`POST values/download`: JSON mit derselben Zeilenidentität, `column` und optional
`preview:true`. Die aktuelle Zeile muss mit `expected` übereinstimmen. Download
streamt Base64-Dekodierung in den HTTP-Writer, mit Content-Length, festem Dateinamen,
`Content-Disposition: attachment`, `application/octet-stream`, `nosniff`, `no-store`
und restriktiver CSP. Keine Credentials in URL oder Dateinamen. NULL und Text werden
abgewiesen. Keine beliebige Inline-Ausgabe und keine Range-Anfragen.

Upload wird begrenzt aus dem Stream gelesen. Die vorhandenen Treiber binden volle
Werte; auch das kanonische Zeilenabbild wird begrenzt im Speicher gelesen. Es wird
keine unbegrenzt speicherkonstante DB-BLOB-Übertragung behauptet. Keine temporären
Dateien und keine neue Abhängigkeit. Nach HTTP-Übertragungsbeginn kann ein Abbruch
nur durch die unvollständige Content-Length erkannt werden; es folgt kein JSON-Fehler.

Vorschau akzeptiert ausschließlich dekodierbare PNG/JPEG/GIF bis 4 Millionen Pixel.
Der Server kodiert das erste Bild als PNG neu, entfernt angehängte Inhalte und
Metadaten und begrenzt das Ergebnis ebenfalls auf 512 KiB. Auch diese Antwort bleibt
Attachment (`image/png`); ein Client kann die authentifiziert geladenen Bytes als
lokales Bild darstellen. SVG, HTML und andere aktive Formate sind ausgeschlossen.

## Fremdschlüssel-Auswahl

`POST foreign-keys/lookup`: `connection`, Quelltabelle `table`, Quellspalte `column`,
`display_column` der Zieltabelle; optional `search`, `page_size`, `offset`, `timeout_ms`.
Das Ziel und die Wertspalte stammen ausschließlich aus frischen FK-Metadaten. Nur ein
bekannter einfacher FK je Quellspalte ist zugelassen; zusammengesetzte, mehrdeutige
und implizit unbekannte Ziele werden erklärt abgewiesen. Die Zielbasistabelle benötigt
einen stabilen PK für reproduzierbare Sortierung. Antworten enthalten Wert und
Anzeigewert als präzise Cells sowie Zielmetadaten und `has_more`.

Suche ist eine gebundene Teilzeichenfolgensuche auf der nach Text konvertierten
Anzeigespalte. `%`, `_` und `!` im Suchtext sind Literale. Maximal 256 Suchbytes,
100 Werte je Seite (Standard 50), Offset bis 1000000; Lookahead ermittelt `has_more`.
DB-Rechte und PostgreSQL-RLS bestimmen die sichtbaren Werte. Kataloglesen und Lookup
bilden keinen gemeinsamen Schemasnapshot; Offsetseiten können sich bei parallelen
Änderungen verschieben. Eine Auswahl garantiert kein später erfolgreiches Schreiben.
