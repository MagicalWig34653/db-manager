# W1 – unabhängiger Site- und Deploymentreview

Stand: 23.10.2026. Eigenständiger Reviewer, kein W1-Autor. Worktree
`.worktrees/w1-review`, Branch `review/w1`. Geprüfte öffentliche Quelle:
`a972fb044454102cad8acb5a9b8a74e5a494a1be`. Keine Produktdatei geändert,
keine Pagesaktivierung, Veröffentlichung oder Mainintegration durchgeführt.

## Urteil zum Erstkandidaten

**Ein niedriger Darstellungsbefund QA-W1-01 bleibt offen.** Alle übrigen endlichen
Prüfungen ohne bestätigten Befund. Noch keine Gesamtfreigabe zur Veröffentlichung;
der gezielte Fix benötigt unabhängige Nachprüfung. Die exakte Pages-CI
[39907930954](https://github.com/MagicalWig34653/db-manager/actions/runs/39907930954)
ist inzwischen tatsächlich erfolgreich; ihr HEAD entspricht exakt dem Quellpin.

## Unabhängig ausgeführte Prüfungen

- Scopevergleich zum Rootmain `8708c3e`: ausschließlich neue Site, neuer
  Pagesworkflow und W1-Handoff. Bestehende Go-/DB-/HTTP-/API-/Docker-/Clientbytes
  unverändert. Kanonische Dokumente werden beim Build übernommen; keine zweite
  eingecheckte Dokumentationswahrheit.
- Frischer eigener Worktree: CPython3.13.14, Node26.5.0/npm11.17.0;
  `python3.13 -m venv site/.venv`, hashgesicherte binäre Installation aus
  `site/requirements.lock`, `npm ci --prefix site --ignore-scripts --no-audit
  --no-fund`. `site/check-dependencies.py` bestätigte22 installierte exakte
  Versionen und bytegleiche Originaltexte gegenüber den jeweiligen installierten
  Distributionen. `site/build.py`, `site/check.py` und `npm test --prefix site`
  bestanden:58 Seiten,1.727 lokale Verweise,38 Originalnotices und18 Browserfälle
  bei1440/1024/390px einschließlich WCAG-A/AA, Tastatur/Skiplink, Suche/XSS, CSP,
  fehlender Cookies/LocalStorage und null automatischer externer Requests.
- Anschließend eigener frischer öffentlicher Checkout von exakt diesem SHA in
  einem zufälligen temporären Verzeichnis ohne Geschwistercheckout: vollständige
  Neuinstallation der gepinnten Werkzeuge, Versions-/Originalnotice-/Build-/Link-
  prüfung erneut bestanden. Alle112 Ausgabedateien bytegleich zum Reviewbuild;
  ein zusätzlicher zweiter lokaler Build reproduzierte ebenfalls sämtliche112
  Dateien. Keine Behauptung pixelgleicher Browseraufnahmen auf anderen Plattformen.
- Eigene Suchindex-Gegenprobe über die echte Suchseite: ein manipulierter lokaler
  Index mit HTML in Titel/Auszug wurde ausschließlich als Text ausgegeben; ein
  fremder Zielhost wurde verworfen. Kein neu erzeugtes HTML-/Script-/SVG-/IMG-
  Element in den Suchtreffern. Diese Probe ergänzt die Benutzerquery-XSS-Probe.
- Desktop-Landingpage und echte Änderungsbestätigung visuell betrachtet; vier
  PNG-Produktaufnahmen mit fixem Root-F4b-Pin und Hashprovenienz. Sichtbare Inhalte
  sind synthetisch; kein Token-/Passwortdialog oder privater Dateipfad abgebildet.
  Tabletaufnahme ist korrekt als Tablet-Web statt nativer iPad-Nachweis bezeichnet.
- Workflow statisch geprüft: relevante Pushes/PRs führen nur Build aus;
  `workflow_dispatch` mit `publish=true` UND `refs/heads/main` ist für Deployment
  erforderlich. Token global nur `contents:read`, Deployjob allein
  `pages:write`/`id-token:write`, Environment `github-pages`, unveränderliche
  Actionpins und kein persistierter Checkouttoken. Siteinhalt behauptet nur
  HTML-Metapolicies, keine selbst kontrollierten GitHub-Pages-HTTPheader.
- Inhaltliche Grenzen geprüft: kein vollständiger Paritäts-, produktiver
  Credentialtresor-, Cloud-/Team-/HA- oder App-Store-Verfügbarkeitsnachweis behauptet.
  Direkte Repo-/Quellenlinks und separater Status für historische Nachweise.

## QA-W1-01 – globaler Horizontaloverflow auf schmalen Dokumentseiten

Schwere: niedrig. Eigene zusätzliche Chromiumprobe mit Viewport320×720px auf dem
unveränderten Kandidaten. Nach Navigation wird
`document.documentElement.scrollWidth` mit `window.innerWidth` verglichen:

| Pfad unter /db-manager/ | Viewport | Globale Scrollbreite |
| --- | --- | --- |
| Startseite | 320 | 320 |
| docs/PARITAET/ | 320 | 347 |
| docs/STATUS/ | 320 | 390 |
| docs/ARBEITSPLAN/ | 320 | 519 |
| docs/security/VAULT-FORMAT-V1/ | 320 | 327 |

Lange gewöhnliche Prosa-/Listenwörter mit Slash-/technischen Pfadbestandteilen
laufen außerhalb des Inhaltsbereichs. Beispiel im Arbeitsplan: das82 Zeichen
lange `Race/Vet-/Vier-Treiber-/TLS-/Lock-/Rechte-/Commit-/HTTP-/Vertrags-/Metaschemafälle`.
Die vorhandenen Umbruchregeln gelten für Überschriften/Links/Code, nicht allgemein
für diese Texte. Tabellen und Codeblöcke haben bereits eigene Scrollbereiche;
Ausblenden dieser Elemente in einer isolierten DOM-Gegenprobe beseitigt den globalen
Overflow nicht. Die18 Autorbrowserfälle prüfen sechs Einstiegsseiten bis390px und
belegen diese zusätzlichen langen Dokumentseiten bei320px nicht.

Erforderlich ist eine enge lesbare Umbruchkorrektur samt320px-Dokumentregression;
ein pauschales Verstecken/Abschneiden mit `body overflow-x:hidden` wäre keine Lösung.
Kein Datenverlust oder Sicherheitsleck wurde behauptet. Autor und Lead erhielten
konkrete Seiten/Breiten vor Abschluss dieses Erstreviews.

## Separater Integrationshinweis

Der noch separat ungeprüfte neue Dokumentationscommit `35c151c` benennt den
akzeptierten Statusabschnitt in `Abgenommene Inkremente und historische Nachweise`
um. Der geprüfte W1-Builder erwartet derzeit exakt `Abgenommen und integriert`.
Damit ist die Kombination beider ungeprüft zu integrierender Pakete noch nicht
buildkompatibel. Dies ist ein konkreter Integrationshinweis außerhalb des hier
festen Quellpins; Releaseengineer muss die abgestimmte kanonische Abschnittsquelle
und den vollständigen kombinierten Build prüfen. Kein zweiter Statusbestand und
keine erfundene Roadmap als Ersatz.

## Ressourcen und Fortsetzung

Eigene Browser und zufällige Loopbackserver wurden jeweils im finally-Block
beendet. Der frische öffentliche Checkout samt separaten Environments wurde
entfernt. Keine Docker-/Simulator-/Keychain-/Signing-/OS-Rechteänderung oder
fremde Ressourcenberührung. Das lokale Reviewenvironment enthält nur ignorierte
Werkzeuge/Ergebnisbilder; keine laufenden Prüfjobs verbleiben.

Nächster Schritt: Autor liefert unveränderlichen engen Fixpin, unabhängige320px-
Nachprüfung sowie aktualisierte exakte CI. Danach erst begrenzte Releasefreigabe;
nach späterem tatsächlichen Deploy sind Live-URL, Basisverzeichnis, Assets und Suche
separat zu prüfen.
