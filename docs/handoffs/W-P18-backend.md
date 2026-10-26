# Handoff W-P18 Backend: Datenbankbenutzer und Rechte

Stand: 25.10.2026. Backend implementiert, lokale Prüfungen unten; echte
Netzwerkdatenbanken und CI bleiben ungeprüft. Keine Integrationsfreigabe daraus.
Worktree `.worktrees/w-p18-backend`, Branch `feat/w-p18-backend`, HEAD
`cd02988` (main `c331c64` eingemergt, WIP `8a350f1` enthalten).
Die Fortsetzung hat ausschließlich Dateien geschrieben, keine Git-Schreibbefehle,
keinen Commit, keine Integration und keine Veröffentlichung ausgeführt.
Der Lead übernimmt Commit und Integration. Zentrale Status-/Planungsdateien,
Vertragsindex und Web-Lock sind unverändert.

## Vertrag und Dateibesitz

API **0.15.0**, endgültige Paketnummer, 28 authentifizierte Routen unter
`/api/v15/`. [JSON-Vertrag](../../contracts/api-v0.15.0.json) additiv über v12;
v1 bis v12 bleiben byteidentisch. V13 bis v15 werden nicht vorausgesetzt oder
erfunden. Der bisherige Paketentwurf `api-v0.18.0.json` ist durch v15 ersetzt.
SHA-256 des v15-Vertrags:
`40d1631ca5e2e38c430cc2230dc5d91a1034e24a7e2d9304ed918d71e68e3752`.

Paketdateien einschließlich des vorhandenen WIP:

- `internal/core/accounts.go`: Datentypen, Treiberfähigkeiten, optionale Engines.
- `internal/database/accounts.go`, `accounts_read.go`, `accounts_plan.go`:
  Sitzungsvorbereitung, Katalogauflösung, Lesen, Planbau und bestätigte Ausführung.
- `internal/httpapi/accounts.go`: Registrierung, Authentifizierung, strikte
  aktionsbezogene Eingaben und sichere Fehler. `features.go` registriert das Paket;
  `features_test.go` berücksichtigt insgesamt 13 Features nach dem Main-Merge.
- `internal/database/accounts_test.go`, `accounts_safety_test.go`,
  `accounts_live_test.go`, `internal/httpapi/accounts_test.go`: lokale Regressionen
  und dbfix-gestützte Live-Abnahmefälle.
- `contracts/api-v0.15.0.json`, [Verhalten](../../contracts/ACCOUNTS-API.md),
  `contracts/fixtures/accounts/`: 16 ausdrücklich synthetische Platzhalterfixtures
  mit README; keine angeblichen Live-Aufzeichnungen.
- `scripts/check-accounts-contract.py`: pinnt v1 bis v12, prüft Additivität über
  v12, exakte Routen/Methoden/Auth, lokale Referenzen, strikte Schemas und Fixtures.
  `scripts/test-accounts.py`: eigene dbfix-Fixtures sequenziell für MySQL 8.4,
  MariaDB 12.3 und PostgreSQL 18.6; validiert temporäre echte Antworten gegen v15.
- `.github/workflows/accounts.yml`: eigener CI-Job für Race, Vet, Wächter und Live.
- [ADR W-P18](../decisions/ADR-W-P18-kontoplaene.md): Passwortplatzhalter,
  eindeutige Kontenkennung, Zweitbestätigung und Grenzen der Kontenvorlage.

Keine neuen Abhängigkeiten oder Datenmigrationen. Eigener Code mit SPDX
AGPL-3.0-only, keine Attribution-Zeilen. Dokumentation Deutsch, Code/Tests Englisch.

## Verhalten und Sicherheitsgrenzen

Datenbankkonten, keine Produktkonten (E2); bestätigte Administration nach E9.
Lesend: Konten, direkte globale/Datenbank-/Tabellen-/Spalten-/Routinenrechte,
direkte Rollen, Ressourcenlimits, primäres Auth-Plugin und SSL-Typ. MariaDB-Rollen
mit leerem Host sind nach Prüfung von `is_role` inspizierbar; globale Rollenrechte
werden aus kataloggeprüften Privilegspalten ergänzt. Moderne globale Rollenrechte außerhalb der Legacybits 0 bis 29 führen sicher zu `unsupported_action`, statt still in der Ansicht zu fehlen; nur der Rechtebitmap wird aus `global_priv` gelesen. Anmelde- und Ressourcenoptionen
sind nur lesend. PostgreSQL-Auth/TLS ist unbekannt (`null`, pg_hba.conf).

