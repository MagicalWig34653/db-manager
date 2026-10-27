# PIN-BUNDLE-2 Review (unabhängig)

Stand 24.10.2026. Reviewer: unabhängig (nicht Autor). Geprüft: Root `release/pin-bundle2` (40d3d8e, Basis 5921a9d) gegen
Web-main 48c2f67; Probemerge mit Root-main b74d10e. Review-Worktrees `review/pin-bundle2` (Root, Web). Autorenbranch
unverändert, nichts gemergt. Last Load 38 bis 200, Läufe nacheinander, Docker nur Wegwerf (aufgeräumt).

## Urteil

**Freigabe mit Auflagen (Approve, keine Blocker).** Pin, Reproduzierbarkeit, Chunks im Service Worker, Reviewer-Fixes aus
Bündel 1 und die neuen Oberflächen gegen die eingebetteten Assets sind belegt. Vor dem Merge nachziehen (nicht
blockierend, aber empfohlen): Befund 1 (Doku widersprüchlich) und Befund 2 (Neubauprüfung schwach).

## Befunde

| # | Schwere | Befund | Beleg | Fix |
| --- | --- | --- | --- | --- |
| 1 | Mittel | Dokumentation widerspricht dem neuen Pin: `docs/WEBBUILD.md` Z. 11 (Kopfabsatz „pinnt auf 8ef738d“), Z. 134 und Z. 183 („API0.6 … nicht konsumiert / bleibt bewusst ungepinnt“, jetzt gepinnt), `docs/LIZENZEN.md` Z. 66 (verlinkt Quelle 8ef738d; in Bündel 1 noch aktualisiert, hier wieder alt, weil Basis 5921a9d). | `git grep 8ef738d`, Abschnitt „Pin Bündel 2“ in WEBBUILD ist korrekt, die älteren Absätze nicht. | Aktuellen Pin in Kopfabsatz und LIZENZEN nennen, Z. 134/183 als historisch kennzeichnen oder anpassen. |
| 2 | Mittel | Lockerung der Neubauprüfung ist als Kompromiss vertretbar, aber zu schwach: `len(non-PWA files) < 5` ist praktisch immer wahr (Pflicht sind schon `index.html`, zwei Lizenzdateien und ein `.js`), prüft also kaum noch etwas. Der Quelllink wird in „irgendeinem“ JS gesucht. Verwaiste oder zusätzlich eingeschleuste Chunks (passendes Namensmuster) würden akzeptiert; es gibt keinen Bezug zwischen `index.html`, Chunks und der Precache-Liste in `sw.js`. | `scripts/build-web.py` `rebuild`, `artifact`. | Siehe Empfehlung unten. |
| 3 | Niedrig | `check-pwa-embedded.mjs` hat jetzt echte Assertions (belegt, Exit 1), prüft aber nicht, dass alle `assets/*`-Chunks im Worker-Cache liegen und offline erreichbar sind (nur `index.html` und „keine API-Pfade“). | Skript. | Assertion: Menge `caches` (ohne Pfade `/api`) gleich Menge der Manifest-Dateien außer Lizenztexte/`sw.js`. |
| 4 | Niedrig | Flake im Browserlauf: `export.spec.ts` PostgreSQL scheiterte einmal (`requests` 20 erwartet, 23 erhalten, drei `/api/`-Anfragen nach `networkidle`), im Wiederholungslauf 13 von 13. Ursache nicht geklärt (Last 100 bis 140 oder späte Anfragen der Shell); der Test ist anfällig, bei Wiederkehr untersuchen. | zwei Läufe `test-web-integration.py`. | Beobachten; ggf. Anfrageliste statt Zähler protokollieren. |
| 5 | Niedrig | Offene Altpunkte: Unterpfadbetrieb nicht unterstützt (dokumentiert), W-P03-Befunde 2 bis 5, kein Lighthouse. | Docs. | wie dort. |

## (2) Bewertung der Lockerung und Empfehlung

