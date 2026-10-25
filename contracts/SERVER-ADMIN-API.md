<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Serververwaltung (API 0.17.0, vorläufig)

Additiver Vorschlag zu API 0.8: [Schema](api-v0.17.0.json). Alte Vertragsdateien und
Bestandsschemata bleiben unverändert. Nummer und Vertragsfreeze sind vom Lead und
unabhängigen Review zu bestätigen; kein Web-Lock-Pin und keine Paritätsfreigabe.
Alle Routen benötigen Bearer-Authentifizierung, Antworten verwenden `no-store`.
Unbekannte JSON-Felder werden abgewiesen; Verbindung und Passwort sind ausschließlich
flüchtige Anfragewerte. Fehler enthalten feste deutsche Meldungen ohne SQL,
Treibertexte, Passwörter oder Dateipfade. Nur die ausdrücklich verlangte
Schreibvorschau enthält vom Server erzeugte Verwaltungsanweisungen.

| Route unter `/api/v17/` | Wirkung |
| --- | --- |
| `GET capabilities` | Tatsächlich implementierte Aktionen, Treibergrenzen und Limits |
| `POST status` | Statuszähler mit UTC-Messzeitpunkt |
| `POST variables` | Namenfilter und GLOBAL-/SESSION-Werte; sensible/unklassifizierte Werte maskiert |
| `POST processes` | Sichtbare Clientverbindungen, Benutzer, Datenbank, Laufzeit, Selbstmarkierung, Identitätstoken |
| `POST processes/kill/plan`, `POST processes/kill` | Eine gewählte Abfrage oder Verbindung beenden |
| `POST variables/set/plan`, `POST variables/set` | Eine freigegebene dynamische Zahlenvariable setzen |

## Stufe 1: Lesen

Anfragen: `connection`, optional `limit` (0 = 100, maximal 200), `offset` (maximal
10 000), `timeout_ms` (0 = 5000, maximal 10 000), `filter` (maximal 128 Zeichen).
`scope` gilt nur für Variablen und ist `SESSION` (Standard) oder `GLOBAL`.
Variablennamen/Statusnamen werden ohne Beachtung der Großschreibung nach Teiltext
gefiltert und alphabetisch sortiert. Prozesse sind nach numerischer ID sortiert;
ihre Benutzerfilter folgen der Serverkollation. `has_more` kennzeichnet weitere
Treffer; wechselnde Listen sind keine stabilen Snapshots. Es gibt keine Gesamtzahl.
`sampled_at` bezeichnet den Beginn der Beobachtung, keinen atomaren Messzeitpunkt.

MySQL/MariaDB verwenden `SHOW GLOBAL STATUS`, `SHOW SESSION/GLOBAL VARIABLES`
und `information_schema.PROCESSLIST`. Numerische Statuswerte umfassen auch `Com_*`
als Zähler je Abfragetyp. PostgreSQL liefert eine feste Auswahl aus
`pg_stat_database` für die verbundene Datenbank (Transaktionen, Cache-/Tupelzähler,
Deadlocks, temporäre Bytes), Sitzungswerte aus `pg_settings` und Clientverbindungen
aus `pg_stat_activity`. Keine erfundene PostgreSQL-Entsprechung für MySQL-Abfragetypen;
`GLOBAL`-Variablenlesen/-setzen ist hier nicht unterstützt.
SQLite liefert ausschließlich `sqlite_version()` ohne Dateipfade. Variablen,
Prozesse und Schreibaktionen werden dort mit 422 abgewiesen.

Werte bleiben Strings, damit große Ganzzahlen präzise bleiben. `masked=true`
erfordert `value=null`. Namen mit Passwort-/Secret-/Token-/Key-/Pfad-/Datei-/TLS-
oder Credentialbezug sind gesperrt. Darüber hinaus werden Variablenwerte nur für
bekannte unkritische Zahlenvariablen ausgegeben (`max_heap_table_size`,
`tmp_table_size`, `statement_timeout`, `lock_timeout`, `max_connections`, `port`),
und auch diese nur in numerischer Darstellung. Unbekannte Pluginvariablen bleiben
maskiert. Prozesse enthalten **keinen SQL-Text, Host oder freien Zustandstext**.
Die internen MySQL-Abfragetexte dienen nur dem HMAC-Identitätsvergleich und werden
nicht gespeichert oder ausgegeben. Pro Liste höchstens 4096 gelesene Metadatenzeilen,
pro Zelle höchstens 1 MiB, pro Antwort höchstens 8 MiB; Prozesslisten laden nur eine
Seite plus einen Datensatz. Sichtbarkeit ist vom Serverkonto abhängig.

## Stufe 2: Vorschau und Bestätigung

