# PIN-BUNDLE-4 Pin auf Web-Branch release/pin-bundle4 (9545f75)

Stand 26.10.2026. Branch `release/pin-bundle4` (Basis Root-main 751dbd1), nicht nach main gemergt. Folgt auf Bündel 3.

## Ergebnis

- **Web-Branch:** `release/pin-bundle4` im Web-Repo, aus `integration/web-wave-fixes` (ce1ae9b; dessen Merge-Basis ist bereits Web-main
  c14271e, also keine Neubasierung nötig). Nichts nach Web-main gemergt. Commits darauf: `91e35ff` (Konten auf Vertrag 0.23),
  `9545f75` (`test-real-tables.py` mit `DBM_BACKEND_REPO`). **Lock-Commit: `9545f752e56149ede6a07eea52f808691682b908`**,
  `npm_lock_sha256` unverändert `bac4b32b...5c6` (gegen `package-lock.json` des Commits geprüft).
- **Konten auf Vertrag v23:** `src/lib/contract/v23/` (`api-v0.23.0.json` bytegleich zu Root, SHA-256
  `7450a7fc994126beb25799e99dae07d6763e43f3e9ef002b20068cb38e313bd7`, Types und Validatoren generiert, `provenance.json` mit Root-Commit
  751dbd1). Das Konten-Feature importiert Typen und Validatoren aus v23. `AccountResult.reason` enthält `grant_without_effect`.
  Der `test.fail()` für Review F-2 ist entfernt; der Test läuft normal und besteht. v23 fügt keine Pfade hinzu: Kontenrouten bleiben
  `/api/v15/...` (ACCOUNTS-API.md). v17 bis v22 sind nicht kopiert (noch keine Oberfläche). Die Capabilities-Antwort der Konten trägt
  weiter `contract_version` 0.15.0 (Konstante im v23-Schema).
- **Lock:** 17 Verträge. Hinzugekommen gegenüber Bündel 3 (alle Webkopien byte-identisch zu Root): `maintenance` 0.10, `sqlbatch` 0.12,
  `indexes` 0.13, `search` 0.14, `accounts` 0.15, `programs` 0.16, `reasons` 0.23. 0.17 bis 0.22 bewusst ungepinnt.
- **Neubau:** 42 eingebettete Dateien (31 unter `assets/`). `build-web.py` zweimal frisch: Verzeichnisse identisch (`diff -r`), `--check`
  bestanden. Dockerfile-Label auf 9545f75, WEBBUILD (Abschnitt Pin Bündel 4), LIZENZEN, STATUS aktualisiert.
- **MAX_CHUNKS = 64** (vorher 24): reine Plausibilitätsgrenze. Begründung: die Zahl riss mit jeder Welle (31 von 64 belegt); die
  Garantien sind Entry mit Quelllink, transitive Erreichbarkeit jeder Datei unter `assets/` und `SHELL` = Inventar (Review Bündel 3,
  Befund 2). Kommentar im Skript.
- **Precache-Größe: ca. 16,2 MB** (16 226 019 Bytes ohne `sw.js`, davon 16 056 083 unter `assets/`, JS 15 982 712), einmalig je Release
  beim Service-Worker-Install. Bündel 3 hatte ca. 4,8 MB: das Wachstum ist erheblich (großer Feature-Chunk-Anteil). Neu im Build:
  `MAX_PRECACHE_BYTES` (32 MiB) bricht den Neubau darüber ab, mit Test. Offen für die Web-Seite: Lazy-Precaching für selten genutzte
  Features, falls mobile Erstinstallation wichtig ist.
- **Review-Auflagen aus PIN-BUNDLE-3-review:** (1) Web-Skript `scripts/test-real-tables.py` nutzt `DBM_BACKEND_REPO`/`DBM_BACKEND_REV`
  nun auch für das Quellbackend (Export per `git archive`, Fallback weiter der gepinnte Commit). Lauf gegen Root-main 751dbd1:
  10 von 10 Specs bestanden (export, rows, structure, table; SQLite und PostgreSQL verify-full). (2) MAX_CHUNKS siehe oben,
  Precache-Größe genannt. (3) Flake `TestSQLiteSQLBatchLimitsAndCancellation`: der Aggregatfall nutzt jetzt das maximale Zeitbudget
  (`TimeoutMS` 60000) und schlägt mit klarer Meldung fehl, falls je `query_canceled` statt Größenlimit auftritt; 3 Wiederholungen mit
  `-race` grün, Gesamtlauf grün.
- **Korrektur der Aussage in PIN-BUNDLE-3:** "Locatoren passen offenbar nicht mehr zur neuen Shell" ist **falsch**. Ursache der vier
  Fehlschläge war, dass `test-real-tables.py` das Backend aus dem alten Commit in `v5/provenance.json` holte, nicht aus dem Release
  (siehe PIN-BUNDLE-3-review, Befund 1). Mit Quellbackend per `DBM_BACKEND_REPO` bestehen alle 10 Specs. Hinweis: das Skript startet
  die Web-Oberfläche per Vite-Preview, das Backend liefert nur die API; für die eingebettete Oberfläche gilt `test-web-integration.py`.

