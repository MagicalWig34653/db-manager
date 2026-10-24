# W-P03 Review (unabhängig)

Stand 24.10.2026. Reviewer: unabhängig (nicht Autor). Geprüft: Web `feat/w-p03-pwa` (2f273f8, Basis 0982738)
und Root `feat/w-p03-pwa-embed` (7a0b7c1, Basis 02a30db). Review-Worktrees `review/w-p03` in beiden Repos;
Autorenbranches unverändert. Hohe Hostlast (Load 111 bis 178), Läufe strikt nacheinander, `node_modules` als
`cp -cR`-Kopie, Builds nur nach Commit.

## Urteil

**Freigabe (Approve), keine Blocker, keine Hohen oder Mittleren Befunde.** Fünf niedrige Befunde/Hinweise
unten, keiner blockierend. Sicherheitsfokus Service Worker: bestanden (siehe Abschnitt 1).

## Ausgeführte Prüfungen

| Prüfung | Ergebnis |
| --- | --- |
| Web `npm run check` | 0 Fehler, 0 Warnungen |
| Web `npm run format:check`, `npm test` | ok, 9 Dateien / 50 Tests |
| Web `npm run test:csp` | 2 passed |
| Web `npm run test:browser` | 6 passed |
| Web `npm run test:pwa` | 5 passed (Manifest/Installierbarkeit per CDP, SW-Scope, Offline-Shell, API nie aus Cache, Update nach Klick) |
| Zwei frische `vite build` desselben Commits (`diff -r`) | bytegleich; `sw.js` enthält BUILD_ID = vollständiger Commit 2f273f8… |
| Probemerge Web-main (a3cefd3, mit W-P02) in Review-Worktree | konfliktfrei (danach `merge --abort`) |
| Root `go test -race ./internal/webui`, `go vet ./internal/webui` | ok |
| `python3 -B scripts/test-build-web.py` | OK (inkl. neuer PWA-Fälle) |
| `python3 scripts/build-web.py --check` | Verified 5 embedded assets and pinned provenance (Pin 8ef738d gültig) |
| `python3 scripts/check-docs.py` | 425 files, 0 errors |

Nicht ausgeführt: Lighthouse; echte Auslieferung des neuen Builds durch den Go-Server im Browser (Assets noch nicht
eingebettet, wie im Handoff vermerkt); `build-web.py` Neubau (kein Pin-Wechsel, nicht Teil dieses Pakets).

## 1. Service Worker (`src/lib/pwa/sw.template.js`)

- Nur `GET`, same-origin, ohne `Authorization`-Header; Navigation nur für exakt `/` und `index.html`, sonst
  Dateien nur bei exaktem Treffer in der vorab festgelegten Shell-Liste und leerem Query. `/api`, `/healthz`,
  `/readyz`, POST/PUT/DELETE: `respondWith` wird nie aufgerufen (Browser-Standard). Kein Navigations-Fallback auf die
  Shell für andere Pfade.
- `cache.put` nur in `install` (Shell-Liste, `cache: 'reload'`, `credentials: 'omit'`, `response.ok` Pflicht, sonst
  Install-Abbruch und alter Worker bleibt). Kein Laufzeit-Caching, daher kein Cache-Poisoning über Fetch-Pfad,
  keine opaque responses im Cache. Kein IndexedDB/localStorage im Worker oder in `register.ts`/`notices.ts`; die
  W-P02-Shell hält Sitzung/Token nur im Speicher (belegt in `session.svelte.ts`).
- Cache-Name `db-manager-shell-<Commit>`; `activate` löscht nur Präfix-Caches anderer Builds; `skipWaiting` nur per
  Nachricht `SKIP_WAITING` nach Klick; Reload nur nach Klick (`reloadRequested`). `clients.claim` beim ersten
  Install ist unkritisch. Scope `./` (Wurzel), Go sendet kein `Service-Worker-Allowed`, `updateViaCache: 'none'`.
- Navigation network-first: online nie veraltete Shell; offline nur die Shell der letzten Installation, Daten kommen
  ausschließlich aus API-Antworten und fehlen offline (Fehlermeldung bzw. Hinweis „Keine Verbindung zum Server“).
  Keine veralteten Daten als aktuell.
- Probe (`/healthz`, `no-store`, `credentials: 'omit'`) geht am Worker vorbei und liest keine Daten.

## 2. Manifest, Icons, Build

