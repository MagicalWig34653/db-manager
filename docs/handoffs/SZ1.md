# SZ1: Serverzustand-Fundament (externe MariaDB)

Branch `feat/sz1-server-state` (Worktree `.worktrees/sz1`), Basis `main` `dd7b1d8`.
Grundlage: [ADR-0006](../decisions/ADR-0006-serverzustand.md), Option (c) vom Nutzer am
23.10.2026 ausdrücklich akzeptiert. **Nach unabhängigem Review und Nachreview ([SZ1-review](SZ1-review.md)) am 23.10.2026 als
`90044dc`/`4853761` nach `main` integriert.** Der Branchstand davor ist unten historisch
beschrieben. STATUS/ARBEITSPLAN führen SZ1 separat.

## Ergebnis (Umfang exakt SZ1, kein Passkey-/Team-Feature)

- `internal/serverstate`: Konfiguration (`DBM_STATE_*`, Secrets nur aus Dateien, redigierte
  `String()`), eigener Pool mit TLS und Zertifikats-/Hostnamenprüfung ohne Klartextfallback
  (`DBM_STATE_DB_TLS=disable` nur ausdrücklich für Entwicklung), Migrationsrunner
  (`schema_migrations` mit Prüfsumme, `GET_LOCK` auf dedizierter Verbindung, Plan-Modus),
  AES-256-GCM-Feldverschlüsselung mit Schlüsselkennung und Kontextbindung, Health/Monitor.
  Erste Migration: Schemaversion und Testtabelle `state_key_probe`.
- `internal/httpapi`: optionales `WithStateStore`. `/healthz` bleibt Liveness (`{"status":"ok"}`),
  `/readyz` meldet den Speicher, `RequireState` sperrt Routen fail closed (derzeit keine Route).
- `cmd/db-manager`: Start mit Serverzustand (Migration nur mit Migrationsbenutzer, dann
  Verbindung und Schemaprüfung, sonst Abbruch); Befehl `db-manager state-migrate [--plan]`.
- `compose.state.yaml`, `compose.state-migrate.yaml`, `deploy/swarm.state.yaml`,
  `deploy/swarm.state-migrate.yaml` als optionale Zusatzdateien; `compose.yaml` und
  `deploy/swarm.yaml` nur um Kommentare ergänzt. Doku: BETRIEB (neuer Abschnitt),
  ARCHITEKTUR, LIZENZEN, ADR-0006 (Status, Entscheidung).
- Keine neue Abhängigkeit: der bereits inventarisierte `go-sql-driver/mysql` (MPL-2.0)
  wird weiterverwendet; `go.mod`/`go.sum` unverändert.

## Prüfstand (23.10.2026, lokal macOS arm64, Docker 29.5.2, Go 1.27.1)

| Prüfung | Ergebnis |
| --- | --- |
| `gofmt -l .` | leer |
| `go vet ./...` | ok |
| `go test -race -count=1 ./...` | alle Pakete ok (ohne Fixture sind die Integrationstests übersprungen) |
| `python3 scripts/test-server-state.py` | grün: Go-Integrationstests mit `-race` gegen Wegwerf-MariaDB 12.3 mit TLS (24 Tests, keine übersprungen) und Containerphase |
| `python3 scripts/check-docs.py` | ohne Fehler |
| `check-{create,insert,mutation,structure,table}-contract.py` | alle bestanden |
| `python3 scripts/test-deployment.py` | Compose und isolierter Swarm (DinD) grün, Standardbetrieb unverändert |

Abnahmekriterien ADR-0006 Abschnitt 5 und Nachweis (Punkt 3 nach F3: Ausfall führt zu `/readyz` 503, Liveness und Bearer-API laufen weiter, Erholung ohne Neustart; der ADR-Wortlaut „`unhealthy`“ ist durch die Koordinatorentscheidung ersetzt):

1. Migration einmalig: Go-Test mit 8 parallelen Migratoren (genau einer wendet an) und
   Containerlauf mit zwei gleichzeitig gestarteten Replikaten (Protokoll im Skriptlog:
   ein Replikat „Migration 1 angewendet“, das andere „Schema aktuell“; eine Zeile in
   `schema_migrations`). Lock-Warten und Timeout (`ErrMigrationLocked`) sind getestet.
2. TLS und Rechte: unbekannte CA, falscher Hostname und Downgrade auf eine DB ohne TLS
   werden abgelehnt (Go-Test, Container mit fremder CA endet mit Exit 1 und TLS-Meldung).
   Laufzeitbenutzer scheitert an CREATE/ALTER/DROP/TRUNCATE/INDEX, anderem Schema,
   `mysql.user`, `CREATE DATABASE`; ohne Migrationsbenutzer kann er nicht migrieren.
3. Neustart/Entfernen eines Replikats ohne Zustandsverlust, Ausfall (nach F3 geändert): `/readyz` 503, `/healthz` und Bearer-API
   unverändert, Docker bleibt `healthy`; Wiederkehr der DB: Erholung ohne Neustart (RestartCount und
   StartedAt unverändert). Zusätzlich Go-Test mit Proxy-Ausfall derselben `Store`-Instanz.
4. Verschlüsselung: Rohzeilen enthalten keinen Klartext; falscher Schlüssel `ErrUnknownKey`,
   fehlender `ErrNoKey`, in andere Zeile verschobener Wert `ErrDecrypt`; kein Panic.
