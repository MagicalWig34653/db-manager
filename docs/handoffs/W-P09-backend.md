# W-P09 Backend: Indizes, Primär- und Fremdschlüssel

Stand: 25.10.2026. Reviewkandidat auf `feat/w-p09-backend`, Worktree
`.worktrees/w-p09-backend`, Basis main `c331c642fa4957747845458b079cffc32f6b1ff6`.
Kein Commit, keine Git-Schreibbefehle, kein Merge/Push. Lead übernimmt Commit,
unabhängigen Review, Integration und zentrale Status-/Vertragsindexpflege.

## Ergebnis und Dateien

- `internal/core/indexes*.go`: Typen, strikte aktionsbezogene JSON-Decodierung,
  Fähigkeiten; `internal/database/indexes*.go`: Metadaten, Vorschau, Duplikat-/NULL-
  Prüfung, Bestätigung, Ausführung und Tests; `internal/httpapi/indexes*.go`: drei
  authentifizierte Routen unter `/api/v13/`. `features.go` genau ein neuer Eintrag;
  `features_test.go` Anzahl 12 → 13 und unbekannte Folgeroute v14.
- `contracts/api-v0.13.0.json`: additiv zu v12; SHA-256
  `e05a2402a8e6ec6a1901afeac03969e34d4fa1821e50a5c8675a6e20f5c1d3d7`.
  Vorgänger unverändert. `contracts/INDEXES-API.md`, `contracts/fixtures/indexes/`:
  27 Fixtures, davon zehn echte SQLite-/HTTP-Antworten und 17 ausdrücklich
  handgeschriebene Platzhalter beziehungsweise absichtlich ungültige Proben.
- `scripts/check-indexes-contract.py`, `scripts/test-indexes.py`,
  `.github/workflows/indexes.yml`: Byte-/Additivitäts-/Schemawächter und sequenzielle
  Vier-DB-Suite über `dbfix.py`, auch `--sqlite-only` und `--keep`.
- [ADR-W-P09](../decisions/ADR-W-P09-indexausdruecke.md): enger strukturierter
  PostgreSQL-Ausdrucksumfang und Ersetzung; Leadannahme offen. Keine Abhängigkeiten
  ergänzt, eigener Code mit SPDX AGPL-3.0-only; keine Attribution-Zeilen.

Index CREATE/Ersetzen/Umbenennen/DROP, PK setzen/ersetzen/entfernen, native FK
anlegen/ersetzen/entfernen. FULLTEXT/SPATIAL und Präfixe nur MySQL/MariaDB InnoDB;
PostgreSQL `lower`/`upper` aus `pg_catalog` und strukturierte NULL-Teilbedingungen.
SQLite ausschließlich CREATE/DROP INDEX, kein Tabellenneuaufbau. Bestehende Ziele
und getippter Tabellenname kommen aus Metadaten; Quoting ausschließlich `dbplan`.

Vorhandene Duplikate werden mit Gruppenanzahl angezeigt und vor DDL abgewiesen;
PRIMARY prüft NULLs. Typvergleich für FK konservativ exakt, endgültige Integrität
prüft die Engine. Destruktive Aktionen verlangen Aktion, getippten Katalognamen
und frischen Planhash. W-P08-Sitzung/DDL-Runner liefern sichere Outcomes und
getrennte Spaltenbeobachtung; Schlüssel werden unabhängig über API v5 nachgelesen.
Keine rohen Treiberfehler in Antworten; kein Retry-/Replay-/ABA-Versprechen.

## Tatsächliche Prüfungen

Go 1.27.1, darwin/arm64. Go-Befehle mit
`GOCACHE=/private/tmp/dbmanager-w-p09-gocache GOPROXY=off`, keine Netzabhängigkeiten geladen.

