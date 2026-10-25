# Suche und Ersetzen – vorläufig API 0.15.0

[Schema](api-v0.15.0.json), [synthetische Fixtures](fixtures/search/README.md).
Kandidat für W-P14, kein Vertragsfreeze und kein Web-Pin. Alle Bestandsrouten und
Schemas bis einschließlich 0.8 bleiben unverändert. Jede Route verlangt Bearer-Auth,
eine flüchtige Verbindung wie bisher und strikt strukturierte JSON-Anfragen bis 1 MiB.
Unbekannte Felder, explizites `null`, fehlende Pflichtfelder und ungültige Grenzen
werden abgewiesen. Keine Anfrage enthält freies SQL.

| Route | Zweck |
| --- | --- |
| `GET /api/v15/capabilities` | Modi, Treibergrenzen und Limits |
| `POST /api/v15/search` | Mehrtabellensuche mit Trefferzählung und Vorschau |
| `POST /api/v15/tables/browse` | Bestehende Zeilenansicht mit erweiterten UND-Filtern |
| `POST /api/v15/replace/plan` | Ersetzungsvorschau ohne UPDATE oder Commit |
| `POST /api/v15/replace` | Bestätigte transaktionale Ersetzung |

## Suche und Filter

`targets` enthält 1–20 ausgewählte Tabellen, je 1–32 eindeutige Textspalten.
Bezeichner werden frisch gegen den Katalog geprüft und dialektgerecht gequotet;
Sichten, virtuelle/fremde Tabellen und nichttextartige Spalten sind ausgeschlossen.
Texttypen umfassen CHAR/VARCHAR/TEXT/CLOB einschließlich Größenvarianten. JSON,
Arrays, Binärtypen und beliebige Typkonvertierungen sind ausgeschlossen. SQLite
prüft zusätzlich die tatsächliche Speicherklasse je Suchspalte; BLOB-Werte einer
TEXT-Spalte werden weder durchsucht noch als Vorschautext dargestellt.

`term` ist nichtleer, höchstens 4096 UTF-8-Byte, ohne NUL. Modi:

- `substring`: literaler Teilstring; `%`, `_` und `!` werden mit `!` escaped.
  Alle drei Dialekte verwenden ausdrücklich `LIKE <Parameter> ESCAPE '!'`.
- `all_words`: höchstens 32 durch Leerraum getrennte Wörter; alle müssen in der
  Zeile vorkommen, dürfen aber in verschiedenen ausgewählten Spalten stehen.
- `exact`: DB-eigene Gleichheit; Kollation und CHAR-Leerzeichenregeln gelten.
- `regex`: native PostgreSQL-POSIX-Regex (`~`), MySQL/MariaDB `REGEXP`;
  SQLite nicht verfügbar, da der verwaltete Adapter keine Erweiterung nachlädt.

`max_rows` ist ein gemeinsames Trefferbudget über alle Tabellen (Standard 100,
maximal 1000). Ein gebundener Probeabruf über das verbleibende Budget hinaus
ermittelt `count_exact`. Bei `false` ist `count` nur eine Untergrenze, auch 0 nach
Budgetverbrauch; kein vollständiges COUNT wird behauptet. Tabellen werden in
Anfragereihenfolge abgearbeitet. `preview_rows`: Standard 5, maximal 20 je Tabelle;
Textvorschau auf 256 Zeichen je Wert gekürzt. Die Vorschau enthält nur ausgewählte
Spalten, keine Zeilenidentität oder vollständigen Snapshots. Keine stabile Reihenfolge
oder gemeinsame Snapshotgarantie über Tabellen hinweg. Drilldown erfolgt über
`tables/browse`; nicht über gekürzte Vorschauwerte mutieren.

Die erweiterte Zeilenansicht erhält Paging/Sortierung und typisierte Filterwerte
von API 0.2. Zusätzliche Operatoren: `ne`, `lt`, `lte`, `gt`, `gte`, `contains`
(literaler Teilstring) und `like` (bewusstes SQL-LIKE-Muster mit Escape `!`). Zwei
Bereichsgrenzen werden als zwei UND-Filter ausgedrückt. NULL bleibt `is_null`/
`is_not_null`; NULL bei normalen Vergleichen folgt SQL-Dreiwertlogik. Keine freien
Operatoren, OR-Gruppen, Joins, gespeicherten Suchen oder vollständige QBE-Oberfläche.

## Ersetzungsplan und Ausführung

Anfrage: `table`, `column`, `term`, `replacement` (auch leer), `mode` und optionale
Limits. `substring` nutzt natives `REPLACE`, `exact` einen gebundenen Ersatzwert
bei DB-Gleichheit, `regex` natives `REGEXP_REPLACE` (PostgreSQL mit Flag `g`).
Regexersatz und Rückreferenzen folgen dem jeweiligen Treiber; kein gemeinsamer
Rückreferenzstandard. MariaDB wird über `mysql` angesprochen. Geprüfte Zielversionen
sind MySQL 8.4, MariaDB 12.3, PostgreSQL 18.6; lokale Live-Nachweise siehe Handoff.

Literalersetzung kann im Gegensatz zum LIKE-Vergleich case-sensitiv sein. Daher
zählt `affected_rows` ausschließlich Werte, die sich durch die native Funktion
wirklich ändern. No-op-Zeilen lösen kein UPDATE aus. Groß-/Kleinschreibung und
Regex-Kollation werden nicht durch eine API-eigene Normalisierung vereinheitlicht.