Vorschauen verändern nichts. Ausführen verlangt identische Eingaben sowie
`expected_plan_sha256` und `confirm: {action, target_name}`. Der Plan wird auf einer
gebundenen Verbindung neu aus Metadaten gebaut und mit `dbplan.Check` geprüft.
Abweichender Plan/Prozess: 409 `plan_mismatch`; ungültiger Hash oder falsche Aktion/
Zielbestätigung: 400 `confirmation_mismatch`. Nicht sichtbares Ziel: 404
`object_not_found`; ungültige Eingabe: 400 `invalid_server_admin`; nicht unterstützte
Funktion: 422 `unsupported_action`. Weitere sichere Transport-/Enginefehler folgen
den bisherigen HTTP-Regeln.

`kill`: `id` als kanonische positive 32-Bit-Zahl, `mode=query|connection` und
`identity` aus der Prozessliste. Für `query` ist **query_identity**, für `connection`
**identity** zu senden. Das Bestätigungsziel ist exakt **ID:Benutzer**, vom Server
ermittelt, nicht aus einem ungeprüften Anfragenamen übernommen. MySQL/MariaDB nutzen
`KILL QUERY/CONNECTION`, PostgreSQL `pg_cancel_backend/pg_terminate_backend`.
Die aktive Verwaltungssitzung und Systemprozesse sind gesperrt. Es werden keine
Rollen/Rechte verändert und keine Zugangsdaten-/Verbindungsvariablen angeboten.
Die Datenbank prüft die tatsächlichen Rechte beim Eingriff; Fähigkeiten versprechen
keine Berechtigung.

`set_variable`: `name`, `scope`, `value` (kanonische dezimale Ganzzahl als String).
Freigabe ausschließlich:

| Treiber | Variable | Scope | Wertebereich |
| --- | --- | --- | --- |
| MySQL/MariaDB | `tmp_table_size`, `max_heap_table_size` | SESSION oder GLOBAL | 16384 bis 1073741824 |
| PostgreSQL | `statement_timeout`, `lock_timeout` | SESSION | 100 bis 600000 Millisekunden |

Der exakte Name muss zusätzlich im Serverkatalog vorkommen; PostgreSQL verlangt
`context=user|superuser`. Keine frei wählbaren Namen, SQL-Fragmente, persistenten
Konfigurationen oder Variablen zur Authentifizierung, Verbindungsabschaltung,
Protokollierung oder Rechteumgehung. `target_name` ist exakt **SCOPE:Variablenname**;
der Wert steht in der bestätigten Anweisung und ist damit vom Planhash abgedeckt.
Ein Kommentar im SQL bindet HMAC-Fingerprints der Verbindung und des bisherigen
Variablenwerts bzw. der Prozessidentität in den Planhash ein. Er enthält keine
Passwörter, Rohabfragen oder privaten Pfade.

**SESSION wirkt nur auf die kurzlebige API-Verbindung dieser Anfrage.** Rücklesen
findet auf derselben Verbindung statt, danach endet die Sitzung. Es gibt keine
Sitzungspersistenz für spätere SQL-Anfragen. GLOBAL wirkt serverweit auf neue
Sitzungen, ohne `SET PERSIST`. MySQL kann Werte runden; `observed_value` zeigt das
getrennte Rücklesen, bei fehlgeschlagenem Rücklesen `null`.

## Ausgang und verbleibende Grenzen

`completed` bedeutet Serverbestätigung, bei Prozessaktionen eine Signalannahme,
nicht den nachgewiesenen Prozessabschluss. PostgreSQL `false` ergibt
`rejected/not_signaled`. Sichere Serverablehnung ergibt `rejected` mit festem
Grundcode (zum Beispiel `permission_denied`). Verbindungs-/Zeitverlust oder
serverseitige Disconnect-/Cancelmeldungen nach Senden ergeben konservativ
`outcome_unknown`; es gibt keinen automatischen Retry. Abbruch vor dem Senden
ist `rejected/canceled`. Rücklesefehler ändern eine bereits bestätigte Ausführung
nicht in eine Ablehnung.

HMAC-Token sind pro Backendprozess gültig und verfallen beim Neustart. Ein anderer
Backendprozess kann denselben Plan nicht bestätigen. Prozess- und Abfragewechsel
werden vor der Ausführung erneut geprüft; danach bleibt ein Rennen bis zum
Servereingriff. PostgreSQL bindet `backend_start`/`query_start`; MySQL bindet
ID/Benutzer/Datenbank/Host und für Abfragen zusätzlich den aktuellen Abfragetext.
Identischer Folgetext oder schnelle Wiederverwendung kann nicht vollständig erkannt
werden. Keine ABA-, Einmaligkeits- oder Exactly-once-Garantie. Clients sperren nach
`completed` und `outcome_unknown` die Wiederholung und verlangen eine neue Vorschau.

FLUSH (`PMA-NEU-ADMIN-005`) bleibt ausdrücklich `unsupported`: Der konkrete Auftrag
beschränkt die Schreibstufe auf KILL und dynamische Variablen. Monitor, Advisor,
Replikation, Binlog, historische Abfragen und Weboberfläche gehören nicht zu diesem
Paket. Synthetische [Fixtures](fixtures/server-admin/README.md) sind keine Livebelege.
