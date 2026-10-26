# Indizes und Schlüssel (API 0.13.0)

[Vertrag](api-v0.13.0.json), additiv zu v12. Alle Vorgängerverträge bleiben unverändert.
Bearer-Authentifizierung, strikte JSON-Objekte ohne Nullwerte, höchstens 1 MiB je Anfrage.
MariaDB verwendet den Treiber `mysql`. Kein freies SQL und keine neuen Abhängigkeiten.

## Routen und Bestätigung

- `GET /api/v13/capabilities`: Fähigkeiten je Treiber und Aktion, mit Einschränkungen.
- `POST /api/v13/indexes/plan`: `connection`, `table`, `action`, aktionsabhängige Felder und optional `timeout_ms`.
- `POST /api/v13/indexes/alter`: dieselben Felder plus `expected_plan_sha256` und `confirm`.

| action / confirm.action | weitere Felder | getippter Tabellenname |
| --- | --- | --- |
| `index_create` | `index` | nein |
| `index_alter` | `name` des bisherigen Indexes, vollständiger neuer `index` | ja |
| `index_drop` | `name` | ja |
| `index_rename` | `name`, `new_name` | ja |
| `primary_set` | `index` mit `kind=primary`; ersetzt einen vorhandenen PK | ja |
| `primary_drop` | keine; bisheriger Name aus Metadaten | ja |
| `foreign_key_create` | `foreign_key` | nein |
| `foreign_key_alter` | `name`, vollständiger neuer `foreign_key` | ja |
| `foreign_key_drop` | `name` | ja |

`confirm.target_name` ist der exakte **Tabellenname aus dem Katalog**, kein Index- oder
Constraintname. Bestehende Tabellen, Spalten, Indizes und Constraints werden vor dem Quoting
im Katalog aufgelöst. Nur neue Namen sind validierte Deklarationen (höchstens 63 Byte,
keine Rand-Leerzeichen, Steuerzeichen oder NUL). Bezeichner ausschließlich über `dbplan`.

Die Vorschau liefert exakte `statements`, Warnungen zu Sperren, Laufzeit, Speicherbedarf und
Verlust von Integritätsregeln, `affected_objects`, tabellenweite `dependencies` mit `effect=info`,
`destructive`, `target_name`, `ddl_transactional`, `contains_null` sowie `plan_sha256`.
Abhängigkeiten bezeichnen mögliche Betroffenheit, keine vollständige Sperrprognose für einen
bestimmten Index. Selbstreferenzen und Trigger/externe Verweise sind nicht vollständig erfasst.
Die Engine prüft ohne CASCADE und ohne Abschaltung der Fremdschlüsselprüfung.

Der Hash deckt Treiber und normalisierte Anweisungen ab, keine Verbindung, Warnungen oder
Prüfzähler. Die Ausführung plant auf derselben Sitzung neu und prüft mittels `dbplan.Check`.
Falscher Hash: 409 `plan_mismatch`; falsche Aktion/getippter Name: 400 `confirmation_mismatch`.
Kein Replay-/ABA-Schutz: Clients sperren nach Abschluss oder unbekanntem Ausgang und verlangen
neue Vorschau und Bestätigung. Namensersetzung durch konkurrierende DDL bleibt ein Zeitfenster.

## Definitionen und Fähigkeiten

`index`: `name`, `kind` (`primary`, `unique`, `index`, `fulltext`, `spatial`), ein bis
16 geordnete `terms` (`column`, optional `direction=asc|desc`, `prefix_length`, `expression`).
`index_alter` ersetzt die Definition durch DROP und CREATE; Constraintindizes lassen sich nicht
als normale Indizes ändern/entfernen. Für Primärschlüssel dienen die eigenen Aktionen.
MySQL/MariaDB führen den Primärschlüssel unabhängig vom deklarierten Constraintnamen als `PRIMARY`.

| Bereich | SQLite | MySQL/MariaDB | PostgreSQL |
| --- | --- | --- | --- |
| normale/UNIQUE-Indizes | nur CREATE/DROP, Richtung möglich | CREATE/Ersetzen/Umbenennen/DROP, Richtung und Präfix | CREATE/Ersetzen/Umbenennen/DROP, Richtung |
| Primärschlüssel | nicht unterstützt, kein Neuaufbau | setzen/ersetzen/entfernen | setzen/ersetzen/entfernen |
| Fremdschlüssel | nicht unterstützt, kein Neuaufbau | InnoDB | reguläre Tabellen |
| FULLTEXT/SPATIAL | nein | InnoDB, geeignete Spaltentypen erforderlich | nein |
| Ausdruck/Teilindex | nein | nein | eingeschränkt wie unten |

