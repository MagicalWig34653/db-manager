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
