# Wave 3 Re-Review (unabhängig)

Reviewer: unabhängig (nicht Autor). Geprüft: `integration/wave3-backend` @ 3ba387d, Branch `review/wave3-rereview`, 26.10.2026.
Gegenstand: Auflagen aus den Reviews W-P18, W-P19, W-P09, W-P14 und neue Fehler in den Fixes.

## Urteil

**Freigabe mit Auflagen.** Alle Auflagen der Erstreviews sind behoben und durch Tests abgesichert (außer M1 aus W-P14, siehe unten: wirksam, aber ohne Regressionsassertion). Es gibt keine Blocker. Die Fixes bringen drei neue mittlere/niedrige Befunde (N1 bis N3), die vor einem Produktivrelease nachgezogen werden sollten.

## Ausgeführt

| Prüfung | Ergebnis |
| --- | --- |
| `gofmt -l .`, `go vet ./...` | sauber |
| `go test -race -count=1 ./...` | alle Pakete ok |
| alle `scripts/check-*-contract.py`, `check-docs.py` | exit 0 |
| `test-accounts.py`, `test-programs.py`, `test-indexes.py`, `test-search.py` (SQLite, MySQL 8.4, MariaDB 12.3, PostgreSQL 18.6, Container nacheinander) | alle exit 0, keine Container übrig |
| `search.yml`: alle referenzierten Skripte | vorhanden (`test-search-guard.py` entfernt) |
| eigene Livetests `wave3_rereview_test.go` | siehe Befunde |
| Mutationsproben | siehe unten |

## Auflagen aus den Erstreviews: Status

| Auflage | Status | Beleg |
| --- | --- | --- |
| W-P18 M1 Systemkonten | behoben | `mysql.sys`, `mysql.session`, `mysql.infoschema`, `mariadb.sys` (jeder Host) und `pg_*` (auch gemischte Schreibung) werden bei drop/lock/password/grant/revoke abgelehnt (`system_account`); `root`/`postgres` verlangen Zweitbestätigung |
| W-P18 M2 PG GRANT ohne Wirkung | behoben | `grant_without_effect`, Rollback; Test im Bestand |
| W-P18 M3 privileged | behoben | GRANT OPTION, SUPER u. a. globale Rechte, `role_grant`, Kopie: `destructive=true` mit getipptem Namen und Warnung (live auf allen Systemen) |
| W-P18 M4 Klartext im PG-Log | behoben | clientseitiger SCRAM-SHA-256 (Format `SCRAM-SHA-256$4096:salt$stored:server`, 16 Byte Zufallssalt); Login mit Klartext klappt (pgx und libpq); `test-accounts.py` prüft `docker logs` auf Marker |
| W-P18 L1/L2 Limits | behoben (siehe N3) | MySQL 256 Zeichen Passwort, Name 32 (MySQL) / 80 (MariaDB) / 63 (PG) |
| W-P19 M1 Auto-Restore | teilweise (siehe N1) | Funktion, Trigger, Event werden bei fehlerhaftem Körper wiederhergestellt (`rejected`, "restored"), MySQL und MariaDB |
| W-P19 M2 PG SECURITY DEFINER | behoben | Warnung nennt search_path und PUBLIC |
| W-P19 L1 Definer-Vorwarnung | behoben | Warnung bei fehlendem SUPER/SET_USER_ID/SET_ANY_DEFINER |
| W-P14 M1 Serverabbruch | behoben | Mit korrekter Probe (COMMAND Query und Execute) 0 aktive Abfragen nach Deadline auf MySQL 8.4 und MariaDB 12.3 (Suche und Replace-Plan); ohne den Fix lief eine Regex-Suche über 600 s weiter (Mutation) |
| W-P14 M2 search.yml | behoben | Zeile entfernt, alle Skripte existieren |
| W-P14 L1 ON UPDATE | behoben | `unsupported_search` im Plan (Mutation schlägt an) |
| W-P09 L1 AUTO_INCREMENT, FK-Waisen | behoben | Warnungen im Plan, Vorprüfung per `NOT EXISTS` mit gebundenen Katalognamen, MATCH-SIMPLE-konform (NULL-Zeilen übersprungen), kein Teilzustand |
| W-P09 L2 Sperrhinweise | behoben | aktionsbezogen je Treiber |

## Neue Befunde

### N1 (mittel): Auto-Restore scheitert bei Routinen mit abweichendem `sql_mode`, Objekt geht verloren
Beleg (MySQL 8.4 und MariaDB 12.3, `wave3_rereview_test.go`, `TestRereviewW3RestoreFidelity`): Funktion, die unter `sql_mode='ANSI_QUOTES,STRICT_ALL_TABLES'` angelegt wurde. `SHOW CREATE` liefert `CREATE DEFINER="root"@"%" FUNCTION "rr_f"...` mit doppelten Anführungszeichen; die Wiederherstellung läuft aber mit dem Sitzungs-`sql_mode` der Verbindung und scheitert mit Syntaxfehler. Ergebnis `outcome_unknown / restore_failed`, die Funktion fehlt. Die Meldung gibt die Definition zur manuellen Wiederherstellung aus (ehrlich), der Datenverlust ist also begrenzt, aber der Fix greift genau im Sonderfall nicht.
Zweitwirkung: Bei erfolgreicher Wiederherstellung (Event) weicht `sql_mode` des Objekts vom Original ab (Session-Modus der Verbindung statt Originalmodus). Die Plan-Warnung nennt das.
Fix: vor der Wiederherstellung `SET SESSION sql_mode = <Originalwert aus Spalte 2 von SHOW CREATE>` setzen und danach zurücksetzen.