PostgreSQL unterstützt `expression=lower|upper` ausschließlich für Textspalten; erzeugt wird
`pg_catalog.lower/upper` über den kataloggeprüften Spaltennamen. `predicate` enthält ausschließlich
`column` und `operator=is_null|is_not_null`. Keine Ausdrücke als SQL-Text, Funktionen des Clients,
Operatoren oder Literalfragmente. PRIMARY erlaubt keine Ausdrücke, Präfixe, Richtung oder Bedingung.
FULLTEXT/SPATIAL erlauben keine Richtung/Präfixe; SPATIAL nur eine nicht nullbare Spalte.
Die Engine entscheidet über Versions-, Typ-, Kollations-, SRID- und Sortierungsunterstützung.
PostgreSQL-Vererbung/Partitionen/Systemschemas, SQLite-Systemtabellen und andere MySQL-Engines
sind ausgeschlossen. Bestehende Identity-/AUTO_INCREMENT-Spalten sind zulässig;
andere generierte oder versteckte Spalten werden konservativ abgewiesen. Kein CONCURRENTLY, INCLUDE, Opclass oder allgemeiner Ausdruckseditor.

Bei UNIQUE/PRIMARY liefert `duplicate_groups` die Anzahl doppelter Wertgruppen, unter Beachtung
der Indexterme und Teilbedingung; bei UNIQUE werden NULL-Schlüssel ausgenommen. PRIMARY prüft
zusätzlich vorhandene NULL-Werte. Das kann vollständige Scans benötigen. Ein positiver Befund
führt nach gültiger Bestätigung vor DDL zu `rejected/invalid_value`; die Engine sichert zusätzlich
gegen konkurrierende Datenänderungen. Timeout 0 bedeutet 30 s, maximal 60 s, Sperrwartezeit 10 s.

`foreign_key`: `name`, geordnete `columns`, `referenced_table`, gleich viele `referenced_columns`,
`on_delete`, `on_update` (`no_action`, `restrict`, `cascade`, `set_null`, PostgreSQL auch `set_default`).
Spalten und Referenztabelle werden im Katalog aufgelöst; Katalogtypen müssen konservativ exakt
übereinstimmen. SET NULL benötigt nullbare lokale Spalten. Referenzschlüssel, Kollationen und
vorhandene Daten prüft danach die Engine. CASCADE/SET-Aktionen können bei späteren Datenänderungen
Daten ändern/löschen; die Vorschau warnt ausdrücklich. Die Beziehungsübersicht bleibt API v5.

## Ergebnis und Grenzen

`completed`, `rejected`, `outcome_unknown` mit festen Gründen und Meldungen; keine rohen
Treiberfehler, SQL-Texte oder Dateipfade in Fehlermeldungen. SQL steht ausschließlich in der
bewusst angeforderten Vorschau. Ausführung verwendet den geprüften W-P08-DDL-Runner:
SQLite/PostgreSQL transaktional mit Tabellenbeobachtung vor Commit, MySQL/MariaDB eine kombinierte
ALTER-Anweisung mit implizitem Commit. Unbekannte Ausführungs-/Commitbestätigung bedeutet
`outcome_unknown`, niemals automatischen Retry. `statements_executed` beschreibt bestätigte
Anweisungen, keine Garantie für den endgültigen Zustand.

`description` beobachtet Spalten; Index-/FK-Metadaten sind getrennt über API v5 nachzulesen.
`observation=transactional|observed|unavailable`, `atomic` nur nach bestätigtem Transaktionscommit.
Ausgeführte MySQL-DDL bleibt bei fehlender nachträglicher Beobachtung `completed`.
Schema-/Objektsichtbarkeit und tatsächliche Datenbankrechte begrenzen die Vorschau.
Weitere Fehler: 400 `invalid_indexes`, 404 `object_not_found`/`column_not_found`/`table_not_found`,
409 `object_exists`, 422 `unsupported_action`/`unsupported_feature`, bestehende Auth-/Größen-/Zeitfehler.
