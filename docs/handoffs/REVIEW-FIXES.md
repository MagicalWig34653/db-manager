# Reparaturpaket Review-Befunde

Stand: 17.11.2026. Branch: `codex/review-fixes`; exklusiver Dateibesitz für die betroffenen Konten-, Programm- und Importdateien samt Regressionstests sowie Paketdokumentation beim Lead. Keine parallelen schreibenden Worker.

Abnahme: alle hohen und mittleren Befunde am Code nachprüfen, jeweils zuerst Regressionstest schreiben, anschließend minimal beheben und separat auf Deutsch committen. Niedrigen SQLite-Befund wegen kleinen Fixumfangs einschließen. Keine Builds, Tests, Docker- oder Netzaufrufe; Prüfung durch den Aufrufer. Keine Veröffentlichung.

## Befund 1: PostgreSQL-Rechteentzug (hoch)

Bestätigt: Nur GRANT besaß eine Nachprüfung; PostgreSQL kann REVOKE ohne Wirkung mit bloßer Warnung quittieren. Neue Regression `TestLivePostgresRevokeWithoutAuthority` ruft den echten Plan-/Ausführungspfad als eingeschränktes Konto auf. Sie verlangt Abweisung bei erhaltenem direkten Recht und prüft erfolgreiche Entzüge trotz PUBLIC-Recht sowie ausschließlich entzogene Grant Option.

Fix: direkte gespeicherte ACLs innerhalb derselben Transaktion nachlesen; bei verbliebenem angefragtem Recht oder fehlgeschlagener Prüfung zurückrollen und spezifischen Grund liefern. Geerbte Rechte werden nicht mit direkten Rechten verwechselt. Keine Änderung am Planformat. Das unabhängige statische Review fand zusätzlich nichtkanonische Routinen-OIDs als Vergleichslücke; die Regression wurde zuerst um führende Nullen erweitert, danach die OID für den Vergleich numerisch kanonisiert.

Prüfstand: Test vor Produktänderung geschrieben, absichtlich nicht ausgeführt. Der bisherige Code liefert für die erste Assertion `completed`; dies ist statisch hergeleitet, kein beobachteter Rotlauf. Exakte Aufruferbefehle folgen im Wurzelbericht. Keine eigenen laufenden Jobs. Unabhängiges Review vor Integration bleibt erforderlich.

## Befund 2: Unicode-Passwörter (mittel)

Bestätigt: Die bisherige SASLprep-Näherung ignorierte Bidi- und mehrere Verbotsregeln und normalisierte vor dem Mapping. Regression `TestSASLPrepPostgresFallbackAndMapping` wurde zuerst ergänzt: arabischer Buchstabe plus feminines Ordinalzeichen muss die Originalbytes behalten; außerdem Mappingreihenfolge, gültiges Bidi, Verbots- und Unicode-3.2-Tabellen sowie deterministische SCRAM-Ableitung.

Fix: vollständiges RFC-4013-Profil von `github.com/xdg/stringprep` v1.0.3 mit PostgreSQLs Vorrang der Leerzeichenabbildung und Rückfall auf unveränderte Originalbytes bei Fehlern. Lizenztext und Inventar ergänzt, Modul und Summen ausschließlich aus lokalem Cache. Keine Netzaufrufe, keine ausgeführten Tests; das vorherige Fehlverhalten ist statisch nachgewiesen. Keine API-Änderung.

## Befund 3: Routinenrechte bei Wiederherstellung (mittel)

Bestätigt: Vor DROP wurden nur Definition und Sitzungseinstellungen gesichert. DROP entfernt lokale Routinenrechte, die automatische Neuanlage vergibt Rechte an den aktuellen Ersteller. Zuerst Regression `TestLiveMySQLRoutineRestorePrivileges` geschrieben: anderes Erstellerkonto, automatische EXECUTE-/ALTER-ROUTINE-Rechte, zusätzlich explizite EXECUTE-Grant-Option für ein zweites Konto, danach fehlerhafter Ersatz und unabhängiger Vergleich aller lokalen ACLs.

