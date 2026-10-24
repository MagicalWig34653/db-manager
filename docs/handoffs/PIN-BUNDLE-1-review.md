# PIN-BUNDLE-1 Review (unabhängig)

Stand 24.10.2026. Reviewer: unabhängig (nicht Autor). Geprüft: Root `release/pin-bundle1` (ca5db0d, Basis e7bff98).
Review-Worktree `review/pin-bundle1`; Autorenbranch unverändert, nichts gemergt. Läufe nacheinander, nur
Wegwerf-Container (aufgeräumt). Der Reviewer-Branch enthält zusätzlich einen Fix-Vorschlag-Commit (Befunde 2 und 3).

## Urteil

**Nicht freigeben (Changes requested).** Ein Hoher Befund (Regression im Pin, deterministisch), zwei Mittlere
(Pin-Konsistenz, Teststand). Pin-Mechanik, Reproduzierbarkeit, PWA-Auslieferung und Container sind in Ordnung.

## Befunde

| # | Schwere | Befund | Beleg | Fix |
| --- | --- | --- | --- | --- |
| 1 | Hoch | Der Browser-Test `export.spec.ts` (Browserpfad, SQLite und PostgreSQL) scheitert deterministisch im gepinnten Webstand: `page.waitForLoadState('networkidle')` läuft in den Timeout. Ursache ist keine Lastflake: `src/lib/pwa/register.ts` `probe()` ruft `await fetch('/healthz', ...)` auf und liest den Body nie; Chromium/Playwright zählt die Anfrage dann dauerhaft als offen (Seite wird nie „networkidle“). Das ist ein Produktmangel des W-P03-Webcodes und auch real relevant (offene Verbindung/Antwort bleibt unverbraucht). | Mit dem Go-Server des Pins: Seite laden, 4 s warten, `networkidle` Timeout, einzig offene Anfrage `/healthz`. Isoliert: `fetch('/healthz')` ohne Body-Lesen = TIMEOUT, mit `await r.text()` = idle. Derselbe Test mit dem alten Stand (main-Binary, Asset `index-Ay2aZLYy.js`) = idle. Lauf `test-web-integration.py`: 11 passed, 2 failed (beide export.spec). | Web: in `probe()` den Body verwerfen (`const r = await fetch(...); await r.body?.cancel()` oder `await r.text()`), neuer Web-Commit, Review, Neupin. Pin nicht ohne Fix mergen. W-P03-Web-Test `test:pwa` hat das nicht erkannt (Mock ohne networkidle). |
| 2 | Mittel | `Dockerfile` Label `org.dbmanager.frontend.source` zeigt weiter auf `8ef738d`, nicht auf den neuen Pin `6cb9613`; `verify_image` in `scripts/test-web-integration.py` bricht deshalb nach dem CLI-Teil ab (AssertionError), das Image trägt falsche Provenienz. Der Autor hat den Lauf ausgelassen und es so nicht gesehen. | `Dockerfile:17`; Abbruch bei `web_test_support.py:125`. | Label auf `6cb9613eebcca3196e115f30e67c9be1c2f4b943` (im Review-Commit enthalten). Wünschenswert: Label aus `web-build.lock.json` ableiten bzw. `check-docs`/Wächter dafür. |
| 3 | Mittel | `verify_http` in `scripts/web_test_support.py` erwartet `immutable` für jede `.js`-Datei im Manifest; `sw.js` (no-store, korrekt) lässt den Test fehlschlagen. Auch das war nur durch den ausgelassenen Lauf auffindbar. | `AssertionError` bei `verify_http` im CLI-Teil. | Prüfung auf `assets/`-Pfade begrenzen (im Review-Commit enthalten). |
| 4 | Niedrig | `scripts/check-pwa-embedded.mjs` macht keine Zusicherungen (druckt nur JSON), hängt von `PLAYWRIGHT_CORE` ab und ist in keine Suite eingehängt; ein Rückschritt würde nicht auffallen. | Datei. | Assertions ergänzen oder in `test-web-integration.py` einhängen. |
| 5 | Niedrig | Docs: STATUS nennt den Branch als „Review ausstehend, nicht in main“ (ehrlich). `ARBEITSPLAN.md`/Teile von STATUS zeigen noch `8ef738d` als Webstand (historisch, im Kontext der alten Pakete, nicht falsch). | `git grep 8ef738d`. | Nach Merge STATUS/ARBEITSPLAN aktualisieren. |
| 6 | Niedrig | Offene W-P03-Review-Befunde 2 bis 5 bleiben bestehen (dokumentiert, niedrig). | W-P03-review.md | Siehe dort. |

