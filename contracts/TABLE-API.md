# Tabellen-API 0.2.0 – Kandidat B2a

Status: Implementierungs-/Reviewkandidat, **nicht für Clientbindung freigegeben**.
Der unabhängige Vertragsreview und ein separater Freeze erfolgen vor F3-Verwendung.
Spezifikation: [api-v0.2.0.json](api-v0.2.0.json). Die unveränderte
[API 0.1.0](api-v0.1.0.json) bleibt inklusive ihrer v1-Routen und Capability-Antwort gültig.
`GET /api/v1/capabilities` meldet weiterhin `contract_version: "0.1.0"` ohne neue Felder,
damit bestehende strikte F1-Validatoren weiter funktionieren.

## Routen und Grenzen

Alle neuen Routen benötigen denselben Bearer-Token, dieselben transienten Verbindungswerte
und dieselbe Ziel-/TLS-/SQLite-Root-Policy. Authentifizierung erfolgt vor Body-/DB-Zugriff.
Keine CORS-Erweiterung, kein Cookie und kein zusätzlicher Datenspeicher.

| Route | Zweck |
| --- | --- |
| `GET /api/v2/capabilities` | Version 0.2.0 und verfügbare `table_describe`/`table_browse`-Fähigkeiten; `mutations=false` |
| `POST /api/v2/tables/describe` | Objektbezug und strukturierte Spalten-/PK-/Engine-Metadaten |
| `POST /api/v2/tables/browse` | Begrenzte Tabelle/View mit kontrollierter Sortierung und gebundenen Filtern lesen |

Ein Tabellenbezug besteht aus `schema` und `name`, jeweils höchstens 1024 UTF-8-Bytes;
`name` ist erforderlich. Leeres Schema bedeutet SQLite `main`, MySQL die angegebene
Verbindungsdatenbank, PostgreSQL `public`. Es gibt keine implizite Suche durch mehrere
Schemas. SQLite akzeptiert nur `main` und keine internen `sqlite_`-Objekte. Namen werden
zuerst gegen Metadaten aufgelöst und anschließend nach dem DB-Dialekt gequotet.
Auch Schema-/Tabellen-/Spaltennamen mit SQL-Sonderzeichen bleiben Identifier.

`page_size`: 0/fehlend bedeutet 100, höchstens 1000; `offset`: 0 bis 1000000;
`timeout_ms`: 0/fehlend 10000, höchstens 60000. Höchstens 16 Sortierspalten und 16
Filter. Es gelten zusätzlich 1 MiB Anfrage, 1 MiB Rohdaten je Ergebniszelle, 8 MiB
**tatsächlich encodierte Gesamtantwort** und die vier globalen DB-Anfrageplätze.
Beschreibungsmetadaten werden ebenfalls in ihrer Größe begrenzt. Ein Größen-/Zeitfehler
ist kein vollständiges Teilergebnis. Bestehende Fehlerhülle und Abbruchbehandlung bleiben.
Unbekannte Requestfelder und rohe JSON-Nullwerte werden wie in v1 abgewiesen.

## Metadaten und Schreibgrenzen

Describe liefert Spaltennamen, serverseitige Typbeschreibung, Nullable, optionales
`default_value`, PK-Position (1-basiert, 0 bedeutet kein Schlüssel), Generated-Art und
`read_only`; außerdem Objektart, Primärschlüsselreihenfolge, Storage-Engine und
`transactional` (`yes`/`no`/`unknown`). `default_value` ist **uninterpretierte servernative
Metadatenrepräsentation**, kein vereinheitlichtes SQL-Literal und keine zur Ausführung
freigegebene SQL-Zeichenfolge. Fehlender Default ist ausgelassen, nicht als leerer Text erfunden.

**`read_only=true` beschreibt die aktuell angebotene strukturierte Produktfähigkeit.**
In B2a sind alle Tabellen und Spalten so markiert; es existiert kein Mutationsendpunkt.
Das ist weder eine Aussage über tatsächliche DB-Privilegien noch eine Sicherheitsgarantie
für den unverändert mächtigen freien SQL-Endpunkt. PK-/Generated-/Nullable-Angaben sind
zusätzliche Informationen für eine spätere erneute B2b-Schreibprüfung, keine Editierfreigabe.
Views, virtuelle/Foreign-Tabellen, fehlender PK und ungeklärte Rechte werden nicht als
bearbeitbar behauptet. MySQL InnoDB wird als transaktionsfähige Datenengine erkannt;
MyISAM/MEMORY/CSV/ARCHIVE als nicht transaktional, andere Engines bleiben `unknown`.
Diese Kennzeichnung verspricht keine transaktionale MySQL-DDL oder HTTP-übergreifende Transaktion.

