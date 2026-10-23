# W1 – unabhängiger Site- und Deploymentreview

Stand: 23.10.2026. Eigenständiger Reviewer, kein W1-Autor. Worktree
`.worktrees/w1-review`, Branch `review/w1`. Geprüfte öffentliche Quelle:
`a972fb044454102cad8acb5a9b8a74e5a494a1be`. Keine Produktdatei geändert,
keine Pagesaktivierung, Veröffentlichung oder Mainintegration durchgeführt.

## Urteil zum Erstkandidaten

**Zum Erstkandidaten blieb QA-W1-01 offen; der Finalnachreview unten schließt ihn.** Alle übrigen endlichen
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


## Finalnachreview – 23.10.2026

**QA-W1-01 einschließlich des zusätzlichen fontabhängigen Navigationsumbruchs ist
unabhängig geschlossen. W1 ist im geprüften begrenzten Umfang zur koordinierten
Integration und Veröffentlichung freigegeben.** Unveränderliche Produktquelle:
`08021c184c0f515a0390a955af799624bf8f3dd6`. Der reine Handoffnachtrag
`07b6b9c448775815e2672e564459ee155703599b` ändert danach ausschließlich W1-Dokumentation;
keine weitere Site-, Asset- oder Workflowänderung folgt aus dieser Freigabe.

Der ursprüngliche Reviewcommit `86b460b` bleibt in der Review-Ancestry erhalten.
Die eingefrorenen Fixes und der tatsächliche Root-Dokumentations-/B4-Stand wurden
in den eigenen Reviewbranch übernommen; Sitequellen und kanonische STATUS-/Plan-/
TEAM-Quellen entsprechen dem freigegebenen Pin. Der Reviewbranch enthält zusätzlich
nur diese eigene Nachweisdokumentation.

Gezielte unabhängige Ergebnisse:

- CSS-Diff kontrolliert: gewöhnliche Contentprosa darf umbrechen; Tabellen und
  Codeblöcke behalten ihre lokalen Scrollbereiche. Die Headernavigation verwendet
  flex-wrap. Keine body-/main-hidden- oder Clippinglösung. Geometrische und
  visuelle Prüfung bei320px auf Startseite, PARITAET, STATUS, ARBEITSPLAN und
  VAULT-FORMAT-V1: globale Breite jeweils320/320.
- Zusätzliche eigene Schrift-/Spacingprobe: Arial mit17px in der Navigation,
  0,12em Zeichenabstand, 0,16em Wortabstand und1,5-facher Zeilenhöhe. Auf denselben
  fünf Seiten bleiben alle vier Hauptnavigationslinks horizontal und vertikal im
  Header; keine verdeckte Zeile oder unzugängliche Navigation. Die320px-
  Statusansicht wurde zusätzlich visuell angesehen.
- `site/tests/build_sources.py` besteht mit vier Fällen einschließlich fehlender
  und mehrdeutiger Quelltabellen. Darüber hinaus eigener direkter Aufruf des
  tatsächlichen Builders mit den unveränderten Git-STATUS-Quellen8708c3e,
  35c151c, e554bb0 und1b02ea3: alle vier liefern die jeweilige kanonische
  Ergebnistabelle. Die früher festgestellte Kombination mit konsolidierten
  Überschriften ist damit geprüft; keine Root-Dokumentänderung als Ausweichlösung.
- Versions-/Originalnoticesprüfung unverändert grün; Build und vollständiger
  Link-/Policycheck mit aktuellem Root-Dokumentationsbestand bestanden. Der eigene
  Reviewbuild rendert wegen des zusätzlichen eigenen Reviewdokuments63 statt
  62 Seiten; das ist keine Produktänderung oder neue Quellbehauptung.
- Die vorhandene Browserprüfung wurde am integrierten Reviewstand bestanden:
  vier320px-Langdokumentregressionen, verbreiterte Navigation und sämtliche18
  bisherigen Desktop-/Tablet-/Mobil-, WCAG-A/AA-, Tastatur-, Such-/XSS-, CSP-,
  Storage- und Null-Externe-Requestfälle. Originalscreenshots, Quellprovenienz,
  Locks,38 Originalnotices, Such-JavaScript und Theme sind zum Erstpin bytegleich.
  Keine zusätzliche breite Backend- oder Datenbanksuite durchgeführt.
- Workflowdiff beschränkt sich auf volle Gitgeschichte für die realen
  Quellprofiltests und deren Buildaufruf. Die Veröffentlichungsbeschränkung bleibt
  unverändert: ausschließlich manueller main-Dispatch mit publish=true, minimale
  Deployrechte; kein PR-/Push-Deployment.
- Alle zehn exakten CIs am Produktpin08021c1 eigenständig als completed/success
  bestätigt, insbesondere
  [Pages39910428081](https://github.com/MagicalWig34653/db-manager/actions/runs/39910428081),
  [Docs39910427901](https://github.com/MagicalWig34653/db-manager/actions/runs/39910427901),
  [Backend39910427918](https://github.com/MagicalWig34653/db-manager/actions/runs/39910427918)
  und [Web39910427907](https://github.com/MagicalWig34653/db-manager/actions/runs/39910427907).
  Die frühere fehlgeschlagene Linuxserie am Zwischenfix9004 bleibt fehlgeschlagen
  dokumentiert; sie wurde nicht in einen Erfolg umgedeutet.

Keine weiteren bestätigten Findings im begrenzten Sitepaket. Keine Pages-/Repo-
Einstellung, Domain, Mainintegration oder Veröffentlichung durch den Reviewer.
Eigene Browser-/HTTPprozesse beendet, keine lokalen Prüfjobs offen. Releaseagent
muss den tatsächlich integrierten Gesamtstand bauen und nach autorisiertem Deploy
Live-URL, Assets, Basisverzeichnis und Suche separat prüfen; dies ist kein schon
vorweggenommener Live-Nachweis.