## Ausgeführte Prüfungen

| Prüfung | Ergebnis |
| --- | --- |
| Pin: `web-build.lock.json` commit = `git rev-parse main` im Web-Repo | `6cb9613eebcca3196e115f30e67c9be1c2f4b943`, gleich |
| npm_lock_sha256 gegen `git show 6cb9613:package-lock.json` | `bac4b32b…15c6`, gleich; Vertragseinträge im Diff unverändert (nur `commit` geändert) |
| `python3 scripts/build-web.py` (frischer Neubau, zweimal nacheinander) | je 13 Dateien, `git status` danach sauber (Assets und Manifest bytegleich zum eingebetteten Stand) |
| `build-web.py --check`, `test-build-web.py` | Verified 13 embedded assets; OK |
| `go test -race ./...`, `go vet ./...`, `gofmt -l .` | ok, keine Ausgabe |
| Vertragswächter table/mutation/insert/structure/create, `check-docs.py` | bestanden, 436 Dateien 0 Fehler |
| `scripts/test-web-integration.py` (Docker, PostgreSQL TLS, Compose, Playwright `tests/web-runtime`) | mit Fix-Commit bis zum Browserteil: 11 von 13 bestanden (rows, runtime inkl. Quelldialog/Notices, structure, tables je SQLite und PostgreSQL), 2 durch Befund 1 fehlgeschlagen |
| `scripts/test-deployment.py` (Compose, Swarm) | bestanden, Ressourcen entfernt |
| `scripts/check-pwa-embedded.mjs` gegen den echten Go-Server | Worker aktiv, Scope `/`, Installierbarkeitsfehler leer, Manifest 0 Fehler, Cache `db-manager-shell-6cb9613…` nur Shell (kein `/api`, `/healthz`), offline `/healthz` und API Netzwerkfehler (nie aus Cache), Offline-Shell deutsch, 0 CSP-Verstöße; alle 8 PWA-Dateien 200, richtige MIME, `no-store`, `nosniff`, ohne `Service-Worker-Allowed`; CSP mit `manifest-src`/`worker-src`, ohne `unsafe-*` für Skripte |

## Eingebettete Assets

Keine `sourceMappingURL`, keine `/Users/`-Pfade in `static/`. `LICENSE.txt` und `THIRD-PARTY-NOTICES.txt` weiter im Manifest;
der Browsertest `runtime.spec` (Quelldialog und Originalnotices) besteht. Externer Netzwerkzugriff der Shell ist durch
`connect-src 'self'`, `default-src 'none'` unterbunden (keine CSP-Verletzung im Lauf).

## Doku

`WEBBUILD.md` benennt ehrlich „API 0.6 nicht gepinnt“ (Web enthält keine Kopie) und den nicht unterstützten Unterpfadbetrieb
(absolute Pfade). `PIN-BUNDLE-1.md` benennt die ausgelassenen Läufe selbst; deren Nachholung hat die Befunde 1 bis 3 ergeben.
Nicht ausgeführt: Lighthouse.

## Weiteres Vorgehen

Web-Fix für Befund 1, neuer Web-main-Commit, Pin auf diesen, Neubau, danach `test-web-integration.py` vollständig grün
nachweisen (Review-Commit mit Befunden 2 und 3 übernehmen).
