# W1 – statischer Projektauftritt

Eigenes MkDocs-Theme, deutsche Landingpage, lokal durchsuchbare kanonische
Dokumentation und Roadmap. Basis `/db-manager/`, keine Produkt-API, Cookies,
externen Fonts oder Tracker. Name bleibt DB Manager.

## Reproduzieren

Buildtoolchain: CPython **3.13.14**, Node **26.12.1**, npm-Lockformat3.
MkDocs **1.6.1** und vollständiger transitiver Pythonhashlock; npm ist ausschließlich
für Browser-/Accessibilitytests erforderlich. Kein Einfluss auf Go-Build oder Runtime.

```sh
python3.13 -m venv site/.venv
site/.venv/bin/python -m pip install --only-binary=:all: --require-hashes -r site/requirements.lock
site/.venv/bin/python site/tests/build_sources.py
site/.venv/bin/python site/build.py
site/.venv/bin/python site/check.py
npm ci --prefix site --ignore-scripts --no-audit --no-fund
site/.venv/bin/python site/check-dependencies.py
cd site
npx playwright install chromium
npm test
```

Linux-CI installiert mit `playwright install --with-deps chromium` die notwendigen
Browser-Systembibliotheken. Das ist kein Auftrag, lokale Systemrechte zu verändern.
Der Browsertest startet ausschließlich seinen eigenen zufälligen Loopbacklistener
und beendet ihn im `finally`-Block. Vorschau aus dem Rootcheckout:

```sh
site/.venv/bin/python -m http.server 8765 --bind 127.0.0.1 --directory site/dist
```

Dann `http://127.0.0.1:8765/db-manager/` öffnen; eigenen Server danach beenden.

## Quellen und Grenzen

`build.py` kopiert Root-README, Einrichtung, `docs/**/*.md` und `contracts/*.md`
nur in ein ignoriertes temporäres Stagingverzeichnis. `pages/*.md.in` sind
Websitevorlagen mit Links auf diesen generierten Dokumentbaum; die eigenen
Linkprüfungen validieren ihre tatsächlich gerenderten Ziele. MkDocs/Python-Markdown
rendern diese Quellen. Repositorylinks werden an die gerenderte Pfadstruktur
angepasst; Code-/JSON-/andere Quelldateien verweisen auf den exakten Gitcommit.
Kanonische Dokumente erhalten einen Quellenlink. Kein zweiter eingecheckter
Dokumentationsbestand. Die Roadmap zieht ihre Ergebnistabelle aus `docs/STATUS.md`.
Arbeitsplan und Paritätsmatrix bleiben die maßgeblichen Detailquellen; keine
erfundenen Termine, Prozentstände oder globale Produktfreigaben. Die Ergebnistabelle
wird anhand ihrer eindeutigen kanonischen Spalten Paket/Ergebnis/Nachweis gelesen,
nicht anhand einer festen Abschnittsüberschrift. Fehlende oder mehrdeutige Tabellen
brechen den Build verständlich ab. Regressionen prüfen die echten historischen
Statusquellen an zwei unveränderlichen Gitcommits sowie aktuellen STATUS/Plan/TEAM.
Dafür muss der Checkout deren öffentliche Gitgeschichte enthalten (CI: fetch-depth0).

Die Suche lädt nur den eigenen statischen JSONindex und schreibt Treffer mit
DOM-Textknoten. MkDocs-Suchruntime/Lunr/Worker werden nicht ausgeliefert.
Originalnotices und Inventare sämtlicher 17 Python- und fünf npm-Werkzeuge
stehen in `licenses/`; eigene Assets und Websitecode sind AGPL-3.0-only.
Siehe [LIZENZEN](LIZENZEN.md). Chromium wird nicht mit der Website verteilt.

## Echte Produktaufnahmen

`assets/screenshots/` zeigt den tatsächlichen eingebetteten Webclient des
öffentlichen Rootcommits `8aab52bc9be8ccaf84dbdb0eda61ec46a60181d3` (F4b).
Desktop1440×1000 und Tablet-Web1024×900. Die synthetische SQLitefixture ist
vollständig in `capture.py` definiert; keine produktiven Daten und keine native
Apple-UI. Die Aufnahmeversion behauptet weder neuere F6-Exportfunktionen noch
eine native iPad-Freigabe. SHA-256 und Quellenpin: `assets/screenshots/provenance.json`.

```sh
site/.venv/bin/python site/capture.py
```

Voraussetzung: gepinnter Rootcommit im lokalen Gitobjektspeicher, Go gemäß
Roottoolchain und installierter Testbrowser. Das Skript archiviert ausschließlich
diesen Commit, baut darin das Go-Produkt, erzeugt einen zufälligen kurzlebigen
Token mit privater Datei und startet einen eigenen Loopbackserver. Es setzt den
Token im echten UI, schließt den Authdialog und fotografiert Tabellenansicht,
SQLergebnis, Bestätigung einer noch nicht ausgeführten Änderung und Tabletbreite.
Danach werden ausschließlich eigener Prozess und Wegwerfverzeichnis entfernt.
Keine Docker-, Simulator-, Screenrecording- oder OS-Berechtigungsänderung.
Browserpixel können sich zwischen Plattformen unterscheiden; die eingecheckten
Originalbilder bleiben über ihre Hashes nachvollziehbar.

## Veröffentlichung nach unabhängiger Abnahme

`.github/workflows/pages.yml` prüft PRs und relevante Branchänderungen. Kein Deploy bei PR
oder Push. Veröffentlichung erfolgt ausschließlich über manuelles
`workflow_dispatch` an `main` mit ausdrücklich gesetztem `publish=true`, nach
Peerreview und koordinierter Releasefreigabe. Nur der Deployjob erhält
`pages:write`/`id-token:write`; Umgebung `github-pages`, gepinnte Actions.
Pages muss danach separat auf GitHub-Actions-Publishing eingestellt werden.
W1 aktiviert weder Pages noch eine Domain und führt diesen Schritt nicht aus.
GitHub Pages HTTPheader sind Hostingverhalten; der Build setzt nur die in
LIZENZEN dokumentierten HTML-Metapolicies. Nach Release ist die tatsächliche
Live-URL einschließlich Basepath/Assets/Suche separat zu prüfen.

Primärquellen für den gewählten Standardbuild und die Deploymentgrenzen:
[MkDocs](https://www.mkdocs.org/),
[Konfiguration](https://www.mkdocs.org/user-guide/configuration/),
[GitHub-Pages-Workflows](https://docs.github.com/en/pages/getting-started-with-github-pages/using-custom-workflows-with-github-pages),
[Publishingquelle](https://docs.github.com/en/pages/getting-started-with-github-pages/configuring-a-publishing-source-for-your-github-pages-site).
