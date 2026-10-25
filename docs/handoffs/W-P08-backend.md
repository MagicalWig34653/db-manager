# Handoff W-P08 Backend: Spalten ändern (API 0.9)

Stand: 24.10.2026. Implementiert und lokal eingeschränkt geprüft, **keine vollständige
Vier-DB-Abnahme und noch kein unabhängiger Review**. Worktree `.worktrees/w-p08-backend`,
Branch `feat/w-p08-backend`, unveränderter Basiscommit
`5921a9dc1b813cc63b212fa5579f813b1595b761`. Keine Git-Schreibbefehle, kein Commit,
kein Merge/Push, kein Webpin und kein Assetbuild. Lead übernimmt Veröffentlichung
und zentrale Status-/Vertragsindexpflege.

## Ergebnis

- API 0.9 mit `GET /api/v9/capabilities`, `POST /api/v9/columns/plan` und
  `POST /api/v9/columns/alter`: Batch bis 16 Änderungen für add/change/rename/drop/reorder.
  Fester gemeinsamer Dialekt-Typkatalog für W-P09; MySQL/MariaDB zusätzlich ENUM/SET
  und Position first/before/after. NULL, konstante Defaults, DB-Kommentare und
  katalogvalidierte VARCHAR-Kollationen. Keine generierten Ausdrücke (E5).
- Vorschau mit exakten Anweisungen, Hash, Warnungen, NULL-Zählung bei NOT NULL und
  DROP-Abhängigkeiten. Typ-/Attributersetzung, Umordnung und DROP sind konservativ
  zerstörend: `columns_alter`, getippter **Katalog-Tabellenname**, frischer Plan-Hash.
  Bezeichner über `dbplan`, Katalogwerte gebunden; DDL-Konstanten nach API-0.6-Muster
  validiert und unter gepinnten Literalmodi gequotet (DDL akzeptiert keine Platzhalter).
- SQLite/PostgreSQL: Transaktion mit Rücklesen vor Commit, Batchfehler rollen zurück.
  PostgreSQL nur reguläre Tabellen ohne Vererbung/Partitionen und feste Katalogcasts.
  SQLite ausschließlich native ALTER-Funktionen, kein Neuaufbau. MySQL/MariaDB nur
  InnoDB, ein kombiniertes ALTER je Batch, getrennte Strukturbeobachtung. Eigene
  Attribute werden vollständig ersetzt; nicht modellierte Sonderattribute werden
  abgewiesen, Kommentar und VARCHAR-Kollation bei Ersetzung ausdrücklich verlangt.
- Ergebnisse `completed`, `rejected`, `outcome_unknown`; sichere feste Gründe und
  Meldungen, keine Treiberfehler/SQL-/Pfadlecks in Fehlerantworten. Kein Retry/Replay-
  oder ABA-Schutz versprochen. Nach Abschluss/unbekanntem Ausgang muss der Client
  sperren, neu planen und neu bestätigen (Konvention aus OBJECT-OPS-API).
- Vertrag SHA-256: `508b9951d102ec44bf4cafc9fe444f7266b31f898d9e91c887f4bef39735e3f8`.
  Strikte Objekte/Varianten, lokale Referenzen, separate Plan-/Ausführungsschemata.
  Bestehende Vertragsdateien bleiben byteidentisch. V9 enthält unverändert die
  v8-Definitionen des W-P07-Vertrags mit SHA
  `74aa39ed06311f2c45afc274a2412a0ac30912e3a5683ec96b812fd473a28264`.
  W-P07-Dateien fehlen auf dieser Basis und wurden aus dem Nachbarworktree nur
  gelesen. V8-Laufzeitrouten werden hier nicht zusätzlich implementiert. Der Wächter
  prüft die geerbten Definitionen über einen kanonischen Digest, auch ohne lokale
  v8-Datei; nach W-P07-Integration zusätzlich deren Vertragsbytes.
- 29 JSON-Fixtures: sechs **echte SQLite-/HTTP-Antworten**, SQLite 3.53.4;
  23 **handgeschriebene Vertragsplatzhalter**, ausdrücklich keine MySQL-/MariaDB-/
  PostgreSQL-Nachweise. Herkunft steht im Fixture-README. Live-Skript nutzt `dbfix.py`,
  startet Systeme nacheinander und validiert Antwortdumps gegen den Vertrag.

