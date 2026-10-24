# Datenbank- und Tabellenoperationen (API 0.8.0)

Additive Routen unter `/api/v8/`. Schema: [api-v0.8.0.json](api-v0.8.0.json). V1 bis v7 bleiben byteidentisch.
Alle Routen verlangen das Bearer-Token; jede Anfrage trägt die Verbindung wie in den früheren Versionen
(kein Passwort in Antworten, keine Persistenz). Unbekannte Felder und `null` werden abgewiesen, Anfragen sind
auf 1 MiB begrenzt. Es gibt kein freies SQL vom Client: Anfragen enthalten nur strukturierte Felder
(Namen, Modi, Optionswerte). Jede Anweisung wird serverseitig erzeugt, Bezeichner werden mit dem Treiberquoting
gesetzt, Werte aus dem Serverkatalog (Engines, Kollationen) werden zuvor gegen den Katalog geprüft.

## Plan/Ausführen-Muster

Jede Aktion hat zwei Routen. Beide nehmen dieselben Felder; die Ausführung verlangt zusätzlich `confirm` und
`expected_plan_sha256`.

| Aktion (`confirm.action`) | Vorschau (ohne Wirkung) | Ausführen | zerstörend (getippter Name) |
| --- | --- | --- | --- |
| `database_create` | `POST /api/v8/databases/create/plan` | `POST /api/v8/databases/create` | nein |
| `database_drop` | `POST /api/v8/databases/drop/plan` | `POST /api/v8/databases/drop` | ja, Datenbankname; Verbindungsdatenbank mit Zweitbestätigung |
| `database_collation` | `POST /api/v8/databases/collation/plan` | `POST /api/v8/databases/collation` | nein |
| `table_rename` | `POST /api/v8/tables/rename/plan` | `POST /api/v8/tables/rename` | ja, alter Tabellenname |
| `table_copy` | `POST /api/v8/tables/copy/plan` | `POST /api/v8/tables/copy` | nein |
| `table_truncate` | `POST /api/v8/tables/truncate/plan` | `POST /api/v8/tables/truncate` | ja, Tabellenname |
| `table_drop` | `POST /api/v8/tables/drop/plan` | `POST /api/v8/tables/drop` | ja, Tabellenname |
| `table_options` | `POST /api/v8/tables/options/plan` | `POST /api/v8/tables/options` | nein |

Dazu `GET /api/v8/capabilities` und `POST /api/v8/tables/dependencies` (lesend).

1. **Vorschau:** liefert `statements` (die exakten Anweisungen dieses Treibers), `warnings`, `affected_objects`,
   `dependencies` (Fremdschlüssel, Sichten mit Wirkung `blocks`, `breaks`, `follows`, `info` und Gewissheit
   `exact`/`textual`), `current_options`, `ddl_transactional`, `confirmation` (Aktion, zerstörend ja/nein, der
   zu tippende `target_name`, `current_database`) und `plan_sha256`. Zielobjekte werden aus dem Katalog
   aufgelöst; `target` und `confirmation.target_name` tragen die Katalogschreibweise, nie ungeprüften Anfragetext.
2. **Ausführen:** berechnet den Plan neu, prüft mit `dbplan.Check` Plan-Hash (409 `plan_mismatch`), Aktion und
   bei zerstörenden Aktionen den getippten Namen (400 `confirmation_mismatch`) und erst dann die Anweisungen.
   Der Hash deckt Treiber und normalisierte Anweisungen ab (Warnungen und Abhängigkeiten nicht). Die
   Verbindungsdatenbank (`database_drop`, MySQL/MariaDB) verlangt zusätzlich
   `confirm.acknowledge_current_database=true`; PostgreSQL kann sie selbst nicht löschen (400 `invalid_object_op`).
3. **Antwort:** immer `200` mit `outcome`:
   * `completed`: ausgeführt; `observation` zeigt das Ergebnis, `state=transactional` (PostgreSQL/SQLite innerhalb
     derselben Transaktion vor dem Commit; stimmt es nicht, wird zurückgerollt und `rejected/observation_mismatch`
     gemeldet) oder `observed` (getrennt nach dem impliziten Commit, kein atomarer Snapshot) oder `unavailable`.
   * `rejected`: nichts geändert; `reason` aus `permission_denied`, `object_exists`, `object_not_found`,
     `dependency_blocks`, `lock_timeout`, `in_use`, `invalid_value`, `database_error`, `observation_mismatch`,
     `canceled`. Rohe Treiberfehler erscheinen nie.
   * `outcome_unknown`: Verbindung oder Zeitlimit brachen nach dem Start ab, oder eine MySQL/MariaDB-Folge aus
     mehreren Anweisungen scheiterte nach einer bereits wirksamen Anweisung (`statements_executed` nennt den
     Stand). Es gibt keinen automatischen Retry; der Zustand ist zu prüfen.

