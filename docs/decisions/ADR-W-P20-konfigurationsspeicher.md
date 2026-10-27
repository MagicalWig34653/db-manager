# ADR-W-P20: Serverseitiger Konfigurationsspeicher für Komfortdaten

Status: **Vorgeschlagen** durch Paket W-P20 (Backend), Stand 27.10.2026. Verbindlich werden Entscheidung und
Grenzen erst mit Review und Integration durch den Lead. Baut auf der akzeptierten Entscheidung
[ADR-0006](ADR-0006-serverzustand.md) (externe MariaDB als einziger Serverzustand) und auf Entscheidung E1 in
[PWA-PLAN](../PWA-PLAN.md) auf. Dieses Dokument trennt Ist-Zustand, Anforderung, Entscheidung und offene Punkte.

## 1. Ist-Zustand (geprüft am Quelltext)

- Serverzustand gibt es **nur** als externe MariaDB (`internal/serverstate`, SZ1): eigener Pool, verifiziertes TLS,
  versionierte Migrationen unter einer Sperre, AEAD-Feldverschlüsselung mit Schlüsselkennung, Zustandsprüfung ohne
  Netzzugriff pro Anfrage. Eine lokale SQLite-Variante des Serverzustands existiert **nicht** und ist durch ADR-0006
  (Option a abgelehnt, SQLite nur für verwaltete Nutzerdatenbanken) auch nicht vorgesehen.
- `API.RequireState` existiert, wurde bisher von **keiner** Route benutzt. W-P20 ist die erste Nutzerin.
  Ohne konfigurierten Zustandsspeicher antwortet es mit 503, die Bearer-API läuft unverändert weiter.
- Konten, Passkeys, Sitzungen und Teams gibt es nicht (W-P30). Der Server kennt genau eine Identität: das gemeinsame
  Bearer-Token.
- Die Weboberfläche (W-P16, Branch `feat/w-p16-web`) speichert Verlauf, Lesezeichen und Opt-in im Browser
  (`localStorage`, Schlüssel `db-manager.sql-editor.v1.<SHA-256 der Verbindungsidentität>`), mit Grenzen 200 Einträge,
  512 KiB je Quelle, 100.000 Zeichen SQL, Secret-Ablehnung per Muster, Verlauf standardmäßig aus.

## 2. Anforderung

Aus der Paritätsmatrix: `CON-006` (Favoriten, zuletzt genutzt), `SQL-008`/`SQL-009` (Verlauf, Lesezeichen),
`UX-003`/`UX-004` (persönliche Einstellungen, Export/Import/Reset), `UX-011` (kein falscher Speichererfolg ohne
Speicher), `NEU-NAV-001/002/004/005`, `NEU-DATA-003` (Spaltenpräferenzen), `NEU-UX-002/003` (Reiter, Anzeigeoptionen),
`NEU-MAINT-007` (Zustand des Konfigurationsspeichers sichtbar). Gemeinsam: Komfortdaten sollen Neustarts,
Replikate und Geräte überleben, **ohne** je ein Passwort oder Token zu speichern, und dem Nutzer jederzeit Export und
Löschen geben. Ohne Serverzustand muss der W-P16-Lokalmodus weiterlaufen und das ehrlich erkennbar sein.

## 3. Entscheidung

1. **Speicherort:** die bestehende Serverzustands-MariaDB (ADR-0006), zwei neue Tabellen per Migration 2
   (`config_owners`, `config_items`). Kein zweiter Speicher, keine Datei, kein SQLite-Zustand.
2. **Zuordnung über eine schmale Schnittstelle:** jede Zeile gehört einem `owner_id`. Das Backend löst den Besitzer
   über `httpapi.OwnerResolver` auf. Heute gibt es keinen Resolver und jede authentifizierte Anfrage gehört dem
   konstanten Besitzer `single`; die Fähigkeitsroute meldet `owner_model: "single"`, also **keine** Nutzertrennung.
   W-P30 setzt einen Resolver ein, der die Konto-ID der Sitzung liefert (Zeichen `[A-Za-z0-9_-]`, bis 64), meldet
   `owner_model: "account"` und überführt die Zeilen des Besitzers `single` per einmaligem `UPDATE` auf das
   Administratorkonto. Der Resolver erhält nur den Anfragekontext (nie Kopfzeilen, Query oder Körper): der Besitzer stammt aus serverseitig geprüftem Zustand, nicht aus Daten, die der Client wählt. Der Rückgabewert muss `[A-Za-z0-9_-]{1,64}` entsprechen; der reservierte Besitzer `single` wird abgelehnt (401), damit ein Konto nie die Altdaten des Bearer-Besitzers erreicht. Ein Resolver, der keinen Besitzer findet, ergibt 401. Das Paket nimmt W-P30 nicht vorweg:
   keine Konten, Sitzungen, Rollen oder Freigaben; der Besitzerbegriff ist nur der Schlüssel der Mandantentrennung.