Schreibend mit Plan und Bestätigung: Anlegen, Umbenennen, Löschen, Passwort,
GRANT/REVOKE, versionsabhängige Kontosperren, Rollenzuweisung/-entzug, begrenzte
Kontenkopie und explizites MySQL/MariaDB-FLUSH. SQLite ist nicht unterstützt.
Jede Ausführung baut den Plan auf gepinnter Sitzung neu und prüft Plan-Hash und
Aktion mit `dbplan.Check`; destruktive Aktionen verlangen die getippte Kennung.
Eigene destruktive Änderungen verlangen Zweitbestätigung; vorsorglich auch jeder
Rechte-/Rollenentzug, jede Löschung und FLUSH wegen indirekter Selbstaussperrung.

Passwort ausschließlich als `secret` bei Ausführung; niemals in Antworten,
Protokollen, Vorschautext oder Plan-Hash. Der Hash enthält den festen Platzhalter.
Erst nach vollständiger Bestätigung wird die dafür bestimmte Position ersetzt.
Keine Serverpassworthashes lesen. Keine rohen Treiberfehler oder ausgeführtes SQL
in Ergebnis-/Fehlerantworten. Bestehende Namen aus exakten Katalogtreffern, neue
Namen validiert; alles treiberspezifisch gequotet. Kein freies SQL vom Client.

MySQL/MariaDB-GRANTEE ist ein binär verglichener gebundener Metadatenwert, kein
SQL-Literal. Kollisionen seines rohen Anzeigeformats werden vor Rechteanzeige/Kopie
sicher abgewiesen. Dynamische Rechte werden dedupliziert. MySQL-Datenbank-GRANT auf
Mustersymbole wird abgewiesen; Routinenidentität bei PostgreSQL über OID und
serverformatierte Argumentsignatur. Vollständige effektive/geerbte/PUBLIC-/Besitz-/
Defaultrechte, Schema-/Sequenzrechte und vollständige MariaDB-Mehrfachauthentifizierung
werden nicht behauptet. Die Kopie umfasst direkte angebotene Rechte und Rollen,
keine ADMIN OPTION, Anmelde-/Ressourcenparameter oder PostgreSQL-Rollenattribute.

MySQL/MariaDB-Schrittfolgen sind nicht atomar; nach Teilwirkung oder unsicherer
Verbindung `outcome_unknown`. PostgreSQL nutzt Transaktion; unsicherer Commit
bleibt unbekannt. Kein automatischer Retry, kein atomarer Nachher-Snapshot.
Bestehende Sitzungen werden nicht beendet. Namensbasiertes Zeitfenster und
Client-Sperrkonvention gegen Replay wie API 0.8, im Verhaltenstext beschrieben.

## Tatsächlicher Prüfstand dieser Fortsetzung

Befehle am 25.10.2026 im Paket-Worktree. Go mit
`GOCACHE="$PWD/.build/w-p18-go-cache" GOPROXY=off`; keine Netzwerkdownloads.
Der lokale Buildcache ist ignoriert und kein Produktartefakt.

| Befehl | Ergebnis |
| --- | --- |
| `gofmt -l .` | bestanden, keine Ausgabe |
| `go vet ./...` | bestanden, keine Ausgabe |
| `go test -race -count=1 ./...` | versucht, scheitert ausschließlich an bestehendem `TestMySQLPerConnectionLoggerSuppressesServerInput`: Sandbox verbietet `listen tcp 127.0.0.1:0`; übrige Pakete bestanden |
| `go test -race -count=1 ./... -skip '^TestMySQLPerConnectionLoggerSuppressesServerInput$'` | bestanden; genau dieser bestehende Sockettest ausdrücklich ausgeschlossen |
| `go test -race -count=1 ./internal/database ./internal/httpapi -run Accounts` | bestanden; drei Live-Untertests ohne Fixtureports übersprungen |
| alle zwölf `scripts/check-*-contract.py` | bestanden; Accounts: v1–v12 eingefroren, v15 additiv, 16 Platzhalterfixtures |
| `python3 -m py_compile scripts/check-accounts-contract.py scripts/test-accounts.py` | bestanden |
| `git diff --check` | bestanden, rein lesend |
| `python3 scripts/check-docs.py` | bestanden; abschließende Dateizahl siehe Prüfnachtrag |
| `python3 scripts/test-accounts.py` | versucht, Docker-Vorabprüfung scheitert mit Socketzugriff `permission denied`, vor jeder Containeranlage |

Die vollständige Suite ist wegen der Sandboxgrenze ausdrücklich **nicht** als
uneingeschränkt grün behauptet. Keine Tests geändert, um diese Grenze zu verstecken.

