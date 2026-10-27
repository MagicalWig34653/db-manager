# W-P20 Backend: unabhängiges Re-Review der Auflagen

Stand: 27.10.2026. Reviewer: unabhängig, nichts implementiert. Geprüft: `origin/feat/w-p20-backend` @ `4af7d52` (enthält `main` @ `daceabb` und
`review/w-p20`), Worktree `.worktrees/w-p20-rereview`, Branch `review/w-p20-rereview`. Grundlage: [Handoff](W-P20-backend.md) (Abschnitt Review-Auflagen),
[Erstreview](W-P20-backend-review.md) (F-01..F-16), [ADR-W-P20](../decisions/ADR-W-P20-konfigurationsspeicher.md) (Abschnitt 9, Restrisiken),
[Vertrag](../../contracts/CONFIG-STORE-API.md).

## Urteil

**Nicht freigegeben, ein Blocker (B-01).** Die neu eingeführte Krypto (cfg2-Kontext mit Version, HMAC-Installationsschlüssel, Besitzer-Salz) ist
korrekt umgesetzt und hielt allen Angriffen stand. Der Blocker liegt in der **neuen** Logik für F-13: `hasDuplicateKeys` (`internal/httpapi/config.go`)
läuft bei fehlerhaftem oder zu tief verschachteltem JSON mit Arrays **endlos** (eine authentifizierte Anfrage belegt dauerhaft einen Kern). Der Fehler
ist mit wenigen Zeilen zu beheben (Abbruch beim ersten Decoderfehler) und braucht keine Neuauslegung. Danach ist der Stand aus meiner Sicht
freigabefähig; die übrigen Befunde sind Low/Info und blockieren nicht.

Vertrag v24 unverändert (SHA-256 `e2f2e057f341e52d23b9ec098c2a1b04e5f90dd18f5e560cbfaad844f463da7a`, seit dem Einführungscommit nicht angefasst), Kette bis v24
und Wächter grün; der Wächter pinnt die Quoten (Mutationen N22/N23 getötet).

## Befunde nach Schwere

### Blocker

**B-01 `hasDuplicateKeys` terminiert nicht bei fehlerhaftem oder tief verschachteltem JSON (DoS, authentifiziert).** In `decodeConfig` läuft die Funktion
vor dem eigentlichen Decoder auf dem rohen Körper. Bei einem Fehler von `dec.Token()` gibt `walk` `false` zurück, die umgebende Schleife
`for dec.More()` bleibt aber wahr (Go 1.27.1, Decoder auf der v2-Implementierung: `More()` meldet nach einem Fehler weiter `true`), und die Rekursion wird
unbegrenzt wiederholt. Reproduziert mit: `{"data":[1,2` (abgeschnitten), `{"data":[1 2]}` (fehlendes Komma), `[`, und mit **gültigem** JSON,
das tiefer als 10 000 Ebenen verschachtelt ist (`[`×100000 `]`×100000, passt in die 1 MiB). Objekte mit demselben Fehler terminieren (`{"a":` liefert `true`),
nur Arrays hängen. Folge: jeder Aufruf einer Konfigurationsroute mit Token und solchem Körper (auch ein Clientfehler) bindet einen Kern und eine Goroutine
bis zum Prozessende; wenige Anfragen legen den Dienst lahm. Die Tests des Erstreviews und der Umsetzung enthalten nur wohlgeformte Doppelschlüssel-Körper.
Reproduktion (läuft nur auf Anforderung, weil sie eine drehende Goroutine zurücklässt):
`DBM_REVIEW_DEMO_HANG=1 go test ./internal/httpapi -run RereviewHang` (alle vier Fälle hängen).
Empfehlung: bei jedem Tokenfehler sofort mit „kein Duplikat, Decoder meldet den Fehler“ zurückkehren (Fehlerflag in der Schleife prüfen) oder vorher
`json.Valid(raw)` aufrufen; zusätzlich eine eigene Tiefenbremse (`MaxJSONDepth` ist für Dokumente geprüft, aber erst *nach* dem Durchlauf). Regressionstest ist der
oben genannte; er soll danach ohne Variable laufen.

### Low

