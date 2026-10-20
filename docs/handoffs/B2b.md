# B2b – Strukturierte Einzelzeilenänderungen

Stand: 20.10.2026. Backend & Platform, Worktree `.worktrees/table-edit`, Branch
`feat/table-edit`, Basis `6009e1d`. Reviewfähiger Implementierungskandidat; noch kein Review,
Produktintegration oder 0.3.0-Freeze. Kein Produktclient darf den Kandidaten binden.

## Auftrag, Architektur und Besitz

Neue getrennte v3-Fähigkeiten/Eignungsprüfung plus UPDATE/DELETE einzelner bestehender
Zeilen. [Vertragsbeschreibung](../../contracts/ROW-MUTATION-API.md) enthält präzise
Transaktions-, Snapshot-, Fehler- und Betriebsgrenzen. Lead bestätigte den begrenzten
Ansatz; Architecturepeer bestätigte FOR-UPDATE-/BEGIN-IMMEDIATE-/Commitfehlerregeln.
Kein Insert, PK-Update, Schema, Batch, Client oder dauerhafter Tresor.

Besitz: neue Core-/HTTP-/Datenbank-Mutationsdateien, enge HTTP-Routenregistrierung,
SQLite-Lese-/Bindungs-/Transaktionshelper, Metadatenquery-Interface, neue Tests,
eigene Vertrags-/Live-Prüfskripte/CI und diese Übergabe. Kein main, Docker/Compose,
GoMod, Webwrapper oder S1. Alte API0.1.0/0.2.0-Dateien, Routen und Capabilityantworten
bleiben unverändert; eigene Vertragsprüfung schützt ihre Bytes und Definitionen.


## Kandidat und Verhalten

[Schema0.3.0](../../contracts/api-v0.3.0.json), SHA256
`ec77c9739cda11df3f4cb9fe756a632e67840fdc0f6190deee1aa1d1333dc308`.
Separate authentifizierte `/api/v3/capabilities`, `/tables/eligibility`,
`/rows/update`, `/rows/delete` (sämtlich mit `/api/v3`-Präfix).
Vollständiger geordneter Snapshot, präzise Pflichtfeldpräsenz und `confirm:true`
sind erforderlich. Alle Identifier frisch aus Metadaten, alle Werte gebunden.
SQLite: gepinnte Conn, Fremdschlüsselprüfung per privater nativer DBConfig aktiv,
BEGIN IMMEDIATE und native StorageClass-Reads. NW: Read-Committed-Tx, FOR UPDATE,
Metadaten nach Rowlock erneut geprüft. PG adressiert zusätzlich `ONLY` die Basisrelation.
Exakter Vergleich vollständiger kanonischer Cells statt SQL-Gleichheit alter Werte.
Nicht passende/fehlende Zeile HTTP409; nach Commitversuchfehler HTTP503
`outcome_unknown`, keine Wiederholung. Direkt betroffene Zeilenanzahl maximal1,
No-op ohne UPDATE-Anweisung. Nachherzeile vor Commit gelesen und JSON-Größe geprüft.

## Tatsächliche Prüfungen

Lokal am 20.10.2026 ausgeführt und bestanden:

- `go test -race -count=1 ./...`, `go vet ./...`.
- `python3 scripts/test-row-mutations.py`: echte TLS-Wegwerfziele MySQL8.4,
  MariaDB12.3, PostgreSQL18.6, jeweilige Tests ebenfalls mit `-race`.
- `python3 scripts/check-table-contract.py` und `python3 scripts/check-mutation-contract.py`:
  eingefrorene 0.1-/0.2-Bytes sowie deren Definitions-/Routengleichheit im neuen Schema.
- `python3 scripts/check-docs.py`, `git diff --check`.