| Befehl | Ergebnis |
| --- | --- |
| `gofmt -w internal/{core,database,httpapi}/indexes*.go`, anschließend `gofmt -l` einschließlich Featuredateien | formatiert, keine Restmeldungen |
| `go vet ./...` | bestanden, auch nach letzter Codeänderung |
| `go test -race -count=1 ./...` | bestehender `TestMySQLPerConnectionLoggerSuppressesServerInput` scheitert an verbotenem lokalem TCP-Listener (`bind: operation not permitted`); übrige Pakete bestanden |
| `go test -race -count=1 ./... -skip '^TestMySQLPerConnectionLoggerSuppressesServerInput$'` | bestanden; Abschlusslauf nach letzter Codeänderung |
| `go test -race -count=1 ./internal/core ./internal/database ./internal/httpapi -run 'TestIndexes\|TestSQLiteIndexes\|TestColumnsExecutionOutcomes'` | bestanden; einschließlich Fehler-/Commitverlustfälle des gemeinsam verwendeten DDL-Runners |
| `python3 -B scripts/test-indexes.py --sqlite-only` | bestanden, zehn echte Antworten schemavalidiert; ein zusätzlicher Lauf mit `--keep` erzeugte die versionierten SQLite-Fixtures |
| `python3 -B scripts/check-*-contract.py` (jedes der zwölf Skripte einzeln) | alle bestanden, einschließlich v13 und eingefrorener v1–v12-Bytes |
| `python3 -B scripts/check-docs.py`, `git diff --check` | bestanden |
| `docker info --format '{{.ServerVersion}}'` | Docker-Socketzugriff verboten; keine Container gestartet |
| `ps -axo pid,comm` | Prozessabfrage verboten; keine Aussage über fremde Jobs möglich |

Lokale Fälle: Sondernamen und Schutz der Nachbartabelle, Indexmetadaten über API v5,
UNIQUE-Duplikate und mehrere NULLs, stabiler/falscher Hash, falsche Aktion und
getippte Namen, Replay nach DROP, Constraintindex abgewiesen, SQLite-Fähigkeitsfehler,
strikte Felder/Nullwerte, Authentifizierung und HTTP-Größenlimit.

## Offen / nicht ausgeführt

- `python3 -B scripts/test-indexes.py` ohne `--sqlite-only`: **nicht gestartet**, da
  Docker-Vorprüfung verboten. Geschriebene, kompilierte Livefälle für MySQL 8.4,
  MariaDB 12.3 und PostgreSQL 18.6/TLS sind kein Laufnachweis: Indexersetzung/-rename,
  Reihenfolge/Präfixe, PK-Ersetzung/NULL-Ablehnung/FK-Blockade, FK-Namen/Spalten/
  ON-Aktionen/Typfehler, Ausdruck/Teilindex, FULLTEXT/SPATIAL, Identity-Spalten und
  SELECT-Rolle ohne ALTER-Rechte. Unabhängiges Nachlesen über API v5 ist enthalten.
- Kein echter Netzwerk-Lock-/Verbindungsabbruch-/Commitverlusttest; sichere Outcomes
  bislang nur über die vorhandenen Runner-Faults und lokale SQLite-Tests geprüft.
- Uneingeschränkt grüner vollständiger Race-Lauf wegen Listener-Sandbox offen.
  CI-Workflow, Compose, Swarm, Browser/Webbuild und Deployment nicht ausgeführt.
- Unabhängiger Backend-/Vertragsreview und Leadannahme der ADR fehlen. Vertrag noch
  nicht für einen Webclient gepinnt; keine vollständige Vier-DB-/Paritätsabnahme.
- Abhängigkeiten sind tabellenweit und teilweise textuell, Selbstreferenzen/Trigger
  nicht vollständig. Hash deckt Anweisungen ab, keine Warnungen, Prüfzähler oder
  Verbindungsidentität; Namensersetzung zwischen Planung und DDL bleibt begrenzt.
  Generierte/versteckte Spalten außer Identity konservativ gesperrt. FK-Typvergleich
  enger als manche Enginekompatibilität; freie Ausdrücke/Prädikate nicht unterstützt.

Keine eigenen DB-/Server-/Containerjobs verbleiben. Unintegrierte Arbeit liegt nur
in den genannten Dateien dieses Worktrees. Nächster Schritt: Lead committen lassen,
unabhängigen Review und Docker-Livetests ausführen, danach gezielte Nachbesserung
und Integrationsfreigabe. Zentrale Plan-/Statusdateien und Webassets unverändert.
