# Unabhängiger F3b-Review – Backend & Platform

Stand: 20.10.2026. Reviewer ist nicht F3b-Autor. Eigener Worktree
`.worktrees/f3b-backend-review`, Branch `review/f3b-backend`.
Produktkandidat `639ba447edeeb92486013e78360de2f47f333531`, Test-/Dokustand
`7500ed2b5764894afe2bdee50e16da2c1f61892d`. Der letztere verändert gegenüber639
keine Produkt-/Asset-/Modul-/Docker-/Vertrags-/Buildskriptbytes; nur zusätzlicher
Dialog-CSP-Collector und Dokumentation/Testinventar. Autorencheckpoint `3f7a863`
ändert danach nur den Handoff. Keine Produktquelle durch Reviewer geändert.

## Urteil

**Kein bestätigter Befund. F3b im geprüften begrenzten Umfang zur Leadintegration
geeignet.** Kein Review von 0.3-/0.4-Schreiboberflächen und keine globale Produkt-,
Distributions-, HA-, Vault-/Recovery- oder Paritätsfreigabe. Zusätzliche CIs am
Test-/Dokustand waren bei Abschluss teilweise noch aktiv; ihr tatsächlicher Ausgang
bleibt durch Lead zu kontrollieren. Alle sechs CIs des unveränderten Produktkandidaten639
sind tatsächlich erfolgreich, auch Web39502414582 und Backend39502414522.

## Unabhängig ausgeführt und geprüft

- Quell-/Dateigrenze `7f3191a..639ba44` geprüft: ausschließlich gepinnte Webassets,
  Manifest/Lock, Image-Quelllabel, eigene Build-/Laufzeitprüfungen und Dokumentation.
  Keine DB-/Core-/HTTP-/GoMod-/API-/Webquelländerung. Bundle enthält keine
  `/api/v3/`-/`/api/v4/`-Verwendung. Bestehender Go-Webwrapper unverändert.
- Pin exakt `6c6eb33c93eed355a10cd114f61ce27be7f3f55e`; öffentliches GitHub-Compare
  von diesem Commit nach Web-main meldet `ahead`, also bestätigte main-Ancestry.
  Sourcecheckouts prüfen beide Vertragskopien (0.1/0.2), npm-Lockhash und exakte
  Node26.5.0/npm11.17.0-Werkzeuge. Backend-Vertragshashes bleiben
  `4b598dc69a1904685e67a717997412654440ac10f47c17f7a025ccd114e294be` und
  `c15a848f01bb779429fd68a2b6eb6bba1575d3e07a4c68e6796cf7840a661838`.
- `python3 scripts/build-web.py --check`, acht Python-Buildprüfungen,
  `go test -race -count=1 ./...`, `go vet ./...` bestanden.
- Zweimal `python3 scripts/build-web.py`, jeweils anschließend `git diff --exit-code
  -- internal/webui/static internal/webui/assets-manifest.json`: zwei unabhängige
  frische öffentliche Builds liefern dieselben fünf Originalartefakte und bytegleiches
  Manifest. Original-AGPL-/Drittanbietertexte werden gegen Originalquellbytes geprüft;
  exakter eingebauter Quelllink bestätigt. Kein Umstempeln/Säubern der Buildbytes.
- `python3 scripts/test-web-integration.py`: alle fünf echten Chromiumfälle bestanden,
 19.4 Sekunden; tatsächliches eingebettetes Go-Produkt in Compose gegen SQLite und
  PostgreSQL-TLS, kein Mock-/Viteproxy. Tabellen/Paging/Sort/gebundene AND-/NULL-/
  Leerwert-/Präzisionsfilter, Struktur/Generated/Defaults/View/unstabile Ordnung und
  QA-F3-01 (ungültiger Entwurf → Zurücknehmen → Weiter/gezählter Refresh ausschließlich
  mit angewendetem alten Zustand) geprüft. Tabellenpfad sendet keine freien SQL-Fragmente.
  Bestehende SQL-/Truncation-/Session-/Storagegrenzen sowie tatsächlich beobachtete
  laufende DB-Arbeit vor Abbruch und anschließende DB-Freigabe bestanden.
  Quellen-/Lizenzdialog: feste Links, Originalhashes, Escape/Focusreturn und null
  CSP-Verletzungen. Nur gleiche Herkunft, keine Credentials im Browserstorage.
- PG-Bootstrapbarriere gelesen und tatsächlich ausgeführt: Socket akzeptiert während
  angehaltenem Bootstrap, TCP wird noch verweigert; erst Freigabe und endgültige
  TCP-Readiness erlauben Tests. CLI und Compose prüfen zuerst erfolgreiche
  verify-full-Verbindung und danach den explizit freigegebenen falschen Hostnamen.
- `python3 scripts/test-deployment.py`: Compose sowie eigener Docker29.5.2-DinD-Swarm
  bestanden Originalassets/-notices, strikte CSP, v1/v2-Auth/Fähigkeiten, API404 ohne
  HTMLmaskierung, Health, UID10001, Read-only-Root und echte SQLite-Neustartpersistenz.
  Swarm zusätzlich Secret, Placement, Routing Mesh, Taskersatz und Persistenz.
- Zusätzlicher frischer öffentlicher Clone von exakt7500ed2 ohne Geschwistercheckout:
  CLI mit `CGO_ENABLED=0 GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local` auf nachweislich
  Node-/npm-freiem PATH gebaut. Go-Modulcache des Hosts verwendet, keine Behauptung
  eines vollständig cachefreien Builds. Dasselbe Checkout baut reguläres Dockerimage;
  Laufzeitimage enthält ebenfalls kein Node/npm. Imagebau darf seine öffentlichen
  Go-Abhängigkeiten beziehen, also kein behaupteter netzloser Imagebau. Git sauber.
- Dokumentationsprüfung:241 Dateien, null Fehler. Produktworktree blieb nach allen
  Prüfungen sauber; nur diese unabhängige Reviewdokumentation wird zusätzlich committed.

## CI und Ressourcen

Am Produkt639: Docs39502414538, Web39502414582, Table39502414542,
Mutation39502414532, Backend39502414522, Engine39502414571 erfolgreich.
Am Teststand7500: Docs39502849655, Mutation39502849624, Table39502849640 und
Web39502849674 bereits erfolgreich; Engine39502849585 und Backend39502849685
waren beim geprüften Zwischenstand noch aktiv. Keine Erfolge vorweggenommen.

Autor meldete vor Review keine aktiven eigenen F3b-Läufe. Reviewer verwendete ausschließlich
zufällige eigene Skriptressourcen `dbmanager-f3b-web-*`/`dbmanager-f3b-deploy-*` sowie
zusätzliches eigenes Clean-Build-Image. Nach Abschluss keine Container/Images/Netze/
Volumes mit F3b-Präfix vorhanden, alle kurzlebigen Credentials/Clones entfernt.
Host-Swarm vorher und nachher `inactive`, fremde Entwicklungscontainer unangetastet.
Keine eigenen Prüfjobs mehr aktiv. Nächster Schritt: Lead kontrolliert verbleibende
CI-Gates und integriert nach eigener Entscheidung; Reviewer danach idle.
