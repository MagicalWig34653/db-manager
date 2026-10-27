# API 0.24: Serverseitiger Konfigurationsspeicher (W-P20)

Maschinenlesbar: [OpenAPI 0.24.0](api-v0.24.0.json), kumulativ und rein additiv auf 0.23 (Wächter
`scripts/check-config-store-contract.py`, Kette `scripts/contract_chain.py`). Entscheidung und Datenmodell:
[ADR-W-P20](../docs/decisions/ADR-W-P20-konfigurationsspeicher.md). Beispiele: [fixtures/config-store](fixtures/config-store/README.md).

Alle Routen verlangen das Bearer-Token. Die Datenrouten laufen zusätzlich hinter `RequireState`: ohne konfigurierten
oder erreichbaren Serverzustand antworten sie `503 state_unavailable` mit `Retry-After: 5`. Nur
`GET /api/v24/capabilities` antwortet immer und sagt ehrlich, ob der Serverzustand verfügbar ist. Alle Antworten tragen
`Cache-Control: no-store`. Fehler sind `{"error":{"code","message"}}` mit festen Texten (nie Eingabe, SQL, Treibertext).

## Fähigkeiten und Modus

`GET /api/v24/capabilities` liefert `available`, `mode` (`server` | `local_only`), `reason` (`ok` | `not_configured` |
`unreachable`), `owner_model` (`single` heute; `account` mit W-P30), `history_requires_opt_in`, `secrets_policy: reject`,
die Grenzen je Sammlung und die Gesamtgrenzen. Bei `local_only` **muss** der Client im Lokalmodus (W-P16) bleiben und
darf keinen Speichererfolg melden. Der Wert wird aus dem zwischengespeicherten Zustand gelesen (kein Datenbankzugriff
pro Anfrage); Erholung nach einem Ausfall braucht keinen Neustart.

## Sammlungen

`history`, `bookmarks`, `searches`, `favorites`, `recents`, `settings`, `layouts`. Nutzdaten (`data`, strikt, unbekannte
Felder werden mit `422 invalid_data` abgelehnt):

| Sammlung | `data` | Schlüssel (`key`) | `client_ref` |
| --- | --- | --- | --- |
| `history` | `{"sql"}` | – | ja |
| `bookmarks` | `{"name","sql","variables"?}` | – | ja |
| `searches` | `{"name","definition"}` | – | ja |
| `favorites` | `{"database"?,"table"}` | abgeleitet | – |
| `recents` | `{"database"?,"table"}` | abgeleitet | – |
| `settings` | Dokument; `privacy` = `{"history_enabled":bool}` | Pflicht `[a-z][a-z0-9_.-]{0,63}` | – |
| `layouts` | Dokument | Pflicht, bis 200 Zeichen | – |

`scope` (optional, Standard leer = global) ordnet einen Eintrag einer Verbindung zu: undurchsichtige Kennung
`[A-Za-z0-9._:~-]{0,128}`; der Server erhält nie Verbindungsdaten.

## Routen

| Route | Zweck |
| --- | --- |
| `GET /api/v24/config/status` | Verbrauch je Sammlung, Bytes, Schemaversion |
| `GET /api/v24/config/{kind}?scope=&key=&limit=&cursor=` | Seite, neueste zuerst; ohne `scope` alle Scopes; `limit` 1 bis 100 (Standard 50); `key` nur bei `settings`/`layouts` und mit `scope`; unbekannte oder wiederholte Parameter: 400 |
| `POST /api/v24/config/{kind}` | Anlegen: `{"scope"?,"key"?,"client_ref"?,"data"}`; `201` neu mit `ETag`/`Location`; `200` bei bereits vorhandenem idempotentem Eintrag (Favorit, `client_ref`, `recents`: Eintrag wird nach oben gezogen, Version +1) |
| `GET /api/v24/config/{kind}/{id}` | Lesen, `ETag: "<Version>"` |
| `PUT /api/v24/config/{kind}/{id}` | Ändern: `{"data"}`, **`If-Match` Pflicht** (`"<Version>"` oder `*`); `428` fehlt, `412 version_conflict` veraltet; `history` und abgeleitete Schlüssel (`favorites`, `recents`) sind nicht änderbar |
| `DELETE /api/v24/config/{kind}/{id}` | Löschen, `If-Match` Pflicht, `204` |
| `POST /api/v24/config/import` | bis 100 Einträge `{"kind","scope"?,"key"?,"client_ref"?,"time"?,"data"}` + `overwrite_documents`; je Eintrag `created`/`exists`/`updated`/`rejected` mit Grundcode, nie Eingabetext; Originalzeit (RFC 3339) bleibt erhalten |
| `GET /api/v24/config/export` | vollständiger Export `dbm-config-export` v1 als Download |
| `POST /api/v24/config/clear/plan` | Vorschau: `{"kinds":[...],"scope"?}` → Anzahl/Bytes je Sammlung, `plan_hash`, `confirmation` (`LÖSCHEN <n>`) |
| `POST /api/v24/config/clear/execute` | `{"kinds","scope"?,"plan_hash","confirmation"}`; `409 plan_stale` bei Änderung, `422 confirmation_invalid`; sonst Transaktion |

