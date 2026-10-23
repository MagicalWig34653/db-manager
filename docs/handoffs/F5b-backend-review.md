# Unabhängiger F5b-Backend-/Platformreview

Stand: 23.10.2026. Reviewer ist kein F5b-Autor. Eigener Worktree
`.worktrees/f5b-backend-review`, Branch `review/f5b-backend`.
Produkt exakt `0dc9557c6a4d7ecf58aedc40987c64526aae416d`, Gesamtdiff ab
`1b02ea30447f6ff100ef00a24b7e174bb8618cd0`. Nachfolgender öffentlicher
Autorenhandoff `83dae46f5bead7d6c6141c195096af0e9337af16` ändert ausschließlich
Dokumentation und bestätigt die tatsächlichen Abschlussnachweise.

## Urteil

**Kein bestätigter Befund. F5b ist im geprüften begrenzten Umfang zur
Agentintegration durch die Projektleitung geeignet.** Keine Selbstintegration.
Das Urteil gilt ausschließlich dem unveränderten Produkt0dc9557 mit Webquelle
`8ef738d22a1164291e2c8183d80e72d1f043c0f8`; spätere Produktbytes benötigen eine
neue begründete Prüfung.

Keine vollständige Paritäts-, allgemeine Produktrelease-, HA-/Multinode-,
Powerloss-, Exactly-once-, native Geräte-/Signing- oder Vault-/Teamfreigabe.
SQLite bleibt im dokumentierten exklusiven lokalen Einzelinstanzbetrieb.

## Dateigrenze und reproduzierbare Herkunft

- Gesamtdiff und insbesondere Bytegleichheit der DB-/Core-/HTTP-/GoMod-/GoSum-/
  API-/Site-/zentralen Status-/Planbereiche geprüft: keine entsprechenden Änderungen.
  Produktänderung ausschließlich geprüfte Webassets/Manifest/Lock und ein
  Docker-Source-Label; dazu enge Build-/Runtime-/Deploymenttests und Dokumentation.
- Exakter Webpin8ef738d und unverändertes npm-Lockhash
  `bac4b32ba0548428186f20b1d6382f3554b1a477c0fb89957b123820495b15c6`.
  Vertragspins0.1–0.5 mit tatsächlichen kanonischen Vertragsbytes und den frisch
  bezogenen Webkopien übereinstimmend, einschließlich Struktur-SHA256
  `b5225339511d80fe4f52f09118fdad2060d8a92f31d44b73dd89fe9f76178c95`.
  Keine API0.6-Clientintegration aus dem neueren Backend abgeleitet.
- Zweimal unabhängig `python3 scripts/build-web.py` aus jeweils eigenem frischem
  öffentlichen Quellcheckout; nach jedem Lauf `git diff --exit-code --
  internal/webui/static internal/webui/assets-manifest.json` bestanden.
  Beide liefern dieselben fünf Originalartefakte und bytegleiches Manifest.
- Node26.5.0/npm11.17.0 exakt; Original-AGPL-/Drittanbietertexte gegen ursprüngliche
  Quellbytes geprüft. Gebauter Quelllink zeigt den tatsächlichen8ef738d-Commit,
  keine Sourcemaps/privaten Pfade im ausgelieferten Paket. Keine neue Abhängigkeit.
- Originale generierte JS-Zeile1 enthält bereits trailing whitespace und blieb
  entsprechend Sourcehash unverändert. Kein manuelles Säubern fremder Buildbytes.
  Die bekannte Vite-Chunkwarnung ist ein dokumentierter Größenhinweis, kein Testfehler.

## Unabhängige Prüfungen

```sh
go test -race -count=1 ./...
go vet ./...
python3 scripts/build-web.py --check
python3 scripts/test-build-web.py
python3 scripts/check-structure-contract.py
python3 scripts/check-create-contract.py
python3 scripts/test-web-integration.py
python3 scripts/test-deployment.py
```

Vollständiger Racelauf/Vet, Offline-Manifest mit fünf Assets, neun Pythonprüfungen
und eingefrorene/additive Vertragswächter bestanden. Keine neue umfassende
Vier-Treiber-/Native-App-Prüfung daraus behauptet.

### Eingebetteter CLI-/Compose-/Browserpfad

13 echte Chromiumfälle bestanden, 24,5 Sekunden, gegen das tatsächlich eingebettete
Go-/Composeprodukt mit SQLite und PostgreSQL verify-full. Kein Viteproxy.

- Tatsächliche Indexterme/Partial-/Expression-/DESC-/Include-/FK-/Check-/Coverage-
  Metadaten, SQLite-CHECK-Lücke/rowid-Hilfsterm und explizit zitierter bekannter
  leerer Bezeichner. Viewgrenze und verspätete echte Strukturantwort nach Tokenwechsel
  geprüft. Der klar benannte v5-HTTP404-Kompatibilitätsfall allein setzt eine
  synthetische Statusantwort; reguläre Daten stammen aus den echten Datenbanken.
