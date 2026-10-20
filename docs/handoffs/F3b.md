# F3b – geprüfter Tabellenclient im Go-/Containerprodukt

Stand: 20.10.2026. Frontend & UX Engineer, Branch `feat/table-web-runtime`,
Basis `7f3191a`. Prüfkandidat für unabhängige QA, keine eigene Release-/Integrationsfreigabe.

Ausschließlicher Webquellpin `6c6eb33c93eed355a10cd114f61ce27be7f3f55e`, nach geschlossenem
QA-F3-01 unabhängig freigegeben und öffentlich in Web-main-Ancestry erhalten.
SQL-Vertrag0.1.0 SHA256 `4b598dc69a1904685e67a717997412654440ac10f47c17f7a025ccd114e294be`,
zusätzlicher Tabellenvertrag0.2.0 SHA256
`c15a848f01bb779429fd68a2b6eb6bba1575d3e07a4c68e6796cf7840a661838`.
Keine Vertrags-/DB-/Core-/HTTP-/Go-Modul-/Webquelländerung und kein API0.3-Verbrauch.

## Umsetzung und bisherige Prüfungen

Lock/Buildskript prüfen beide unveränderten Vertragskopien und das unveränderte npm-Lockfile.
Zwei frische öffentliche Quellcheckouts mit Node26.5.0/npm11.17.0 und Produktionsbuild
lieferten identische fünf Originalartefakte und bytegleiches Manifest. HTML424 Byte,
CSS42254 Byte, JS383087 Byte, AGPL34523 Byte, Drittanbieterhinweise92896 Byte.
Originale Lizenztexte bleiben unverändert. Der Build prüft außerdem den tatsächlich
enthaltenen exakten Web-Quelllink; kein Umstempeln von Artefakten.

Docker-Provenienzlabel auf denselben Pin aktualisiert. Keine Wrapper-/Endpunktänderung
nötig. Bestehende CSP, Cache-/MIME-/Traversalregeln und Originaltextauslieferung bleiben.
Shared-Runtimehelfer prüft zusätzlich v2-Auth/Fähigkeiten und API404 ohne HTMLmaskierung.

Die PG-Fixture führt jetzt eine deterministische Bootstrap-Regression aus:
Privates Initialisierungsskript hält den socketbasierten Bootstrap an. Unix-Readiness
muss erfolgreich, TCP-Readiness erfolglos sein; erst nach Freigabe wird auf endgültiges
TCP gewartet. Positive verify-full-Verbindung muss vor negativer Hostnamenprobe
funktionieren, sowohl CLI als auch Compose. Das schließt eine Teststartgrenze des
F2b-Aufbaus, keinen nachgewiesenen Produktfehler.

Neue Root-Browserfälle übernehmen die abgenommene echte Tabellenregression aus dem
festen öffentlichen Webquellcommit, angepasst nur auf eigene Root-Fixtures/Compose-
Konfiguration. Einschließlich ungültiges Anwenden → Entwurf zurücknehmen → Weiter/
gezählte Refresh-Anfrage mit ausschließlich altem angewendetem Zustand, Struktur,
Sortierung/Filter/NULL/Präzision/Binär/View/fehlende stabile Ordnung. Bestehender SQL-
/Abbruch-/Sessionpfad bleibt, neuer Dialogfall prüft exakten Quelllink und Lizenzbytes.
Keine API-Mocks. Ressourcenprefixe `dbmanager-f3b-web-` und `dbmanager-f3b-deploy-`;
private kurzlebige Credentials, keine Logs/URLs/Prozessargumente mit Secrets.

