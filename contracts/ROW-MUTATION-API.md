# Strukturierte Einzelzeilenänderung – freigegebener Vertrag 0.3.0

Status: **nach unabhängiger Integritätsprüfung und nativer CI für Clientbindung freigegeben**.
Herkunft `946241d7f10f7444e9e5efebbe8b3a38a8120a9b`, Schema-SHA256
`ec77c9739cda11df3f4cb9fe756a632e67840fdc0f6190deee1aa1d1333dc308`. [OpenAPI](api-v0.3.0.json), [Übergabe](../docs/handoffs/B2b.md).
Die Dateien, Routen und Capabilities von API0.1.0 und0.2.0 bleiben unverändert.
Insbesondere bleibt `read_only=true` im v2-Describe die dort angebotene
Produktfähigkeit; neue v3-Fähigkeiten sind getrennt davon zu bewerten.

| Route | Bedeutung |
| --- | --- |
| `GET /api/v3/capabilities` | Verfügbare Mutationsfunktionen und Grenzen |
| `POST /api/v3/tables/eligibility` | Frische Metadaten plus konservative strukturelle Eignung |
| `POST /api/v3/rows/update` | Einzelne bestehende Zeile nach vollständigem Snapshotvergleich ändern |
| `POST /api/v3/rows/delete` | Einzelne bestehende Zeile nach vollständigem Snapshotvergleich löschen |

Alle Routen benötigen den bestehenden Bearerzugang. Credentials bleiben je Request
flüchtig; Host-/TLS-Policy, sichere Fehler, Authentifizierung und No-store gelten weiter.
Keine Inserts, PK-Änderungen, Schema-, Batch- oder Massenänderungen.

## Eignung und Eingaben

Eligibility verwendet denselben `connection`-/`table`-Bezug wie v2-Describe.
`description` bleibt das unveränderte v2-Modell; `eligible`, `reason` und
`writable_columns` ergänzen die aktuelle strukturelle Produktfähigkeit. Eine positive
Eignung beweist keine tatsächlichen DB-Schreibprivilegien. Freigegeben sind reale
Basistabellen mit vollständigem nichtnullable PK und bekannter Transaktionsfähigkeit:
SQLite unter ADR0004, PostgreSQL ohne Vererbungs-/Partitionierungsbeziehungen,
MySQL/MariaDB mit InnoDB. Views, Foreign-/virtuelle Tabellen, fehlende/nullable PK,
unbekannte Engines und MyISAM/MEMORY/CSV/ARCHIVE bleiben gesperrt. Generated-,
Identity- und PK-Spalten sind keine Änderungsziele. Die Eignung wird bei jeder
Mutation neu geprüft; sie ist kein dauerhaftes Ticket oder Privilegiennachweis.

Update verlangt `connection`, `table`, `confirm:true`, `expected` und `changes`.
Delete verlangt dieselben Felder ohne `changes`; auch ein leeres Changesfeld ist dort
unzulässig. `expected` enthält sämtliche `columns` in frischer Metadatenreihenfolge
und die gleich langen typisierten `values` einer vollständigen bestehenden Zeile.
PK-Werte werden daraus entnommen; ein freies WHERE existiert nicht.
`changes` enthält ein bis1024 verschiedene `{column,value}`-Objekte. Leere
Spaltenbezeichner sind explizit möglich, wenn sie tatsächlich existieren; fehlende
Bezeichner sind ungültig. Nicht-NULL-Zellen benötigen ausdrücklich `kind` und `value`,
auch bei `value:""`; NULL ist ausschließlich `{kind:"null"}`. JSON-null ist verboten.
Alle Namen stammen nach Prüfung aus Metadaten, Werte werden gebunden.

Einzelzellen sind auf1MiB Rohdaten begrenzt, vollständige Antworten auf8MiB tatsächliches
JSON. Das vorhandene HTTP-Requestlimit1MiB gilt auch für Snapshot plus Änderung und
Credentials. Deshalb sind manche gültigen großen Browsezeilen nicht editierbar.
`timeout_ms`: fehlend/0=10000, maximal60000. Keine Transaktion über mehrere Requests.

## Konflikt-, Transaktions- und Ergebnisregeln

