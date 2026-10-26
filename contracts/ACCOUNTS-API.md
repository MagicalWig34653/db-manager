# Datenbankkonten und Rechte (API 0.15)

Additiver [OpenAPI-Vertrag](api-v0.15.0.json) zu API 0.12; bisherige Vertragsbytes bleiben
unverändert. Die Nummer 0.15 ist endgültig vergeben (Integration Welle 3). Kein Web-Lock-Pin, keine Produktkonten oder
Änderung des Bearer-Token-Modells (E2). E9 erlaubt bestätigte Datenbankadministration.
Alle Routen sind authentifiziert; JSON-Anfragen höchstens 1 MiB, keine Nullwerte,
unbekannten oder aktionsfremden Felder. Dokumentierter Umfang ist Backend; keine Webintegration.

## Routen und Eingaben

`GET /api/v15/capabilities` liefert die statischen Treibergrenzen. Alle weiteren Routen
liegen unter `POST /api/v15/accounts/` und tragen `connection` sowie optional
`timeout_ms` (Standard 30000, höchstens 60000).

| Route | Zusätzliche Eingaben | Ergebnis |
| --- | --- | --- |
| `list` | `limit` (Standard 100, maximal 500), `offset` (maximal 1000000) | Konten mit Host, Status, Ressourcenlimits, Plugin-/TLS-Anzeige; `truncated` |
| `inspect` | `account` | Konto, direkte Rechte, direkte Rollenmitgliedschaften, Grenzen |
| `access` | keine | konservative Liste sichtbarer Schreibaktionen für diese Verbindung |
| `<aktion>/plan` | aktionsspezifische Felder, niemals `secret` | Vorschau, Bestätigungsanforderungen, SHA-256 |
| `<aktion>` | dieselben Felder, `confirm`, `expected_plan_sha256`, ggf. `secret` | `completed`, `rejected`, `outcome_unknown` |

`account`, `new_account`, `role`: `{name, host?}`. MySQL-/MariaDB-Konten verlangen
expliziten Host; PostgreSQL hat keinen Hostteil. Neue Namen sind höchstens 32 Byte
(MySQL/MariaDB, konservativ) oder 63 Byte (PostgreSQL), Host höchstens 255 Byte,
keine NUL-/Zeilenumbruchzeichen. MariaDB-Rollen haben den leeren Host; `inspect` prüft dafür zusätzlich `is_role` im Katalog. Benutzeränderungen verlangen weiterhin einen Host.
Bestehende Ziele werden exakt aus dem Katalog aufgelöst und dann gequotet;
neue Namen werden validiert, auf Kollision geprüft und gequotet.

| Aktion | Felder außer Verbindung/Zeitlimit | Getippter Zielname |
| --- | --- | --- |
| `create` | `new_account` | nein |
| `rename` | `account`, `new_account` | ja |
| `drop` | `account` | ja |
| `password` | `account` | ja |
| `grant`, `revoke` | `account`, `grant` | bei `revoke` ja |
| `lock`, `unlock` | `account` | bei `lock` ja |
| `role_grant`, `role_revoke` | `account`, `role` | bei `role_revoke` ja |
| `copy` | `account`, `new_account` | nein |
| `flush` | keine | ja, `Serverrechte` |

`create`, `copy`, `password` verlangen ausschließlich bei Ausführung `secret`
(1 bis 4096 Byte UTF-8, ohne NUL). Der Wert ist geschützt und `writeOnly`.
Die Vorschau enthält ausschließlich `<protected-secret>` an der vorgesehenen
Passwortposition. Dieser feste Platzhalter, niemals das Passwort oder dessen Hash,
geht in `dbplan.Digest` ein. Erst nach vollständiger Bestätigung ersetzt execute die
letzte Platzhalterposition der ersten Anweisung durch ein gequotetes Literal auf der
vorbereiteten Sitzung. Es gibt keine Request-/SQL-/Passwortprotokollierung im Paket,
keine Hashwerte von Serverpasswörtern und keine Geheimnisse in Antworten oder Fehlern.
Die Zugangsdaten der Verbindung bleiben wie bisher flüchtige Eingabewerte.

