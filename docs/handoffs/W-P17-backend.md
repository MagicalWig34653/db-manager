<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Handoff W-P17 Backend: lesende Serververwaltung

Stand: 26.10.2026. Branch `feat/w-p17-backend`, HEAD `d55b567` mit integriertem
main `c331c64`; WIP `b5eeed6` wird durch die folgenden uncommitteten Änderungen
auf die ausdrücklich beauftragte lesende Stufe begrenzt. Keine Git-Schreibbefehle;
Commit, unabhängiger Review und Integration durch den Lead. API-Nummer endgültig
**0.15**, Vertragsfreeze noch offen. Keine Veröffentlichung oder Vollparität.

## Ergebnis, Dateibesitz und Abnahme

- `internal/core/serveradmin.go`: ausschließlich Lesetypen, optionale
  `ServerAdminEngine.ServerAdminRead`-Schnittstelle und ehrliche Treiberfähigkeiten.
- `internal/database/serveradmin.go`: Status, Variablenfilter mit Maskierung,
  Prozessliste mit Paging und Zeitlimit; keine Schreibmethoden, kein HMAC-Schlüssel,
  keine SQL-/Hostspalten aus Prozesslisten. MySQL/MariaDB über SHOW GLOBAL STATUS,
  SHOW SESSION/GLOBAL VARIABLES und PROCESSLIST; PostgreSQL über pg_stat_database,
  pg_settings (SESSION) und pg_stat_activity; SQLite nur Version ohne Pfade.
  Scanbudget zusätzlich auf insgesamt 8 MiB begrenzt, Zahlenformate validiert.
- `internal/httpapi/serveradmin.go`: genau vier authentifizierte Routen unter
  `/api/v15/`: capabilities, status, variables, processes. Schreib-Routen und
  v17-Registrierung entfernt. Featureanzahl bleibt 13; Registrierungsdateien unverändert.
- `internal/database/serveradmin_test.go`, `serveradmin_live_test.go`,
  `internal/httpapi/serveradmin_test.go`: SQLite, Grenzen/Timeout beim Warten auf
  Engine-Slot, Maskierung, Zahlvalidierung, deterministische Metadatenabfragen,
  Filter/Paging/Sortierung, Zeilen-/Zell-/Gesamtbytegrenzen, sichere Fehler,
  Authentifizierung, no-store und nicht vorhandene Schreib-/v17-Routen.
- [API0.15-Vertrag](../../contracts/api-v0.15.0.json) additiv über v12;
  `contracts/api-v0.17.0.json` und Schreib-Fixtures entfernt.
  [Verhalten](../../contracts/SERVER-ADMIN-API.md),
  [Fixtures und Herkunft](../../contracts/fixtures/server-admin/README.md),
  [ADR](../decisions/ADR-W-P17-lesende-serververwaltung.md).
- `scripts/check-server-admin-contract.py`: pinnt v1..v12, prüft Additivität gegen
  v12, lokale Referenzen, strikte neue Schemata, ausschließlich Lesefähigkeiten und
  Fixtures. v13/v14/v16+ sind optionale Nachbarpakete, keine Dateivoraussetzungen.
- `scripts/test-server-admin.py`: `scripts/dbfix.py`, drei eigene kurzlebige
  Netzwerkfixtures nacheinander, echte Antwortdumps werden gegen v15 validiert.
  `.github/workflows/server-admin.yml`: Version/Titel auf lesende API0.15 angepasst.
- Keine Abhängigkeiten oder Lizenzinventare geändert, eigener Code AGPL-3.0-only.
  Zentrale Status-/Plan-/Vertragsindexdateien, Web-Lock und Assets unverändert.

Abnahmekriterien dieses Pakets: lesende API0.15 ohne SET/KILL-Implementierung,
Maskierung und sichere Fehler, begrenzte Listen/Abfragen, echte lokale SQLiteprobe,
Livetests für die Netzwerkengines geschrieben, erreichbare Prüfungen ausgeführt und
übrige Gates ausdrücklich offen. Vollständige Netzwerkabnahme ist hier nicht belegt.

## Stufe 2 (nicht Teil dieses Pakets)

