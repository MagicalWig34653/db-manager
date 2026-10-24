# W-P04 Review (unabhängig)

Stand 24.10.2026. Reviewer: unabhängig (nicht Autor). Geprüft: Root `feat/w-p04-lock` (08b23c8) und Web
`feat/w-p04-contracts` (c547e45), je gegen `main`. Review-Worktrees `review/w-p04` in beiden Repos; Autorenbranches
unverändert. Hohe Hostlast (Load 136 bis 775), Builds strikt nacheinander, `node_modules` als `cp -cR`-Kopie.

## Urteil

**Freigabe (Approve), keine Blocker, keine Hohen/Mittleren Befunde.** Zwei niedrige Befunde und Hinweise unten sind
nicht blockierend. Beide Branches sind unabhängig voneinander mergebar (beliebige Reihenfolge).

## Ausgeführte Prüfungen

| Prüfung | Ergebnis |
| --- | --- |
| `python3 scripts/build-web.py --check` | Verified 5 embedded assets and pinned provenance |
| `python3 -B scripts/test-build-web.py` | 16 Tests OK |
| `python3 scripts/build-web.py` (frischer Neubau Pin 8ef738d, ein Lauf, 19 s) | erfolgreich, `git status` leer (Assets und Manifest bytegleich) |
| `go vet ./internal/webui`, `go test -race ./internal/webui` | ok |
| `python3 scripts/check-docs.py` | 411 files, 0 errors |
| Web `npm run contract:check`, `validators:check`, `format:check` | ok, Worktree sauber |
| Web `npm test` | 8 Dateien, 47 Tests ok |
| Web `npm run build` | ok, Worktree sauber (kein Diff an Bestandsdateien, `package-lock.json` unverändert) |

## Negativtests Lock-Schema v2 (eigenes Skript, 26 Fälle gegen `load_lock`)

Abgewiesen (ValueError): `contracts` fehlend, `null`, `{}`, leere Liste; doppelte Familie/Version; umgekehrte
Reihenfolge; falscher Hash (Bestandsvertrag und neuer); Großbuchstaben-Hash; Nicht-String-Hash; alter flacher
Schlüssel; unbekannter Lock-Schlüssel; zusätzlicher Eintragsschlüssel (`path`); `../0.1.0` in `version`,
`../a` in `family`; Version `0.06.0`, `0.0.0`, Unicode-Ziffer `0.٦.0`, Version mit `\n`, Version als Zahl;
Familie mit `\n`; fehlender oder umbenannter Bestandsvertrag; `schema_version: true`.
Akzeptiert korrekt: Baseline, 0.6.0 mit echtem SHA und neuer Familie (`create`). Falscher SHA für 0.6.0 wird
abgewiesen. Pfadtricks sind ausgeschlossen, weil nur `0.N.0` per `fullmatch` und `\Z` in Dateipfade eingeht; die
Familie fließt nie in Pfade. Ein Vertrag ohne Hash-Prüfung lässt sich nicht durchschleusen: jeder Eintrag durchläuft
Format-, Duplikat-, Reihenfolge- und Rootbyte-SHA-Prüfung; beim Neubau zusätzlich die Webkopie (Datei, kein Symlink,
SHA). Manifest-`source` für die fünf Bestandsverträge entspricht byte-/wertgleich dem eingebetteten Manifest
(Dictvergleich im `--check`, Git-Diff leer). Mit Zusatzvertrag wechselt `source` korrekt auf `schema_version` 2 mit
`additional_contracts`. Go liest `source` nur in `handler_test.go` (`contract_sha256`, bleibt für Bestand gültig).

## Negativtests Web (`scripts/contracts.mjs`)

Ohne/ungültiger Modus: Exit 2 mit Usage. Leerer Ordner `v6`: harter Fehler (Exit 1, "Missing contract file").
`v01` (Duplikat Major 1): Exit 1. `vx`/`v7a`: ignoriert. Drift in `v3/api-types.ts` bzw. `v4/validators.js`: Exit 1 bei
`check` und `validators-check`; Bestand danach wiederhergestellt. Reihenfolge numerisch aufsteigend (nicht
lexikografisch, 10 nach 9). `spawnSync` ohne Shell, `major` nur aus `^v[0-9]+$` numerisch, Argumente als Array: keine
Injection (Test mit `--v2;touch${IFS}/tmp/PWN`: nichts angelegt).

