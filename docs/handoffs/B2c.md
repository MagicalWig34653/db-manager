# B2c – Sicheres einzelnes INSERT

Stand: 20.10.2026. Backend & Platform, Branch `feat/row-insert`, Worktree
`.worktrees/row-insert`, Basis `31dd06f`. Reviewfähiger Implementierungskandidat,
noch kein unabhängiger Review/Integration/0.4.0-Freeze oder Produktclient.

## Umfang und Vertrag

[Schema0.4.0](../../contracts/api-v0.4.0.json), SHA256
`4763ebf8632a9ef977cf7d3a6037299fc4dafde0a581fe10cb34d844f8356fbf`.
[Verhaltensdokumentation](../../contracts/ROW-INSERT-API.md) ist maßgeblich für konkrete
Identitäts-, Default-/NULL-, Konversions-, Trigger-, Limit- und Commitgrenzen.
Neue Routen `/api/v4/capabilities`, `/api/v4/tables/insert-eligibility`,
`/api/v4/rows/insert`. Alte API0.1–0.3-Dateien, Definitionen, Routen und Capabilities
bytes-/definitionsgleich; eigene Prüfung sichert alle drei eingefrorenen Hashes.

Nur eine direkte neue Zeile mit Bestätigung. Keine REPLACE/IGNORE/UPSERT-/Batchfunktion,
kein Client, kein Schema-/Modul-/Docker-/Vaultpaket. Besitz neue Core-/DB-/HTTP-Dateien,
enge Routingregistrierung und identische Extraktion des vorhandenen Transaktionsstarts,
eigene Regressionen/Liveprüfung/CI, eigener Vertrag/Verhaltensdoku/Handoff.
Keine zentralen STATUS-/Plan-/Vertragsindexdateien geändert.

Metadaten-/Wertprüfung, echte Tx, Netzwerk-MDL vor Insert erneut geprüft, eindeutiger
vollständiger expliziter oder eng belegter Auto-PK. MySQL/MariaDB-ID als UInt64-String
in derselben Session; SQLite signedRowid; PG Identity/Serial RETURNING ausschließlich
für interne PK-Ermittlung. Tatsächliche vollständige Nachherzeile wird immer erneut
gelesen und vor Commit auf PK/Metadaten/JSON-Größe geprüft. Nullable/fehlende PK,
Views/virtuelle/Foreign-/vererbte/partitionierte/Rewrite-Rule-Tabellen und unbekannte/
nichttransaktionale Engines konservativ gesperrt. Werte ausgelassen=Default, explizites
NULL verschieden; doppelte/unbekannte/generated/identity-Eingaben abgewiesen.

## Tatsächliche Prüfungen

Lokal ausgeführt:

- `go test -race -count=1 ./...`, `go vet ./...`.
- `python3 scripts/test-row-insert.py`: echte isolierte TLS-MySQL8.4-/MariaDB12.3-/
  PostgreSQL18.6-Proben, Go-Tests darin ebenfalls mit Raceprüfung.
- Alte und neue Vertragsprüfungen: `check-table-contract.py`,
  `check-mutation-contract.py`, `check-insert-contract.py` unter `scripts/`.
- Dokumentations-/Diffprüfung vor Commit; Native-Linuxamd64-CI folgt nach Push und
  bleibt bis zum tatsächlichen Ergebnis offen. Keine Selbstfreigabe.

Belegte Fälle: Sondernamen/CompositePK, Default gegenüber NULL, Leertext/Leerblob,
präzise große Decimalwerte und wirkliche serverseitige Skalarundung1.234567→1.23457,
Generatedwerte, explizite PK-Konversion mit Rückrollen, unbestätigte/fehlende Pflichtfelder,
Auth/409/503/413, DuplicatePK ohne Überschreiben, konkurrierender gleicher PK,
SQLite-Rowidalias sowie PG Identity/Serial und MySQL/MariaDB Auto-ID oberhalb Int64
(`9223372036854775811`) ohne Überlauf. Fremde Auto-ID aus Audittrigger überschreibt
nicht den ursprünglichen Sessionnachweis. Nachherzeile enthält tatsächliche
Triggeränderung. SQLite/PG-AFTER-PK-Verlagerung und MySQL-BEFORE-Identitätsänderung
rollen Insert plus lokale Auditzeile zurück. SQLite RAISE(IGNORE) und PG-BEFORE-RETURN-NULL
werden nicht als Erfolg gemeldet. Echte deferred-FK-Commitfehler auf SQLite/PG melden
`outcome_unknown`. Netzwerk-Lockdeadline mit Connectionfreigabe, SQLite Zelllimit,
Netzwerk-JSON-Limit (5.4MB Rohtext, durch Escaping über8MiB JSON) vor Commit mit
nachgewiesen leerer Zieltabelle. Explizite SQLite-Spalten-/PK-Kollationsabweichung
bleibt eindeutig. PG-INSTEAD-UPDATE-Regel wird vor Insert blockiert.

