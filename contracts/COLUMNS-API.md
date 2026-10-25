# Spalten ändern (API 0.9.0)

Additiver Vertrag [api-v0.9.0.json](api-v0.9.0.json). Routen:
`GET /api/v9/capabilities`, `POST /api/v9/columns/plan`, `POST /api/v9/columns/alter`.
Bearer-Authentifizierung, JSON ohne unbekannte Felder oder `null`, Standard-Routenklasse
mit 1 MiB. Verbindung wie bisher je Anfrage; keine Speicherung oder Passwortechos.
Die v8-Vertragsdefinitionen sind unverändert übernommen; auf einer Basis vor W-P07
werden dessen Laufzeitrouten erst mit W-P07 integriert.

## Anfrage und Bestätigung

Beide POST-Routen verlangen `connection`, `table` und `changes` (1–16 Änderungen).
`timeout_ms`: 0 oder ausgelassen = 30 Sekunden, höchstens 60 Sekunden. Jede Änderung
bezieht sich auf den anfänglichen Katalogstand; eine Spalte darf im Batch nur einmal
Ziel sein. Neue Namen dürfen nicht mit vorhandenen Namen kollidieren; neue Namen
sind höchstens 63 Byte lang, gültiges UTF-8 ohne NUL. Es gibt keine Namensverkettung
von neu angelegten/umbenannten Spalten innerhalb desselben Plans.

| `action` | Felder | Wirkung |
| --- | --- | --- |
| `add` | `definition`, optional `position` | neue Spalte |
| `rename` | `column`, `new_name` | vorhandene Spalte umbenennen |
| `change` | `column`, `definition`, optional `position` | vollständige neue Definition |
| `reorder` | `column`, `definition`, `position` | MySQL/MariaDB: vollständige Definition mit neuer Position |
| `drop` | `column` | Spalte ohne CASCADE entfernen |

`definition`: `name`, strukturierter `type`, `nullable` (Pflichtbool), `default`
(Pflichtobjekt), optional `comment` (0–2048 Byte) und `collation` (Katalogname).
MySQL/MariaDB verlangen bei `change`/`reorder` einen ausdrücklichen Kommentar
(auch leer) und bei VARCHAR eine ausdrückliche Kollation. Eine vollständige
Definition ersetzt die Attribute; bestehende Sonderattribute wie AUTO_INCREMENT,
ON UPDATE, unsichtbare oder generierte Spalten werden nicht rekonstruiert, sondern
abgewiesen. Neue Attribute müssen daher bewusst gewählt werden.

`position`: `{mode:"first"}` oder `{mode:"before"|"after",column:"Katalogname"}`.
Die Referenz muss eine andere, bereits vorhandene Spalte sein. `before` wird anhand
der Katalogreihenfolge in FIRST/AFTER aufgelöst. Bei mehreren Änderungen ist die
in der Vorschau sichtbare sequenzielle SQL-Reihenfolge maßgeblich.

Die Vorschau enthält die exakten Anweisungen, Warnungen, betroffene Objekte,
`dependencies`, `table` in Katalogschreibweise, `destructive`, `target_name`,
`ddl_transactional` und `plan_sha256` aus `dbplan.NewPlan`.
Die Ausführung verlangt zusätzlich `confirm.action="columns_alter"` und
`expected_plan_sha256`. Bei DROP sowie jeder Typ-/Attributersetzung einschließlich
Umordnen ist `confirm.target_name` der getippte Tabellenname aus dem Katalog.
Der Plan wird unmittelbar vor Ausführung auf derselben Verbindung neu gebaut;
`dbplan.Check` erhält ausschließlich den Katalognamen als Bestätigungsziel.
Hashabweichung: 409 `plan_mismatch`; falsche Bestätigung: 400 `confirmation_mismatch`.

## Typen, Konstanten und Dialekte

Der gemeinsame feste Typkatalog liegt in `internal/dbplan/types.go`; W-P09 kann ihn
lesend nutzen. `int64` → BIGINT, `text` → VARCHAR (`length` 1–1024), `decimal` →
DECIMAL (`precision` 1–38, `scale` 0–precision; ausgelassen = 0), `binary` → BLOB
bzw. PostgreSQL BYTEA, außerdem BOOLEAN, DATE, TIMESTAMP. Nur MySQL/MariaDB:
ENUM (1–256 Werte) und SET (1–64 Werte). Werte höchstens 255 Byte, eindeutig,
ohne NUL oder nachgestelltes Leerzeichen; Kommas in ENUM erlaubt, in SET nicht.
Kein freier Typtext, kein freier USING-Ausdruck. PostgreSQL verwendet ausschließlich
den aus diesem Katalog erzeugten Cast `USING <Katalogspalte>::<fester Zieltyp>`.

`default.mode`: `none`, `null` (nur bei nullable), `value` mit typisierter Konstante
`{kind,value}`. Integer und Dezimalzahlen werden wie API 0.6 validiert; Text muss
in die VARCHAR-Länge passen. Binärdefaults sind nicht unterstützt. Boolean ist
`kind=boolean`, Wert `true`/`false`; Datum/Zeit sind begrenzte ISO-Zeichenfolgen mit
`kind=text` (Kalenderprüfung durch die Engine). ENUM/SET-Konstanten müssen aus der
Werteliste stammen. Keine Funktionen, SQL-Fragmente oder generierten Ausdrücke.

