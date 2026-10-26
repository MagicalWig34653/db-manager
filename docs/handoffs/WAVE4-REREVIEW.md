<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Unabhängiges Re-Review Welle 4 (Nachbesserungen v23)

Stand: 26.10.2026. Geprüft: `integration/wave4-backend` @ `d66a59a` (Review-Branch `review/wave4-rereview`).
Unabhängiger Reviewer, nichts implementiert, kein Produktcode geändert, nichts gemergt. Dieser Bericht ist ein
Prüfergebnis, keine Annahme von Vertrag oder ADR.

## Urteil

**Nicht freigegeben: ein Blocker (B1).** Die behauptete Behebung von Review A F1 (Einmal-Plan) wirkt nur auf Engine-Ebene,
nicht auf der echten HTTP-Route `POST /api/v18/imports/execute`: ein erneut gesendeter, bestätigter CSV-Import fügt die Daten
ein zweites Mal ein. Alle übrigen Auflagen sind behoben oder nur mit niedriger Restabweichung (F6 auf PostgreSQL, F5 Speicher).
Die Vertragskette (v1 bis v17 unverändert zu main, v18 bis v23 gepinnt, Superset) ist korrekt.

| Auflage | Ergebnis |
| --- | --- |
| A-F1 Einmal-Plan (Ledger) | **Teilweise: Blocker B1** (HTTP-Import wiederholbar); Ledger selbst korrekt |
| A-F2 SQL-Import-Warnung | behoben (Warnung mit Klassenzählung) |
| A-F3 `rolled_back` / Vertrag | behoben; Fixture fehlt (N3) |
| A-F4 Dateiersetzen-Budget | behoben (512 KiB ersetzt 512 KiB, +1 Byte gibt `file_too_large`; SQLite, MySQL, MariaDB, PostgreSQL) |
| A-F5 Importspeicher | verbessert, Vertragsangabe stimmt nicht mehr (N1) |
| A-F6 Abbruch einheitlich | **nicht auf PostgreSQL behoben** (N2) |
| B-F1 YAML-Escapes | behoben (PyYAML, CSafeLoader, Psych verlustfrei, auch Schlüssel/Namen) |
| B-F2 Monitor-Fehlerklassen | behoben (live auf MySQL, MariaDB, PostgreSQL) |
| B-F3 Server-Admin Zähler | behoben, keine Secrets geöffnet |
| v23 `grant_without_effect` und Reason-Test | behoben, Mutation schlägt an |

## Befunde nach Schwere

### B1 (Hoch, Blocker) HTTP-Importausführung schaltet den verbrauchten Plan selbst wieder frei
`internal/httpapi/import.go` (`importUpload`, Zweig `execute`, Format `csv`) ruft vor dem Jobstart
`engine.PlanImport(...)` als Vorabprüfung auf. `PlanImport` ruft `e.plans.arm("import:"+hash)` auf und löscht damit den
Verbrauchseintrag. Die anschließende `ExecuteImport` im Job verbraucht den Hash erneut und läuft durch. Folge: dieselbe bestätigte
Execute-Anfrage wird beliebig oft akzeptiert (jedes Mal `202`, jedes Mal wird eingefügt). Genau der Netzwerk-Retry-Fall aus Review A F1.
Die Replay-Tests der Autoren (`TestReviewA_CSVImportReplayRefused`) rufen die Engine direkt auf und sehen die HTTP-Route nicht.
Reproduktion (neu, SQLite, Tabelle ohne Schlüssel): `DBM_REREVIEW_FINDINGS=1 go test ./internal/httpapi -run TestReReview_ImportExecuteReplayOverHTTP`:
zwei identische Execute-Aufrufe ergeben `[202 202]`, Tabelle enthält 4 statt 2 Zeilen. Bulk (`rowsbulk` Plan/Execute getrennte
Routen, Execute ruft nie `arm`) ist nicht betroffen; SQL-Import ist planlos und bleibt wiederholbar (dokumentiert, F2).
Empfehlung: Vorabprüfung ohne Freischaltung (interne Variante `planImport(arm bool)` oder Prüfung nur durch `ExecuteImport`), `arm` ausschließlich auf der
Plan-Route; HTTP-Regressionstest wie oben. Parallele Executes desselben Plans: genau einer läuft (getestet, kein Race).

