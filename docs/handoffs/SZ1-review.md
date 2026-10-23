# SZ1: unabhängiges Review

Reviewer: unabhängig vom Autor. Geprüft: Branch `feat/sz1-server-state` (`62578ba`, Basis `main` `dd7b1d8`),
Review-Branch `review/sz1`. Datum 23.10.2026, macOS arm64. Kein Merge nach `main`.

## Urteil

**Freigabefähig unter Auflagen.** Kein Blocker, kein Sicherheitsbefund hoher Schwere. Vor Integration sind
F1 und F2 (kleine, lokale Korrekturen) zu beheben; F3 ist eine Entscheidung des Leads/Nutzers. Die übrigen
Befunde sind als Folgepunkte zu dokumentieren.

## Eigene Läufe (nicht aus dem Handoff übernommen)

| Prüfung | Ergebnis |
| --- | --- |
| `gofmt -l .`, `go vet ./...` | sauber |
| `go test -race -count=1 ./...` | alle Pakete ok (Integrationstests ohne Fixture übersprungen) |
| `scripts/test-server-state.py` | grün: 22 Go-Integrationstests mit `-race` gegen Wegwerf-MariaDB mit TLS; Containerphase: fremde CA Exit 1, zwei Replikate (genau eine Migration), Entfernen/Neustart, Ausfall 503/unhealthy und Erholung ohne Neustart, keine Secrets in Logs/inspect/history/compose config |
| `scripts/check-docs.py` | 386 Dateien, 0 Fehler |
| `check-{create,insert,mutation,structure,table}-contract.py` | alle bestanden |
| `scripts/test-deployment.py` | Compose und isolierter Swarm grün, Aufräumen bestätigt |
| `git diff main...HEAD -- go.mod go.sum` | leer (unverändert) |
| Eigene Experimente (Wegwerf-MariaDB, Test nicht committet) | siehe F1 und F2 |

Nicht ausgeführt: `test-engine.py` (Engine-Code unverändert), Swarm mit externer DB, Mehrknoten, Crash-Tests.

## Befunde

### F1 (mittel): Abgebrochene Health-Anfrage schaltet das Gate für alle `/api/`-Anfragen auf 503

Beleg: `Store.Ping` setzt `healthy.Store(err == nil)` auch bei Abbruch des übergebenen Kontexts. `/healthz`
ruft `Check(r.Context())` unauthentifiziert auf. Eigener Test: erster Ping `healthy: true`, Ping mit
abgebrochenem Kontext liefert `context canceled` und `healthy: false`. Bis zum nächsten Monitor-Tick
(bis 3 s) antwortet die gesamte API 503. Ein unauthentifizierter Client kann das wiederholt auslösen
(Verbindung zu `/healthz` aufbauen und schließen). Zusätzlich löst jeder `/healthz`-Aufruf eine echte DB-Abfrage aus.
Fix: `healthy` nur aktualisieren, wenn der Fehler kein `context.Canceled` ist und der übergebene Kontext
nicht selbst beendet wurde (nur echte Erreichbarkeitsfehler zählen); optional `/healthz` aus dem
gecachten Zustand mit begrenzter Aktualisierungsrate beantworten (ein Ping je Intervall, singleflight).
Test ergänzen (abgebrochener Kontext verändert `Healthy()` nicht).

### F2 (niedrig bis mittel): `LockTimeout` wird faktisch auf 10 s gekappt und meldet dann „nicht erreichbar“

Beleg: Pool mit `ReadTimeout` 10 s; `GET_LOCK(name, 30)` bei von anderer Verbindung gehaltener Sperre endet nach
10,01 s mit `ErrUnavailable` statt `ErrMigrationLocked` (eigener Lauf). Funktional gemildert, weil
`reportMigration` bei `ErrUnavailable` bis 2 Minuten wiederholt. Dasselbe `ReadTimeout` bricht künftig lange
DDL-Anweisungen (große Tabellen) nach 10 s ab und wird als `ErrMigrationFailed` gemeldet; die Sperre fällt
dann serverseitig mit der Verbindung weg, die Migration ist per Wiederholung heilbar, aber der Fehler irreführend.
Der Test `TestMigrationWaitsForLockAndTimesOut` nutzt nur ca. 1 s und deckt das nicht auf.
Fix: für den Migrationspool `ReadTimeout`/`WriteTimeout` auf mindestens `LockTimeout` plus Reserve bzw. großzügig
setzen (separate Pool-Konfiguration für Migration); Test mit Wartezeit über 10 s oder Timeout-Parameter.

