# Handoff W-P18 Backend: Datenbankbenutzer und Rechte

Stand: 24.10.2026. Implementiert im vorhandenen Worktree `.worktrees/w-p18-backend`,
Branch `feat/w-p18-backend`, unveränderte Basis/HEAD
`b74d10e6ff50ed49dccb0484ee3f8d2f0ca5b656`. Ausschließlich Dateien geschrieben;
keine Git-Schreibbefehle, kein Commit, keine Integration oder Veröffentlichung.
Lead übernimmt Commit und Push. API **0.18 ist vorläufig**, kein Web-Lock-Pin und
keine Änderung des zentralen Vertragsindex oder der Status-/Planungsdateien.

## Ergebnis und Dateibesitz

- `internal/core/accounts.go`: Eingabe-/Antworttypen, konservative statische
  Treiberfähigkeiten und optionale Engine-Schnittstellen.
- `internal/database/accounts.go`, `accounts_read.go`, `accounts_plan.go`:
  eigene kompatible Sitzungsvorbereitung, Katalogauflösung, Benutzerliste,
  direkte globale/Datenbank-/Tabellen-/Spalten-/Routinenrechte, direkte Rollen,
  Ressourcenlimits, primäre Auth-Plugin-/SSL-Anzeige. Neue Konten, Umbenennen,
  Löschen, Passwort, GRANT/REVOKE einschließlich Weitergabeberechtigung,
  versionierte Sperren, Rollenzuweisung/-entzug, Kontenvorlage und Rechte-Neuladen.
- `internal/httpapi/accounts.go`: 28 authentifizierte Routen unter `/api/v18/`,
  aktionsspezifische Feld-/Präsenzprüfung auch bei verbotenen Nullwerten und
  numerischen Nullwerten, sichere Fehler. `access` ist eine konservative,
  verbindungsbezogene Anzeigehilfe; der Datenbankserver entscheidet über Rechte.
- `internal/database/accounts_test.go`, `accounts_safety_test.go`,
  `accounts_live_test.go`, `internal/httpapi/accounts_test.go`: Validierung,
  Quoting, Versionen, SQLite-Nichtunterstützung, Geheimnis-/Fehlerhygiene,
  Ergebniszustände, Plan-/Bestätigungsschutz, Authentifizierung und strikte Felder.
  Lokale Protokollfixture testet echten Planbau und Ausführungslogik ohne Socket.
- `contracts/api-v0.18.0.json`, [Verhalten](../../contracts/ACCOUNTS-API.md),
  `contracts/fixtures/accounts/`: 16 ausdrücklich synthetische gültige/ungültige
  Platzhalterfixtures plus README. SHA-256 des JSON-Vertrags:
  `07abe7a0541c01b5033e9a9df73796cc164187f0bdab202d48cfd5018be8fe1e`.
  Additiv zum vorhandenen v8; v1 bis v8 byteidentisch. Keine API 0.9–0.17 erfunden.
- `scripts/check-accounts-contract.py`: alte Bytes, Additivität, Auth,
  strikte Schemata, geschützte Eingabefelder, Fixtures und Integerwerte von
  Ressourcenlimit-Maps. `scripts/test-accounts.py`: sequenzielle eigene
  dbfix-Fixtures MySQL 8.4, MariaDB 12.3, PostgreSQL 18.6, echte Antwortvalidierung.
- `.github/workflows/accounts.yml`: eigener CI-Prüfjob einschließlich Livetests.
- [ADR W-P18](../decisions/ADR-W-P18-kontoplaene.md): geschützte Passwortwerte,
  eindeutige Kontenkennung, konservative Zweitbestätigung, Grenzen der Kontenvorlage.
- Konfliktstellen minimal: `internal/httpapi/features.go` genau ein neuer Eintrag;
  `features_test.go` ausschließlich Zählung 8 auf 9. Alle v18-Routentests separat.

Keine neuen Abhängigkeiten, keine Datenmigration und kein persistenter
Produktzustand. Eigener Code AGPL-3.0-only mit SPDX, keine Attribution-Zeilen.

## Sicherheitsverhalten und Abgrenzung

Datenbankkonten, keine Produktkonten (E2). Rechteänderungen sind nach E9 bestätigt.
Alle Ausführungen berechnen den Plan auf einer gepinnten Sitzung neu und verlangen
Hash und passende Aktion; destruktive Aktionen zusätzlich die exakte getippte
Kontenkennung aus der Vorschau. Löschen und Rechte-/Rollenentzug verlangen immer
zusätzliche Bestätigung, um indirekte Selbstaussperrung vorsorglich zu schützen.
Eigene Sperren/Umbenennungen/Passwortänderungen sind ebenfalls zweitbestätigt.