3. **Sammlungen (Kinds):** `history`, `bookmarks`, `searches` (gespeicherte Suchen), `favorites`, `recents`
   (zuletzt benutzt), `settings` (benannte Dokumente, z. B. `navigation`, `table_view`, `privacy`), `layouts`
   (Schlüssel-Dokumente, z. B. `table:<db>.<tabelle>` für Spaltenreihenfolge/-breite). Der Server kennt je Sammlung
   ein festes Schema der Nutzdaten (Abschnitt 5) und lehnt Unbekanntes ab; Dokumente (`settings`, `layouts`,
   `searches.definition`) sind begrenzte, flache JSON-Objekte ohne Null und ohne doppelte Schlüssel.
4. **Zuordnung zur Verbindung (`scope`):** der Client liefert eine undurchsichtige Kennung (`[A-Za-z0-9._:~-]`, bis 128),
   im Web dieselbe SHA-256-Verbindungsidentität wie bei W-P16 (ohne Passwort, Token, Anzeigename). Der Server sieht nie
   Verbindungsdaten. Leerer `scope` bedeutet „global“ (Einstellungen). Abfragen ohne `scope`-Parameter umfassen alle
   Scopes des Besitzers.
5. **API 0.24** (`/api/v24/...`, kumulativ auf 0.23, rein additiv, siehe
   [CONFIG-STORE-API](../../contracts/CONFIG-STORE-API.md)): Fähigkeiten, Status, CRUD je Sammlung mit
   **ETag/If-Match** (Version je Eintrag, `428` ohne Vorbedingung, `412` bei Konflikt), Seitenabruf mit Cursor,
   Import (Stapel, je Eintrag unabhängig, idempotent über `client_ref`), vollständiger Export und **Plan → Bestätigung**
   ausschließlich für das Massenlöschen („alles löschen“, optional je Scope).
6. **`RequireState` und ehrliche Fähigkeiten:** alle Datenrouten laufen hinter Bearer-Prüfung und `RequireState`
   (503 `state_unavailable`, `Retry-After`, fail closed). `GET /api/v24/capabilities` ist **nicht** durch `RequireState`
   geschützt, damit der Client entscheiden kann: `available`/`mode` (`server` oder `local_only`) und `reason`
   (`ok`, `not_configured`, `unreachable`). Bei `local_only` bleibt die Weboberfläche im Lokalmodus; sie darf nie
   „gespeichert“ melden, solange ein Schreibaufruf nicht mit 2xx beantwortet wurde (`UX-011`).
7. **Verlauf nur mit Opt-in, serverseitig erzwungen:** `history` wird nur angenommen, wenn das Dokument
   `settings/privacy` des Besitzers `{"history_enabled": true}` enthält; sonst `409 history_disabled`. Standard ist aus
   (E1). Das Opt-in ist je Besitzer, nicht je Scope. Die Prüfung läuft im Repository **unter der Besitzersperre** unmittelbar vor dem Einfügen, ein gleichzeitiges Abschalten lässt also keinen Eintrag mehr durch.

## 4. Alternativen

| Alternative | Bewertung |
| --- | --- |
| Weiter nur browserlokal | bleibt als Rückfall (Lokalmodus), erfüllt aber keine Gerätesynchronisation und keinen Neustart des Browserprofils; deshalb zusätzlich, nicht stattdessen. |
| Datei/SQLite im Server | widerspricht ADR-0006 (Replikate, NFS-Locking); verworfen. |
| Ein JSON-Blob je Nutzer | Schreibkonflikte über alle Sammlungen, keine Quoten je Sammlung, wachsende Zeilen; verworfen zugunsten einer Zeile je Eintrag. |
| Klartextspalten für SQL/Namen | MariaDB-Administratoren und Dumps sähen alle Abfragen. Verworfen: Nutzdaten werden mit dem Serverschlüssel per AEAD verschlüsselt (wie SZ1 vorsieht). |
| Besitzer = Hash des Bearer-Tokens | koppelt Daten an ein Geheimnis, Rotation verliert alle Daten, Ableitung aus Geheimnis ist unnötiges Risiko; verworfen. |
| Eintragsgrenzen nur im Client | nicht durchsetzbar gegen eigene Clients; verworfen, der Server erzwingt alle Grenzen. |
| Verlauf immer serverseitig mitschreiben | widerspricht E1/Datenschutz; verworfen, Opt-in. |
| Cursor/Seiten über OFFSET | instabil bei gleichzeitigem Schreiben; gewählt wird Keyset-Paging über `(updated_at, id)`. |

