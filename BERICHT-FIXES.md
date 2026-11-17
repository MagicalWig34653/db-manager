# Bericht zu den Review-Reparaturen

Stand: 17.11.2026. Branch: `codex/review-fixes`, Ausgangscommit `f0c2e19`.
Alle vier Befunde wurden zuerst am tatsächlichen Code nachgeprüft und bestätigt;
kein Befund wurde verworfen. Auch der niedrige Befund wurde wegen des kleinen
Fixumfangs behoben. Pro Befund wurde zuerst der Regressionstest geschrieben,
danach der Produktcode geändert und beides in einem eigenen deutschen Commit gesichert.

**Keine Builds, Tests, Docker-Befehle oder Netzaufrufe ausgeführt.** Insbesondere
wurden weder ein roter Vorherlauf noch ein grüner Nachherlauf beobachtet.
Die Fehlerwirkung vor dem Fix ist aus den Codepfaden und Assertions hergeleitet.
Go-Dateien wurden mit `gofmt` formatiert; Git-Hooks wurden beim Commit deaktiviert,
um keine indirekten Prüfungen zu starten. Nicht gepusht.

## 1. Hoch: Wirkungsloser PostgreSQL-Rechteentzug

**Bestätigt und behoben. Commit:** `4a36084bc96013583c222ca60b3f0076c86cfef7`
— PostgreSQL-Rechteentzug anhand direkter ACLs prüfen.

Vorher prüfte die Ausführung nur GRANT nach. PostgreSQLs warnungsbasiertes REVOKE
konnte daher ohne Rechteänderung als `completed` enden. Jetzt werden die direkten
ACL-Einträge innerhalb derselben Transaktion nachgelesen. Verbliebene angefragte
Rechte führen zu `rejected/revoke_without_effect`, Lesefehler zu
`rejected/revoke_verification_failed`; die Transaktion wird zurückgerollt.
PUBLIC- und geerbte Rechte werden nicht als direkte Rechte fehlinterpretiert.
Bei `GRANT OPTION FOR` wird ausschließlich die Grant Option geprüft.
Numerische Routinen-OIDs werden für den Vergleich kanonisiert.

Regression: `TestLivePostgresRevokeWithoutAuthority` prüft eingeschränktes Konto,
wirkungslose REVOKE-Warnung, erhaltenes direktes Recht, wirksamen Entzug trotz
PUBLIC-Recht, isolierten Grant-Option-Entzug und Routinen-OIDs mit führenden Nullen.
Vorher scheitert bereits die erste erwartete Abweisung statisch an `completed`.

Benötigt: **ausschließlich reservierte Wegwerf-PostgreSQL-18-Datenbank** `probe`,
Admin `postgres`, TLS auf `localhost` mit passender CA und Passwortauthentifizierung.
Der Admin muss Rollen, Tabellen und Funktionen anlegen/löschen und Rechte vergeben
dürfen. Die Testrollen müssen sich mit Passwort anmelden können.
Der Aufrufer setzt Port, CA-Datei und Fixture-Passwort; der folgende Befehl verlangt
sie ausdrücklich, damit der Test nicht unbemerkt übersprungen wird:

```bash
DBM_TEST_POSTGRES_PORT="${DBM_TEST_POSTGRES_PORT:?Fixture-Port setzen}" \
DBM_TEST_CA_FILE="${DBM_TEST_CA_FILE:?Fixture-CA setzen}" \
DBM_TEST_PASSWORD="${DBM_TEST_PASSWORD:?Fixture-Passwort setzen}" \
go test ./internal/database -run '^TestLivePostgresRevokeWithoutAuthority$' -count=1 -v -timeout 2m
```

## 2. Mittel: Unbrauchbare Unicode-Passwörter

**Bestätigt und behoben. Commit:** `8f5c8a6cd5f533d998211b3ed7cdd6279bfaa875`
— PostgreSQL-Passwörter vollständig mit SASLprep vorbereiten.

Vorher fehlten Bidi-, Unassigned- und Teile der Verbotsprüfung; außerdem fand die
Normalisierung vor dem Mapping statt. Jetzt übernimmt `github.com/xdg/stringprep`
v1.0.3 die RFC-4013-Regeln, mit PostgreSQLs Vorrang der Leerzeichenabbildung.
Bei ungültiger Vorbereitung werden die unveränderten Originalbytes verwendet.
Für `"\u0627\u00AA"` wird deshalb kein Verifier mehr für `"\u0627a"` erzeugt.
Modul, Prüfsummen und Apache-2.0-Lizenz stammen aus dem bereits vorhandenen lokalen
Cache; Lizenzinventar und Originaltext wurden ergänzt.

Regression: `TestSASLPrepPostgresFallbackAndMapping` prüft den konkreten Bidi-Fall,
gültiges Bidi, Mapping vor Normalisierung, Leerzeichenpriorität, verbotene und in
Unicode 3.2 unbelegte Zeichen sowie die Weitergabe der erwarteten Bytes an SCRAM.
Im alten Code weicht unter anderem der konkrete Bidi-Fall von den Originalbytes ab.
Kein Dienst und keine Umgebungsvariable erforderlich:

```bash
go test ./internal/database -run '^TestSASLPrepPostgresFallbackAndMapping$' -count=1 -v
```

## 3. Mittel: Routinenwiederherstellung verliert Rechte

**Bestätigt und behoben. Commit:** `64f998ec3c973c0982515ae3c4d49787d37ea9fe`
— Routinenrechte bei automatischer Wiederherstellung sichern und prüfen.