**L-01 F-13 nur teilweise: Doppelschlüssel, die sich in der Groß-/Kleinschreibung unterscheiden, werden akzeptiert.** `{"scope":"a","Scope":"b",...}` und
`{"data":{..},"DATA":{..}}` ergeben 201/200, der letzte gewinnt (Gos Struktur-Decoder ordnet Feldnamen ohne Beachtung der Schreibung zu). Unicode-Escapes
(`"\u0073cope"`) und verschachtelte Duplikate werden erkannt. Vertrag (Zeile 53) und Handoff versprechen „Doppelte Schlüssel irgendwo“; Test:
`TestRereviewGap_CaseVariantDuplicateEnvelopeKeys`. Fix: Schlüssel für die Duplikatprüfung der Hülle klein falten oder einen strikten Decoder verwenden.

**L-02 Geheimnisfaltung: Lücken in der Look-alike-Tabelle.** (a) Der Eintrag `ϲ` (U+03F2) ist wirkungslos, weil NFKC vor der Tabelle U+03F2 zu `ς` umschreibt
(einziger tote Eintrag von allen, geprüft): `seϲret` wird nicht abgelehnt. (b) Nicht abgebildet: lateinische Kapitälchen (`ᴘᴀꜱꜱᴡᴏʀᴅ`), armenisches `ո` für `n`
(`tokeո`) u. a. Die Tabelle ist laut Handoff keine UTS-#39-Skeleton-Tabelle; ich bewerte es als Heuristiklücke. Gegenprobe: keine Kollision, die legitime Namen
ablehnt (kyrillische/griechische Namen wie `Заказы`, `Αθήνα`, `пользователи`, `Σύνολο` werden gespeichert, und die gespeicherten Werte bleiben unverändert
`TestRereviewHolds_FoldingDoesNotChangeStoredValue`); jede erreichbare Tabellenersetzung in `password/secret/token/bearer` wird abgelehnt
(`TestRereviewHolds_ConfusableTableIsSane`). Tests: `TestRereviewGap_DeadConfusableEntry`, `…ConfusablesOutsideTheTable`.

**L-03 Strukturregeln und Dokumentschlüssel: weitere Umgehungen (Heuristik).** `redis://:hunter2hunter2@cache:6379` (leerer Benutzer; die URL-Regel verlangt
mindestens ein Zeichen) und Go-DSN `user:pw@tcp(db:3306)/app` werden nicht erkannt. In Dokumenten (`layouts`, `settings`) wird ein String unter einem
**verschachtelten Objekt** unter einem Zugangsdatenschlüssel akzeptiert (`{"password":{"value":"…"}}`: der Schlüssel `value` ersetzt `password`), ebenso `pw`, `auth`
und Zahlen (`{"password":12345678}`; Zahlen sind laut Kommentar ausgenommen). Test: `TestRereviewGap_StructuralMisses`, `…DocumentKeyRuleBypasses`.
Gehalten: direkte Strings, Arrays, Unicode-Escape im Schlüssel, Homoglyphe im Schlüssel. Vorschlag: den vererbten Schlüssel durch Objekte weiterreichen
(analog Arrays) und `pw|auth|dsn` aufnehmen.

**L-04 F-12 nur teilweise: `Verify` prüft nur Schlüsselzusammensetzung.** Eine vorab angelegte Tabelle `config_meta (name VARCHAR(32) PRIMARY KEY)` (ohne
`key_id`/`sealed`) oder `config_owners` mit nicht binärer Sortierung wird akzeptiert und als Version 2 verbucht; Folge: `IndexKey` schlägt zur Laufzeit fehl
(`state_unavailable`) statt die Migration abzulehnen (`TestRereviewLiveGap_MigrationVerifyChecksKeysOnly`). Auch `sealed`-Typ und Sortierung der Spalten von `config_items` werden
nicht verglichen. Fremde Tabellen mit falschen *Schlüsseln* werden korrekt abgelehnt; Teilzustand und Wiederholbarkeit sowie Parallelstart von 8 Instanzen
(`TestRereviewLiveHolds_ParallelStartCreatesMetaOnce`, `TestReviewLiveMigration2`) sind in Ordnung.