Frühe Fixturefehler wurden korrigiert: MySQL-TEXT-Default benötigt passende Syntax,
daher portables VARCHAR-Default; PG GENERATED-ALWAYS-Identity darf im Trigger nicht
beliebig umnummeriert werden, deshalb echte Verlagerungsprobe auf zusätzlichem
Serial-PK. Identity selbst bleibt separat positiv geprüft. Keine fehlgeschlagene
Fixtureprobe wird als Produktnachweis ausgegeben. Keine neuen Abhängigkeiten/Lizenzen.

## Getrennte B2b-Backports aus demselben Abgleich

SQLite-Schemadefault `ON CONFLICT REPLACE` kann auch bei gewöhnlichem INSERT/UPDATE
andere Zeilen entfernen. B2c generiert deshalb ausdrücklich `INSERT OR ABORT`.
Regression schützt PK und sekundäres UNIQUE bei REPLACE-Schema. B2b-UPDATE wurde
separat als echter2→1-Zeilenverlust reproduziert und in eigenem Paket
`fix/sqlite-update-conflict` als `20492c2` korrigiert; dort eigener QA- und CI-Nachweis.
Der identische UPDATE-OR-ABORT-Fix bleibt in dieser Helperdatei erhalten.

PG-Rewrite-Regel-Umleitung wurde auf Leadauftrag auch für B2b real bestätigt
(UPDATE/DELETE ändern fremde Zeilen). Eigener minimaler Backport `5ef6a57` auf
`fix/postgres-mutation-rules` sperrt solche Tabellen vor Mutation und erneut nach
Rowlock, mit eigenen tatsächlichen Race-/TLS-Repros und Regressionen. Dieser
identische Fix bleibt in der Helperdatei des B2c-Kandidaten erhalten. Keine unbelegte breite Triggeranalyse oder physische
Einzeilenwirkung behaupten.

## Grenzen, Ressourcen und Fortsetzung

PK-Konversion ist konservativer Rückrollgrund; Nicht-PK-Konversionen/Rundung liefert die
DB und wird unverfälscht zurückgegeben. Kein Versprechen exakter angeforderter Speicherung,
auch nicht bei Decimal/SQL-Modi. Constraint-/Trigger-/Cascade-/DB-Programme bleiben
serverseitige Semantik; keine externe oder umfassende Rollbackgarantie. Sequence- und
Auto-ID-Lücken trotz Rollback möglich. Verbindungsverlust exakt während MySQL-/MariaDB-
COMMIT wurde nicht reproduziert; gemeinsame Fehlerbehandlung ist konservativ, reale
Commitfehler auf SQLite/PG belegt. Ergebnis zu groß verhindert Commit. Keine breite
Deploymentwiederholung bei unveränderten main-/Dockerdateien; B1/ADR0004 und vier
aktive Anfragen bleiben Betriebsgrenzen. Keine Clientbindung vor separater Freigabe.

Nur eigene `dbmanager-b2c-<Zufall>`-Container, Label `dbmanager.task=B2c`, dynamische
Loopbackports und temporäre öffentliche Testzertifikate; vor Start Ressourcen gelesen,
alle eigenen Container/Volumes nach jedem Lauf entfernt. Fremde A2-/F3-/Entwicklungs-
ressourcen unangetastet. Vor Übergabe alle lokalen Jobs abschließen, Commits und
CI-IDs sichern; anschließend idle für unabhängige QA. Lead allein integriert/friert den Vertrag.
