# Partitionen und Tabellenpräfix-Stapel (API 0.22)

[Schema](api-v0.22.0.json), additiv auf API 0.12; v1 bis v12 bleiben byteidentisch.
Die Nummer 0.22 ist endgültig. Reserviert für andere Pakete sind 0.17 (W-P17), 0.18 (W-P12), 0.19 (W-P13), 0.20 (W-P27) und 0.21 (W-P28); diese Lücken sind keine Voraussetzung. Der Wächter pinnt nur die in diesem Stand vorhandenen Vorgänger (v1 bis v16).
Alle Routen benötigen Bearer-Authentifizierung und eine flüchtige Verbindung.
1 MiB Anfrage, 30 Sekunden Standardzeitlimit, höchstens 60 Sekunden; unbekannte
Felder und `null` werden vom HTTP-Decoder abgewiesen. Keine neuen Abhängigkeiten.

| Route | Zweck |
| --- | --- |
| `GET /api/v22/capabilities` | Fähigkeiten je Treiber und Partitionsaktion; Präfixstapel als Feature |
| `POST /api/v22/tables/partitions/list` | `connection`, `table`, optional `timeout_ms` |
| `POST /api/v22/tables/partitions/plan` | strukturierte Partitionsaktion, Vorschau ohne Ausführung |
| `POST /api/v22/tables/partitions/execute` | gleiche Anfrage mit `confirm`, `expected_plan_sha256` |
| `POST /api/v22/tables/prefix/list` | `connection`, `prefix`, optional `schema`, `timeout_ms` |
| `POST /api/v22/tables/prefix/plan` | Präfixaktion als gemeinsamer Stapelplan |
| `POST /api/v22/tables/prefix/execute` | gleicher Stapel mit Bestätigung und Gesamt-Hash |

## Partitionen

Die Liste enthält Katalogschreibweise der Tabelle, `method`, `expression` und bis
zu 100 Partitionen mit `table`, `bound`, `estimated_rows` (bei unbekanntem Wert
`null`). Die Expression und Grenzen sind reine Anzeige, niemals Eingabe-SQL.
Nicht partitionierte Tabellen liefern eine leere Liste und leere Methode.
Subpartitionen in MySQL/MariaDB werden ausdrücklich abgewiesen. PostgreSQL zeigt
direkte deklarative Kinder; gewöhnliche Vererbung ist keine Partitionierung.

`action` in der Plananfrage:

| Aktion | MySQL/MariaDB | PostgreSQL |
| --- | --- | --- |
| `define` | bestehende Tabelle partitionieren oder neu partitionieren; RANGE/LIST/HASH/KEY | kein automatischer Umbau bestehender Tabellen |
| `add` | RANGE/LIST mit Definitionen; HASH/KEY mit `count` | `CREATE TABLE … PARTITION OF` |
| `drop` | RANGE/LIST, Daten werden gelöscht | `DROP TABLE` des Katalogkindes, Daten werden gelöscht |
| `reorganize` | ausgewählte RANGE/LIST-Partitionen aufteilen oder zusammenführen | nicht unterstützt |
| `coalesce` | HASH/KEY um `count` Partitionen verringern | nicht unterstützt |
| `remove` | `REMOVE PARTITIONING`, Daten bleiben erhalten | nicht unterstützt |
| `analyze` | `ANALYZE PARTITION` | `ANALYZE` der ausgewählten Kinder |
| `check`, `optimize`, `rebuild`, `repair` | entsprechende Partitionswartung, soweit Engine unterstützt | nicht unterstützt |
| `truncate` | `TRUNCATE PARTITION` | `TRUNCATE TABLE` der ausgewählten Kinder |
| `attach`, `detach` | nicht unterstützt | deklaratives `ATTACH/DETACH PARTITION` |

SQLite meldet Partitionierung stets als nicht unterstützt (422), ohne eine
Tabelle oder Datei anzulegen. Präfixstapel stehen auch für SQLite zur Verfügung.
InnoDB-REPAIR wird vorab abgewiesen. OPTIMIZE kann bei InnoDB die gesamte Tabelle
neu aufbauen; die Vorschau warnt davor. Wartungsantworten mit MySQL-Fehlerzeilen
führen zu `outcome_unknown`, weil der Server bereits Teilwirkungen haben kann.

`define` akzeptiert `method`, `column` (exakt eine vorhandene Katalogspalte), bei
HASH/KEY `count` (1 bis 100), bei RANGE/LIST `definitions`. Freie Ausdrücke,
mehrspaltige Schlüssel, Subpartitionen, SQL-Optionen und automatische Migrationen
gehören nicht zu diesem strukturierten Teilumfang. Vorhandene Ausdrücke bleiben
lesbar. Die Server prüfen Typ-, Schlüssel-, Grenz- und Enginebedingungen.

`partitions` wählt bestehende Katalogpartitionen für Wartung, Löschen, Leeren,
Abtrennen und REORGANIZE. Doppelte oder unbekannte Namen werden abgewiesen. PostgreSQL akzeptiert auch
qualifizierte Katalognamen (`schema.name`); mehrdeutige Namen werden abgewiesen.
Neue Namen in `definitions` werden validiert und mit `dbplan` gequotet; ATTACH
löst eine vorhandene Basistabelle im Schema der Elterntabelle aus Metadaten auf.

Jede Definition hat `name` und genau die Felder ihrer Methode:

- MySQL/MariaDB RANGE: `less_than`.
- PostgreSQL RANGE: `from`, `to` (untere Grenze inklusive, obere exklusiv).
- LIST: `values`, bis 100 Werte.
- PostgreSQL HASH: `modulus`, `remainder`.

