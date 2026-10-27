# PIN-BUNDLE-5 Pin auf Web-Branch release/pin-bundle5 (4b24e53)

Stand 27.10.2026. Branch `release/pin-bundle5` (Basis Backend-`main` cf3d406, Verträge v1..v23), nicht nach main gemergt. Folgt auf Bündel 4.

## Ergebnis

- **Web-Branch:** `release/pin-bundle5` im Web-Repo = `integration/web-wave2-fixes` (Basis `review/web-wave2` c48d476 mit Welle 2). **Lock-Commit:
  `4b24e530af8f27700722379d4073a2edbbaa0d22`**. `npm_lock_sha256` unverändert `bac4b32b...5c6` (gegen `package-lock.json` des Commits geprüft).
  Nichts nach Web-main gemergt.
- **Inhalt gegenüber Bündel 4:** Web-Welle 2 (Import, Mehrzeilen-Aktionen und Dateiwerte, Serveradministration, Überwachung, erweiterte Exportformate,
  Partitionen und Präfix-Stapel) samt Review-Fixes (WEB-WAVE2-FIXES): Schließsperre der Schreibdialoge während des Requests (F-1), SQL-Abschlussprüfung
  mit Lexer (F-2), Katalog-Texte de/en (F-4), Import-Upload-Race und wiederholbarer Abbruch (F-5/F-6).
- **Lock:** 23 Verträge. Neu gegenüber Bündel 4 (alle Webkopien byte-identisch zu Root, SHA-256 gegen `contracts/` geprüft): `serveradmin` 0.17,
  `import` 0.18, `bulk` 0.19, `monitor` 0.20, `exportext` 0.21, `partitions` 0.22. Reihenfolge aufsteigend, `reasons` 0.23 zuletzt. Die Vertragskopien liegen im Web
  unter `src/lib/contract/v17` bis `v22` (Provenienz je Version im Web). Hinweis: Bündel 4 sprach von "v17 bis v22 nicht kopiert"; jetzt haben alle sechs
  eine Oberfläche.
- **Neubau:** 49 Dateien ohne `sw.js` (50 mit), davon 39 unter `assets/`. `build-web.py` zweimal frisch: Verzeichnisse identisch (`diff -r`), Manifest bytegleich
  (`cmp`), `--check` bestanden. Dockerfile-Label, WEBBUILD (Abschnitt Pin Bündel 5), LIZENZEN, STATUS auf 4b24e53; `test-build-web.py` auf 23 Verträge/neue Reihenfolge.
- **Precache-Größe: 4 530 880 Bytes (4,32 MiB)** ohne `sw.js` (davon 4 360 944 unter `assets/`, 39 Dateien); Bündel 4: 3,6 MB. Grenzen eingehalten:
  `MAX_PRECACHE_BYTES` 8 MiB (54 % Reserve), `MAX_CHUNKS` 64 (39 von 64 belegt).
- **Angepasste Backend-Spec:** `tests/web-runtime/rows.spec.ts` ("actual committed insert with lost response") drückte während eines gehaltenen Writes Escape und
  erwartete, dass der Editor schließt. Das ist genau das Verhalten, das F-1 verbietet. Die Spec prüft jetzt, dass Escape den Dialog offen lässt, wartet die verlorene
  Antwort ("unbekannt") ab und trennt danach die Sitzung (Tokenwechsel); der Fall "späte Antwort nach Sitzungswechsel" ist weiter durch die Web-Spec
  "changing the backend token invalidates the complete previous session and delayed query" gedeckt.
- **Gefundene Regression (im Web behoben, `c32d946`):** Die Laufzeitspecs `export.spec.ts` und `structure.spec.ts` schlugen zunächst fehl, weil der Zeileneditor
  bei jedem Tabellenladen `/api/v5/tables/structure` las (Fremdschlüssel für die Nachschlagehilfe). Jetzt erst beim Öffnen eines Editors, einmal je Arbeitsbereich.

- **Nachbesserung nach PIN-BUNDLE-5-review (bd7e97f), Web `4b24e53`:** (1) `tests/real/rows.spec.ts` wie die Laufzeit-Spec angepasst; (2) SQL-Lexer kennt
  SQLite-Klammerbezeichner (`[o'k]`), Dollar-Quotes bewusst nicht (Backend gibt keine aus); (3) Fremdschlüssel-Laden meldet Fehler sichtbar und versucht es beim
  nächsten Editor erneut; (4) Web-Spec für die späte Schreibantwort nach Sitzungswechsel (SQL-Stapel); (5) `DBM_EMBEDDED=1` für alle `scripts/test-real-*.py`
  (siehe unten). Lock-Commit jetzt `4b24e530af8f27700722379d4073a2edbbaa0d22`, `npm_lock_sha256` unverändert.
- **`DBM_EMBEDDED=1`:** Die Web-Skripte `test-real-*.py` überspringen dann Build und Vite-Preview; `scripts/embedded-proxy.mjs` reicht einen freien Port
  an das Go-Backend durch, dessen eingebettete Oberfläche geprüft wird. Handoffs sollen künftig ausweisen, ob gegen Preview oder Einbettung gelaufen wurde.

## Prüfungen (Worktree `.worktrees/release/pin-bundle5`, nacheinander)

| Befehl | Ergebnis |
| --- | --- |
| `build-web.py` zweimal, `--check`, `test-build-web.py` | 50 Dateien, identisch (`diff -r`, `cmp`); `--check` bestanden; Tests OK (26, inkl. Precache und MAX_CHUNKS) |
| `go test -race ./...` | alle Pakete ok (Gesamtlauf mit Datenbankpaket 129 s; letzter Lauf nach reiner Dokumentänderung teils aus dem Cache, `internal/webui` frisch) |
| `gofmt -l .`, `go vet ./...` | leer, ok |
| 24 `check-*-contract.py` und `check-docs.py` | alle bestanden (1386 Dateien, 0 Fehler) |
| `test-web-integration.py` | Exit 0, 13 von 13 Browserfälle (SQLite + PostgreSQL TLS), `PWA embedded check passed.` |
| `test-deployment.py` | Exit 0 (Compose, Swarm), Aufräumen gemeldet |

Web-Prüfungen am Pin-Stand (4b24e53): siehe `docs/handoffs/WEB-WAVE2-FIXES.md` im Web-Repo (check, 970 Vitest-Tests, format, contract, validators, build, CSP 2, PWA 6, Playwright 118; Echtläufe Export und Export-Formate).

## Einschränkungen

- Die Echtläufe `test-real-*.py` der Welle-2-Familien (Import, Bulk, Partitionen, Server-Admin) sind im Web-Review (c48d476) gegen Backend `cf3d406` gelaufen, nicht erneut
  gegen die eingebetteten Assets dieses Bündels; gegen die Einbettung läuft `test-web-integration.py` (Tabellen, Zeilen, Struktur, Export, Laufzeit).
- MySQL/MariaDB-Echtläufe für Import, Bulk, Export-Formate und Partitionen fehlen weiter. Kein Lighthouse, kein Unterpfadbetrieb, Bytegleichheit nur lokale Plattform/Toolchain.
- Port 4179 (fremder Prozess) und fremde Container blieben unangetastet.

## Eingebettete Dateien

Das vollständige Inventar mit SHA-256 steht in `internal/webui/assets-manifest.json` (50 Dateien).
