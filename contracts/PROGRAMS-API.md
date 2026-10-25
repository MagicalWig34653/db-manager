# Programmobjekte – API 0.19 (vorläufig)

W-P19 ergänzt API 0.8 um sechs Routen. Die Versionsnummer ist vorläufig und nicht
im Web-Lock gepinnt. Eigener Code steht unter AGPL-3.0-only. Bestehende Vertragsbytes
bleiben unverändert. Die [Schemas](api-v0.19.0.json) sind maßgeblich für JSON-Felder;
dieser Text beschreibt zusätzliche semantische und dialektbezogene Prüfungen.

| Methode und Route | Zweck |
| --- | --- |
| GET `/api/v19/capabilities` | Fähigkeiten je Treiber, Grenzen und ausgeschlossene Aktionen |
| POST `/api/v19/programs/list` | Liste je Schema, Objektart und optional Tabelle für Trigger; Limit/Offset |
| POST `/api/v19/programs/describe` | Ein Objekt mit Definition, Körper, Parametern, Eigenschaften, Definer, Sicherheitskontext und Abhängigkeiten |
| POST `/api/v19/programs/dependencies` | Dieselbe Detailantwort, einschließlich Abhängigkeiten |
| POST `/api/v19/programs/plan` | Wirkungsloser Plan für `create`, `replace` oder `drop` |
| POST `/api/v19/programs/execute` | Neu berechneten Plan nach Hash- und Bestätigungsprüfung ausführen |

Alle Routen sind Bearer-authentifiziert. Zugangsdaten bleiben wie bei den
Bestandsrouten Teil der flüchtigen Verbindungseingabe. Unbekannte JSON-Felder
werden auch in geschachtelten Eingaben abgewiesen. Verbindungsdaten werden weder
in Antworten noch in Fehlern gespiegelt. SQL-Definitionen sind ausdrücklich
angeforderter, potenziell vertraulicher Inhalt und dürfen vom Client nicht
protokolliert oder dauerhaft gespeichert werden.

## Fähigkeiten und Objektbezüge

| Treiber | Views | Funktionen/Prozeduren | Trigger | Events |
| --- | --- | --- | --- | --- |
| SQLite | CREATE; Ersetzen als transaktionales DROP/CREATE; DROP | nicht unterstützt | CREATE; Ersetzen als transaktionales DROP/CREATE; DROP | nicht unterstützt |
| MySQL/MariaDB (`mysql`) | CREATE / CREATE OR REPLACE / DROP | CREATE; Ersetzen als nicht atomare DROP/CREATE-Folge; DROP | CREATE; Ersetzen als nicht atomare DROP/CREATE-Folge; DROP | CREATE; Ersetzen als nicht atomare DROP/CREATE-Folge; DROP |
| PostgreSQL | CREATE / CREATE OR REPLACE / DROP | CREATE / CREATE OR REPLACE / DROP; eigener Dialekt | CREATE; Ersetzen als transaktionales DROP/CREATE; DROP; separate Triggerfunktion | nicht unterstützt |

`object` enthält `kind`, `schema`, `name`, bei PostgreSQL-Routinen zusätzlich
`identity` und bei PostgreSQL-Triggern gegebenenfalls `table`. `identity` ist exakt
`pg_get_function_identity_arguments` aus einer Listen-/Detailantwort, kein freies
Typfragment. Leere Identität bezeichnet eine Routine ohne Identitätsargumente;
sie ist kein Platzhalter für eine beliebige Überladung. Die Tabelle löst
mehrdeutige Triggernamen auf. Nicht eindeutige Bezüge werden abgewiesen.

Bestehende Ziele und referenzierte Tabellen/Funktionen werden im Katalog gesucht;
ausschließlich ihre Katalogschreibweise wird gequotet. Neue Namen werden auf
Dialektlänge, Steuerzeichen, NUL und reservierte SQLitepräfixe geprüft und
anschließend gequotet. Systemschemas sind ausgeschlossen. Ohne `schema` gelten
`main`, die aktuelle MySQL-Datenbank oder PostgreSQLs `current_schema()`.
Ein Tabellenfilter ist nur zusammen mit `kind=trigger` zulässig.

Listen liefern die verfügbaren Katalogfelder; vollständige Definitionen und
Parameter werden über `describe` geladen. MySQL/MariaDB verwendet dort SHOW
CREATE, SQLite `sqlite_schema`, PostgreSQL `pg_get_functiondef` bzw.
`pg_get_triggerdef`. PostgreSQL-Views sind aus `pg_get_viewdef` und Katalogoptionen
abgeleitet und als solche gekennzeichnet. `definition=null` bedeutet nicht
verfügbar; fehlende Rechte werden nicht durch privilegierte Verbindungen umgangen.
Definitionen lassen sich als Text exportieren; es gibt keine zusätzliche Datei-
oder Downloadroute. Eigentümer, Privilegien und Sitzungszustand sind keine
vollständige portable Sicherung und werden durch einen Definitionsexport nicht
pauschal wiederhergestellt.

