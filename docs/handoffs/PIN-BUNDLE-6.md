# PIN-BUNDLE-6 Pin auf Web-Branch release/pin-bundle6 (3e422dd)

Stand 27.10.2026. Branch `release/pin-bundle6` (Basis Backend-`main` 92ec417 = W-P20-Merge a40b20d plus Lizenzdoku; Verträge v1..v24), nicht nach main gemergt. Folgt auf Bündel 5.

## Nachtrag nach Review (Neubau auf 3e422dd)

Web-Autor setzte die Review-Auflagen um: `feat/w-p20-web` = **`3e422ddd71bd32219f062c39adda468326c59584`** (f163862 ist Vorgänger, kein History-Rewrite). Web `release/pin-bundle6` per Fast-Forward darauf. `npm_lock_sha256` und alle Verträge unverändert
(Prüfung: Hash und `git diff` über `src/lib/contract`, `package-lock.json`, Notices leer). Lock, Dockerfile-Label, LIZENZEN, WEBBUILD auf 3e422dd. `review/pin-bundle6` (0975e3c: Review-Bericht `PIN-BUNDLE-6-review.md` und Differenztest
`internal/configstore/pin6_differential_test.go`) gemergt. Die Zahlen unten unter "Ergebnis" und "Prüfungen" beschreiben den Zwischenstand f163862; maßgeblich ist dieser Nachtrag.

- **Neubau:** 53 Dateien, zwei frische Neubauten identisch (`diff -r`, `cmp`), `--check` bestanden, `test-build-web.py` 26 Tests OK (inkl. `test_manifest_source_of_the_current_pin_matches_the_committed_manifest`).
- **Precache: 5 145 226 Bytes (4,91 MiB)** ohne `sw.js`, davon 4 956 303 unter `assets/`, 42 Chunks (Grenzen 8 MiB / 64).
- **Prüfungen:** `go test -race -count=1 ./...` ok, gofmt/vet leer, 23 `check-*-contract.py` und `check-docs.py` ok, `test-web-integration.py` 13 von 13 + PWA-Check, `test-deployment.py` Exit 0 (Compose, Swarm).
- **Echtläufe `DBM_EMBEDDED=1`** (Backend e761b2e, Web-Checkout 3e422dd): tables 10/10, indexes, programs, search, sql-batch, import, export-formats, partitions, server-admin, sql-editor, config-store: alle Tests grün.
  config-store (jetzt 9 Tests inkl. `pin6-appearance`): 11 Läufe, in allen 9 von 9 grün; fünf Läufe in Serie ohne Fehler und mit Exit 0. Bei sechs früheren Läufen endeten zwei (und je einmal search und export-formats) trotz grüner Tests mit Exit 1 durch
  Umgebung: `Directory not empty` beim Aufräumen von `node_modules/.cache/...` bzw. fehlendes PyYAML im Standard-`python3` (nur `python3.14` hat es; export-formats mit `python3.14` neu: Exit 0, search neu: Exit 0).
- **Umgebung:** Colima hat inzwischen keinen Zugriff mehr auf `~/Documents` (macOS-Berechtigung): Bind-Mounts der Skripte `test-web-integration.py`/`test-deployment.py` scheiterten im Worktree ("operation not permitted"). Beide liefen daher aus einer
  Kopie des Worktrees (ohne `.git`) in einem Verzeichnis unter dem Home außerhalb von Documents (danach gelöscht); Inhalt identisch zum Commit.

## Toolchain-Anhebung

Der Lock pinnte node 26.5.0 / npm 11.17.0; auf dem Rechner sind inzwischen **node 26.12.1 / npm 11.20.0** installiert (26.5.0 nicht mehr verfügbar, ein Download wurde nicht erlaubt). Auf ausdrückliche Entscheidung des Leads bewusst angehoben:
`web-build.lock.json` (toolchain), `internal/webui/assets-manifest.json` (Quelle), `docs/WEBBUILD.md`, `EINRICHTUNG.md`, `docs/PWA-PLAN.md`, `site/package.json` (engines), `site/package-lock.json`, `site/README.md`, sowie
`.github/workflows/pages.yml` und `web-runtime.yml` (`node-version`). Weitere Workflows oder Dockerfiles pinnen Node nicht (Node-Stage gibt es nicht). Historische Handoffs (`PIN-BUNDLE-1..5`) bleiben unverändert.
**Nicht geprüft:** ob `actions/setup-node` die Version 26.12.1 auflöst (kein Netzzugriff); sie ist eine reguläre Node-Release und sollte auflösbar sein; der erste CI-Lauf zeigt es. `site/` wurde nicht neu gebaut oder getestet.
**Nachweis der Toolchain-Unabhängigkeit:** Der alte Pin f163862 wurde mit node 26.12.1 / npm 11.20.0 neu gebaut; `internal/webui/static` ist byte-identisch (`diff -r`) zum mit 26.5.0 gebauten Stand (Commit 802fa6a). Der Bytewechsel gegenüber f163862 beim neuen Stand
kommt deshalb ausschließlich vom neuen Web-Commit 3e422dd.

## Ergebnis

