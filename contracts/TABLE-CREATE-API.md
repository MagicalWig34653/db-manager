# Bestätigte neue Tabelle – Vertragskandidat 0.6.0

B4a ergänzt `GET /api/v6/capabilities` und `POST /api/v6/tables/create`.
V1–v5 bleiben byte- und verhaltensgleich. Dieser 0.6-Kandidat ist vor unabhängigem
Review/exakter CI nicht für Produktclients freigegeben. Keine UI oder DDL-Vollparität.

Eine Anfrage bezeichnet Verbindung, Schema/Tabelle, 1–32 geordnete Spalten,
`primary_key` als explizite Liste (leer oder höchstens 8 Spalten) und `confirm:true`.
Jede Spalte benötigt Name, strukturierten Typ, `nullable` und expliziten Defaultmodus.
Fehlend, JSON-null und legitimer leerer Textdefault werden unterschieden. Namen sind
nichtleer, NUL-frei und gültiges UTF-8 mit höchstens 63 Bytes; Sonderzeichen werden
als Bezeichner gequotet. SQLite nur main und keine sqlite_-Tabellen; geschützte
MySQL-/PostgreSQL-Systemschemas gesperrt. Leeres Schema wählt main, Verbindungs-DB
bzw. public. Es wird kein Schema automatisch angelegt.

| Typkennung | SQLite | MySQL/MariaDB | PostgreSQL |
| --- | --- | --- | --- |
| int64 | BIGINT | BIGINT | BIGINT |
| text mit length1–1024 | VARCHAR(n) | VARCHAR(n), utf8mb4 | VARCHAR(n) |
| binary | BLOB | BLOB | BYTEA |
| decimal mit precision1–38, scale0–precision | DECIMAL(p,s) | DECIMAL(p,s) | DECIMAL(p,s), Katalog NUMERIC(p,s) |

SQLite bleibt flexibel typisiert: deklarierte Textlänge/Decimalpräzision sind keine
Speicherklassen-, Längen- oder exakte Dezimalspeicherungsgarantie. Insbesondere kann
SQLite numerische Defaults später in REAL umwandeln. BIGINT-PK ist ausdrücklich kein
INTEGER-Rowid-Alias und keine Auto-ID. MySQL/Maria werden mit InnoDB angelegt;
Zeichensatz utf8mb4, Kollation nach DB-Vorgabe. Keine Kollations-/Engineauswahl.

Defaultmodi: `none` erzeugt keine DEFAULT-Klausel; `null` erzeugt DEFAULT NULL und
benötigt nullable:true; `value` benötigt eine getaggte Stringzelle. Int64 akzeptiert
nur kanonische ganze Werte im signed64-Bereich, Decimal nur feste Dezimalschreibweise
ohne Exponent und innerhalb der gewählten Stellen/Skala, Text gültiges UTF-8 ohne
NUL und innerhalb Länge/4096 Rohbytes. Binary hat vorerst keinen konstanten Default.
Es gibt kein DEFAULT-Funktions-/Ausdrucks-/Typ-/Constraint-SQL als Eingabe. Die Engine
setzt auf ihrer kurzlebigen MySQL-Session NO_BACKSLASH_ESCAPES/STRICT_ALL_TABLES/
NO_ENGINE_SUBSTITUTION und utf8mb4, auf PostgreSQL standard_conforming_strings=on.
Nur dann erzeugt sie feste, korrekt gequotete Literale. Katalogdefault-SQL wird
niemals ausgewertet; ausschließlich bekannte konstante Darstellungen werden verglichen.
MariaDBs gequotete Katalogliterale und MySQLs rohe Textwerte werden anhand des
beobachteten Serverherstellers getrennt behandelt. Nullable ohne Default und
explizites DEFAULT NULL können im nativen Katalog gleich normalisiert erscheinen.

PK-Spalten müssen vollständig in der Definition vorkommen, nichtnullable und vom
Typ int64/text sein. Reihenfolge bleibt erhalten. Namen werden konservativ ohne
Groß-/Kleinschreibungsduplikate behandelt; gesamte Textbreite höchstens 4096 Zeichen,
PK-Budget höchstens 2048 Bytes nach utf8mb4-Abschätzung. Kein Generated/Identity/FK/
CHECK/Unique-Extraindex/Partition/Vererbung/temporäre Tabelle/CREATE-AS/DDL-Batch.

