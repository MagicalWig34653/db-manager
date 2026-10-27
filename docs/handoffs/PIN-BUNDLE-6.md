# PIN-BUNDLE-6 Pin auf Web-Branch release/pin-bundle6 (f163862)

Stand 27.10.2026. Branch `release/pin-bundle6` (Basis Backend-`main` 92ec417 = W-P20-Merge a40b20d plus Lizenzdoku; Verträge v1..v24), nicht nach main gemergt. Folgt auf Bündel 5.

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