## Geschriebene Live-Abnahme, noch nicht ausgeführt

`TestLiveAccounts/{mysql,maria,postgres}` prüft Anlegen und unabhängiges Nachlesen,
alte/neue Passwortanmeldung, unveränderten Passwortplan-Hash, falsche Aktion/Hash/
getippten Namen, globale und alle angebotenen Objektrechte, Rechteanzeige vor/nach
Entzug, SHOW-GRANTS-/PostgreSQL-ACL-Vergleich, Rollen, exakte Rechte-/Rollenidentität
der Kopie, Sperren/Entsperren und Anmeldung, eingeschränkte Verbindung, Eigenkonto-
Zweitbestätigung, Umbenennen/Löschen bei aktiver Sitzung sowie unbeteiligte Tabelle.
Zusätzlich: MySQL/MariaDB-Plugin-/SSL-/Ressourcenanzeige nach Direktänderung,
PostgreSQL-Verbindungslimit, Konten mit Quote/Sonderzeichen und unterschiedlicher
Groß-/Kleinschreibung, MariaDB-Rolleninspektion mit Tabellen- und globalem Recht.
Temporäre Antworten werden anschließend gegen v15 geprüft und gelöscht.

Nicht ausgeführt/belegt: diese Live-Abnahme auf den drei Netzwerkengines,
ältere Serverversionen außer lokalen Versionsgrenzentests, produktionsnahe
Verbindungsabbrüche/Sperrkonflikte, CI, Web-/Browserintegration, Compose, Swarm
oder Deployment. Vorhandene andere Live-Handoffs sind kein Beleg für W-P18.

## Unabhängiges statisches Review

Ein gesonderter Reviewer prüfte in dieser Fortsetzung ausschließlich lesend und
startete keine Jobs. Befunde und lokale Regressionen:

- GRANTEE-SQL-Escaping war für Metadaten falsch; Rohformat korrigiert.
- Nichtbinäre Kontenvergleiche konnten Rechte verschiedener Schreibweisen mischen;
  binäre Vergleiche ergänzt.
- Dynamische MySQL-Rechte konnten doppelt erscheinen; Deduplizierung ergänzt.
- Mehrdeutige rohe GRANTEE-Kennungen: Kollisionserkennung ergänzt.
- MariaDB-Rollen mit leerem Host: sichere lesende Katalogauflösung ergänzt.
- Globale MariaDB-Rollenrechte fehlen in USER_PRIVILEGES: separate lesende Quelle
  und lokale/live Regression ergänzt; moderne nicht darstellbare Rechte werden
  nach Bitmapprüfung sicher abgewiesen.

Dialektbelege: [MySQL 8.4](https://raw.githubusercontent.com/mysql/mysql-server/8.4/sql/auth/sql_authorization.cc)
und [MariaDB 12.3](https://raw.githubusercontent.com/MariaDB/server/12.3/sql/sql_acl.cc).
Eine vermutete Nullwertlücke wurde nach Prüfung des gemeinsamen Token-Decoders
zurückgezogen; zusätzliche Regressionen sichern erlaubte Nullwertfelder ab.
Schlussnachreview: alle bestätigten Befunde geschlossen, kein weiterer statischer
Blocker. Die Bitmapgrenze und Legacyquelle wurden ebenfalls nachgeprüft.
Statisches Review ersetzt keine Live-Abnahme oder Leadfreigabe.

## Fortsetzung für den Lead

Alle Änderungen dieser Fortsetzung liegen uncommittet im genannten Worktree;
keine Fremdänderungen verworfen. Keine eigenen Container angelegt.
Keine eigenen Prüfprozesse nach Abschluss übrig. Nächster konkreter Schritt:
auf freigegebenem Host `python3 scripts/test-accounts.py` und vollständiges
`go test -race -count=1 ./...` ohne Ausschluss ausführen, echte Ergebnisse/CI
unabhängig abnehmen; anschließend durch den Lead committen und integrieren.
Vertragsindex und Web-Lock bleiben separate Lead-/Web-Gates.

## Prüfnachtrag

Abschließende Dokumentationsprüfung: 883 Dateien, 0 Fehler. Go-Vet,
Formatierung, alle zwölf Vertragswächter, Python-Kompilierung und Diffprüfung
bestanden. Vertrags-SHA direkt gegen die geschriebene v15-Datei geprüft.
Die öffentlichen Metadaten des unveränderten HEAD verwenden weiterhin die
bestehende Noreply-Autoradresse; keine Identität geändert und kein Commit erzeugt.