5. Secrets: Prüfskript sucht Passwörter, Serverschlüssel, Token und Root-Passwort in
   Containerlogs, `docker inspect`, `docker history`/Imagemetadaten und `docker compose config`.
6. Ohne Konfiguration unverändert: Unit-Test auf `/healthz`-Bytes und Bearer-Pfad;
   `test-deployment.py` grün.
7. Dieses Handoff; Review steht aus; Swarm siehe offene Punkte.

## Nachbesserung nach unabhängigem Review (`review/sz1` `1619ccc`, freigabefähig unter Auflagen)

- F1: `Store.Ping` ändert den Zustand nicht, wenn der Aufrufer abbricht oder dessen Frist abläuft
  (Test). Keine DB-Abfrage mehr pro HTTP-Aufruf; `/readyz` liest nur den Cache der Hintergrundprüfung.
- F2: Migrationspool mit eigenem I/O-Timeout (Sperrwartezeit plus 10 Minuten); Test mit
  `LockTimeout` über 10 s liefert `ErrMigrationLocked` nach voller Wartezeit. Sub-Sekunden-Wartezeit wird aufgerundet.
- F3 (Entscheidung Koordinator): globales 503-Gate entfernt. `/healthz` ist reine Liveness (Bytes
  unverändert, Healthchecks unverändert), neue `/readyz` meldet den Speicher, `API.RequireState` ist
  der routenweise Mechanismus (getestet, derzeit von keiner Route benutzt). Bearer-Modus läuft bei
  DB-Ausfall weiter; keine Austauschschleife durch Healthcheck. ADR-0006 und BETRIEB nachgeführt.
- F4: `state-migrate` ist dokumentierter Standard (Einmalbefehl), Start-Migration als Ausnahme gekennzeichnet;
  Containerlauf prüft `--plan`, Anwenden, Wiederholung und ein Replikat ohne DDL-Zugangsdaten.
- F5 behoben: Passwort-/Schlüsselfelder `json:"-"`, Nullschlüssel abgelehnt. Als bekannte Grenzen
  dokumentiert (BETRIEB): CA ergänzt statt Pinning, keine Prüfung der Secret-Dateirechte, keine
  Entropieprüfung, keine Hysterese der Readiness.

## Offene Punkte und Grenzen (ehrlich)

- **Unabhängiges Review fehlt.** Kein Merge nach `main` bis dahin.
- Swarm: `swarm.state*.yaml` sind nur als gerendertes Stack-Dokument geprüft (`docker stack
  config`), **kein** Swarm-Deployment mit DB, kein Multinode-/HA-Nachweis. `swarm.yaml`
  bleibt ein Replikat mit lokalem SQLite-Volume. Die Healthchecks hängen nicht mehr von der DB ab (keine Austauschschleife durch `unhealthy`); Ein Neustart bei DB-Ausfall scheitert weiter an `WaitReady` (30 s, Exit 1) und wird von `restart_policy` wiederholt (nicht im Swarm geprüft).
- Zwei Replikate wurden als Compose-Dienst plus `docker run` im selben Netz geprüft, nicht
  als Swarm-Service mit zwei Replikaten.
- Kein MariaDB-Dienst/Compose-Profil mit DB im Stack (ADR nennt es als optional); nur
  externer Anschluss. Die Einrichtung von DB, Benutzern und Zertifikaten ist Betreiberaufgabe.
- Mit Start-Migration (Ausnahme) liegt das DDL-Passwort im Serverprozess; Standard ist `state-migrate` als Einmalbefehl. Ein Swarm-Job dafür ist nicht deployt.
- Imageebenen wurden nur über `docker history`/`inspect` auf Secrets geprüft, nicht Layer-Inhalt;
  Secrets werden zur Buildzeit nie eingebracht.
- Schema neuer als Programm wird abgelehnt (Rollback/Rolling-Update mit älteren Replikaten
  nicht untersucht). Migrationen sind nicht transaktional (MariaDB-DDL) und daher idempotent
  geschrieben; Absturz zwischen DDL und Versionszeile ist per Wiederholung heilbar, aber
  nicht per Crash-Test belegt.
- MariaDB getestet nur mit 12.3. Unterstützte Versionen, MySQL-Kompatibilität, Galera,
  Isolationsstufe sind offene Frage 7. `REQUIRE SSL`-Benutzer sind Empfehlung, nicht erzwungen.
- Serverschlüssel: nur ein aktiver Schlüssel konfigurierbar (Keyring mit Altschlüsseln ist im
  Code, aber nicht über Konfiguration erreichbar); Rotation und Backup/Restore (Frage 5) offen.
- Offene Fragen 1 bis 11 aus ADR-0006 bleiben offen (Passkeys, Sessions, Mandanten, Audit usw.).
- `test-engine.py` (MySQL/PostgreSQL-Live-Engine-Tests) wurde nicht erneut ausgeführt; der
  Engine-Code ist unverändert.

## Nächster Schritt

Unabhängiges Review von Branch `feat/sz1-server-state` (Schwerpunkte: Gate-Umfang,
Migrationslock, Secret-Behandlung, Fehlerklassifikation), danach Lead-Entscheidung zur
Integration und Pflege von STATUS/ARBEITSPLAN. Laufende Jobs: keine; alle Wegwerfressourcen
des Prüfskripts sind entfernt.