SQLite-Typdeklarationen bleiben Metadaten. Jede SQLite-Ergebniszelle folgt ihrer
**tatsächlichen Runtime-Speicherklasse**; TEXT in einer INTEGER-Spalte bleibt Text,
BLOB in TEXT bleibt Binärwert einschließlich Leerblob. Exakte Integer-/Decimal- und
NULL-/Leerwertsemantik der v1-Zellen bleibt unverändert. SQLite-NUMERIC-Affinität kann
Werte bereits beim Speichern umwandeln; die API erfindet daraus keine verlorene Präzision.

## Pagination, Reihenfolge und Filter

Die Abfrage liest höchstens `page_size+1` Datensätze und führt **kein implizites COUNT**
über die Datentabelle aus. `has_more` stammt aus genau einer Lookahead-Zeile, deren Zellen
nicht in das Antwortbudget aufgenommen werden. Die ausgegebene `result.rows` enthält
höchstens `page_size`; `result.truncated=false` bleibt ausdrücklich, weil `has_more`
die Pagination beschreibt. Ein exakter Gesamtzähler wird nicht geliefert.

Sortierung enthält ausschließlich vorhandene Metadatenspalten und `asc`/`desc`.
Nicht bereits gewählte PK-Spalten werden in ihrer Schlüsselreihenfolge aufsteigend
angehängt. SQLite hängt stets alle PK-Spalten zusätzlich mit festem `COLLATE BINARY`
aufsteigend als letzten Tiebreak an, auch wenn sie bereits explizit sortiert wurden.
Damit erzeugen abweichende Spalten-/PK-Indexkollationen keine unerkannten Gleichstände. Ohne Sortierung wird der PK verwendet, soweit vorhanden. `stable_order=true`
bedeutet nachgewiesene eindeutige PK-Ordnung mit nicht-nullbaren Schlüsselspalten.
Ohne PK oder bei nullable SQLite-PK gilt `stable_order=false`, auch wenn eine vom Nutzer
gewählte Sortierung zufällig eindeutige Werte enthält. SQLite-INTEGER-Rowid-Alias und
`INTEGER PRIMARY KEY DESC` werden anhand tatsächlicher Indexmetadaten unterschieden.

Offsetseiten sind **kein über mehrere Requests festgehaltener Snapshot**. Gleichzeitige
DB-Änderungen können Einträge zwischen Seiten verschieben, auch bei eindeutiger Sortierung.
Metadatenabruf und SELECT verwenden einen kurzen anfrageeigenen Pool ohne neues
Transaktionsversprechen. Gleichzeitige DDL kann zu einem sichtbaren DB-Fehler führen.
Views können trotz fehlendem COUNT teuer sein; dieselben Zeitlimits bleiben wirksam.

Filter werden mit AND kombiniert. Nur diese Operatoren sind vorhanden:

- `eq` benötigt eine getaggte Stringzelle als `value`; keine rohe SQL-Zeichenfolge.
- `is_null` und `is_not_null` dürfen kein `value` enthalten.
- NULL-Gleichheit wird ausdrücklich über `is_null` dargestellt, nicht `eq` mit NULL.

Binärfilter verwenden Base64 und werden als Bytes gebunden, einschließlich Leerblob.
Integer, Decimal, Float und Boolean werden vor Bindung geprüft; Text-/Zeitwerte bleiben
gebundene Daten und werden vom jeweiligen DB-Typ verglichen. Es gibt keine freien
SQL-Operatoren, Kollations-/Ausdrucksfragmente, Tabellenjoins oder vom Client gelieferten
SELECTs in diesen Endpunkten. Der freie SQL-Endpunkt ist davon unabhängig unverändert.

## Nachweise und nächste Entscheidung

Prüfstand, Commit-/CI-Herkunft und verbleibende Grenzen stehen in
[Handoff B2a](../docs/handoffs/B2a.md). Erst unabhängige QA-/Securityprüfung der Spezifikation
und Implementierung, danach ausdrücklicher Freeze mit Schemahash; erst dann Clientbindung.
Keine neue API-/Mutationsfunktion außerhalb dieses begrenzten Leseumfangs ableiten.