Ersetzen verlangt eine transaktionale Basistabelle mit stabilem, nichtnullablem
Primärschlüssel. MySQL: ausschließlich InnoDB. PostgreSQL: keine Partitionierung,
Vererbung oder Rewrite-Regeln. Zielspalten dürfen weder PK, Identity, generiert noch
binär sein. Werte vor/nach Ersetzung höchstens 4096 Byte. Für den vollständigen
Konfliktvergleich werden alle Spalten gelesen, einschließlich Binär-/Präzisionszellen;
es gelten zusätzlich die Bestandsgrenzen von 1 MiB je Zelle und 8 MiB je Ergebnis.
Übergroße oder nicht darstellbare Bestände werden sicher abgewiesen.

**Konservative Obergrenze:** `max_rows` begrenzt beim Ersetzen den vollständigen
Tabellenbestand, nicht nur Treffer (Standard 100, maximal 1000). Überlauf ergibt
422 `replace_limit`; es gibt keine stillschweigende Teilersetzung.

Der Plan enthält Anzahl tatsächlich geänderter Zeilen, höchstens 20 Beispiele mit
PK und typisiertem Vorher/Nachher, gebundene UPDATE-Vorlage, Warnungen und
`plan_sha256`. SHA-256 ist domänengetrennt und deckt Treiber, Vorlage, kanonische
Metadaten, Spalte, Such-/Ersatzwert, Modus, sämtliche vollständigen Zeilen in
PK-Reihenfolge und berechnete Nachherwerte ab. Keine Credentials im Hashmaterial.

Ausführung verlangt dieselbe Anfrage plus `expected_plan_sha256` und
`confirm:{action:"replace",target_name:<getippter Tabellenname>,affected_rows:<Anzahl>}`.
Der Plan wird in einer neuen Transaktion unter Sperren neu berechnet; abweichender
Hash ergibt HTTP409 `plan_mismatch`, falsche Bestätigung HTTP400
`confirmation_mismatch`. Keine Änderung vor erfolgreichem Vergleich.

SQLite verwendet `BEGIN IMMEDIATE`, aktive FK-Prüfung, exakte Speicherklassen,
BINARY-PK-Vergleiche und `UPDATE OR ABORT` gegen fremde REPLACE-Löschungen.
Netzwerktreiber verwenden Read-Committed und `SELECT ... FOR UPDATE` über den
vollständigen begrenzten Bestand. PostgreSQL sperrt zusätzlich die Relation
(SHARE beim Plan, SHARE ROW EXCLUSIVE bei Ausführung). Nach Sperren werden Metadaten
erneut verglichen. Jede zu ändernde Zeile wird unmittelbar vor dem UPDATE vollständig
verglichen; nach allen Updates wird der vollständige Bestand geprüft. Generierte
Nachherwerte dürfen sich entsprechend der Änderung berechnen. Triggeränderungen an
normalen Nebenfeldern/anderen Zeilen, PK-Verschiebungen, Constraints und abweichende
Zeilenanzahlen führen zum Rollback. Keine atomare Einzeltransaktion über mehrere Tabellen.

Antwort HTTP200:

- `completed`: Commit bestätigt, `affected_rows` nennt direkte Änderungen.
- `rejected`: vor Commit abgebrochen/zurückgerollt, Anzahl 0; redigierter Grund
  `database_error`, `row_conflict` oder `invalid_value`.
- `outcome_unknown`: Fehler beim Commitversuch, Anzahl 0 bedeutet **unbekannt**,
  nicht „sicher keine Änderung“; `reason:commit_unknown`. Zustand laden, niemals blind
  wiederholen. Auch `completed` verhindert keinen Replay nach späterer Wiederherstellung
  identischer Daten; es gibt keinen persistenten Plan-/Idempotenzspeicher.

## Grenzen und Fehler

B2c-Konfliktmodell: kanonische vollständige Cells und Metadaten, keine historische
ABA-Erkennung oder physische Identitätsgarantie. Einfügen nach dem letzten
Netzwerksnapshot kann außerhalb des bestätigten Bestands liegen (MySQL Read-Committed);
kein Phantom-/Exactly-once-Versprechen. Trigger, Kaskaden, nichttransaktionale
Nebentabellen und externe Funktionseffekte können über direkte Änderungen hinauswirken.
Privilegien sind die tatsächlichen DB-Rechte; Fähigkeiten sind keine Rechteprüfung.
Plan benötigt Sperr-/SELECT-Rechte und führt keine UPDATE-Anweisung aus.

Zeitlimit über die ganze Anfrage: Standard 10000, maximal 60000 ms, einschließlich
Katalogabfragen und Slots. Request-Abbruch wird weitergereicht, garantiert aber keinen
Rollback bereits externer Effekte. Leere/fehlende Treffer sind reguläre Ergebnisse.
Fehler vor dem Schreibversuch: 400 `invalid_search`, 404 `table_not_found`,
409 `plan_mismatch`/`row_conflict`, 422 `unsupported_search`/`replace_limit`;
sonst bestehende redigierte Verbindungs-/DB-/Timeoutfehler. Keine rohen SQL-,
Treiber-, Zugangsdaten- oder Pfadtexte in Antworten.
