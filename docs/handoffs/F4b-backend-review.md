# F4b – unabhängiger Backend- und Deployreview

23.10.2026. Reviewer Backend & Platform, kein Autor dieses Web-/Integrationspakets.
Eigener Branch `review/f4b-backend`, Worktree `.worktrees/f4b-backend-review`.
Geprüfter Produktcommit `3770c4e95ae4617a6b4858c87f1e69c963253416`,
Handoff `46e4bf1`, Basis `4044341526e9cd1f0cc2484916b4922eb0ce4958`.
Keine Produktdatei geändert; dieser Review ist keine Integration.

## Urteil und Umfang

**Keine bestätigten Findings. Das begrenzte F4b-Paket ist aus diesem unabhängigen
Review zur Integration freigegeben.** Es bindet ausschließlich die bereits
unabhängig freigegebene Webquelle
`3a4a3fd0a345f8c127758ac5eadc4e6feb8d21ec` ein. Es ist keine neue API-, Backend-,
Tresor-, HA-, Geräte- oder allgemeine Releasefreigabe. API0.5 ist weder enthalten
noch verbraucht; B3a wird getrennt geprüft.

Der Diff enthält keine Änderung an DB/Core/HTTP, GoMod/GoSum, API-Vertragsbytes,
Main oder Compose-/Swarm-Konfiguration. Änderungen beschränken sich auf
Originalassets/Manifest/Lock, Quelllabel im Dockerfile, vier Vertragspins im
Buildwächter, eng zugehörige Browser-/Deploymenttests/CI und Dokumentation.
Der reine Nachtrag `46e4bf1` ändert ausschließlich den Autorenhandoff.
Öffentliche Commitmetadaten verwenden die vorhandene Identität.

## Unabhängig reproduzierte Prüfungen

- `go test -race -count=1 ./...` vollständig und `go vet ./...` bestanden.
- `python3 scripts/build-web.py --check` und
  `python3 -B scripts/test-build-web.py`: neun Prüfungen bestanden, darunter
  fehlende/falsche v3-/v4-Pins.
- Zweimal `python3 scripts/build-web.py`: jeweils neuer öffentlicher Quellcheckout,
  gepinnte Node26.5.0/npm11.17.0, Lock-/Verträge1–4 exakt geprüft, vollständiger
  Produktionsbuild. Anschließender Gitvergleich von Assets und Manifest leer:
  beide Neubauten ergeben exakt die fünf eingecheckten Originaldateien.
  Original-AGPL und Third-Party-Notices werden gegen die Quellbytes verglichen;
  exakter Quelllink ist im fertigen JavaScript vorhanden. Keine Sourcemaps oder
  privaten Pfadmarker. Das ursprüngliche trailing whitespace in JavaScriptzeile1
  bleibt bewusst unverändert; der Diffcheck aller übrigen Dateien ist sauber.
- `python3 scripts/test-web-integration.py`: neun echte Chromiumfälle in 14,3 s
  bestanden, gegen eingebettete Rootassets und reale SQLite-/PostgreSQL-TLS-DBs.
  PostgreSQL-Bootstrapbarriere reproduziert den noch ungenügenden Socketzustand;
  erst endgültiges TCP wird akzeptiert. Eine positive verify-full-Verbindung geht
  der negativen Hostnamenprobe voraus.
- Browser: tatsächliches einzelnes INSERT/UPDATE/DELETE, DB-Default vs. NULL vs.
  Leertext, Auto-ID, große Decimalpräzision und Binary; volle typisierte Snapshots,
  PK-/Generated-/View-/NoPK-Grenzen; tatsächliche parallele Änderung führt zu409
  ohne Überschreiben. Doppelte synchrone Bestätigung erzeugt genau einen Request.
- Unknown-Proben leiten echte Requests weiter, prüfen den tatsächlich erfolgten
  Commit und verwerfen erst dessen Antwort. Genau eine gespeicherte Zeile wird
  separat nachgelesen; kein Retry. Ein weiterer echter Commit mit gehaltener
  Antwort und Tokenwechsel reaktiviert keine alten Daten. Keine erfundenen
  DB-Erfolgsantworten. Dies prüft nicht jede mögliche COMMIT-Ausfallphase.
- Vorhandene SQL-, echte laufende Abbruch-/Freigabe-, Auth-, Token-/Session-,
  Pagination-/Filter-/QA-F3-01-Regressionsfälle sowie Quell-/Lizenzdialog und
  CSP-Verstoßsammler bestanden. Die Produktions-CSP verlangt script-src self
  ohne unsafe-eval; Requests/Antwortstatus werden vom bestehenden API delegiert.
- `python3 scripts/test-deployment.py`: Compose und eigener isolierter DinD-Swarm
  bestanden. Jeweils echte v4INSERT/v3UPDATE+DELETE, stale-Snapshot409,
  Default/NULL/Präzision/Binary und exakt erhaltene typisierte Zeile nach
  Compose-Neustart bzw. Swarmtaskersatz. Originalassets/Notices, API/Auth/CSP,
  Healthchecks, UID10001 und read-only Root geprüft.
- Separater frischer öffentlicher Rootclone des exakten Produktcommits ohne
  Geschwisterrepos: CLI mit `CGO_ENABLED=0`, `GOPROXY=off`, `GOSUMDB=off`,
  `GOTOOLCHAIN=local` gebaut. Werkzeug-PATH enthielt nachweislich weder Node noch
  npm. Bestehender Go-Modulcache wurde benutzt; keine cachefreie Behauptung.
  Dockerimage ebenfalls aus diesem Clone mit demselben Host-PATH gebaut;
  normale gepinnte Go-Downloads/Buildcaches erlaubt. Runtime enthält kein
  Node/npm. Rootlizenz, alle vorhandenen Go-Lizenztexte, Manifest/Lock und
  Quell-/AGPL-Labels im fertigen Image gegen Originale geprüft. Eigene Clean-
  Images und temporäre öffentliche Checkouts anschließend entfernt.

## Exakte öffentliche CI und Ressourcen

Alle sieben CIs an **3770c4e95ae4617a6b4858c87f1e69c963253416** beim Review
abgeschlossen mit success: Dokumentation39901535040, Web39901535086,
Backend/Container39901535009, Engine39901535020, Tabellen39901535055,
Update/Delete39901535031 und Insert39901535030. Autorennachweis und unabhängige
lokale Reproduktion sind getrennte Belege.

Ausschließlich eigene zufällig benannte `dbmanager-f4b-web-*`,
`dbmanager-f4b-deploy-*` und `dbmanager-f4b-review-clean-*` Ressourcen verwendet.
Keine globalen Prunes oder Host-Swarmoperationen. Nach Abschluss keine eigenen
Container/Netze/Volumes/Images oder lokalen Prüfjobs mehr; Host-Swarm weiterhin
inactive und dieselben vier fremden Container unverändert. Temporäre Tokens,
DB-Credentials und CA-Dateien wurden durch die Skriptbereinigung entfernt.
Kein produktiver Dienst, keine Migration und keine automatische Wiederholung
unbekannter Schreibausgänge.

Nächster Schritt: Lead koordiniert einen freigegebenen Agent-Integrationsschritt
mit dem unveränderten Produktcommit und diesem Review. Die eingeschränkten
SingleReplica-/exklusiven SQLite-Volumegates und offenen allgemeinen Produktziele
gelten weiter. Keine eigene Integration oder Änderung des F4b-Kandidaten.