Bisher bestanden: beide frischen Builds/Manifestvergleich,
`python3 scripts/build-web.py --check`, acht Python-Buildprüfungen (einschließlich fehlendem/falschem Tabellenvertrag-Pin),
`go test -race ./internal/webui`, `go vet ./internal/webui`.
`python3 scripts/test-web-integration.py` bestanden: fünf echte Chromiumfälle in
7,8 Sekunden gegen das tatsächlich eingebettete Go-Produkt, ohne Vite-/Mockproxy.
SQLite und PostgreSQL-TLS jeweils Tabellen-Paging-/Filter-/Strukturregression sowie
SQL/Begrenzung/Abbruch/Sessionpfad; separater sichtbarer Quell-/Lizenzdialog.
SQL-Abbruch wurde wie bisher erst bei nachgewiesener laufender DB-Arbeit ausgelöst,
danach tatsächliche DB-Freigabe geprüft. Null CSP-Verletzungen, private Credentials
nicht im Browserstorage. Ein erster Lauf hatte im angepassten PG-Test die falsche
Konfigurationsposition für das Passwort gelesen; behoben, vollständiger Lauf grün.

`python3 scripts/test-deployment.py` bestanden: Compose und isolierter Docker29.5.2-
DinD-Swarm mit allen Originalassets/-notices, strikter CSP, v1/v2-Auth/Fähigkeiten,
API404 ohne HTML, Healthcheck, UID10001/Read-only-Root und echten SQLite-Abfragen.
Daten überleben Compose-Neustart und Swarm-Taskersatz. Kein Host-Swarm verändert.
Nur eigene Testressourcen entfernt; keine laufenden eigenen Server/DB-Container.
Noch offen: Abschluss der öffentlichen CI und unabhängige QA. Linux-CI reproduziert zusätzlich den öffentlichen Quellbuild und
verlangt unveränderte eingecheckte Artefakte/Manifest.

Keine Tabellenmutation, keine Schreibfreigabe des separat behandelten SQLite-UPDATE-
Konfliktfalls, kein HA-/Mehrinstanz-SQLite, keine vollständige Parität. Freie SQL-Anfragen
behalten ihre vorhandenen Berechtigungen; Tabellen-Browse ist ausdrücklich read-only.
Zentrale Status-/Planungsdateien bleiben beim Lead. F3b liefert keine öffentliche
Deployment-/Distributionsfreigabe; AGPL-Quellangebot und Release-SBOM bleiben Betreiber-/Releasegates.


## Sauberer Checkout, letzter Browsernachweis und Übergabe

Öffentlicher Implementierungskandidat `639ba447edeeb92486013e78360de2f47f333531`,
Branch `feat/table-web-runtime`. Bestehende öffentliche Autoridentität unverändert.
Ein frischer öffentlicher Clone dieses Commits ohne Geschwistercheckouts baute das
vollständige CLI und Dockerimage mit einem Werkzeug-PATH, auf dem Node/npm nachweislich
fehlten. CLI: `CGO_ENABLED=0`, `GOPROXY=off`, `GOSUMDB=off`, `GOTOOLCHAIN=local`,
`go build -trimpath -o <temporary-binary> ./cmd/db-manager`. Image: regulärer gepinnter
Go-Moduldownload im Dockerfile, keine Behauptung eines vollständig netzlosen Imagebaus.
Im fertigen Laufzeitimage fehlen Node/npm ebenfalls. Offline-Asset-/Dualvertrags- und
Dokumentationsprüfung bestanden, Checkout blieb sauber; eigenes Image/Clone entfernt.

`go test -race -count=1 ./...`, `go vet ./...`, acht Python-Buildprüfungen und Dokuprüfung
bestanden. Nicht-Fixture-Go-Netzwerktests werden wie bisher ohne externe Fixturevariablen
übersprungen; die genannten echten Laufzeitproben werden dadurch nicht ersetzt.
Diffprüfung eigener Dateien bestanden. Einzige Ausnahme: Original-JS
`internal/webui/static/assets/index-BtD3D_KM.js`, Zeile1, enthält vom freigegebenen
Produktionsbuild stammenden End-Leerraum. Er bleibt unverändert, statt Originalartefakt/
SHA zu manipulieren; Sourcepin und beide Neubauten belegen dieselben Bytes.