Regressionen: Sondernamen und CompositePK, NULL/Leertext/Leerblob, unveränderte große
Decimal-/Binary-/Default-/Generated-Spalten, PK-/Generated-Zielverbot, Bestätigung,
vollständige Snapshotform, Konflikt nach Änderung/fehlender Zeile, konkurrierende
Schreiber mit gleichem Snapshot (genau ein Erfolg), No-op, Delete, View/NoPK/MyISAM.
SQLite zusätzlich nullablePK, BINARY-PK bei NOCASE-Spalte (nur richtige Identität
geändert), TEXT→BLOB-Speicherklassenkonflikt, ungültigesUTF8-TEXT gesperrt,
Constraint-Rollback, nonblocking Busy und anschließende Connectionfreigabe.
Netzwerk: echter gehaltener Rowlock mit Contextdeadline und verzögernder UPDATE-Trigger
mit Deadline; danach ursprünglicher Wert und neue Verbindung geprüft.
Echte deferred-FK-Commitfehler auf SQLite und PostgreSQL ergeben `outcome_unknown`
statt Erfolg, Daten bleiben im konkreten Test unverändert. PG-Vererbungsparent/-child und auch ein leerer Partitionparent
werden konservativ gesperrt. HTTP prüft Auth, Bestätigung, fehlend/NULL gegen echte
Leerwerte/-identifier, Snapshotpflichtfelder, HTTP409/503 und Requestgrenze413.

Der erste MySQL-Trigger-Setupversuch scheiterte, weil CREATE TRIGGER dort nicht als
Prepared Statement unterstützt wird. Fixtureadministration erzeugt nur diesen Trigger
nun direkt über die ausschließlich taskeigene DB-Verbindung; Produktmutation bleibt
gebunden und unverändert. Alle anschließenden vollständigen TLS-Proben bestanden.
Keine zusätzlichen Abhängigkeiten, Lizenz- oder Moduländerungen.

## Einschränkungen und offene Nachweise

Keine Clientbindung/Integration/Selbstfreigabe. Native Linuxamd64-CI nach Commit und
unabhängige QA sowie separater 0.3.0-Freeze sind Gates. Vollständiges Deployment wurde
für diese begrenzte DB-/HTTP-Erweiterung nicht erneut ausgeführt; main/Docker unverändert.
Keine tatsächliche Verbindungsunterbrechung exakt während COMMIT auf MySQL/MariaDB
reproduziert; reale SQLite-/PG-Commitfehler belegen den gemeinsamen Fehlerzweig.
Kein allgemeines Stromausfall-/Exactly-once-/Trigger-/Cascade-Rollbackversprechen.

Snapshotvergleich betrifft ausdrücklich nur das gemeinsame kanonische Cellmodell;
keine vollständige physische DB-Zustands-/Schema-/Historienkonflikterkennung. ABA und
nicht darstellbare frühere Speicherklassen bleiben Grenzen. Aktuell ungültigesUTF8-TEXT
in SQLite wird gesperrt; Browse0.2 bleibt unverändert Binarydarstellung. NaN/Infinity
und nicht bindbare Spezialwerte verhindern Mutation. Körperlimit1MiB schließt manche
8MiB-Browsezeilen aus. Keine PK-/Identity-/Generated-Änderung oder Vererbung/Partitionen.
Eligibility beweist keine Rechte; Trigger/Funktionen/Cascades können mehr als die
direkt adressierte Zeile oder externe Ziele beeinflussen. B1/ADR0004-TLS-, Root-,
SingleReplica-/Volume-/DELETE-Journal- und global4Anfragen-Grenzen gelten unverändert.

## Ressourcen, Jobs und nächster Schritt