Fix: vor jedem zweistufigen Routinenersatz die vollständigen lokalen Rechte aus `mysql.procs_priv` sichern (bei fehlender Katalogberechtigung vor DROP abweisen). Nach Wiederherstellung Rechte einschließlich Grant Option abgleichen, zusätzliche automatische Rechte entfernen und den Katalog erneut vergleichen. Nur bestätigte Wiederherstellung erlaubt `rejected/restored`; sonst `outcome_unknown/restore_failed`. Keine direkte Systemtabellenänderung, nur GRANT/REVOKE. Globale und schemaweite Rechte werden nicht verändert. Parallele fremde DDL-/ACL-Änderungen sind weiterhin nicht atomar abgesichert.

Test zuerst geschrieben, nicht ausgeführt. Benötigt Wegwerf-MySQL bzw. -MariaDB mit `automatic_sp_privileges=1`, TLS-Fixture und administrativem Katalog-/Rechtezugriff. Erfolgreicher regulärer Ersatz bleibt im bisherigen Umfang; dieser Fix betrifft die Wiederherstellung nach fehlgeschlagenem CREATE. Keine laufenden Jobs.

## Befund 4: Automatischer SQLite-Rollback (niedrig)

Bestätigt und wegen kleinen Fixes übernommen: Die Statuskorrektur hing ausschließlich vom Erfolg des abschließenden expliziten ROLLBACK ab. Zuerst `TestSQLiteImportSQLAutomaticRollback` ergänzt: ON CONFLICT ROLLBACK in stop/continue, bereits früher bestätigter Autocommit und spätere Autocommit-Anweisung getrennt prüfen.

Fix: Nach Ausführungsfehler in einer expliziten SQLite-Transaktion den nativen Autocommit-Zustand auf der gepinnten Verbindung lesen. Ist die Transaktion bereits beendet, frühere Anweisungen genau dieser Transaktion als `rolled_back` markieren, Änderungszahlen entfernen und die Transaktionszuordnung beenden. Unbekannter Gesamtfehler bleibt konservativ bestehen. Keine Interpretation von Fehlermeldungstexten.

Test zuerst geschrieben; keine Tests, Builds, Docker- oder Netzaufrufe ausgeführt. Keine laufenden Jobs. Die Regression verlangt im alten Code `rolled_back`, erhält statisch hergeleitet jedoch `completed`.

## Übergabe und Fortsetzung

Nachbesserung 17.11.2026 auf Basis `786aba0`: externer Fixture-Lauf laut Aufrufer
bis auf Routinen-OID mit führenden Nullen grün. Dateibesitz dieses Nachtrags:
`accounts_plan.go`, `accounts_scram.go`, `accounts_test.go`, `accounts_live_test.go`
und die vier Paket-/Statusdokumente. Gemeinsame dezimale 32-Bit-Kanonisierung vor
beiden OID-SQL-Parametern und beim ACL-Vergleich; Unit-Regression und Live-GRANT
mit führenden Nullen ergänzt, REVOKE-Erwartung unverändert. Die wechselnde OID
erklärt statisch die unterschiedlichen Parserfehler; keine zusätzliche
Isolationsänderung begründet. Einzelheiten im Fixbericht.
Unabhängiges lesendes Agent-Review: keine blockierenden Befunde.

Stand 19.11.2026: abgenommen. `e135469` ist auf `main` integriert. Gegen eine
Wegwerf-PostgreSQL 18.6 mit TLS sind im Gesamtlauf (CI
[Lauf 44093186734](https://github.com/MagicalWig34653/db-manager/actions/runs/44093186734))
und im lokalen Einzellauf beide Tests ausgeführt und grün:
`TestLivePostgresRevokeWithoutAuthority` und `TestCanonicalPostgresRoutineOID`.
Einzelheiten im [Fixbericht](../../BERICHT-FIXES.md). Paket abgeschlossen, keine
offenen Schritte und keine laufenden Jobs.
