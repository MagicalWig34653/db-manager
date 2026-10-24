# PIN-BUNDLE-2 Pin auf Web-main 48c2f67

Stand 24.10.2026. Branch `release/pin-bundle2` (Basis Root-main 5921a9d), nicht nach main gemergt. Ersetzt das abgelehnte
Bündel 1 ([Review](PIN-BUNDLE-1-review.md) liegt im Review-Branch `review/pin-bundle1`).

## Ergebnis

- Lock: commit `48c2f67113d8fd0a27e9cf966d20904fd7d0da61`, `npm_lock_sha256` `bac4b32ba0548428186f20b1d6382f3554b1a477c0fb89957b123820495b15c6`
  (unverändert, gegen `package-lock.json` des Commits geprüft). Verträge: 0.1 bis 0.5 unverändert, neu `create` 0.6.0
  `b08b6aba858f72a3bad1fce2933e487eed017f5e35b4690cfd9fae3a590a6887` und `overview` 0.7.0
  `c6fd8c91315e25185d949d309397c7c0d80cc67fcab87ae521e38068e7004d17`; Webkopien v6/v7 vorab per SHA-256 als identisch belegt,
  der Neubau prüft sie erneut.
- Neubau: 17 eingebettete Dateien (Web ist jetzt code-gesplittet, vier `feature-*`-Chunks). Dafür wurde die harte Zahl "genau fünf
  Originaldateien" in `build-web.py` zu "mindestens fünf" gelockert; Quelllink-, Lizenz-, Sourcemap- und Pfadprüfungen unverändert.
- Reviewer-Fixes übernommen: Dockerfile-Label auf 48c2f67; `verify_http` verlangt `immutable` nur für `assets/`; neuer Test in
  `test-build-web.py` erzwingt Label = Lock-Commit. `TestBundledAssetsMatchManifest` prüft alle PWA-Dateien (Status, MIME, no-store).
- `scripts/check-pwa-embedded.mjs` mit echten Assertions (Exit 1 bei Fehler) und aus `test-web-integration.py` nach den
  Playwright-Tests gegen den Compose-Container aufgerufen (damit auch im CI-Workflow `web-runtime.yml`).
- STATUS/ARBEITSPLAN/WEBBUILD aktualisiert.

## Eingebettete Dateien

| Pfad | Bytes | SHA-256 |
| --- | --- | --- |
| `LICENSE.txt` | 34523 | `0d96a4ff68ad6d4b6f1f30f713b18d5184912ba8dd389f86aa7710db079abcb0` |
| `THIRD-PARTY-NOTICES.txt` | 92896 | `5757e0b3179e381d4393f2e51784a7109c00b3276546b8fcf551cd8f1309135e` |
| `apple-touch-icon.png` | 3664 | `16d0cdb88277d760d947ea00749dd912057995e2c50db8bc5bc985408892081d` |
| `assets/feature-BPqPrbnZ.css` | 2053 | `79fd88e5e2782cdaf5677b7f56d593244e7499ec5d3783e78e0b984e027bc9a3` |
| `assets/feature-BbAEuRYR.js` | 474054 | `c9315d6ad6e046b84062bd16ab78d4150763f6d98bb6938d054c1fe0ef6ae27c` |
| `assets/feature-C2xS-vVO.js` | 311642 | `f569a58658ce59dfa686399024f9604831af28be35cba352143613eda2cfdca1` |
| `assets/feature-D2nHilXW.css` | 1917 | `3677b26565ec5cb5acc15900cc1a260afa2194ef2a61277cca9f2017d6ef160a` |
| `assets/index-CY5VlWzu.css` | 47970 | `e8cfb814af629e7596b9540b1b09ef4dcaa2e74d173b445bd64010386368005a` |
| `assets/index-CnOlEE_p.js` | 999405 | `43ff5b2a8e18adbbe643b8ad0e58ad7bc8f675516aefe5af4cb1572dad8a0d1b` |
| `favicon-32.png` | 1196 | `7b45cb31c1b94f78b7d9b6745c33a544f5426d87df779197eb64cc38a198b001` |
| `icon-192.png` | 6990 | `530e8598c7367a2ee3999250cd00962651cd995e536c3e1520e461ec2e0d650b` |
| `icon-512.png` | 19251 | `9f1074d82139629dc9350ba12c0fb21f79ea78295a5c85bc0466a44c72b4ba76` |
| `icon-maskable-512.png` | 9612 | `e1fd5502743b542a488dd22d3d4fb3144d9e6814bf20c5eaeb20224f94790bba` |
| `icon.svg` | 269 | `d3dce0051a47d063a496849024190072fb3e2868c8d025a949760945456786b1` |
| `index.html` | 666 | `2aad309a5248c2d36b275f1ca481b6a32bce22605b5ea4b4940eb00c3b174393` |
| `manifest.webmanifest` | 796 | `7949f25cca67f0956a0d81e9561d1d5d1888765b56c12eb90943613fea2d122b` |
| `sw.js` | 3380 | `0f1abaa6dc78958aaf69e40a999e53682a3257ad45f8c8db7bbc791b60d510c2` |

## Prüfungen (Last: Load 50 bis 190, Läufe nacheinander)

| Befehl | Ergebnis |
| --- | --- |
| `python3 scripts/build-web.py` zweimal frisch | je 17 Dateien; Hashliste aller `static/`-Dateien plus Manifest identisch (`cmp`) |
| `build-web.py --check`, `python3 -B scripts/test-build-web.py` | Verified 17 embedded assets; OK |
| `go test -race ./...`, `go vet ./...`, `gofmt -l .` | ok |
| Vertragswächter (sechs vorhandene `check-*-contract.py`: table, mutation, insert, structure, create, overview), `check-docs.py` | alle bestanden; 487 Dateien 0 Fehler |
| `scripts/test-web-integration.py` (Docker, PostgreSQL TLS, Compose, Playwright) | Exit 0; 13 von 13 Browserfälle (SQLite und PostgreSQL), danach `PWA embedded check passed.` |
| `scripts/test-deployment.py` (Compose, Swarm) | Exit 0, Ressourcen entfernt |
| `check-pwa-embedded.mjs` | im Integrationslauf bestanden (Worker aktiv/Scope, Installierbarkeitsfehler leer, Offline-Shell, API nie aus Cache, CSP, Header) |

## Einschränkungen

- Es gibt keine Playwright-Fälle in `tests/web-runtime` für die neuen Oberflächen (Tabelle anlegen, Katalog/Übersicht); die 13
  Fälle sind die bisherigen. Sie belegen Nichtregression, nicht die neuen Funktionen gegen den Go-Server.
- Es existieren sechs, nicht sieben, Vertragswächter-Skripte im Repo.
- Kein Lighthouse; W-P03-Review-Befunde 2 bis 5 und der nicht unterstützte Unterpfadbetrieb bleiben bestehen.
- Bytegleichheit gilt für dieselbe lokale Plattform/Toolchain; der Neubau holt den Commit aus GitHub.
