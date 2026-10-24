# Review W-P07 Backend: Datenbank- und Tabellenoperationen (API 0.8)

Reviewer: unabhängig (nicht Autor). Geprüft: `feat/w-p07-backend` (bebe262, Basis main 5921a9d), 24.10.2026. Schwerpunkt destruktive Pfade.
Review-Branch: `review/w-p07-backend` (nur Tests und dieses Dokument).

## Urteil

Freigabe mit Auflagen (accept with changes): zwei mittlere Befunde (M1, M2) sollten vor dem Webpaket behoben oder im Plan ausgewiesen werden.
Die Bestätigungslogik, Injektionsabwehr und der Zerstörungsradius in MySQL, MariaDB und SQLite sind ohne Befund.

## Ausgeführt

| Prüfung | Ergebnis |
| --- | --- |
| `gofmt -l .`, `go vet ./...` | ohne Ausgabe |
| `go test -race -count=1 ./...` | alle Pakete ok |
| fünf Wächter, `check-overview-contract.py`, `check-object-ops-contract.py` | bestanden (v1 bis v7 byteidentisch, v8 additiv, 80 Fixtures) |
| SHA-256 `contracts/api-v0.8.0.json` | `74aa39ed...a28264`, stimmt mit Handoff |
| `scripts/test-object-ops.py` | bestanden, 44 echte Antworten gegen v8-Schema; keine Container übrig |
| `scripts/check-docs.py` | 0 Fehler |
| eigene Tests `objectops_review_test.go` | SQLite, MySQL 8.4, MariaDB 12.3, PostgreSQL 18.6 bestanden (je ein Wegwerf-Container, nacheinander) |

## Befunde

### M1 (mittel): PostgreSQL TRUNCATE/DROP auf Eltern wirkt auf Kindtabellen, Plan verschweigt es
Beleg (Livetest, PostgreSQL 18): `TRUNCATE TABLE "s"."rv_parent"` (Vererbung, Kind via `INHERITS`) leert auch `rv_inh_child` (3 Zeilen, danach 0); der Plan nennt das Kind weder in `affected_objects` noch in den Warnungen.
`DROP TABLE` einer partitionierten Tabelle entfernt auch die Partitionen; Plan und Warnung nennen sie nicht (`affected=[table:s.rv_part]`).
Das widerspricht dem Zerstörungsradius „trifft nur das Ziel". Der getippte Zielname schützt nur den Elternnamen.
Fix: für Vererbungselternteile `TRUNCATE ONLY` verwenden (Partitionen: Hinweis, da ONLY dort nicht möglich ist), Kinder/Partitionen aus `pg_inherits` in `affected_objects` und Warnungen aufnehmen (der Hash ändert sich dadurch automatisch), oder solche Tabellen mit `unsupported_action` ablehnen.

### M2 (mittel/niedrig): Replay einer Bestätigung bleibt gültig
Beleg: Der Plan-Hash ist zustandslos. Nach `table_truncate` und erneutem Befüllen führt dieselbe Anfrage mit demselben `expected_plan_sha256` und getipptem Namen die Leerung erneut aus (SQLite-Test, gleiches Verhalten auf allen Treibern; für drop/rename/database_drop schlägt der Replay mit `object_not_found` fehl).
Einschränkung: Aufrufer ist per Bearer-Token authentifiziert, Replay erfordert den Besitz der vollständigen Anfrage; kein Fremdangriff, aber ein veralteter UI-Zustand (doppelter Klick, Wiederholung nach Netzfehler) kann Daten erneut löschen.
Fix: Einmal-Nonce/Zeitstempel nicht im Vertrag vorsehen, stattdessen im Plan beim Truncate den Zeilen-/Zustandsstempel (zum Beispiel geschätzte Zeilenzahl oder Plan-Zeit) nicht hashen, sondern die Webschicht verpflichten, nach `completed`/`outcome_unknown` die Aktion zu sperren; als Konvention im Vertragstext festhalten.

### L1 (niedrig): TOCTOU-Fenster zwischen Planneuberechnung und Ausführung
Beleg: Der Plan wird auf derselben Sitzung unmittelbar vor der Ausführung neu berechnet (Millisekundenfenster). Ersetzt eine andere Sitzung die Tabelle in diesem Fenster, wirkt die unveränderte Anweisung auf das neue Objekt gleichen Namens. Die Anweisungen sind namensbasiert, der Hash deckt dies nicht ab.
Fix (optional): in PostgreSQL vor der Neuberechnung `LOCK TABLE ... IN ACCESS EXCLUSIVE MODE` innerhalb der Transaktion, in MySQL Metadatensperre via `LOCK TABLES`; sonst als bekannte Grenze dokumentieren.

