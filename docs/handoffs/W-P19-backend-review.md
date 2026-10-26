# Review W-P19 Backend: Views, Routinen, Trigger, Events (API 0.16)

Reviewer: unabhängig (nicht Autor, Codex-Paket). Geprüft: `integration/wave3-backend` (f345adb), 26.10.2026.
Review-Branch: `review/wave3-a`. Schwerpunkt: freie Definitionskörper, Definer, Ersetzen, Abhängigkeiten.

## Urteil

Freigabe mit Auflagen (accept with changes). Kein Statement-Schmuggel, keine Injektion, keine Ausführung von Routinen. Ein mittlerer Befund (M1: Ersetzen mit fehlerhaftem Körper löscht in MySQL/MariaDB das Objekt) und ein mittlerer/niedriger (M2: SECURITY DEFINER ohne search_path-Hinweis).

## Ausgeführt

| Prüfung | Ergebnis |
| --- | --- |
| `gofmt -l .`, `go vet ./...`, `go test -race -count=1 ./...` | sauber, alle Pakete ok |
| alle `check-*-contract.py` | bestanden (programs: Wächter 0.16, 42 Fixtures) |
| SHA-256 `contracts/api-v0.16.0.json` | `d90ac3fd...7bf3`, stimmt mit Integrationshandoff |
| `scripts/test-programs.py` | bestanden: 18 echte Antworten validiert, vier Systeme, keine Container übrig |
| `scripts/check-docs.py` | 0 Fehler |
| eigene Tests `TestReviewW3ProgramsLive` (mysql, maria, postgres) und `TestReviewW3ProgramsSQLite` | bestanden |
| Mutationstest Wächter | wirksam |

## Befunde

### M1 (mittel): MySQL/MariaDB-Ersetzen besteht aus DROP + CREATE; fehlerhafter neuer Körper löscht das Objekt
Beleg (MySQL 8.4 und MariaDB 12.3): `replace` einer Funktion `rv_fn` mit Körper `RETURN (((` ergibt `outcome_unknown`, `statements_executed=1/2`; `information_schema.ROUTINES` danach: 0 Zeilen, die alte Funktion ist weg. Preview-Warnung nennt das Risiko ("bei Teilfehlern ist das Ziel möglicherweise entfernt"), der Ausgang ist aber in Wahrheit bekannt (Objekt gelöscht), und die alte Definition wird weder gesichert noch zurückgegeben. Betrifft Routinen, Trigger, Events (Sichten nutzen natives `CREATE OR REPLACE`).
Fix: alte Definition im Plan (bei replace) und im Ergebnis bei Teilfehler ausgeben; besser: neuen Körper vorab unter einem temporären Namen anlegen und wieder löschen (Syntaxprüfung) oder bei CREATE-Fehler die alte Definition automatisch wiederherstellen und `rejected` melden.

### M2 (mittel/niedrig): PostgreSQL `SECURITY DEFINER` ohne search_path- und PUBLIC-Hinweis
Beleg: Plan für `CREATE FUNCTION ... SECURITY DEFINER` enthält nur die allgemeine "Privilegierte Aktion"-Warnung; kein Hinweis auf `SET search_path` (klassischer Eskalationsweg) und auf das Standardrecht EXECUTE für PUBLIC. Die API bietet keine `search_path`-Einstellung.
Fix: Warnung ergänzen; optional Feld `search_path` und `REVOKE EXECUTE ... FROM PUBLIC` als eigener Planschritt.

### L1 (niedrig): Definer-Wechsel wird nicht vorab auf SET_USER_ID/SUPER geprüft
Beleg: Ein Benutzer mit nur CREATE VIEW erhält beim Versuch `DEFINER=root` einen sicheren generischen Serverfehler, nichts wird angelegt (Test). Die Vorschau warnt ("Privilegierte Aktion"); der Server entscheidet. Fix (optional): vorab `SET_ANY_DEFINER`/`SUPER`/`SET_USER_ID` der Verbindung prüfen und früh klar melden.