Passwort nur `secret` der Ausführung, nie Plananfrage, Vorschautext, Plan-Hash,
Antwort oder Protokoll. Der feste Platzhalter geht in den Hash ein; das Passwort
wird erst nach Bestätigung für die Ausführung gequotet. Keine Server-Passworthashes
lesen. MySQL-Treiberlogger ist im bestehenden Verbindungsbau unterdrückt;
keine eigenen Request-/SQL-Logs. Rohe Treiberfehler werden nie weitergegeben.

Namen bestehender Konten/Objekte aus exakten Katalogtreffern; neue Namen validiert
und gequotet. Festes Rechtevokabular, kein SQL vom Client. PostgreSQL-Routinen werden
per OID und vom Server formatierter Argumentsignatur identifiziert. MySQL-DB-GRANT
auf Namen mit Mustersymbolen wird sicher abgewiesen statt fremde DBs zu erfassen.

SQLite unsupported; globale PostgreSQL-Rollenattribute, Plugin-/TLS-/Limitänderungen
nur lesend. PostgreSQL-Objektrechte aus der verbundenen Datenbank; direkte Rechte
sind kein vollständiges Modell effektiver/geerbter/PUBLIC-/Besitz-/Defaultrechte.
Keine Schema-/Sequenzrechte. MariaDB primäres Plugin statt vollständiger
Mehrfachauthentifizierung. Die Kopie übernimmt direkte angebotene Rechte und
Rollen ohne ADMIN OPTION, keine Anmelde-/Ressourcenparameter oder Besitzrechte;
nicht unterstützte direkte Rechte verhindern die ganze Vorschau.
MySQL/MariaDB-GRANT-OPTION-Entzug wirkt auf den gesamten Scope und wird ausdrücklich
in der Vorschau gewarnt. Vollständige phpMyAdmin-Parität wird nicht behauptet.

MySQL/MariaDB-Schrittfolgen sind nicht atomar; nach Teilwirkung oder unsicherer
Verbindung `outcome_unknown`. PostgreSQL nutzt Transaktion, unsicherer Commit
bleibt unbekannt. Kein automatischer Retry. Nach Ausführung separat neu lesen;
keine Behauptung eines atomaren Nachher-Snapshots. Bestehende Sitzungen werden
nicht beendet. Namensbasierte Pläne haben dasselbe dokumentierte Zeitfenster und
dieselbe Client-Sperrkonvention gegen Replay wie API 0.8.

## Tatsächlich ausgeführte Prüfungen

Alle Befehle im Paket-Worktree am 24.10.2026. Für Go ist
`GOCACHE="$PWD/.build/w-p18-go-cache"` gesetzt; der globale Buildcache war zunächst
mit `operation not permitted` gesperrt. Der lokale Cache ist ignoriert, kein
Produktartefakt. Kein Netzdownload war für die erfolgreichen Go-Prüfungen nötig.

| Befehl | Tatsächlicher Ausgang |
| --- | --- |
| `gofmt -l .` | bestanden, keine Ausgabe |
| `go vet ./...` mit lokalem GOCACHE | bestanden, keine Ausgabe |
| `go test -race -count=1 ./...` mit lokalem GOCACHE | nicht vollständig grün: bestehender `TestMySQLPerConnectionLoggerSuppressesServerInput` kann `127.0.0.1:0` nicht binden (`operation not permitted`); zusätzlich in diesem ersten Lauf bestehender `TestCrashRecovery` mit `child timeout`; übrige Pakete ok |
| `go test -race -count=1 ./... -skip '^TestMySQLPerConnectionLoggerSuppressesServerInput$'` mit lokalem GOCACHE | alle Pakete bestanden, einschließlich SQLite-Crash-Recovery und neuer Accounts-Tests; ein bekannter Sockettest ausdrücklich ausgeschlossen |
| `go test -race -count=1 ./internal/database ./internal/httpapi -run Accounts` mit lokalem GOCACHE | bestanden; Live-Untertests ohne Fixtureports übersprungen, keine Live-Abnahme daraus |
| `python3 scripts/check-accounts-contract.py` | bestanden: v1–v8 eingefroren, v18 additiv, strikte Schemata, 16 Platzhalterfixtures, negativer Map-Typtest |
| übrige sieben `scripts/check-*-contract.py` (table, mutation, insert, structure, create, overview, object-ops) | alle bestanden; bestehende v7-/v8-Fixtures unverändert gültig |
| `python3 -m py_compile scripts/check-accounts-contract.py scripts/test-accounts.py` | bestanden |
| `git diff --check` | bestanden |
| `python3 scripts/test-accounts.py` mit lokalem GOCACHE | versucht; vor Containeranlage abgebrochen: Docker-Socketzugriff `permission denied`; keine Live-DB gestartet |
| `python3 scripts/check-docs.py` | abschließender Ausgang wird unten ergänzt |

