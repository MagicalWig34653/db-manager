# Handoff Integration Welle 2 (Backend)

Branch `integration/wave2-backend` (Basis main `b74d10e`), nicht gemergt. Vier von Codex ohne Docker und Netzwerk gebaute
Backendpakete wurden nacheinander integriert, mit echten Engine-Fixtures und Livetests nachgeprüft. Endstand siehe `git log`.

## Reihenfolge, Nummern, SHAs

| Schritt | Paket (Quelle) | endgültige API | Merge-Commit | Vertrag SHA-256 |
| --- | --- | --- | --- | --- |
| 0 | W-P07 Datenbank-/Tabellenoperationen (main) | 0.8 | main | `74aa39ed06311f2c45afc274a2412a0ac30912e3a5683ec96b812fd473a28264` |
| 1 | W-P08 Spalten (`4b66dfd`) | 0.9 (unverändert) | `7ccdcc7` | `508b9951d102ec44bf4cafc9fe444f7266b31f898d9e91c887f4bef39735e3f8` |
| 2 | W-P10 Tabellenwartung (`276171b`) | 0.11 -> **0.10** | `a56c9f5` | `077a0e0222963b112bd6ea528ef5668d2ff7fbe2b2d3555669487c4beaf55cc5` |
| 3 | W-P11 Export/SQL-Dump (`9c1310c`) | 0.12 -> **0.11** | `0ceeb59` | `b3bcc38b44b02033…` (`shasum -a 256 contracts/api-v0.11.0.json`) |
| 4 | W-P15 SQL-Stapel/EXPLAIN/Simulation (`e5425c1`) | 0.16 -> **0.12** | `c562a61` | `7f0446edccdc368a202e4eaae5689d4b44d641e34bdf77ec8314de9f3e3f2dcd` |

Nummern sind lückenlos und unabhängig von den Vorschlägen in PWA-PLAN.md (dort ergänzt). W-P09 (Indizes, Schlüssel) erhält die
nächste freie Nummer (0.13) bei seiner Integration. Neu gebaut wurde jeweils die Datei `api-v0.N.0.json` als Vertrag über dem
Vorgänger (Skript: Schemas/Pfade des Pakets gegenüber seiner Basis v7 ermittelt, Pfade `/api/vOLD` -> `/api/vN`, Konstante
`0.OLD.0` -> `0.N.0`), weil die Codex-Verträge nur auf v7 aufsetzten; der Vorgänger bleibt byteidentisch enthalten. Das
W-P11-Formatschema `export-format-v1.schema.json` (`f21a9e7d…`) ist unverändert. Alle Wächter prüfen jetzt die Bytes der Vorgänger
(v1 bis v(N-1)) und Additivität gegenüber dem direkten Vorgänger; nichts ist im Web-Lock gepinnt, `contracts/README.md` ist
unverändert (Lead).

Konflikte: nur `internal/httpapi/features.go` (eine Zeile je Paket, Reihenfolge Objektops, Spalten, Wartung, Export, SQL-Stapel) und
`features_test.go` (Zählung jetzt 12; Negativroute `/api/v13/capabilities`). Keine Konflikte in contracts/README oder Workflows.

## Livetests und Prüfstand (Host-Last 30 bis 300, Container strikt nacheinander, Präfix `dbmanager-<paket>-`, danach keine übrig)

| Paket | Befehl | Ergebnis |
| --- | --- | --- |
| W-P08 | `python3 scripts/test-columns.py --keep <dir>` | SQLite, MySQL 8.4, MariaDB 12.3, PostgreSQL 18.6 PASS, 21 echte Antworten gegen v9 validiert; 13 echte Fixtures ergänzt (`real-mysql/mariadb/postgres`) |
| W-P10 | `python3 scripts/test-maintenance.py <dir>` | alle vier Systeme PASS beim ersten Lauf (zusätzlich Parameter für Dumpverzeichnis), 51 echte Fixtures `real-*` |
| W-P11 | `python3 scripts/test-export.py --live --keep <dir>` | alle vier Systeme PASS: Restore des SQL-Dumps in leere Datenbank mit Zeilen-/Spalten-/Indexvergleich, Snapshot gegen fremden Schreiber, fehlende Rechte, MyISAM abgewiesen; echte JSON/SQL/CSV-Dateien als Fixtures |
| W-P15 | `python3 scripts/test-sqlbatch.py --keep <dir>` | alle vier Systeme PASS, 10 echte Antworten (SQL, EXPLAIN, SQLite-Simulation); 10 echte Fixtures |
| alle | `gofmt -l .`, `go vet ./...` | ohne Ausgabe |
| alle | `go test -race -count=1 ./...` | alle Pakete ok, einschließlich der von Codex ausgelassenen Listener-/Crash-Recovery-Tests (`internal/database`, `serverstate`, `sqlitesandbox`, `sqlsplit`) |
| alle | `check-{table,mutation,insert,structure,create,overview,object-ops,columns,maintenance,export,sqlbatch}-contract.py` | alle bestanden |
| W-P07 | `python3 scripts/test-object-ops.py` (Regression nach den Merges) | bestanden |
| Doku | `python3 scripts/check-docs.py` | 0 Fehler (nach Anlage dieses Handoffs) |