Manifest: `display: standalone`, `start_url`/`scope`/`id` `./`, `lang: de`, Icons 192/512 `any`, 512 `maskable`
(ohne Alpha, Motiv innerhalb der Safe Zone, visuell geprüft), SVG `any`; Installierbarkeitsfehler laut CDP leer.
apple-touch-icon 180x180, favicon 32x32. Provenienzprüfung (Commit-Quell-Link) greift weiter (`test:csp` grün).
Keine neue Abhängigkeit, `package-lock.json` unverändert. Ableitung der Symbole aus dem eigenen Projektsymbol.

## 3. Go-Seite und Build-Skript

Exakte `pwaFiles`-Map (8 Namen, keine Wildcards), MIME wie geplant, `no-store` (außer gehashte Assets, unverändert),
kein `Service-Worker-Allowed`, nur POST wird mit 405 abgewiesen, Pfadtraversal/`/sw.js/`/`/Sw.js`/`sw.js.map`/
`icon-unlisted.png`/`assets/sw.js` liefern 404 (Tests). CSP nur um `manifest-src 'self'; worker-src 'self'` erweitert
(Test auf exakte Zeichenkette, kein `unsafe-*` außer bestehendem `style-src-attr`). Bestehende Assets und Pin
unverändert (`web-build.lock.json`, `assets-manifest.json`, `static/` nicht im Diff). `PWA_FILES` „alle oder keine“
korrekt (`inventory`), Quellprüfung zählt nur Nicht-PWA-Dateien (5), unerwartete Namen und Sourcemap-Marker
abgelehnt. Alter Pin besteht `--check` und Tests.

## 4. Zusammenspiel mit W-P02

Keine Datei aus `src/lib/shell`, `src/lib/features`, `App.svelte`, `main.ts` im Diff gegen die Basis; Probemerge mit
W-P02-main konfliktfrei. Die Registrierung ist ein eigenes Modul (im Build in das Bundle gefaltet). Gemeinsamer
Pin: W-P03 braucht beim Neubau den Web-Commit, auf dem W-P02 und W-P03 vereint sind; `sw.js`/Build-ID ändern
sich dann mit dem Pin-Commit (erwartet).

## 5. Doku / E13

`BETRIEB.md` und `WEBBUILD.md` nennen HTTPS/localhost-Bedingung, Offline nur Shell, nie Daten, ehrlich und
passend zum Code. Handoff benennt Lighthouse-Verzicht und fehlende echte Go-Auslieferungsprüfung offen.

## Befunde

| # | Schwere | Befund | Beleg | Empfehlung |
| --- | --- | --- | --- | --- |
| 1 | Niedrig | Absolute Pfade in `index.html` (`/manifest.webmanifest`, `/icon.svg`) und `fetch('/healthz')`, aber relative Registrierung `./sw.js` und relatives Manifest. Hinter einem Unterpfad-Proxy inkonsistent. | `index.html`, `register.ts` | Wie bestehende absolute `/assets/`-Pfade hinnehmen; Unterpfadbetrieb nicht unterstützt dokumentieren. |
| 2 | Niedrig | Offline-Hinweis nur bei Laden und `online`/`offline`-Ereignissen; fällt der Server aus, während der Browser online meldet, erscheint kein Hinweis (die Shell zeigt dann API-Fehler). | `register.ts` `probe` | Optional Probe bei API-Fehlern/Intervall; kein Datenrisiko. |
| 3 | Niedrig | Mehrere Tabs: nur der klickende Tab lädt neu; andere Tabs laufen mit alter Shell, deren Cache nach Aktivierung gelöscht ist (ein Bundle, Fallback Netz, daher unkritisch). | `activate`, `register.ts` | Hinweis in Doku genügt. |
| 4 | Niedrig (Prozess) | Dateien außerhalb der Liste in A.3 (`tests/pwa/**` statt `tests/browser/pwa.spec.ts`, `playwright.pwa.config.ts`, `package.json`, `scripts/render-icons.mjs`); Test-Server dupliziert die Go-Header manuell. | Diffstat | Beim Integrations-Pin einmal gegen den echten Go-Handler prüfen (Handoff sieht das vor). |
| 5 | Niedrig | `sw.js`-Build-ID-Ersetzung per `String.replace` mit erstem Treffer; Platzhalter dürften in der Vorlage nicht doppelt vorkommen. | `vite.config.ts` | Heute korrekt (je einmal); optional per Test absichern. |

## Hinweise für die Integration

Nach dem gemeinsamen Pin: `build-web.py`-Neubau, `--check`, zwei Neubauten bytegleich, Go-Server im Browser mit
`sw.js`, Manifest und Symbolen prüfen (MIME, kein `Service-Worker-Allowed`, Installierbarkeit auf localhost),
`TestBundledAssetsMatchManifest` um die PWA-Dateien erweitern.
