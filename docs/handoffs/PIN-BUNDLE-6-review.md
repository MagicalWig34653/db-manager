# PIN-BUNDLE-6 Review (unabhängig)

Stand 27.10.2026. Reviewer: unabhängig (nicht Autor, am Produkt nichts geändert). Geprüft: Backend `release/pin-bundle6` (18f3833, Review-Worktree `review/pin-bundle6`) gegen Web `release/pin-bundle6` (f163862, eigener
Detached-Worktree, daraus Branch `review/pin-bundle6-web` nur mit neuen Tests, Commit 33ef0fd). Autorenbranches unverändert, nichts gemergt. Läufe nacheinander; Docker nur eigene Wegwerf-Container
(aufgeräumt, 0 Reste), Port 4179 und fremde Container/Prozesse nicht berührt (Ports zufällig oder 479xx).

## Urteil

**Freigabe (Approve) mit vier Auflagen, kein Blocker für den Pin.** Das Artefakt ist korrekt gepinnt, reproduzierbar, die Lizenzinventur stimmt, die Spec-Änderungen schwächen die Prüfung nicht ab (eine
formale Lücke, siehe B), alle Fixes W-01..W-12 halten gegen eigene Angriffstests. Die Ursache des roten config-store-Laufs ist ein **Produktfehler** (Auflage 1, Race im Autosave, Schwere mittel-niedrig:
Einstellung geht verloren, kein Datenleck). Dazu zwei Härtungsbefunde an `secrets.ts` (Auflagen 2 und 3) und ein flackernder Test im Web-Repo (Auflage 4).

## Befunde

