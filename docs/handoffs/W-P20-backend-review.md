# W-P20 Backend: unabhängiges Sicherheitsreview

Stand: 27.10.2026. Geprüft: Branch `feat/w-p20-backend` @ `3583fa8` (Basis `main` `84a5e2e`; `main` ist inzwischen `daceabb`, nur
Dokumentation). Review-Worktree `.worktrees/w-p20-review`, Branch `review/w-p20`. Nichts implementiert und nichts gemergt; es
kamen nur Angriffstests, zwei Prüfskripte und dieser Bericht hinzu. Bezugsdokumente: ADR-W-P20, ADR-0006,
`docs/handoffs/W-P20-backend.md`, `contracts/CONFIG-STORE-API.md`.

## Urteil

**Freigabe mit Auflagen.** Kein Blocker. Besitzerisolation, AEAD-Kontextbindung (Besitzer, Sammlung, ID, Scope, nkey), Sperr- und
Quotenlogik, ETag/If-Match, Plan -> Bestätigung, Opt-in des Verlaufs, Authentisierung, Fehlerklassen und Vertragskette halten
allen Angriffen stand, die ich ausprobiert habe. Offen sind Härtungen, von denen drei vor dem Merge billig sind und später teuer
würden, weil sie Datenformat oder Schnittstelle betreffen:

1. **F-02** Version in den AEAD-Kontext aufnehmen (Format `cfg1` -> `cfg2` später nur mit Migration der Zeilen).
2. **F-03** Den reservierten Besitzer `single` von einem eigenen `OwnerResolver` ablehnen (Schnittstelle, die W-P30 bindet).
3. **F-05** `scope` und `client_ref` wie Schlüssel auf Geheimnisse prüfen (der Scope steht im Klartext in der Datenbank).

Alle übrigen Befunde sind Low/Info und können als Folgeaufgaben oder dokumentierte Restrisiken laufen.

## Ausgeführte Prüfungen

| Prüfung | Ergebnis |
| --- | --- |
| `gofmt -l .`, `go vet ./...` | leer / sauber (auch mit meinen neuen Tests) |
| `go test -race -count=1 ./...` | alle Pakete ok (Datenbankpaket 139 s), vor und nach Hinzufügen der Angriffstests |
| alle 23 `scripts/check-*-contract.py` und `check-docs.py` | grün (1442 Dateien, 0 Fehler) |
| `scripts/test-config-store.py` gegen eigene Wegwerf-MariaDB 12.3 (TLS) | grün, 12 echte Antworten gegen API 0.24 validiert, Container entfernt |
| `scripts/review-w-p20-live.py` (eigene Angriffe gegen eigene MariaDB 12.3) | grün, `leftovers()` leer; fremde Container (`todo-dev-*`, `timetrack-dev-*`, `dbmanager-wp16-*`, `mariadb`) nicht berührt |
| Vertragskette | v1..v23 gegenüber `84a5e2e` byte-identisch (`git diff` leer); v23 SHA `7450a7fc...` = Pin; v24 SHA `e2f2e057...`; v24 fügt nur 8 Pfade hinzu, ändert kein Schema, Sicherheitsschemata gleich |
| Mutationsproben | 40 Quellmutationen (M01-M40) + 10 Wächtermutationen (G1-G10), siehe unten |

## Befunde nach Schwere

Kennungen F-xx stehen in den Kommentaren der Angriffstests. `Gap`-Tests halten das heutige Verhalten fest und müssen nach einer
Korrektur umgedreht werden; `Holds`-Tests müssen dauerhaft grün bleiben.

### Medium