Vorher wurden vor DROP nur Definition und Sitzungseinstellungen gesichert. Jetzt
werden zusätzlich alle lokalen Routinenrechte einschließlich Grant Option aus
`mysql.procs_priv` gesichert. Ist das nicht vollständig möglich, wird vor DROP
abgewiesen. Nach Wiederherstellung werden fehlende Rechte per GRANT ergänzt und
zusätzliche automatische Erstellerrechte per REVOKE entfernt; eine erneute
Katalogabfrage muss die ursprünglichen Rechte bestätigen. Erst dann wird
`rejected/restored` gemeldet. Andernfalls bleibt es bei `outcome_unknown/restore_failed`.
Globale und schemaweite Rechte bleiben unberührt. Gleichzeitige fremde DDL- oder
Rechteänderungen sind weiterhin nicht atomar abgesichert. Der Fix betrifft die
Wiederherstellung nach fehlgeschlagenem CREATE.

Regression: `TestLiveMySQLRoutineRestorePrivileges` erstellt als eigenes App-Konto
eine Prozedur, prüft die automatisch vergebenen EXECUTE-/ALTER-ROUTINE-Rechte und
ergänzt ein zweites Konto mit expliziter EXECUTE-Grant-Option. Ein Administrator
ersetzt die Routine mit fehlerhafter Syntax. Danach müssen die vollständigen
lokalen Rechte mit dem ursprünglichen Katalogzustand übereinstimmen. Der alte
Code meldet Wiederherstellung, obwohl dieser Vergleich fehlschlägt.

Benötigt: **ausschließlich reservierte Wegwerf-MySQL-8.4- und MariaDB-Fixtures**
(Repository-Fixture: MariaDB 12.3), jeweils Datenbank `probe`, Admin `root`,
`automatic_sp_privileges=1`, TLS auf `localhost` und passende CA.
Der Admin braucht Konten-/Routinenverwaltung, SELECT auf `mysql.procs_priv` und
GRANT-/REVOKE-Berechtigungen. Port, CA und Passwort werden vom Aufrufer gesetzt.
Beide Untertests einzeln ausführen:

```bash
DBM_TEST_MYSQL_PORT="${DBM_TEST_MYSQL_PORT:?MySQL-Fixture-Port setzen}" \
DBM_TEST_CA_FILE="${DBM_TEST_CA_FILE:?Fixture-CA setzen}" \
DBM_TEST_PASSWORD="${DBM_TEST_PASSWORD:?Fixture-Passwort setzen}" \
go test ./internal/database -run '^TestLiveMySQLRoutineRestorePrivileges$/^mysql$' -count=1 -v -timeout 2m

DBM_TEST_MARIA_PORT="${DBM_TEST_MARIA_PORT:?MariaDB-Fixture-Port setzen}" \
DBM_TEST_CA_FILE="${DBM_TEST_CA_FILE:?Fixture-CA setzen}" \
DBM_TEST_PASSWORD="${DBM_TEST_PASSWORD:?Fixture-Passwort setzen}" \
go test ./internal/database -run '^TestLiveMySQLRoutineRestorePrivileges$/^maria$' -count=1 -v -timeout 2m
```

## 4. Niedrig: SQLite-Importstatus nach automatischem Rollback

**Bestätigt und behoben. Commit:** `4f02038de940df7d7db856db16fbeae4bd5434e9`
— Automatischen SQLite-Rollback im Importstatus berücksichtigen.

Vorher wurden frühere Anweisungen nur bei erfolgreichem expliziten ROLLBACK
ummarkiert. Jetzt prüft der Fehlerpfad auf der gepinnten Verbindung den nativen
Autocommit-Zustand. Hat SQLite bereits automatisch zurückgerollt, werden frühere
Anweisungen dieser Transaktion als `rolled_back` ohne Änderungszahlen markiert.
Die Zuordnung endet sofort, sodass spätere Autocommit-Anweisungen im continue-Modus
korrekt bleiben. Der Gesamtausgang bleibt konservativ `outcome_unknown`.

Regression: `TestSQLiteImportSQLAutomaticRollback`, jeweils stop und continue,
mit früherem Autocommit, automatischem Konflikt-Rollback, späterem Autocommit und
unabhängiger Zeilenzählung. Vorher bleibt der zurückgerollte INSERT `completed`.
Kein Dienst und keine Umgebungsvariable erforderlich; lokale Wegwerfdateien werden
durch die Testhelfer angelegt:

```bash
go test ./internal/database -run '^TestSQLiteImportSQLAutomaticRollback$' -count=1 -v -timeout 2m
```

## Offene Abnahme

Alle genannten Befehle sind **noch auszuführen**. Sie setzen die Go-Toolchain aus
`go.mod` und verfügbare Modulabhängigkeiten voraus. Bestehende Live-Harness-Skripte
verwenden feste Testnamensfilter und starten die neuen Tests nicht automatisch;
deshalb sind die oben genannten direkten Aufrufe maßgeblich. Die Live-Tests
überspringen ohne Portvariable ihren jeweiligen Fixture-Fall.

Unabhängiges statisches Review durchgeführt; dabei gefundene OID-Vergleichslücke
mit zusätzlicher Regression korrigiert. Praktische Datenbank- und Testabnahme durch
den Aufrufer bleibt offen. Keine Merge-/Releasefreigabe und keine laufenden
Build-, Test-, Container- oder Netzjobs aus dieser Bearbeitung.
Dateien und Fortsetzung: [Paketübergabe](docs/handoffs/REVIEW-FIXES.md).