## Dateien / Besitz

| Dateien | Inhalt |
| --- | --- |
| `internal/dbplan/types.go`, `types_test.go` | gemeinsamer fester Typkatalog, Parameter-/ENUM-/SET-Validierung, SQL-Rendering; W-P09 liest |
| `internal/core/columns.go` | Anfrage-/Antworttypen, Engine-Interface, Fähigkeiten und strikte Aktionsdecoder |
| `internal/database/columns.go`, `columns_plan.go`, `columns_test.go` | gepinnte Sitzung, Planbau je Dialekt, Katalogauflösung/Abhängigkeiten, Bestätigung, Ausführung, sichere Fehler, lokale und geschriebene Live-Tests |
| `internal/httpapi/columns.go`, `columns_test.go` | authentifizierte Standardrouten, Validierung, Fehlerstatus und HTTP-Fälle |
| `internal/httpapi/features.go` | genau eine Registrierungszeile angehängt |
| `internal/httpapi/features_test.go` | ausschließlich Featureanzahl 7 → 8; nach paralleler W-P07-Integration durch Lead auf 9 zusammenführen |
| `contracts/api-v0.9.0.json`, `contracts/COLUMNS-API.md`, `contracts/fixtures/columns/*` | Vertrag, Verhalten, 29 JSON-Fixtures und Herkunft |
| `scripts/check-columns-contract.py`, `scripts/test-columns.py` | Byte-/Additivitäts-/Schemawächter, sequenzielle Vier-DB-Suite mit SQLite-only-Option |
| `.github/workflows/columns.yml` | CI für Go-Race/Vet, Vertragswächter und echte DB-Suite |
| `docs/handoffs/W-P08-backend.md` | diese Übergabe |

Keine neuen Abhängigkeiten; eigener Code AGPL-3.0-only. Zentrale Plan-/Statusdateien,
Vertragsindex und Web-Lock unverändert.

## Tatsächlich ausgeführte Prüfungen

Alle Go-Prüfungen nach dem ersten Cachefehler mit
`GOCACHE=/private/tmp/dbmanager-w-p08-gocache` (beschreibbarer temporärer Cache).
Der Standardcache war in der Sandbox nicht verwendbar; kein Dependency-/Netzwerk-
Fallback nötig. Datum aller Prüfungen: 24.10.2026.

| Befehl | Ergebnis |
| --- | --- |
| `gofmt -l .` | keine Ausgabe |
| `GOCACHE=/private/tmp/dbmanager-w-p08-gocache go vet ./...` | bestanden, keine Ausgabe |
| `GOCACHE=/private/tmp/dbmanager-w-p08-gocache go test -race -count=1 ./...` | ausgeführt, scheitert ausschließlich am bestehenden `TestMySQLPerConnectionLoggerSuppressesServerInput`: lokaler TCP-Listener `bind: operation not permitted`; übrige Pakete erfolgreich |
| `GOCACHE=/private/tmp/dbmanager-w-p08-gocache go test -race -count=1 ./... -skip '^TestMySQLPerConnectionLoggerSuppressesServerInput$'` | alle Pakete erfolgreich; Live-DB-Fälle ohne Fixturevariablen regulär übersprungen |
| `GOCACHE=/private/tmp/dbmanager-w-p08-gocache go test -race -count=1 ./internal/database ./internal/httpapi ./internal/dbplan -run 'TestColumn\|TestSQLiteColumns'` | Paketprüfungen für Katalog, Validierung, Outcomes und SQLite/HTTP erfolgreich |
| `GOCACHE=/private/tmp/dbmanager-w-p08-gocache python3 -B scripts/test-columns.py --sqlite-only` | bestanden: SQLite 3.53.4, HTTP, atomarer Batch-Rollback, Sichtabhängigkeit; sechs echte Antworten schemavalidiert |
| `python3 -B scripts/check-table-contract.py` | bestanden |
| `python3 -B scripts/check-mutation-contract.py` | bestanden |
| `python3 -B scripts/check-insert-contract.py` | bestanden |
| `python3 -B scripts/check-structure-contract.py` | bestanden |
| `python3 -B scripts/check-create-contract.py` | bestanden |
| `python3 -B scripts/check-overview-contract.py` | bestanden, 32 Bestandsfixtures |
| `python3 -B scripts/check-columns-contract.py` | bestanden, geerbte Definitionen und 29 W-P08-Fixtures |
| `python3 -B scripts/check-docs.py` | bestanden, 518 Dateien, null Fehler |
| `git diff --check` | bestanden (nur lesend) |
| `docker info --format '{{.ServerVersion}}'` | Docker-Socketzugriff verweigert; keine Container gestartet |
| `shasum -a 256 contracts/api-v0.9.0.json` | oben dokumentierter SHA bestätigt |

