# Handoff W-P14 Backend – Suche und Ersetzen (API 0.14)

Stand: 25.10.2026. Implementierungskandidat im zugewiesenen Worktree
`.worktrees/w-p14-backend`, Branch `feat/w-p14-backend`. Verifizierter HEAD
`b5651cc5cac140fcc919827b107eba522762436d`: enthält den unreviewten WIP
`f9ffa3d3ccd3ec6fd322f28c951325be073f1c45` und den bereits abgeschlossenen Merge
von main `c331c64`. Die hier beschriebenen Nacharbeiten sind **uncommittete Dateien**.
Keine Git-Schreibbefehle, kein Commit, Push oder Merge in dieser Sitzung.
Lead übernimmt Commit, unabhängigen Review und Integration. Kein Vertragsfreeze,
Web-Pin, Deployment oder vollständiger Paritätsnachweis.

## Auftrag, Dateibesitz und Ergebnis

Begrenztes Backendpaket für PMA-DATA-010 (erweiterte gebundene Filter),
PMA-DATA-011 (gewählte Tabellen/Spalten) und PMA-DATA-012 (Plan und bestätigte
Einspaltenersetzung). Abnahme: reine Vorschau, gebundene Werte und kataloggeprüfte
Identifier, harte Grenzen/Abbruch, genau bestätigte Änderungen, Konfliktprüfung
und ehrliche Ergebnisse. Keine Weboberfläche oder zentralen Statusdateien geändert.

- `internal/core/search.go`: Typen, feste Grenzen, treiberspezifische Fähigkeiten,
  endgültige Versionskonstante `0.14.0`.
- `internal/database/search.go`: Mehrtabellen-/Mehrspaltensuche; Teilstring, alle
  Wörter, exakt und native Regex; Werte einschließlich Limits gebunden, explizites
  LIKE-Escape `!`, gemeinsames Trefferbudget und gekürzte Vorschau. Erweiterte
  UND-Filter `ne/lt/lte/gt/gte/contains/like` ausschließlich über v14.
- `internal/database/search_replace.go` (aus dem WIP übernommen und geprüft):
  Plan/Bestätigung je Tabelle/Spalte; vollständiger kanonischer Bestandsvergleich
  im Hash, native Berechnung, transaktionale PK-Updates und Nachprüfung vor Commit.
  Ergebnisse `completed/rejected/outcome_unknown`, feste redigierte Fehlergründe.
- `internal/httpapi/search.go`: authentifizierte `/api/v14/capabilities`, `/search`,
  `/tables/browse`, `/replace/plan`, `/replace`; strikte Dekodierung/Feldpräsenz.
  Bereits aufgelöste `features.go`/`features_test.go` unverändert, Zähler **13**.
- `internal/database/search_test.go`, `internal/httpapi/search_test.go`: SQLite,
  HTTP und geschriebene TLS-Livetests für MySQL/MariaDB/PostgreSQL. Neu: sichere
  Filtererzeugung bei SQL-Schlüsselwörtern und `$1`/`$2` in gequoteten Katalognamen;
  SQL-Vergleich für alle Dialekte und echte SQLite-Ausführung.
- `contracts/api-v0.14.0.json`, `contracts/SEARCH-API.md`,
  `contracts/fixtures/search/`: 25 synthetische JSON-Fixtures; Hash-Platzhalter
  ausdrücklich gekennzeichnet. Provisorische v15-Datei entfernt.
- `scripts/check-search-contract.py`, `scripts/test-search-guard.py`,
  `scripts/test-search.py`: eingefrorene Vorgänger, Additivität, Schema-/Fixture-
  Prüfung; isolierte Guardregressionen; dbfix-Suite mit Schemaabgleich echter
  Antworten und `--sqlite-only`. Netzwerkfixtures jetzt strikt nacheinander.
- `.github/workflows/search.yml`: API 0.14, vollständige Race-/Vet-/Guard-/Livechecks
  vorbereitet. `docs/decisions/ADR-0008-suche-ersetzen.md`: vollständige begrenzte
  Bestandsprüfung; unabhängige Annahme durch Lead offen. Dieses Handoff.

Keine neuen Abhängigkeiten, Migrationen oder Lizenzänderungen; eigener Code mit
SPDX AGPL-3.0-only. Gemeinsame Helper, bisherige Browse-/Mutationsimplementierungen,
Vertragsdateien v1–v12, Vertragsindex, zentrale Pläne, Web-Lock und Assets unverändert.

## Vertragsbasis und Übergang zu v13

Endgültige Paketnummer **0.14**, Pfade `/api/v14/`, Vertragsdatei
`contracts/api-v0.14.0.json`. SHA-256:
`fda8077af88401eb7a306dba8aedb963d6befc98e43a098b751d381e6539206c`.