| # | Schwere | Befund | Beleg | Empfehlung |
| --- | --- | --- | --- | --- |
| 1 | Mittel/Niedrig (Produktfehler, Ursache des roten Erstlaufs) | `AppearanceSync.push()` verwirft jede Änderung, solange `loaded` noch `false` ist (`if (!this.loaded) return`), und nichts sendet sie später nach: der `$effect` in `ConfigStorePanel.svelte` hängt nur an `preferences`, `load()` wendet die Serverwerte an, aber die verworfene Eingabe ist weg. Ein Thema- oder Sprachwechsel, der in das Fenster Fähigkeitenabruf, Opt-in-Lesen, `settings/appearance` lesen fällt, wird nie gespeichert; der Server behält den alten Zustand, die Oberfläche zeigt aber den neuen (stille Divergenz). Im Echttest klickt `selectOption('dark')` unmittelbar nach `toServer()`, also genau in diesem Fenster; Poll ohne `appearance` nach 10 s. **Reproduziert deterministisch:** `tests/real/pin6-appearance.spec.ts` (Capabilities-Antwort 1,5 s verzögert) bleibt dauerhaft ohne Server-Eintrag (`test.fail`, bestätigt den Defekt), Kontrolle mit Wartezeit besteht; zusätzlich Unit-Test `it.fails` in `pin6.review.test.ts`. Häufigkeit gemessen: Autor 1 rot, mein Erstdurchlauf 5 grün, 1 rot (ca. 1 von 7, eingebettet). Die W-09-Zusammenführung (Änderung während laufendem Speichern) ist nicht die Ursache und wirkt (Test besteht). | `appearance.svelte.ts:49`, `ConfigStorePanel.svelte:46-66`, Playwright-Fehlerbild `Expected {locale:de,theme:dark} Received undefined`, `real/16-config-store.log` | Änderungen vor `loaded` merken (`wanted` setzen) und nach `load()` senden, sofern sie vom Serverwert abweichen; Echttest bis dahin auf das Ende des Ladens warten lassen. |
| 2 | Niedrig (Härtung) | `secrets.ts` ist **nicht** äquivalent zu den Backend-Regeln: Differenztest (Backend-ScanSQL gegen Client, 150 056 Mutationen aus 516 Seeds, davon **24 619 vom Backend abgelehnt**) ergibt **513 (2,1 %) vom Backend abgelehnte, vom Client akzeptierte Strings**, also Teilmengenbeziehung verletzt. Klassen: (a) 305: Ziffer, `.`, `+` oder `-` unmittelbar vor dem URL-Schema (`8redis://:geheim@h`, `74a+b.c-d://x:y@z`): die Lookbehind `(?<![a-z0-9+.-])` am Schemaanfang (zur Linearität) lässt Go-Treffer wie `edis://:...@` mitten im Schema aus; (b) 164: Leerraum, den JS `\s` als Trenner kennt, Go (RE2) aber nicht (NBSP, VT, U+2028, U+3000, U+FEFF ...) innerhalb von Zugangsdaten (`user:\u00a0pw@`, `curl -u a:\u3000b`); (c) 37 und 7: Confusables `ᴊ` (U+1D0A) und `ʜ` (U+029C) fehlen in der JS-Tabelle (Go hat sie), Rest Kombinationen. Folge: Der Server lehnt diese Einträge dennoch ab (`secret_rejected`); der Client lässt sie nur in den **lokalen Ausfallpuffer** (`localStorage`, Klartext) und in die Migration, wo der Server sie ablehnt (Zähler `rejected`). Praktisch erreichen reale Zugangsdaten das nicht, die Zeichenfolgen sind Angriffe auf die Heuristik. Alle 10 Standardvektoren des Autors, JWT, DSN, PEM, Basic, curl -u, Tokenformate und 32er-Läufe werden erkannt. | `src/lib/features/config-store/pin6.differential.test.ts` (`it.fails`, Korpus per `DBM_DIFF_CORPUS`), Backend `internal/configstore/pin6_differential_test.go` (erzeugt den Korpus, `DBM_DIFF_OUT`) | Schemaanfang ohne Lookbehind prüfen (z. B. `[a-z][a-z0-9+.-]*` ohne führende Zeichenklassenbindung, dafür Länge begrenzen), Leerraum auf `[ \t\n\r\f]` normalisieren (zusätzlich zu `fold`), Tabelle um `ᴊ`/`ʜ` ergänzen; Gleichheitsvektoren aus dem Backend als gemeinsame Fixture ablegen. |
| 3 | Niedrig (ReDoS, begrenzt) | Die DSN-Regel `(?<![^\s:@/])[^\s:@/]+:[^\s@/]+@(?:tcp...)\(` ist auf `('a'x30 + ':')*n + '@'` **quadratisch**: 0,47 s bei 100 000 Zeichen (= `MAX_SQL`, der Aufrufer begrenzt davor), 1,4 s bei 256 KiB, 4,3 s bei 512 KiB, **19 s bei 1 MiB**. 33 weitere 1-MiB-Muster (Wiederholungen von `:`, `@`, `://`, `curl`, `password`, `eyJ`, `$`, Leerraum, gefaltete Zeichen, lange Läufe) bleiben alle unter 1,5 s. Reale Reichweite: SQL ist vorher auf 100 000 Zeichen begrenzt (maximal ca. 0,5 s Hänger im Hauptthread); Namen sind kurz. Wo `looksSecret` ohne diese Grenze auf größere Eingaben liefe (Importtexte, `anySecret` über Dokumente), wäre es ein Hänger. | `pin6.differential.test.ts` (`it.fails` für 256 KiB, Rest als normale Tests), Messreihe oben | Eingabelänge in `looksSecret` auf z. B. 130 000 Zeichen begrenzen (darüber als geheim werten) oder die Regel zeichenklassenweise linear umbauen. |
| 4 | Niedrig (Testschuld Web-Repo) | `tests/real/sql-editor.spec.ts` ist flackernd: der Testhelfer `query()` ruft das Backend direkt und trifft auf die UI-Anfrage derselben SQLite-Datei; Antwort 422 `database_busy` ("Erneut versuchen"), vom Backend so vorgesehen. Vor dem Fix **2 von 6** Läufe rot (eingebettet), nach Wiederholung bei `database_busy` 7 von 8 grün (der eine Ausfall war ein Umgebungsfehler: `node` nicht im PATH), zusätzlich 2 weitere grün. Kein Produktfehler. | `real/se-*.log`, `se2-*.log`; Fix auf `review/pin-bundle6-web` | Fix übernehmen. |
| 5 | Info (zu A/B) | **Spec-Gleichwertigkeit (B):** Der neue Regex `/^(-- Schreibe hier deine SQL-Abfrage\.[\s\S]*)?$/` entspricht dem alten `toHaveValue('')` für den **Normalfall**; Nachprüfung am echten Editor (eingebetteter Build): frischer Editor, geleerter Editor und `fill('')` liefern den Platzhaltertext (akzeptiert); `SELECT 1`, `SELECT 'secret'` und Leerzeichen werden **abgelehnt**, beides wie gewünscht. **Lücke:** ein Editorinhalt, der selbst mit der Platzhalterzeile beginnt (`-- Schreibe hier deine SQL-Abfrage.` + Zeilenumbruch + `SELECT secret ...`), wird ebenfalls akzeptiert (`[\s\S]*` nach dem Prefix): der Regex unterscheidet nicht zwischen Platzhalter-Widget und echtem Inhalt. Für das Ziel der Specs (nach Tokenwechsel/Trennen darf kein SQL-Text der alten Sitzung stehen) ist das praktisch unerheblich, da der alte Text nicht mit dem Platzhalter anfängt, aber ein Rest, der so beginnt, entginge. Genauer wäre `toHaveText('')` auf `.cm-content` ohne Platzhalter (der Platzhalter ist ein `.cm-placeholder`-Widget, bei leerem Dokument genau 1, bei Inhalt 0) oder `expect(page.locator('.cm-placeholder')).toHaveCount(1)`. Keine Abschwächung im Sinne eines nicht mehr erkannten Normalfehlers, aber stumpfer als vorher. | Probeskript (Reviewer-lokal, 6 Fälle am echten Backend) | Optional auf `.cm-placeholder` zählen statt Textregex. |