## 5. Datenmodell

`config_owners(owner_id PK, created_at)` – eine Zeile je Besitzer, dient als **Schreibsperre je Besitzer**
(`SELECT ... FOR UPDATE`). Dadurch sind Quoten, Verdrängung (FIFO) und Versionsprüfungen auch über mehrere
Replikate exakt, ohne globale Sperre. Verklemmungen werden dreimal wiederholt.

`config_items(owner_id, kind, item_id, scope, nkey, key_id, sealed, plain_size, version, created_at, updated_at)`,
PK `(owner_id, kind, item_id)`, eindeutig `(owner_id, kind, scope, nkey)`, Index `(owner_id, kind, scope, updated_at, item_id)`.
Alle Kennungsspalten ASCII binär (groß-/kleinschreibungsempfindlich).

- **Klartext in der Datenbank** (bewusst, für Abfragen/Quoten): Besitzer, Sammlung, Eintrags-ID (128 Bit zufällig),
  `scope` und `nkey` als **Schlüssel-Hashes** (HMAC-SHA-256, siehe unten), Version, Zeiten, Größe,
  Schlüsselkennung. **Nicht** im Klartext: SQL, Namen, Variablen, Tabellennamen, Schlüssel, Dokumentinhalt, `client_ref` und der Klartext-Scope des Clients (liegt nur im Siegel).
- **`sealed`:** AEAD (AES-256-GCM, `serverstate.Cipher`) über `{"k":<Schlüssel>,"r":<client_ref>,"d":<Daten>}`.
  Zusätzliche authentifizierte Daten `cfg2|owner|kind|id|scope-hash|nkey-hash|version`: eine in einen anderen Besitzer, eine andere ID
  oder einen anderen Scope kopierte Zeile ist **nicht entschlüsselbar**, ebenso ein auf eine ältere Version derselben Zeile zurückgesetzter Siegel-Blob (Rollback). Es gibt kein Altformat (`cfg1` wurde nie veröffentlicht) (`state_unreadable`, kein Klartext). Falscher
  Serverschlüssel führt ebenfalls zu `state_unreadable`; Rotation (alter Schlüssel als zweiter Eintrag) liest alte
  Zeilen und schreibt bei Änderung mit dem neuen Schlüssel.
- **Schlüssel-Hashes:** `scope_hash = HMAC(K, "scope" \0 Besitzer \0 Scope)` und `nkey = HMAC(K, Art \0 Sammlung \0 scope_hash \0 Schlüssel)`.
  `K` ist ein zufälliger Installationsschlüssel in `config_meta`, einmal erzeugt und **sealed** (unter dem Serverschlüssel) gespeichert, nie
  abgeleitet: Rotation des Verschlüsselungsschlüssels ändert keinen Hash, im Dump steht `K` nie im Klartext, ein Wörterbuchangriff auf
  Verbindungs- oder Tabellennamen ist ohne `K` nicht möglich. Der Scope wird je Besitzer gesalzen. Fehlt `config_meta`, obwohl `config_items` Zeilen enthält, wird **kein** neuer Schlüssel erzeugt (`state_unreadable`), damit Zeilen nicht lautlos verwaisen. Der AEAD-Kontext kodiert die Felder längenpräfixiert (injektiv).
- **Natürliche Schlüssel:** `favorites`/`recents` je `(scope, Datenbank, Tabelle)`, `settings`/`layouts` je `(scope, key)`;
  doppeltes Anlegen ist bei Favoriten idempotent (200 mit bestehendem Eintrag), bei `recents` ein „Anfassen“
  (Version +1, ganz nach oben), bei `settings`/`layouts` `409 already_exists` (dann `PUT`). `history`, `bookmarks`,
  `searches` haben keine natürlichen Schlüssel; mit `client_ref` (z. B. lokale Eintrags-ID) sind sie idempotent.
