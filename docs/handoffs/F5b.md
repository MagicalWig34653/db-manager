# F5b – eingebettete Strukturansicht und Ergebnisexport

Stand: 23.10.2026, vorbereitender Checkpoint. **Noch kein Produktkandidat und keine
Selbstfreigabe.** Eigener Branch `feat/schema-web-runtime`, Worktree
`.worktrees/schema-web-runtime`, Basis `8708c3ed1dcc6b45c9a96f46e1e498795bacc05b`.

## Auftrag und Dateibesitz

Nach ausdrücklicher Leadfreigabe der unabhängig geprüften F5-Webquelle die bereits
akzeptierte F6-Exportfunktion und F5-Strukturansicht gemeinsam reproduzierbar in das
Go-/Dockerprodukt übernehmen. Besitzer: Plattform-Build-Engineer. Unabhängiger
Review und getrennte Leadintegration folgen nach Fertigstellung.

Erlaubt sind Webassets/Manifest/Lock, eng begrenzte Webbuild-/Browser-/Deployment-
Prüfungen und zugehörige Dokumentation. Keine DB-/Core-/HTTP-/GoMod-/API-Vertrags-,
Site-, Apple- oder zentralen STATUS-/ARBEITSPLAN-Änderungen.

## Tatsächlich ausgeführte Vorbereitung

- AGENTS, STATUS/ARBEITSPLAN, Entwicklungsablauf, Webbuild, F4b-Handoff,
  ADR-0003/0004 und Lizenzinventar gelesen; vorhandene Build-/Runtime-/Deploypfade
  sowie die neuen Webtests ausschließlich lesend untersucht.
- Gitbasis und eigener neuer Worktree/Branch geprüft. Der bisherige Lock pinnt
  weiterhin F4-Quelle `3a4a3fd`; kein neuer Pin, Assetneubau oder Build erfolgt.
- Bestehender Build prüft API1–4. F5b muss nach Freigabe den unveränderten
  API0.5-Vertrag zusätzlich in Lock, Quellkopie, Manifest- und Negativprüfungen
  binden. Die Runtimeprüfung benötigt zusätzliche echte F5-/F6-Fälle neben den
  bestehenden neun Fällen; bloße Testdateiexistenz ist kein Ausführungsnachweis.
- Read-only Ressourcencheck: Host mit 48 GiB, `memory_pressure` meldet 57 % verfügbare
  Systemressourcen; 44 GiB Datenträger frei. Docker-VM mit 7,737 GiB, nur vier vorhandene
  fremde Container mit zusammen unter 0,9 GiB. Host-Swarm `inactive`. Beim Check keine
  aktiven fremden Xcode-/Go-/Swift-/Browser-/Deploymentprüfprozesse gefunden.

## Geltende Gates und nächster Schritt

F5-Kandidat `87751cceed47332f2164f2c3c584ddcd0ef2edd8` wird unabhängig geprüft;
dieser Vorbereitungscheckpoint erteilt keine Quellfreigabe. Erst den exakten vom
Lead angenommenen Pin übernehmen. Danach zunächst reine öffentliche Node-/Asset-
und Go-Prüfungen. Aufwendige Dockerprüfungen gesondert koordinieren: höchstens zwei kleine
DB-Suites gleichzeitig; ein eigener isolierter DinD-Lauf erst nach bestätigter
Bereinigung der B4a-Drei-DB-Reviewmatrix. Keine zweite parallele Swarm-/MySQL-Suite.

Keine neuen Produkt-/Browser-/Containerprüfungen ausgeführt, keine eigenen
Prüfprozesse oder Container gestartet. Fremde Ressourcen, Images und ursprüngliche
TestFlightarchive bleiben unverändert. Originale generierte JS-/Lizenzbytes werden
auch beim späteren Neubau nicht nachträglich bereinigt.