**L-05 Verlust von `config_meta` verwaist Daten still (kein stiller Fehlzuordnung).** Wird die Zeile `index_key` gelöscht (Betreiber, Teilwiederherstellung,
Angreifer mit Schreibrecht), erzeugt der nächste Start einen neuen Schlüssel. Bestätigt: nichts wird falsch zugeordnet (Zeilen bleiben AEAD-gebunden, Scope-Hashes ändern sich),
aber alte Zeilen sind in Listen und der Duplikaterkennung unsichtbar, zählen weiter auf die Quote, werden weiter exportiert, Duplikate werden angelegt, und das
Verlauf-Opt-in geht verloren (Verlauf 409, fail closed). Ein **beschädigter** oder fremder Schlüsselblob wird dagegen nie ersetzt, sondern liefert `state_unreadable`
(`TestRereviewLiveHolds_IndexKeyTamperFailsClosed`). Zusätzlich lädt jede Instanz den Schlüssel einmal und cached ihn für immer: nach einem Löschen und Neuerzeugen
arbeiten laufende Replikate mit dem alten, neue mit dem neuen Schlüssel (Split). Test: `TestRereviewLiveGap_DeletedIndexKeyOrphansRows`. Empfehlung: Betriebshandbuch
(Zeile `config_meta` nie löschen; Wiederherstellung nur zusammen mit `config_items`), optional beim Erzeugen prüfen, ob bereits Zeilen existieren, und dann `state_unreadable` melden statt neu zu erzeugen.

### Info

- **I-01 Index-Schlüssel wird nie neu versiegelt.** Er behält die Schlüsselkennung des Erststarts; nach einer Rotation bleibt der alte Serverschlüssel für immer erforderlich,
  auch wenn alle Zeilen neu versiegelt sind (ohne ihn: `state_unreadable` bei jedem Zugriff, der Hashes braucht). Im Betriebshandbuch steht „alter Schlüssel bleibt erforderlich“,
  es fehlt aber ein Umsiegelungsweg. Test: `TestRereviewLiveGap_RotationNeedsTheOldKeyForever`.
- **I-02 AEAD-Kontext mit `|` als Trennzeichen.** Über den gültigen Wertebereich injektiv (50 000 Zufallstupel, keine Kollision; Besitzer `[A-Za-z0-9_-]`, ID 32 Hex, Scope/nkey Hex,
  Sammlung fest, Version Dezimal; Index-Schlüssel nutzt `cfg2|meta|index_key` mit anderer Feldzahl, Kreuzverwendung scheitert). Mit einem `|` in einem Freitextfeld wäre er mehrdeutig (gezeigt im Test); das schützt allein die
  Validierung (`validOwner` u. a., `TestRereviewHolds_OwnerGrammar`). Da `cfg2` unveröffentlicht ist, ist ein Längenpräfix jetzt billig; kein Befund, sondern Härtung.
- **I-03 Version: Überlauf/Wraparound.** Spalte `BIGINT UNSIGNED`, Go `int64`: Werte ab 2^63 und Manipulationen (±1, 0, 2^64−1) führen zu `state_unreadable`, nie zu einem Absturz oder Wraparound;
  Updates auf manipulierten Zeilen schlagen fehl. 16 gleichzeitige Updates: genau Version 17 und lesbar (Siegel und Version wechseln atomar unter der Besitzersperre,
  `TestRereviewLiveHolds_VersionColumnTamperAndConcurrency`). Rollback nur des Siegels scheitert (`TestReviewLiveHolds_RollbackRejected`). Siegel **und** Versionsspalte gemeinsam
  zurückgesetzt sowie Wiedereinspielen einer gelöschten Zeile bleiben unerkannt (`TestRereviewLiveGap_PairwiseRollbackAndResurrection`); das entspricht dem in ADR 9 dokumentierten Restrisiko
  und ist dort korrekt und vollständig beschrieben.
- **I-04 Quoten: Memory- und MariaDB-Repo identisch im Erfolgsfall, nicht atomar im Fehlerfall.** Differentialtest über 6×120 zufällige Einfügungen (FIFO, Scope-/Besitzer-Anzahl, Byte-Quote mit
  Verlauf-vor-Recents-Verdrängung) liefert dieselben Überlebenden und Fehlerklassen (`TestRereviewLiveHolds_QuotaDifferentialMemoryVsMariaDB`). Nach einer **fehlgeschlagenen** Einfügung hat das Memory-Repo
  bereits verdrängt, MariaDB rollt zurück (`TestRereviewLive_QuotaFailureIsAtomicOnlyOnMariaDB`, `TestRereviewGap_MemoryRepoEvictsBeforeFailingQuota`). Nur Testrepo; die gemeinsame Suite sollte
  das festschreiben. Byte-Quote unter 40 Schreibern auf zwei Replikaten bleibt exakt (≤ Grenze).
