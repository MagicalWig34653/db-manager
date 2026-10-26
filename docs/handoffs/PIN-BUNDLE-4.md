# PIN-BUNDLE-4 Pin auf Web-Branch release/pin-bundle4 (f830326)

Stand 26.10.2026. Branch `release/pin-bundle4` (Basis Root-main 751dbd1), nicht nach main gemergt. Folgt auf Bündel 3.

## Ergebnis

- **Web-Branch:** `release/pin-bundle4` im Web-Repo, aus `integration/web-wave-fixes` (ce1ae9b; dessen Merge-Basis ist bereits Web-main
  c14271e, also keine Neubasierung nötig). Nichts nach Web-main gemergt. Commits darauf: `91e35ff` (Konten auf Vertrag 0.23),
  `9545f75` (`test-real-tables.py` mit `DBM_BACKEND_REPO`), `f830326` (schlanke Validatoren, Precache 16,2 auf 3,6 MB). **Lock-Commit: `f8303266a403ebf2c75280aa68ff875a03dae31d`**,
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
  bestanden. Dockerfile-Label auf f830326, WEBBUILD (Abschnitt Pin Bündel 4), LIZENZEN, STATUS aktualisiert.
- **MAX_CHUNKS = 64** (vorher 24): reine Plausibilitätsgrenze. Begründung: die Zahl riss mit jeder Welle (31 von 64 belegt); die
  Garantien sind Entry mit Quelllink, transitive Erreichbarkeit jeder Datei unter `assets/` und `SHELL` = Inventar (Review Bündel 3,
  Befund 2). Kommentar im Skript.
- **Precache-Größe: 3,6 MB** (3 616 607 Bytes ohne `sw.js`, davon 3 446 671 unter `assets/`, 31 Dateien), gemessen am Neubau. Bündel 3: ca. 4,8 MB.
  Ein erster Stand dieses Bündels (Web-Commit 9545f75, Backend 795bab5) hatte 16,2 MB. Ursache: Die generierten Validatoren
  `contract/vN/validators.js` enthielten je Version den Code der gesamten API bis N (v23: 6,6 MB), jeder Feature-Chunk bündelte
  seine Kopie. Fix im Web (`f830326`): `scripts/validator-roots.json` + Generator exportiert nur benötigte Schemas (Typen: Generics auf
  `keyof typeof validators` eingeengt). Alle anderen Verdächtigen (Fixtures, Sourcemaps, unminifiziert) waren nicht die Ursache. Da unter 8 MB,
  keine Änderung der Service-Worker-Strategie. `MAX_PRECACHE_BYTES` jetzt 8 MiB (Test vorhanden). Hinweis: `contract/v15` (nicht mehr
  verwendet, 4,6 MB Quelltext) liegt unverändert im Web-Repo, wird aber nicht gebündelt.
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
| `npm test` | 54 Dateien, 694 Tests bestanden (nach Validator-Slimming erneut) |
| `format:check`, `contract:check`, `validators:check` | bestanden |
| `npm run build` | bestanden (nach Commit) |
| `npm run test:csp` | 2 von 2 (Port 4180) |
| `npm run test:pwa` | 6 von 6 (Port 4181) |
| Playwright `tests/browser` (temporäre Konfiguration Port 4391, Port 4179 nicht angefasst) | 72 von 72, darunter REVIEW F-2 als normaler Test |
| `scripts/test-real-tables.py` mit `DBM_BACKEND_REPO` | 10 von 10 |

## Backend-Prüfungen (Worktree `.worktrees/pin-bundle4`, nacheinander)

