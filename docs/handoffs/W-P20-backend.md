# W-P20 Backend: Serverseitiger Konfigurationsspeicher (API 0.24)

Stand: 27.10.2026. Worktree `.worktrees/w-p20-backend`, Branch `feat/w-p20-backend`, Basis `main` (`84a5e2e`).
Nichts nach main gemergt; Lead integriert nach unabhängigem Review. Zentrale Status-/Planungsdateien, `contracts/README.md`
und `docs/BETRIEB.md` bleiben unverändert beim Lead. Kein Webpin, kein Assetbuild, kein Deployment.

## Ergebnis

- [ADR-W-P20](../decisions/ADR-W-P20-konfigurationsspeicher.md) (Status: vorgeschlagen): Ort (Serverzustands-MariaDB nach
  ADR-0006), Besitzerschnittstelle, Datenmodell, Grenzen, Geheimnisse, Migration der W-P16-Daten, Datenschutz.
- **API 0.24** unter `/api/v24/`, [Verhaltensvertrag](../../contracts/CONFIG-STORE-API.md), OpenAPI
  `contracts/api-v0.24.0.json` (SHA-256 `e2f2e057f341e52d23b9ec098c2a1b04e5f90dd18f5e560cbfaad844f463da7a`), rein additiv auf 0.23.
  `scripts/contract_chain.py` pinnt jetzt v1 bis v23 (neu: v23) und prüft die Kette bis v24.
- Sieben Sammlungen (Verlauf, Lesezeichen, gespeicherte Suchen, Favoriten, zuletzt benutzt, Einstellungen, Layouts),
  CRUD mit ETag/If-Match, Cursor-Seiten, Stapelimport (idempotent), Export, Löschen nur über Plan → Bestätigung,
  Status, ehrliche Fähigkeitsroute (`available`, `mode`, `reason`).
- Erste Nutzung von `RequireState` (503 fail closed); `GET /api/v24/capabilities` bewusst ohne `RequireState`.

## Was der Serverzustand heute kann (Befund)

Nur externe MariaDB (SZ1: TLS, Migrationen unter `GET_LOCK`, AEAD, Healthcache). **Keine** SQLite-Variante des
Serverzustands, **keine** Konten (W-P30). Daher: Besitzer über `httpapi.OwnerResolver`; heute konstant `single`
(`owner_model: "single"`, keine Nutzertrennung). W-P30 setzt einen Resolver auf die Konto-ID und stellt per `UPDATE` um.
`RequireState` war vorher von keiner Route benutzt.

## Dateien

| Datei | Inhalt |
| --- | --- |
| `internal/configstore/{types,errors,validate,secrets,service}.go` | Richtlinie: Sammlungen, Grenzen, Validierung, Geheimnisablehnung, Versionen, Plan/Bestätigung, Import/Export; schmale `Repo`-Schnittstelle |
| `internal/configstore/memrepo.go`, `repotest/suite.go`, `memrepo_test.go` | In-Memory-Repo (nur Tests) und die gemeinsame Verhaltenssuite (15 Szenarien), die auch auf MariaDB läuft |
| `internal/serverstate/migrate.go` | Migration 2 `config_store` (`config_owners`, `config_items`) |
| `internal/serverstate/configrepo.go` | MariaDB-Repo: AEAD mit Kontextbindung, Besitzersperre, FIFO, Quoten, Digest/Clear |
| `internal/serverstate/configrepo_live_test.go`, `configapi_live_test.go` | Livetests (Konformanz, zwei Replikate, Verschlüsselung/Bindung/Rotation, Ausfall/Erholung, HTTP-Fluss mit echten Antworten) |
| `internal/httpapi/config.go`, `config_test.go` | Routen, `WithConfigStore`, `WithOwnerResolver`, HTTP-Tests |
| `internal/httpapi/server.go`, `features.go`, `features_test.go` | Felder `config`/`owners`, Token als verbotener Literal, ein Featureeintrag, Zähler 22 → 23 |
| `cmd/db-manager/main.go` | Dienst verdrahtet den Konfigurationsspeicher, wenn der Serverzustand konfiguriert ist |
| `contracts/api-v0.24.0.json`, `CONFIG-STORE-API.md`, `fixtures/config-store/*` | Vertrag, Verhalten, 35 Beispiele (12 echt, 12 Platzhalter `…placeholder…`, 11 `.invalid`, siehe README dort) |
| `scripts/check-config-store-contract.py`, `scripts/test-config-store.py`, `scripts/contract_chain.py` | Wächter, Livetest mit Wegwerf-MariaDB, Kettenpin v23 |
| `.github/workflows/config-store.yml` | CI wie die übrigen Paketworkflows |