Der Besitzer jeder Anfrage kommt aus dem Besitzerauflöser des Servers (`single`), nie aus Anfrageinhalten; ein Auflöser kann `single` nicht liefern. Doppelte Schlüssel irgendwo im Anfragekörper ergeben `400 invalid_request`.

## Verbindliche Eigenschaften

- **Optimistic Concurrency:** Version je Eintrag; gleichzeitige Schreiber mit derselben Version: genau einer gewinnt.
- **Grenzen** erzwingt der Server atomar (auch über Replikate): je Scope/Besitzer/Bytes laut ADR; `history` und `recents`
  verdrängen bei der Anzahlgrenze den ältesten Eintrag (FIFO), alle anderen Sammlungen antworten `409 quota_exceeded`. Die Byte-Grenze je Besitzer
  verdrängt bei jeder Einfügung zuerst Verlauf, dann `recents`; reicht das nicht: `409 quota_exceeded`.
- **Verlauf nur mit Opt-in:** ohne `settings/privacy` = `{"history_enabled":true}` antwortet `history` mit `409 history_disabled`.
- **Geheimnisse:** SQL, Namen und Dokumentwerte mit Passwort-/Token-/Schlüsselmustern, URLs mit Zugangsdaten, PEM, JWT,
  bekannten Tokenformaten, langen undurchsichtigen Läufen oder dem Bearer-Token des Servers: `422 secret_rejected`
  (ohne Treffernennung); gilt auch für `scope`, `key` und `client_ref`. Dokumente lehnen zusätzlich Zeichenketten unter Schlüsseln wie `password`/`token`/`secret` ab. Die Erkennung ist konservativ mit Falsch-Positiven (SHA-256-Literal im `WHERE`, Tabelle `tokens`, Bezeichner ab 32 gemischten Zeichen) und Lücken (siehe ADR).
- **Text:** gültiges UTF-8 ohne NUL/Steuerzeichen/U+FFFD; kein JSON-Null, keine doppelten Schlüssel, höchstens 5 Ebenen.
- **Verschlüsselung:** Nutzdaten liegen mit dem Serverschlüssel per AEAD verschlüsselt in der Datenbank, gebunden an
  Besitzer, Sammlung, ID, Scope und Version; Scope und Schlüssel stehen nur als Schlüssel-Hash (HMAC) in der Datenbank.
- **Verlauf-Opt-in** wird unter der Besitzersperre unmittelbar vor dem Schreiben geprüft.

## Fehlercodes

`invalid_request` 400, `unsupported_media_type` 415, `request_too_large` 413, `item_too_large` 413, `unknown_kind` 404,
`not_found` 404, `already_exists` 409, `quota_exceeded` 409, `history_disabled` 409, `plan_stale` 409,
`version_conflict` 412, `precondition_required` 428, `invalid_data` 422, `secret_rejected` 422,
`confirmation_invalid` 422, `state_unreadable` 500, `state_unavailable` 503, `unauthorized` 401.

## Nicht Teil von 0.24

Konten, Sitzungen, Teilen, Rollen (W-P30); Synchronisationsereignisse oder Push; serverseitige Volltextsuche im Verlauf;
Tresorinhalte (ADR-0005, bleiben clientseitig verschlüsselt); jede Speicherung von Verbindungsdaten.