CREATE enthält weder IF NOT EXISTS noch OR REPLACE. Ein vorheriges Nachschlagen
verbessert Fehlertexte; erst echtes CREATE entscheidet konkurrierende Namensvergabe.
Vorhandene Tabellen werden nicht verändert, geleert oder als neu angelegt bestätigt.
Es existiert keine Retry-, Drop-, Reparatur- oder permanente Markierungslogik.

## Abschluss und getrennte Strukturprüfung

`outcome: created` bedeutet sicher bestätigten Abschluss: MySQL/MariaDB liefern ein
positives CREATE-Ack, SQLite/PostgreSQL einen erfolgreichen Transaktionscommit.
`actual_description` enthält die tatsächlich gelesenen Metadaten im bisherigen
Describeformat. `read_only` behält dessen Produktfähigkeitsbedeutung.

| structure_check | Bedeutung |
| --- | --- |
| transactional | SQLite/PG: DDL und erfolgreiche Strukturprüfung in derselben echten Transaktion, danach Commit bestätigt; snapshot_atomic:true. |
| observed | MySQL/Maria: CREATE bestätigt; anschließend separat gelesene Struktur entspricht den unterstützten geprüften Attributen; snapshot_atomic:false. |
| different | MySQL/Maria: CREATE bestätigt; separate Beobachtung kann nicht als übereinstimmend bestätigt werden. Tatsächliche Metadaten zurückgegeben, keine Reparatur. |
| unavailable | MySQL/Maria: CREATE bestätigt, Strukturprüfung fehlgeschlagen/abgebrochen/zu groß. „Anlage bestätigt, Strukturprüfung nicht verfügbar“, ohne actual_description. |

`snapshot_atomic` bezeichnet ausschließlich die zusammengehörige DDL-/Metadaten-
beobachtung derselben Transaktion. Es gibt keine Zustandsgarantie nach Commit oder
Antwort, keine Garantie fremder Trigger-/Funktions-/externer Effekte. MySQL/MariaDDL
verursacht implizite Commits. Eine kurze nachfolgende Read-only-Transaktion erlangt
über SELECT LIMIT0 eine MDL und schützt erst ab dieser Lockaufnahme die separate
Beobachtung; sie beweist keine physische Identitätskontinuität vom CREATE-Ack bis
zum Rücklesen. Ein fremdes DROP/RECREATE/ALTER dazwischen kann eine andere Beobachtung
erzeugen. Es werden keine heimlichen Produktmarker ins Nutzerschema geschrieben.

Bei SQLite/PG führen fehlende/nicht verifizierbare Metadaten vor Commit zum Abbruch
der eigenen DDL-Transaktion. Bei MySQL bleibt ein sicher bekanntes Ack auch bei
nachfolgendem Fehler created/unavailable. Transport-/Abbruchfehler während MySQL-CREATE
ohne sicheres Ack und Fehler beim SQLite-/PG-Commitversuch ergeben dagegen 503
`outcome_unknown`: Anlage könnte erfolgt sein; Zustand prüfen, niemals blind erneut
senden oder entfernen. Eine verlorene HTTP-Erfolgsantwort bleibt beim Client ebenfalls
ungewiss. Aus allgemeinen Fehlerantworten kein universelles DDL-Rollbackversprechen
ableiten. Ein bestätigter Namenskonflikt ergibt 409 `create_conflict`.

Bodylimit 1 MiB; timeout_ms0 bedeutet zehn Sekunden, höchstens 60 Sekunden; bestehende
vier Engineplätze und Verbindungsfreigabe gelten. Erfolg wird vor Commit auf die
wirkliche JSON-Antwortgrenze 8 MiB inklusive Newline geprüft. Nach sicherem MySQL-Ack
führt eine zu große Beobachtung stattdessen zu created/unavailable. Auth/TLS/
Zielhostpolicy und sichere deutsche Fehler bleiben unverändert.

Primärquellen: [SQLite CREATE](https://www.sqlite.org/lang_createtable.html),
[SQLite Typaffinität](https://www.sqlite.org/datatype3.html),
[MySQL implizite Commits](https://dev.mysql.com/doc/refman/8.4/en/implicit-commit.html),
[MySQL MDL](https://dev.mysql.com/doc/refman/8.4/en/metadata-locking.html),
[MariaDB CREATE](https://mariadb.com/docs/server/server-usage/tables/create-table),
[MariaDB Katalogdefaults](https://mariadb.com/docs/server/reference/system-tables/information-schema/information-schema-tables/information-schema-columns-table),
[PostgreSQL CREATE](https://www.postgresql.org/docs/18/sql-createtable.html).