Vertrag additiv auf dem tatsächlichen v12-Vertrag aufgebaut; sämtliche geerbten
Pfade, Schemata und Authdefinitionen unverändert. Wächter pinnt die Bytes von
**v1 bis v12**. `api-v0.13.0.json` (paralleles W-P09) fehlt hier und ist optional:
Sobald vorhanden, verlangt der Wächter auch dessen Versionsnummer, unveränderte
v12-Definitionen und additive Übernahme sämtlicher v13-Pfade/-Schemata in v14.
Der Lead muss diese Übernahme beim Merge ausführen; eine bereits erfolgte
Additivität gegenüber dem noch fehlenden v13 wird nicht behauptet. Guardtests
belegen Abwesenheit, fehlende Übernahme (Ablehnung), übernommene Vorgänger sowie
manipulierte v12-Bytes/-Definitionen (Ablehnung), ausschließlich in temporären Kopien.

## Tatsächlicher Prüfstand dieser Sitzung

Go 1.27.1, macOS arm64; alle Go-Befehle einschließlich Python-Teststarter mit
`GOCACHE=/private/tmp/dbmanager-w-p14-go-cache GOPROXY=off` ausgeführt.
Keine Netzwerkdownloads oder Änderungen an Modulen. `rg` fehlt; Dateisuche mit
`grep`/Python durchgeführt. Alle folgenden Ergebnisse stammen vom aktuellen Paket,
keine Übernahme früherer Erfolgsaussagen aus dem WIP-Handoff.

| Befehl | Tatsächliches Ergebnis |
| --- | --- |
| `gofmt -l .` | bestanden, keine Ausgabe |
| `go vet ./...` | bestanden, keine Ausgabe |
| `go test -race -count=1 ./...` | ausgeführt, nicht grün: ausschließlich bestehender `TestMySQLPerConnectionLoggerSuppressesServerInput` scheitert an verbotenem lokalem TCP-Listener (`bind: operation not permitted`); übrige Pakete grün |
| `go test -race -count=1 ./... -skip TestMySQLPerConnectionLoggerSuppressesServerInput` | alle Pakete bestanden, genau dieser Bestandsfall ausgenommen; Netzwerk-Livefälle ohne Fixturevariablen übersprungen |
| `python3 -B scripts/test-search.py --sqlite-only` | bestanden mit Race; SQLite-Suche, Safety/Boundaries, gequotete Namen, Drei-Dialekt-SQL-Erzeugung und HTTP; echte Engine-/HTTP-Antworten gegen v14 validiert, drei Netzwerkfälle SKIP |
| `python3 -B scripts/test-search.py` | versucht, bereits vor Fixtureanlage bei lesendem `docker ps` abgebrochen: Docker-API-Socketzugriff verweigert, kein Container gestartet |
| `python3 -B scripts/test-search-guard.py` | bestanden; optionale v13-Konfigurationen, Additivitäts- und Byte-Manipulationen isoliert geprüft |
| `python3 -B scripts/check-table-contract.py` | bestanden |
| `python3 -B scripts/check-mutation-contract.py` | bestanden |
| `python3 -B scripts/check-insert-contract.py` | bestanden |
| `python3 -B scripts/check-structure-contract.py` | bestanden |
| `python3 -B scripts/check-create-contract.py` | bestanden |
| `python3 -B scripts/check-overview-contract.py` | bestanden, 32 Fixtures |
| `python3 -B scripts/check-object-ops-contract.py` | bestanden, 80 Fixtures |
| `python3 -B scripts/check-columns-contract.py` | bestanden, 44 Fixtures |
| `python3 -B scripts/check-maintenance-contract.py` | bestanden, 83 Fixtures |
| `python3 -B scripts/check-export-contract.py` | bestanden, 23 JSON- und 3 Streamfixtures |
| `python3 -B scripts/check-sqlbatch-contract.py` | bestanden, 23 Fixtures |
| `python3 -B scripts/check-search-contract.py` | bestanden, v1–v12 gepinnt, v13 ausdrücklich abwesend/optional, additive v14-Schemas und 25 W-P14-Fixtures |
| Python-Syntaxprüfung mit `compile()` ohne Bytecodeausgabe für die drei Searchskripte | bestanden |
| `python3 -B scripts/check-docs.py` | bestanden, null Fehler, nach Handoff erneut geprüft |
| `git diff --check` | bestanden, nur lesend |