Lokale Fälle: Sondernamen mit Quote/Backtick/Semikolon und unbeteiligte Nachbartabelle,
konstante Werte mit Quote/Backslash/SQL-Text, stabiles Planhash, falscher Hash/Aktion/
getippter Name, native Addition/Rename/Drop, SQLite-Nichtunterstützung, abhängige
Sicht, PK-Drop abgewiesen und vorherige Batchaddition vollständig zurückgenommen,
HTTP-Auth/null/fremde Felder/Größenlimit/Replay, Typ-/Defaultvalidierung.
Ausführungsfehler werden mit Fault-Injection geprüft: Rechte, Sperren, Verbindungsverlust,
fehlende Beobachtung, verlorenes Commit und Abbruch vor Versand; keine Live-Störung
als getestet behauptet.

## Nicht ausgeführt / Grenzen / offene Punkte

- `python3 -B scripts/test-columns.py` **nicht ausgeführt**, weil Docker-Socketzugriff
  schon in der Vorprüfung verboten war. Daher **keine** Live-Abnahme für MySQL 8.4,
  MariaDB 12.3 und PostgreSQL 18.6/TLS. Die Live-Tests sind geschrieben: Addition,
  Attributersetzung mit unabhängigem Kataloglesen von Typ/Länge/NULL/Default/Kommentar,
  MySQL/MariaDB ENUM/SET und Position, NOT-NULL-Warnung und Ablehnung vorhandener NULLs,
  Rename/Drop, FK-Blockade, SELECT-Rolle ohne ALTER-Recht und Schutz der Nachbartabelle.
  Diese Fälle und deren Antwortdumps müssen auf Docker-fähigem Host tatsächlich laufen.
- Der bestehende Listener-Test braucht eine Umgebung mit erlaubtem lokalem TCP;
  ein uneingeschränkt grüner vollständiger Race-Lauf wird ausdrücklich nicht behauptet.
- Kein echter konkurrierender Lock-/Verbindungsabbruch-/Commitverlusttest gegen die
  drei Netzwerkengines; deren Fehlerpfade sind bislang durch Unit-Faults geprüft.
- Kein Compose-/Swarm-/Browser-/Webbuild-/Deploymentlauf für dieses Backendpaket.
  Kein W-P07-Wächterlauf: das Skript fehlt auf dieser Basis, seine geerbten
  Vertragsdefinitionen werden stattdessen im W-P08-Wächter geschützt.
- DROP-Abhängigkeiten sind begrenzt und teilweise textuell, keine vollständige
  Trigger-/View-/Externeffektanalyse. Berechtigungen können Katalogsicht einschränken.
  Native Engineprüfungen bleiben aktiv, kein CASCADE. Timeout beendet keine
  serverseitige Arbeit garantiert; bei verlorener Bestätigung Ausgang unbekannt.
- Keine generierten Spalten und kein SQLite-Neuaufbau gemäß E5; keine PostgreSQL-
  ENUM-Typanlage oder Positionsänderung, keine MySQL-Sonderattributrekonstruktion.
  SQLite-Affinität garantiert keine VARCHAR-/DECIMAL-Präzision. NOT-NULL-Vorzählungen
  können teuer sein und veralten; alle Attributersetzungen benötigen Bestätigung.
- Unabhängiger Review und Leadintegration stehen aus. Keine offenen eigenen Jobs
  oder gestarteten DB-Ressourcen nach Abschluss. Änderungen sind ausschließlich
  uncommittete Dateien im zugewiesenen Worktree.

Nächster konkreter Schritt: Lead führt auf Docker-fähigem Host die geschriebene
Vier-DB-Suite und den vollständigen Race-Lauf aus, lässt Vertrag und destruktive
Pfade unabhängig prüfen und integriert W-P07/W-P08 mit zusammengeführter
Featureanzahl. Erst danach Vertragsindex/Webkopie und später den Webpin aktualisieren.