### L2 (niedrig): `confirm.action` wird gegen die Aktion der Anfrage statt der Route verglichen
Beleg: `dbplan.Check(..., r.Action, ...)`; Aktion (create/replace/drop) steckt aber in den Anweisungen und damit im Hash, daher kein Bypass. Hinweis, nicht dringlich.

### L3 (niedrig): `create` ist nicht destruktiv und braucht keinen getippten Namen
Beleg: Vertretbar (legt nur neu an, bestehende Objekte werden nicht ersetzt: `create` über vorhandener Sicht scheitert). Events werden aktiv angelegt, wenn `status=enabled`; die Warnung "zeitversetzt Code ausführen" ist vorhanden.

## Geprüft ohne Befund
1. Freie Körper: Sichten mit `...; DROP TABLE rv_victim`, mit Zeilenumbruch/Kommentar/zweitem Statement, `DELIMITER`-Direktive, NUL: nie `completed`, Tabelle unversehrt (MySQL/MariaDB: Verbindung ohne MultiStatements, PostgreSQL und SQLite: `Prepare` lehnt Mehrfach-Statements ab). MySQL: Routinen, Trigger und Events mit `BEGIN ... END; DROP TABLE ...` abgewiesen. SQLite-Trigger mit angehängtem `DROP TABLE` abgewiesen. Es gibt kein Client-Splitting.
2. PostgreSQL: Funktionskörper mit `$$`, Quotes und `;DROP TABLE` wird als String-Literal exakt gespeichert (`prosrc` identisch) und führt nichts aus; SQL-Funktion mit zweitem Statement bei Anlage abgelehnt; Trigger auf nicht vorhandener Tabelle abgelehnt; Trigger nutzt nur die vom Katalog aufgelöste Funktion mit Rückgabetyp `trigger`.
3. Bezeichner: Namen mit Backtick/Quotes (Sichten `rv`w"v`, PostgreSQL/SQLite `rv"w'v`), Abhängigkeitssuche erkennt die eingehende Sicht (`rv_dep`) mit verdoppelten Backticks/Quotes. Drop-Identität in PostgreSQL stammt aus dem Katalog: `a integer); DROP TABLE ...` wird `object_not_found`; `a integer` löscht genau diese Überladung, die andere bleibt.
4. Bestätigung: replace/drop eines vorhandenen Objekts ist `destructive` mit getipptem Katalognamen; fehlender, großgeschriebener, mit Leerzeichen versehener oder fremder Name, falsche Aktion, fremder Plan, Nullhash und Großbuchstaben-Hash lehnen ab, Sicht unverändert. Der Plan weist darauf hin, dass der bestätigte Plan keine Einmalfreigabe ist.
5. Keine Ausführung: weder CALL noch `SELECT func()` im API-Code; Anlegen und Planen einer Prozedur führt sie nicht aus (Zähltabelle bleibt 0). Event-Scheduler wird nur gelesen (`@@GLOBAL.event_scheduler` vorher/nachher gleich), ein deaktiviertes Event wird deaktiviert gespeichert; `ALTER EVENT` wird nicht erzeugt.
6. SQLite: Sichten/Trigger ja; Funktionen/Prozeduren/Events `unsupported_action`; `load_extension` und `readfile` in einer Sicht lassen sich anlegen, scheitern aber beim Lesen in der Sandbox.
7. Verträge/Auth: SHA stimmt; Mutationstest: angehängtes Byte an v14 und v15, entfernte Strikt-Eigenschaft in v16 und Zusatzfeld in einem echten Fixture (`ProgramPlan.real-mariadb.json`) lassen den Wächter fehlschlagen. Alle fünf v16-Routen hinter Bearer, POST/GET wie im Vertrag, 1-MiB-Limit.