**F-02 AEAD bindet die Version nicht: Rollback und Wiederbeleben möglich.** `itemContext` (`internal/serverstate/configrepo.go`)
enthält `cfg1|Besitzer|Sammlung|ID|Scope|nkey`, nicht die Version. Wer in der Datenbank schreiben, den Schlüssel aber nicht lesen
kann, setzt einen älteren `sealed`-Blob zurück (der Eintrag zeigt die alten Daten, `version` und `updated_at` bleiben unauthentisiert) oder
spielt eine gelöschte Zeile aus einem Backup oder einer Kopie wieder ein; beides entschlüsselt fehlerfrei. Reproduktion:
`TestReviewLiveGap_RollbackAndResurrection` (alter Blob nach Update zurückgeschrieben -> `Get` liefert `{"v":"old"}` mit Version 2;
Zeile nach `DELETE` aus Kopie eingefügt -> wieder lesbar). Zeilentausch zwischen Einträgen, Kind-/nkey-/Scope-/Besitzeränderung
sind dagegen sicher (`TestReviewLiveHolds_RowSwapAndKindChange`, Mutationen M19-M21 getötet). Auswirkung: Integrität gegen einen
Datenbank-Schreiber ohne Schlüssel ist unvollständig (Löschen bleibt ohnehin unerkennbar). Empfehlung: `version` in den Kontext
(unter der Besitzersperre ist die neue Version bei Insert 1 und bei Update `alt + 1` bekannt), Format `cfg2`; jetzt, solange keine
Produktionszeilen existieren. Vollständigen Schutz gegen Löschen/Zurücksetzen auf eine alte Sicherung bietet das nicht, das Restrisiko
ins Bedrohungsmodell schreiben.

**F-03 Besitzer `single` ist für einen eigenen Resolver nicht reserviert.** `validOwner` lässt `[A-Za-z0-9_-]{1,64}` zu, also auch
`single`, den Besitzer aller heutigen Zeilen. W-P30 überführt diese Zeilen per `UPDATE` auf das Administratorkonto; bis dahin und für
jede Kontokennung, die aus einem Namen abgeleitet wird, ist `single` ein gültiger Kontoname, der fremde Altdaten liest.
`OwnerResolver` bekommt das ganze `*http.Request` und liefert einen ungeprüften String: ein Resolver, der eine Kopfzeile oder ein
Formfeld auswertet, würde den Besitzer vom Request beeinflussen lassen (die Schnittstelle verhindert das nicht, nur die Konvention).
Heute (ohne Resolver) kann ein Request den Besitzer nicht beeinflussen. Reproduktion:
`TestReviewGap_ResolverCanCollideWithLegacyOwner`. Empfehlung: in `WithOwnerResolver` den Rückgabewert `single` ablehnen (401),
Konto-IDs über ein eigenes Präfix (`acct_`) trennen und im Vertrag festhalten, dass der Resolver nie aus Request-Daten ableiten darf,
die der Client wählt (nur aus der serverseitig geprüften Sitzung).

**F-01/F-11 `nkey` und `scope` sind ungesalzene SHA-256 über niedrige Entropie (konkret bewertet).** Der Angreifer ist ein Leser der
Datenbank (Betreiber, Backup-Dieb) ohne Serverschlüssel. Er sieht `scope` (SHA-256 der Verbindungsidentität, laut ADR 4 wie im Web)
und `nkey = SHA-256("key"\0 Sammlung \0 Scope \0 Schlüssel \0)`. Mit Wörterbuch für Verbindungsidentitäten
(`mariadb://root@localhost:3306/shop` ist erratbar) und Tabellennamen (`customers`, `salaries`) bestätigt er, welche Verbindungen ein
Nutzer nutzt, welche Tabellen er favorisiert und welche Layout-Schlüssel (`table:db.tabelle`) existieren. Reproduktion:
`TestReviewLiveGap_NKeyDictionaryRecoversTableNames` (Scope erraten, Favoriten-Tabelle und Layout-Schlüssel aus dem Dump
wiederhergestellt). Inhalte (SQL, Namen, Dokumente) bleiben unlesbar. ADR 9 akzeptiert das Restrisiko ausdrücklich; ich bewerte es als
Medium, weil Tabellennamen oft selbst sensibel sind und die Gegenmaßnahme billig ist: `nkey` als HMAC-SHA-256 mit einem eigenen,
von der Verschlüsselungsrotation getrennten Schlüssel (HKDF aus dem Serverschlüssel mit fester Kennung, oder zusätzlicher stabiler
Schlüssel); den Scope vom Client salzen (z. B. mit einem installationsspezifischen Wert, den der Server zurückgibt). Kein Blocker,
aber vor der Weboberfläche entscheiden, weil das Scope-Format dort entsteht.

### Low

