# Einzelnes strukturiertes Einfügen – Vertragskandidat0.4.0

Status: noch kein unabhängiger Freeze oder Clientvertrag. [OpenAPI](api-v0.4.0.json),
[Handoff](../docs/handoffs/B2c.md). Die eingefrorenen API0.1–0.3-Dateien, Routen und
Capabilityantworten bleiben unverändert. Alle neuen Routen benötigen die bestehende
Bearerauthentifizierung; Credentials bleiben flüchtig pro Request.

| Route | Bedeutung |
| --- | --- |
| `GET /api/v4/capabilities` | Aktuell vorhandene Insertfunktionen und feste Grenzen |
| `POST /api/v4/tables/insert-eligibility` | Strukturelle Eignung und mögliche Identitätspfade |
| `POST /api/v4/rows/insert` | Genau eine direkte Neuanlage mit bestätigter Nachherzeile |

## Request und Eignung

Eligibility verlangt denselben `connection`-/`table`-Bezug wie Describe.
`description` bleibt unverändertes v2-Metadatenmodell mit `read_only:true` für die dortige
Produktfähigkeit. `eligible`, `reason`, `insertable_columns`, `required_pk` und optional
`auto_pk` beschreiben zusätzlich den v4-Insertumfang. Das beweist keine Schreibprivilegien
und ist kein dauerhaftes Ticket; Insert prüft Metadaten und Eigenschaften erneut.

Zugelassen sind bekannte transaktionale Basistabellen mit vollständigem nichtnullable PK:
SQLite unter ADR0004, PostgreSQL ohne Vererbung/Partitionierung/Rewrite-Regeln und
MySQL/MariaDB mit InnoDB. Views, virtuelle/Foreign-Tabellen, fehlende/nullable PK und
unbekannte/nichttransaktionale Engines bleiben gesperrt. Generierte/Identity-Spalten
werden nicht als explizite Eingabe angenommen; unverifizierbare generierte PK sperren
Insert. Ausnahmen sind keine direkten Identitywerte, sondern nur die unten belegten
Auslassungs-/Auto-PK-Pfade. SQLite-Rowidalias und PostgreSQL-Serial können zusätzlich
explizit in kanonischer Form angegeben werden.

Insert verlangt `connection`, `table`, `confirm:true` und ausdrücklich `values` als
Array mit null bis1024 `{column,value}`-Objekten. Fehlende Felder sind ungültig;
explizites `values:[]` ist nur erfolgreich, wenn alle nötigen Defaults/Auto-PK vorhanden
sind. Nicht angegebene Spalten werden vollständig aus SQL weggelassen und erhalten
DB-Defaults. Das ist verschieden von einem ausdrücklich gelieferten `{kind:"null"}`.
Nicht-NULL-Zellen verlangen `kind` und `value`, auch bei `value:""`; JSON-null ist
verboten. Leere Spaltenbezeichner sind nur explizit und nach Metadatenprüfung möglich.
Unbekannte, doppelte und generierte Ziele sind Fehler. Kein SQL-Fragment wird angenommen;
alle Identifier kommen nach Prüfung aus Metadaten, sämtliche Werte werden gebunden.

Kein REPLACE, IGNORE, UPSERT oder Batch. SQLite verwendet ausdrücklich `INSERT OR ABORT`,
um auch im Tabellenschema definierte REPLACE-/IGNORE-Konfliktstrategien zu übersteuern.
Bekannte UNIQUE-/PK-Verletzung ergibt HTTP409 `insert_conflict`, andere belegte
Constraintverletzungen `constraint_violation`. Ein durch Trigger unterdrückter Insert
mit null direkt eingefügten Zeilen ergibt `insert_not_applied`, keinen Erfolg.
Allgemeine DB-Fehler bleiben redigiert. Trigger und DB-Programme können eigene weitere
Wirkungen auslösen; die API verspricht keine globale physische Einzeilenwirkung.

## Identität und Transaktion

Immer echte Transaktion auf einer gepinnten Verbindung: SQLite mit aktivierter
Fremdschlüsselprüfung und `BEGIN IMMEDIATE`, InnoDB/PostgreSQL Read Committed.
Bei Netzwerkzielen hält ein leeres `SELECT ... FOR UPDATE` zunächst den Tabellen-/
Metadatenlock; danach werden Tabellenart, Engine und Spalten erneut geprüft, bevor
INSERT möglich ist. Das verhindert insbesondere ein erst nach dem Schreiben erkanntes
InnoDB→MyISAM-Rennen. PostgreSQL liest ausdrücklich nur die geprüfte Basistabelle (`ONLY`).