Grenzwerte sind Zeichenketten als Daten: vorzeichenbehaftete 64-Bit-Ganzzahlen
werden kanonisch numerisch gesetzt, andere Werte als gequotete Textliterale;
`MINVALUE`/`MAXVALUE` sind nur in RANGE-Grenzen Schlüsselwörter. Kein SQL wird aus
Anfragegrenzen übernommen. Nicht passende Grenzwerte weist der Server zurück.

## Tabellenpräfix-Stapel

`prefix` ist nicht leer und wird wörtlich verglichen, einschließlich `%` und `_`.
`schema` ist standardmäßig `main` (SQLite), Verbindungsdatenbank (MySQL/MariaDB)
oder `public` (PostgreSQL). Systemschemas sind ausgeschlossen. Es werden sämtliche
passenden Basistabellen ausgewählt, keine Sichten. Mehr als 100 Treffer werden
abgewiesen; es gibt keinen still gekürzten Schreibstapel.

`action`: `drop`, `truncate`, `replace`, `remove`, `add`, `copy`.
`replace` setzt `new_prefix` anstelle des ausgewählten Präfixes, `remove` entfernt
es, `add` setzt `new_prefix` vor den vollständigen Katalognamen. `copy` ersetzt
wie `replace` das Präfix und kopiert Struktur und Daten mit dem W-P07-Modus `both`.
Zielnamen entstehen ausschließlich aus validiertem Präfix und Katalognamen;
freie Zieltabellenlisten gibt es nicht. Kollisionen und leere Zielnamen werden
vor der Ausführung abgewiesen. Alle Pläne werden vor der ersten Änderung gebaut.
Die konkrete W-P07-Planung wird wiederverwendet, einschließlich Kindobjekten,
Abhängigkeiten, Quoting und Kopiergrenzen (siehe [Objektoperationen](OBJECT-OPS-API.md)).

## Teilzustand im Präfixstapel

`PrefixStructureResult.tables` nennt je Katalogtabelle des Stapels in Planreihenfolge
`status` (`applied`, `not_applied`, `unknown`), `statements_total` und
`statements_executed`. Ein Stapel wird nie stillschweigend halb ausgeführt: PostgreSQL
und SQLite führen ihn in einer Transaktion aus (Fehler: alle `not_applied`, unsicherer
Commit: alle `unknown`). MySQL/MariaDB bestätigen DDL implizit und haben keinen
Stapelrollback; nach einem Fehler sind bereits abgeschlossene Tabellen `applied`, die
Tabelle der fehlgeschlagenen oder unterbrochenen Anweisung `unknown` (nie als nicht
angewendet behauptet), alle folgenden `not_applied`, `outcome` ist `outcome_unknown`
und `message` nennt den Teilzustand. Es gibt keinen Retry und kein automatisches
Zurückrollen; der Client lädt eine neue Vorschau und bestätigt neu.

## Bestätigung und Ergebnis

Die Vorschau enthält exakte `statements`, `warnings`, `affected_objects`, `action`,
`confirmation` und einen Gesamt-`plan_sha256` aus `dbplan`. Ausführung baut auf
derselben Verbindung den vollständigen Plan neu und prüft Hash und Aktion.
`confirm.action` lautet `partition_<action>` beziehungsweise `prefix_<action>`.
Zerstörende Aktionen verlangen `confirm.target_name`: Katalogname der Elterntabelle
beziehungsweise wörtliches gemeinsames Präfix der Katalogtreffer. Nur ANALYZE,
CHECK und Präfixkopien brauchen keinen getippten Namen. Ein neuer Treffer ändert
den Stapelhash. Warnungen nennen Datenverlust bei DROP/TRUNCATE und Sperren bei
REORGANIZE. Neue Definitionen und beobachtete Kinder stehen in `affected_objects`.

Partitionsausführung liefert `StructureResult`, Präfixausführung `PrefixStructureResult` (gleiche Felder plus `tables`), jeweils mit `completed`, `rejected` oder
`outcome_unknown`, Anweisungszähler, `atomic` und `observation=null`. Es wird kein
zusätzlich beobachteter Nachzustand behauptet. PostgreSQL/SQLite verwenden eine
Transaktion; MySQL/MariaDB können nach erfolgreicher erster Anweisung Teiländerungen
behalten. Verbindungsfehler und unsicheres Commit werden nie automatisch wiederholt.
`rejected` bedeutet keine bleibende Änderung durch die Anweisungen; externe
Triggerwirkungen und Sequenzeffekte sind davon nicht pauschal umfasst.

Fehler vor Ausführung folgen W-P07: 400 `invalid_object_op` oder
`confirmation_mismatch`, 404 `object_not_found`, 409 `object_exists` oder
`plan_mismatch`, 422 `unsupported_action`, Größen-/Zeitlimits wie bisher.
Keine rohen Treiberfehler, Verbindungsdaten oder Dateipfade in Antworten.
Der Hash ist kein Replay-Schutz und bindet keine Objektidentität. Namensänderungen
zwischen Katalogabfrage und DDL sowie ABA bleiben möglich. Nach Abschluss oder
unbekanntem Ausgang müssen Clients eine neue Vorschau mit erneuter Bestätigung laden.

Semantikquellen: [PostgreSQL-Partitionierung](https://www.postgresql.org/docs/18/ddl-partitioning.html),
[MySQL-Partitionsoperationen](https://dev.mysql.com/doc/refman/8.4/en/alter-table-partition-operations.html).