## Geschriebene, aber nicht ausgeführte Live-Abnahme

`TestLiveAccounts/{mysql,maria,postgres}` enthält: anlegen und unabhängig im Katalog
nachlesen; alte/neue Passwortanmeldung; falscher Hash/Name/Aktion; unveränderter
Passwortplan-Hash nach Passwortwechsel; alle angebotenen Rechteebenen einschließlich
Routine und Rollen; inhaltliche unabhängige SHOW-GRANTS-/PG-ACL-Prüfung;
Kontenkopie und Vergleich direkter Rechte/Rollen; Lock-/Unlock-Anmeldung;
eingeschränkte Verbindung mit ausgeblendeten Schreibaktionen und Serverablehnung;
eigene Sperre ohne Zweitbestätigung; Umbenennen und Löschen bei weiter bestehender
Sitzung; neue Anmeldung nach Löschen abgewiesen; unbeteiligte Tabelle bleibt erhalten.
SQLite-Nichtunterstützung wird lokal getestet. Echte temporäre Antworten würden
anschließend mit dem v18-Vertrag validiert und gelöscht.

**Nicht belegt:** Live-Verhalten dieser neuen Suite auf MySQL/MariaDB/PostgreSQL,
echte TLS-/Plugin-/Ressourcenanzeigen, produktionsnahe Fehlerszenarien, ältere
Serverversionen (nur lokale Versionsgrenzen-Tests), CI, Web-/Browserintegration,
Compose-/Swarm-/Deploymentprüfung. Keine vorhandenen Live-Handoffs als Nachweis
für diese neue Funktion umgedeutet. Docker-Vorabprüfung scheiterte vor jeder
Ressourcenanlage. Prozessliste (`ps`) ist ebenfalls gesperrt; alle eigenen
Tool-Testprozesse werden anhand ihrer zurückgegebenen Sitzungen beendet abgewartet.

## Unabhängige Reviews und Befunde

Zwei getrennte Reviewer ausschließlich lesend, keine parallelen Schreibzugriffe,
keine eigenen Builds oder Gitaktionen:

1. Sicherheitsreview: MySQL/MariaDB-Syntax und Wirkung von GRANT-OPTION-Entzug
   korrigiert, Warnung ergänzt; PostgreSQL-Katalogauflösung für materialisierte
   Sichten korrigiert. Nachreview geschlossen. Keine bestätigte SQL-Injektion,
   Passwortausgabe oder Bestätigungsumgehung im statischen Review.
2. Funktions-/Vertragsreview: aktionsspezifische verbotene Nullwertfelder und
   erforderliches `grant_option` strikt geprüft; eigene Sessionvorbereitung ohne
   MySQL-8-Statistikvariable für ältere Server. Nachreview geschlossen. Zusätzliche
   inhaltliche SHOW-GRANTS-/ACL-Assertions und Map-Schema-Validierung ergänzt;
   gezielte Nachprüfung angefordert.

Die statischen Reviews ersetzen weder Live-Abnahme noch zentrale Integrationsfreigabe.

## Fortsetzung für den Lead

Keine eigenen Jobs oder Container verbleiben; keine unintegrierten Fremdänderungen
verworfen. Alle Paketänderungen liegen uncommittet im zugewiesenen Branch-Worktree.
Nächster konkreter Schritt: auf einem erlaubten Host `scripts/test-accounts.py`
mit dbfix ausführen, vollständiges `go test -race -count=1 ./...` ohne Ausschluss,
CI und Livebefunde unabhängig abnehmen; anschließend durch den Lead committen,
API-Nummer und Vertragsindex freigeben. Web-Lock erst nach separater Webabnahme pinnen.
