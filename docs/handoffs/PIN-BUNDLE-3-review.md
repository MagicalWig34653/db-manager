# PIN-BUNDLE-3 Review (unabhängig)

Stand 26.10.2026. Reviewer: unabhängig (nicht Autor). Geprüft: Root `release/pin-bundle3` (2d6d622, Basis c331c64) gegen Web-main
c14271e. Review-Worktree `review/pin-bundle3`; Autorenbranch unverändert, nichts gemergt. Last Load 130 bis 250, Läufe nacheinander,
Docker nur Wegwerf (aufgeräumt).

## Urteil

**Freigabe (Approve), keine Blocker.** Die vier Fehlschläge der Legacy-Specs sind **keine Regression des Builds**, sondern ein
Artefakt des Hilfsskripts (alter Backend-Checkout, siehe Befund 1). Nachzuziehen: Doku des Handoffs korrigieren (Befund 1),
MAX_CHUNKS-Konzept (Befund 2).

## Befunde

| # | Schwere | Befund | Beleg | Fix |
| --- | --- | --- | --- | --- |
| 1 | Mittel (Prozess/Doku, kein Produktfehler) | Ursache der 4 Fehlschläge (export.spec, structure.spec je SQLite/PostgreSQL): `scripts/test-real-tables.py` baut das Backend aus dem in `src/lib/contract/v5/provenance.json` gepinnten **alten** Backend-Commit (Git-Fetch von GitHub), nicht aus `release/pin-bundle3`. Dessen eingebettete Oberfläche ist die alte (Seite lädt `index-Bk7Yt41f.js`, 724 kB, ohne Chunks). Der Autor-Hilfsbranch ersetzte nur den Vite-Preview durch die Backend-URL und prüfte daher nicht den gepinnten Build. Bei Ersetzen des Backends durch den Review-Worktree (Kopie des Skripts mit `git archive` des Branches statt Pin-Fetch) lädt die Seite `index-BnxbHZ7e.js` plus 14 Chunks und **alle 10 von 10 Specs bestehen** (inkl. export und structure, SQLite und PostgreSQL). Gleiche Specs gegen Vite-Preview mit altem Backend: export scheitert ebenfalls (2 von 10), also unabhängig vom Go-Handler. | Ressourcenliste der Seite (Performance-API) im Fehllauf vs. Lauf mit Branch-Backend; Playwright-Läufe: Autorenvariante 6 von 10, Branch-Backend 10 von 10 (Exit 0). Kein CSP-Verstoß, Service Worker im Test irrelevant (`serviceWorkers: 'block'` ändert nichts am Fehlverhalten des alten Backends); Go-Handler liefert identische Bytes (Etag = SHA im Manifest). Kein Locator-/Timing-Problem im Produkt. | Handoff-Aussage "Locatoren passen nicht zur neuen Shell" korrigieren. Web-Skripte `test-real-*.py` müssen bei Einbettungsprüfung `DBM_BACKEND_REPO` auch für das Quellbackend nutzen (`test-real-tables.py` tut es nicht, die neueren Skripte schon). Bei Pin-Prüfungen immer prüfen, welche `index-*.js` geladen wird. |
| 2 | Niedrig | `MAX_CHUNKS` 12 auf 24 ist als Notmaßnahme vertretbar, aber die Zahl ist die schwächste der vier Prüfungen und wird mit jeder Welle wieder reißen (15 von 24 belegt). Die eigentlichen Garantien sind Entry+Quelllink, transitive Erreichbarkeit jeder Datei unter `assets/` und `SHELL == Inventar`. Mutationstest (Kopie von `static/`): verwaiste Datei, Chunk fehlt in `SHELL`, Quelllink fehlt, 27 Chunks: jeweils `ValueError` (Exit 1); Baseline besteht. | `verify_build` mit fünf Fällen. | Obergrenze ersatzlos streichen oder als hohe Plausibilitätsgrenze (z. B. 64) belassen und Precache-Größe begrenzen (Befund 3). Aus dem Lock ableiten bringt keinen Gewinn. |
| 3 | Niedrig | Der Service Worker legt beim Install alle 14 Assets plus 8 Shell-Dateien an: 4,6 MB unter `assets/` (davon 1 MB `index`, 1,2 MB und 0,9 MB zwei Feature-Chunks), ca. 4,8 MB insgesamt, einmalig je Release. Offline-Start belegt (alle 15 Chunks in `SHELL`, `check-pwa-embedded.mjs` besteht: Cache = `SHELL`, offline alle 200). Die Größe ist in der Doku nicht genannt. | `du`, `sw.js`, PWA-Check im Integrationslauf. | Precache-Größe in WEBBUILD/BETRIEB nennen; optional Schwellwert im Build (z. B. 8 MB). |
| 4 | Niedrig (Flake) | `TestSQLiteSQLBatchLimitsAndCancellation` ("aggregate limit not exercised") scheiterte einmal im Gesamtlauf `go test -race ./...` bei Load 180 bis 250, isoliert wiederholt grün. Lastabhängig (Zeitlimit vor Größenlimit). | Gesamtlauf rot, Einzellauf 4,5 s grün. | Test unabhängig vom Zeitbudget machen (größere Zeitgrenze oder kleineres Ergebnislimit). |

## Geprüft, ohne Befund

- **Pin:** Lock-Commit = `c14271ec…`, `npm_lock_sha256` = SHA des `package-lock.json` im Commit; alle zehn Verträge (0.1 bis 0.9, 0.11) zwischen Root und Webkopie SHA-identisch; 0.10 und 0.12 im Web nicht vorhanden und ungepinnt (dokumentiert).
- **Neubau:** `build-web.py` zweimal frisch, `git status` jeweils sauber (bytegleich); `--check` 26 Dateien; `test-build-web.py` OK.
- **Läufe:** `gofmt` leer, `go vet` ok, elf Wächter ok, `check-docs` 0 Fehler, `test-web-integration.py` Exit 0 (13/13 plus `PWA embedded check passed.`), `test-deployment.py` (Compose, Swarm) ok.
- **Neue Oberflächen gegen eingebettete Assets:** Die Web-Skripte create, overview, object-ops, columns, export laufen laut Autor grün; ich habe `test-real-tables` (Tabelle, Zeilen, Struktur, Export) mit Branch-Backend selbst wiederholt: 10/10.
- **Lecks/CSP:** keine `sourceMappingURL`, keine privaten Pfade in `static/` (geprüft durch `build-web.py`), Lizenztexte im Manifest, CSP unverändert mit `manifest-src`/`worker-src`, keine Verstöße in den Läufen.
