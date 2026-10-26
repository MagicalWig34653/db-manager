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