`confirm.action` muss der Route entsprechen. Für destruktive Aktionen muss
`confirm.target_name` exakt dem Vorschauwert entsprechen. Die eindeutige Kontokennung
lautet `<UTF-8-Byteanzahl des Namens>:<Name>@<Host>`; Clients zeigen den Vorschauwert
zum Abtippen an. `confirm.acknowledge_self=true` ist zusätzlich erforderlich, wenn
`self=true`: eigenes Konto bei destruktiven Aktionen, vorsorglich jede Löschung und
jeder Rechte-/Rollenentzug (auch indirekte Selbstaussperrung), sowie `flush`.

## Rechte und Rollen

`grant` enthält `scope`, festen `privilege`-Namen, `grant_option` und die zum Scope
gehörenden Felder. Keine freien SQL-Fragmente, keine pauschalen ALL-/CASCADE-Befehle.

| Scope | Objektauflösung |
| --- | --- |
| `global` | MySQL/MariaDB `*.*`; PostgreSQL-Rollenattribute separat nur lesend |
| `database` | `database` aus Serverkatalog |
| `table` | `database`, `object`; PostgreSQL zusätzlich `schema` in verbundener Datenbank |
| `column` | zusätzlich `column` aus Katalog |
| `routine` | `database`, `object`, `routine_kind=function/procedure`; PostgreSQL zusätzlich `schema`, `routine_id` als Katalog-OID |

PostgreSQL erzeugt Routinenargumente ausschließlich mittels
`pg_get_function_identity_arguments` aus Katalogmetadaten; überladene Routinen bleiben
eindeutig. Tabellen einschließlich materialisierter Sichten kommen aus `pg_class`.
MySQL/MariaDB-Datenbanknamen mit `_`, `%` oder Backslash werden für Datenbankrechte
abgewiesen: GRANT interpretiert sie als Muster und könnte fremde Datenbanken treffen.
Tabellen-/Spaltenrechte für diese Datenbanken bleiben möglich.
Die MySQL-/MariaDB-GRANTEE-Anzeige wird binär und als gebundener Rohwert verglichen.
Erzeugen mehrere Konten denselben mehrdeutigen Anzeigetext (eingebettete Quote-/@-Folgen),
werden Details und Kopie sicher abgewiesen; Konten werden nicht zusammengeführt.
Dynamische Rechte aus mehreren Katalogquellen werden dedupliziert. Globale MariaDB-Rollenrechte werden aus kataloggeprüften Privilegspalten von `mysql.user` ergänzt, da `USER_PRIVILEGES` Rollen auslässt. Enthält eine Rolle moderne oder unbekannte globale Rechte außerhalb der dort dargestellten Bits 0 bis 29, wird `inspect` sicher als nicht unterstützt abgewiesen; der reine Rechtebitmap wird dafür ohne Authentifizierungsdaten geprüft.

`revoke` mit `grant_option=true` entzieht die Weitergabeberechtigung: PostgreSQL
für das Einzelrecht; MySQL/MariaDB für den gesamten gewählten Geltungsbereich,
mit ausdrücklicher Warnung in der Vorschau. Ohne dieses Flag wird das Recht selbst
entzogen. Neue Konten verwenden das vom Server festgelegte Standardverfahren.
Plugin, SSL-Typ und Ressourcenlimits werden angezeigt, nicht verändert.

`copy` erzeugt ein neues Konto mit eigenem Passwort und kopiert direkte SQL-Rechte
(einschließlich GRANT OPTION) sowie direkte Rollenmitgliedschaften ohne ADMIN OPTION.
Es kopiert keine Passwörter, Plugins, TLS-Vorgaben, Ressourcenlimits, Standardrollen,
Besitzrechte oder PostgreSQL-Rollenattribute. Nicht unterstützte direkte Rechte
führen vor jeder Änderung zur Ablehnung des gesamten Plans. Es werden keine direkten
Systemtabellenänderungen vorgenommen. `flush` ist ein explizites, bestätigtes
`FLUSH PRIVILEGES` (MySQL/MariaDB); normale Kontenbefehle benötigen es nicht.