## Geprüft, ohne Befund

### (A) Pin

- Lock: `commit` f163862 (= `origin/release/pin-bundle6`), `npm_lock_sha256` `970dd5d2...4ff3` = SHA-256 der `package-lock.json` im Pin-Commit. **24 Verträge**: Root `contracts/api-v*.json` gegen Webkopie im Pin-Commit
  (`git show f163862:...`, Web v24 = `contracts/api-v0.24.0.json`) und gegen die Lock-SHAs einzeln: 24 von 24 bytegleich, 0 Abweichungen; letzter Eintrag `config-store` 0.24.0 (`e2f2e057...63da7a`), stimmt mit `provenance.json` des Web-Repos.
- **Zwei frische Neubauten** (`build-web.py`, Node 26.5.0): `static/` per `diff -r` identisch, Manifest `cmp` identisch, beide gleich den eingecheckten Dateien (`git status` leer), `--check`: 53 Dateien bestanden; `test-build-web.py`: OK.
- **Mutationsproben** von `verify_build` auf Kopie des echten Builds (Reviewer-lokales Skript, alle als `ValueError`): Baseline ok; verwaiste Datei "Unreferenced asset chunks"; Chunk in `SHELL` entfernt "service worker shell list differs";
  Quelllink entfernt "exact built source link is missing"; 67 Chunks "Too many asset chunks: 67 > 64"; Precache 9 337 972 Bytes "precache too large". Alle sechs wie erwartet.
- `internal/webui/static`: 0 `*.map`, kein `sourceMappingURL`, keine Benutzerpfade (`/Users/`, `/home/`, `C:\Users`), keine Secret-Muster (AKIA, PEM, `ghp_`, `sk-`). **Precache 5 143 647 Bytes = 4,905 MiB** (52 Dateien ohne `sw.js`, davon 42 Chunks, 4 954 724 Bytes unter `assets/`), 39 % Reserve zu 8 MiB, 42 von 64 Chunks.
- **Lizenzinventar:** die 15 CodeMirror-/Lezer-MIT-Pakete (`@codemirror/*` x8, `@lezer/*` x3, `@marijn/find-cluster-break`, `crelt`, `style-mod`, `w3c-keyname`) stehen mit **Version und Lizenz** in `docs/LIZENZEN.md`
  (15 von 15 exakt gegen `package-lock.json` des Pins) und in `THIRD-PARTY-NOTICES.txt` (je Version gleich, Quelle, Lizenz MIT); der **Lizenztext** jedes Notice-Blocks entspricht (whitespace-normalisiert) der `LICENSE`-Datei im installierten Paket (`npm ci` im Pin-Worktree): 15 von 15.