Fehlerstatus vor der Ausführung: 400 `invalid_object_op` (fremdes oder fehlendes Feld, ungültiger Name, Systemobjekt,
Basistabelle erforderlich, Wert nicht im Serverkatalog, keine Änderung), 400 `confirmation_mismatch`,
404 `object_not_found` (auch nicht sichtbare Objekte), 409 `object_exists`, 409 `plan_mismatch`,
422 `unsupported_action` (Fähigkeit des Treibers „unsupported“, mit Grund), 422 `unsupported_feature`, 408, 413.

## Fähigkeiten je Treiber

`GET /api/v8/capabilities` liefert `drivers.<treiber>.<aktion>` mit `support` (`supported`, `limited`,
`unsupported`) und bei `limited`/`unsupported` einem Grund, außerdem `table_option_fields` je Treiber.
MariaDB ist Treiber `mysql`.

| Aktion | SQLite | MySQL/MariaDB | PostgreSQL |
| --- | --- | --- | --- |
| Datenbank anlegen/löschen/Kollation | nein (verwaltete Kennung) | anlegen ja; löschen eingeschränkt; Kollation nur Standard der Datenbank | anlegen eingeschränkt (libc-Locales, CREATEDB); löschen eingeschränkt (nicht die verbundene, keine offenen Sitzungen); Kollation nein |
| Tabelle umbenennen | eingeschränkt (nur `main`) | eingeschränkt (implizites Commit, Sichten werden ungültig); Verschieben per `new_schema` | ja; Verschieben per `new_schema` (zweite Anweisung, selbe Transaktion) |
| Tabelle kopieren | eingeschränkt (`CREATE TABLE AS`: keine Schlüssel, Indizes, Standardwerte) | eingeschränkt (`LIKE`, zwei nicht atomare Schritte, keine Fremdschlüssel/Trigger) | eingeschränkt (`LIKE ... INCLUDING ALL`, serial teilt die Sequenz, Identity-Zähler der Kopie nicht nachgeführt) |
| Tabelle leeren | eingeschränkt (`DELETE FROM`, AUTOINCREMENT-Zähler bleibt) | eingeschränkt (`TRUNCATE`, Zähler wird zurückgesetzt, scheitert bei Fremdschlüsselverweisen) | ja (`TRUNCATE`, optional `restart_identity`) |
| Tabelle löschen | ja | eingeschränkt (kein CASCADE) | ja (kein CASCADE) |
| Tabellenoptionen | nein | Engine, Kommentar, Kollation, Auto-Inkrement | nur Kommentar |

## Aktionen im Einzelnen

* `database_create`: `database` (Pflicht), `collation` (optional; MySQL/MariaDB aus `information_schema.COLLATIONS`
  mit abgeleitetem Zeichensatz, PostgreSQL nur libc-Kollationen, Anlage mit `TEMPLATE template0 ENCODING 'UTF8'`).
  Reservierte Systemnamen und vorhandene Namen werden abgewiesen.
* `database_drop`: `database` (Pflicht); Katalogtreffer mit exakt gleichem Namen, keine Systemdatenbank. MySQL/MariaDB
  nennen betroffene Objekte (höchstens 50) und Fremdschlüssel aus anderen Datenbanken (`blocks`).
* `database_collation`: nur MySQL/MariaDB, ändert `CHARACTER SET`/`COLLATE` der Datenbank, nicht der Tabellen.
* `table_rename`: `table`, `new_name` und/oder `new_schema` (mindestens eine Änderung); Ziel darf nicht existieren.
* `table_copy`: `table`, `new_name` (Pflicht), `new_schema`, `mode` = `structure`, `data` (in eine vorhandene Tabelle
  mit allen Quellspalten) oder `both` (Standard). Daten kopiert `INSERT ... SELECT` mit Spaltenliste aus dem Katalog;
  generierte und versteckte Spalten werden ausgelassen. `observation.rows` und `source_rows` sind exakte Zählungen.
* `table_truncate`: `table`, `restart_identity` (nur PostgreSQL). `observation.rows` ist die Zeilenzahl danach,
  `observation.auto_increment` der beobachtete Zähler (MySQL/MariaDB, SQLite `sqlite_sequence`).