### N1 (Mittel) Speicher: Vertragsangabe "zwei Slots unter etwa 300 MiB Spitze" stimmt nicht für die Ausführung
Messung (`DBM_REREVIEW_HEAP=1 go test ./internal/httpapi -run TestReReview_ImportPeakHeap -v`, echter Server, gestreamte 61-MiB-CSV mit 61 Zeilen
zu 1 MiB, zwei Slots parallel auf zwei SQLite-Dateien, `HeapAlloc` Spitze über Ruhebasis, einschließlich noch nicht eingesammelter Garbage):
- Plan, 2 Slots: Spitze 320 MiB (Delta 318 MiB), etwa 157 MiB je Slot (Review A: etwa 310 MiB für einen Slot, also Halbierung).
- Execute, 2 Slots: Spitze 503 MiB (Delta 501 MiB), etwa 250 MiB je Slot (Datei, Entpackung/Dekodierung, zweite Vorab-Plan-Dekodierung im Handler, Jobdaten).
`contracts/IMPORT-API.md` Z. 43 f. nennt "unter etwa 300 MiB". Gilt nicht für Execute. Empfehlung: Angabe korrigieren (etwa 500 MiB) und Containerlimit
dokumentieren, oder Vorabplan im Handler entfallen lassen (zugleich B1-Fix). Kein Containerlimit im Repo gesetzt.

### N2 (Niedrig) F6: Abbruch auf PostgreSQL weiter `outcome_unknown` mit `inserted=500`
Reproduzierbar 6 von 6 Läufen (`python3 scripts/test-rereview-live.py TestReReview_CSVCancelUniform`, PostgreSQL 18.6): Ausgabe
`outcome=outcome_unknown reason=query_canceled inserted=500`, Tabelle enthält 0 Zeilen. MySQL, MariaDB, SQLite: `rejected`, `inserted=0`.
Ursache: `tx.Rollback()` liefert bei pgx nach Abbruch `timeout: context already done: context canceled`, nicht `sql.ErrTxDone`; die
Behandlung in `rollbackImport` greift nur für `ErrTxDone`. Das Verhalten ist ehrlich konservativ (kein falsches `completed`), aber nicht einheitlich;
die Zusage "auf allen Engines" im Handoff stimmt nicht für PostgreSQL. Mutation `ErrTxDone`-Toleranz entfernt überlebt alle Live-Tests (Testlücke).
Empfehlung: Abbruch vor COMMIT mit Kontextfehler auch als zurückgerollt werten (Verbindung ist serverseitig beendet) oder `inserted` bei `outcome_unknown` nicht
als gesicherte Zahl ausgeben; Test `DBM_REREVIEW_FINDINGS=1`.

### N3 (Niedrig) `rolled_back` in v18 ohne Import-Fixture
`ImportStatementResult.outcome` enthält `rolled_back`, aber unter `contracts/fixtures/import/` existiert weder ein echtes noch ein Platzhalter-Fixture
mit diesem Wert (die gefundenen `rolled_back`-Fixtures gehören zu `sqlbatch`). Der Wächter validiert den Wert also nie gegen eine Antwort. Empfehlung:
`ImportResult.real-sqlite-rollback.json` aus `TestReviewA_SQLImportPartialStateHonesty` erzeugen.

### N4 (Niedrig) Verbrauchter Hash nach Fehler vor dem ersten Schreiben: irreführende Meldung
Ein fehlgeschlagener Execute (z. B. UNIQUE-Konflikt beim Kopieren, nichts geschrieben) verbraucht den Hash; die Wiederholung meldet
"Plan wurde bereits ausgeführt. Neue Vorschau erforderlich." statt eines Hinweises "nicht ausgeführt". Der Weg ist korrekt (neuer Plan-Aufruf gibt frei;
verifiziert), die Aussage "bereits ausgeführt" ist aber falsch. Test `TestReReview_FailedBulkExecuteConsumesPlan` (Log).
Das ist sonst folgerichtig: Hash wird vor dem ersten Schreiben verbraucht, auch bei `outcome_unknown`, Nutzer wird nie in einer Sackgasse gelassen.