Vertretbar, weil `inventory`/`artifact()` weiterhin für **alle** Dateien einschließlich Chunks gelten: nur erlaubte Namen
(`assets/<name>-<hash>.js|css`, Wurzel- und PWA-Dateien), kein `sourceMappingURL`, kein absoluter lokaler Benutzerpfad, keine Symlinks. Das
Bundle enthält nur die Quell-URLs `github.com/MagicalWig34653/...@48c2f67`, keine Sourcemaps, keine privaten Pfade (selbst
nachgegrept). Lizenzdateien werden bytegleich zur Webquelle geprüft. Aber die Zahlprüfung ist wertlos geworden.
Empfehlung (konkret, ersetzt die Zahl):
1. `index.html` parsen: genau ein Entry-`<script type=module>` und ein Entry-Stylesheet, beide müssen existieren; der
   Quelllink muss in **diesem** Entry-JS stehen.
2. Jede Datei unter `assets/` muss aus dem Entry-JS/-CSS (transitiv per Dateiname) referenziert sein (keine Waisen), und
   die Zahl der Chunks hat eine feste Obergrenze im Skript (z. B. `MAX_CHUNKS = 12`).
3. Die `SHELL`-Liste in `sw.js` (JSON im Quelltext) muss exakt der Menge `index.html` + PWA-Bilder/Manifest + `assets/*` entsprechen.
Eine explizite Dateiliste mit Hashes im Lock ist nicht nötig: der Build ist deterministisch und `assets-manifest.json`
mit `--check` pinnt Namen und SHA-256 bereits; sie würde nur Pflege je Pin verursachen.
Alter Pin: nachgewiesen reproduzierbar. Lock von 5921a9d eingesetzt, `build-web.py` baute 5 Assets, `Verified 5 embedded
assets`, alle Dateien bytegleich zu 5921a9d (`cmp`); danach zurückgesetzt.

## Ausgeführte Prüfungen

| Prüfung | Ergebnis |
| --- | --- |
| Pin: Lock-Commit = `git rev-parse main` Web (48c2f67…), `npm_lock_sha256` = SHA-256 `package-lock.json` des Commits, Verträge create 0.6.0 `b08b6aba…` und overview 0.7.0 `c6fd8c91…` identisch zwischen Root `contracts/` und Web `src/lib/contract/v6/`, `v7/`; 0.1 bis 0.5 im Diff unverändert | ok |
| `build-web.py` zweimal frisch | je 17 Dateien, `git status` sauber (bytegleich zum eingebetteten Stand) |
| `build-web.py --check`, `test-build-web.py` | Verified 17 embedded assets; OK (inkl. Test Dockerfile-Label = Lock) |
| Probemerge `main` (b74d10e) in Review-Worktree | konfliktfrei; danach `--check` ok, `check-docs` 588 Dateien 0 Fehler, `go vet`, `go build` ok |
| `go test -race ./...` (Probemerge-Stand), `gofmt -l .` | alles ok bis auf `sqlitesandbox` `TestCrashRecovery` (child timeout bei Load 190); isoliert wiederholt ok → Lastflake |
| Sieben Wächter `check-{table,mutation,insert,structure,create,overview,object-ops}-contract.py` (Probemerge-Stand) | alle bestanden |
| `test-web-integration.py` (Docker, PostgreSQL TLS, Compose, 13 Playwright-Fälle, PWA-Check) | Lauf 1: 12/13 (Flake Befund 4); Lauf 2: 13/13 + `PWA embedded check passed.` |
| `test-deployment.py` (Compose, Swarm) | bestanden, Ressourcen entfernt |
| Web `scripts/test-real-create.py` und `test-real-overview.py` **gegen die eingebetteten Assets** (Kopien im Web-Review-Worktree: Basis-URL = Go-Server des Branches statt Vite-Preview, `DBM_BACKEND_REPO` = Review-Worktree, Wegwerf-SQLite und PostgreSQL 18.6 TLS verify-full) | Tabelle anlegen: 2/2 bestanden; Server-Info, Datenbankliste, Objektübersicht, exakter Count, Definition: 2/2 bestanden (je SQLite und PostgreSQL) |
| Eigene Browserprobe gegen den Go-Server (online, dann offline, Routen `#/server`, `#/tables/new`, `#/overview`) | alle 6 Asset-Dateien (2 Entry, 4 Chunks) werden beim Start geladen und liegen im Cache `db-manager-shell-48c2f67…` (`SHELL` in `sw.js` enthält alle); offline lädt die Shell mit allen Chunks (HTTP 200 aus dem Cache), nur `/healthz` schlägt fehl, 0 CSP-Verstöße; alle Chunks mit `public, max-age=31536000, immutable` und richtigem MIME |
| `check-pwa-embedded.mjs`: grün; Fehler provoziert (Proxy setzt `Service-Worker-Allowed` bzw. `Cache-Control` am Manifest) | je `AssertionError`, Exit 1 |
| Reviewer-Fixes Bündel 1: Dockerfile-Label = 48c2f67 und per Test erzwungen; `verify_http` `immutable` nur unter `assets/`; `TestBundledAssetsMatchManifest` prüft die PWA-Dateien (Status, MIME, no-store, nosniff) | wirksam (Läufe grün) |
| Quelltext-Lecks: `sourceMappingURL`, lokale Benutzerpfade in `static/`; externe URLs in den Bundles | keine Treffer; nur w3.org-Namensräume, svelte.dev-Fehlerlinks (Text), Projekt-/Quelllinks; `connect-src 'self'` |