* `table_drop`: `table`; nur Basistabellen, keine Sichten und keine Systemschemas.
* `table_options`: `engine` (muss in `information_schema.ENGINES` unterstützt sein), `comment` (höchstens 2048
  Zeichen, wird als Literal mit verdoppelten Anführungszeichen unter `NO_BACKSLASH_ESCAPES` bzw.
  `standard_conforming_strings=on` gesetzt), `collation` mit `collation_mode` `default` (neue Spalten) oder
  `convert` (wandelt Spalten um, Datenverlust möglich), `auto_increment`. Optionen, die dem aktuellen Wert
  entsprechen, entfallen; bleibt nichts übrig, ist die Anfrage `invalid_object_op`.
* `tables/dependencies`: Fremdschlüssel (`foreign_key_in`, `foreign_key_out`) und Sichten, höchstens 100.

## Konventionen für Clients und bekannte Grenzen

* **Kein Replay:** `plan_sha256` ist zustandslos und deckt Treiber und Anweisungen ab. Dieselbe Bestätigung kann nach einem
  `completed` erneut wirken, wenn die Tabelle inzwischen wieder Daten hat (`table_truncate`; `table_drop`, `table_rename` und
  `database_drop` scheitern dann mit `object_not_found`). Clients müssen die Aktion nach `completed` und `outcome_unknown`
  sperren und für eine weitere Ausführung einen neuen Plan laden und neu bestätigen lassen; kein automatischer Retry.
* **Zeitfenster:** Plan und Ausführung bilden ein Fenster von Millisekunden (Plan wird auf derselben Sitzung unmittelbar vor
  der Ausführung neu berechnet). Ersetzt eine andere Sitzung in diesem Fenster ein Objekt gleichen Namens, wirkt die
  namensbasierte Anweisung auf das neue Objekt. Eine Sperre schließt dieses Fenster nicht vollständig und wird nicht versprochen.
* **Kindobjekte:** PostgreSQL-Vererbungskinder und Partitionen sowie MySQL/MariaDB-Partitionen stehen in `affected_objects`
  und `warnings`. `table_truncate` einer Tabelle mit Vererbungskindern verwendet `TRUNCATE ONLY`; eine partitionierte
  Tabelle wird samt Partitionen geleert, `table_drop` entfernt Partitionen mit.
* `table_options` mit Engine-Wechsel oder `collation_mode=convert` ist zerstörend (`confirmation.destructive`, getippter Name).
* Die Verbindungsdatenbank wird in MySQL/MariaDB zusätzlich über `SELECT DATABASE()` der Sitzung erkannt.

## Abhängigkeiten und Wirkung

Die Vorschau warnt vor Folgen, ohne sie zu verhindern oder zu umgehen: Fremdschlüsselprüfung bleibt aktiv, es gibt
kein CASCADE. Löschen/Leeren einer referenzierten Tabelle wird vom Server abgelehnt (`dependency_blocks`); das Umbenennen
führt Fremdschlüssel nach (SQLite, PostgreSQL, MySQL InnoDB), Sichten nur in SQLite und PostgreSQL; in MySQL/MariaDB werden
Sichten ungültig. PostgreSQL verweigert das Löschen einer von Sichten verwendeten Tabelle. Sichten werden in
MySQL/MariaDB und SQLite am Quelltext erkannt (`certainty=textual`); der Server zeigt `VIEW_DEFINITION` nur mit dem Recht
SHOW VIEW, Trigger werden nicht ausgewertet. Die Suche über alle Sichten kann auf Servern mit sehr vielen Sichten
dauern und endet am Zeitlimit.

DDL ist nicht pauschal transaktional: MySQL/MariaDB bestätigen jede DDL-Anweisung implizit, PostgreSQL und SQLite führen
Tabellen-DDL in einer Transaktion aus, `CREATE/DROP DATABASE` laufen in PostgreSQL außerhalb einer Transaktion.
Vor der Ausführung setzt die Sitzung ein kurzes Sperrzeitlimit (10 s), damit Sperren zu `lock_timeout` statt zu
einem unbekannten Ausgang führen.

## Grenzen

`timeout_ms` 0 bedeutet 30 000, höchstens 60 000. Plan höchstens 8 Anweisungen, 100 Abhängigkeiten, Antwort höchstens 8 MiB.
Namen neuer Objekte: nicht leer, ohne NUL und Steuerzeichen, ohne führende/folgende Leerzeichen, MySQL höchstens 64 Zeichen,
PostgreSQL 63 Byte, SQLite 255 Byte (kein Präfix `sqlite_`). Systemdatenbanken und Systemschemas sind ausgeschlossen.
