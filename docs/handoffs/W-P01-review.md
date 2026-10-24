# Review W-P01: Backend-Registry und Fixture-Harness

Reviewer: unabhängig (nicht Autor). Geprüft: `feat/w-p01-registry` (8703af9) gegen `main` (bd7d21c), 24.10.2026.
Review-Branch: `review/w-p01` (nur Tests und dieses Dokument, Autorenbranch unverändert).

## Urteil

Freigabe (accept). Keine blockierenden Befunde. Zwei niedrige Hinweise und drei Beobachtungen für Folgepakete unten.

## Ausgeführt

| Prüfung | Ergebnis |
| --- | --- |
| `gofmt -l .`, `go vet ./...` | ohne Ausgabe |
| `go test -race -count=1 ./...` | alle Pakete ok (inkl. ergänzter Tests) |
| `check-{table,mutation,insert,structure,create}-contract.py` | alle fünf bestanden |
| `scripts/check-docs.py` | 421 files, 0 errors |
| `scripts/test-dbfix.py` | MySQL/MariaDB/PostgreSQL gestartet, `TestLiveCreate` PASS, danach keine Container mit Label `dbmanager.task=w-p01`, Temp-Verzeichnis weg |
| `go test -fuzz FuzzQuoteIdentifier` 20 s | 9,2 Mio. Ausführungen, kein Fund |

Last: Host-Load zeitweise > 600, Docker-Suite erst nach Abklingen (Load 142) und einzeln gestartet.

## Befunde

### (1) Routenverhalten: kein Befund
Beleg: Alte (main) und neue Fassung als Testbinary gebaut, identische Sondierung `TestProbeDump` gegen beide:
6175 Anfragen (23 Pfade inkl. /healthz, /readyz, v1 bis v6, unbekannte, abschließender Slash; Methoden GET/POST/PUT/DELETE/HEAD/OPTIONS;
Auth leer/falsch/richtig/ohne Bearer; Content-Type leer/json/json+charset/text; Bodys leer/kaputt/unbekanntes Feld/zwei Objekte/> 1 MiB).
Vergleich von Status, sämtlichen Headern und Antwortbytes nach Sortierung: identisch, 24 Antworten mit 413 in beiden, keine Panik.
(Rohausgabe unsortiert unterscheidet sich nur durch Map-Iterationsreihenfolge der Testbodys.) `decode` nutzt weiterhin 1 MiB; Registrierungsreihenfolge
und Pfade identisch; `max_request_bytes` in den Capability-Antworten unverändert (Wächter bestanden).

### (2) dbplan
- Digest korrekt: Domain `dbmanager/plan/v1`, Treiber, Anzahl und jede normalisierte Anweisung uvarint-längenpräfixiert. Zusatztest
  `TestDigestCollisionResistance` (Grenzverschiebung, 127/128-Byte-Längen, NUL, 0x01-Präfixe, Treibersuffix) ohne Kollision. Kein Befund.
- Check: Hex-Validierung (nur Kleinbuchstaben, 64) vor Konstantzeitvergleich; leere, verlängerte, verkürzte, nicht-ASCII-, Leerzeichen-Digests abgelehnt;
  Aktion und Zieltext exakt (Groß-/Kleinschreibung, Whitespace, NUL); bei destruktiv und leerem `wantTarget` immer `ErrConfirmTarget`.
  Test `TestCheckCannotBeBypassed` ergänzt. Kein Befund im Paket.
- QuoteIdentifier: Escape, NUL, leer, Unicode, 200 KB Name, Semikolon-Namen getestet; Fuzz bestätigt Gleichheit mit Referenzkopie und dass das Ergebnis genau ein
  geschlossener Bezeichner ist. `internal/database/quote_parity_test.go` prüft direkt gegen das unexportierte `database.quoteIdentifier` (4 Treiber, 11 Namen): identisch.

Niedrig/Hinweis:
- L1 (niedrig): Das Backtick-Quoting hängt an der Treiberkennung `"mysql"` (MariaDB nutzt dort ebenfalls `mysql`). Ein künftiger Treiberwert `mariadb` bekäme stillschweigend Doppelanführungszeichen. Fix: im Folgepaket ADR-0007 (nativer Adapter) Treibernamen beim Einführen zentral abgleichen; der Edge-Test dokumentiert das Verhalten.
- L2 (niedrig, Konvention): `Check` ist nur sicher, wenn `wantAction`/`wantTarget` aus Katalog/Route stammen und nie aus dem Request. Der Handoff nennt „Zielname" nicht ausdrücklich als serverseitig. Fix: Satz im Handoff ergänzen (hier nicht geändert, da Autorendokument).
- B1 (Beobachtung): Der Digest deckt nur Treiber und Anweisungen ab, nicht `warnings`/`affected_objects`; für die Bestätigungssicherheit richtig, aber Folgepakete dürfen keine ausführungsrelevante Information nur in Warnungen führen.

### (3) Routenklassen
Limit wirkt über Context vor `MaxBytesReader`; unbekannte oder nicht positive Klasse fällt auf 1 MiB zurück; Routen ohne Wrapper bleiben 1 MiB; `ClassUpload` ist derzeit 1 MiB und ohne Verbraucher, also kein Bypass. Hinweis B2: `classBodyLimits` ist eine veränderliche Paketvariable ohne Sperre (nur der Test schreibt); ein Importpaket muss den Wert statisch ändern, nicht zur Laufzeit.

### (4) scripts/dbfix.py
- Aufräumen: nur per exaktem Namen `dbmanager-<paket>-<hex>` der selbst angelegten Container (`docker rm -f -v <name>`), Registrierung vor `docker create`; keine Netze, Volumes, Images oder Filter-Massenlöschung. Fehlerpfad und KeyboardInterrupt (`BaseException`) räumen auf. Kein Fremdbezug.
- TLS: eigene Wegwerf-CA (1 Tag), SAN localhost, Server-Key 0600, Ports an 127.0.0.1 gebunden, MySQL-Ready mit `--ssl-mode=REQUIRED`. Korrekt.
- Secrets: nur das öffentliche Fixture-Passwort (Konstante, nicht produktiv); nicht geloggt. Es steht in `docker exec`-Argumenten (lokal sichtbar), akzeptabel für Wegwerfsysteme.
- B3 (Beobachtung, niedrig): SIGTERM (nicht SIGINT) beendet das Skript ohne `__exit__`; Container blieben dann bis zum manuellen Entfernen über Label `dbmanager.fixture=<präfix>`. Optionaler Fix: Signalhandler, der `cleanup()` aufruft.

### (5) Dokumentation
Handoff ehrlich (offene Punkte: SQLite nur als Verzeichnis, `ClassUpload` unbenutzt, Review ausstehend). Konventionen (eigene Datei je Feature, eine Zeile am Ende von `featureList`, Env-Variablen der Fixtures) sind für Folgepakete brauchbar. Die dort genannte Prüfzahl 419 Dateien bei check-docs ist durch spätere Ergänzungen überholt (jetzt 421 inkl. dieses Dokuments); unkritisch.

## Ergänzte Tests (Review-Branch)
- `internal/dbplan/review_test.go`: `FuzzQuoteIdentifier`, `TestQuoteIdentifierEdgeTable`, `TestDigestCollisionResistance`, `TestCheckCannotBeBypassed`.
- `internal/database/quote_parity_test.go`: Parität `dbplan.QuoteIdentifier` gegen `database.quoteIdentifier`.