## Korrekturen (jede dokumentiert)

1. W-P08 `columns_test.go`: MariaDB meldet `COLUMN_DEFAULT='NULL'` (Text) für ein nullbares Feld ohne Default; die Testabfrage erwartete
   SQL-NULL und scheiterte nur auf MariaDB. Abfrage behandelt beides als "kein Default". Produktcode unverändert.
2. W-P08 Wächter: pinnt jetzt die Bytes von v8 (zuvor nur bedingt).
3. Wächter von W-P10, W-P11, W-P15 auf den jeweiligen Vorgängervertrag und die Nummern 0.10/0.11/0.12 umgestellt; Dateinamen, Pfade, Konstanten,
   Texte, Workflows und Handoffs der Pakete umbenannt ("vorläufig" entfernt).
4. Testskripte `test-maintenance.py` (Dumpverzeichnis als Argument), `test-export.py` (jetzt ein Container nach dem anderen statt alle drei
   gleichzeitig, `--keep`, Livedumps JSON/SQL/CSV und deren Schemaprüfung), `test-sqlbatch.py` (ein Container nach dem anderen, `--keep`)
   angepasst; der Livetest des Exports schreibt die echten Dumps in `DBM_EXPORT_DUMP`.
5. Echte Engine-Fixtures ergänzt (Platzhalter bleiben als Schema-Beispiele, READMEs der Fixtureordner nennen den Unterschied).
6. Produktionscode der Pakete brauchte keine Korrektur: alle Livetests liefen nach der Testkorrektur 1 grün.

## Sicherheitsblick (destruktive Pfade)

- W-P08: Zielname für `dbplan.Check` ist `d.Table.Name` aus `describe` (Katalog); Bezeichner über `dbplan.QuoteIdentifier`; Typen aus fester
  Typliste; Livetests prüfen falschen Namen/Hash/Aktion, FK-Blockade, SELECT-Rolle ohne ALTER-Recht, Nachbartabelle unverändert.
- W-P10: Ziel aus `describe`, `dbplan.Check` mit Katalogname und `TypedConfirmation` für zerstörende Aktionen (Reparatur/VACUUM FULL),
  neue SQLite-Verbindungsart `OpenMaintenance` erlaubt nur `integrity_check` mit Argument, normale Verbindungen behalten die Positivliste.
- W-P11: nur lesend; MySQL/PostgreSQL im READ-ONLY-Snapshot, SQLite mit `query_only`. Hinweis: In SQLite erlaubt die Sandbox das Zurücksetzen von
  `query_only`, wie schon die bestehende Query-Route; kein zusätzlicher Rechtegewinn, aber für den Abfrageexport kein harter Schreibschutz.
- W-P15: Ausführung ist vom Nutzer bestätigtes freies SQL (wie `/api/v1/query`, ohne Plan/Hash, ausdrücklich `confirm`); EXPLAIN nur mit
  SELECT/WITH/VALUES/INSERT/UPDATE/DELETE ohne ANALYZE; Transaktions-/Sitzungsanweisungen im Stapel abgewiesen; Simulation nur SQLite.
  Keine rohen Treiberfehler in den geprüften Antworten.
- Keine Auffälligkeiten, die eine Codeänderung erzwangen; Review pro Paket bleibt Leadgate.

## Offene Punkte

- Unabhängige Reviews der Pakete W-P08, W-P10, W-P11, W-P15 stehen aus (nur Livetests und Eigenprüfung hier).
- `internal/sqlsplit` kommt aus W-P15, obwohl der Plan es W-P12 zuordnet (W-P12 muss darauf aufbauen oder abgleichen).
- ADR-W-P11 und ADR-W-P15 tragen weiter die Nummern der Pakete; Annahme durch den Lead offen.
- `contracts/README.md` (Vertragsindex), STATUS/ARBEITSPLAN/PARITAET nicht aktualisiert (Lead).
- Weiterhin nicht getestet: echte Sperrkonflikte/Verbindungsabbrüche gegen die Netzwerkengines (nur Fault-Injection), W-P15-Simulation für
  MySQL/MariaDB/PostgreSQL (bewusst nicht unterstützt).