Keine neuen Abhängigkeiten (`golang.org/x/text` ist bereits direkt eingebunden und inventarisiert).

## Prüfstand (ausgeführt)

- `gofmt -l`: leer. `go vet ./...`: sauber. `go test -race -count=1 ./...`: alle 11 Pakete ok (Datenbankpaket 157 s).
- `python3 scripts/test-config-store.py`: grün gegen **MariaDB 12.3 mit TLS** (Wegwerfcontainer `dbmanager-w-p20-*`,
  danach entfernt, `leftovers()` leer): Konformanzsuite (15 Szenarien) auf MariaDB, zwei Replikate (genau ein Gewinner
  je Version, Quote exakt über Replikate), Klartext/Schlüsselmaterial nicht im Dump, in andere Besitzer/Scopes kopierte
  Zeilen nicht lesbar, falscher Schlüssel → `state_unreadable`, Schlüsselrotation (alter Schlüssel liest, Schreiben mit neuem),
  Laufzeitnutzer ohne DDL, Ausfall → 503 und `local_only/unreachable`, Erholung ohne Neustart, HTTP-Fluss mit 12 echten
  Antwortdumps gegen API 0.24 validiert. Bestehende SZ1-Tests (Migration einmalig, Plan, Mindestrechte, Verschlüsselung) laufen
  auf der neuen Migration weiter grün.
- Alle `scripts/check-*-contract.py` (23) und `scripts/check-docs.py` grün.
- Angriffstests in der gemeinsamen Suite: 22 Geheimnisfälle (inkl. Vollbreite, Zero-Width, Soft-Hyphen, URL mit Passwort,
  PEM, JWT, Tokenformate, Base64/Hex-Läufe), Gutfälle (Spaltennamen wie `password_hash` in Layouts, Hash-Scope),
  Unicode/NUL/Steuerzeichen/U+FFFD/ungültiges UTF-8, Größen an der Grenze (100.000 Zeichen innen/außen, Mehrbyte außen),
  Injektionsnutzlasten in Daten und Bezeichnern (bleiben Daten, Bezeichner abgelehnt), JSON (doppelte Schlüssel, Null, Tiefe,
  Knoten, Arrays, Zahlen), Konkurrenz (16 Schreiber, Lösch-gegen-Änderungs-Rennen, Quote 20 Schreiber auf 5 Plätze, zwei Replikate),
  Besitzerisolation, Planveralterung, falsche Bestätigung.

## Nicht ausgeführt / offen

- **MySQL 8.4 / PostgreSQL / SQLite als Serverzustand:** nicht Teil (ADR-0006: MariaDB). Kein Test auf MariaDB 10.11.
- **Web:** keine Anbindung (Weboberfläche liest `capabilities`, migriert nach ADR Abschnitt 8). Das Web-Paket (Folgepaket zu
  W-P16) ist offen; die Migration der lokalen Daten ist nur als API-Mechanismus (Import) und Ablauf belegt, nicht gegen die echte Weboberfläche.