### F3 (Entscheidung, mittel): Gate-Umfang im Bearer-Modus

Das Gate liefert bei DB-Ausfall für den gesamten `/api/`-Bereich 503, obwohl SZ1 keine Funktion hat, die den
Serverzustand liest, und obwohl `/healthz` den Container `unhealthy` markiert. Folge im Swarm: `swarm.yaml`
nutzt `restart_policy: on-failure`; ein dauerhaft `unhealthy` Task (Healthcheck 15 s, 3 Versuche) wird ersetzt, der
Neustart scheitert wegen `WaitReady` (30 s) mit Exit 1, danach Wiederholung alle 5 s: eine DB-Störung legt den
Dienst samt SQLite-Zugriff vollständig lahm und erzeugt eine Austauschschleife (aus der Konfiguration abgeleitet,
nicht im Swarm geprüft; in Compose startet `unless-stopped` nur nach Exit neu, nicht bei `unhealthy`).
Bewertung/Empfehlung: Das Gate muss bestehen, sobald eine Funktion tatsächlich Serverzustand liest (Konten,
Sessions, Freigaben) und dann pro Route, nicht global. Für SZ1 und den unveränderten Bearer-Modus ist ein
globales Gate nicht erforderlich. Empfohlen: (a) Gate nur auf Routen, die Serverzustand benötigen (derzeit keine),
(b) `/healthz` in zwei Teile trennen (Liveness für Docker/Swarm ohne DB-Abhängigkeit, Readiness
z. B. `/readyz` mit Zustandsspeicher für Load Balancer), damit Orchestratoren nicht bei DB-Ausfall Tasks
ersetzen. Wenn der Lead das globale fail closed als Sicherheitsvorgabe (ADR Frage 8) beibehält, dokumentieren
und Swarm-Healthcheck/Restart-Folge ausdrücklich als Betriebsrisiko ausweisen. Das Gate sitzt vor der
Authentifizierung: Unauthentifizierte sehen 503 statt 401 und erfahren so den DB-Zustand (gering, ebenso
`state_store` in `/healthz`).

Positiv: Ohne Store ist `/healthz` byte-identisch (`{"status":"ok"}`, Test vorhanden) und der Bearer-Pfad
unverändert; Gate liest nur ein `atomic.Bool` (race-frei, `-race` grün); Erholung ohne Neustart nachgewiesen.

### F4 (niedrig): Migrationsbenutzer-Passwort bleibt im Serverprozess

Mit `DBM_STATE_DB_MIGRATE_*` wird das DDL-Passwort gelesen und in `Config.MigrationPassword` für die gesamte
Prozesslaufzeit gehalten; das Secret liegt im Container. Das Handoff nennt dies ehrlich. Empfehlung: `state-migrate`
als Einmalschritt (Init-Container/Job) als dokumentierten Standard für Swarm; optional Passwort nach der Migration
aus der Config löschen und das Secret nur im Migrationsschritt einbinden. Die Variante ohne Migrationsbenutzer
migriert nie, korrekt.

### F5 (niedrig): Kleinere Beobachtungen

- `Config` hat exportierte Passwortfelder; `String()`/`GoString()` redigieren, `json.Marshal` oder `%+v` auf eine
  Struktur, die `Config` enthält, würde nicht über diese Methoden laufen (aktuell kein solcher Aufrufer).
  Felder `json:"-"` markieren oder unexportiert machen.
- `DBM_STATE_DB_CA_FILE` ergänzt die Systemwurzeln statt zu ersetzen (dokumentiert). Für eine private DB wäre
  Pinning auf die eigene CA strenger; als Option erwägen.
- Schlüssel: kein Entropie-/Nullschlüssel-Check (32 Nullbytes werden akzeptiert); Dateirechte der Secret-Dateien
  werden nicht geprüft.
