<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Serververwaltung: lesende Stufe (API 0.17.0)

Additiv zu API 0.12: [Schema](api-v0.17.0.json). V1 bis v12 bleiben byteidentisch;
benachbarte Pakete v13/v14/v16+ sind keine Voraussetzung. Nummer 0.17 ist zugewiesen,
unabhängiger Review und Vertragsfreeze bleiben offen. Kein Web-Lock-Pin.
Alle Routen benötigen Bearer-Authentifizierung, Antworten verwenden `no-store`.
Unbekannte JSON-Felder werden abgewiesen; Verbindung und Passwort sind ausschließlich
flüchtige Anfragewerte. Fehler enthalten feste deutsche Meldungen ohne SQL,
Treibertexte, Passwörter oder Dateipfade.

| Route unter `/api/v17/` | Wirkung |
| --- | --- |
| `GET capabilities` | Lesefähigkeiten je Treiber und Limits |
| `POST status` | Statuszähler mit UTC-Messzeitpunkt |
| `POST variables` | Namenfilter und GLOBAL-/SESSION-Werte; sensible/unklassifizierte Werte maskiert |
| `POST processes` | Sichtbare Clientverbindungen mit ID, Benutzer, Datenbank, Laufzeit und Selbstmarkierung |

## Paging und Grenzen

Anfragen: `connection`, optional `limit` (0 = 100, maximal 200), `offset` (maximal
10 000), `timeout_ms` (0 = 5000, maximal 10 000), `filter` (maximal 128 UTF-8-Bytes).
`scope` gilt nur für Variablen und ist `SESSION` (Standard) oder `GLOBAL`.
Variablen-/Statusnamen werden ohne Beachtung der Großschreibung nach Teiltext
gefiltert und alphabetisch sortiert. Prozesse sind nach numerischer ID sortiert;
ihre Benutzerfilter folgen der Serverkollation. Filterwerte werden gebunden.
`has_more` kennzeichnet weitere Treffer; wechselnde Listen sind keine stabilen
Snapshots. Es gibt keine Gesamtzahl. `sampled_at` bezeichnet den Beginn der
Beobachtung, keinen atomaren Messzeitpunkt.

Anfragen sind auf 1 MiB begrenzt. Status-/Variablenabfragen lesen vor Filterung und
Paging höchstens 4096 Einträge, 256 Bytes je Name, 1 MiB je Quellwert und insgesamt
8 MiB Quelldaten. Antworten sind ebenfalls auf 8 MiB begrenzt. Prozessabfragen
fordern nur die Seite plus einen Eintrag an; Benutzer und Datenbank maximal je
256 Bytes. Zeitlimit und Abbruch gelten auch für Verbindungsaufbau und Wartezeiten.
Ein Clientabbruch garantiert keinen sofortigen Abschluss der Serverabfrage.

## Treiber und Sichtbarkeit

MySQL/MariaDB verwenden `SHOW GLOBAL STATUS`, `SHOW SESSION/GLOBAL VARIABLES`
und `information_schema.PROCESSLIST`. Numerische Statuswerte umfassen `Com_*`
als Zähler je Abfragetyp. MariaDB verwendet den Treiberbezeichner `mysql`.
PostgreSQL liefert eine feste Auswahl aus `pg_stat_database` für die verbundene
Datenbank (Transaktionen, Cache-/Tupelzähler, Deadlocks, temporäre Bytes), wirksame
Sitzungswerte aus `pg_settings` und Clientverbindungen aus `pg_stat_activity`.
Es gibt keine PostgreSQL-Entsprechung für MySQL-Abfragetypzähler; `GLOBAL`-Lesen
wird bei PostgreSQL mit 422 abgewiesen. SQLite liefert ausschließlich
`sqlite_version()` ohne Pfade. Variablen und Prozesse werden dort mit 422 abgewiesen.

Werte bleiben Strings, damit große Ganzzahlen präzise bleiben. `masked=true`
erfordert `value=null`. Namen mit Passwort-/Secret-/Token-/Key-/Pfad-/Datei-/TLS-
oder Credentialbezug sind gesperrt. Variablenwerte werden ausschließlich für
bekannte unkritische Zahlenvariablen ausgegeben (`max_heap_table_size`,
`tmp_table_size`, `statement_timeout`, `lock_timeout`, `max_connections`, `max_user_connections`, `port`; Status `Connections` und `Max_used_connections` sind als exakte Zähler ebenfalls sichtbar),
sofern sie als Zahl mit höchstens 64 Bytes darstellbar sind. Unbekannte Variablen
und nichtnumerische Statuswerte bleiben maskiert. Die SQLite-Version ist eine
feste, unkritische Ausnahme.

Die Sichtbarkeit bestimmt der Datenbankserver: MySQL ohne PROCESS-Recht sieht
nur eigene Sitzungen; PostgreSQL kann fremde IDs/Benutzernamen zeigen, während
weitere Metadaten eingeschränkt sind. Es werden weder SQL-Texte noch Hosts,
Clientadressen, Anwendungsnamen, Freitextzustände oder Identitätstoken ausgewählt
oder ausgegeben. `seconds` ist die Laufzeit des aktuellen/letzten Befehls nach
Serversemantik, kein Beweis einer aktiven Abfrage. Kein Snapshotversprechen.

## Fehler und Umfang

400 `invalid_request`/`invalid_server_admin`/`invalid_connection`, 401,
408 `query_canceled`, 413, 422 `unsupported_action`/`unsupported_feature`/
`database_error`/`result_too_large`; bestehende Fehlerkonventionen gelten weiter.
Fehlende Serverrechte erscheinen als sichere feste Datenbankfehler oder als
serverseitig eingeschränkte Sichtbarkeit, ohne rohe Treiberdetails.

Es gibt keine Routen, Typen oder Engine-Methoden für SET, KILL, FLUSH oder deren
Planung. Schreibfunktionen gehören zur späteren Stufe 2 und benötigen ein eigenes
Paket mit Vertrag und unabhängiger Sicherheitsabnahme. Der WIP-Vertrag v17 wurde
entfernt; `/api/v17/` wird durch dieses Paket nicht registriert. Teilumfang von
PMA-ADMIN-002..005/PMA-NEU-ADMIN-001; keine vollständige Matrixerfüllung.
