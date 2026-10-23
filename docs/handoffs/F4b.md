# F4b – eingebetteter Zeileneditor

Stand: 23.10.2026, Prüfkandidat, keine Selbstfreigabe. Eigenes Rootworktree/Branch
`feat/row-web-runtime` ab `4044341526e9cd1f0cc2484916b4922eb0ce4958`.

## Herkunft und Besitz

Ausschließlich unabhängig freigegebene öffentliche Webquelle
`3a4a3fd0a345f8c127758ac5eadc4e6feb8d21ec`. P0-Review01933d5 ohne Findings und drei
exakte Quell-CIs erfolgreich; Web-main nach delegierter Agentintegrationbf75748
enthält Feature- und Review-Ancestry. Keine Webproduktbyteänderung nach Pin.
Besitz Assets/Manifest/Lock, enge Build-/Wrapper-/Dockerprovenienz, Runtimebrowser-/
Deploymenttests und zugehörige Dokumentation. Keine DB-/Core-/HTTP-/GoMod-/API-
Vertragsänderung, keine RootSTATUS/ARBEITSPLAN, kein Verbrauch des API0.5-Kandidaten.

## Reproduzierbarkeit und Prüfstand

- Zwei frische öffentliche Quellcheckouts: fünf Assets und Manifest bytegleich.
  API1–4- und npm-Lock-Pins exakt; originale AGPL-/Notice-Texte bytegleich. Keine
  Sourcemaps, privaten Pfade, Node_modules oder Secrets in Assets.
- `python3 scripts/build-web.py --check` und neun Python-Buildprüfungen bestanden,
  einschließlich fehlender/falscher v3-/v4-Pins. Go-Webpaket Race/Vet bestanden.
- `python3 scripts/test-web-integration.py`: neun echte eingebettete Chromiumfälle
  in 14,5 s bestanden. SQLite und PostgreSQL verify-full mit positiver TLS-Probe vor
  falschem Host und deterministischer Bootstrap-/TCP-Barriere.
- CRUD/Default/NULL/Leertext/AutoID/Precision/Binary, voller typisierter Originalsnapshot,
  PK/generated-Sperren, tatsächliche parallele Änderung→409 ohne Überschreiben, View/
  noPK-Sperren. Zwei synchrone Bestätigungsklicks erzeugen genau einen Request.
- Echte INSERT-Antwort nach tatsächlichem `outcome: committed` verloren: genau ein
  Request und eine gespeicherte Zeile separat nachgelesen, ehrlicher Unknown-/NoRetry-
  Zustand. Zweiter realer Commit mit gehaltener Antwort und Tokenwechsel reaktiviert
  keine alten Daten. Netzwerkfehlerinjektion verwirft echte Antworten; keine erfundene
  DB-Erfolgsantwort. Keine Aussage über sämtliche möglichen COMMIT-Ausfallphasen.
- Ursprüngliche SQL-/Abbruchfreigabe-/Auth-/Sitzungs-/F3-Pagingfälle sowie exakter
  Quell-/Originalnotice-Dialog und CSP-Verstoßsammler weiterhin grün.
- `python3 scripts/test-deployment.py`: Compose und eigener isolierter DinD-Swarm
  bestanden. Jeweils tatsächlicher v4-INSERT/v3-UPDATE+DELETE, stale-Snapshot409,
  DB-Default/NULL/Präzision/Binary und exakt erhaltene Zeile nach Neustart/Taskersatz.
  Assets/CSP/Auth/Health/UID10001/read-only-Root geprüft. Hostswarm unverändert.

Der erste Rootbrowserlauf erfasste nur die fünf alten Fälle, weil die bestehende
explizite Testliste den neuen Dateinamen noch nicht enthielt. Liste ergänzt;
oben zählt ausschließlich der anschließende vollständige Neun-Fälle-Lauf.

## Grenzen und nächste Schritte

