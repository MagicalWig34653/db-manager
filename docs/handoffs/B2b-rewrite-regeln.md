# B2b-Nacharbeit – PostgreSQL-Rewrite-Regeln

Stand: 20.10.2026. Separater minimaler Backport, Basis `31dd06f`, Branch
`fix/postgres-mutation-rules`, Worktree `.worktrees/postgres-mutation-rules`.
Kein neuer APIumfang, keine Änderung eingefrorener Vertragsbytes oder Capabilities.

## Tatsächlicher Befund und Korrektur

Jeweils echte PostgreSQL18.6-TLS-Probe über den integrierten Produktpfad:
Basistabelle `rule_source_*` mit `(1,'source')`, fremde Tabelle `rule_target_*` mit
`(1,'target')`; vollständiger typisierter Snapshot der Quelle.

- INSTEAD-UPDATE-Regel schreibt den angeforderten Wert in die Zieltabelle. Produkt
  meldet vorher `committed`, `changed:false`, keinen Fehler, obwohl Quelle unverändert
  bleibt und fremde Zielzeile nun `redirected` enthält.
- INSTEAD-DELETE-Regel löscht die fremde Zielzeile. Produkt meldet vorher
  `committed`, `changed:true`, keinen Fehler; Quelle bleibt bestehen, Ziel ist leer.

Beide eigenen Race-Repros scheiterten vor Fix genau mit diesem Zustand. Der fixierte
v3-Eignungscheck sperrt PostgreSQL-Basistabellen mit `pg_class.relhasrules`
konservativ als `eligible:false`, `reason:unsupported_table`. Mutation prüft dies
selbst, zusätzlich erneut nach dem Rowlock unter gehaltenem Relationslock, damit
keine Regel zwischen erster Metadatenprüfung und Sperraufnahme unbemerkt hinzukommt.
Keine Unterscheidung einzelner Regelprogramme und keine breite Triggeranalyse.

Primärquellen: [PostgreSQL DML-Regeln](https://www.postgresql.org/docs/18/rules-update.html),
[CREATE RULE](https://www.postgresql.org/docs/18/sql-createrule.html) und
[pg_class](https://www.postgresql.org/docs/18/catalog-pg-class.html).
Sie beschreiben die mögliche Ersetzung/Umleitung ursprünglicher DML-Anweisungen;
die obigen tatsächlichen Produktproben belegen den konkreten Fehler.

## Prüfungen und Ressourcen

Nach Fix `python3 scripts/test-row-mutations.py` erfolgreich, inklusive neuer
`TestLiveMutationsPostgresRules/update` und `/delete` unter Raceprüfung. Das bestehende
Skript trifft den neuen Test über seinen Regex `TestLiveMutations` automatisch.
Regression verlangt abgewiesene Mutation, negative Eignung und unveränderte
Quell- und Fremdzeilen. Bestehende MySQL/MariaDB/PostgreSQL-Mutationsproben ebenfalls
bestanden. `go test -race -count=1 ./...`, `go vet ./...`, Vertrags-/Docs-/Diffprüfung
vor Commit. Keine Abhängigkeits-/Lizenzänderung und keine Docker-/HTTP-Vertragsänderung.

Nur eigene zufällig benannte `dbmanager-b2b-*`-Fixtures des bestehenden Skripts mit
Label `dbmanager.task=B2b`, dynamische Loopbackports und temporäre Testzertifikate.
Alle eigenen Container/Volumes entfernt, fremde Ressourcen unverändert; keine lokalen
Jobs mehr aktiv. Nativer Feature-CI-Ausgang nach Push noch offen. Nächster Schritt:
unabhängige P0-Prüfung und Lead-Backport; bis dahin keine v3-Schreibfreigabe.
B2c bleibt eigenständiger noch nicht freigegebener Kandidat und übernimmt diese Grenze.