SQLite aktiviert auf der privaten Mutationsverbindung vor `BEGIN IMMEDIATE` ausdrücklich
die Fremdschlüsselprüfung über die native DBConfig-API. Die alten v1/v2-Verbindungen
und der Authorizer bleiben unverändert. PostgreSQL und
InnoDB verwenden eine echte Read-Committed-Transaktion mit `SELECT ... FOR UPDATE`.
PostgreSQL adressiert zusätzlich ausdrücklich `ONLY` die geprüfte Basistabelle.
SQLite-PK-Lookups und Schreibprädikate verwenden festes `COLLATE BINARY`, sodass
abweichende Spalten-/PK-Kollationen nicht eine andere oder mehrere Zeilen auswählen.
Mehrere gefundene Zeilen sind stets Fehler, keine Auswahl der ersten Zeile.
Nach dem Rowlock werden Metadaten erneut geprüft. Die vollständigen aktuellen
kanonischen Cell-Werte werden in Go exakt nach Art und Stringwert mit `expected`
verglichen; SQL-Gleichheit alter Werte, Kollationen oder numerische Coercion ersetzen
diesen Vergleich nicht. Fehlende Zeile, andere Spaltenform oder anderer Snapshot
führen zu HTTP409 `row_conflict`, ohne strukturierte Änderung.

Verglichen wird das gemeinsame Cellmodell, keine vollständige physische DB-Repräsentation
oder Historie. Zwischenzeitliche Änderung und Rückänderung auf denselben Snapshot
(ABA) ist nicht erkennbar. Ungültiges UTF-8 in aktuell gelesenem SQLite-TEXT wird für
Mutation konservativ abgewiesen, da Browse es als Binary darstellt. Eine vorangegangene
ungültige TEXT-Darstellung kann daraus rückwirkend nicht rekonstruiert werden.
PostgreSQL-NaN/Infinity oder andere vom Wertbinder nicht unterstützte kanonische
Spezialwerte erlauben keine Mutation. Keine unbelegte DB-Zustandskonflikterkennung.

Nach passendem Snapshot wird höchstens eine direkt adressierte Zeile geändert;
RowsAffected über1 wird abgewiesen. Ein identischer Updatewunsch führt zu keiner
UPDATE-Anweisung; danach wird die Transaktion abgeschlossen. `changed` beschreibt
bei Update die Differenz zwischen dem gelesenen Vorher-/Nachher-Cellmodell, nicht
Triggerausführung oder eine Anzahl physischer Schreibvorgänge. Defaults/Generated-
Spalten werden vollständig mitverglichen; serverseitige Typkonversion bleibt zulässig
und die gespeicherte Zeile wird zurückgegeben. PK-Veränderungen durch Trigger führen
zum Fehler, wenn die ursprüngliche Identität nicht mehr lesbar ist.

Ein bestätigter Erfolg enthält `outcome:"committed"`, `changed` und bei Update `row`
mit vollständigem aktualisiertem Snapshot; Delete hat kein `row`. Ein Fehler nach
versuchtem Commit ergibt stets HTTP503 `outcome_unknown`: Zustand neu laden und
abgleichen, niemals blind wiederholen. Das gilt konservativ auch für konkret vom
Server abgelehnte Commits. Ausfall der HTTP-Antwort nach erfolgreichem DB-Commit bleibt
für den Client ebenfalls unbestimmt. Vor Commit führt ein Fehler zur Rollbackanforderung;
keine Zusage über externe Trigger-/Funktionswirkungen, nichttransaktionale Nebenziele,
Cascades oder mehr als die direkt adressierte Zeile. Kein allgemeines Rollback-,
Exactly-once-, vollständiges Isolation-/DoS- oder HA-Versprechen.


Die Transaktionswahl stützt sich auf die Primärdokumentation zu
[SQLite-Transaktionen](https://www.sqlite.org/lang_transaction.html),
[PostgreSQL-Locks](https://www.postgresql.org/docs/18/explicit-locking.html) und
[InnoDB Locking Reads](https://dev.mysql.com/doc/refman/8.4/en/innodb-locking-reads.html).
Die aufgeführten eigenen Laufzeitproben sind zusätzliche Implementierungsnachweise,
keine aus den Dokumentationen abgeleitete globale Produktfreigabe.