Ein vollständiger expliziter PK muss nicht-NULL sein und nach dem Insert exakt in
kanonischer Cellform wiedergefunden werden. Für die eindeutige Identität sind
Typkonversionen am PK konservativ ein Rückrollgrund. SQLite-PK-Prädikate verwenden
festes `COLLATE BINARY`, unabhängig von abweichenden Spalten-/Indexkollationen.

Eng belegte automatische Pfade für einen einzelnen PK:

- SQLite: echter `INTEGER PRIMARY KEY`-Rowidalias ohne PK-Index; ID aus dem Resultat
  derselben Verbindung als signedInt64. Kein WAL/Fremdprozess-/Mehrreplikatversprechen.
- MySQL/MariaDB: einzelne AUTO_INCREMENT-PK-Spalte. Session-ID vorab auf0 gesetzt;
  nach erfolgreichem einzelnen Insert `CAST(LAST_INSERT_ID() AS CHAR)` innerhalb
  derselben Transaktion gelesen und als UInt64-Zeichenfolge geprüft. Kein Int64-Cast,
  keine Abfrage globaler MAX-ID und keine fremde Session.0/unklare IDs sind Rückrollgrund.
- PostgreSQL: einzelner Integer-PK mit belegter Identity-/Serial-Sequenzzuordnung und
  Serverdefault. `INSERT ... RETURNING pk` dient ausschließlich als interner Hinweis
  auf die soeben erzeugte PK-Identität. Die tatsächliche Nachherzeile wird gesondert gelesen.

Auf allen Pfaden folgt nach vollständig ausgeführtem INSERT ein erneutes vollständiges
SELECT der Zeile über den belegten PK, bei Netzwerkzielen `FOR UPDATE`. Genau eine
Zeile, exakter kanonischer PK und unveränderte Metadaten müssen bestätigt sein.
AFTER-Trigger-PK-Verlagerung, keine/mehrere gefundene Zeilen oder unklare Identität
führen zu `insert_identity_unknown` und Rollbackanforderung. Ein RETURNING-Ergebnis
allein wird niemals als finale Zeile ausgegeben. Lokale transaktionale Auditwrites
werden in den belegten Rückrollfällen ebenfalls verworfen; keine Zusage über externe
Trigger-/Funktionswirkungen oder nichttransaktionale Nebenziele.

## Nachherwerte, Limits und Fehler

Erfolg enthält `outcome:"committed"` und `row` als vollständigen kanonischen
Zeilensnapshot. Zurückgegeben werden tatsächliche gespeicherte Werte einschließlich
Defaults, Generated-/Triggerwerten und DB-seitiger Konversion. Gebundene Eingaben
sind kein Versprechen bitgleicher Speicherung: etwa `DECIMAL(30,5)` speichert auf den
geprüften Netzwerkzielen `1.234567` als `1.23457`. SQLite-Affinität kann Zahlen anders
speichern; das bestehende Runtime-StorageClass-Modell bleibt maßgeblich. Es gibt
keine zusätzliche Floatkonversion im Clientvertrag. Nicht darstellbares aktuelles
SQLite-TEXT (ungültiges UTF-8) oder unbindbare Spezialwerte verhindern Bestätigung.

Die tatsächliche komplette JSON-Antwort muss vor Commit innerhalb8MiB bleiben;
je Rohzelle maximal1MiB. HTTP-Request einschließlich Credentials und aller Werte
maximal1MiB. Timeout fehlend/0=10000ms, maximal60000ms, global maximal vier aktive
DB-Anfragen. Bei Fehler vor Commit wird Rollback angefordert. Nach Commitversuchfehler
weiterhin HTTP503 `outcome_unknown`: Zustand abgleichen, niemals blind wiederholen.
Eine verlorene HTTP-Antwort nach erfolgreichem Commit ist für den Client ebenfalls
unklar; Insert ist nicht automatisch idempotent. Automatische IDs/Sequences können
auch nach Rollback verbraucht bleiben und Lücken zeigen. Keine umfassende
Crash-/Powerloss-/Exactly-once-/DoS-/HA-/Trigger-Isolationsgarantie.

Primärquellen: [SQLite-Konfliktstrategien](https://www.sqlite.org/lang_conflict.html),
[SQLite Last Insert Rowid](https://www.sqlite.org/c3ref/last_insert_rowid.html),
[SQLite RETURNING-Grenzen](https://www.sqlite.org/lang_returning.html),
[MySQL LAST_INSERT_ID](https://dev.mysql.com/doc/refman/8.4/en/information-functions.html#function_last-insert-id),
[MariaDB LAST_INSERT_ID](https://mariadb.com/docs/server/reference/sql-functions/secondary-functions/information-functions/last_insert_id).
Die eigenen tatsächlichen Trigger-/Auto-ID-/Rollbackproben stehen im Handoff;
Dokumentation allein ist kein Laufzeitnachweis.
