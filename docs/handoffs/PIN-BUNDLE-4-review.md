# PIN-BUNDLE-4 Review (unabhängig)

Stand 27.10.2026. Reviewer: unabhängig (nicht Autor). Geprüft: Root `release/pin-bundle4` (56f89fc, Basis Root-main 751dbd1) gegen Web-Branch
`release/pin-bundle4` (f830326; per `push -f` überschrieben, Remote-Stand geprüft). Review-Worktree `review/pin-bundle4`; Autorenbranch
unverändert, nichts gemergt. Läufe nacheinander, Docker nur Wegwerf (aufgeräumt; die 12 fremden Container unangetastet, Port 4179 nicht berührt).

## Urteil

**Freigabe (Approve), keine Blocker.** Das Validator-Slimming ist verhaltensgleich zum vorherigen Stand (Differenzialtest, keine einzige
Abweichung), alle Echtläufe gegen die **eingebetteten** Assets bestehen, Precache 3,6 MB. Drei niedrige Nachzügler (Prozess/Testwerkzeug).

## Befunde

| # | Schwere | Befund | Beleg | Empfehlung |
| --- | --- | --- | --- | --- |
| 1 | Niedrig (Prozess) | Die Web-Skripte `test-real-*.py` starten weiterhin `npm run preview` (Vite-Build des Webrepos, Proxy auf das Backend) und prüfen damit **nicht** die im Backend eingebetteten Assets. Für diese Freigabe habe ich Kopien der Skripte auf dem Web-Checkout f830326 so umgestellt (Web-Basis-URL = Backend, kein Preview, kein `npm run build`), so dass die Seite vom Go-Server aus `internal/webui/static` kam. Das Ergebnis ist unten belegt. Der Ansatz ist ein Review-Hilfsmittel, nichts davon ist im Repo. | Siehe "Echtläufe". | Option `DBM_EMBEDDED=1` in den Web-Skripten (Basis-URL = Backend, Preview und Build überspringen) und in Handoffs ausweisen, ob gegen Preview oder Einbettung gelaufen wurde. |
| 2 | Niedrig (Lücke) | Für die Wartung gibt es zwar `tests/real/maintenance.spec.ts`, aber **kein** `scripts/test-real-maintenance.py`. Ich habe es aus `test-real-sql-batch.py` abgeleitet (SQLite und PostgreSQL verify-full): 2 von 2 bestanden gegen die Einbettung. Für Konten gibt es keinen Echtlauf (braucht MySQL/MariaDB/PostgreSQL mit Konten-Rechten); sie sind nur durch Browserspezifikation (Mock) und Fixture-Differenz gedeckt. | `real-maintenance.log` (Reviewer-lokal). | `test-real-maintenance.py` ins Webrepo aufnehmen. Konten-Echtlauf als offenen Punkt führen. |
| 3 | Niedrig (Wartbarkeit) | Die Wurzelliste `scripts/validator-roots.json` ist handgepflegt. Fehlt ein Name, schlägt zur Entwicklungszeit `svelte-check` fehl (die `.d.ts` sind auf die Wurzeln eingeengt, Zugriffe sind als `validators[name]` mit `keyof`-Union typisiert); ein Laufzeitfehler "validator not found" ist dadurch nicht zu erwarten und trat in keinem Echtlauf auf. `validators:check` hält Generat und Wurzeln konsistent. Der Quelltext `contract/v15` (4,6 MB, nicht mehr importiert, nicht gebündelt) liegt weiter im Repo. | `npm run check` 0 Fehler, `validators:check` aktuell. | Kein Handlungsbedarf für die Freigabe; `v15`-Aufräumen bei Gelegenheit. |

## Geprüft, ohne Befund

### (1) Lock, Provenance, Verträge

- Web-Remote `origin/release/pin-bundle4` = `f8303266a403ebf2c75280aa68ff875a03dae31d`; Lock-`commit` identisch; `npm_lock_sha256` (`bac4b32b...5c6`) = SHA-256 von `package-lock.json` in diesem Commit. Quelllink im Entry-Script (`index-BVScxKFM.js`), `sw.js` (`BUILD_ID`), Dockerfile-Label, `assets-manifest.json`, WEBBUILD, LIZENZEN zeigen auf f830326.
- Lock-Schema 2, 17 Verträge: Root-Dateien und Webkopien (`src/lib/contract/vN/api-v*.json` im Lock-Commit) sind bytegleich zum Lock-SHA, darunter v10, v12 bis v16 und v23 (`7450a7fc...`). 0.17 bis 0.22 bewusst ungepinnt.

### (2) Neubau und Prüfungen von `build-web.py`

- `--check`: 42 Dateien. `build-web.py` zweimal frisch (Node 26.5.0, npm 11.17.0, Fetch von GitHub): `diff -r` der beiden `static/`-Kopien und der Manifeste leer, `git status` jeweils sauber (bytegleich). `test-build-web.py` OK.
- Mutationsproben (Kopie von `static/`, `verify_build`): Baseline besteht; verwaiste Datei: "Unreferenced asset chunks"; Chunk fehlt in `SHELL`: "service worker shell list differs"; Quelllink falsch: "exact built source link is missing"; 91 Chunks (> 64): "Too many asset chunks"; Precache 13 MB (> 8 MiB): "precache too large". Jede Probe: `ValueError`.
- Keine `*.map`, kein `sourceMappingURL`, kein `/Users/` oder `/home/` in `static/` (`grep`); Lizenztexte im Build.