Die abschließende enge Testergänzung misst CSP-Verletzungen zusätzlich ausdrücklich
während des Quellen-/Lizenzdialogs. Erneut `python3 scripts/test-web-integration.py`:
fünf echte Root-Embed-Fälle bestanden, 20,1 Sekunden, einschließlich dieses Dialogs
bei null CSP-Verletzungen und Focusreturn per Escape. Keine Produkt-/Assetänderung
gegenüber dem frisch gebauten Kandidaten. Keine eigenen Jobs/DB-Container/Netzwerke/
Volumes/Images offen, Host-Swarm unverändert. Keine Mutation oder API0.3-Verwendung.

Öffentliche CI des ersten Kandidaten ist gestartet; Dokumentation39502414538 erfolgreich,
Web39502414582 hat zusätzlich den öffentlichen Quellbuild unter Linux mit bytegleich
unveränderten eingecheckten Assets/Manifest erfolgreich reproduziert; sein echter
Runtime-Schritt und die weiteren regulären Workflows waren beim Zwischenstand noch
aktiv. Der finale Test-/Dokucommit startet erneut die regulären Branchprüfungen;
deren IDs werden im abschließenden reinen Handoffcheckpoint ergänzt. Keine eigene
Abnahme, unabhängige QA und Integration durch den Lead erforderlich.

## Öffentlicher Abschlusscheckpoint

Endgültiger Test-/Dokukandidat `7500ed2b5764894afe2bdee50e16da2c1f61892d`, Produkt-/Asset-
Implementierung `639ba447edeeb92486013e78360de2f47f333531`; Webquelle bleibt ausschließlich
`6c6eb33c93eed355a10cd114f61ce27be7f3f55e`. Keine neue UI-/API-Funktion.
Alle sechs öffentlichen Workflows des ersten Kandidaten sind inzwischen erfolgreich:
[Web39502414582](https://github.com/MagicalWig34653/db-manager/actions/runs/39502414582),
[Backend39502414522](https://github.com/MagicalWig34653/db-manager/actions/runs/39502414522),
[Engine39502414571](https://github.com/MagicalWig34653/db-manager/actions/runs/39502414571),
[Tabellen39502414542](https://github.com/MagicalWig34653/db-manager/actions/runs/39502414542),
[Einzelzeilen39502414532](https://github.com/MagicalWig34653/db-manager/actions/runs/39502414532),
[Doku39502414538](https://github.com/MagicalWig34653/db-manager/actions/runs/39502414538).
Der Webworkflow belegt damit sowohl Linux-Neubau mit bytegleichen freigegebenen
Originalassets als auch die fünf echten Root-Embed-Browserfälle.

Für den abschließenden zusätzlichen Dialog-CSP-Collector und die Prüfdokumentation
läuft erneut die öffentliche CI; bei Übergabe noch queued/aktiv, Beobachtung durch Lead:
[Web39502849674](https://github.com/MagicalWig34653/db-manager/actions/runs/39502849674),
[Backend39502849685](https://github.com/MagicalWig34653/db-manager/actions/runs/39502849685),
[Engine39502849585](https://github.com/MagicalWig34653/db-manager/actions/runs/39502849585),
[Tabellen39502849640](https://github.com/MagicalWig34653/db-manager/actions/runs/39502849640),
[Einzelzeilen39502849624](https://github.com/MagicalWig34653/db-manager/actions/runs/39502849624),
[Doku39502849655](https://github.com/MagicalWig34653/db-manager/actions/runs/39502849655).

Dieser reine Handoffcheckpoint vermeidet mit `[skip ci]` eine dritte identische Serie.
Doku-/Diffprüfung bestanden; Arbeitsbaum sauber, keine eigenen lokalen Jobs/Ressourcen,
Host-Swarm weiterhin `inactive`, fremde Dienste unverändert. Worker gibt den Slot frei.
Unabhängiger Reviewer wird durch den Lead zugewiesen; keine Selbstfreigabe/Integration.
