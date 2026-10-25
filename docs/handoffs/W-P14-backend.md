# Handoff W-P14 Backend – Suche und Ersetzen

Stand: 24.10.2026. Implementierung im vorhandenen Worktree `.worktrees/w-p14-backend`,
Branch `feat/w-p14-backend`, unveränderte Basis/HEAD
`b74d10e6ff50ed49dccb0484ee3f8d2f0ca5b656`. Alle Paketänderungen liegen als Dateien vor.
**Keine Git-Schreibbefehle, kein Commit, kein Push, keine Integration.** Lead übernimmt
Commit und unabhängige Abnahme. Kein Vertragsfreeze oder Web-Pin behauptet.

## Ergebnis und Dateibesitz

- `internal/core/search.go`: additive Typen, Suchmodi, Treiberfähigkeiten und feste Grenzen.
- `internal/database/search.go`: kataloggeprüfte Mehrtabellen-/Mehrspaltensuche mit
  Teilstring, allen Wörtern, exakt und nativer Regex; gebundene Werte, explizites
  LIKE-Escape `!` in allen Dialekten, gemeinsames Trefferbudget und gekürzte Vorschau.
  Erweiterte UND-Filter ausschließlich über v15 (`ne/lt/lte/gt/gte/contains/like`).
- `internal/database/search_replace.go`: Plan/Bestätigung je Tabelle/Spalte,
  vollständiger kanonischer Bestandsvergleich im Hash, native REPLACE-/REGEXP_REPLACE-
  Berechnung, transaktionale PK-Updates, Rücklesen und Rollback bei Konflikten.
  Ergebnis `completed/rejected/outcome_unknown`; keine rohen Treiberdiagnosen.
- `internal/httpapi/search.go`: authentifizierte `/api/v15/capabilities`, `/search`,
  `/tables/browse`, `/replace/plan`, `/replace`, strikte Dekodierung und Feldpräsenz.
  `features.go`: genau ein zusätzlicher Eintrag; `features_test.go`: nur Zähler 8→9.
- `internal/database/search_test.go`, `internal/httpapi/search_test.go`: SQLite-,
  HTTP- und vorbereitete TLS-Livetests für MySQL/MariaDB/PostgreSQL.
- `contracts/api-v0.15.0.json`, `contracts/SEARCH-API.md`,
  `contracts/fixtures/search/` (25 JSON-Fixtures plus README; gültige/ungültige Fälle,
  Hash-Platzhalter ausdrücklich gekennzeichnet).
- `scripts/check-search-contract.py`, `scripts/test-search.py` (dbfix, optional
  `--sqlite-only`, echte Antworten einschließlich HTTP gegen Schema geprüft),
  `.github/workflows/search.yml` für native Linux- und Vier-Treiber-Prüfung.
- `docs/decisions/ADR-0008-suche-ersetzen.md`: konservative vollständige Tabellen-
  begrenzung und Konfliktmodell; Nummer/Entscheidung vom Lead unabhängig prüfen.
- Dieses Handoff. Zentrale Status-/Plan-/Vertragsindexdateien, Bestandsverträge,
  Web-Lock, Assets, Module und bestehende Mutation-/Browseimplementierungen unverändert.

Vorläufiger Vertrag 0.15.0, SHA-256:
`bd82f86aff1f9551f1abc96036f4246f72580da3fd1674d1f48dcf286cef6a20`.
Wächter prüft eingefrorene v1–v8-Bytes, unveränderte Bestandsdefinitionen/-routen,
Auth, lokale Referenzen, strikte neue Objekte und Fixtures. Keine Clientbindung vor
unabhängiger Freigabe.

## Tatsächlicher Prüfstand

Go 1.27.1, macOS arm64. Für Go-Prüfungen verwendet:
`GOCACHE=/private/tmp/dbmanager-w-p14-go-cache GOPROXY=off`.
Der Standard-Go-Cache ist in dieser Sandbox nicht beschreibbar; der erste normale
Teststart scheiterte schon vor dem Kompilieren. Mit temporärem Cache liefen alle
unten als erfolgreich bezeichneten Go-Prüfungen ohne Netzwerkabruf.