## Web-Prüfungen (Branch release/pin-bundle4, Node 26.5.0, npm 11.17.0)

| Befehl | Ergebnis |
| --- | --- |
| `npm run check` | 0 Fehler, 0 Warnungen (5144 Dateien) |
| `npm test` | 54 Dateien, 694 Tests bestanden |
| `format:check`, `contract:check`, `validators:check` | bestanden |
| `npm run build` | bestanden (nach Commit) |
| `npm run test:csp` | 2 von 2 (Port 4180) |
| `npm run test:pwa` | 6 von 6 (Port 4181) |
| Playwright `tests/browser` (temporäre Konfiguration Port 4391, Port 4179 nicht angefasst) | 72 von 72, darunter REVIEW F-2 als normaler Test |
| `scripts/test-real-tables.py` mit `DBM_BACKEND_REPO` | 10 von 10 |

## Backend-Prüfungen (Worktree `.worktrees/pin-bundle4`, nacheinander)

| Befehl | Ergebnis |
| --- | --- |
| `build-web.py` zweimal, `--check`, `test-build-web.py` | 42 Dateien, identisch; OK (inkl. neuer Precache-Test) |
| `go test -race ./...`, `go vet ./...`, `gofmt -l .` | ok, leer |
| 24 `check-*-contract.py` und `check-docs.py` | alle bestanden (`check-extended-export-files.py` braucht ein Argument und ist kein Wächter) |
| `test-web-integration.py` | Exit 0, 13/13 Browserfälle SQLite+PostgreSQL, `PWA embedded check passed.` |
| `test-deployment.py` | Exit 0 (Compose, Swarm), Aufräumen gemeldet |

## Eingebettete Dateien