| Befehl | Ergebnis |
| --- | --- |
| `build-web.py` zweimal, `--check`, `test-build-web.py` | 42 Dateien, identisch (`diff -r`); OK (inkl. Precache-Test); alles nach dem Slimming erneut ausgeführt |
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
| `assets/ConfirmDestructiveDialog-Bh7z8zqi.js` | 2435 | `173a290557c264129c2462a4934c0edb718831f6d13b6701e81f15c32e33c88f` |
| `assets/PlanPreview-DIdf5lVd.css` | 532 | `d666349262672b2949e94fd4b85df4ec6b6e97cb0042381c3a5350426d4419b6` |
| `assets/PlanPreview-DeCDsfEs.js` | 1579 | `933b5470c7335d0212047cba926db59c18f2168f6d5821912c97390fafd1d342` |
| `assets/circle-check-Cp2hF6EH.js` | 363 | `9044886b1ae480c6491bc976f2f5f6981e00dc1e4da194ec3e8fd78c82712bf3` |
| `assets/client-CCkPvLtJ.js` | 147588 | `ad26f57d51f98516666835494e6a3ffa6c98c946e1aa2b4c482e9321ea59624c` |
| `assets/feature-ADYQo2EN.css` | 835 | `4f0f00320f32fcea82e7f5ec14423bb8cc0daafb45c5fa1807cb3ac653feae66` |
| `assets/feature-BCTW6oLM.js` | 113408 | `b62e288e0a9dd977000228825d621580e90f6b20b5af199fdbc2233694857ac6` |
| `assets/feature-BPqPrbnZ.css` | 2053 | `79fd88e5e2782cdaf5677b7f56d593244e7499ec5d3783e78e0b984e027bc9a3` |
| `assets/feature-BXrbnqoD.css` | 581 | `5ede5125b941127c76c7a0ee71cce84acc349014b29f781b2ed597a3945a8d88` |
| `assets/feature-BYjOjN0t.js` | 90832 | `ea5ee6b297b1e202a5f72acd50bfc4296a538e6ea0513d8420de7d2f4afa1911` |
| `assets/feature-Bb6ZvVSj.css` | 2134 | `6b7e00bdce4a606a6e70abddcd411c6fbc7b3f6487bf9fc35bfa7b84976f5929` |
| `assets/feature-BcixFSIF.css` | 2792 | `943088577fcb657325fc54578ebd8561684244bad12a49eae94a03205e0afa9c` |
| `assets/feature-BrkxlQp0.js` | 159494 | `bb35af47e0b20a04bd03576ede1c2fb0030c4984a13bc5580f44e37c4ac3961b` |
| `assets/feature-Bteq-aQc.js` | 506351 | `92d07e5e055258a43487d62e0f0c11d5ff66acb4d20e89c55575a7190661b9a3` |
| `assets/feature-BzsSHduT.css` | 1225 | `5d978885b1c20e3004b7eb6b820797db04eefada86eddc4b725f32c0733d4481` |
| `assets/feature-CyQ7YOqw.js` | 2347 | `80c1afca25f8703ec3b3a2952ccd86f857e13b7c85eef996da4ec1654b33927f` |
| `assets/feature-CzdzV0ta.css` | 1570 | `4599f0965b6204e390ac4284ab5eac9ab4dc222f4996d5558df98b23929b1693` |
| `assets/feature-D0hNuw8K.js` | 247029 | `e9c8e153405667be17138ef201dbb967b65fec5ede182a9495c248250fa44fd9` |
| `assets/feature-DQmygWNQ.js` | 250584 | `d8890c92e3252adf851a3ce55ac4e7b73ea67dd3efa7ea84f214d2f208e21500` |
| `assets/feature-DfJiTbF9.js` | 171579 | `9542d9beee29b625fd734edcfc4088ab73c79e62a8bb6fb24f9d038fd7453ec0` |
| `assets/feature-DpG8GTFc.js` | 29524 | `645b9ce834e209a0d6b48552db551213bcf066ae0564bd00699ee825dd990a9e` |
| `assets/feature-DwBRdFbC.js` | 65749 | `765b85557ebe5ee8b693577a7220fb893b096e51b47b156c0b58a00bd2a5a774` |
| `assets/feature-MZl8L44p.js` | 229800 | `2991054a0f5c56b8d460baa7b700eddb6e238d0f91acbfbdd5b6e73ff444f6c0` |
| `assets/feature-TAIK-rSh.css` | 2217 | `ef0bb1ebdc277310ee49c87dacb492d9dd03aa6ea3a6f1534b5ac037452063e2` |
| `assets/feature-X26ey4L0.js` | 510509 | `84855d91ea4354c4720ef19f545b0596a291af2dc1615bc35176d4db3b11131f` |
| `assets/feature-h-4sSlrU.css` | 1148 | `b629662caf089a783c2addd697b52085630d2bfc5a81d4bb900acc95747f56bc` |
| `assets/feature-waj8jLyE.css` | 847 | `1d5d34b7ce85bae10a24e2c4cc560b0c23ca1288dbf428baf136ecc6c0558d96` |
| `assets/feature-xM-9dNS5.css` | 851 | `d2daf06a8ec8aa5dd94033f2fb185ed0b5ae1ff80e6b182ceac95408f7c77e79` |
| `assets/i18n-Kc9hv7_9.js` | 203867 | `8663c86ad3ea75be9f8f56dfd35ff5f3f96f1e4b5813dca0c0102167b5a87cb7` |
| `assets/index-BVScxKFM.js` | 640262 | `a2c68e704674773a05566f32c277b21c7cc9dc7f0ba9eed4749a3941ba5be6ba` |
| `assets/index-DDp0P-Oi.css` | 56586 | `6729ba7f654909e2afe5a987ea0f1b1b70ad90cdbd3e09a998b53446cda68fae` |
| `favicon-32.png` | 1196 | `7b45cb31c1b94f78b7d9b6745c33a544f5426d87df779197eb64cc38a198b001` |
| `icon-192.png` | 6990 | `530e8598c7367a2ee3999250cd00962651cd995e536c3e1520e461ec2e0d650b` |
| `icon-512.png` | 19251 | `9f1074d82139629dc9350ba12c0fb21f79ea78295a5c85bc0466a44c72b4ba76` |
| `icon-maskable-512.png` | 9612 | `e1fd5502743b542a488dd22d3d4fb3144d9e6814bf20c5eaeb20224f94790bba` |
| `icon.svg` | 269 | `d3dce0051a47d063a496849024190072fb3e2868c8d025a949760945456786b1` |
| `index.html` | 739 | `7913a505a922801581205484a7d3d12850d0da6c17bc2fdbe9b215b41e093668` |
| `manifest.webmanifest` | 796 | `7949f25cca67f0956a0d81e9561d1d5d1888765b56c12eb90943613fea2d122b` |
| `sw.js` | 4141 | `6079ef09119e76705a9f995052571c3a535a9f74d93b5441080740ccb6569a43` |

## Einschränkungen

- Neue Web-Oberflächen jenseits der Konten (Wellen 2 bis 4: Programme, Suche, Indizes, SQL-Batch, Wartung) laufen nur gegen die
  Browserspezifikationen des Webrepos, nicht gegen die eingebetteten Assets; `test-real-*.py` der neuen Familien wurden hier nicht
  ausgeführt (nur `test-real-tables.py`).
- Kein Lighthouse, Unterpfadbetrieb nicht unterstützt, Bytegleichheit nur lokale Plattform/Toolchain.
- Alle 12 vorhandenen Docker-Container waren vor den Läufen vorhanden und wurden nicht angefasst.
