# B2b-Nacharbeit – SQLite-Konfliktstrategie

Stand: 20.10.2026. Eigenes begrenztes Paket, Basis `31dd06f`, Branch
`fix/sqlite-update-conflict`, Worktree `.worktrees/sqlite-update-conflict`.
Unabhängig von B2c/0.4.0 integrierbar; keine Schema-/Index-/Capabilityänderung.

## Bestätigter Befund und Fix

SQLite-UNIQUE im Schema kann `ON CONFLICT REPLACE` festlegen. Strukturierter UPDATE
mit vollständigem Snapshot von Zeile1 und dem UNIQUE-Wert von Zeile2 meldete vorher
`committed`, obwohl dadurch Zeile2 gelöscht wurde. Eigene tatsächliche Race-Repro:
vorher2 Zeilen, erfolgreicher Update, anschließend1 Zeile. P0 bestätigte denselben
Fall unabhängig als **QA-B2b-01, hoch**. RowsAffected1 ist hier kein Nachweis dafür,
dass keine andere Zeile implizit ersetzt wird.

SQLite-Update wird jetzt ausdrücklich als `UPDATE OR ABORT` generiert. Das übersteuert
die Tabellenkonfliktstrategie, löst bei UNIQUE-Konflikt einen Fehler aus und lässt
beide Originalzeilen bestehen. Andere Treiber und Vertragsdateien bleiben unverändert.
Keine globale Änderung des freien SQL-Endpunkts: dessen SQL wird weiterhin bewusst
vom Benutzer formuliert. Keine Zusage über allgemeine Trigger-/Cascade-Nebenwirkungen.

Primärquelle: [SQLite ON CONFLICT](https://www.sqlite.org/lang_conflict.html)
beschreibt REPLACE auch für UPDATE, dessen nicht im Änderungscounter enthaltene
implizite Löschungen und den Vorrang der ausdrücklich gewählten Statementstrategie.

## Prüfungen und Übergabe

Vor Fix scheitert `go test -race ./internal/database -run
TestSQLiteUpdateOverridesTableReplacePolicy -count=1` tatsächlich mit
`UNIQUE REPLACE returned success; remaining rows=1 readErr=<nil>`.
Nach Fix: `go test -race -count=1 ./...` und `go vet ./...` erfolgreich;
Regression prüft Fehler sowie beide unveränderten Originalzeilen. Dokumentations-
und Diffprüfung vor Commit. Keine eigenen Container, Netzwerk- oder Deploymentjobs
für den rein lokalen SQLite-Fix gestartet. Kein Lizenz-/Abhängigkeitswechsel.

Nächster Schritt: separaten Fixcommit durch P0 eng nachprüfen und vom Lead nach main
zurückportieren, unabhängig vom noch nicht freigegebenen Insert-Paket. Native CI
nach Push ist ein zusätzliches Gate; zum Commit keine Selbstfreigabe behauptet.


Produktcommit `20492c20eb9a2d228072eaf521cfa502ae4a0ebd` ist gepusht.
Zum Übergabezeitpunkt laufen native CI-Jobs noch: Mutation39501377601,
Engine39501377584, Table39501377685, Backend39501377686, Web39501377729,
Docs39501377673 (jeweils GitHubActions-Run-ID im Hauptrepository).
Dieser reine lokale Handoffcheckpoint korrigiert das Datum und sichert Jobprovenienz;
kein erneuter Push unveränderten Produktcodes. Worktree danach sauber, keine lokalen Jobs.