### L2 (niedrig): `table_options` mit `collation_mode=convert` und Engine-Wechsel ohne getippten Namen
Beleg: Beide können Daten verändern (Zeichensatzumwandlung) beziehungsweise Funktionen entfernen (Fremdschlüssel bei Engine-Wechsel), sind aber nicht `destructive`; die Warnung steht im Plan, der Hash ist bestätigt. Fix: `destructive=true` für `convert` und Engine-Wechsel, oder Webpaket zeigt die Warnung zwingend an.

### L3 (niedrig): Verbindungsdatenbank-Erkennung vergleicht Zeichenfolgen exakt
Beleg: `isCurrent := env.c.Database == name`. Auf Servern mit `lower_case_table_names` ungleich 0 (macOS/Windows) könnte die Schreibweise abweichen. Im Test mit Großschreibung der Verbindungsdatenbank lehnt der Server die Verbindung ab (kein Umgehen auf Linux). Fix: stattdessen `SELECT DATABASE()` der Sitzung vergleichen.

### Hinweise (informativ)
- MySQL/MariaDB-Kopie `both` ist nicht atomar; ehrlich als `outcome_unknown` mit `statements_executed` und bleibender leerer Kopie dokumentiert.
- Sichtsuche (`LOCATE` über alle Sichten) ist auf 101 Treffer und das Zeitlimit (Standard 10 s, höchstens 30 s) begrenzt; auf Servern mit sehr vielen Sichten kann sie das Zeitbudget aufbrauchen und führt dann zu einem sicheren Fehler statt zu einer unvollständigen Liste.
- Triggerabhängigkeiten werden nicht ausgewertet (dokumentiert); die Warnung genügt für MySQL-Sichten (`textual`), für Trigger sollte das Webpaket den Hinweis dauerhaft anzeigen.
- Nicht umgesetzt wie angegeben: OBJ-004, Teil von OBJ-005. Doku stimmt damit überein.

## Geprüft ohne Befund
1. Bestätigung: falscher/leer/Groß-/Kleinschreibung/Leerzeichen/Unicode-NFD getippter Name, falsche Aktion, vertauschter Plan, Nullhash, Großbuchstaben-Hash, kurzer/leerer Hash: alle abgelehnt (`confirmation_mismatch`, `plan_mismatch`, `invalid_object_op`), Zustand unverändert (drop, truncate, rename). Ziel kommt aus dem Katalog (exakter Namensvergleich, keine Normalisierung). Verbindungsdatenbank ohne `acknowledge_current_database` abgelehnt, Datenbank bleibt.
2. Injektion: zehn Angriffsnamen je Treiber (Backtick, Quotes, `;DROP`, Unicode, Backslash, Maximallänge) bei rename und copy; acht ungültige Namen (leer, Rand-Leerzeichen, NUL, Steuerzeichen, Zeilenumbruch, zu lang, ungültiges UTF-8, `sqlite_`) abgelehnt; Kommentar mit Quote/Backslash/SQL-Text round-trip exakt; Kollation und Engine nur aus Katalog (Injektionsversuche abgelehnt). Nachbartabelle blieb unverändert.
3. Radius: FK-Kind blockiert drop und truncate (kein CASCADE, nichts verändert); gleichnamige Tabelle im anderen Schema, Nachbartabellen und Sichten bleiben; SQLite: kein Schema- oder Dateizugriff (nur `main`, `sqlite_*` abgelehnt). Rechte: SELECT-Rolle ergibt `rejected/permission_denied` ohne Änderung (Autor-Test, im Lauf bestanden).
4. Ergebnismodell: rejected/outcome_unknown/completed wie beschrieben; Meldungen aus fester Tabelle, keine Treiberfehler, SQL oder Pfade.
6. Vertrag: SHA stimmt, additiv, Fixtures echt, alle Routen hinter `a.auth`, Body-Limit 1 MiB, Methoden POST/GET wie im Vertrag.

## Ergänzte Tests (Review-Branch)
`internal/database/objectops_review_test.go`: `TestReviewObjectOpsSQLite`, `TestReviewObjectOpsLive` (mysql, maria, postgres). Die PostgreSQL-Fälle für M1 protokollieren das Ist-Verhalten (`t.Logf`) und schlagen nach einer Korrektur nicht fehl.