- GET_LOCK-Timeout wird per `int(LockTimeout/Second)` gerundet; Werte unter 1 s werden zu 0.
- `Ping` bei Pool-Erschöpfung/Last kann in 2 s laufen und das Gate flackern lassen (Hysterese, z. B. zwei
  Fehlschläge in Folge, erwägen).

## Geprüft ohne Befund

- **Secrets:** nur Dateien (`readSecret`, 4 KiB-Limit), feste Fehlertexte ohne Host/User/Treibertext (`classify`),
  `NopLogger` für den Treiber, Containerlog/inspect/history/compose-config-Nachweis im Skript eigenständig
  gelaufen. Layerinhalt wurde (wie im Handoff) nicht geprüft; Secrets kommen nie zur Buildzeit.
- **TLS:** `tls.Config` mit `MinVersion` 1.2, `ServerName`, Wurzeln; `AllowFallbackToPlaintext=false`,
  `mysql.ErrNoTLS` wird als `ErrTLS` klassifiziert; Tests: unbekannte CA, falscher Host, Server ohne TLS. `disable`
  nur ausdrücklich, schließt CA-Datei aus.
- **AES-GCM:** 96-Bit-Zufallsnonce aus `crypto/rand` je Seal (Grenze etwa 2^32 Nachrichten je Schlüssel, für
  diesen Zweck unkritisch; bei Schlüsselrotation fortschreiben), Header (Version, Schlüsselkennung) und
  Kontext (Tabelle/Spalte/Zeile) als AAD, Schlüsselkennung als abgeleiteter Digest, einheitlicher `ErrDecrypt`,
  kein Panic bei `nil`-Cipher. Keyring-Rotation nur im Code, nicht konfigurierbar (im Handoff genannt).
- **SQL/Identifier:** Datenbankname per Whitelist-Regex, Verbindung über Treiberkonfiguration, keine SQL-Konkatenation
  von Identifiern; Migrations-SQL statisch; `MultiStatements=false`; Werte parametrisiert.
- **GET_LOCK:** dedizierte `sql.Conn`, Namen aus Hash des Datenbanknamens, Ergebnisprüfung (1), erneute Prüfung
  `IS_USED_LOCK()=CONNECTION_ID()` vor jeder Migration, Freigabe mit frischem Kontext, Verbindungsende gibt
  serverseitig frei (bei Absturz). Einschränkung siehe F2.
- **Prüfsummen:** SHA-256 über Statements, Abweichung und neuere DB-Version werden als `ErrSchemaMismatch`
  abgelehnt (Unit- und Integrationstest). Idempotente DDL; Crash zwischen DDL und Versionszeile heilt per
  Wiederholung (Argument schlüssig, kein Crash-Test).
- **Compose/Swarm-Zusatzdateien:** `docker compose config`/`docker stack config` im Skript gerendert; Syntax und
  Secrets realistisch; Pflichtvariable `DBM_STATE_DB_HOST` bricht mit klarer Meldung ab. Kein Swarm-Deployment mit DB.
- **Lizenz/Abhängigkeiten:** keine neue Abhängigkeit, `go.mod`/`go.sum` unverändert, `LIZENZEN.md` konsistent
  (Treiber MPL-2.0, `mariadb:12.3` nur Testwerkzeug).
- **Doku:** Handoff, BETRIEB und ADR benennen Grenzen ehrlich (kein HA/Multinode, Swarm nur gerendert, gate weit,
  Passwort im Prozess, nur MariaDB 12.3). Eine Ergänzung nötig: Die Doku nennt `unhealthy` ohne die
  Swarm-Austauschfolge klar als Betriebsrisiko (F3), und nennt nicht, dass `/healthz` unauthentifiziert eine
  DB-Abfrage auslöst (F1).

## Offene Punkte nach Review

F1 und F2 beheben und mit Tests absichern; F3 entscheiden und Doku anpassen; F4/F5 als Folgepunkte in
ARBEITSPLAN/Handoff führen. Danach kurzer Nachprüflauf (`go test -race`, `test-server-state.py`).

## Nachreview (Commit `5aaa1df` auf `feat/sz1-server-state`)