### (C) Fixes W-01..W-12 (eigene Angriffstests `pin6.review.test.ts`, 12 bestanden + 1 erwarteter Fehlschlag)

Mutationsprobe der Tests: Entfernen der Signaturprüfung (W-01) und der Bytegrenze (W-02) bzw. der `sending`-Meldung und der Owner-Bindung (W-04/W-06) macht die jeweiligen Tests rot, sie erkennen also die Defekte.

- **W-01 Migrationsbestätigung:** Umschalten der Verlaufsoption, Änderung der lokalen Daten zwischen zwei Vorschauen (3 statt 2 Lesezeichen) widerrufen die Bestätigung; `run()` ohne neue Bestätigung sendet **nichts** und setzt kein Opt-in. Gesendet werden exakt die `client_ref` der bestätigten Vorschau; Opt-in nur mit Verlauf. Restlücke: die Signatur nutzt Anzahl plus erster/letzter `client_ref`; eine Änderung in der Mitte bei gleicher Anzahl und gleichen Rändern würde sie nicht widerrufen (kaum erreichbar, `client_ref` hängt am Inhalt, Vorschau läuft neu).
- **W-02 Stapel 900 KiB:** 5 Bibliotheken x 20 Lesezeichen x 12 KB migrieren vollständig (`created` = vorhandene Anzahl), jede Anfrage unter 900 KiB + 1 KiB; Hintergrund-Flush nach Ausfall (150 Verlaufseinträge à 2,5 KB, später 100er-Grenze) teilt in mindestens zwei Importe, jeder unter 1 MiB und höchstens 100 Einträge, Puffer danach leer.
- **W-03 Neuprüfung:** verzögerte Fähigkeitenabfrage (40 ms Gate, im 0-ms-Takt abgetastet) lässt `phase/mode` durchgehend `ready/server`, kein Rückfall durch `loading`; Neuprüfung im Ausfall wird `degraded`, nie lokal; im Ausfall höchstens alle 5 s (Throttle belegt: 1 s später keine Anfrage, 6 s später ja).
- **W-04 Puffer-Besitzer:** Besitzer = SHA-256 von `dbm-config-buffer\0<token>`, die ersten 32 Hexzeichen. Das Token (und sein Anfang) taucht in **keinem** `localStorage`-Schlüssel/-Wert auf; ein zweiter Token im selben Browser sieht 0 Einträge, lädt nichts hoch (0 Importanfragen) und kann fremde Einträge weder per `clear`, `removeKinds` noch `remove(refs)` löschen (beide Einträge bleiben). Bewertung Kurzhash: siehe unten.
- **W-06 `writeCount`/Schreibphasen:** der Store meldet `sending` und danach `settled`/`unknown` (ausgeglichen) für Opt-in, Lesezeichen anlegen, ändern, löschen, Favoriten, Zuletzt, Dokument, Import, Löschplan/-ausführung, verlorene Antwort (`unknown`) und den Hintergrund-Flush beim Neuprüfen. Beobachter: nur gemountete Panels (`ConfigStorePanel`, `ServerLibraryView`) leiten an `Session.writeCount` weiter. Schreibzugriffe ohne gemountetes Panel (**`recordRecent` beim Öffnen einer Tabelle**, `recordServerHistory` beim Abfragestart, Flush bei `ensureReady` aus diesen Pfaden) melden **an niemanden** (Test `FINDING`): das ist bewusst Komfortdaten bzw. Verlauf, aber die Zusage "alle Writer" gilt nur für die Panel-Wege. Kein Datenverlust (idempotent, `If-Match`), nur kein Navigations-/Trennschutz; Auflage 4 unten als Hinweis.
- **W-09 Autosave:** eine Änderung während laufendem Speichern geht danach raus (zwei Writes, zweiter mit `light/en`). **Aber** die Änderung vor Abschluss von `load()` geht verloren (Befund 1).
- **Altdaten:** lokale Einträge mit JWT oder `curl -u` werden gelesen (3 Einträge bleiben im Browser sichtbar), von der Migration übersprungen und als `skipped` gezählt (2), weder JWT noch Passwort in irgendeinem Request.
- **Offline-Brute-Force des Besitzer-Kurzhashs (Bewertung):** Hash = ungesalzenes, einfaches SHA-256 mit festem Präfix, auf 128 Bit gekürzt. Das Kürzen schwächt nicht (zwei Token mit gleichem 128-Bit-Wert zu finden ist ohnehin ein Kollisions-, kein Vorbildproblem; ein Vorbild bleibt 2^128). Entscheidend ist die Tokenentropie: das Backend verlangt nur **mindestens 32 Zeichen** (`config.go:40`), der Generator `create-dev-token.py` nutzt `token_urlsafe(48)` (384 Bit): dann ist ein Offline-Angriff aussichtslos. Ein menschlich gewähltes 32-Zeichen-Token (Wörter, Muster) fiele dagegen einem schnellen Wörterbuchangriff zum Opfer (SHA-256 ist schnell, keine Schlüsselableitung, kein Salz, Milliarden Versuche/s auf GPU), **sofern** jemand `localStorage` lesen kann (lokaler Zugriff auf das Profil, XSS; dann liegen dort ohnehin die gepufferten SQL-Texte im Klartext, das Token selbst nicht). Zusätzlich verrät der gleiche Hash, dass zwei Profile/Browser dasselbe Token benutzen (Verknüpfbarkeit, gering). **Urteil: ausreichend für zufällige Token, schwach für gewählte.** Empfehlung: Token-Mindestentropie im Backend dokumentieren/prüfen, oder Besitzer mit zufälligem Browser-Salt plus gestreckter Ableitung (PBKDF2/Argon2 über WebCrypto) bilden, oder Besitzer rein zufällig je Sitzung vergeben und nicht aus dem Token ableiten.