| Pfad | Bytes | SHA-256 |
| --- | --- | --- |
| `LICENSE.txt` | 34523 | `0d96a4ff68ad6d4b6f1f30f713b18d5184912ba8dd389f86aa7710db079abcb0` |
| `THIRD-PARTY-NOTICES.txt` | 92896 | `5757e0b3179e381d4393f2e51784a7109c00b3276546b8fcf551cd8f1309135e` |
| `apple-touch-icon.png` | 3664 | `16d0cdb88277d760d947ea00749dd912057995e2c50db8bc5bc985408892081d` |
| `assets/ConfirmDestructiveDialog-CSv6taNs.js` | 2435 | `76414a289d8da518ba5da0f74f24a11a83d3e6b529ffc87dba5d02305fcd0c4e` |
| `assets/PlanPreview-CXTuyKb9.js` | 1579 | `e54b9c7c74a6990c9060cc9713b39990fb1bd13244d4f8df7b815067cc2d0e0f` |
| `assets/PlanPreview-DIdf5lVd.css` | 532 | `d666349262672b2949e94fd4b85df4ec6b6e97cb0042381c3a5350426d4419b6` |
| `assets/circle-check-D0HqYeNO.js` | 363 | `2912a23270b6fb73bf84238f7bd9208b81e9c731d206ff6cd4dd5f63ce8c9fee` |
| `assets/client-LSdyyzKw.js` | 443992 | `79dfc2de56452e0ac750baea2311ac1a70c90fbaf40984f9532ac449b8c98147` |
| `assets/feature-7Lq1Bwpj.js` | 2413627 | `e40cead4c6b93ee1e8a031101b687de10a56ab141d9722533967f04c122b9c75` |
| `assets/feature-ADYQo2EN.css` | 835 | `4f0f00320f32fcea82e7f5ec14423bb8cc0daafb45c5fa1807cb3ac653feae66` |
| `assets/feature-B87oFnCN.js` | 1752466 | `9d0839cd1f34c380011768f7c33159602539719023f79bf59eab07b43a7c11a8` |
| `assets/feature-BPqPrbnZ.css` | 2053 | `79fd88e5e2782cdaf5677b7f56d593244e7499ec5d3783e78e0b984e027bc9a3` |
| `assets/feature-BWF60C02.js` | 1054706 | `599647016faf411fba11335b1e1c6d49255cb56069a00c163844a324b3743048` |
| `assets/feature-BXrbnqoD.css` | 581 | `5ede5125b941127c76c7a0ee71cce84acc349014b29f781b2ed597a3945a8d88` |
| `assets/feature-Bb6ZvVSj.css` | 2134 | `6b7e00bdce4a606a6e70abddcd411c6fbc7b3f6487bf9fc35bfa7b84976f5929` |
| `assets/feature-BcixFSIF.css` | 2792 | `943088577fcb657325fc54578ebd8561684244bad12a49eae94a03205e0afa9c` |
| `assets/feature-Bi78JpLS.js` | 758612 | `f84ee9ab87877b6ec9f35ab1627c16f73c9d926af41461d5c50cb2afa415b17b` |
| `assets/feature-BzsSHduT.css` | 1225 | `5d978885b1c20e3004b7eb6b820797db04eefada86eddc4b725f32c0733d4481` |
| `assets/feature-C3VbvhN6.js` | 311522 | `838571cc5efe95474771a8ee631d5c6a80e9bb86e5f86b22cd8ce9e796e6921b` |
| `assets/feature-Cc6hvP7B.js` | 29524 | `e11dbc4716ef3215f4d1f08a8afd223168aca2d2f2bb4be7f336294490d12b79` |
| `assets/feature-CcT1h_AV.js` | 3140297 | `b5607c3afc0d9ebcaf747a6548e4aadf0a712779fc9a8dc6a339466a9c610602` |
| `assets/feature-CzdzV0ta.css` | 1570 | `4599f0965b6204e390ac4284ab5eac9ab4dc222f4996d5558df98b23929b1693` |
| `assets/feature-D2dwx9Kw.js` | 936457 | `a1e3b03090a12b4e110853277ec7fc5c1485fd92a49a609cd4f8ba9de8e767e3` |
| `assets/feature-N75mQuD4.js` | 1625098 | `6a9f3a9c4e97881e43acfa85c14af2ad417e1e1298abb1cb035a36ebd28e9503` |
| `assets/feature-S9VUcT0E.js` | 1200049 | `f6ce5272e557bd67d6ca6b3b427b9afca1d18a8fbab89468e3725b401259f373` |
| `assets/feature-TAIK-rSh.css` | 2217 | `ef0bb1ebdc277310ee49c87dacb492d9dd03aa6ea3a6f1534b5ac037452063e2` |
| `assets/feature-h-4sSlrU.css` | 1148 | `b629662caf089a783c2addd697b52085630d2bfc5a81d4bb900acc95747f56bc` |
| `assets/feature-pIB27gao.js` | 1158553 | `a30765e855345e69d55919b69f47b4f20d7a1fd38332ba11c8f620804bfddfe1` |
| `assets/feature-waj8jLyE.css` | 847 | `1d5d34b7ce85bae10a24e2c4cc560b0c23ca1288dbf428baf136ecc6c0558d96` |
| `assets/feature-xHmDGcod.js` | 2347 | `06bcc5ed2d06b7789952df186615dae9c397204fe0eb1599564ac7d313e1e392` |
| `assets/feature-xM-9dNS5.css` | 851 | `d2daf06a8ec8aa5dd94033f2fb185ed0b5ae1ff80e6b182ceac95408f7c77e79` |
| `assets/i18n-Kc9hv7_9.js` | 203867 | `8663c86ad3ea75be9f8f56dfd35ff5f3f96f1e4b5813dca0c0102167b5a87cb7` |
| `assets/index-D6brZ7xF.js` | 947218 | `3ee302b524d3b2061f6131fa0729e95007e078a8bbec368d9db1c55555824b0c` |
| `assets/index-DDp0P-Oi.css` | 56586 | `6729ba7f654909e2afe5a987ea0f1b1b70ad90cdbd3e09a998b53446cda68fae` |
| `favicon-32.png` | 1196 | `7b45cb31c1b94f78b7d9b6745c33a544f5426d87df779197eb64cc38a198b001` |
| `icon-192.png` | 6990 | `530e8598c7367a2ee3999250cd00962651cd995e536c3e1520e461ec2e0d650b` |
| `icon-512.png` | 19251 | `9f1074d82139629dc9350ba12c0fb21f79ea78295a5c85bc0466a44c72b4ba76` |
| `icon-maskable-512.png` | 9612 | `e1fd5502743b542a488dd22d3d4fb3144d9e6814bf20c5eaeb20224f94790bba` |
| `icon.svg` | 269 | `d3dce0051a47d063a496849024190072fb3e2868c8d025a949760945456786b1` |
| `index.html` | 739 | `fcb139ffee3e0caedb46bd096093230d511d591059016cb8fe2b01842f7f720e` |
| `manifest.webmanifest` | 796 | `7949f25cca67f0956a0d81e9561d1d5d1888765b56c12eb90943613fea2d122b` |
| `sw.js` | 4141 | `66039e14e0a990d5272dca5d600315fe4520b9a15968bd5f3ed44326b03074f2` |

## Einschränkungen

- Neue Web-Oberflächen jenseits der Konten (Wellen 2 bis 4: Programme, Suche, Indizes, SQL-Batch, Wartung) laufen nur gegen die
  Browserspezifikationen des Webrepos, nicht gegen die eingebetteten Assets; `test-real-*.py` der neuen Familien wurden hier nicht
  ausgeführt (nur `test-real-tables.py`).
- Kein Lighthouse, Unterpfadbetrieb nicht unterstützt, Bytegleichheit nur lokale Plattform/Toolchain.
- Alle 12 vorhandenen Docker-Container waren vor den Läufen vorhanden und wurden nicht angefasst.
