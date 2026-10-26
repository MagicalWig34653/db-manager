# W-P29 Backend: Partitionen und Tabellenpräfix-Stapel

Stand: 26.10.2026. Fertiggestellt im Worktree `.worktrees/w-p29-backend`, Branch
`feat/w-p29-backend`; Entwurf eines abgebrochenen Codex-Laufs übernommen, geprüft,
auf API 0.22 umnummeriert, um Teilzustandsberichte je Tabelle ergänzt und mit `main`
(`3a7e1f1`) zusammengeführt. Nichts nach main gemergt; Lead integriert nach unabhängigem Review. Zentrale Status-/Planungsdateien und Vertragsindex bleiben
beim Lead. Kein Webpin, Assetbuild oder Deployment.

## Ergebnis und Abnahmeumfang

Neue [API 0.22](../../contracts/PARTITIONS-API.md) unter `/api/v22/`, additiv auf
v12; Nummer durch Auftrag endgültig festgelegt. V1 bis v12 unverändert gepinnt,
v13 bis v20 keine Voraussetzung. Registrierung genau ein neuer Featureeintrag,
Featurezählung 13. Keine neuen Abhängigkeiten oder Änderungen an SQLite-Sandbox,
Treibern oder bestehender Objektoperationslogik. Eigener Code AGPL-3.0-only.

Partitionen: Katalogliste mit Methode, Ausdruck, Grenzen und Zeilenschätzungen;
MySQL/MariaDB definieren/neu partitionieren (RANGE/LIST/HASH/KEY), ADD/DROP,
REORGANIZE zum Teilen/Zusammenführen, COALESCE, REMOVE PARTITIONING,
ANALYZE/CHECK/OPTIMIZE/REBUILD/REPAIR nach Enginefähigkeit und TRUNCATE.
PostgreSQL deklarativ CREATE PARTITION OF, ATTACH/DETACH, DROP/TRUNCATE/ANALYZE
an ausgewählten Kindern. SQLite meldet ehrlich keine Partitionsunterstützung.

Präfixstapel: wörtliche Präfixsuche, DROP/TRUNCATE, Ersetzen/Entfernen/Hinzufügen
und Kopie mit Struktur und Daten; Zielnamen aus Präfix und Katalognamen abgeleitet.
Bis 100 Tabellen, vollständige W-P07-Pläne vor der ersten Änderung, ein Gesamt-Hash.
Die vorhandenen `buildOp`-Funktionen liefern SQL, Kindobjekte, Abhängigkeiten,
Zielprüfung und Warnungen. Keine duplizierten Kopier-/Umbenennungs-SQL-Generatoren.

Gemeinsam: Vorschau mit exakten Anweisungen, Warnungen (insbesondere Datenverlust
bei DROP/TRUNCATE und Sperren bei REORGANIZE), `dbplan.Check`, getippter
Katalogname beziehungsweise gemeinsames Katalogpräfix bei zerstörenden Aktionen.
Vollständige Neuplanung auf gepinnter Verbindung vor der Ausführung. Ergebnisse
`completed`, `rejected`, `outcome_unknown`; kein Retry. PostgreSQL/SQLite in einer
Transaktion, MySQL/MariaDB nach Teilwirkung konservativ unbekannt.
MySQL-Wartungsfehler in Ergebniszeilen werden ebenfalls erkannt; keine rohen
Treiberfehlermeldungen, SQLtexte oder Pfade in Fehlerantworten.

## Dateien / Besitz

