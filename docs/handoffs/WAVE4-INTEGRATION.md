# Handoff Integration Welle 4 (Backend)

Branch `integration/wave4-backend` (Basis main `3a7e1f1`, API 0.13-0.16), nicht gemergt. Sechs Pakete per `git merge --no-ff` in Nummernreihenfolge
integriert und mit echten Engine-Fixtures (MySQL 8.4, MariaDB 12.3, PostgreSQL 18.6, SQLite; Container nacheinander durch `dbfix`) geprüft.

## Reihenfolge, Verträge

| Paket | Branch / Quelle | API | Vertrag SHA-256 |
| --- | --- | --- | --- |
| W-P17 Serververwaltung (lesend) | `feat/w-p17-backend` | 0.17 (unverändert) | `ec2f75e2159f06fcc841c19089ba82fa9e58b998f55dfc27efcabed6771b39f3` |
| W-P12 Import | `feat/w-p12-backend` (`ea8ea0e`) | 0.18 (neu aufgebaut) | `0d44d0c68f9d4d697eab30e60d8237db4a50663f91b512d919135a415f42375e` |
| W-P13 Mehrzeilen/Dateien | `feat/w-p13-backend` (`2287e57`) | 0.19 (neu aufgebaut) | `98de2bbe9302506b20cbd44d50448bee841d51572fe0fd3d3c7cee76d24017ea` |
| W-P27 Monitor/Replikation | `feat/w-p27-backend` | 0.20 (neu aufgebaut) | `6aa1a09df215d5602bc343d72bf91fd9a944a026bac5fdee9155d9cf5ffe6e8c` |
| W-P28 Exportformate | `feat/w-p28-backend` | 0.21 (neu aufgebaut) | `52ccefab4b92fab7a2772668de74cefb437aa60f8ed60f3abb8bd9d5296fec11` |
| W-P29 Partitionen | `feat/w-p29-backend` | 0.22 (neu aufgebaut) | `d0a09f3458c699f87701503d30ae6f54014a2b4310e1ad62b3af72144a8f4651` |

Ausgangslage: W-P12/W-P13 basierten auf v12 (ohne v13-v16-Pfade), W-P27/28/29 additiv auf v16 (ohne die jeweils anderen Pakete); nur v17 passte.
Deshalb wurden v18-v22 aus den Branch-Verträgen neu erzeugt: Deltas (Pfade/Schemas, die gegenüber der jeweiligen Branch-Basis neu sind) auf den Vorgänger
aufgesetzt, `info` des Pakets behalten, Kollisionen wären Fehler gewesen (keine). Ergebnis: lückenlose Kette v1-v22, jede Version Superset der vorherigen
(Pfade 96 -> 100 -> 105 -> 111 -> 120 -> 122 -> 129). Keine SHAs gemergter Verträge (v1-v16) geändert.

## Wächter
Neues `scripts/contract_chain.py`: pinnt v1..v21 per SHA-256, `check(N)` prüft Pins v1..N-1, `info.version` und Superset-Beziehung v(k-1) -> v(k) bis N.
Die Wächter v17-v22 rufen `contract_chain.check(N)` auf und vergleichen gegen v(N-1) (statt v12/v16), entfernte optionale "Vorgänger fehlt"-Zweige.
Alle `check-*-contract.py` und `check-docs.py` (0 Fehler, 1340 Dateien) grün. `check-extended-export-files.py` braucht ein Verzeichnisargument (läuft in `test-extended-export.py --live`, dort bestätigt: 24 Dateien).

## Konflikte
- `internal/httpapi/features.go`/`features_test.go`: beide Seiten behalten (Reihenfolge der Merges), Zähler jetzt 22.
- `go.mod` (W-P12): `golang.org/x/text v0.42.0` direkte Abhängigkeit, Doppeleintrag entfernt; Lizenzinventur `docs/LIZENZEN.md` enthält x/text (BSD-3-Clause); `go mod verify` ok.
- Folgetests: `TestAccountsCapabilitiesVersion` und `TestAllKnownRoutesRegisteredAndAuthenticated` nutzten v18 als "unbekannte Version" -> jetzt v99; Routen v17-v22 in die Authentifizierungsliste aufgenommen.

## Prüfstand
gofmt, `go vet ./...`, `go test -race -count=1 ./...` (alle Pakete grün), alle `check-*-contract.py`, `check-docs.py`, `test-web-integration.py` (13 passed), Livetests je auf SQLite, MySQL 8.4, MariaDB 12.3, PostgreSQL 18.6:
`test-server-admin.py` (12/12/11 Antworten gegen 0.17), `test-import.py --live` (15 Antworten), `test-rows-bulk.py` (14), `test-monitor.py` (24), `test-extended-export.py --live` (unabhängige Parser, 24 Dateien), `test-partitions.py` (19). Alle `*.real-*`-Fixtures der neuen Pakete werden von den Wächtern gegen den kumulativen Vertrag validiert; keine Produktfehler gefunden, kein Produktcode geändert.