### N5 (Niedrig) Ledger-Eigenschaften (Info)
Race: 200 Runden mal 16 parallele `consume` desselben Hashs, genau ein Gewinner (`-race`). Speicher: harte Obergrenze 4096 Einträge, bei Vollstand
Entfernen abgelaufener Einträge, sonst des ältesten; nach 4096 weiteren Ausführungen oder 1 h ist ein Replay des ältesten Plans wieder möglich (dokumentiert
"nach Neustart"; die Eviction-Grenze fehlt im Vertragstext). Jeder Plan-Aufruf mit gültigem Token gibt frei (gewollt, kein Rechteproblem bei Einzeltoken).
Ein Plan-Aufruf während eines laufenden Jobs gibt einen zweiten Lauf frei (durch B1 ohnehin erreichbar).

### N6 (Niedrig/Info) Weitere
- `TestReviewA_GzipBomb` schlug im ersten vollen `go test -race ./...` einmal fehl (Heap 475 MiB gegen Schwelle 400 MiB), isoliert und im zweiten vollen Lauf grün: lastabhängig flaky (GC-Zeitpunkt).
- v23 `info.description` nennt noch W-P29; der Handoff behauptet Beschreibungsänderungen in v23, tatsächlich ändert sich nur das `reason`-Enum (zusätzlich `info.version`).
- `scripts/contract_chain.py` pinnt v23 selbst nicht (erst mit v24 nötig).

## Prüfung der einzelnen Punkte

1. **Plan-Ledger** (`plan_ledger.go`): atomar unter Mutex; Race-Test grün; Eviction/TTL/Purge getestet (neue Tests `TestReReview_PlanLedger*`);
   Mutationen: consume immer wahr, arm ohne Wirkung, consume im Import/Bulk entfernt, keine Eviction, keine Ablaufbereinigung, Mutex entfernt: alle getötet.
   Freigabe durch Plan-Aufruf: gewollt; B1 zeigt den Missbrauchs-/Fehlerpfad. Fehlermeldungen: Import `plan_mismatch`, Bulk `plan_conflict`, Text "Neue Vorschau erforderlich" klar (N4).
2. **Vertrag v18 bis v23**: v1 bis v16 SHA-256 identisch zu main `3a7e1f1` (Byte-Vergleich, 16 von 16). v17 bis v23 nicht auf main. `contract_chain.check(23)` grün; eigener
   Strukturvergleich: v18 bis v22 je +Pfade/Schemas, keine Änderung oder Entfernung außer `info`; v22 nach v23 nur `AccountResult.reason` + `grant_without_effect`;
   v17 nach v23: nichts entfernt, einzige Änderung am Schema `AccountResult` (Enum-Erweiterung). v18 weicht von `feat/w-p12-backend` nur im `rolled_back`-Wert
   von `ImportStatementResult.outcome` ab; v19 gleich `feat/w-p13-backend` (Delta). Manipulationsproben: Byte in v5 geändert, Pfad aus v23 entfernt: Wächter schlägt an.
   Fixtures `contracts/fixtures/reasons/` real (Konto-Antwort) und ungültig (unbekannter Wert) geprüft; Import-Fixture für `rolled_back` fehlt (N3).
3. **F4/F5/F6**: siehe Befunde N1, N2; F4 sauber (alle vier Systeme).
4. **YAML**: neuer Sweep `TestReReviewYAMLSweep` mit etwa 14.000 Codepunkten (U+0001 bis U+30FF, Randbereiche U+D7F0, U+E000, U+FDD0 bis U+FDEF, U+FFF0 bis U+FFFF,
   Plane 1 bis 16 inkl. U+1FFFE/U+10FFFF) als Zellwerte, Spaltennamen und Tabellennamen; Gesamtdokument gegen JSON-Export verglichen:
   PyYAML safe 6.0.3, CSafeLoader und Ruby 2.6 Psych: alle OK (`scripts/review-wave4-rereview-yaml.py`). Mutationen: U+007F, C1-Bereich, U+FFFE, U+FFFF, Header-Namenspfad entfernt: alle getötet.