| Dateien | Inhalt |
| --- | --- |
| `internal/core/partitions.go` | Typen, Engineinterface, Fähigkeiten, Limits |
| `internal/database/partitions.go`, `prefix.go` | Katalogauflösung, strukturierte Partitions-SQL-Erzeugung, W-P07-Stapelplanung, Bestätigung und Ausführung |
| `internal/database/partitions_test.go`, `partitions_live_test.go` | SQLite-/Negativ-/Outcomefälle und vorbereitete Netzwerk-Livetests |
| `internal/httpapi/partitions.go`, `partitions_test.go` | authentifizierte Routen, HTTP-Validierung und echte Antwortdumps |
| `internal/httpapi/features.go`, `features_test.go` | eine Registrierungszeile, Zählung 12 → 13 |
| `contracts/api-v0.22.0.json`, `PARTITIONS-API.md`, `fixtures/partitions/*` | additiver Vertrag, Verhalten, zwölf gekennzeichnete Platzhalter und Herkunft |
| `scripts/check-partitions-contract.py`, `test-partitions.py` | Byte-/Additivitäts-/Schemawächter und sequenzieller dbfix-Livetest mit SQLite-only-Modus |
| `.github/workflows/partitions.yml` | zukünftige CI: Go-Race/Vet, Wächter, vollständige Fixture-Suite |
| `docs/decisions/ADR-W-P29-partitionen.md` | begrenzte strukturierte Definitionen und Wiederverwendung der Objektplanung |
| `docs/handoffs/W-P29-backend.md` | Prüfstand und Fortsetzung |

Vertrag SHA-256: `99e32721fd07b9610b6fd5aa6db7c9de7f8e93c8e9ce7d9b4163d0d489a5cb52`.
Die eingecheckten JSON-Fixtures sind ausdrücklich Platzhalter. Echte lokale
Antwortdumps entstehen nur im temporären Prüfverzeichnis und wurden nicht als
Netzwerkbelege übernommen.

## Teilzustand im Präfixstapel (neu gegenüber dem Entwurf)

Präfixausführung liefert `PrefixStructureResult` (StructureResult plus `tables`):
je Tabelle `status` `applied`/`not_applied`/`unknown` mit Anweisungszählern.
PostgreSQL/SQLite: eine Transaktion, Fehler gleich alle `not_applied`, unsicherer
Commit gleich alle `unknown`. MySQL/MariaDB: abgeschlossene Tabellen `applied`,
unterbrochene Tabelle `unknown`, Rest `not_applied`, `outcome_unknown`, Meldung nennt
Teilzustand. Kein Retry, kein automatisches Rückgängigmachen. Tests:
`TestPrefixTableResultsPartialStates`, `prefixPartialCase` (SQLite und alle drei
Netzwerk-Server; zweite Tabelle wird von außerhalb des Stapels per Fremdschlüssel
referenziert).

## Prüfungen am 26.10.2026

| Befehl | Ergebnis |
| --- | --- |
| `gofmt -l .`, `go vet ./...` | bestanden, keine Ausgabe |
| `go test -race -count=1 ./...` | alle Pakete grün |
| `python3 -B scripts/test-partitions.py --keep <Verzeichnis>` | bestanden: SQLite, dann MySQL 8.4, MariaDB 12.3, PostgreSQL 18.6.x in je eigenem Wegwerfcontainer nacheinander (`dbfix.py`, danach leer); 19 echte Antworten gegen v22 validiert |
| alle 16 `scripts/check-*-contract.py` | bestanden (v22: 23 Fixtures) |
| `python3 -B scripts/check-docs.py` | 1159 Dateien, 0 Fehler |

Fixtures `*.real-mysql|mariadb|postgres|sqlite.json` sind unveränderte Livetest-Dumps;
übrige Dateien bleiben gekennzeichnete Platzhalter. SQLite: Partitionen ehrlich 422
`unsupported_action`; Präfixstapel funktionieren.

## Nicht ausgeführt / Grenzen

- Unabhängiges Review, CI-Lauf von `partitions.yml`, Compose/Swarm, Web-/Browserintegration,
  reale Netzwerkabbrüche und Sperrkonflikte nicht ausgeführt.
- Definitionen auf eine Katalogspalte und skalare Werte begrenzt; keine freien
  Ausdrücke, Subpartitionen, Defaultpartitionen, automatische PostgreSQL-Migration.
  REPAIR für eine tatsächlich unterstützende MariaDB-Engine nicht live belegt
  (InnoDB wird abgewiesen).
- `observation=null`: kein nachgelesener Nachzustand. Plan-Hash ist kein Replay-/ABA-Schutz.
- Zentrale Doku-/Vertragsindexpflege (STATUS, ARBEITSPLAN, PWA-PLAN-Nummernliste) bleibt beim Lead.

## Fortsetzung

Lead: unabhängiges Review vergeben, Befunde beheben lassen, erst danach integrieren.
Keine eigenen Jobs oder Container aktiv.