### (3) Validator-Slimming

- Differenzialtest alter Stand (Web 9545f75, volle Validatoren) gegen neuen (f830326) über alle 592 JSON-Fixtures aus `contracts/fixtures/**` (real, placeholder, invalid) und alle exportierten Schemas aller Vertragsversionen: 920 Fixture/Version-Paare, **0 Abweichungen**; jedes Fixture wird von mindestens einer Version dem Dateinamen entsprechend akzeptiert bzw. (`.invalid.`) abgelehnt.
- Mutationen (jeder Schlüssel jedes Objekts einzeln entfernt, je Objekt ein Zusatzfeld `additionalProperties`): 16 088 Mutationen valider Fixtures, 14 763 abgelehnt, **0 Abweichungen zwischen alt und neu** (die übrigen 1 325 sind optionale Felder, die beide Stände gleich akzeptieren). Pro Familie (Mutationen/abgelehnt): Konten 1587/1213, Programme 912/812, Suche 1877/1766, Indizes 2942/2790, SQL-Stapel 1061/985, Wartung 1946/1796, Overview 1438/1416, Spalten 1460/1348, Objektops 2380/2190, Export 424/409.
- 122 Fixtures (Import, Monitor, Partitionen, Bulk-Zeilen, Server-Admin, erweiterter Export, Vault) betreffen v17 bis v22 bzw. Nicht-App-Schemas ohne Oberfläche, bewusst nicht exportiert.
- Laufzeit: `npm run check` 0 Fehler/0 Warnungen (5144 Dateien), `npm test` 54 Dateien bestanden, `validators:check` aktuell. Echtläufe (unten) rufen jedes Feature im Browser gegen das eingebettete Backend auf; kein Laufzeitfehler "validator not found", keine Konsolenfehler in den Specs.

### (4) Echtläufe gegen die eingebetteten Assets (Backend aus dem Review-Worktree, `DBM_BACKEND_REPO`, Rev 56f89fc)

| Skript | Ergebnis |
| --- | --- |
| `test-real-tables.py` (Tabelle, Zeilen, Struktur, Export; SQLite + PostgreSQL verify-full) | 10 von 10 |
| `test-real-indexes.py` (SQLite) | 1 von 1 |
| `test-real-programs.py` (SQLite) | 1 von 1 |
| `test-real-search.py` (SQLite) | 1 von 1 |
| `test-real-sql-batch.py` (SQLite + PostgreSQL) | 3 von 3 |
| Wartung (abgeleitetes `test-real-maintenance.py`, SQLite + PostgreSQL) | 2 von 2 |
| Zusatz: overview / columns / object-ops / create / export | 2 / 2 / 4 / 2 / 2, alle bestanden |

### (5) PWA

- `sw.js`: Scope nur App-Shell; antwortet nie auf `/api`, `/healthz`, `/readyz`, nicht auf Anfragen mit `authorization`, nur auf exakte GET-Anfragen auf vorgeladene Dateien; Cache nur beim Install, Name `db-manager-shell-<BUILD_ID>`. `SHELL` = Inventar (39 Einträge, vom Build und meiner Mutationsprobe erzwungen).
- Precache gemessen: 3 616 607 Bytes (41 Dateien ohne `sw.js`; davon 3 446 671 unter `assets/`, 31 Dateien). Unter 8 MiB. Offline-Start: `check-pwa-embedded.mjs` ("PWA embedded check passed.") im Integrationslauf.

### (6) Läufe

`go test -race ./...` grün (kein Flake, Last 7 bis 9); `gofmt -l` leer; `go vet ./...` ok; 23 `check-*-contract.py` plus `check-docs.py` ok; `test-web-integration.py` Exit 0 (13 von 13 Browserfälle, SQLite + PostgreSQL, `PWA embedded check passed.`); `test-deployment.py` Exit 0 (Compose und Swarm, Aufräumen gemeldet).

### (7) 320 px und axe

axe-core (wcag2a/2aa/21a/21aa/22aa) und 320-px-Überlauf nach jedem Echtlauf-Spec auf dem eingebetteten Build (Fixture des Webrepos `tests/review/axe-fixture`, Zustand am Testende und bei 320 px; `bypassCSP` nur im Prüfwerkzeug, da die CSP das Einschleusen von axe sonst zu Recht blockiert): 7 Features (Tabelle/Zeilen/Struktur/Export, Suche, Wartung, Programme, SQL-Stapel, Indizes, Overview), 24 Scans: **0 Verstöße, 0 Überläufe**. Einschränkung: nur der Endzustand je Test, keine Zwischenzustände.

## Einschränkungen

- Bytegleichheit nur lokale Plattform/Toolchain (macOS, Node 26.5.0). Kein Lighthouse. Konten ohne Echtlauf (Befund 2). Docker: nur SQLite/PostgreSQL 18.6 Wegwerfcontainer.