## Strukturierte Eingaben und Nutzer-SQL

`input.body` enthält nur den View-SELECT beziehungsweise den Routinen-, Trigger-
oder Eventkörper, keinen vollständigen CREATE-Wrapper. Der Plan kennzeichnet
solche Eingaben als `user_sql=true`. E5 erlaubt diese Körper ausschließlich mit
Vorschau und Bestätigung. Keine eigene SQL-Sicherheitsanalyse oder Syntax-
Linterzusage: Die Datenbank prüft die Syntax bei der Ausführung. Es gibt keinen
separaten Weg, einen Körper testweise auszuführen.

Strukturiert sind:

- Routineparameter: bis 32, eindeutiger Name, Modus `in`, `out`, `inout` und fester
  Typschlüssel. MySQL-Funktionen akzeptieren nur IN. Typschlüssel: `integer`,
  `bigint`, `text`, `boolean`, `date`, `timestamp`, `numeric`, `real`, `double`,
  `binary`, `varchar`. `varchar` bedeutet VARCHAR(255), MySQL `numeric`
  DECIMAL(65,30), PostgreSQL NUMERIC, `binary` BLOB bzw. BYTEA. PostgreSQL erlaubt
  zusätzlich die Funktionsrückgaben `void` und `trigger`.
- PostgreSQL-Routinen: Sprache `sql` oder `plpgsql`; SQL SECURITY `invoker` oder
  `definer`. Ersetzen erhält Parameteranzahl, Namen, Modi und kanonische Typen
  derselben Überladung. Variadische/Tabellen-/unbenannte und weitere spezialisierte
  Signaturen können gelesen und gelöscht, aber nicht strukturiert ersetzt werden.
  Rückgabetypänderungen unterliegen den Regeln von CREATE OR REPLACE des Servers.
- MySQL/MariaDB: SQL SECURITY sowie `deterministic` für Routinen; optionaler
  `definer` aus Name und Host. Ein anderer Definer muss im sichtbaren
  `mysql.user`-Katalog vorhanden sein; der aktuelle Benutzer wird über
  CURRENT_USER validiert. Explizite Definer-/DEFINER-Aktionen tragen eine Warnung.
- PostgreSQL: optionaler `definer.name` ist ein Katalogrollenname und erzeugt ein
  zusätzlich angezeigtes ALTER OWNER. Trigger selbst haben keinen unabhängigen
  Eigentümer. Ein Hostfeld ist hier unzulässig.
- Views: SQL SECURITY soweit der Dialekt es unterstützt und `check_option`
  `local`/`cascaded`. PostgreSQL verwendet `security_invoker`; SQLite weist diese
  Optionen ab.
- Trigger: `object.table`, Zeitpunkt `before`/`after`/`instead_of` und ein Ereignis
  `insert`/`update`/`delete`; jeweils FOR EACH ROW. MySQL unterstützt hier kein
  INSTEAD OF. PostgreSQL verlangt `input.function` als Katalogbezug auf eine
  parameterlose Funktion mit Rückgabetyp `trigger`; `body` ist dabei leer. Andere
  Treiber verlangen den Körper einschließlich BEGIN/END, soweit syntaktisch nötig.
- Events: Zeitpunkt `schedule.at` oder positive Ganzzahl `every` mit Einheit
  `second`/`minute`/`hour`/`day`/`week`/`month`/`year`, optional STARTS/ENDS,
  `preserve` und `status=enabled|disabled`. Zeitwerte sind gültige Kalenderwerte
  im Format `YYYY-MM-DD HH:MM:SS` in der Sitzungszeitzone. ON COMPLETION und
  Aktivierungsstatus stehen ausdrücklich in den Statements. Aktivierte Events
  können später Code ausführen; entsprechende Warnung ist Teil jeder Vorschau.

Nicht zur Objektart passende Optionen werden abgewiesen. SQLite-Tabellen-
Neuaufbau ist nicht implementiert; das transaktionale Ersetzen einer View oder
eines Triggers baut keine Tabelle neu auf. Keine Kaskaden, keine temporäre
Abschaltung von Fremdschlüsseln oder Triggern.

## Plan, Bestätigung und Ergebnis