### (D) Echtläufe gegen die eingebetteten Assets (`DBM_EMBEDDED=1`, Quellbackend = Review-Worktree 18f3833, Wegwerf-Container, Web-Checkout f163862)

| Skript | Ergebnis |
| --- | --- |
| tables | 10 von 10 (SQLite, PostgreSQL verify-full) |
| indexes / programs / search | je 1 von 1 |
| sql-batch | 3 von 3 |
| import | 6 von 6 |
| export-formats | 2 von 2 |
| partitions | 4 von 4 |
| server-admin | je 2 von 2 (SQLite, MySQL, MariaDB, PostgreSQL) |
| sql-editor | **flackernd** (Befund 4): 4 von 6 grün vor, 9 von 10 nach dem Testfix, einziger Ausfall Umgebung (`node` fehlte im PATH des Hintergrundlaufs) |
| config-store | Lauf 1 bis 5 grün (7 von 7), **Lauf 6 rot** ("theme and language ...", `config-store.spec.ts:244`: 10-s-Poll, `Received undefined`). Zusammen mit dem Autorenlauf: 2 rote von 12. |

**config-store-Ursache:** Produkt-Race im Autosave (Befund 1), nicht Test-Timing im engeren Sinn und nicht W-09. Der Test wählt das Thema sofort nach `toServer()`; das Panel hat zu diesem Zeitpunkt den Server noch
nicht geladen (`AppearanceSync` entsteht erst nach `phase ready && mode server`, `loaded` erst nach `getDocument`); `push()` verwirft. Bei schnellem Backend gewinnt der Lauf, sonst fehlt der Eintrag. Mit künstlicher Verzögerung
deterministisch rot (eigener Spec), Kontrolle grün.