### N2 (niedrig/mittel): PostgreSQL-SCRAM ohne SASLprep, Passwörter mit nicht normalisiertem Unicode können sich nicht anmelden
Beleg (PG 18.6): Rolle mit Passwort `ﬁx①ｗpw` (NFKC-verändernd) und `café-pw` (NFD): Anlage `completed`, Anmeldung mit libpq/psql scheitert (`password authentication failed`); NFD scheitert auch mit pgx. ASCII und NFC-Umlaute/Emoji funktionieren. Ursache: Der Server und libpq wenden SASLprep (NFKC) an, der Verifier wird aus rohen UTF-8-Bytes berechnet (im Code als Einschränkung kommentiert, aber ohne Prüfung).
Fix: SASLprep anwenden (`golang.org/x/text/secure/precis`) oder Passwörter, die sich unter NFKC ändern, mit `invalid_secret` ablehnen und dokumentieren.

### N3 (niedrig): Namenslängenlimit zählt Bytes
Beleg: MySQL, Kontoname aus 20 Zeichen `ü` (40 Byte) wird mit "32 Zeichen" abgelehnt, obwohl er nach MySQL-Regel (Zeichen) gültig wäre. Gleiches gilt für Passwortgrenze 256 und MariaDB 80. Konservativ und sicher, aber Meldung ist ungenau. Fix: `utf8.RuneCountInString`.

### N4 (niedrig): Wiederherstellung läuft auch bei unbestimmtem Fehlerausgang
`ProgramExecute` stellt nach jedem CREATE-Fehler nach erfolgreichem DROP wieder her, auch wenn das CREATE (z. B. bei Abbruch/Timeout) serverseitig noch gelungen sein könnte. Dann schlägt das Restore mit "existiert bereits" fehl und die Meldung behauptet "Das Objekt fehlt". Selten; Fix: bei `restore_failed` Existenz per Katalog prüfen und Meldung anpassen.

### N5 (niedrig): Umbenennen auf MariaDB auf den Namen `mysql.sys` wird geplant
`rename` mit `new_account=mysql.sys@localhost` ist auf MariaDB planbar (auf MySQL lehnt der Server "existiert" ab). Die Sperrliste prüft nur `account`, nicht `new_account`/`create`. Kein Datenverlust.

### N6 (niedrig, Testlücke)
- Der Bestandstest `TestReviewW3BScanCancelLive` protokolliert nur und prüft nicht; seine Probe (`INFO LIKE '%FROM \`probe\`.\`big\`%'`) trifft die eigene Abfrage und zeigt immer 1. `TestRereviewW3ReplaceCancel` im Review-Branch ersetzt das durch eine echte Assertion (kein Server-Query aktiv nach Deadline) mit Probe `COMMAND IN ('Query','Execute')`.
- `TestScramVerifierFormatAndDeterminism` pinnt die Iterationszahl von `scramVerifier()` nicht (Mutation 4096 auf 4000 wird nicht erkannt; funktional harmlos, da der Server die gespeicherte Zahl nutzt).
- Die Löschung des Klartext-Schutzes wird nur durch `test-accounts.py` (Logprüfung) erkannt, nicht durch `go test` allein.

## Mutationsproben

| Mutation | Ergebnis |
| --- | --- |
| mariadb.sys aus Sperrliste entfernt | `TestSystemAccountsAreProtected` schlägt an |
| SCRAM-Iterationen 4096 auf 4000 | nicht erkannt (N6) |
| PG sendet Klartext statt Verifier | `go test` besteht, `test-accounts.py` schlägt an (Log-Marker) |
| GRANT-Wirkungsprüfung deaktiviert | `TestReviewW3Accounts` schlägt an |
| Auto-Restore deaktiviert (MariaDB) | `TestReviewW3ProgramsLive` schlägt an |
| PG SECURITY DEFINER-Warnung entfernt | `TestReviewW3ProgramsLive` schlägt an |
| `searchServerLimit` deaktiviert (MariaDB) | Regex-Abfrage lief über 600 s weiter; nur durch die neue Assertion im Review-Test sichtbar (Bestandstests loggen nur) |
| ON UPDATE-Sperre deaktiviert | `TestReviewW3BReplaceLive` schlägt an |
| AUTO_INCREMENT-Warnung deaktiviert | `TestReviewW3BIndexesLive` schlägt an |
| FK-Waisen-Warnung | Assertion im Bestandstest vorhanden (nicht mutiert) |

## Eigene Tests

`internal/database/wave3_rereview_test.go` (Live nur mit dbfix-Umgebung, sonst Skip): `TestRereviewW3Accounts`, `TestRereviewW3ScramSASLprep`, `TestRereviewW3RestoreFidelity`, `TestRereviewW3ReplaceCancel`. `RestoreFidelity` und `ScramSASLprep` dokumentieren N1 und N2 und schlagen daher bis zum Fix absichtlich fehl bzw. protokollieren Anmeldefehler; im Standardlauf ohne Fixture werden sie übersprungen.

## Nicht geprüft
Netzwerkfehler während Restore (N4 nur per Code-Lektüre), MySQL-Rollenrechte in der Definer-Vorwarnung (laut Warnung nicht berücksichtigt).