**F-05 `scope` und `client_ref` werden nicht auf Geheimnisse geprüft.** `prepare` ruft `LooksSecret(..., ScanIdentifier)` nur für den
Schlüssel auf. Der Scope steht im Klartext in `config_items.scope`, `client_ref` im Siegel. Ein Tokenformat (`ghp_...`, `AKIA...`)
wird als Scope gespeichert (`TestReviewGap_ScopeAndClientRefCarryUnscannedSecrets`,
`TestReviewLiveGap_ScopeIsPlaintextInTheDump`). Das widerspricht der Aussage "Klartext/Schlüssel nicht im Dump" nur für
böswillige oder fehlerhafte Clients, aber der Server verspricht Ablehnung. Empfehlung: Scope und `client_ref` mit `ScanIdentifier`
prüfen; Scope-Länge/Format (64 Hex) für die Weboberfläche vorschreiben.

**F-06 Dokumente (`settings`, `layouts`) speichern Klartext-Zugangsdaten unter harmlosen Schlüsseln.** Nur strukturelle Regeln
gelten (ADR 7, bewusst, damit `password_hash` als Spaltenname geht). Angenommen werden `{"password":"hunter2"}`,
`{"dsn":"host=db user=app password=hunter2"}`, `curl -u admin:hunter2`, `{"pin":"4711"}`, 32 Hex-Zeichen
(`TestReviewGap_DocumentsAcceptPlainCredentials`). Empfehlung: Regel auf den Wert ausdehnen, wenn der zugehörige Schlüssel
`pass|secret|token|key|pwd` enthält und der Wert ein String mit mehr als einem Zeichen ist (Spaltenlisten sind Arrays von Namen,
nicht Schlüssel-Wert-Paare mit Geheimwert); mindestens `password=`/`pwd=` im Wert für Dokumente ablehnen.

**F-07 Das verbotene Literal (Bearer-Token) wird nur roh verglichen.** `strings.Contains(text, value)` ohne NFKC-Faltung und
ohne Entfernung unsichtbarer Zeichen; `tokenvalue<ZWSP>-...` oder die Vollbreitenform passiert in Dokumenten (in SQL-Sammlungen
fängt oft das Wort `token`; Layouts/Settings haben keine Wortregeln). Der Klartextvergleich selbst (inkl. `Bearer <token>` über die
Strukturregel) greift (`TestReviewHolds_BearerTokenLiteralRejected`). Auch Token unter 8 Zeichen werden ignoriert
(`AddForbidden`); das Server-Start-Token ist durch die Konfiguration länger. Empfehlung: Vergleich auf `fold(text)` gegen
`fold(value)`; Mindestlänge im Konfigurationscode erzwingen.