Nicht ausgeführt: Lighthouse.

## Nachreview (Autorenstand ef792ad, in den Review-Branch gemergt)

Geprüft: `git diff 40d3d8e..origin/release/pin-bundle2` (Root-main b74d10e eingemergt, Assets unverändert).

**Urteil: Freigabe.** Die Auflagen 1 bis 3 sind erfüllt, die Flake-Änderung ist plausibel. Keine Blocker, keine neuen Mittleren Befunde.

| Prüfung | Ergebnis |
| --- | --- |
| (a) `verify_build` mit sechs provozierten Fehlern (Aufruf mit Kopie von `static/`) | verwaiste Datei: `Unreferenced asset chunks`; Chunk fehlt in `SHELL`: `shell list differs`; Quelllink fehlt: abgelehnt; Quelllink nur in einem Chunk statt im Entry: abgelehnt; 19 Chunks: `Too many asset chunks: 19 > 12`; zweites Entry-Skript: abgelehnt. Alle als `ValueError`, den `main()` mit Exit 1 meldet. Unverändertes Inventar besteht. |
| (a) alter Pin 8ef738d | Lock aus 5921a9d eingesetzt, `build-web.py` ok (5 Assets), `index.html`, Assets, Lizenztexte und `assets-manifest.json` bytegleich zu 5921a9d; danach zurückgesetzt, `--check` mit 17 Dateien ok |
| (b) `check-pwa-embedded.mjs` | grün; Fehler provoziert (Proxy lässt `cache.put` für einen Chunk aus): `worker cache differs from SHELL`, Exit 1. Der Offline-Abruf aller Shell-Dateien (HTTP 200) ist im grünen Lauf Teil der Assertions |
| (c) `export.spec.ts` | Begründung plausibel: Die Tabellenansicht stößt asynchron `POST .../insert-eligibility` an (zuvor 3 unerwartete Anfragen nach `networkidle`). Der Test sammelt jetzt Methode und Pfad, wartet auf `networkidle` und fordert `[]` neue API-Anfragen während der Downloads; die Aussagekraft bleibt, nur dieser eine schreibfreie Prüfpfad ist ausgenommen |
| (d) Doku | `WEBBUILD.md` und `LIZENZEN.md` nennen 48c2f67; die verbleibenden Nennungen von 8ef738d und „ungepinnt“ sind ausdrücklich als historisch gekennzeichnet bzw. bedingt formuliert; `check-docs` 591 Dateien 0 Fehler |
| Läufe | `build-web.py --check` (17), `test-build-web.py` OK, `go test -race ./...` ohne Fehler, `go vet`, `gofmt` leer, sieben Wächter ok, `test-web-integration.py` Exit 0 (13 von 13, `PWA embedded check passed.`), `test-deployment.py` (Compose, Swarm) ok, Ressourcen entfernt |

Hinweise (niedrig, nicht blockierend): Der Ausschluss in `export.spec.ts` ist ein Suffixvergleich (`endsWith('/insert-eligibility')`); enger wäre der exakte Pfad `/api/v4/tables/insert-eligibility`. Der Cache-gegen-`SHELL`-Vergleich im PWA-Check ist selbstbezüglich (der Worker legt seine eigene Liste an); dass `SHELL` dem Inventar entspricht, stellt `verify_build` sicher. Beides zusammen ist ausreichend.