| Befehl | Tatsächliches Ergebnis |
| --- | --- |
| `gofmt -l $(rg --files -g '*.go' internal cmd)` | keine Ausgabe |
| `go vet ./...` | erfolgreich, keine Ausgabe |
| `go test -race -count=1 ./...` | ausgeführt, **nicht grün**: ausschließlich vorhandener `TestMySQLPerConnectionLoggerSuppressesServerInput` scheitert an `listen tcp 127.0.0.1:0: bind: operation not permitted`; übrige Pakete grün, Livefälle ohne Fixtureports übersprungen |
| `go test -race -count=1 ./... -skip TestMySQLPerConnectionLoggerSuppressesServerInput` | erfolgreich für sämtliche Pakete; genau dieser bestehende Listener-Test ausgenommen, keine Sandboxumgehung |
| `python3 -B scripts/test-search.py --sqlite-only` | erfolgreich mit Race; `TestSQLiteSearch`, `TestSQLiteReplaceSafety`, `TestSQLiteSearchBoundaries`, `TestSearchHTTP`; echte Engine-/HTTP-Antworten gegen v15-Schema validiert; drei Netzwerk-Subtests ausdrücklich SKIP |
| `python3 -B scripts/test-search.py` | versucht, bereits bei lesendem `docker ps` abgebrochen: Zugriff auf Docker-API-Socket verweigert; **kein** Container gestartet |
| `python3 -B scripts/check-table-contract.py` | erfolgreich |
| `python3 -B scripts/check-mutation-contract.py` | erfolgreich |
| `python3 -B scripts/check-insert-contract.py` | erfolgreich |
| `python3 -B scripts/check-structure-contract.py` | erfolgreich |
| `python3 -B scripts/check-create-contract.py` | erfolgreich |
| `python3 -B scripts/check-overview-contract.py` | erfolgreich, 32 Fixtures |
| `python3 -B scripts/check-object-ops-contract.py` | erfolgreich, 80 Fixtures |
| `python3 -B scripts/check-search-contract.py` | erfolgreich, additive v15-Schemas und alle 25 eigenen JSON-Fixtures |
| `python3 -B scripts/check-docs.py` | erfolgreich, null Fehler; nach Handoff erneut geprüft |
| `git diff --check` | erfolgreich (nur lesend) |
| `python3 -B -m py_compile scripts/check-search-contract.py scripts/test-search.py` | erfolgreich |

Belegte W-P14-Fälle: Sondernamen mit Quotes/Backticks/SQL-Text, gebundene
Injektionswerte, literale `%/_/!`, Wörter über verschiedene Spalten, exakte Suche,
NULL, Binärspalten und SQLite-Laufzeit-BLOBs ausgeschlossen, gemeinsames Budget und
explizit ungenaue Zählung, SQL-seitige Vorschaukürzung, ungültige Grenzen und Abbruch/
Zeitlimit. Plan vor/nach mit vollständigen gelesenen Resultaten verglichen: keine
Datenänderung. Ersetzung nativ berechnet, stale Hash bei geänderter fremder Spalte,
Replay nach geänderten Daten abgewiesen, PK/Generated/Binär/NoPK gesperrt, leere
Ersatzwerte und No-op, erzeugte Nachherwerte, Constraint-/REPLACE-Schutz, Trigger-
Nebenänderung mit Rollback, konkurrierende Bestätigungen höchstens ein Erfolg,
echter verzögerter FK-Commitfehler ergibt `outcome_unknown`. HTTP: Auth, fehlend/NULL,
Bestätigungsanzahl, fremde Felder, 409 und redigierte 404.