**F-04 Unsichtbare Zeichen außerhalb von Unicode-Kategorie Cf teilen Schlüsselwörter.** `fold` entfernt NFKC-gefaltete Zeichen und Cf
(Zero-Width, Soft Hyphen, Word Joiner, Bidi, Tags), nicht aber U+034F (Combining Grapheme Joiner), Variation Selectors
(U+FE00-FE0F), Hangul-Füller (U+115F, U+3164, U+FFA0) und nicht Homoglyphen (kyrillisches a). `SET PASS<U+034F>WORD = 'x'`
wird gespeichert (`TestReviewGap_InvisibleNonCfCharactersSplitKeywords`). Solcher Text ist kein gültiges SQL; es geht nur um
absichtliche Umgehung durch den eigenen Nutzer, daher Low. Empfehlung: Mn/Me-Zeichen und die Füllzeichen mit entfernen,
Confusables-Skeleton (UTS #39) für ASCII-Zielwörter ergänzen.
Gehalten: Vollbreite, ZWSP, Soft Hyphen, Word Joiner, RTL-Override, Tags, kombinierende Akzente (`TestReviewHolds_FoldedKeywordsRejected`).

**F-10 ADR 6 und Verhalten weichen ab: Byte-Quote verdrängt Verlauf nur für Verlauf.** ADR: "32 MiB je Besitzer ... Verlauf wird
zuerst verdrängt, sonst `quota_exceeded`". `makeRoom` verdrängt nur Zeilen der Sammlung des neuen Eintrags und nur, wenn diese FIFO ist;
ein Lesezeichen bei vollem Speicher durch Verlauf ergibt `409 quota_exceeded` (`TestReviewGap_OwnerByteQuotaDoesNotEvictHistoryForOtherKinds`).
Unkritisch für die Sicherheit (fail closed), aber ein Nutzer sperrt sich über den Verlauf selbst aus. Empfehlung: ADR/Vertrag
korrigieren oder bei der Byte-Quote Verlauf (dann Recents) vor Ablehnung verdrängen.

**F-12 Migration 2 akzeptiert fremde Vorab-Tabellen.** `CREATE TABLE IF NOT EXISTS` plus Prüfsumme über den Anweisungstext: eine
vorhandene, falsch definierte `config_items` (ohne Primär-/Unique-Schlüssel) wird als migriert verbucht
(`TestReviewLiveGap_ForeignTableAcceptedByMigration`); danach sind doppelte IDs möglich. Empfehlung: nach der Migration `SHOW CREATE
TABLE`/`information_schema` gegen den erwarteten Primär-/Unique-Schlüssel prüfen (`VerifySchema`). Voraussetzung ist, dass jemand mit
DDL-Rechten vorher falsche Tabellen anlegt; Low.

**F-13 Doppelte Schlüssel in der Anforderungshülle werden akzeptiert (letzter gewinnt).** `{"scope":"a","scope":"b",...}` ergibt Scope `b`
(`TestReviewGap_DuplicateEnvelopeKeys`). Die Dokumentinhalte (`data`) und versteckte Nullen hinter doppelten Hüllschlüsseln sind
dagegen geschützt (`TestReviewHolds_DuplicateKeyInsideData`: 422 bzw. 400). Der gemeinsame `decode` ist älter als W-P20; harmlos, weil
alle Felder derselben Authentisierung unterliegen. Info/Low.

**F-16 Opt-in-Prüfung des Verlaufs liegt außerhalb der Besitzersperre (nicht reproduziert, Quelltextbefund).** `historyEnabled`
liest, danach schreibt `Insert` unter eigener Transaktion; ein gleichzeitiges Abschalten kann einen letzten Verlaufseintrag
durchlassen, der Stapelimport prüft einmal je Stapel. Praktisch ohne Wirkung (der Nutzer kann ihn löschen); als Eigenschaft im Vertrag
nennen oder die Prüfung in die Transaktion ziehen.

### Info

- **F-08/F-09 Heuristik:** nicht erkannt werden 32-Hex-Schlüssel, 64 Hex mit Leerzeichen, Base32, `curl -u`, Basic-Auth-Header,
  `base64(user:passwort)`; es gibt Falschpositive: SHA-256 eines Datei-Hashes im `WHERE`, Tabelle `tokens`, `uid = 0`, lange
  gemischte Bezeichner (>= 40 Zeichen). Das passt zu "konservativ" (ADR 7); die Fälle stehen in
  `TestReviewGap_OpaqueRunHeuristicMisses` und `TestReviewGap_FalsePositives` (protokollieren, nicht festschreiben).
- **F-14 Programmrücknahme:** Ein Altprogramm (nur Version 1) startet gegen Schema 2 nicht (`ErrSchemaMismatch`, getestet in
  `TestReviewLiveMigration2`). Ein Rollback des Programms setzt Datenbank-Wiederherstellung oder ein Gegenstück voraus; in
  `docs/BETRIEB.md` ergänzen. Neues Programm auf Schema 1: `ErrSchemaOutdated`, die Routen antworten 503 (fail closed), der Repo-Zugriff
  meldet `state_unavailable`, kein Treibertext.
- **F-15 Wächterlücken:** `TestAllKnownRoutesRegisteredAndAuthenticated` führt weder v23- noch v24-Routen (die Liste endet bei v22); v24 deckt
  `TestConfigRoutesRequireAuthentication` mit allen 11 Methode/Pfad-Paaren ab. Die Mengenlimits je Sammlung (Quoten, Größen) pinnt der
  Wächter nicht (nur `MaxOwnerBytes`; Probe G10 überlebt). Beides in der Wächterliste nachtragen.
- **F-17 Schlüsselhandhabung/Rotation:** Rotation (neuer Schlüssel zuerst, alter als zweiter) liest alte Zeilen und schreibt bei Änderung
  neu (getestet); es gibt keinen Neuverschlüsselungslauf, der alte Schlüssel muss unbegrenzt bleiben. `key_id` im Klartext ist nicht
  authentisiert, aber unverwertbar (der Siegel enthält die Kennung im authentisierten Kopf). Schlüssel und Klartext im Dump: keine
  (bestehender Test plus meine Zeilentausch-/Kopieproben). Nonce 96 Bit zufällig: bei <= 2^32 Siegeln je Schlüssel unkritisch.
- **ETag-Formen:** schwache (`W/"1"`) und Listen-Werte (`"1", "2"`) ergeben 400 statt der RFC-Auswertung; das ist strenger und sicher.
  `If-Match: "0"` ergibt 412, Überlauf 400.
- **capabilities ohne RequireState:** nur mit Bearer erreichbar, kein Informationsleck jenseits von `available`/`reason`/`owner_model`/Grenzen;
  bei Ausfall `local_only/unreachable`, die Datenrouten 503 mit `Retry-After`, ohne Token 401 (nie 503 vor 401).
- **Nicht geprüft:** MariaDB 10.11/MySQL, echte Weboberfläche, Last über mehrere Minuten, Verhalten eines hängenden (nicht abgeschalteten)
  Datenbankservers im Stapelimport (15 s je Operation, bis zu 100 Einträge; Quelltextbefund, nicht gemessen).

## Geprüfte Bereiche (kurz)

**(1) Isolation, AEAD, Migration.** Fremde Eintrags-ID/Sammlung (Get/Put/Delete) -> `not_found`; Cursor eines Besitzers bei einem anderen
liefert nie Fremdzeilen; Clear/Plan-Hash eines Besitzers wirkt nicht für einen anderen; Zeile in anderen Besitzer/Scope/Sammlung/nkey
kopiert oder getauscht -> `state_unreadable`; falscher Schlüssel -> `state_unreadable`; Export mit einer defekten Zeile schlägt
geschlossen fehl. Migration 2 unter `GET_LOCK`: 8 gleichzeitige Instanzen wenden jede Version genau einmal an; Abbruch nach der ersten Anweisung
wird fortgesetzt (`IF NOT EXISTS`, Verbuchung erst nach beiden); zweiter Lauf ist ein No-op. Rollback der Daten gibt es nicht (DDL, keine
Gegenmigration, F-14).

**(2) Geheimniserkennung.** siehe F-04..F-09; gehalten: Strukturregeln auch in Schlüsselpositionen, verschachtelt und JSON-escaped.

**(3) API.** If-Match/428/412 inkl. paralleler Replikate (Gewinner genau einer), Quoten exakt unter 24 Parallelschreibern je Besitzer,
Cursor-Manipulation (ungültige Zeichen, Pfade, fremde IDs -> 400), Grenzen (100 Zeichen Name, 100000 Zeichen SQL innen/außen,
Importstapel 100/101, 1 MiB Body -> 413, Content-Type 415), strenges JSON (doppelte Schlüssel auch per Escape, Null, Tiefe 6, 1e999, NUL,
einzelnes Surrogat, Format-/Bidi-Zeichen in Schlüsseln, Kommentare, NaN), Fehlertexte ohne Eingabe-Echo, `Cache-Control: no-store`,
Löschprotokoll (Bestätigung, Groß-/Kleinschreibung, Leerzeichen, fremder/verbreiterter Plan -> 409/422, Wiedergabe nach Ausführung -> 409),
Verlauf-Opt-in (Anlegen, Import, `false`, Typfehler, fremder Besitzer, anderer Scope -> 409 `history_disabled`), alle 11 Routen 401 ohne Token.

**(4) Vertragskette.** Siehe Tabelle; Fixtures: 12 `real-*` stammen aus dem Livelauf (mein eigener Lauf hat 12 echte Antworten erneut gegen API 0.24
validiert), 12 Platzhalter sind als solche benannt, 11 `.invalid` werden vom Wächter abgelehnt.

## Mutationsproben

Quellmutationen (`scripts/review-w-p20-mutations.py`, je eine Zeile geändert, Tests müssen fehlschlagen; 40 Proben, alle getötet, nachdem
ich drei Lücken in der Testsuite geschlossen habe):

| Mutation | Anfangs | Ergebnis |
| --- | --- | --- |
| M01 NFKC-Faltung aus, M02 Cf-Zeichen behalten, M03 Passwortregel, M04 URL-Regel, M05 Lauf 40 -> 80 | getötet | getötet |
| M06 verbotenes Literal aus, M07/M08 Verlaufssperre in Create/Import aus | getötet | getötet |
| M09 Löschbestätigung aus, M10 `validOwner` immer wahr, M11 Liste ignoriert Scope | getötet | getötet |
| M12 Dubletten-Schlüssel, M13 Tiefenlimit, M14 SQL-Zeichenlimit +1, M15 kein 428 | getötet | getötet |
| M16 ohne `RequireState`, M17 Capabilities ohne Anmeldung, M18 Privacy-Schlüssel verfälscht | getötet | getötet |
| M19-M21 AEAD-Kontext ohne Besitzer/Scope/Sammlung | getötet | getötet |
| M22 Besitzersperre ohne `FOR UPDATE`, M23 `Get` ohne Besitzer, M28 Liste ohne Besitzer, M35 Export ohne Besitzer, M36 Clear ohne Besitzer | getötet | getötet |
| M24 Scope-Quote um 1, M26 Byte-Quote, M25 Versionsprüfung, M29 Delete-Version, M34 Version nicht erhöht, M27 veralteter Löschplan | getötet | getötet |
| M30 FIFO des Verlaufs, M31 Zukunftszeit im Import, M33 NUL erlaubt, M38 Schlüssel nicht gescannt, M39 Touch bei Recents | getötet | getötet |
| M32 Cursor-ID-Muster aus | **überlebte** | getötet (neuer Test `ForgedCursorWithOddID`) |
| M37 Verlauf änderbar | **überlebte** | getötet (neuer Test `HistoryImmutable...`) |
| M40 `client_ref`-Idempotenz bei Create aus | **überlebte** | getötet (neuer Test `RefIdempotentOnCreate`) |

Wächtermutationen (`check-config-store-contract.py`): G1 Bytes von v22 geändert, G2 Bytes von v23 geändert, G3 v24 verliert einen Pfad,
G4 neues Schema offen (`additionalProperties`), G5 ererbtes Schema verändert, G6 Route ohne `security`, G7 ungültiges Beispiel wird gültig,
G8 `MaxOwnerBytes`-Drift, G9 Route fehlt im Go-Code: alle getötet. **G10 (Quote `PerScope` 200 -> 999 im Code) überlebt**, der Wächter
pinnt nur `MaxOwnerBytes` (F-15).

## Hinzugefügte Dateien (Review)

- `internal/configstore/review_attack_test.go`, `internal/httpapi/config_review_test.go`, `internal/serverstate/configreview_live_test.go`
- `scripts/review-w-p20-live.py` (eigene Wegwerf-MariaDB, nur `dbmanager-w-p20-rv-*`), `scripts/review-w-p20-mutations.py` (`dbmanager-w-p20-mu-*`)

Ausführen: `go test -race -count=1 ./internal/configstore ./internal/httpapi -run TestReview`; `python3 scripts/review-w-p20-live.py`;
`python3 scripts/review-w-p20-mutations.py [Präfix...]`. Die `Gap`-Tests sind absichtlich grün und kippen, sobald der Befund behoben ist.

## Empfohlene Reihenfolge der Korrekturen

1. F-02 (Version im AEAD-Kontext, `cfg2`), F-03 (`single` ablehnen, Resolver-Vertrag), F-05 (Scope/`client_ref` scannen), danach erneute Review-Tests drehen.
2. F-01/F-11 (HMAC-nkey, Scope-Salz) vor dem Web-Paket entscheiden.
3. F-06, F-07, F-04 (Regeln nachziehen), F-10 (ADR oder Verhalten), F-12 (Schemaprüfung nach Migration), F-15 (Wächterliste, Quoten pinnen), F-14 (Betriebshandbuch).
