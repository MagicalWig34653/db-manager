<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Handoff W-P17 Backend: lesende Serververwaltung

Stand: 26.10.2026. Branch `feat/w-p17-backend`, main `3a7e1f1` (Wave 3, API 0.13-0.16) integriert; WIP `b5eeed6` wird durch die folgenden uncommitteten Änderungen
auf die ausdrücklich beauftragte lesende Stufe begrenzt. Commit und Push durch Claude-Lauf;
unabhängiger Review und Integration durch den Lead. API-Nummer endgültig
**0.17**, Vertragsfreeze durch Lead. Keine Veröffentlichung oder Vollparität.

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
  `/api/v17/`: capabilities, status, variables, processes. Schreib-Routen und
  v17-Registrierung entfernt. Feature-Liste nach Merge mit main: 17 (registerServerAdmin zuletzt).
- `internal/database/serveradmin_test.go`, `serveradmin_live_test.go`,
  `internal/httpapi/serveradmin_test.go`: SQLite, Grenzen/Timeout beim Warten auf
  Engine-Slot, Maskierung, Zahlvalidierung, deterministische Metadatenabfragen,
  Filter/Paging/Sortierung, Zeilen-/Zell-/Gesamtbytegrenzen, sichere Fehler,
  Authentifizierung, no-store und nicht vorhandene Schreib-/v17-Routen.
- [API0.17-Vertrag](../../contracts/api-v0.17.0.json) additiv über v16 (Basis v16; frühere v0.15-Nummer kollidierte mit Accounts);
  `contracts/api-v0.17.0.json` und Schreib-Fixtures entfernt.
  [Verhalten](../../contracts/SERVER-ADMIN-API.md),
  [Fixtures und Herkunft](../../contracts/fixtures/server-admin/README.md),
  [ADR](../decisions/ADR-W-P17-lesende-serververwaltung.md).
- `scripts/check-server-admin-contract.py`: pinnt v1..v16 (verkettet Programs-Wächter), prüft Additivität gegen
  v16, lokale Referenzen, strikte neue Schemata, ausschließlich Lesefähigkeiten und
  Fixtures. v13/v14/v16+ sind optionale Nachbarpakete, keine Dateivoraussetzungen.
- `scripts/test-server-admin.py`: `scripts/dbfix.py`, drei eigene kurzlebige
  Netzwerkfixtures nacheinander, echte Antwortdumps werden gegen v17 validiert.
  `.github/workflows/server-admin.yml`: Version/Titel auf lesende API0.17 angepasst.
- Keine Abhängigkeiten oder Lizenzinventare geändert, eigener Code AGPL-3.0-only.
  Zentrale Status-/Plan-/Vertragsindexdateien, Web-Lock und Assets unverändert.

Abnahmekriterien dieses Pakets: lesende API0.17 ohne SET/KILL-Implementierung,
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

macOS arm64, Docker; Fixtures nur eigene Wegwerf-Container über `scripts/dbfix.py`
(MySQL 8.4, MariaDB 12.3, PostgreSQL 18.6, nacheinander, danach entfernt).

| Befehl | Ausgang |
| --- | --- |
| `git merge main` (3a7e1f1) | Konflikte in features.go/features_test.go (beide Seiten behalten, Zähler 17) und api-v0.15.0.json (main-Version übernommen, eigener Vertrag nach v0.17.0 verschoben, auf v16 rebased). |
| `gofmt -l .`, `go vet ./...` | sauber / bestanden |
| `go test -race -count=1 ./...` | alle Pakete bestanden |
| alle `scripts/check-*-contract.py`, `check-docs.py` | bestanden (Server-Admin-Wächter pinnt v1..v16) |
| `python3 -B scripts/test-server-admin.py` | bestanden: TestLiveServerAdmin mysql/maria/postgres ohne SKIP; 12/12/11 echte Antworten gegen API 0.17 validiert; SQLite über Unit-Tests mit echter Datei |

Sicherheitsbewertung: Prozessliste liefert weder SQL-Text noch Host (nur ID, Benutzer,
Datenbank, Sekunden, self); Variablen/Status nur für bekannte numerische Werte offen,
alles andere maskiert; Fehler laufen durch `safe()`, Passwort-Echo-Test vorhanden.
Schreibpfade (SET/KILL/FLUSH) sind nicht implementiert, Routen liefern 404.

## Offene Punkte

- Unabhängiger Review und Vertragsfreeze durch den Lead; Zentraldateien (STATUS/Plan) unverändert.
- Keine Web-Oberfläche (W-P17 Web), kein Monitor/Advisor/Replikation, keine Engines-/Zeichensatzlisten
  als eigene Routen (nur Status, Variablen, Prozesse).
- `information_schema.PROCESSLIST` ist in MySQL 8.4 als veraltet markiert, funktioniert aber.