Variablen setzen (SESSION/GLOBAL), KILL QUERY/CONNECTION bzw.
pg_cancel_backend/pg_terminate_backend, Plan-/Ausführungsschnittstellen,
Bestätigungs-/Hash-/Identitätstoken, Rücklesen nach SET und Schreib-Livefälle sind
aus diesem Reviewkandidaten entfernt. Der historische WIP bleibt in Git lesbar,
ist aber kein aktueller Implementierungsstand. FLUSH bleibt ebenfalls außerhalb.
Eine spätere Stufe benötigt einen neuen Auftrag, Vertrag und unabhängige Prüfung
zu Rechten, Zielbindung, Selbstschutz, Rennen und unbekanntem Ausführungsausgang.

PMA-ADMIN-002..005/PMA-NEU-ADMIN-001 sind nur teilweise abgedeckt; keine vollständige
Matrixerfüllung. Kein Web-, Monitor-, Advisor- oder Replikationsumfang.

## Tatsächliche Prüfungen

Lokales macOS arm64, ohne Netzwerkdownloads:
`GOCACHE=/private/tmp/dbmanager-w-p17-gocache GOPROXY=off GOSUMDB=off` vor allen
Go-Befehlen bzw. dem Live-Skript. Standardmodulcache nur gelesen. Die erste
Zwischenprüfung offenbarte einen Schema-Namensfehler und eine falsche Wächterbasis;
beide vor den erfolgreichen Wächterläufen korrigiert.

| Befehl | Tatsächlicher Ausgang |
| --- | --- |
| `gofmt -l .` | Keine Ausgabe. |
| `go vet ./...` | Bestanden. |
| `go test -race -count=1 ./internal/database ./internal/httpapi -run 'TestSQLiteServerAdmin\|TestServerAdmin\|TestLiveServerAdmin' -v` | Lokale Zieltests bestanden; mysql/maria/postgres ausdrücklich SKIP mangels Fixturevariablen. |
| `go test -race -count=1 ./...` | Läuft noch; Endstand wird nachgetragen. |
| Alle zwölf `scripts/check-*-contract.py` einzeln über Python ausgeführt | Bestanden, einschließlich eingefrorener v1..v12 und additiver v15. |
| Python-`ast.parse` für beide W-P17-Skripte | Bestanden. |
| `python3 -B scripts/test-server-admin.py` | Noch ausstehend. |
| `python3 -B scripts/check-docs.py` | Abschließender Lauf ausstehend. |
| `git diff --check` | Bestanden; nur lesender Git-Befehl. |

## Geschriebene Livefälle und offene Prüfungen

`TestLiveServerAdmin/{mysql,maria,postgres}` prüft Status/Messzeitpunkt,
MySQL-Com_select-Zuwachs durch synthetische SELECT-Last (Metrik explizit gefiltert),
Variablenfilter/Sortierung/Seiten ohne Überschneidung, sensible Werte, GLOBAL-Lesen
bei MySQL/MariaDB und GLOBAL-Ablehnung bei PostgreSQL, sichtbare Kontrollverbindung,
Prozesspaging/Filter, eingeschränktes Konto und falsches Passwort ohne Echo.
CREATE/GRANT/DROP dienen ausschließlich der Einrichtung eines Wegwerfkontos im
Fixturecontainer. Es gibt keine SET-/KILL-Aktionen oder entsprechende Tests.

Offen: tatsächliche Livefälle auf MySQL 8.4, MariaDB 12.3, PostgreSQL 18.6 samt
TLS-/Rechtesichtbarkeit und Schemaprüfung echter Antworten, CI-Ausführung, Linux-
Abnahme und unabhängiger Review. Keine Compose-/Swarm-/Browser-/Webbuildprüfungen
für dieses begrenzte Backendpaket durchgeführt. Hostweite Prozessansicht mittels
`ps` ist in der Sandbox verweigert. Keine eigenen Container angelegt.

## Fortsetzung

Unintegrierte Arbeit sind ausschließlich die beschriebenen Dateien im zugewiesenen
Worktree. Kein Index/Ref/Commit verändert. Laufender Job: vollständige Go-Racesuite;
Endstand wird vor Abschluss erfasst. Nächster Schritt: Lead prüft Dateidiff, erstellt
den Commit und veranlasst unabhängigen Review sowie `scripts/test-server-admin.py`
auf einem Host mit Dockerzugriff; anschließend Integrationsentscheidung.
