# PIN-BUNDLE-3 Pin auf Web-main c14271e

Stand 25.10.2026. Branch `release/pin-bundle3` (Basis Root-main c331c64), nicht nach main gemergt. Folgt auf Bündel 2.

## Ergebnis

- Lock: commit `c14271ec12bfb7940253787c2f84aeb65cc4252e`, `npm_lock_sha256` `bac4b32ba0548428186f20b1d6382f3554b1a477c0fb89957b123820495b15c6`
  (unverändert, gegen `package-lock.json` des Commits geprüft). Neue Verträge (Root und Webkopie `src/lib/contract/vN/` SHA-identisch):
  `object-ops` 0.8.0 `74aa39ed06311f2c45afc274a2412a0ac30912e3a5683ec96b812fd473a28264`,
  `columns` 0.9.0 `508b9951d102ec44bf4cafc9fe444f7266b31f898d9e91c887f4bef39735e3f8`,
  `export` 0.11.0 `b3bcc38b44b020335151aead207e281f9c54bdc7463d6fb1ddd7a6cc22396620`. 0.1 bis 0.7 unverändert.
- **Nicht gepinnt:** 0.10.0 (maintenance) und 0.12.0 (sqlbatch). Das Web enthält keine Kopie (`v10/`, `v12/` fehlen); Root-Verträge
  existieren, werden von diesem Webstand nicht konsumiert.
- Neubau: 26 eingebettete Dateien. Der Webbuild hat 15 Dateien unter `assets/` (zuvor 6). `MAX_CHUNKS` wurde **von 12 auf 24
  angehoben** (Begründung: lazy Feature-Chunks je JS und CSS wachsen mit jeder Welle; die Waisenprüfung und die SHELL-Gleichheit
  bleiben die eigentliche Absicherung). Kommentar im Skript, Doku in WEBBUILD.
- Dockerfile-Label auf c14271e (Test erzwingt es); WEBBUILD, LIZENZEN, STATUS aktualisiert.

## Eingebettete Dateien

| Pfad | Bytes | SHA-256 |
| --- | --- | --- |
| `LICENSE.txt` | 34523 | `0d96a4ff68ad6d4b6f1f30f713b18d5184912ba8dd389f86aa7710db079abcb0` |
| `THIRD-PARTY-NOTICES.txt` | 92896 | `5757e0b3179e381d4393f2e51784a7109c00b3276546b8fcf551cd8f1309135e` |
| `apple-touch-icon.png` | 3664 | `16d0cdb88277d760d947ea00749dd912057995e2c50db8bc5bc985408892081d` |
| `assets/PlanPreview-DIdf5lVd.css` | 532 | `d666349262672b2949e94fd4b85df4ec6b6e97cb0042381c3a5350426d4419b6` |
| `assets/PlanPreview-DUiKGtiA.js` | 3792 | `06a754b3db2e1ff44780e0e299bd30bab846d204c70e670ecf1b20a30a07e986` |
| `assets/circle-check-CDGqhoW1.js` | 331 | `91bcb2db867ccdd9b92bd47392a8b1f137dab1937dacf99fb653b7fcc328cedc` |
| `assets/feature-BPqPrbnZ.css` | 2053 | `79fd88e5e2782cdaf5677b7f56d593244e7499ec5d3783e78e0b984e027bc9a3` |
| `assets/feature-BR5z3Dr0.js` | 311551 | `63154e5fc3c9ff03e2dfb2580220621e20bfaa7d386e516bfecf412978eeba05` |
| `assets/feature-BcixFSIF.css` | 2792 | `943088577fcb657325fc54578ebd8561684244bad12a49eae94a03205e0afa9c` |
| `assets/feature-Cc4YJa3q.css` | 1875 | `1268f91a3283fc28ba64d73a5e6cd3b38cffd3e31e27a3238ac5c1d386f3e86d` |
| `assets/feature-CzdzV0ta.css` | 1570 | `4599f0965b6204e390ac4284ab5eac9ab4dc222f4996d5558df98b23929b1693` |
| `assets/feature-D2nHilXW.css` | 1917 | `3677b26565ec5cb5acc15900cc1a260afa2194ef2a61277cca9f2017d6ef160a` |
| `assets/feature-DY8XpTED.js` | 474139 | `3acf1e7cb97bd5d11e4189cd5ec43194bc6fcad8bf6088029339f4a8e35552d9` |
| `assets/feature-DZ6C2MWM.js` | 758526 | `bc9b2310ed703fd5d4f3018e67b4284303fcfbb2f7cbcf685f1c89a1b23bf7e0` |
| `assets/feature-DzT_VAM5.js` | 1158588 | `54c80f9b66152e9e5c2781dfc45332ea7ec9fc7a1b20980f6f6af479b6bff01e` |
| `assets/feature-QvgXdtKz.js` | 936502 | `70a5f4ccb2661fbadbae13c89e33170bd930c5bba0bd6fcffcc69bbc08db397d` |
| `assets/index-BnxbHZ7e.js` | 1026314 | `2716c7a4daf71d372900746c0c308fc1479377636413cfd6da781ac1e94abf52` |
| `assets/index-Dp8BW1VF.css` | 48392 | `ecbd7b2c776799ca6225317d46f557e48ca2ef30810eb421793105ed8b4a2443` |
| `favicon-32.png` | 1196 | `7b45cb31c1b94f78b7d9b6745c33a544f5426d87df779197eb64cc38a198b001` |
| `icon-192.png` | 6990 | `530e8598c7367a2ee3999250cd00962651cd995e536c3e1520e461ec2e0d650b` |
| `icon-512.png` | 19251 | `9f1074d82139629dc9350ba12c0fb21f79ea78295a5c85bc0466a44c72b4ba76` |
| `icon-maskable-512.png` | 9612 | `e1fd5502743b542a488dd22d3d4fb3144d9e6814bf20c5eaeb20224f94790bba` |
| `icon.svg` | 269 | `d3dce0051a47d063a496849024190072fb3e2868c8d025a949760945456786b1` |
| `index.html` | 666 | `c2f596f6ddf086c3ae2ba62aedfe89227c04eac02c04b9207f4d92d80e366e7f` |
| `manifest.webmanifest` | 796 | `7949f25cca67f0956a0d81e9561d1d5d1888765b56c12eb90943613fea2d122b` |
| `sw.js` | 3658 | `628ff73c185d563660d3e7e82908a8fee2a4691ebc7dc9fb3e1dd96082e89597` |