- Tatsächliche JSON-/CSV-Downloadbytes und feste Dateinamen, geordnete doppelte Namen,
  Integer-/Decimalpräzision, NULL/Leer/Binary, beide expliziten CSVvarianten.
  Echte Steuerzeichen bleiben JSON-exakt und verhindern CSV vor Blob/URL.
  SQL-Trunkierung und aktueller Seitenscope/Offset/has_more korrekt; keine weitere
  DB-Anfrage pro Export. Gestarteter Download bleibt trotz anschließendem Cleanup
  intakt, keine SQL-/Token-/Credential-/Profilfelder im Export.
- Bestehende CRUD-/409-/Originalsnapshot-/PK-/Generated-/Default-/Doppelklickfälle,
  realer Commit vor Antwortverlust ohne Retry, separate DB-Bestätigung genau einer
  Zeile, Token-/Sessionwechsel, SQL-/Abbruch-/DB-Freigabe und F3-Pagingzustand grün.
  Source-/Originallizenzdialog und CSP ohne Verstöße.
- Eigene PostgreSQL-Bootstrapbarriere tatsächlich reproduziert Socket-only-Readiness
  und lehnt sie ab; danach endgültige TCP-Readiness. Positive TLS-Verbindung geht
  expliziter falscher-Hostname-Ablehnung voraus.

### Compose und eigener isolierter Swarm

Echter separater Deploymentlauf vollständig bestanden, jeweils Originalassets/
Notices, strikte CSP, Auth/Fähigkeiten/404 ohne HTMLmaskierung, Health, UID10001,
schreibgeschütztes Root, v4-INSERT/v3-UPDATE/DELETE/409 und typisierte Defaults/
NULL/Präzision/Binary. SQLitezeile nach Compose-Neustart und Swarm-Taskersatz exakt
wieder vorhanden.

V5-Struktur eines tatsächlich benannten partiellen DESC-Indexes wurde vor und nach
Neustart/Taskersatz vollständig verglichen; Index ist nicht bloß eine leere Liste.
PK/Coverage sind ebenfalls enthalten. Swarm auf eigenem Docker29.5.2-DinD-Daemon,
Secret/Placement/Routing Mesh geprüft; kein Host-Swarm oder Multinode-/HA-Nachweis.

### Frischer öffentlicher Rootclone ohne Node/npm/Geschwister

Zusätzlich eigener frischer öffentlicher Clone von exakt0dc9557, ohne benachbarte
Clientrepos. Nachweislich Node-/npm-freier eigener PATH:

- CLI mit `CGO_ENABLED=0 GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local` gebaut;
  vorhandener Host-Go-Modulcache verwendet, kein cachefreier Build behauptet.
- Dasselbe CLI unter diesem PATH gestartet und alle fünf HTTP-Assetbytes gegen die
  tatsächlichen Manifesthashes geprüft. Kein Host-Node/npm zur Laufzeit.
- Reguläres Dockerimage aus demselben Clone unter demselben Node-/npm-freien
  Host-PATH gebaut. Öffentliche gepinnte Go-Moduldownloads im Dockerbuilder erlaubt;
  kein netzloser Imagebau behauptet.
- Laufzeitimage ohne Node/npm; Originalroot-/Go-/Weblizenzen, Lock, Manifest und
  OCI-/Frontendquelllabels durch den tatsächlichen Produktprüfer verifiziert.
  Eigener Clone blieb sauber. Scratch/CLI/Token/individuelles Image entfernt.

## Exakte CI-Gates

Alle neun folgenden Läufe wurden eigenständig per GitHub-API als
`completed/success` am richtigen Produkt-SHA0dc9557 bestätigt:

Docs39910176520, Engine39910176605, Create39910176514, Insert39910176523,
Backend39910176639, Mutation39910176612, Structure39910176656,
Web39910176583 und Table39910176571.

## Ressourcen, öffentliche Daten und Fortsetzung

Vor eigenen Dockerläufen tatsächliches Autoren-Terminal-/Cleanupsignal abgewartet.
Eigene Browser-/PostgreSQL-Suite, danach genau ein eigener Deployment-/DinD-Lauf,
danach eigener Cleanimageproof sequenziell. Nur einzigartige eigene Tasknamen,
synthetische Credentials und private kurzlebige Dateien; keine echten DBdaten.

Alle eigenen Container/Volumen/Netze/Images/CLI-/Browserprozesse und Scratchs nach
Abschluss entfernt. Autoritative Abfrage: nur vier fremde laufende Container,
Host-Swarm `inactive`. Keine ursprünglichen TestFlightarchive berührt, keine
OS-/Keychainrechte geändert, kein globales Prune. Keine lokalen Prüfjobs offen.
Gitidentität unverändert; allein dieser unabhängige Review wird zusätzlich committed.

Nächster Schritt: konkrete Leadfreigabe für Agentintegration von Produkt0dc9557,
reinem Autorenhandoff83dae46 und diesem unabhängigen Review; anschließend passende
kombinierte Hauptchecks und tatsächliche Main-CI-Ausgänge dokumentieren.
