# Handoff F2a – Webbuild und Go-Wrapper

Stand: 20.10.2026. Frontend & UX Engineer (`p1a_parity`), Hauptrepo,
Branch `feat/web-integration`, eigener Worktree `.worktrees/web-integration`.
Unabhängiger Review vor Integration ausstehend. API0.1.0 unverändert.

## Besitz und Ergebnis

Eigene Dateien: `internal/webui/**`, `scripts/build-web.py`,
`scripts/test-build-web.py`, `web-build.lock.json`, `docs/WEBBUILD.md`, dieser
Handoff sowie die vom Lead ergänzend freigegebene produktspezifische READMEkorrektur.
Keine Änderungen an `cmd`, HTTP-API, Core, Modulen, Dockerfile, Compose oder Swarm.
Backend-Agent und Architecture/Security wurden zu Schnittstelle und CSP konsultiert.

Der geprüfte Produktionsbuild ist als fünf versionierte Dateien im Hauptrepo
enthalten, mit Go `embed` eingebunden und über `webui.Wrap(api http.Handler)`
als eigenständiges Paket prüfbar. **Keine tatsächliche Mainverdrahtung und keine
Web→Go→DB- oder Containerproduktfreigabe** aus diesem Paket ableiten; das ist F2b.

## Quelle und Buildnachweise