Eine zusätzlich geschriebene große SQLite-Fixture überschritt zunächst die bereits
bestehende Zellgrenze. Fixture auf 100000 Zeichen verkleinert; die abschließende
Grenzfallserie und die Gesamtsuite mit dem benannten Listener-Ausschluss sind grün.
Dies war kein fehlgeschlagener Produktfall.

## Exakt nicht ausgeführte oder nicht erfolgreich abgeschlossene Nachweise

1. **Keine realen W-P14-Läufe gegen MySQL 8.4, MariaDB 12.3 oder PostgreSQL 18.6.**
   Docker-Socket gesperrt; damit auch deren native Regex-/Ersetzungssemantik,
   PostgreSQL-Parameterslots der erweiterten Filter, TLS-verify-full-Verbindungen,
   gehaltene Rowlock-Deadline, MyISAM-/Vererbungsverbote und redigierte Regexfehler
   noch nicht live belegt. Diese Fälle sind in `TestLiveSearch` und seinen Helfern
   geschrieben und über `scripts/dbfix.py` ausführbar.
2. **Kein erfolgreicher uneingeschränkter Gesamtracelauf.** Der bestehende
   MySQL-Logger-Regressionstest braucht einen lokalen Listener, dessen Bindung die
   Sandbox untersagt. Er wurde nicht verändert; der erfolgreiche Ersatzlauf
   schließt ausschließlich diesen Test aus.
3. **Keine neue Linux-amd64-CI**, kein Docker-/Compose-/Swarm-, Web-/Browser-/Build-
   oder Release-Lauf. Neue Workflowdatei nur vorbereitet, nicht veröffentlicht.
   Keine tatsächliche Netzwerkunterbrechung genau während COMMIT reproduziert.
4. **Kein unabhängiger Review, Vertragsfreeze, Commit oder Push.** Dateiarbeit ist
   abgeschlossen; diese Gates und die Integration liegen beim Lead.

## Grenzen und nächster konkreter Schritt

Ersetzung begrenzt absichtlich den **vollständigen** Tabellenbestand auf Standard
100, maximal 1000 Zeilen (nicht nur Treffer). Darüber 422 statt Teilersetzung.
Search-Vorschau ist auf 256 Zeichen gekürzt und kein Mutationssnapshot. Kollation,
case-sensitive Literalersetzung und Regex-Rückreferenzen sind treiberspezifisch;
SQLite bietet keine Regex. Kein vollständiger QBE-/Join-/gespeicherte-Suche-Editor.

Wie B2c: kanonische Zellen, kein historischer ABA-/Exactly-once-Nachweis; externe
Trigger-/Kaskadeneffekte und nichttransaktionale Nebentabellen bleiben begrenzt.
MySQL Read-Committed verspricht keinen Phantomschutz nach dem letzten Snapshot.
Nur transaktionale Basistabellen mit stabilem PK; PostgreSQL keine Partitionen,
Vererbung oder Regeln. Keine PK-/Generated-/Binärersetzung. Bestehende SQLite-VFS-,
Dateibesitz-/SingleReplica- und vier globale Anfragegrenzen bleiben bestehen.
Keine neuen Abhängigkeiten, Datenmigrationen oder Lizenzänderungen; neuer Code AGPL-3.0-only.

Keine eigenen Container gestartet, keine laufenden Prüfjobs bei Übergabe.
Prozessauflistung über `ps` wurde durch die Sandbox untersagt; eigene über Tools
bekannte Sitzungen wurden bis zum Abschluss verfolgt. Temporärer Go-Cache bleibt
außerhalb des Repositories; dbfix-SQLite-/Ergebnisverzeichnisse wurden aufgeräumt.
Unintegrierte Arbeit: sämtliche oben genannten Dateien im vorhandenen Branch.
Nächster Schritt für den Lead: Diff unabhängig prüfen lassen, Vier-Treiber-Suite
und uneingeschränkten Racelauf auf geeignetem Host ausführen, ADR/API-Nummer und
Vertrag freigeben, anschließend selbst committen und integrieren.