Eigenes Skript nutzt nur `dbmanager-b2b-<Zufall>`, Label `dbmanager.task=B2b`,
dynamische127.0.0.1-Ports, temporäre synthetische Zertifikate und die öffentliche
Fixturepassphrase. Mit Frontendowner abgestimmt. Vorher Ressourcenstand gelesen,
alle eigenen Container/Volumes anschließend mit Skriptcleanup entfernt; fremde
A2-/F3-/Entwicklungsressourcen unverändert. Keine produktiven Daten oder Migrationen.
Alle lokalen Prüfjobs beendet, letzte vollständige TLS-Mutationssuite einschließlich
Partitionparent erfolgreich. Keine eigenen B2b-Container mehr vorhanden. Produktbranch
wird für native CI gepusht; genaue Commit-/Job-IDs folgen als Übergabecheckpoint.
Danach Worker idle für unabhängige QA. Bei Finding nur begrenzte
Nacharbeit; Lead allein integriert und entscheidet über Vertragsfreeze.

## Commit- und CI-Checkpoint

Produktcommit `6ad5833ff8d819d41a674bd583878868e486677b` ist auf
`origin/feat/table-edit` gepusht. Reiner lokaler Handoffcheckpoint folgt ohne erneuten
Push unveränderten Produktcodes. Worktree nach Checkpoint sauber; keine lokalen Jobs.
Diese nativen Linuxamd64-/Containerjobs laufen zum Übergabezeitpunkt noch; kein
Erfolg behauptet:

- [Strukturierte Einzelzeilenänderungen](https://github.com/MagicalWig34653/db-manager/actions/runs/39499012540)
- [Datenbankengine](https://github.com/MagicalWig34653/db-manager/actions/runs/39499012436)
- [Tabellenmetadaten und Lesen](https://github.com/MagicalWig34653/db-manager/actions/runs/39499012496)
- [Backend und Container](https://github.com/MagicalWig34653/db-manager/actions/runs/39499012514)
- [Echter Web-Datenbank-Pfad](https://github.com/MagicalWig34653/db-manager/actions/runs/39499012519)
- [Dokumentation](https://github.com/MagicalWig34653/db-manager/actions/runs/39499012400)

Lead/QA prüfen Jobausgänge und Kandidat unabhängig. Worker bleibt idle, kein weiteres
Paket vor erneuter Beauftragung; 0.3.0-Clientbindung bleibt gesperrt.

## Unabhängige Abnahme und Vertragsfreeze

P0-QA hat `6ad5833`/`a80b934` ohne bestätigte Befunde zur Produktintegration und
API0.3.0-Clientbindung freigegeben. Eigener sauberer Archivlauf mit vollständiger
Go-Race-Suite, Vet, beiden Vertragsprüfungen und Dokumentation238/0 sowie echter
TLS-Mutationslauf gegen MySQL8.4/MariaDB12.3/PostgreSQL18.6 erfolgreich.
Implementierung/Snapshotvergleich nach Rowlock, Identifier/Bindparameter,
Tabelleneignung, SQLite-Kollation/Fremdschlüssel, Deadlines/Rollback und ehrlicher
unbekannter Commitausgang geprüft. Zusatzangriff: AFTER-UPDATE-Trigger verschiebt
PK und schreibt Auditnebenziel; Operation scheitert und beide Änderungen werden
zurückgerollt. Identisches Update löst keinen UPDATE-Trigger aus.

Alle sechs Feature-CIs am exakten Produktcommit erfolgreich:
[Mutation](https://github.com/MagicalWig34653/db-manager/actions/runs/39499012540),
[Engine](https://github.com/MagicalWig34653/db-manager/actions/runs/39499012436),
[Tabellen](https://github.com/MagicalWig34653/db-manager/actions/runs/39499012496),
[Backend](https://github.com/MagicalWig34653/db-manager/actions/runs/39499012514),
[Web](https://github.com/MagicalWig34653/db-manager/actions/runs/39499012519),
[Dokumentation](https://github.com/MagicalWig34653/db-manager/actions/runs/39499012400).
Lead integrierte `946241d`/`e4465ac`; integrierte Go-Race/Vet-, Vertrags- und
Dokumentationsprüfung bestanden. Schema unverändert. Keine Sourceänderungen durch
QA, eigene Archive/Container entfernt, keine Jobs offen. Dokumentierte Grenzen
für ABA, externen Triggerumfang, Exactly-once und HA gelten weiter.