- **Web-Branch:** `release/pin-bundle6` im Web-Repo = `feat/w-p20-web` (enthält `review/w-p20-web` und Web-main `1145be4` mit CodeMirror W-P16), gepusht. **Lock-Commit:
  `f163862cfe8c85c4982f4709ae4c973fac17dc9c`**. `npm_lock_sha256` neu: `970dd5d2559a9c9fb283a8199d857ccd14f965b738c00403160412679f4c4ff3` (gegen `package-lock.json` des Commits geprüft).
- **Lock:** 24 Verträge; neu `config-store` 0.24.0 (letzter Eintrag nach `reasons` 0.23). Webkopie `src/lib/contract/v24/api-v0.24.0.json` ist byte-identisch zu `contracts/api-v0.24.0.json`
  (`cmp`, SHA-256 im Lock). Der Build prüft die Webkopie gegen den Lock.
- **Neubau:** 53 Dateien (52 ohne `sw.js`), davon 42 Chunks unter `assets/`. `build-web.py` zweimal frisch: `static/` identisch (`diff -r`), Manifest bytegleich (`cmp`), `--check` bestanden.
  Angepasst: Dockerfile-Label, `LIZENZEN.md` (Web-Commit-Verweis des W-P16-Eintrags auf f163862, als gepinnt/eingebettet markiert; 15 CodeMirror-Pakete gegen `package-lock.json` des Pins
  und die mitgelieferten `THIRD-PARTY-NOTICES.txt` geprüft: Versionen gleich, alle genannt), `WEBBUILD.md` (Abschnitt Pin Bündel 6), `test-build-web.py` (24 Verträge, letzter Eintrag config-store).
- **Precache: 5 143 647 Bytes (4,91 MiB)** ohne `sw.js` (davon 4 954 724 unter `assets/`, 42 Chunks). Bündel 5: 4,32 MiB, 39 Chunks. Grenzen eingehalten:
  `MAX_PRECACHE_BYTES` 8 MiB (39 % Reserve), `MAX_CHUNKS` 64 (42 von 64).
- **Angepasste Backend-Specs:** `tests/web-runtime/runtime.spec.ts` und `tables.spec.ts` lasen den SQL-Editor mit `toHaveValue('')`. Der CodeMirror-Editor ist ein `contenteditable`-Textbox
  (kein Input), und im leeren Zustand zeigt er den Platzhalter. Jetzt `toHaveText(/^(-- Schreibe hier deine SQL-Abfrage\.[\s\S]*)?$/)` (Platzhalter oder leer). Kein Produktfehler;
  `fill` funktioniert unverändert. `test-web-integration.py` war davor 11 von 13, danach 13 von 13.

## Prüfungen (Worktree `.worktrees/release/pin-bundle6`)

| Befehl | Ergebnis |
| --- | --- |
| `build-web.py` zweimal, `--check`, `test-build-web.py` | identisch, bestanden, 26 Tests OK |
| `go test -race -count=1 ./...` | alle Pakete ok (Exit 0) |
| `gofmt -l .`, `go vet ./...` | leer, ok |
| 23 `check-*-contract.py` (inkl. config-store) | alle bestanden; `check-extended-export-files.py` ist kein Vertragscheck und braucht ein Argument, nicht ausgeführt |
| `check-docs.py` | bestanden nach Anlage dieses Handoffs |
| `test-web-integration.py` | Exit 0, 13 von 13 (SQLite + PostgreSQL TLS), `PWA embedded check passed.` |
| `test-deployment.py` | Exit 0 (Compose, Swarm), Aufräumen gemeldet |

## Echtläufe gegen die EINGEBETTETEN Assets (`DBM_EMBEDDED=1`, Backend `8705fbe`, Web-Checkout f163862)

Alle bestanden: tables (10 von 10), indexes, programs, search, sql-batch, import (6), export-formats, partitions, server-admin (SQLite/MySQL/MariaDB/PostgreSQL), sql-editor (5, SQLite + PostgreSQL),
config-store (7 von 7, Wegwerf-MariaDB-Config-Store, zweites Backend ohne Server-State).
**config-store: ein roter Erstlauf**, danach vier grüne eingebettete Läufe und ein grüner Preview-Lauf. Der rote Lauf: Test "theme and language are stored on the server" (`tests/real/config-store.spec.ts:244`)
erhielt innerhalb der 10-s-Poll-Frist keine `appearance`-Einstellung vom Server; die drei nachfolgenden Tests liefen nicht. Vermutlich Timing (400-ms-Pause plus Poll), nicht eingegrenzt; Wiederholung empfohlen im Review.

## Einschränkungen

- MySQL/MariaDB-Echtläufe für Import, Bulk, Export-Formate und Partitionen fehlen weiter (nur server-admin deckt sie ab). Kein Lighthouse, kein Unterpfadbetrieb, Bytegleichheit nur lokale Plattform/Toolchain.
- Port 4179 (fremder Prozess) und fremde Container blieben unangetastet; eigene Container sind aufgeräumt.
- Unabhängiges Review dieses Bündels steht aus.

## Eingebettete Dateien

Inventar mit SHA-256: `internal/webui/assets-manifest.json` (53 Dateien).