## Prüfungen (Last 100 bis 270, nacheinander)

| Befehl | Ergebnis |
| --- | --- |
| `build-web.py` zweimal frisch | je 26 Dateien, Hashliste identisch (`cmp`) |
| `build-web.py --check`, `test-build-web.py` | Verified 26 embedded assets; OK |
| `go test -race ./...`, `go vet ./...`, `gofmt -l .` | ok |
| elf `check-*-contract.py`, `check-docs.py` | alle bestanden |
| `test-web-integration.py` | Exit 0, 13/13 Browserfälle SQLite+PostgreSQL, PWA-Check bestanden |
| `test-deployment.py` | Exit 0, keine Restcontainer |

## Neue Oberflächen gegen die EINGEBETTETEN Assets

Web-Hilfsbranch `helper/pin-bundle3-real` (Worktree `.worktrees/pb3` auf c14271e, nur lokal, nicht gepusht, nicht Web-main): die
Web-Skripte `scripts/test-real-*.py` so angepasst, dass kein Vite-Build/Preview läuft und die Basis-URL der Go-Server des Branches
`release/pin-bundle3` (Commit f44b842, Assets eingebettet) ist (`DBM_BACKEND_REPO`/`DBM_BACKEND_REV`), Wegwerf-SQLite und
PostgreSQL 18.6 TLS verify-full, Playwright `playwright.real.config.ts`:

| Skript | Ergebnis |
| --- | --- |
| `test-real-create.py` (Tabelle anlegen) | 2/2 SQLite+PostgreSQL |
| `test-real-overview.py` (Server-Info, Übersicht, Definition) | 2/2 |
| `test-real-object-ops.py` (Datenbank-/Tabellenoperationen) | 4/4 |
| `test-real-columns.py` (Spalten) | 2/2 |
| `test-real-export.py` (Export-Assistent, Restore-Vergleich, JSON/CSV/GZip) | bestanden SQLite und PostgreSQL |
| `test-real-tables.py` (ältere Specs `tests/real`) | **Exit 1**: 6 bestanden, 4 fehlgeschlagen: `export.spec.ts` (Button "Ergebnis herunterladen" nicht gefunden) und `structure.spec.ts` (Warten auf `/api/v5/tables/structure`) je SQLite/PostgreSQL |

Zur letzten Zeile: die älteren Specs prüfen das Vor-Shell-Layout; ihre Locatoren passen offenbar nicht mehr zur neuen Shell
(Export-Assistent ersetzt den Button). Ob sie auch gegen Vite-Preview auf Web-main scheitern, habe ich nicht geprüft (nicht
Ursache-belegt). Gleichwertige Abdeckung für Export und Struktur liefern `tests/web-runtime` (export/structure, 13/13 grün) und
`test-real-export.py`. Das ist ein offener Punkt für die Web-Seite.

## Einschränkungen

- Die angepassten Skripte liegen nur im lokalen Web-Hilfsbranch (nicht im Root-CI). `tests/web-runtime` deckt Tabelle anlegen,
  Übersicht, Objektoperationen, Spalten weiterhin nicht ab.
- Kein Lighthouse, Unterpfadbetrieb nicht unterstützt, Bytegleichheit nur lokale Plattform/Toolchain.
