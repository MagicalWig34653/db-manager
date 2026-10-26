# Versionierter Webbuild und Go-Paket

Stand: 23.10.2026, F4b integriert, F5b-Prüfkandidat. Der unveränderte geprüfte Webbuild
ist im Hauptrepo enthalten. Der Servereinstieg umschließt jetzt die reale Go-API
mit `webui.Wrap`; CLI und Container liefern Oberfläche und API von derselben Origin.
Prüfstand und unabhängige Abnahme: [F5b-Handoff](handoffs/F5b.md).

## Quelle, Vertrag und Reproduzierbarkeit

[web-build.lock.json](../web-build.lock.json) pinnt das öffentliche Webrepository auf
Commit [c14271e](https://github.com/MagicalWig34653/db-manager-web/tree/c14271ec12bfb7940253787c2f84aeb65cc4252e)
(Pin Bündel 3, siehe Abschnitt am Ende). Der folgende Text beschreibt den **historischen** Pin
[8ef738d](https://github.com/MagicalWig34653/db-manager-web/tree/8ef738d22a1164291e2c8183d80e72d1f043c0f8):
F5 einschließlich lesender Index-/Constraintmetadaten, F6-Ergebnisexport und bisheriger
Einzelzeilenaktionen wurde unabhängig
geprüft; der Commit ist in der Web-main-Historie erreichbar. Vertrag `0.1.0`, API-SHA-256
`4b598dc69a1904685e67a717997412654440ac10f47c17f7a025ccd114e294be`;
Tabellenvertrag `0.2.0`, SHA256
`c15a848f01bb779429fd68a2b6eb6bba1575d3e07a4c68e6796cf7840a661838`;
zusätzlich npm-Lockfile-SHA und exakte Node-/npm-Versionen sind im Lock festgehalten.
Das Lock hat seit W-P04 `schema_version` 2 (siehe Abschnitt „Vertragsliste im Lock“).

Neubau mit Python mindestens 3.9, Git, Node26.5.0 und npm11.17.0:

```sh
python3 scripts/build-web.py
```

Das Skript lädt ausschließlich den vollen geprüften Git-Commit in einen frischen
temporären Checkout. Es verwendet keine Branchspitze, keinen Nachbarcheckout und
keinen bestehenden Frontendbuild. Es prüft Quellcommit, Vertragsbytes und npm-Lockfile,
führt `npm ci --no-audit --no-fund` und anschließend den Produktionsbuild aus.
Die Installationsphase installiert Entwicklungswerkzeuge; der eigentliche Build
setzt ausdrücklich `NODE_ENV=production`. Persönliche npm-/Git-Konfigurationen und
Token-/`VITE_*`-Umgebungsvariablen werden nicht an den Build weitergegeben.
Temporäre Konfigurationen und Quellcheckouts werden nach Abschluss entfernt.

Nur HTML, originaler AGPL-Text, originale Fremdlizenzhinweise und benannte gehashte
JS-/CSS-Artefakte werden nach `internal/webui/static/` übernommen. Keine Sourcemaps,
privaten Pfade, Symlinks, `.env`, `node_modules` oder Quellrepository-Kopie.
[assets-manifest.json](../internal/webui/assets-manifest.json) erfasst Quelle, Toolchain,
Dateipfad, Bytezahl und SHA-256 ohne Zeitstempel. Zwei frische Produktionsneubauten
lieferten lokal identische fünf Artefakte und ein bytegleiches Manifest. Das belegt
den geprüften lokalen Buildpfad, keine allgemeine Mehrplattformreproduzierbarkeit.

Offlineprüfung der eingecheckten Dateien ohne Node, Git-Fetch oder Netzwerk:

```sh
python3 scripts/build-web.py --check
python3 -B scripts/test-build-web.py
go test -race ./internal/webui
go vet ./internal/webui
```

Ein sauberer Hauptcheckout kann das Paket allein mit Go bauen. Node und npm sind
nur zur bewussten Erneuerung der eingecheckten Artefakte erforderlich. Sourcepin-
Änderungen benötigen erneut unabhängigen Clientreview, passenden Vertragsstand,
Neubau und Review der neuen Provenienz-/Hashänderungen.

## HTTP-Vertrag des Pakets

`webui.Wrap(api http.Handler) http.Handler` aus `internal/webui`:

- `/api`, alle `/api/`-Unterpfade und `/healthz` einschließlich Unterpfaden werden
  mit unverändertem Request an den übergebenen Handler weitergereicht. Dessen
  Status, Body und Header bleiben erhalten; API-401/422/404 werden kein HTML.
- `/` und `/index.html` liefern die eingebettete deutsche Oberfläche; `/assets/…`
  nur bekannte JS-/CSS-Dateien. Lizenztexte sind unter `/LICENSE.txt` und
  `/THIRD-PARTY-NOTICES.txt` verfügbar. Keine Verzeichnisauflistung oder SPA-Fallbacks;
  unbekannte Pfade und Traversalversuche ergeben 404.
- Statische Ressourcen erlauben GET/HEAD; andere Methoden erhalten 405. MIME-Typen
  sind festgelegt, `nosniff` gesetzt. HTML und ungehashte Dateien sind `no-store`;
  gehashte Assets erhalten `public, max-age=31536000, immutable`. SHA-ETags, HEAD,
  bedingte Anfragen und Bytebereiche werden geprüft.
- Die CSP erlaubt Scripts nur von derselben Origin, ohne `unsafe-eval`, Inline-Scripts
  oder Scriptattribute. Layout-Stilattribute bleiben gezielt erlaubt; der Architecture/
  Security-Engineer hat diese begrenzte Ausnahme bewertet. Keine fremden Fonts oder
  externen Netzwerkdienste sind für die Oberfläche nötig.

Der Client benötigt die gleiche Origin für UI und API. Für entfernte Instanzen ist
HTTPS erforderlich; keine CORS-Aufweichung. API-Authentifizierung, Netzwerkzielregeln,
DB-Credentials und SQLite-Sandbox verbleiben im Backend, nicht im statischen Handler.
Der Wrapper schreibt keine Request-/Credentiallogs und legt keine Browserpersistenz an.

Technische Primärquellen am 20.10.2026 gelesen:
[Go embed](https://pkg.go.dev/embed), [Go ServeContent](https://pkg.go.dev/net/http#ServeContent),
[CSP3](https://www.w3.org/TR/CSP3/). Tatsächliche Prüfungen und noch offene Integrations-
und Gerätefälle stehen im [F2a-Handoff](handoffs/F2a.md).


## Laufzeitimage und Quellherkunft

Das Dockerfile baut ausschließlich aus diesem Hauptcheckout. Es benötigt weder Node
noch ein Web-Nachbarrepository. Vor dem Go-Build prüft es das eingebettete Paket samt
Manifest. Das Laufzeitimage enthält keinen Node-/npm-Prozess. Unter
`/usr/share/db-manager/` liegen `web-build.lock.json` und `web-assets-manifest.json`;
unter `/usr/share/licenses/db-manager/` der originale AGPL-Text und unter `go/` die
unveränderten vorhandenen Go-Lizenzhinweise. Die originalen Webhinweise sind weiterhin
über `/LICENSE.txt` und `/THIRD-PARTY-NOTICES.txt` erreichbar.

OCI-Labels referenzieren das öffentliche
[Hauptrepository](https://github.com/MagicalWig34653/db-manager), dessen Betriebsdoku
und den exakten öffentlichen Frontend-Quellcommit. Bei öffentlicher Netzbereitstellung
muss das passende vollständige Quellangebot einschließlich Änderungen sichergestellt
werden; dieses Entwicklungsinkrement ist keine öffentliche Deployment-/Releasefreigabe.
Der sichtbare Dialog „Quellcode & Lizenzen“ verweist auf exakt den gebauten Webcommit,
das Go-Serverprojekt, dessen Buildherkunft und die lokal gelieferten Originaltexte.
Die Artefakte werden unverändert aus dem gepinnten Quellbuild übernommen.


## F5b-Laufzeitgrenzen

Die Tabellenansicht verwendet strukturierte read-only API0.2.0-Anfragen für Daten,
Metadaten, Sortierung und getaggte UND-Filter. API0.1.0 bleibt für freie SQL-Anfragen
unverändert; daraus folgt kein allgemeiner SQL-Lesemodus. Die eingebettete Quelle
enthält den unabhängig angenommenen Fix QA-F3-01: Abgewiesene Entwürfe verändern den
angewendeten Seiten-/Filter-/Sortierzustand nicht. Neue Einzelzeilenaktionen nutzen
API0.3/0.4 und verlangen gültige v4-Fähigkeiten sowie frische v3-Eignung. PK/generated
bleiben gesperrt; vollständige Originalsnapshots und explizite Bestätigung schützen
den Ablauf. Ältere Backends behalten SQL/v2 ohne neue Schreibaktionen. Endgültige Abnahme des neuen Root-Pins bleibt unabhängiger QA vorbehalten.

Die PostgreSQL-Fixture demonstriert mit privater Initialisierungsbarriere, dass der
Socket-only-Bootstrapserver noch keine TCP-Bereitschaft bedeutet. Erst nach expliziter
TCP-Readiness und erfolgreicher `verify-full`-Verbindung folgt die negative Hostnamen-
probe. Das härtet die Teststartgrenze von F2b; kein behaupteter Produktfehler.

## Zusätzliche Schreibvertragspins

`web-build.lock.json` und Manifest pinnen zusätzlich den Update-/Deletevertrag 0.3.0
(SHA256 `ec77c9739cda11df3f4cb9fe756a632e67840fdc0f6190deee1aa1d1333dc308`)
und Insertvertrag 0.4.0
(`4763ebf8632a9ef977cf7d3a6037299fc4dafde0a581fe10cb34d844f8356fbf`).
Build und Offlineprüfung vergleichen alle fünf Rootverträge; Neubau vergleicht außerdem
alle fünf unveränderten Webvertragskopien. Zusätzlich wird der freigegebene lesende Strukturvertrag 0.5.0 mit SHA256
`b5225339511d80fe4f52f09118fdad2060d8a92f31d44b73dd89fe9f76178c95` gepinnt.
Historisch (Pin 8ef738d): API0.6 war im Backend vorhanden, wurde von dieser Webquelle nicht konsumiert; seit Pin 48c2f67 sind 0.6 und 0.7 gepinnt.
Die Quellen des eingefrorenen F4-Clients referenzieren Backend72855cf mit beiden
unabhängig geschlossenen v3-Schutzfixes. Vertragsgleichheit allein belegt keinen
gepatchten Serverstand. Keine Änderung an freigegebenen Produktbytes nach dem Pin.


## Strukturmetadaten und lokaler Ergebnisdownload

Die Strukturansicht ergänzt API0.5-Index-/Constraintangaben mit katalogbelegten
Eigenschaften, geordneten Termen und expliziter Abdeckung. Unbekannte Eigenschaften
bleiben unbekannt; SQLite-CHECK-Angaben werden ohne Parser nicht erfunden. Bekannte
leere Bezeichner werden eindeutig JSON-zitiert. Alte Backends behalten die v2-
Spaltenstruktur. Die Metadaten sind keine atomare Zusage über spätere Schemaänderungen.

Ergebnisdownload geschieht ausschließlich nach Benutzerklick aus dem geladenen
Snapshot, ohne zusätzliche DB-Abfrage. JSON erhält getaggte Werte/NULL/Leer/Binärdaten,
Präzision, Spaltenreihenfolge und doppelte Namen. SQL-Ergebnis bzw. geladene Tabellenseite,
Offset/Seitengröße/has_more und Trunkierung bleiben ausdrücklich gekennzeichnet;
kein vollständiger Tabellendump. Höchstens 8 MiB endgültige UTF-8-Datei.

CSV bietet getrennte Rohwerte und bewusst mit Apostroph präfixierte Werte für
Tabellenkalkulationen; weder Typ-/Quote-/NULL-Roundtrip noch universelle Excel-
Sicherheit wird zugesagt. UTF-8 ohne BOM, Komma/CRLF, Header/Nicht-NULL gequotet,
NULL als ungequotetes `\N`. Literaltext `\N` bleibt gequotet. C0 außer CR/LF sowie DEL/C1
werden vor Blob/URL/Download ausdrücklich abgewiesen; JSON erhält sie unverändert.
Quelle-/Token-/Resultwechsel räumt Downloads auf; gestartete Downloads bleiben intakt.


## Vertragsliste im Lock (Schema 2, W-P04)

`web-build.lock.json` pinnt API-Verträge nicht mehr über feste Schlüsselnamen, sondern über
`contracts: [{family, version, sha256}]`. Aktuelle Einträge (unverändert zu den bisherigen Pins):

| family | version | SHA-256 |
| --- | --- | --- |
| api | 0.1.0 | `4b598dc69a1904685e67a717997412654440ac10f47c17f7a025ccd114e294be` |
| table | 0.2.0 | `c15a848f01bb779429fd68a2b6eb6bba1575d3e07a4c68e6796cf7840a661838` |
| mutation | 0.3.0 | `ec77c9739cda11df3f4cb9fe756a632e67840fdc0f6190deee1aa1d1333dc308` |
| insert | 0.4.0 | `4763ebf8632a9ef977cf7d3a6037299fc4dafde0a581fe10cb34d844f8356fbf` |
| structure | 0.5.0 | `b5225339511d80fe4f52f09118fdad2060d8a92f31d44b73dd89fe9f76178c95` |

Regeln, die `scripts/build-web.py` hart prüft (Fehler, kein Warnhinweis): Liste vorhanden und nicht
leer; jeder Eintrag hat genau `family`, `version` (`0.N.0`) und `sha256`; keine doppelte Familie, keine
doppelte Version; aufsteigende Versionsreihenfolge; die fünf Bestandsverträge sind immer enthalten;
jede SHA-256 muss mit `contracts/api-v<version>.json` im Root übereinstimmen. Beim Neubau muss
außerdem die Kopie im Webcheckout (`src/lib/contract/api-v0.1.0.json` bzw.
`src/lib/contract/v<N>/api-v0.<N>.0.json`) bytegleich sein. Unbekannte Lock-Schlüssel und die
alten flachen Schlüssel (`contract_sha256`, `table_contract_*` usw.) werden abgewiesen.

Historisch (Schema-2-Einführung, Pin 8ef738d): API 0.6 blieb ungepinnt, weil der Webstand ihn nicht konsumierte; seit Bündel 2 ist er gepinnt. Ein neuer
Vertrag wird beim nächsten Pin als weiterer Listeneintrag ergänzt (Familie frei wählbar, z. B.
`create`); `assets-manifest.json` zeigt solche Einträge unter `source.additional_contracts` und
wechselt dann `source.schema_version` auf 2. Solange nur die fünf Bestandsverträge gepinnt sind,
bleibt die Manifest-`source` im bisherigen flachen Format, die eingebetteten Assets und das
Manifest sind byte-identisch zum Pin `8ef738d`. Ein Vertragswechsel erfordert weiterhin
Neubau und unabhängiges Review; die Bytevergleiche der fünf Originalassets bleiben verbindlich.


## PWA-Dateien (W-P03)

`internal/webui` liefert zusätzlich eine exakte Namensliste (keine Wildcards): `manifest.webmanifest`
(`application/manifest+json`), `sw.js` (`text/javascript`), `icon.svg` (`image/svg+xml`) sowie
`icon-192.png`, `icon-512.png`, `icon-maskable-512.png`, `apple-touch-icon.png`, `favicon-32.png`
(`image/png`). Alle sind `no-store`; kein `Service-Worker-Allowed`. Die CSP ergänzt nur
`manifest-src 'self'` und `worker-src 'self'`. `scripts/build-web.py` kennt die Menge als
`PWA_FILES`: ein Build enthält alle acht Dateien oder keine (der bisherige Pin `8ef738d` enthält
keine und bleibt gültig). Der Neubau mit PWA-Dateien erfolgt erst mit dem nächsten Pin im
gemeinsamen Bündel; Details: [W-P03-Handoff](handoffs/W-P03.md).

## Pin Bündel 2 (24.10.2026, ersetzt Bündel 1)

Das Lock pinnt Web-main `48c2f67113d8fd0a27e9cf966d20904fd7d0da61` (Shell, PWA mit Probe-Body-Fix, Tabelle anlegen,
Katalog/Übersicht). Neu in der Vertragsliste: `create` 0.6.0 (`b08b6aba...6887`) und `overview` 0.7.0
(`c6fd8c91...4d17`); die Webkopien (`src/lib/contract/v6/`, `v7/`) sind byte-identisch zu den Rootverträgen. Damit
erscheint im Manifest `source.schema_version` 2 mit `additional_contracts`. Der Webbuild ist code-gesplittet
(`assets/feature-*.js/css`); die Neubauprüfung (`verify_build` in `build-web.py`) ersetzt die feste Dateizahl durch drei Prüfungen:
(1) `index.html` hat genau ein Entry-Modulskript und ein Entry-Stylesheet, der Quelllink des gepinnten Commits steht im
Entry-Skript; (2) jede Datei unter `assets/` ist transitiv vom Entry referenziert, höchstens `MAX_CHUNKS` (24; bei Bündel 3 von 12 begründet angehoben, siehe PIN-BUNDLE-3) Dateien;
(3) die `SHELL`-Liste in `sw.js` entspricht exakt `index.html` + PWA-Dateien (ohne `sw.js`) + `assets/*`. Der alte Pin
8ef738d besteht diese Prüfungen unverändert und bleibt bytegleich rekonstruierbar. Das Dockerfile-Label `org.dbmanager.frontend.source` wird bei Pinwechsel von Hand gepflegt;
`scripts/test-build-web.py` und `verify_image` schlagen bei Abweichung zum Lock fehl. Details: [PIN-BUNDLE-2](handoffs/PIN-BUNDLE-2.md).

## Pin Bündel 3 (25.10.2026)

Das Lock pinnt Web-main `c14271ec12bfb7940253787c2f84aeb65cc4252e` (zusätzlich zu Bündel 2: Objektoperationen, Spalten, Export,
Navigationssperre). Neue Vertragseinträge: `object-ops` 0.8.0 (`74aa39ed...8264`), `columns` 0.9.0 (`508b9951...e3f8`),
`export` 0.11.0 (`b3bcc38b...6620`), Webkopien v8, v9, v11 byte-identisch zu den Rootverträgen. 0.10 (maintenance) und 0.12
(sqlbatch) liegen nur im Root, das Web enthält keine Kopie: **nicht gepinnt**. Details: [PIN-BUNDLE-3](handoffs/PIN-BUNDLE-3.md).