- **I-05 Index-Schlüssel (2).** 32 Byte aus `crypto/rand`; Rennen von 16 Replikaten beim Erststart: ein Gewinner (`INSERT IGNORE` plus Neuladen), alle Instanzen gleicher Schlüssel, eine Zeile in `config_meta`
  (`…IndexKeyRaceHasOneWinner`); unabhängige Installationen haben verschiedene Schlüssel; Schlüssel nicht im Klartext im Dump (versiegelt mit dem aktiven Serverschlüssel). Scope ist je Besitzer gesalzen, `nkey`
  je Installation verschieden, kein Klartext-/SHA-Wert (`TestRereviewHolds_KeyedScopeAndNKey`; Wörterbuchangriff des Erstreviews bleibt wirkungslos). Im Go-Code werden keine geheimen Werte verglichen (Bearer
  über `ConstantTimeCompare`; Hashvergleiche im SQL über öffentliche Werte); NUL in Scope/Schlüssel/Bezug wird abgelehnt, nur dadurch ist `macHex` injektiv
  (`TestRereviewHolds_NULNeverReachesTheMAC`).
- **I-06 OwnerResolver.** Gehalten: nur `ctx` (nie Request), `single` → 401, Format → 400 (`"a|b"`, Leerzeichen, 65 Zeichen, Nicht-ASCII, `cfg2|meta`), `Single`/`SINGLE` sind getrennte Besitzer
  (ASCII-binär) ohne Zugriff auf `single` (`TestRereviewHolds_ResolverOwnerVariants`).
- **I-07 ReDoS.** Alle Regexe sind RE2 (linear). Messung 1 MiB ohne `-race`, 11 pathologische Eingaben (`a`×N, `http://a:a:…`, `curl -u -u …`, `basic basic …`, `pass pass …`, `a://b:`…, `Ab1`-Läufe, Zero-Width, kombinierende Marken, NFKC-Ligaturen,
  `curl x\n`…): 0,04–0,56 s je Modus (schlimmster Fall `http://a:a:…` 0,56 s; kombinierende Marken mit 1,5 MiB 0,81 s); gemessen mit `DBM_REVIEW_FULL=1`, im Standardlauf 128 KiB.
  Unter `-race` etwa 15- bis 20-fach. Kein Befund.
- **I-08 Stale Mutationsskript.** 8 der 40 Proben von `scripts/review-w-p20-mutations.py` sind nach den Auflagen ungültig (M02, M05–M08, M18–M20; Muster kommen nicht mehr vor, u. a. die AEAD-Besitzer-/Scope-Proben M19/M20).
  Ersetzt durch `scripts/rereview-w-p20-mutations.py` (N25/N26/N27). Mutante N24 (Schlüssel bei jedem Aufruf neu laden) überlebt als äquivalent (nur Leistung, kein Test pinnt das Caching).

## Prüfungen (Auflagen 1 bis 7)

| Auflage | Ergebnis |
| --- | --- |
| (1) cfg2 / Version / Kontext | Rollback nur des Siegels scheitert; Version wechselt atomar mit Siegel; Überlauf/Manipulation → `state_unreadable`; Kontext über gültigen Bereich injektiv (I-02, I-03) |
| (2) Installationsschlüssel | CSPRNG 32 Byte, Rennen mit einem Gewinner, kein Klartext im Dump, Besitzersalz, Wörterbuchangriff scheitert; Verlust → verwaiste Daten statt Fehlzuordnung (L-05); Rotation braucht alten Schlüssel (I-01) |
| (3) OwnerResolver | gehalten (I-06) |
| (4) Geheimnisfaltung | Gespeicherter Wert bleibt unverändert; keine Kollision mit legitimen Namen; Lücken L-02/L-03; kein ReDoS (I-07) |
| (5) Quoten, Migration, F-13, F-16 | Differentialtest identisch (I-04); F-12 teilweise (L-04); **F-13 B-01 und L-01**; F-16: Opt-in im Repo unter der Besitzersperre, Mutation N21 getötet |
| (6) Vertrag/Kette/Wächter | SHA unverändert, alle `check-*-contract.py` und `check-docs.py` (24 Skripte) grün (Kette bis v24), Quotenmutationen N22/N23 getötet |
| (7) Läufe | siehe unten |