Belegte Fälle: Sondernamen, gebundene Injektionswerte, literale `%/_/!`, Wörter über
Spalten, exakt, NULL, Binär-/SQLite-Laufzeit-BLOB-Ausschluss, Trefferbudget und
explizit ungenaue Zählung, SQL-seitige Vorschaukürzung, Grenzen und Abbruch/Timeout.
Vorschau per vollständigem Vorher/Nachher-Vergleich ohne Änderung geprüft.
Ersetzung: stale Hash bei Änderung einer fremden Spalte, Replay nach geänderten
Daten, PK/Generated/Binär/NoPK-Verbot, leere Ersatzwerte, No-op, generierte
Nachherwerte, Constraint-/SQLite-REPLACE-Schutz, Triggerkonflikt mit Rollback,
konkurrierende Bestätigungen (genau ein Erfolg) und tatsächlicher verzögerter
SQLite-FK-Commitfehler (`outcome_unknown`). HTTP prüft Auth, fehlend/NULL,
Bestätigungszahl, fremde Felder, 409 und redigierte 404.

Bei der Nacharbeit wurde ein WIP-Fehler in erweiterten Filtern geschlossen:
Globale Parameterslot-Ersetzung und die Suche nach `ORDER BY`/`LIMIT` konnten
gequotete Katalognamen verändern. Jetzt wird der exakte katalogbasierte SELECT-
Präfix verwendet und die Pagination in derselben Parameterfolge gebunden.
Die neuen Regressionen sind im aktuellen Race-/SQLite-Lauf grün.

## Nicht ausgeführte Nachweise und Grenzen

1. **Keine W-P14-Liveläufe gegen MySQL 8.4, MariaDB 12.3 oder PostgreSQL 18.6.**
   Docker ist gesperrt. Deren Regex-/Ersetzungssemantik, TLS verify-full,
   Rowlock-Deadline, MyISAM-/Vererbungsverbote und redigierte Regexfehler sind als
   `TestLiveSearch` mit dbfix vorbereitet, hier nicht live belegt. SQL-Erzeugung
   für PostgreSQL ist offline geprüft, ersetzt keinen echten PG-Lauf.
2. **Kein uneingeschränkt erfolgreicher Gesamtracelauf:** Listener-Test wegen
   Sandboxverbot fehlgeschlagen, unverändert belassen. Nur der Lauf mit dem exakt
   benannten Ausschluss ist grün. Keine tatsächliche Netzwerkunterbrechung während
   COMMIT reproduziert; keine echten Rechte-/Verbindungsverlustproben gegen die
   Netzwerkengines in dieser Sitzung.
3. **Keine Linux-amd64-CI, Docker-/Compose-/Swarm-, Web-/Browser-/Build-/Release-
   Prüfung.** Workflow nur vorbereitet; keine Veröffentlichung oder Integration.
4. **Unabhängiger Review und Vertragsfreeze offen.** Der Lead reconciliert v13,
   entscheidet über ADR und nimmt das Backend getrennt von Web/Parität ab.

Ersetzung begrenzt den **vollständigen** Tabellenbestand auf Standard 100,
maximal 1000 Zeilen, darüber 422 statt Teilersetzung. Suche zählt nur bis zum
Trefferbudget; 256-Zeichen-Vorschau ist kein vollständiger Mutationssnapshot.
Kollation, case-sensitive Literalersetzung und Regex-Rückreferenzen sind
DB-spezifisch; SQLite ohne Regex. Clients sperren nach Abschluss/unbekanntem
Ausgang und verlangen neue Planung/Bestätigung, kein automatischer Retry.

B2c-Modell: vollständige kanonische Cells, keine historische ABA-/Exactly-once-
Garantie. Externe Trigger-/Kaskadeneffekte und nichttransaktionale Nebentabellen
bleiben begrenzt. MySQL Read-Committed verhindert keine Phantome nach dem letzten
Snapshot. Nur transaktionale Basistabellen mit stabilem PK, PostgreSQL ohne
Vererbung/Partitionen/Regeln, keine PK-/Generated-/Binärersetzung. SQLite-VFS,
SingleReplica und globale Anfragegrenzen bleiben unverändert.

## Fortsetzung und Ressourcen

Keine eigenen Container angelegt, temporäre dbfix-/Guard-/Antwortverzeichnisse
aufgeräumt. Alle eigenen Prüfprozesse sind abgeschlossen; keine eigenen Jobs offen.
Temporärer Go-Cache bleibt außerhalb des Repositories. Unintegriert sind die oben
beschriebenen Änderungen gegenüber HEAD; `git status --short` zeigt auch die neue
v14-Datei und das neue Guardtestskript, die der Lead beim Commit aufnehmen muss.

Nächster konkreter Schritt: Lead lässt den Diff unabhängig prüfen, führt auf einem
geeigneten Host `python3 -B scripts/test-search.py` und den uneingeschränkten
Gesamtracelauf aus, übernimmt den finalen v13-Vertrag additiv in v14, prüft alle
Wächter erneut und committet/integriert nach Freigabe selbst.