Urteil: **freigabefähig.** F1 bis F5 sind geschlossen, kein neuer Blocker. Zwei Hinweise niedriger Schwere (N1, N2).

Eigene Läufe auf dem Stand `5aaa1df` (in `review/sz1` gemergt): `gofmt` leer, `go vet` ok, `go test -race -count=1 ./...`
alle Pakete ok, `scripts/test-server-state.py` grün (26 PASS-Tests, 0 übersprungen, Containerphase inklusive Ausfall,
Erholung, Replikat ohne DDL-Zugang, Secret-Prüfung), `check-docs.py` 388 Dateien ohne Fehler, fünf Vertragswächter ok,
`test-deployment.py` (Compose und isolierter Swarm) grün, Wegwerfressourcen entfernt.

| Befund | Status | Beleg |
| --- | --- | --- |
| F1 | geschlossen | `Ping` ändert `healthy` nicht, wenn der Aufrufer-Kontext beendet ist; Test `TestCanceledCallerDoesNotChangeHealth` (Abbruch und abgelaufene Frist). `/healthz` fragt die DB nicht mehr, `/readyz` liest nur den Cache. |
| F2 | geschlossen | Migrationspool mit `LockTimeout` plus 10 min I/O-Timeout; `TestMigrationLockWaitBeyondRuntimeIOTimeout` (Wartezeit über 10 s, `ErrMigrationLocked`); `lockSeconds` rundet auf. Das Laufzeit-`ioTimeout` bleibt 10 s. |
| F3 | geschlossen | Globales Gate entfernt; `/healthz` ohne Store und mit Store identisch `{"status":"ok"}` (Test), Bearer-API bei Ausfall 200 (Test, Containerlauf), `/readyz` 200/503 nur aus Cache, `RequireState` fail closed auch ohne Store, Erholung getestet. Der Containerlauf zeigt Docker `healthy` bei DB-Ausfall, also keine Austauschschleife durch `unhealthy`. `Wrap` leitet `/readyz` unverändert an die API. |
| F4 | geschlossen (Doku und Beispiele) | `state-migrate` ist Standard, Start-Migration als Ausnahme gekennzeichnet; Containerlauf deckt Replikat ohne DDL-Zugang ab. Ein Swarm-Job ist ausdrücklich nicht bereitgestellt oder getestet. |
| F5 | behoben bzw. als Grenze dokumentiert | `json:"-"` mit Test, Nullschlüssel abgelehnt mit Test; CA-Ergänzung, Dateirechte, Entropie und Hysterese stehen als bekannte Grenzen in BETRIEB. |

Race-Freiheit: `Healthy()` ist ein `atomic.Bool`, `/readyz` und `RequireState` lesen nur diesen Wert; `-race` grün.
Informationsleck: `/readyz` und `/healthz` liefern feste Texte ohne Host, Benutzer oder Treiberfehler; sie sind
unauthentifiziert und zeigen den Speicherzustand, was BETRIEB jetzt benennt.

Doku ehrlich: ADR-0006 Frage 8 bleibt ausdrücklich offen für künftige Konten-Funktionen, die Entscheidung wird als
Koordinatorentscheidung nach dem Review ausgewiesen; Swarm bleibt "nur gerendert", kein Multinode-/HA-Anspruch.

Neue Hinweise:

- **N1 (niedrig):** ADR-0006 Abschnitt 5, Abnahmekriterium 3, nennt weiterhin "fail closed und `unhealthy`"
  (Zeile 195), während Handoff und BETRIEB dies durch `/readyz` 503 ersetzen. Im ADR einen Verweis auf die geänderte
  Umsetzung ergänzen, damit Kriterium und Nachweis nicht widersprechen.
- **N2 (niedrig, Betrieb):** Ein Neustart des Dienstes bei laufendem DB-Ausfall scheitert weiter an `WaitReady` (30 s,
  Exit 1) und wird von `restart_policy` wiederholt; der Bearer-Dienst ist dann bis zur Rückkehr der DB nicht verfügbar.
  Das Handoff nennt es, im Swarm nicht geprüft. Optional später: Start ohne DB im Bearer-Modus zulassen, falls gewünscht.