- **Zeit:** `updated_at` setzt der Datenbankserver (UTC, Mikrosekunden). Beim Import darf die Originalzeit erhalten
  bleiben (nicht vor 2000, nicht über 24 h in der Zukunft).
- **Versionen:** je Eintrag, ab 1, `+1` je Änderung; `ETag` ist `"<Version>"`. `If-Match: *` überspringt die Prüfung
  ausdrücklich. Verlaufseinträge sind unveränderlich (nur anlegen/löschen).

Nutzdaten je Sammlung (alle Felder strikt, unbekannte Felder werden abgelehnt):

| Sammlung | `data` |
| --- | --- |
| `history` | `{"sql"}` (nicht leer) |
| `bookmarks` | `{"name","sql","variables"?}` (Variablen: bis 20 Bezeichner `[A-Za-z_][A-Za-z0-9_]{0,63}`, eindeutig) |
| `searches` | `{"name","definition":{...}}` |
| `favorites`, `recents` | `{"database"?,"table"}` |
| `settings` | freies flaches Dokument; `privacy` genau `{"history_enabled": bool}` |
| `layouts` | freies Dokument |

## 6. Größen- und Anzahlgrenzen

Alle Grenzen erzwingt der Server atomar unter der Besitzersperre; die Fähigkeitsroute meldet sie.

| Sammlung | je Scope | je Besitzer | Verhalten bei Erreichen | Eintrag max. |
| --- | --- | --- | --- | --- |
| `history` | 200 | 5.000 | **FIFO**: ältester Eintrag fällt weg | 257 KiB |
| `bookmarks` | 200 | 2.000 | `409 quota_exceeded` | 260 KiB |
| `searches` | 100 | 1.000 | `409 quota_exceeded` | 32 KiB |
| `favorites` | 500 | 5.000 | `409 quota_exceeded` | 2 KiB |
| `recents` | 100 | 500 | FIFO | 2 KiB |
| `settings` | 100 | 200 | `409 quota_exceeded` | 32 KiB |
| `layouts` | 1.000 | 2.000 | `409 quota_exceeded` | 64 KiB |

Weitere Grenzen: **32 MiB Klartext je Besitzer** über alle Sammlungen (bei Erreichen verdrängt jede Einfügung zuerst den ältesten Verlauf, dann `recents`, gleich welche Sammlung eingefügt wird; reicht das nicht, `quota_exceeded`; Änderungen, die einen Eintrag vergrößern, verdrängen nichts und werden abgelehnt); SQL höchstens 100.000 Zeichen **und** 256 KiB; Namen höchstens 100 Zeichen ohne führende/folgende
Leerzeichen; Dokumente höchstens 5 Ebenen tief, 2.000 Knoten, 64 Schlüssel je Objekt, 256 Elemente je Feld, 4 KiB je
Text, nur endliche Zahlen; Anfragekörper 1 MiB (bestehende Standardklasse); Seiten höchstens 100 Einträge und 4 MiB;
Import höchstens 100 Einträge je Aufruf; `client_ref` und Scope nur `[A-Za-z0-9._:~-]`. Text enthält weder
Steuerzeichen (außer Tab/Zeilenumbruch in SQL und Dokumentwerten), NUL, U+FFFD noch ungültiges UTF-8; Namen und
Schlüssel enthalten zusätzlich keine unsichtbaren Formatzeichen. Unicode wird unverändert gespeichert (keine
Normalisierung der Nutzdaten), für die Geheimniserkennung aber NFKC-gefaltet (Abschnitt 7).

## 7. Geheimnisse: nie Passwörter oder Tokens