`plan` gibt die exakte geordnete Liste `statements`, Warnungen,
`affected_objects`, Abhängigkeiten, `destructive`, `transactional` und
`plan_sha256` aus. Ein Körper wird als **ein Statement über die Treiber-API**
gesendet; BEGIN/END und innere Semikolons bleiben erhalten. Es gibt kein Client-
Splitting, keine DELIMITER-Anweisungen. MySQL hat MultiStatements=false;
PostgreSQL und SQLite verwenden Prepare, das angehängte Statements abweist.
Ein fehlerhafter Körper kann daher einen Plan erhalten und erst beim Anwenden
sicher abgewiesen werden; ein Plan ist kein Syntaxnachweis.

`execute` wiederholt die Eingabe und ergänzt `expected_plan_sha256` und
`confirm.action`. Bei `drop` und jedem `replace` ist zusätzlich der exakt
getippte `confirm.target_name` erforderlich. Planabweichung: 409 `plan_mismatch`;
fehlende/falsche Bestätigung: 400 `confirmation_mismatch`. Der Hash verwendet
unverändert `dbplan` (Treiber und normalisierte Statements), deckt also keine
Identität einer neu erzeugten Verbindung, Warnungen, Abhängigkeiten oder
vorherigen Objektdefinitionen ab.

SQLite/PostgreSQL führen alle angezeigten Statements in einer Transaktion aus;
MySQL/MariaDB bestätigen DDL implizit. Scheitert dort CREATE nach erfolgreichem
DROP, lautet der Ausgang `outcome_unknown`, mit Zahl erfolgreich ausgeführter
Statements. Das entfernte Objekt wird nicht automatisch wiederhergestellt.
`completed` bedeutet bestätigten Abschluss, `rejected` keine angewendete
Änderung, `outcome_unknown` verlangt manuelle Katalogprüfung. `atomic` bezeichnet
nur die transaktionale DDL-Gruppe, keine externen Nebenwirkungen oder Exactly-once-
Garantie. `observation=unavailable`: Die Antwort enthält bewusst keine behauptete
Nachbeobachtung; der Client muss separat neu laden.

Kein automatischer Retry, auch bei Abbruch oder unbekanntem Commit. Nach
`completed` oder `outcome_unknown` muss der Client die Bestätigung sperren und
für eine neue Aktion einen neuen Plan anzeigen. Der Hash ist zustandslos,
keine Einmal-Nonce; ein Replay von CREATE OR REPLACE kann erneut wirken.
Andere Sitzungen können ein namensgleiches Objekt zwischen Katalogprüfung und
Ausführung austauschen. Ersetzen kann Eigenschaften/Privilegien verlieren oder
neu setzen; alle angezeigten Statements und Warnungen sind zu prüfen.

## Abhängigkeiten, Scheduler und Grenzen

`dependencies` nennt Richtung `incoming`/`outgoing` und `certainty=exact|textual`.
Tabellenzuordnung von Triggern ist exakt. PostgreSQL nutzt `pg_depend`,
`pg_rewrite`, `pg_proc` und `pg_trigger`; SQL-Stringkörper, dynamisches SQL,
Erweiterungen, unsichtbare Objekte und noch nicht angelegte Definitionen können
Verweise verbergen. SQLite/MySQL/MariaDB scannen höchstens 1000 sichtbare
Kandidaten im selben Schema textuell, mit möglichen Fehlpositiven. Höchstens
100 Beziehungen; `dependencies_truncated` kennzeichnet gekappte Ergebnisse.
Das ist keine Zusage vollständiger oder schemaübergreifender Analyse.

MySQL/MariaDB liefert globalen Schedulerstatus ausschließlich lesend sowie
Eventstatus, Zeitplan, letzte Ausführung, Zeitzone und ON COMPLETION aus dem
Katalog. Eine portable sichere Eventfehlerliste ist nicht verfügbar:
`event_errors_state=unavailable`, entsprechende Fähigkeit `unsupported`.
Keine Rohlogs, Serverpfade oder Treiberfehler. Routinenaufruf, Routinentest,
Scheduleränderung und Replikationssteuerung sind ausdrücklich ausgeschlossen.
PMA-PROG-005 bleibt einem späteren Paket nach Entscheidung E5/E9 vorbehalten.

Grenzen: Anfrage 1 MiB, Nutzerkörper 256 KiB (Byteprüfung), einzelne Definition
oder gelesener Körper 1 MiB, Gesamtantwort 8 MiB, Liste 200 Objekte,
Offset 100000, maximal drei Statements. Timeout standardmäßig 30000 ms,
höchstens 60000 ms, einschließlich Katalogprüfung und Ausführung. Weitere
semantische Prüfungen erfolgen auf dem Server; Schemakonformität allein bedeutet
keine ausführbare oder unterstützte Dialektdefinition.
