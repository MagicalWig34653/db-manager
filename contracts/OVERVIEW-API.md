# Lesende Übersicht (API 0.7.0)

Additive, rein lesende Routen unter `/api/v7/`. Schema: [api-v0.7.0.json](api-v0.7.0.json). V1 bis v6 bleiben
byteidentisch. Alle Routen verlangen das Bearer-Token; jede Anfrage trägt die Verbindung wie in den
früheren Versionen (kein Passwort in der Antwort, keine Persistenz). Unbekannte Felder werden abgewiesen,
Anfragen sind auf 1 MiB begrenzt. Es wird nie ein Bezeichner aus der Anfrage ungeprüft in SQL übernommen:
Katalogabfragen binden Werte, `SHOW CREATE` und `COUNT(*)` verwenden Namen, die zuvor aus dem Katalog
gelesen und mit dem Treiberquoting gesetzt wurden.

| Route | Zweck |
| --- | --- |
| `GET /api/v7/capabilities` | Fähigkeiten je Treiber und Aktion (`supported`, `limited` mit Grund, `unsupported` mit Grund), Grenzen |
| `POST /api/v7/databases` | sichtbare Datenbanken mit Zeichensatz, Kollation, Objektzahl, Größe |
| `POST /api/v7/objects` | Tabellen/Views eines Schemas mit Zeilenzahl, Engine, Kollation, Größe, Kommentar |
| `POST /api/v7/definition` | Definition eines sichtbaren Objekts |
| `POST /api/v7/server-info` | Version, Laufzeit, Zeichensätze, Kollationen, Engines, Plugins |

Treiber sind `sqlite`, `mysql` (MariaDB als `mysql`, `variant` in server-info ist `mariadb`) und `postgres`.

## Paging und Limits

`limit` 0 bedeutet 200, höchstens 1000; `offset` höchstens 1 000 000; `timeout_ms` 0 bedeutet 10 000,
höchstens 30 000. Verletzungen liefern 400 `invalid_overview`. Antworten tragen `total`, `offset`, `limit`,
`has_more`. Sortierung ist stabil nach Name (bei PostgreSQL nach Schema und Name). Antwort höchstens 8 MiB
(`result_too_large`, 422).

## Schätzung und exakte Werte

* Datenbankliste: `estimated=true` bei MySQL/MariaDB (Größe und Objektzahl stammen aus `information_schema`-
  Statistiken); `object_count` ist `null`, wo der Treiber keine Zählung liefert (PostgreSQL nur für die
  verbundene Datenbank, SQLite nur die eigene Datenbank). `size_bytes` ist `null`, wenn nicht lesbar
  (SQLite: Sandbox verbietet die PRAGMAs; PostgreSQL ohne CONNECT-Recht).
* Objektübersicht: `rows.state` ist `estimated` (MySQL/MariaDB `TABLE_ROWS`, PostgreSQL `reltuples` nach dem
  ersten ANALYZE), `exact` (nur mit `exact_rows=true`) oder `unknown` (Views, SQLite ohne Zählung,
  nie analysierte PostgreSQL-Tabellen, abgebrochene Zählung). Eine Schätzung wird nie als exakt ausgegeben.
* `exact_rows=true` zählt nur Basistabellen der angeforderten Seite mit `COUNT(*)`, höchstens 100 Objekte je
  Anfrage (Standardseite dann 100). Läuft das Zeitlimit ab, bleibt die Katalogseite erhalten, die übrigen
  Zeilen sind `unknown` und `exact_incomplete=true`. Abbruch der Anfrage selbst liefert 408.

## Objekte und Sichtbarkeit

Nur Objekte, die der Verbindung laut Katalog sichtbar sind (`information_schema`-Regeln; PostgreSQL mit
derselben Rechteprüfung). Fehlende Rechte ergeben leere Listen beziehungsweise 404, keine Fehlermeldungen
mit Fremdnamen. `kind`: `table`, `view`, `virtual` (SQLite), `materialized_view`, `foreign_table` (PostgreSQL).
`schema` bei MySQL/MariaDB ist die Datenbank (Standard: die der Verbindung), bei PostgreSQL leer = alle
nicht-systemischen Schemas, bei SQLite `main`.

## Definition

`object` ist `{schema?, name}`. Quelle: MySQL/MariaDB `SHOW CREATE TABLE|VIEW` (`source=show_create`),
SQLite `sqlite_schema` (`source=sqlite_schema`), PostgreSQL aus dem Katalog rekonstruiert
(`derived=true`, `source=catalog_reconstruction`, `notice` und `warnings` nennen Abweichungen: Rechte,
Kommentare, Trigger, Storage-Parameter, Partitionen fehlen). Unbekanntes, nicht sichtbares oder
Systemobjekt: 404 `object_not_found`. Definition höchstens 1 MiB.

## Serverinformationen

Version, `variant`, `uptime_seconds` (MySQL/MariaDB/PostgreSQL), aktuelle Datenbank, nicht geheime
Verbindungsangaben (Host, Port, Datenbank, Benutzer, TLS-Modus; bei SQLite nur die Datenbankkennung),
Sitzungsvariablen (Zeichensatz/Kollation bzw. Encoding), Zeichensätze, Kollationen, Engines
(PostgreSQL: Table Access Methods) und Plugins (PostgreSQL: Extensions). Listen sind auf 1000 Einträge
begrenzt und tragen `*_truncated`. Kein Passwort, keine Hostpfade oder Datenverzeichnisse.

## Fehler

400 `invalid_request`/`invalid_overview`, 401, 404 `object_not_found`, 408 `query_canceled`,
422 `database_error`/`unsupported_feature`/`result_too_large`, 413. Rohe Treiberfehler erscheinen nicht.