- Die API **hat keine Felder** für Verbindungsdaten, Passwörter oder Tokens und speichert nichts davon absichtlich.
- **Secret-Ablehnung wie im Web:** SQL, Namen und alle mit ihnen gespeicherten Texte durchlaufen dieselben
  Wortregeln wie `SECRET_PATTERNS` in W-P16 (PASSWORD/PASSWD/PASSPHRASE, IDENTIFIED, secret/token/credential/api_key/
  bearer/private key, `PRAGMA key`/`KEY '...'`, Dollar-Quotes, USER MAPPING/CREATE SERVER/dblink, `uid=`/`pwd=`) und
  zusätzlich **strukturelle** Regeln (auch Basic-Auth-Header, `curl -u`, `password=`/`token=`-Paare): URLs mit `user:passwort@`, PEM-Privatschlüssel, JWT, `Bearer <Wert>`, bekannte
  Tokenformate (GitHub, AWS, Slack, `sk-`, Google), lange undurchsichtige Base64-/Hex-Läufe (32+ Zeichen mit Ziffern
  und Buchstaben). Der Text wird vor der Prüfung NFKC-gefaltet, von unsichtbaren Formatzeichen, kombinierenden Zeichen (U+034F, Variation Selectors) und Hangul-Füllern befreit, und die häufigsten Look-alikes (Kyrillisch/Griechisch) werden auf ASCII abgebildet (kleine Tabelle, kein vollständiges UTS-#39-Skeleton), damit
  `ＰＡＳＳＷＯＲＤ` oder `PASS<ZWSP>WORD` nicht durchrutschen. Treffer ergeben `422 secret_rejected` **ohne** den Treffer zu
  nennen oder zu protokollieren.
- **Dokumente** (`settings`, `layouts`, `searches`, Tabellenverweise) prüfen die strukturellen Regeln und lehnen zusätzlich **Zeichenketten unter
  Schlüsselnamen** wie `password`, `secret`, `token`, `pwd`, `api_key`, `credential` oder `pin` ab; Spaltenlisten (`"columns":["password_hash"]`)
  und boolesche Optionen bleiben speicherbar. Scope, Schlüssel und `client_ref` werden mit den Bezeichnerregeln geprüft. Bezeichner (Scope, Schlüssel, `client_ref`) lassen kurze
  Hex-Hashes zu (der Scope ist ein SHA-256).
- **Das Bearer-Token des laufenden Servers** wird beim Start als verbotener Literalwert registriert; verglichen wird nach derselben Faltung
  (Vollbreite, Zero-Width). Es wird in keinem Text gespeichert (`secret_rejected`).
- **Falsch-Positive (akzeptiert, dokumentiert):** SQL mit einem SHA-256/MD5-Literal im `WHERE`, eine Tabelle namens `tokens`/`secretary`, Spalten wie
  `access_token`, `uid = 0` und Bezeichner ab 32 Zeichen mit gemischten Ziffern/Groß-/Kleinbuchstaben kosten einen Eintrag.
- **Grenze der Erkennung (bewusst offen):** beliebige Geheimnisse in SQL-Zeichenketten (zum Beispiel
  `WHERE pin = '4711'`) sind nicht erkennbar. Die Weboberfläche weist beim Opt-in darauf hin; der Verlauf ist standardmäßig
  aus. Die Erkennung ist konservativ: Fehlalarme kosten einen Eintrag, ein Fehlschlag würde Klartext speichern.

## 8. Migration der lokalen W-P16-Daten

1. Das Web fragt `GET /api/v24/capabilities`. Nur bei `available: true` wird angeboten, lokale Daten zu übernehmen;
   sonst bleibt der Lokalmodus aktiv und sichtbar (`mode: local_only`, `reason`).
2. Opt-in zuerst: ist lokal „Verlauf speichern“ an, legt der Client `settings/privacy` an
   (`{"history_enabled": true}`); ist es aus, wird kein Verlauf übertragen.
3. Dann `POST /api/v24/config/import` in Stapeln zu höchstens 100 Einträgen: je Eintrag `kind` (`history`/`bookmarks`),
   `scope` (Hash-Suffix des lokalen Schlüssels), `client_ref` = lokale Eintrags-ID (UUID, 36 Zeichen), `time` = lokale
   Zeit, `data` wie lokal. Wiederholung ist unschädlich (`exists`); abgelehnte Einträge (z. B. Secret-Treffer) werden
   mit Grund gemeldet und bleiben lokal. Die Reihenfolge innerhalb des Stapels bleibt erhalten, Originalzeiten werden
   übernommen.
4. Der Client löscht lokale Daten erst, **nachdem** alle Stapel ohne Fehler beantwortet wurden und nur auf ausdrückliche
   Bestätigung des Nutzers; Lesezeichen-JSON-Export (W-P16) bleibt als zusätzlicher Weg bestehen.
5. Rückweg: `GET /api/v24/config/export` liefert alles als `dbm-config-export` v1; derselbe Inhalt lässt sich in einen
   anderen Besitzer oder eine andere Installation importieren (`client_ref` wird beim Export erzeugt, damit der
   Import idempotent ist).

## 9. Datenschutz, Löschen, Export

- **Einzeln löschen:** `DELETE` mit `If-Match`. **Sammlung/Scope/alles löschen:** Plan → Bestätigung. Die Vorschau
  nennt je Sammlung Anzahl und Bytes, einen Hash über die betroffenen Zeilen (Kennung und Version) und die
  geforderte Bestätigung `LÖSCHEN <Anzahl>`. Ausgeführt wird nur, wenn der Hash zum Zeitpunkt der Ausführung noch
  stimmt (`409 plan_stale`, sonst nichts gelöscht) und die getippte Bestätigung passt (`422 confirmation_invalid`).
  Das Löschen läuft in einer Transaktion unter der Besitzersperre.
- **Export:** vollständig, ohne Zusatzdaten fremder Besitzer, als Download (`Content-Disposition: attachment`).
- **Opt-out:** Verlauf ausschalten stoppt das Schreiben sofort (409); vorhandene Einträge löscht der Nutzer über
  die Löschvorschau. Beim Löschen des Besitzers (künftig Konto, W-P30) sind dessen Zeilen mit Plan/Bestätigung zu entfernen.
- **Backups:** Löschen wirkt erst in neuen Datenbanksicherungen vollständig; das ist im Betriebshandbuch zu nennen.
- **Sichtbarkeit für Betreiber:** Datenbankadministratoren sehen Besitzer-, Zeit- und Größenmetadaten sowie Anzahl und Art der Einträge je
  (gesalzenem) Scope. Scope, Schlüssel und Inhalte sind ohne Serverschlüssel nicht lesbar oder erratbar (Abschnitt 5).
- **Integrität gegen einen Datenbank-Schreiber ohne Schlüssel:** Zeilentausch, Besitzer-/Scope-/Sammlungswechsel und das Zurücksetzen eines
  Siegels auf eine ältere Version sind erkennbar (`state_unreadable`). **Nicht** erkennbar sind das Löschen von Zeilen, das Wiedereinspielen
  einer gelöschten Zeile samt Versionsspalte und das vollständige Zurücksetzen einer Zeile (Siegel + Version) oder der Datenbank auf eine alte
  Sicherung; Löschmarken würden unbegrenzt wachsen und ein Schreiber könnte sie ebenfalls entfernen. Restrisiko, ins Bedrohungsmodell einzutragen.

## 10. Fehlerklassen (nie Rohfehler)

Feste Codes und Texte, nie Eingabetext, SQL, Treibermeldung, Host oder Pfad: `invalid_request` 400,
`unsupported_media_type` 415, `request_too_large` 413, `item_too_large` 413, `unknown_kind`/`not_found` 404,
`already_exists`/`quota_exceeded`/`history_disabled`/`plan_stale` 409, `version_conflict` 412,
`precondition_required` 428, `invalid_data`/`secret_rejected`/`confirmation_invalid` 422, `state_unreadable` 500,
`state_unavailable` 503 (`Retry-After`). Antworten tragen `Cache-Control: no-store`.

## 11. Konsequenzen und offene Punkte

- Die Isolation zwischen Nutzern ist **nur so stark wie der Resolver**: heute gibt es genau einen Besitzer. Mehrere
  Personen mit dem gemeinsamen Token teilen Verlauf, Lesezeichen und Einstellungen. Das ist ausdrücklich keine
  Nutzertrennung (`PMA-CON-006`-Teilnachweis „getrennte Konten sehen keine fremde Liste“ gilt erst mit W-P30).
- Der Lokalmodus bleibt Pflicht, bis ein Serverzustand betrieben wird; die Weboberfläche muss `capabilities` auswerten.
- Offene Fragen aus ADR-0006 (3: Sitzungsmodell, 6: Mandantentrennung, 8: Verhalten bei DB-Ausfall, 5: Backup/Restore)
  werden nicht entschieden; W-P20 verhält sich fail-closed und nutzt keine Sitzungen.
- Getestet ist die Zielplattform **MariaDB 12.3** (Fixture); MySQL ist für den Serverzustand nicht zugesagt (E3, offene
  Frage 7 aus ADR-0006).
- Abgrenzung: Tabellenstruktur- und Designer-Layouts (W-P21) und Transformationen (W-P22) nutzen `layouts`/`settings`,
  fügen aber keine eigene Speicherlogik hinzu.