## Fähigkeiten, Berechtigungen und Grenzen

SQLite: alle Kontenaktionen `unsupported`. MySQL-Rollen ab 8, MariaDB-Rollen ab
10.0.5; Kontosperren MySQL ab 5.7.6, MariaDB ab 10.4.2. Unbekannte Versionen werden
abgewiesen. PostgreSQL sperrt mit NOLOGIN und entsperrt mit LOGIN; keine Beendigung
bestehender Sitzungen. Die statische Fähigkeit bedeutet keine Berechtigung.
`access` zeigt konservativ nur bestätigten Administratoren Schreibaktionen an;
objektbezogene oder geerbte Berechtigungen können mehr erlauben. Ausblenden ist
keine Sicherheitsgrenze. Jede Ausführung unterliegt unverändert den Serverrechten.

MySQL-Listen/Details benötigen Zugriff auf `mysql.user`, für vollständige Details
auch Rollen-/Routinen-/dynamische Rechtekataloge. Fehlende Rechte führen zu einem
sicheren Fehler, nicht zu einer angeblich vollständigen leeren Liste. Das eigene
Passwort lässt sich über die serverseitige CURRENT_USER-Metadatenkennung auch ohne
SELECT auf mysql.user planen. PostgreSQL zeigt Rollenattribute und Verbindungslimit;
Auth-Plugin/TLS-Regeln sind unbekannt (`null`, pg_hba.conf). Ererbte, PUBLIC-, Besitz-,
Default-ACL-, Schema-, Sequenz- und andere nicht angebotene Objektrechte gehören
nicht zur direkten Rechteansicht oder Kopie. MariaDB zeigt das primäre Plugin,
kein vollständiges Mehrfachauthentifizierungsmodell.

Listen sind paginiert; Detailrechte und Rollen sind auf je 500 begrenzt und schlagen
bei Überschreitung fehl. Plan höchstens 1001 Anweisungen, je 16 KiB, insgesamt 1 MiB.
Jede Anfrage hat ein Zeitlimit. Es gibt keinen serverseitigen persistenten Planzustand.
Hash umfasst Treiber und deterministische, normalisierte Anweisungen einschließlich
Passwortplatzhalter; Warnungen, effektive Rechte und Objektidentität sind kein CAS.
Namensersetzung zwischen Katalogprüfung und Ausführung ist ein bekanntes Zeitfenster.
Nach `completed` oder `outcome_unknown` sperren Clients Wiederholung bis zu einem
neuen Plan und neuer Bestätigung. Niemals automatischer Retry.

PostgreSQL führt die Schreibfolge in einer Transaktion aus; Commitfehler bedeuten
`outcome_unknown`. MySQL/MariaDB bestätigen einzeln: Fehler nach schon wirksamen
Schritten bedeuten ebenfalls `outcome_unknown`, mit Anzahl bestätigter Anweisungen.
`rejected` bedeutet keine Änderung; bekannte Serverfehler werden auf feste Codes und
deutsche Texte reduziert. Die Antwort behauptet keinen nachträglichen Katalogsnapshot;
Clients lesen nach Ausführung separat erneut. Ein gelöschtes Konto kann vorhandene
Sitzungen weiterbenutzen; Anmeldung und aktive Sitzung sind verschiedene Zustände.

Fehler vor Ausführung: 400 `invalid_request`/`invalid_account_request`/
`confirmation_mismatch`, 403 `permission_denied`, 404 `object_not_found`, 409
`object_exists`/`plan_mismatch`, 422 `unsupported_action`/`unsupported_feature` und
sichere Datenbankfehler, 408 Abbruch, 413 Anfragegröße, 415 Medientyp.

Prüfstand und offene Live-Gates: [W-P18-Backend](../docs/handoffs/W-P18-backend.md).