Frontendquelle: öffentliches `MagicalWig34653/db-manager-web`, voller Commit
`4e5abee074d34853b266f20c344f28517d7924d1`. F1/F1b durch unabhängige QA
freigegeben; der Lead hat diesen Commit als finale Assetquelle bestätigt und per
Fast-Forward in Web-main integriert. Öffentliche Branch-/Raw-Abfrage des Pins
selbst ausgeführt. Beide erfolgreichen Frontend-/Dokumentations-CI-Läufe am Pin mit
`gh run view --json conclusion,headSha,status` selbst bestätigt (success/completed,
exakt derselbe Commit):
[Lauf 39491706244](https://github.com/MagicalWig34653/db-manager-web/actions/runs/39491706244),
[Lauf 39491706204](https://github.com/MagicalWig34653/db-manager-web/actions/runs/39491706204).

[Lock](../../web-build.lock.json): API-SHA
`4b598dc69a1904685e67a717997412654440ac10f47c17f7a025ccd114e294be`,
npm-Lock-SHA `bac4b32ba0548428186f20b1d6382f3554b1a477c0fb89957b123820495b15c6`,
Node26.5.0 / npm11.17.0, ausdrücklich Produktionsmodus.

`python3 scripts/build-web.py` zweimal aus frischem temporärem Git-Checkout
gegen den öffentlichen Pin ausgeführt: `npm ci`, Standalone-Driftcheck,
Produktionsbuild, exakte Original-Lizenzübernahme. Anschließend Manifestdateien
mit `cmp` verglichen: bytegleich. Artefakte: HTML424 Byte, CSS38550 Byte,
JS253502 Byte, AGPL34523 Byte und ursprüngliche Notices92896 Byte.

Die Installationsumgebung wird auf notwendige Pfad-/Home-/Temporärangaben reduziert;
private Git-/npm-Konfigurationen erhalten separate leere temporäre Dateien. Kein
`VITE_*`, Token oder sonstiges Sessionsecret wird an den Build weitergegeben.
Installationsmodus und Produktionsmodus sind getrennt. Nicht eingecheckte
Entwicklungsbuildversuche wurden nicht als Produktartefakte übernommen.
Kein Nachbarrepo, Quellcodearchiv, `node_modules`, Sourcemap oder privater Pfad
in den übernommenen Dateien. Manifest enthält keine lokale Uhrzeit oder Maschinenpfade.

## Verhalten und Prüfungen

Go-Tests prüfen unveränderte API-/Health-Requests und Status/Body/Header einschließlich
401/422/404, fehlende SPA-Maskierung, Root-/Asset-/Lizenz-MIMEs, schädliche/unkanonische
und percentcodierte Pfade, Verzeichnisauflistung, Methoden, SHA-ETag, HEAD und Range.
HTML/ungehashte Dateien sind `no-store`, gehashte Assets immutable.
Alle eingebetteten Bytes werden gegen Manifest/Größen geprüft; unbekannte, private
oder Entwicklungsdateien sind ausgeschlossen. HTTP-Handler schreibt keine Logs.

CSP: nur gleiche Origin für Scripts, keine dynamische Evaluation, Inline-Scripts oder
Scriptattribute; Layout-Stilattribute bleiben die mit Architecture/Security besprochene
enge Ausnahme. F1b-Produktionstest belegt den Client mit Dialogen, Query und ungültiger
APIantwort unter entsprechender CSP ohne Verletzung. Das tatsächliche Go-/Backend-
Browserprodukt folgt trotzdem separat in F2b.

Am 20.10.2026 lokal macOS/arm64, Go1.27.1:

| Prüfung | Tatsächlicher Ausgang |
| --- | --- |
| `python3 scripts/build-web.py` zweimal, `cmp` der Manifeste | identische fünf Produktionsartefakte und Provenienz |
| `python3 scripts/build-web.py --check` | fünf eingecheckte Artefakte/Quelle verifiziert, kein Node/Netzwerk erforderlich |
| `python3 -B scripts/test-build-web.py` | sechs Tests bestanden: offline Hashprüfung/Tamper, Allowlist/Maps, private Pfade, fehlende Notices, Symlinks, voller Commitpin |
| `go test -race ./internal/webui` | bestanden |
| `go vet ./internal/webui` | bestanden |
| `go test ./...` | isolierter Foundation-Gesamtstand mit neuem Paket bestanden; keine B1-Engine in diesem Worktree |
| `gofmt` / `git diff --check` | eigene Go-Dateien formatiert, keine Whitespacefehler |

Frischer isolierter Go-Checkout ohne Node/npm auf PATH, ohne Nachbarrepo und ohne
Modulnetzwerk erfolgreich geprüft; genauer Nachweis im folgenden Nachtrag.
`python3 scripts/check-docs.py`: 134 Dateien / 0 Fehler.

## Grenzen und nächster Schritt

Keine Änderung oder Abnahme des Servereinstiegs, der echten Backendengine oder
Compose-/Swarm-Verdrahtung. Kein tatsächlicher Web-Go-SQL-Browserlauf, kein Safari/
reales iPad, kein Mehrplattform-Reproduzierbarkeitsnachweis des npm-Builds. Diese
Grenzen sind keine fehlgeschlagenen Ergebnisse. Kein Quellcommit ohne vorherigen
unabhängigen Clientreview gepinnt. Bestehende Drittanbieterlizenzen bleiben erhalten.

Temporäre Quellcheckouts werden automatisch entfernt; kein eigener Produktserver,
Container, DB-Prozess oder Browserjob aus diesem Paket gestartet. Unintegrierte Arbeit
liegt auf diesem Workerbranch. Lead beauftragt QA, integriert nach Befundklärung;
Mainverdrahtung und echte Integrationsprüfung erst als F2b nach B1-/Besitzübergabe.

## Cleancheckout-Nachtrag

Implementierungscommit `42a0dec2ed7e10db1fc3c39457791a7560453009` mit
`git clone --quiet --no-hardlinks --single-branch --branch feat/web-integration`
in einen frischen temporären Checkout geklont. Kein Nachbarrepository, keine
npm-Installation, keine zuvor gebauten Assets außerhalb der eingecheckten Dateien.

Der Prüfpfad enthielt nur einen temporären Go-Link und Systemprogramme. Vor dem
Lauf wurde programmatisch bestätigt, dass weder `node` noch `npm` auf diesem PATH
auffindbar sind. Go lief mit `GOPROXY=off`, `GOSUMDB=off`, `GOTOOLCHAIN=local` und
`CGO_ENABLED=0`; damit kein Modulnetzwerk und keine impliziten Node-/C-Buildschritte.

- `go test ./...`: gesamter isolierter Foundationstand einschließlich echten
  eingebetteten Webdateien und Wrapperprüfungen bestanden.
- `/usr/bin/python3 scripts/build-web.py --check`: fünf Assets und volle Provenienz
  offline verifiziert; funktioniert auch mit dem System-Python ohne npm.
- `/usr/bin/python3 scripts/check-docs.py`: 134 Dateien / 0 Fehler.
- `git status --short`: leer; Tests verändern keine eingecheckten Dateien.
- Temporären Checkout und Toollinks automatisch entfernt. Keine Server-/Browserjobs
  gestartet oder zurückgelassen.

Dieser Nachtrag ändert nur den Handoff, keine Assets oder Produktfunktionen.
Finale Übergabe bleibt unabhängig zu reviewen. Echte Go-Engine-/Main-/Docker-
Integration folgt nach B1 als F2b; der saubere Paketbuild ersetzt diese Prüfung nicht.