## Befunde

### B1 niedrig: ungültige `--v<N>`-Flags werden in `tests/generate-validators.mjs` still als Major 1 gewertet
Beleg: `node tests/generate-validators.mjs --v02 --check`, `--v../x`, `--v1` und `--v2;touch...` enden mit Exit 0 und
prüfen stillschweigend 0.1.0 (Regex ignoriert den Treffer-Misserfolg, Standard `'1'`). Vorbestehendes Verhalten
(zuvor galt dasselbe für `--v6`), jetzt nur sichtbarer, weil das Flag erweitert wurde. Keine Injection, kein
Dateizugriff außerhalb. Der Wrapper `contracts.mjs` übergibt nur numerisch erzeugte Flags und ist nicht betroffen.
Fix (optional, im nächsten Webpaket): Argumente `^--v` strikt validieren, unbekannte Formen mit Exit 2 abweisen.

### B2 niedrig: Richtung Web zu Lock ungeprüft
Beleg: ein im Webcheckout vorhandener Vertragsordner (z. B. `v6`), der nicht im Lock steht, wird beim Neubau nicht
bemängelt; `rebuild` prüft nur Lock-Einträge gegen das Web. Der Plan fordert nur fehlende/abweichende Einträge; das
ist erfüllt. Risiko: ein vom Web konsumierter, aber ungepinnter Vertrag. Fix (optional, spätestens beim ersten
neuen Pin W-P05): im `rebuild` die `src/lib/contract/v*/`-Ordner mit der Lockliste abgleichen (oder bewusst
dokumentieren, dass 0.6 ungepinnt bleibt).

### Hinweise (keine Befunde)
- `schema_version: 2.0` (JSON-Float) wird akzeptiert, da `2.0 == 2` in Python. Harmlos; `true` wird abgewiesen.
- Familiennamen sind eindeutig; ein späterer zweiter Vertrag derselben Fachfamilie braucht einen neuen Namen
  (WEBBUILD.md nennt "keine doppelte Familie", aber nicht ausdrücklich die Folge). Doku-Nit.
- CI-Schrittname in `.github/workflows/web-runtime.yml` ("fünf Verträge") bleibt korrekt, solange nur fünf gepinnt sind;
  beim ersten Zusatzpin anpassen. `--check` prüft weiterhin nur Rootbytes, nicht die Webkopie (wie zuvor).
- `docs/WEBBUILD.md` hat vor dem neuen Abschnitt eine doppelte Leerzeile (Format-Nit).

## Integrierbarkeit und Pin-Mechanik

Die Branches berühren disjunkte Repos und Dateien; Mergereihenfolge beliebig. Der Root-Teil (Schema 2) ist
rückwärtskompatibel nur durch gleichzeitigen Lock-Wechsel (alte flache Schlüssel werden bewusst abgewiesen), das
Lock wird mit demselben Branch migriert, also konsistent. Für den ersten neuen Pin ab W-P05: Web-Commit mit
`src/lib/contract/v6/api-v0.6.0.json` (Skript `contracts.mjs` erzeugt Typen und Validatoren automatisch), im Root
`commit`, `npm_lock_sha256` und ein Eintrag `{family, version: "0.6.0", sha256}` ergänzen, danach Neubau; der
Pfad `src/lib/contract/v6/api-v0.6.0.json` ist in `web_contract_path` und `contracts.mjs` identisch abgeleitet.
Plausibel und vollständig. Das Manifest wechselt dann auf `source.schema_version` 2 (Bestandsschlüssel bleiben).

## Doku

`docs/handoffs/W-P04.md` (beide Repos) und `docs/WEBBUILD.md` sind sachlich korrekt und decken sich mit den
Ergebnissen (16 Tests, 47 Tests, bytegleicher Neubau, Einschränkung "beweist das Skript, nicht den Webteil" ehrlich
genannt). Keine überzogenen Aussagen gefunden.