Katalogabfragen binden sämtliche Werte. DDL akzeptiert für Typen, Defaults und
Kommentare keine Parameterplatzhalter: Dort werden ausschließlich validierte
Konstanten mit Quote-Verdopplung erzeugt (wie API 0.6/0.8). MySQL/MariaDB pinnen
`NO_BACKSLASH_ESCAPES,STRICT_ALL_TABLES`; PostgreSQL pinnt
`standard_conforming_strings=on`. Bestehende Bezeichner und Kollationen stammen
aus Katalogtreffern und werden über `dbplan` gequotet; neue Namen werden separat
validiert und ebenso gequotet. Kollationen gelten nur für VARCHAR; MySQL leitet
den Zeichensatz aus dem Kollationskatalog ab. PostgreSQL erlaubt Kollationen aus
dem Tabellenschema oder `pg_catalog`.

| Fähigkeit | SQLite | MySQL/MariaDB | PostgreSQL |
| --- | --- | --- | --- |
| Hinzufügen | eingeschränkt, konstante Defaults; NOT NULL benötigt Default | InnoDB, feste Typen | reguläre Basistabellen |
| Umbenennen, Entfernen | native ALTER-Funktionen; Engine prüft Abhängigkeiten | InnoDB | reguläre Basistabellen ohne Vererbung/Partitionen |
| Definition ändern | nicht unterstützt (E5: kein Neuaufbau) | vollständige Definition, keine Sonderattribute | feste Casts, NULL, Default, Kommentar, Kollation |
| Position | nicht unterstützt | FIRST/BEFORE/AFTER | nicht unterstützt |
| ENUM/SET | nicht unterstützt | unterstützt mit Grenzen | nicht unterstützt (keine benutzerdefinierten Typanlagen) |
| Generiert | nicht unterstützt (E5) | nicht unterstützt (E5) | nicht unterstützt (E5) |
| Kommentar/Kollation | nicht unterstützt | DB-eigene Attribute | DB-eigene Attribute |

SQLite behält seine Affinitätssemantik: VARCHAR-Längen und Dezimalpräzision sind
keine Durchsetzungsgarantie. Kein Tabellenneuaufbau, kein Abschalten von
Fremdschlüsseln. Systemschemas, Sichten, virtuelle/fremde Tabellen und die
Bearbeitung generierter/Identity-Spalten sind ausgeschlossen. MySQL/MariaDB
beschränken Schreibaktionen auf InnoDB; andere Engines werden abgewiesen.

## Ergebnisse, Abhängigkeiten und Grenzen

Erfolgreich angenommene Ausführung: HTTP 200 mit `outcome`:

- `completed`: DDL bestätigt; `observation` ist `transactional` (SQLite/PostgreSQL)
  oder `observed` (MySQL/MariaDB, getrennt nach implizitem Commit), sonst `unavailable`.
  `description` ist die beobachtete Struktur, kein dauerhafter Zustandsbeweis.
- `rejected`: nichts geändert; sicherer `reason` ohne Treiberfehler. Ein bekannter
  Fehler rollt SQLite/PostgreSQL samt vorherigen Batchschritten zurück.
- `outcome_unknown`: verlorene Ausführungs-/Commitbestätigung, Verbindungsabbruch
  oder unklarer Zeitlimitfall. Kein automatischer Retry. Auch eine vor Commit
  gelesene Beschreibung beweist dann keinen Commit und wird nicht ausgegeben.

`atomic=true` nur nach bestätigtem SQLite/PostgreSQL-Commit. MySQL/MariaDB-Batches
werden als eine ALTER-TABLE-Anweisung ausgeführt, aber nicht als universell
transaktional oder rückrollbar versprochen. `statements_executed` zählt bestätigte
Anweisungen; bei bestätigter Rücknahme 0. Fehler vor dem Senden bleiben die üblichen
HTTP-Fehler: 400 ungültige Anfrage, 404 Tabelle/Spalte fehlt, 409 Namenskonflikt/Plan,
422 nicht unterstützte Aktion; Auth-, Größen-, Verbindungs- und Zeitfehler wie bisher.
Öffentliche Antworten enthalten keine rohen Treiberfehler oder Hostpfade.

DROP-Vorschauen nennen PostgreSQL-Katalogabhängigkeiten, MySQL/MariaDB-Schlüssel,
Indizes und textuell gefundene Sichten; SQLite nennt zugehörige Indizes/Trigger
und textuelle Tabellen-/Sichttreffer. Das ist keine vollständige Abhängigkeitsanalyse
(Triggerkörper, Rechte-Sichtbarkeit, externe Verweise). Die Engine bleibt maßgeblich;
kein CASCADE, keine automatische Entfernung fremder Objekte. Höchstens 256 Einträge,
Antwort höchstens 8 MiB. NOT NULL zählt vorhandene NULLs unter dem Anfragezeitlimit
und warnt; keine automatische Datenkorrektur. Alle Typ-/Attributersetzungen gelten
konservativ als potenziell verlustbehaftet, auch bei Erweiterung oder Umordnung.

Der Hash deckt Treiber und Anweisungen ab, keine Objektidentität, Warnungen oder
Datenwerte im Ziel. **Kein Replay-Schutz:** Clients sperren nach `completed` oder
`outcome_unknown` die erneute Ausführung und laden einen neuen Plan mit neuer
Bestätigung. Gleiche Namen nach zwischenzeitlichem Drop/Recreate können denselben
Hash ergeben. Konkurrierende Schemaänderungen zwischen Neuberechnung und DDL sind
nicht vollständig ausgeschlossen; kein Exactly-once- oder ABA-Versprechen.