Sauberer öffentlicher Go-/Imagecheckout ohne Node/npm/Geschwister sowie öffentliche
CI stehen bei diesem Checkpoint aus. Unabhängige Root-QA vor Integration erforderlich.
Keine öffentliche Deployment-/Releasefreigabe, keine HA-/Powerloss-/Exactly-once-
Zusicherung. SQLite weiterhin exklusives lokales SingleReplica-Volume gemäß ADR0004.
Keine neue UI-Funktion und keine Änderung der freigegebenen Vertragsbytes.
Alle bisher gestarteten eigenen Browser-/CLI-/Compose-/PG-/DinD-Jobs und privaten
Wegwerfcredentials sind beendet/entfernt; keine fremden Container angefasst.

## Öffentlicher Abschlusscheckpoint

Produkt-/Testkandidat `3770c4e95ae4617a6b4858c87f1e69c963253416` ist auf
`feat/row-web-runtime` öffentlich gepusht. Frischer öffentlicher Rootclone dieses
Commits ohne Geschwister baute CLI und Dockerimage mit Werkzeug-PATH, auf dem Node/npm
nachweislich fehlen. CLI: `CGO_ENABLED=0`, `GOPROXY=off`, `GOSUMDB=off`,
`GOTOOLCHAIN=local`, bestehender Go-Modulcache. Containerbuild lädt normale gepinnte
Go-Module; kein behaupteter netzwerkfreier Image-Neubau. Fertiges Runtimeimage ohne
Node/npm; originale Root-/Go-Lizenzen und Weblock/Manifest im Image verifiziert.
Eigenes Cleanimage und temporärer Checkout vollständig entfernt.

Zusätzlich `go test -race ./...` und `go vet ./...` vollständig bestanden.
Dokumentation259/0; Diffprüfung für eigene Quellen/Doku/Tests grün. Einzige bewusste
Ausnahme: originale generierte Datei `index-Bk7Yt41f.js`, Zeile1 enthält bereits im
Quellbuild trailing whitespace. Wie beim vorherigen F3b-Pin bleiben Originalbytes
unverändert; zwei öffentliche Neubauten und Manifest beweisen dieselben Bytes.
Keine Handänderung an Minifikat, Lizenztexten oder freigegebener Webquelle.

Öffentliche CI an exakt `3770c4e` bei Übergabe:

- [Dokumentation39901535040](https://github.com/MagicalWig34653/db-manager/actions/runs/39901535040): bestanden.
- [Web39901535086](https://github.com/MagicalWig34653/db-manager/actions/runs/39901535086): aktiv; enthält Linux-Quellreproduktion und neun echte Browserfälle.
- [Backend/Compose/Swarm39901535009](https://github.com/MagicalWig34653/db-manager/actions/runs/39901535009): aktiv.
- [Engine39901535020](https://github.com/MagicalWig34653/db-manager/actions/runs/39901535020): aktiv.
- [Tabellen39901535055](https://github.com/MagicalWig34653/db-manager/actions/runs/39901535055): aktiv.
- [Update/Delete39901535031](https://github.com/MagicalWig34653/db-manager/actions/runs/39901535031): aktiv.
- [Insert39901535030](https://github.com/MagicalWig34653/db-manager/actions/runs/39901535030): aktiv.

Web-main nach delegierter Integration: `bf75748df9eb4422b084c20784d09ce931301d37`.
[Neue echte Webintegration39901222403](https://github.com/MagicalWig34653/db-manager-web/actions/runs/39901222403)
und [Dokumentation39901222564](https://github.com/MagicalWig34653/db-manager-web/actions/runs/39901222564)
bestanden; [Frontend39901222539](https://github.com/MagicalWig34653/db-manager-web/actions/runs/39901222539)
bei Abschluss noch aktiv. Produktquelle bleibt ausschließlich freigegebenes3a4a3fd.

Dieser reine Handoffnachtrag verwendet `[skip ci]`, um unveränderte Produktprüfungen
nicht nochmals zu starten. CI-Kontrolle und unabhängige Reviewzuweisung an den Lead
übergeben. Keine eigenen laufenden Testjobs/Container/Netze/Volumes/Images oder
Credentials verbleiben. Hostswarm weiterhin `inactive`; vier fremde Container
unverändert. Nächster konkreter Schritt: unabhängiger Review des Produktkandidaten
samt Handoff und verbleibende CI-Ergebnisse prüfen, danach gezielte Rootintegration.