### (E) Läufe

`go test -race -count=1 ./...` Exit 0 (alle Pakete ok), `gofmt -l .` leer, `go vet ./...` ok; 24 `check-*-contract.py` (ohne `check-extended-export-files.py`, braucht Argument) alle Exit 0; `check-docs.py` ok;
`build-web.py --check` ok (53); `test-web-integration.py` Exit 0 (13 von 13 Browserfälle, SQLite + PostgreSQL TLS, `PWA embedded check passed.`, Compose Browser); `test-deployment.py` Exit 0.
Web am Pin (Review-Worktree): `npm run check` 0 Fehler/0 Warnungen, `npm test` 91 Dateien, 1242 bestanden, 2 erwartete Fehlschläge (meine `it.fails`), 1 übersprungen (Differenztest ohne Korpus), `format:check` ok, `check-docs.py` ok.
**axe-core + 320 px** auf dem **eingebetteten Build** (Backend 18f3833 mit Wegwerf-Datenverzeichnis, `bypassCSP` nur im Prüfwerkzeug): Browser-Specs config-store (6), sql-editor (3), settings (2) = 22 Scans (11 Endzustand + 11 bei 320 px):
**0 Verstöße, 0 Überläufe, 0 Fehlerläufe** (wcag2a/2aa/21a/21aa/22aa).

## Neue Tests (committet)

- Backend (`review/pin-bundle6`): `internal/configstore/pin6_differential_test.go` (erzeugt den Differenzkorpus, übersprungen ohne `DBM_DIFF_OUT`).
- Web (`review/pin-bundle6-web`, 33ef0fd, gepusht): `src/lib/features/config-store/pin6.review.test.ts` (13 Tests), `pin6.differential.test.ts` (Differenz und 34 ReDoS-Fälle, 2 `it.fails`), `tests/real/pin6-appearance.spec.ts`
  (Echt-Spec Race), Fix in `tests/real/sql-editor.spec.ts`, `scripts/test-real-config-store.py` nimmt `DBM_REAL_SPEC`.

Reproduktion Differenztest: `DBM_DIFF_OUT=<datei> go test -run TestPin6Differential ./internal/configstore` (Backend), dann `DBM_DIFF_CORPUS=<datei> npx vitest run src/lib/features/config-store/pin6.differential.test.ts` (Web).

## Auflagen

1. **Befund 1** (Autosave-Race) beheben oder vom Lead ausdrücklich annehmen; der Echttest config-store bleibt bis dahin flackernd und zählt nicht als stabile Abnahme.
2. **Befund 2** (Clientregeln nicht gleichwertig, 2,1 % des Mutationskorpus), niedrig; Server fängt es ab.
3. **Befund 3** (quadratische DSN-Regel) mindestens durch Längenbegrenzung entschärfen.
4. Testfix aus Befund 4 übernehmen; `recordRecent` und Verlaufsschreiben ohne Panel optional an die Sitzungssperre anbinden (Hinweis zu W-06).

## Einschränkungen

- Bytegleichheit nur lokale Plattform/Toolchain (macOS, Node 26.5.0). Kein Lighthouse, kein Screenreader, kein Safari/Firefox. MySQL/MariaDB-Echtläufe für Import, Bulk, Export-Formate, Partitionen, Indizes, Programme, Suche und SQL-Stapel gibt es weiterhin nicht (nur server-admin).
- Der Differenzkorpus ist ein Mutationskorpus (Seeds aus Backend-Tests, Fixtures, handgeschriebene Atome), kein Beweis der Äquivalenz; die Aussage "Teilmenge" ist widerlegt, nicht bewiesen.
- Spec-Prüfung B und Axe nur am eingebetteten Build mit Chromium; Axe über die Browser-Specs des Web-Repos (Mock-Backend), nur Endzustände.
- Die Mutationsproben und Zeitmessungen sind Reviewer-lokal (Skripte im Scratchpad), die Testdateien oben im Repo.