## Offen
Unabhängige Reviews der sechs Pakete; Vertragsindex/STATUS/PARITAET beim Lead; Wächter älterer Pakete (v1-v16) pinnen die neuen Versionen nicht (nur ihre Vorgänger); `.github/workflows` je Paket unverändert übernommen.

## Nachbesserung nach Review A (`review/wave4-a`, W-P12/W-P13) und Review B (`review/wave4-b`, W-P17/27/28/29)

Beide Branches gemergt (Reviewer-Tests übernommen). Verträge v18-v22: `ImportStatementResult.outcome` um `rolled_back` erweitert plus Beschreibungstexte (nur Integrationsbranch, nicht auf main); v18-v22 neu gepinnt in `scripts/contract_chain.py`:
v18 `17451221…b103e5`, v19 `c7a52e69…0a2d46`, v20 `83b49461…b63b0`, v21 `68db81f9…18881c`, v22 `397db161…44afd1` (v1-v17 unverändert; die SHAs der Tabelle oben sind damit überholt).

Review A:
- F1 Einmaliger Plan: `plan_ledger.go` (im Speicher, 1 h, 4096 Einträge). Plan-Aufruf schaltet den Hash frei, Ausführung verbraucht ihn vor dem ersten Schreiben (auch bei `outcome_unknown`); Wiederholung CSV-Import -> `plan_mismatch`, Bulk -> `plan_conflict` ("bereits ausgeführt"). Tests `TestReviewA_CSVImportReplayRefused`/`BulkCopyReplayRefused` auf SQLite, MySQL, MariaDB, PostgreSQL. Grenze: nach Neustart zurückgesetzt, mehrere Instanzen teilen nichts.
- F2 Entscheidung: SQL-Import bleibt planlos (Dumps sind nicht planbar, Vertrag W-P12). Neu: erste Warnung der Antwort "führt beliebiges SQL ohne Plan aus" mit Zählung der Klassen DDL/DROP/DELETE/UPDATE/INSERT/sonstige der vollständig vorab validierten Datei; im Vertragstext dokumentiert. UI muss die Warnung vor Ausführung zeigen.
- F3: Statements, die auf SQLite/PostgreSQL durch abschließendes ROLLBACK oder COMMIT eines abgebrochenen PG-Rahmens zurückgenommen wurden, melden `rolled_back` (neuer Enum-Wert, Vertrag angepasst).
- F4: Dateiersetzen: `bulkValidateLimit` mit Budget 1 MiB + Base64(512 KiB); Upload-Body-Limit entsprechend; 400->400 KiB funktioniert, Übergröße `file_too_large`.
- F5: Importdatei wird beim Hochladen vorallokiert gelesen, die entpackte Bytefolge wird als String geteilt (keine 64-MiB-Kopie); Speicher/Slots im Vertrag dokumentiert. Verbleibend: Plan hält Datei + dekodierte Kopie (zweifache Dateigröße je Slot).
- F6: Abbruch/Fehler vor COMMIT -> `rejected`, `inserted=0` auf allen Engines (`ErrTxDone` gilt als zurückgerollt).
- Testlücken: Plan mit Confirm/Hash abgelehnt, Neuplan schaltet frei, Datei-vor-Metadaten abgelehnt. Nicht machbar: Metadaten-Neulesen nach Sperre (gleichzeitiges DDL).

Review B:
- F1 YAML: `yamlSafeJSON` escaped DEL, C1 (inkl. U+0085), U+FFFE/FFFF als `\uXXXX`; PyYAML, CSafeLoader, Psych lesen alle Zellen verlustfrei (`review-wave4b-parsers.py` alle OK); Test `TestYAMLSafeJSONEscapesUnreadableCharacters`.
- F2 Monitor: MySQL 1044/1045 nicht mehr `unavailable`; Profiling-Fehler (fehlende Tabelle/Spalte, Syntax) immer echter Fehler auf allen Engines; `unavailable` nur für Rechte-/Quellenfehler. Vertragstext angepasst.
- F3 Server-Admin: exakte Zähler `max_connections`, `max_user_connections`, `Connections`, `Max_used_connections` sichtbar (Allowlist vor Namenssperre), Doku angepasst.
- Testlücken: `wave4_package_test.go` (W-P17 Scope/Limit vor Verbindung, Sensitivnamen, 64/65-Ziffern; W-P29 Mehrbyte-Präfix, `opName`, Typname bei destruktivem Plan; W-P28 XML-Zeichenbereich, excel-csv-Steuerzeichen).