5. **Monitor**: Falsches Passwort (sample/advisor/analysis/engines) ist auf MySQL 8.4, MariaDB 12.3, PostgreSQL 18.6 ein Fehler, ohne Passwort, `Access denied` oder SQLSTATE im Text;
   Profiling mit fehlender Tabelle/Spalte immer Fehler; Klassifikationstabelle (1044/1045/1049/2002 und 28P01/28000/3D000/57014 echte Fehler; 1142/1143/1227 und 42501/42P01 `unavailable`) getestet.
   Mutationen (1045/1044 unavailable, Profiling-Ausnahme entfernt): getötet (Profiling nur live). Hinweis: MySQL 1064 bleibt für Nicht-Profiling-Aktionen `unavailable` (Syntaxunterschiede MariaDB/MySQL), gewollt.
6. **Server-Admin**: Allowlist exakter Namen `max_connections`, `connections`, `max_used_connections`, `max_user_connections` vor Sensitivsperre; nur numerische Werte bis 64 Zeichen;
   Variablen zusätzlich auf feste Namen beschränkt (Groß-/Kleinschreibung fail-closed). Keine Secrets, Pfade, Hosts. Mutationen (Allowlist leer, Sensitivliste leer): getötet. Live-Skript grün.
7. **v23/Reason-Test**: `TestEmittedReasonsAreInLatestContract` schlägt an bei (a) Wert im Code, der nicht im Enum steht und (b) entferntem Enum-Wert in v23. Grenze: Der Test findet
   nur Literale der Muster `Reason:`, `.Reason =`, `reject(` und die Tabellen `reasonText`/`columnsFailure`; indirekt zusammengesetzte Reasons sieht er nicht.

## Ausgeführte Prüfungen

| Prüfung | Ergebnis |
| --- | --- |
| `gofmt -l .`, `go vet ./...` | leer / ok |
| `go test -race -count=1 ./...` | 2 Läufe: erster Lauf `TestReviewA_GzipBomb` einmal fehlgeschlagen (N6), zweiter Lauf alle Pakete grün |
| alle `scripts/check-*-contract.py`, `check-docs.py` | grün |
| `test-import.py --live`, `test-rows-bulk.py`, `test-server-admin.py`, `test-monitor.py`, `test-extended-export.py --live`, `test-partitions.py`, `test-review-a-live.py` | alle Exit 0 (SQLite, MySQL 8.4, MariaDB 12.3, PostgreSQL 18.6; Wegwerfcontainer nacheinander, keine Reste) |
| `scripts/review-wave4b-live.py` | grün; Fehlerklassen wie oben |
| neue Tests `TestReReview_*` (SQLite immer; Live via `scripts/test-rereview-live.py`) | grün bis auf die gegateten Funde (`DBM_REREVIEW_FINDINGS=1`: B1, N2) |
| Mutationsproben `scripts/review-wave4-rereview-mutations.py` | 22 Mutanten: 20 getötet; Y5, N1 erst mit neuem Test getötet; N2 (Profiling) nur live getötet; C2 (`ErrTxDone`) überlebt (Testlücke, zu N2) |

Nicht ausgeführt: Browser-/Compose-Pfade, Replikations-/Lastfälle, echte Verbindungsverluste beim COMMIT, MySQL 1044 mit echtem Konto ohne Datenbankrecht (nur Klassifikation).

## Neue Dateien (Reviewer)
`internal/database/wave4_rereview_test.go`, `internal/httpapi/wave4_rereview_test.go` (Funde gegatet mit `DBM_REREVIEW_FINDINGS=1`, Speicher mit `DBM_REREVIEW_HEAP=1`),
`scripts/review-wave4-rereview-yaml.py`, `scripts/review-wave4-rereview-mutations.py`, `scripts/test-rereview-live.py`.

## Nächste Schritte
1. B1 beheben (Vorabprüfung ohne `arm`), HTTP-Regressionstest übernehmen (Gate entfernen). 2. N2 und N1 beheben oder Vertragstext korrigieren. 3. N3 Fixture. 4. Re-Review nur dieser Punkte genügt.
