# Versionierter Webbuild und Go-Paket

Stand: 20.10.2026, F2a/F2b integriert, F3b-Prüfkandidat. Der unveränderte geprüfte Webbuild
ist im Hauptrepo enthalten. Der Servereinstieg umschließt jetzt die reale Go-API
mit `webui.Wrap`; CLI und Container liefern Oberfläche und API von derselben Origin.
Prüfstand und unabhängige Abnahme: [F3b-Handoff](handoffs/F3b.md).

## Quelle, Vertrag und Reproduzierbarkeit

[web-build.lock.json](../web-build.lock.json) pinnt das öffentliche Webrepository auf
Commit [6c6eb33](https://github.com/MagicalWig34653/db-manager-web/tree/6c6eb33c93eed355a10cd114f61ce27be7f3f55e).
F3 einschließlich atomarer Übernahme gültiger Tabellenentwürfe wurde unabhängig
geprüft; der Commit ist in der Web-main-Historie erreichbar. Vertrag `0.1.0`, API-SHA-256
`4b598dc69a1904685e67a717997412654440ac10f47c17f7a025ccd114e294be`;
Tabellenvertrag `0.2.0`, SHA256
`c15a848f01bb779429fd68a2b6eb6bba1575d3e07a4c68e6796cf7840a661838`;
zusätzlich npm-Lockfile-SHA und exakte Node-/npm-Versionen sind im Lock festgehalten.

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


## F3b-Laufzeitgrenzen

Die Tabellenansicht verwendet strukturierte read-only API0.2.0-Anfragen für Daten,
Metadaten, Sortierung und getaggte UND-Filter. API0.1.0 bleibt für freie SQL-Anfragen
unverändert; daraus folgt kein allgemeiner SQL-Lesemodus. Die eingebettete Quelle
enthält den unabhängig angenommenen Fix QA-F3-01: Abgewiesene Entwürfe verändern den
angewendeten Seiten-/Filter-/Sortierzustand nicht. Keine API0.3-Bindung oder zusätzliche
Schreibfreigabe. Endgültige Abnahme des neuen Root-Pins bleibt unabhängiger QA vorbehalten.

Die PostgreSQL-Fixture demonstriert mit privater Initialisierungsbarriere, dass der
Socket-only-Bootstrapserver noch keine TCP-Bereitschaft bedeutet. Erst nach expliziter
TCP-Readiness und erfolgreicher `verify-full`-Verbindung folgt die negative Hostnamen-
probe. Das härtet die Teststartgrenze von F2b; kein behaupteter Produktfehler.