- **Echte Nutzertrennung:** erst mit W-P30; heute ein Besitzer. Parallele Fenster/Geräte desselben Tokens teilen alles.
- **Restrisiko Metadaten:** `nkey`/`scope` sind ungesalzene Hashes; wer die Datenbank liest, kann bekannte Tabellennamen
  erraten (ADR 9). Entscheidung über einen schlüsselabhängigen Hash (Rotation!) steht aus.
- **Geheimniserkennung** ist heuristisch; beliebige Geheimnisse in SQL-Zeichenketten bleiben möglich (ADR 7).
- **Betrieb:** Löschen wirkt in Backups erst mit der nächsten Sicherung; `docs/BETRIEB.md` und `contracts/README.md` (Index) sind vom Lead zu ergänzen;
  Migration 2 muss vor dem Start der neuen Version mit dem Migrationsbenutzer laufen (`state-migrate`), sonst `ErrSchemaOutdated` beim Start.
- Unabhängiges Review und Integration stehen aus.

## Nächster Schritt

Review (Sicherheit: Geheimnisregeln, AEAD-Bindung, Besitzersperre; Vertrag: Additivität), danach Web-Paket für `capabilities`,
Import der W-P16-Daten und Einstellungs-/Layout-Oberflächen (`UX-003/004`, `NEU-NAV-*`, `NEU-DATA-003`).

## Review-Auflagen umgesetzt (27.10.2026, nach [Review](W-P20-backend-review.md), `review/w-p20` @ `1413e8f` und `main` @ `daceabb` eingemerged)

