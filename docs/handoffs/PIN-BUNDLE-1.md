# PIN-BUNDLE-1 Pin auf Web-main 6cb9613

Stand 24.10.2026. Branch `release/pin-bundle1` (Basis Root-main e7bff98), nicht nach main gemergt.

## Ergebnis

- `web-build.lock.json` (Schema 2): `commit` auf `6cb9613eebcca3196e115f30e67c9be1c2f4b943` (Web-main mit W-P02 und W-P03).
  npm-Lock-SHA `bac4b32ba0548428186f20b1d6382f3554b1a477c0fb89957b123820495b15c6` unverändert (Lockfile im Web unverändert,
  vom Neubau bestätigt), Toolchain Node 26.5.0 / npm 11.17.0. Vertragsliste unverändert die fünf Bestandsverträge.
- API 0.6 nicht gepinnt: Web-main enthält keine Kopie (kein `src/lib/contract/v6/`), also kein SHA-Vergleich möglich
  (Root `contracts/api-v0.6.0.json` SHA-256 `b08b6aba858f72a3bad1fce2933e487eed017f5e35b4690cfd9fae3a590a6887`).
- Frischer Neubau `python3 scripts/build-web.py` ersetzt die eingebetteten Assets: 13 Dateien inkl. aller acht PWA-Dateien;
  `assets-manifest.json` zeigt Commit 6cb9613. Das Manifest-`source` bleibt im flachen Format.
- `TestBundledAssetsMatchManifest` verlangt jetzt alle `pwaFiles` im Manifest und prüft Status, MIME, `no-store`, `nosniff`.
- Neu: `scripts/check-pwa-embedded.mjs` (Browserprobe gegen einen laufenden Go-Server).

## Eingebettete Dateien

| Pfad | Bytes | SHA-256 |
| --- | --- | --- |
| `LICENSE.txt` | 34523 | `0d96a4ff68ad6d4b6f1f30f713b18d5184912ba8dd389f86aa7710db079abcb0` |
| `THIRD-PARTY-NOTICES.txt` | 92896 | `5757e0b3179e381d4393f2e51784a7109c00b3276546b8fcf551cd8f1309135e` |
| `apple-touch-icon.png` | 3664 | `16d0cdb88277d760d947ea00749dd912057995e2c50db8bc5bc985408892081d` |
| `assets/index-BoVDfYJP.js` | 983774 | `2c9dcad7017aa3f7075f395983bb3356b6a6c3c270da38fc490976a9ff8d29a8` |
| `assets/index-D-xDhZ-6.css` | 46204 | `440011f3b3f6da5c5ba62fdecf4819eb21568013c2ae4d5df489e674128bd719` |
| `favicon-32.png` | 1196 | `7b45cb31c1b94f78b7d9b6745c33a544f5426d87df779197eb64cc38a198b001` |
| `icon-192.png` | 6990 | `530e8598c7367a2ee3999250cd00962651cd995e536c3e1520e461ec2e0d650b` |
| `icon-512.png` | 19251 | `9f1074d82139629dc9350ba12c0fb21f79ea78295a5c85bc0466a44c72b4ba76` |
| `icon-maskable-512.png` | 9612 | `e1fd5502743b542a488dd22d3d4fb3144d9e6814bf20c5eaeb20224f94790bba` |
| `icon.svg` | 269 | `d3dce0051a47d063a496849024190072fb3e2868c8d025a949760945456786b1` |
| `index.html` | 666 | `318f5b4caff30556d036371a02554b30a4361851fdf2b60db494057890cc2ead` |
| `manifest.webmanifest` | 796 | `7949f25cca67f0956a0d81e9561d1d5d1888765b56c12eb90943613fea2d122b` |
| `sw.js` | 3262 | `5d56adc4fae604b8544796eb25e8a2fd778ca24a24951974b32d634f7d548798` |

## Prüfungen

| Befehl | Ergebnis |
| --- | --- |
| `python3 scripts/build-web.py` zweimal (frisch, nacheinander) | je 13 Dateien; `shasum -a 256` aller Dateien unter `static/` plus Manifest identisch (`cmp` IDENTICAL) |
| `python3 scripts/build-web.py --check` | Verified 13 embedded assets and pinned provenance |
| `python3 -B scripts/test-build-web.py` | OK |
| `go test -race ./...`, `go vet ./...`, `gofmt -l .` | ok, keine Ausgabe von Fehlern |
| `python3 scripts/check-docs.py` | 0 Fehler |
| `scripts/check-{table,mutation,insert,structure,create}-contract.py` | alle bestanden |
| Browser: `node scripts/check-pwa-embedded.mjs` gegen den echten `cmd/db-manager` (Chromium, Token-Datei, SQLite-Datenverzeichnis) | siehe unten |

Browserbefunde: Service Worker aktiv, Scope `/`, Seite kontrolliert; CDP `Page.getInstallabilityErrors` leer, Manifest ohne
Fehler; Cache `db-manager-shell-6cb9613...` enthält nur Shell-Dateien (kein `/api`, `/healthz`); offline schlagen
`/healthz` und `/api/...` mit Netzwerkfehler fehl (nie aus dem Cache), online 200; Neustart offline zeigt die deutsche Shell;
keine CSP-Verletzungen; alle PWA-Dateien 200 mit korrektem MIME, `no-store`, `nosniff`, ohne `Service-Worker-Allowed`;
CSP enthält `manifest-src 'self'; worker-src 'self'`.

## Nicht ausgeführt / Einschränkungen

- Keine PostgreSQL-/Docker-/Compose-Läufe und nicht `scripts/test-web-integration.py` (zieht Docker-Image, Postgres, npm ci;
  bei hoher Hostlast bewusst ausgelassen). Der Browserpfad lief nur gegen SQLite-Server ohne Fachworkflows (kein Playwright-Suite
  `tests/web-runtime`); das ist ausdrücklich nachzuholen.
- Kein Lighthouse. Installierbarkeit nur per CDP auf `127.0.0.1` (localhost-Ausnahme).
- W-P03-Review Befund 1 (absolute Pfade in `index.html`, Unterpfadbetrieb): nur dokumentiert (WEBBUILD), nicht geändert, da die
  Build-Quelle im Web liegt und ein Fix den Pin ändern würde. Befunde 2 bis 5 unverändert offen (niedrig).
- Der Neubau bezieht den Webcommit aus GitHub; Bytegleichheit gilt für dieselbe lokale Plattform und Toolchain.