### Läufe (Stand `4af7d52` plus Reviewdateien)

- `gofmt -l`: leer. `go vet ./...`: sauber.
- `go test -race -count=1 ./...`: alle Pakete ok (Datenbankpaket 135 s); die neue Reviewsuite lief ebenfalls grün (`TestRereviewHolds_PathologicalInputsStayLinear` auf 128 KiB gekürzt, nachdem die
  1,5-MiB-Fassung unter `-race` und Last 140 s brauchte; `DBM_REVIEW_FULL=1` für 1 MiB).
- `python3 scripts/test-config-store.py` gegen eigene Wegwerf-MariaDB 12.3 mit TLS: grün, 12 echte Antworten gegen API 0.24 validiert, Container entfernt.
- `python3 scripts/review-w-p20-live.py`: 21 Live-Tests grün, 0 Fehler (8 Altproben plus 13 neue `TestRereviewLive*`), Container entfernt.
- Mutationen `scripts/rereview-w-p20-mutations.py` (nur eigene Container): 27 Proben, davon 26 getötet, 1 äquivalent.

| Probe | Ergebnis |
| --- | --- |
| N01 Version nicht im Kontext · N02 Update siegelt alte Version · N03 Insert siegelt Version 2 · N04 Version +2 | getötet |
| N05 Index-Schlüssel mit `REPLACE` · N06 Schlüssel 8 Byte | getötet |
| N07 Scope ohne Besitzersalz · N08 MAC ohne Schlüssel | getötet |
| N09 Look-alikes aus · N10 kombinierende Marken behalten · N11 Hangul-Füller behalten | getötet |
| N12 Zugangsdatenschlüsselregel aus · N13 `curl -u`-Regel aus · N14 `password=`-Regel aus | getötet |
| N15 Basic-Auth-Regel aus | überlebte zuerst (Testlücke), nach `TestRereviewHolds_BasicAuthAndCommandLineForms` getötet |
| N16 Resolver darf `single` · N17 Hüllen-Duplikatprüfung aus | getötet |
| N18 Byte-Quote verdrängt Recents zuerst · N19 Byte-Quote zählt nichts · N20 `Verify` übersprungen · N21 Opt-in nicht im Repo | getötet |
| N22 Besitzerquote erhöht · N23 Verlaufsquote erhöht (Wächter) | getötet |
| N24 Schlüssel nie gecached | überlebt (äquivalent, nur Leistung) |
| N25 Kontext ohne Besitzer · N26 Kontext ohne Scope · N27 verbotenes Literal aus | getötet |

Hinweis: Eine zunächst zu schmale Auswahl der Livetests ließ N25/N26 überleben; mit `TestConfigRowsAreEncryptedAndBoundToTheirOwner` sind beide getötet. Die Mutationsläufe laufen gegen den Arbeitsbaum und stellen
ihn per `git checkout` wieder her.

## Hinzugefügte Dateien

- `internal/configstore/rereview_attack_test.go`, `internal/httpapi/config_rereview_test.go`, `internal/serverstate/configrereview_live_test.go`: `Holds_`-Tests müssen grün bleiben, `Gap_`-Tests pinnen den
  heutigen Befund und sind bei der Korrektur umzudrehen; `TestRereviewHang_*` läuft nur mit `DBM_REVIEW_DEMO_HANG=1`, nach der Korrektur zu einem normalen Test machen.
- `scripts/rereview-w-p20-mutations.py` (24+3 neue Proben), `scripts/review-w-p20-live.py` (führt jetzt auch `TestRereviewLive*` aus).

## Empfohlene Reihenfolge

1. **B-01** (Abbruch beim Tokenfehler, Tiefenbremse vor dem Durchlauf, Regressionstest aktivieren) und **L-01** (Groß-/Kleinschreibung) zusammen.
2. L-03 (verschachtelte Schlüssel, `pw|auth|dsn`, redis-URL), L-02 (U+03F2-Eintrag korrigieren, Kapitälchen/Armenisch ergänzen), L-04 (Spalten und Sortierung in `Verify`), L-05/I-01 (Betriebshandbuch und Umsiegelung).
3. Härtung vor Veröffentlichung von `cfg2`: Längenpräfix im AEAD-Kontext (I-02); Memory-Repo atomar machen oder Suite-Test (I-04).