| Befund | Umsetzung |
| --- | --- |
| F-02 | AEAD-Kontext `cfg2|Besitzer|Sammlung|ID|scope-hash|nkey-hash|Version`; Insert sealt Version 1, Update `alt+1` unter der Sperre. Kein Altformat (nie veröffentlicht). Rollback des Siegels → `state_unreadable` (Live-Test `TestReviewLiveHolds_RollbackRejected`). **Bewusst nicht gelöst:** Wiedereinspielen einer gelöschten Zeile samt Versionsspalte und vollständiges Zurücksetzen auf eine alte Sicherung (Löschmarken wüchsen unbegrenzt und wären für einen Schreiber ebenso löschbar); als Restrisiko in ADR Abschnitt 9 |
| F-03 | `OwnerResolver func(ctx) (owner, ok)`: nur Kontext, nie Request; Rückgabe `single` → 401, Format wird im Service geprüft (sonst 400) |
| F-05 | Scope und `client_ref` mit Bezeichnerregeln auf Geheimnisse geprüft |
| F-01/F-11 | `scope` und `nkey` als HMAC-SHA-256 mit zufälligem Installationsschlüssel (`config_meta`, sealed gespeichert, von Rotation unabhängig, nie im Dump); Scope je Besitzer gesalzen, Klartext-Scope nur im Siegel. Wörterbuchangriff scheitert (`TestReviewLiveHolds_NKeyDictionaryAttackFails`, inkl. Rotation) |
| F-06/F-07/F-04 | Dokumentwerte unter Schlüsseln wie `password/token/secret/pwd/api_key/pin` abgelehnt, Strukturregeln um `password=`/`token=`-Paare, `curl -u`, Basic-Auth erweitert; Bearer-Literal nach Faltung verglichen; Faltung entfernt Mn/Me (U+034F, Variation Selectors) und Hangul-Füller und bildet ~60 Kyrillisch/Griechisch-Look-alikes auf ASCII ab (**keine** vollständige UTS-#39-Skeleton-Tabelle, dokumentiert) |
| F-08/F-09 | 32-Hex-Läufe, `curl -u`, Basic-Auth erkannt; Falsch-Positive (SHA-256-Literal im WHERE, Tabelle `tokens`, Bezeichner ab 32 gemischten Zeichen) in ADR und API-Doku dokumentiert, nicht reduziert; Base32 bleibt unerkannt |
| F-10 | Verhalten umgesetzt: Byte-Quote verdrängt bei jeder Einfügung zuerst Verlauf, dann recents (Memory und MariaDB); ADR/Vertrag angepasst |
| F-12 | Migration 2 hat `Verify` (Primär-/Eindeutigkeitsschlüssel aus `information_schema`), vor dem Verbuchen; Abbruch mit `Migration fehlgeschlagen`. Migration 2 enthält jetzt auch `config_meta` (noch nicht veröffentlicht, Text geändert) |
| F-13 | Doppelte Schlüssel in der gesamten Anfrage → 400 (`decodeConfig`), der gemeinsame `decode` blieb unverändert |
| F-14 | `docs/BETRIEB.md`: Migration vor Start, Altprogramm gegen Schema 2, Schlüsselverlust/Rotation, Sicherungen |
| F-15 | v24- und v23-Fälle in `TestAllKnownRoutesRegisteredAndAuthenticated` (v23 hat keine Pfade: 404); Wächter pinnt Quoten und Größen je Sammlung |
| F-16 | Opt-in-Prüfung als `Requirement` im Repository unter der Besitzersperre (Memory und MariaDB), auch für Import |

`TestReviewGap_*`/`TestReviewLiveGap_*` sind zu `TestReviewHolds_*`/`TestReviewLiveHolds_*` umgedreht (F-08/F-09-Tests protokollieren nur).
`contracts/api-v0.24.0.json` unverändert (SHA `e2f2e057…`), Fixtures neu aus dem Livelauf, Index in `contracts/README.md` ergänzt.
Offen unverändert: Web-Anbindung, echte Nutzertrennung (W-P30), heuristische Geheimniserkennung.

## Re-Review-Auflagen umgesetzt ([Re-Review](W-P20-backend-rereview.md), `review/w-p20-rereview` @ `0f68567` eingemerged)

- **B-01 (Blocker):** `hasDuplicateKeys` bricht bei jedem Tokenfehler sofort ab, schaltet `json.Valid` vor und begrenzt die Tiefe auf 64 (→ 400). Test `RereviewHolds_DuplicateKeyWalkTerminates` läuft ohne Umgebungsvariable mit 3-s-Frist, dazu `HangingBodiesAnswer400` über die Route. Übrige Token-Schleifen im Repo geprüft: `configstore/validate.go` (Fehler beenden die Schleife, Tiefe/Knoten begrenzt) und `httpapi/server.go` (Schleife endet bei Fehler) sind sicher; die übrigen `json.NewDecoder`-Stellen (core, accounts, monitor) nutzen nur `Decode` ohne `More()`-Schleife.
- **L-01:** Schlüssel werden case-insensitiv (Go-Matching) auf Duplikate geprüft. **L-02:** Tabelle läuft vor und nach NFKC, U+03F2 wirkt, Kapitälchen und armenische Look-alikes ergänzt. **L-03:** `redis://:pw@`, Go-DSN erkannt; Strings unter verschachtelten Objekten unter Passwortschlüsseln abgelehnt, `pw`/`auth` ergänzt. **L-04:** Migrationsprüfung vergleicht auch Spalten, Typen, Längen, Sortierung, Nullbarkeit, Vorzeichen, Zeitgenauigkeit. **L-05:** fehlt `config_meta` bei vorhandenen Zeilen: `state_unreadable` statt neuem Schlüssel (BETRIEB/ADR).
- **Härtung:** AEAD-Kontext längenpräfixiert (injektiv), Memory-Repo rollt Verdrängung bei Quotenfehler zurück, die 8 ungültigen Mutationsproben sind aktualisiert.
- Gap-Tests sind umgedreht; absichtlich bleiben Gap-Tests für das dokumentierte Restrisiko (paarweiser Rollback/Wiedereinspielen) und die Rotation des Installationsschlüssels (alter Schlüssel dauerhaft nötig).
