# PWA-Parität: Umsetzungsplan

Stand: 24.10.2026 (Plan); Paketstatus und API-Nummern nachgeführt am 27.10.2026, siehe „Paketstatus“ in Abschnitt 3.
Status des Plans: **Plan und Vorschlag, die Pakete der Wellen 1 bis 4 und vier Pakete der Welle 5 sind umgesetzt (Nachweise im Paketstatus), keine vollständige Parität.** Auftrag: Der Nutzer hat am
24.10.2026 als höchste Priorität freigegeben, alle phpMyAdmin-Funktionen in der PWA (Svelte-Weboberfläche,
ausgeliefert vom Go-Server) verfügbar zu machen, und autonomes Weiterarbeiten erlaubt. Das hebt für diese Linie
die Zwischenregel „keine weitere neue Featureintegration“ aus [ARBEITSPLAN](ARBEITSPLAN.md) Punkt 4 auf. Die
Apple-Linie ist nachrangig und nicht Teil dieses Plans.

Quellen: [AGENTS](../AGENTS.md), [Entwicklungsablauf](ENTWICKLUNGSABLAUF.md), [STATUS](STATUS.md),
[WEBBUILD](WEBBUILD.md), [ARCHITEKTUR](ARCHITEKTUR.md), [Parität](PARITAET.md),
[PMA-Inventar](pma-inventar/README.md) mit [FEATURE-PLAN](pma-inventar/FEATURE-PLAN.md) und
[LUECKEN](pma-inventar/LUECKEN.md), [Verträge](../contracts/README.md), [TEAM](TEAM.md), die Handoffs
[B2c](handoffs/B2c.md), [B3a](handoffs/B3a.md), [B4a](handoffs/B4a.md), [F4b](handoffs/F4b.md),
[F5b](handoffs/F5b.md), [SZ1](handoffs/SZ1.md) sowie [ADR-0006](decisions/ADR-0006-serverzustand.md).
ADR-0007 liegt nur als Vorschlag auf einem Branch und betrifft den nativen Apple-Adapter, nicht diese Linie.
Der Plan trennt Ist-Stand (im Code geprüft, Abschnitt 1), Vorschlag (Abschnitte 2 bis 5) und offene Entscheidungen
(Abschnitt 6). Eine Paketabnahme ist kein Nachweis vollständiger Parität.

## 0. Zusammenfassung

- Backend (Go) ist heute das Nadelöhr: Das Web kennt nur API 0.1 bis 0.5; es gibt keine Strukturänderung, keinen
  Import/Export auf Serverseite, keine Administration. API 0.6 (Tabelle anlegen) ist im Backend vorhanden, aber im Web
  nicht angebunden.
- Die PWA-Eigenschaft fehlt vollständig (kein Manifest, kein Service Worker, keine Icons; Go-Handler und CSP lassen
  diese Dateien auch gar nicht zu). Das ist ein kleines Paket (W-P03) und kommt in Welle 1.
- Die Weboberfläche ist ein Einzelseiten-Monolith ohne Routing (`App.svelte`, 661 Zeilen). Ohne vorherigen Umbau
  (W-P02) lassen sich Pakete nicht parallel ohne Schreibkonflikte bauen. Dasselbe gilt für Routenregistrierung,
  Build-Lock und Fixture-Skripte im Backend (W-P01, W-P04).
- Sechs Wellen, 32 Pakete (Welle 1: 6, Welle 2: 4, Welle 3: 4, Welle 4: 5, Welle 5: 12, Welle 6: 1). Reihenfolge nach Hebel für den phpMyAdmin-Alltag: Fundament und Navigation, Struktur
  ändern, Import und Export, Suche, SQL-Werkzeuge, Server- und Benutzerverwaltung, dann Spezialfunktionen.
- Pro Paket gilt derselbe Ablauf (Abschnitt 4): Vertrag zuerst, Backend und Web parallel gegen Fixtures,
  unabhängiges Review, ein Integrations-Agent für Einbettung und Pin, Dokumentation. Höchstens 3 bis 4 Agents
  gleichzeitig, Builds strikt nacheinander.
- Der Nutzer muss nur wenige Entscheidungen treffen (Abschnitt 6); für alle gilt bis dahin ein sicherer Default.

## 1. Ist-Stand der PWA (im Code geprüft am 24.10.2026)

Geprüft wurden Web-Repo `db-manager-web` (Stand `f75bfa1`, gepinnt im Root auf `8ef738d`) und Root-Repo
(`b148109`): Quellen, `package.json`, `vite.config.ts`, `index.html`, Go-Routen, `internal/webui`, `web-build.lock.json`,
`scripts/build-web.py`. Nicht aus Doku übernommen; Abweichungen zu STATUS gibt es keine.

### 1.1 Funktionen der Oberfläche

| Bereich | Tatsächlich im Code | Quelle |
| --- | --- | --- |
| Verbindungen | Mehrere flüchtige Verbindungsentwürfe im Arbeitsspeicher (SQLite-Kennung, MySQL/MariaDB, PostgreSQL), Test, Bearer-Token-Dialog; keine Persistenz (kein localStorage/IndexedDB) | `ConnectionEditor.svelte`, `App.svelte` |
| Objektbaum | Katalogliste (Name, Schema, Typ) mit Filter und Aktualisieren; kein Datenbankwechsel, keine Größen/Zeilenzahlen | `App.svelte`, `/api/v1/catalog` |
| SQL-Arbeitsbereich | Ein Textfeld, eine Anweisung, Zeilen- und Zeitlimit, Abbrechen, mehrere Ergebnissätze aus einer Antwort, Fehleranzeige; kein Verlauf, keine Lesezeichen, keine Syntaxfarben | `App.svelte`, `/api/v1/query` |
| Tabellenansicht | Describe, Browse mit Seiten, Mehrspaltensortierung, gebundene UND-Filter (Gleich, ist NULL, ist nicht NULL), Typdarstellung NULL/Leer/Binär | `TableWorkbench.svelte`, `/api/v2/*` |
| Einzelzeilen | Einfügen, Ändern, Löschen je einer Zeile mit Eignungsprüfung, Originalsnapshot und Bestätigung | `RowWorkspace.svelte`, `/api/v3/*`, `/api/v4/*` |
| Struktur lesen | Spalten, Indizes, Constraints, Abdeckung (read-only) | `StructurePanel.svelte`, `/api/v5/*` |
| Ergebnisexport | JSON/CSV aus dem geladenen Snapshot, lokal im Browser, höchstens 8 MiB | `ExportControls.svelte`, `export.ts` |
| Quellen/Lizenzen | Dialog mit Webcommit, Lizenztexten | `SourceInformation.svelte` |

Nicht vorhanden im Web: Anbindung von API 0.6 (Tabelle anlegen), Routing/URL-Zustände, Datenbankwahl, jede
Strukturänderung, Import, serverseitiger Export/Dump, Suche über Tabellen hinweg, Verlauf/Lesezeichen, Benutzer und
Rechte, Serverstatus, Routinen/Trigger/Events/Views, Wartung, Designer, Transformationen, Einstellungen, Sprachen
außer Deutsch, Themen.

### 1.2 Backend-Endpunkte (`internal/httpapi`)

| Version | Routen | Im Web angebunden |
| --- | --- | --- |
| `/healthz`, `/readyz` | Liveness und Readiness (mit optionalem Serverzustand SZ1, `RequireState` fail closed, derzeit ohne geschützte Route) | nein, nicht nötig |
| v1 | `GET capabilities`, `POST connections/test`, `POST catalog`, `POST query` | ja |
| v2 | `GET capabilities`, `POST tables/describe`, `POST tables/browse` | ja |
| v3 | `GET capabilities`, `POST tables/eligibility`, `POST rows/update`, `POST rows/delete` | ja |
| v4 | `GET capabilities`, `POST tables/insert-eligibility`, `POST rows/insert` | ja |
| v5 | `GET capabilities`, `POST tables/structure` | ja |
| v6 | `GET capabilities`, `POST tables/create` | **nein** |

Engine-Treiber: `sqlite`, `mysql` (auch MariaDB), `postgres`. Fixtures der Backend-Skripte: MySQL 8.4, MariaDB 12.3,
PostgreSQL 18.6 mit eigener CA und `verify-full`, SQLite als verwaltete Datei. Alle Anfragen tragen die Verbindung
inklusive flüchtigem Passwort; Anfragen sind auf 1 MiB begrenzt (relevant für Import, Abschnitt 6, E4).

### 1.3 PWA-Eigenschaften

| Eigenschaft | Ist | Fehlt |
| --- | --- | --- |
| Manifest | nein; `index.html` hat nur `viewport` und `theme-color` | `manifest.webmanifest` (Name, `start_url`, `display: standalone`, Farben, Sprache `de`) |
| Icons | kein `public/`-Verzeichnis, kein Favicon, kein Apple-Touch-Icon | PNG 192/512, maskable, Favicon, `apple-touch-icon` |
| Service Worker | nein | Registrierung, App-Shell-Cache, Update-Hinweis; niemals `/api` cachen |
| Installierbarkeit | nein | Manifest, SW, sicherer Kontext (HTTPS oder localhost) |
| Offline-Shell | nein | Shell offline startbar mit klarer „Keine Verbindung“-Anzeige; keine Offlinedaten |
| Go-Auslieferung | `staticMediaType` erlaubt nur `index.html`, die beiden Lizenztexte und `assets/*.js|css`; alles andere ist 404 | `manifest.webmanifest`, `sw.js`, Icons (PNG/SVG) mit festen MIME-Typen und `no-store` für `sw.js` |
| CSP | `default-src 'none'` ohne `manifest-src` und `worker-src` | `manifest-src 'self'`, `worker-src 'self'` (kein `unsafe-eval`, keine Lockerung sonst) |
| Build-Pin | `ROOT_FILES` in `scripts/build-web.py` kennt nur drei Wurzeldateien; `ASSET_NAME` nur `assets/*.js|css` | Erweiterung der Inventarregeln (Vertrag Web/Integration, W-P03/W-P04) |
| Routing | kein Router, kein SPA-Fallback im Go-Handler (unbekannte Pfade 404) | Hash-Routing im Client (keine Go-Änderung nötig); optionaler Fallback nur nach Entscheidung |
| Bedienung | Teilweise responsives Layout (320 px geprüft in W1/F5b-Umfeld); Touch/Tastatur nur punktuell | UX-007-Nachweis (Welle 5, W-P26) |

Folge: Die Weboberfläche ist **noch keine PWA**. Das Nachrüsten betrifft beide Repos (Web: Manifest, SW, Icons,
Registrierung; Root: Handler, CSP, Build-Inventar) und ist in W-P03 gebündelt.

### 1.4 Betriebs- und Prozessrahmen, der den Plan bestimmt

- Jeder Pin des Webbuilds (`web-build.lock.json`: Commit, Vertrags-SHAs, npm-Lock-SHA, Node 26.12.1, npm 11.20.0 (angehoben mit Pin Bündel 6)) verlangt
  neuen Clientreview und Neubau; `build-web.py` kennt heute fest die Verträge 0.1 bis 0.5. Jede neue API-Version
  bedeutet Änderungen an Lock, Skript, `test-build-web.py`, Web-`package.json` und `tests/generate-validators.mjs`.
  Das ist der Hauptkonfliktpunkt für parallele Pakete und wird in W-P04 durch eine generische Vertragsliste gelöst.
- Zentrale Dateien sind exklusiv: STATUS, ARBEITSPLAN, PARITAET, Vertragsindex `contracts/README.md`, `web-build.lock.json`,
  `internal/webui/**` (siehe Dateibesitz, Abschnitt 5).
- Sicherheitsleitplanken des Bestands, die für alle neuen Pakete gelten: Identifier nur aus validierten Metadaten
  quoten, Werte nur gebunden, keine freien SQL-Fragmente vom Client in Schreibpfaden, ausdrückliche Bestätigung,
  ehrlicher unbekannter Commitausgang ohne automatischen Retry, keine Credentials in Logs, SQLite nur als verwaltete
  Datenbankkennung (ADR-0004).
- Ressourcen: höchstens zwei kleine DB-Suiten gleichzeitig, DinD exklusiv, kein zweiter paralleler Build
  ([TEAM](TEAM.md)). Der Rechner ist begrenzt, daher Obergrenze 3 bis 4 gleichzeitige Agents.

## 2. Gemeinsame Bausteine und Konventionen für alle Pakete

### 2.1 Aktionsmuster (Vertragsvorgabe für jede schreibende Funktion)

1. **Plan/Vorschau:** Jede Strukturänderung hat eine Vorschau-Route (`.../plan`), die ohne Wirkung die exakten
   SQL-Anweisungen je Treiber, Warnungen und betroffene Objekte liefert und einen `plan_sha256` über die normalisierte
   Anweisungsliste ausgibt. Der Client zeigt die Anweisungen schreibgeschützt an.
2. **Ausführen:** Die Ausführungsroute verlangt dieselben Eingaben plus `confirm` (Art der Aktion, Zielobjekt als
   getippter Name bei zerstörenden Aktionen) und `expected_plan_sha256`. Weicht die neu berechnete Anweisungsliste
   ab, wird mit 409 abgelehnt. Kein freies SQL vom Client.
3. **Antwort:** `completed`, `rejected` oder `outcome_unknown`; kein automatischer Retry; nachträgliche Strukturbeobachtung
   getrennt wie in API 0.6.
4. **Rechte:** Es gelten ausschließlich die tatsächlichen DB-Rechte der Verbindung. Die Oberfläche blendet Aktionen nach
   Fähigkeitsabfrage aus, das ist Komfort und keine Sicherheitsgrenze.
5. **Fähigkeiten je Treiber:** Jede neue Version liefert `capabilities` je Treiber und Aktion (unterstützt, nicht
   unterstützt, nur mit Einschränkung). Nicht unterstützte Fälle werden in der UI erklärt statt versteckt.
6. **Lesende Funktionen** brauchen kein Plan-Muster, aber Limits (Zeilen, Bytes, Zeit) und Quoting aus Metadaten.

### 2.2 Vertrags- und Versionsregeln

- Ein Paket mit Backend-Anteil erhält genau eine neue API-Minor (`api-v0.N.0.json`, Verhaltensvertrag
  `contracts/<NAME>-API.md`). Die Zuordnung steht in Abschnitt 3 und wird vom Lead vergeben, damit parallele Pakete keine
  Nummern kollidieren lassen. Bestehende Vertragsbytes bleiben unverändert.
- Der Web-Client kopiert den Vertrag mit Provenienz nach `src/lib/contract/vN/` (wie v2 bis v5) und prüft ihn mit
  Standalone-Validatoren (CSP ohne `unsafe-eval`).
- Neue Vertragsfamilien werden in W-P04 über eine Liste in `web-build.lock.json` erfasst, nicht mehr über feste
  Schlüsselnamen.

### 2.3 Fixtures und Tests je DB-System

- Jede Abnahme verlangt Läufe gegen **SQLite, MySQL 8.4, MariaDB 12.3 und PostgreSQL 18.6** (Fixture-Versionen), sofern die
  Funktion für das System existiert; sonst ist die Nichtunterstützung selbst ein Testfall (Fähigkeit meldet „nein“,
  Anfrage wird abgewiesen). Teilnachweise, wie in der Matrix gefordert, mit Test-ID, Fixture, Version, Befehl, Datum.
- Unabhängiges Nachlesen in der Datenbank statt Vertrauen in die eigene Antwort (Katalogabfrage oder Direktverbindung).
- Zerstörende Funktionen: negative Tests (falscher Bestätigungsname, abweichender Plan, fehlende Rechte), Sonderzeichen in
  Namen (Quote, Backtick, Leerzeichen, SQL-Text als Name), unbeteiligtes Fixtureobjekt bleibt unberührt.
- Web: Vitest-Unit/Komponenten, Playwright-Fälle gegen Mock (`tests/browser`) und gegen echtes Backend
  (`tests/real`, SQLite und PostgreSQL, MySQL/MariaDB soweit die Integrationsumgebung sie trägt), CSP-Test ohne Verstoß.
- Gemeinsame Fixture-Bibliothek `scripts/dbfix.py` (W-P01) ersetzt die kopierten Container-Skripte, damit nicht jedes
  Paket drei Container eigenständig startet.

### 2.4 Rollen und Modelle

| Rolle | Modell | Einsatz |
| --- | --- | --- |
| Backend-Agent | Sonnet | Go: Core/Datenbank/HTTP, Vertrag, 4-DB-Tests |
| Web-Agent | Sonnet | Svelte-Komponenten, Vertragsclient, Browsertests |
| Integrations-Agent | Sonnet | Pin, Lock, Neubau, Einbettung, Gesamtchecks (einzige Rolle, die Assets baut) |
| Reviewer | Sonnet, anderer Agent als der Autor | unabhängiger Review von Vertrag, Backend, Web, Integration |
| Haiku-Agent | Haiku | mechanische Kleinpakete: Übersetzungen/Formulartexte, Fixtures/Beispieldaten, Mock-Antworten, Doku-Nachzug, Handoff-Gerüste, Icons-Rastern |
| Lead | Root/Tech-Lead | Vergibt Pakete und Versionsnummern, entscheidet Konflikte, integriert nach Review, pflegt zentrale Dokumente (oder delegiert exklusiv an den Doku-Agenten) |

Sicherheitskritische Pakete (Rechte, Import, Konten, Auth) erhalten zusätzlich einen Sicherheitsreview durch einen
zweiten, fachlich geeigneten Sonnet-Reviewer.

## 3. Wellen und Pakete

Legende: **B** Backend, **W** Web, **I** Integration, **R** Review, **D** Doku/Haiku. „API“ nennt die Vertragsversion
(Vorschlag des Leads, endgültige Vergabe beim Start). IDs ohne Präfix `PMA-` (z. B. `DDL-002` = `PMA-DDL-002`);
`NEU-…` = `PMA-NEU-…`; `A..B` = lückenlos von A bis B. Abkürzungen der Treiber: SQ SQLite, MY MySQL, MA MariaDB,
PG PostgreSQL.

### Übersicht (eine Zeile je Welle)

| Welle | Thema | Pakete |
| --- | --- | --- |
| 1 | Fundament, PWA, erste Hebel | W-P01 Registry und Fixture-Harness, W-P02 Web-Shell, W-P03 PWA-Basis, W-P04 Build-Lock-Generalisierung, W-P05 Tabelle anlegen (UI zu v6), W-P06 Katalog/Übersicht/Definition/Serverinfo (v7) |
| 2 | Struktur ändern | W-P07 Datenbank- und Tabellenoperationen (v8), W-P08 Spalten ändern (v9), W-P09 Indizes, Schlüssel, Fremdschlüssel (v10), W-P10 Tabellenwartung (v11) |
| 3 | Daten rein und raus | W-P11 Export und SQL-Dump (v12), W-P12 Import SQL/CSV (v13), W-P13 Mehrzeilen, Dateiwerte, FK-Auswahl (v14), W-P14 Suche, Suchen und Ersetzen (v15) |
| 4 | SQL und Administration | W-P15 SQL-Stapel, EXPLAIN, DML-Simulation (v16), W-P16 SQL-Editor, Verlauf, Lesezeichen (nur Web), W-P17 Serverstatus, Variablen, Prozesse (v17), W-P18 Benutzer und Rechte (v18), W-P19 Views, Routinen, Trigger, Events (v19) |
| 5 | Komfort und Spezialfunktionen | W-P20 Konfigurationsspeicher (v20), W-P21 Designer/Beziehungen/QBE, W-P22 Transformationen, W-P23 Tracking/Zentralspalten/Normalisierung, W-P24 Diagramme, W-P25 Geodaten, W-P26 Einstellungen/Themen/Sprachen/Tastatur, W-P27 Monitor/Advisor/Replikation/Binlog, W-P28 Erweiterte Import-/Exportformate und Dump-Details, W-P29 Partitionen und Präfix-Stapel, W-P30 Anmeldeverfahren/Teams/Gruppen, W-P31 Produktinfo, Setup-Ersatz, Fehlerberichte |
| 6 | Abschluss | W-P32 Paritätsaudit, Lücken-Nacharbeit, Release |

**Endgültige API-Nummern (Integration Welle 2, 25.10.2026):** Die Nummern in diesem Plan sind Vorschläge. Vergeben wurde in
Integrationsreihenfolge ohne Lücken: 0.8 Datenbank-/Tabellenoperationen (W-P07), 0.9 Spalten (W-P08), 0.10 Tabellenwartung (W-P10),
0.11 Export und SQL-Dump (W-P11), 0.12 SQL-Stapel/EXPLAIN/Simulation (W-P15). Spätere Pakete (zum Beispiel W-P09 Indizes) erhalten die
nächste freie Nummer bei ihrer Integration; Pfade `/api/vN/`, Vertragsdatei und Wächter folgen dieser Nummer. Siehe
[WAVE2-INTEGRATION](handoffs/WAVE2-INTEGRATION.md). Welle 3: 0.13 Indizes/Schlüssel (W-P09), 0.14 Suche/Ersetzen (W-P14), 0.15 Benutzer/Rechte (W-P18), 0.16 Views/Routinen/Trigger/Events (W-P19); W-P17 (Serververwaltung) ist noch nicht integriert und erhält die nächste freie Nummer. Siehe [WAVE3-INTEGRATION](handoffs/WAVE3-INTEGRATION.md).
Welle 4: 0.17 Serververwaltung nur lesend (W-P17), 0.18 Import (W-P12), 0.19 Mehrzeilen/Dateiwerte/FK-Nachschlagen (W-P13), 0.20 Monitor lesend (W-P27),
0.21 erweiterte Exportformate (W-P28), 0.22 Partitionen/Präfix-Stapel (W-P29), Zusatz 0.23 (`AccountResult.reason` + `grant_without_effect`, keine neuen Pfade).
Siehe [WAVE4-INTEGRATION](handoffs/WAVE4-INTEGRATION.md). Die Nummern in den Paketüberschriften unten (zum Beispiel „W-P09 API 0.10“, „W-P12 API 0.13“)
sind die ursprünglichen Vorschläge und gelten nicht mehr; maßgeblich ist diese Liste und der [Vertragsindex](../contracts/README.md).

#### Paketstatus (Stand 27.10.2026)

Quellen: Backend-`main` `84a5e2e` (Verträge v1 bis v23), Web-`main` `4b24e53` (Pin [Bündel 5](handoffs/PIN-BUNDLE-5.md)), die genannten Handoffs.
„Review“ bedeutet unabhängiges Review mit schriftlichem Bericht; die Spalte nennt dessen Ergebnis, nicht eine Paritätsabnahme.

| Paket | Backend (API) | Web | Review und offene Punkte |
| --- | --- | --- | --- |
| W-P01 Registry, Fixture-Harness | umgesetzt | – | [Review](handoffs/W-P01-review.md): Freigabe |
| W-P02 Web-Shell, Routing | – | umgesetzt | Review im Web-Repo (W-P02-review) |
| W-P03 PWA-Basis | – / Go-Handler | umgesetzt | [Review](handoffs/W-P03-review.md): Freigabe, niedrige Befunde |
| W-P04 Build-Lock | umgesetzt | umgesetzt | [Review](handoffs/W-P04-review.md): Freigabe |
| W-P05 Tabelle anlegen (UI zu 0.6) | 0.6 (B4a) | umgesetzt | Web-Review W-P05 im Web-Repo |
| W-P06 Übersicht, Definition, Serverinfo | 0.7 | umgesetzt | [Backend-Review](handoffs/W-P06-backend-review.md): Freigabe; Web-Review im Web-Repo |
| W-P07 Datenbank-/Tabellenoperationen | 0.8 | umgesetzt | [Backend-Review](handoffs/W-P07-backend-review.md); Datenbank kopieren/umbenennen (OBJ-004) nicht umgesetzt |
| W-P08 Spalten ändern | 0.9 | umgesetzt | [Backend-Review](handoffs/W-P08-backend-review.md) |
| W-P09 Indizes, Schlüssel | 0.13 | umgesetzt | [Backend-Review](handoffs/W-P09-backend-review.md), [Re-Review](handoffs/WAVE3-REREVIEW.md); Web-Welle unabhängig geprüft (WEB-WAVE-REVIEW) |
| W-P10 Tabellenwartung | 0.10 | umgesetzt | [Backend-Review](handoffs/W-P10-backend-review.md); kein Echtlaufskript im Repo, nur ein reviewerlokaler Lauf gegen die Einbettung (SQLite, PostgreSQL) im [Bündel-4-Review](handoffs/PIN-BUNDLE-4-review.md) |
| W-P11 Export, SQL-Dump | 0.11 | umgesetzt | [Backend-Review](handoffs/W-P11-backend-review.md); Web-Review im Web-Repo |
| W-P12 Import SQL/CSV | 0.18 | umgesetzt | Review A, [Re-Review](handoffs/WAVE4-REREVIEW.md): Blocker B1, Fix nicht erneut unabhängig geprüft; N1 Speicher offen; SQL-Import ohne Plan by design |
| W-P13 Mehrzeilen, Dateiwerte, FK-Nachschlagen | 0.19 | umgesetzt | [Review A](handoffs/WAVE4-REVIEW-A.md); nur BLOB-Dateiwerte, kein TEXT-Upload, keine ENUM/SET-Listen |
| W-P14 Suche, Suchen und Ersetzen | 0.14 | umgesetzt | [Backend-Review](handoffs/W-P14-backend-review.md), Re-Review |
| W-P15 SQL-Stapel, EXPLAIN, Simulation | 0.12 | umgesetzt | [Backend-Review](handoffs/W-P15-backend-review.md); Simulation nur SQLite |
| W-P16 SQL-Verlauf, Lesezeichen | – (nur Web) | umgesetzt, browserlokal, Verlauf standardmäßig aus | WEB-WAVE-REVIEW im Web-Repo |
| W-P17 Serverstatus, Variablen, Prozesse | 0.17 nur lesend | umgesetzt (lesend) | [Review B](handoffs/WAVE4-REVIEW-B.md); schreibende Eingriffe (Variablen setzen, KILL) nicht umgesetzt |
| W-P18 Benutzer und Rechte | 0.15 | umgesetzt | [Backend-Review](handoffs/W-P18-backend-review.md), Re-Review; kein Browser-Echtlauf |
| W-P19 Views, Routinen, Trigger, Events | 0.16 | umgesetzt | [Backend-Review](handoffs/W-P19-backend-review.md), Re-Review |
| W-P26 Einstellungen, Themen, Sprachen | – (nur Web) | Themen, Deutsch und Englisch, Kürzelübersicht, browserlokal | WEB-WAVE-REVIEW; weitere Sprachen, RTL und Touch-Nachweis nicht umgesetzt |
| W-P27 Monitor, Advisor, Replikation, Binlog | 0.20 nur lesend | umgesetzt (lesend) | Review B; Replikation nur Einzelserver, keine Steuerung |
| W-P28 erweiterte Formate, Dump-Details | 0.21 | umgesetzt | Review B; XML, YAML, PHP, LaTeX, MediaWiki, Texy, CodeGen-Text; ODS/ODT/Word/PDF, Importformate ODS/XML/MediaWiki und Serververzeichnisse fehlen; DUMP-001 offen |
| W-P29 Partitionen, Präfix-Stapel | 0.22 | umgesetzt | Review B; nur MySQL/MariaDB/PostgreSQL |
| W-P20 Konfigurationsspeicher, W-P21 Designer/QBE, W-P22 Transformationen, W-P23 Tracking/Zentralspalten/Normalisierung, W-P24 Diagramme, W-P25 Geodaten, W-P30 Teams/Anmeldeverfahren, W-P31 Produktinfo/Setup-Ersatz | nicht begonnen (kein Handoff, kein Commit) | nicht begonnen | – |
| W-P32 Paritätsaudit | nicht begonnen | – | – |

Die Web-Pins (Bündel 1 bis 5) sind in `main` gemergt: Bündel 1 (Shell/PWA, ersetzt durch Bündel 2 nach abgelehntem Review), 2 (Web `48c2f67`, API bis 0.7),
3 (Web `c14271e`, API 0.8/0.9/0.11), 4 (Web `f830326`, 17 Verträge, schlanke Validatoren) und 5 (Web `4b24e53`, 23 Verträge, Precache 4,32 MiB).
Offene Querschnittspunkte stehen in [STATUS](STATUS.md#offene-punkte-ehrliche-liste-stand-27102026).

Reihenfolge innerhalb einer Welle ist Vorschlag; Abhängigkeiten stehen je Paket. Wellen sind keine Gates: ein Paket
startet, sobald seine Abhängigkeiten abgenommen sind (Abschnitt 4.3).

### Welle 1: Fundament, PWA, erste Hebel

**W-P01 Backend-Registry und Fixture-Harness** (B, Sonnet; Haiku-Zuarbeit für Fixture-Daten)
- Matrix: keine (Voraussetzung für alle Backendpakete).
- API: keine neue Version. Nur Struktur: Registrierungsliste für Feature-Routen (`internal/httpapi/features.go`), damit neue
  Pakete nicht dieselbe Zeile in `server.go` ändern; gemeinsame Hilfen: Plan/Ausführen-Muster (SHA über Anweisungsliste,
  Bestätigung), Treiberfähigkeits-Typ, Identifier-Quoting-Hilfe aus validierten Metadaten pro Dialekt (Bestand
  wiederverwenden, nicht neu erfinden), Größenlimits je Routenklasse (für spätere Import-Uploads).
- Test-Harness: `scripts/dbfix.py` (Container MySQL 8.4, MariaDB 12.3, PostgreSQL 18.6 mit eigener CA, SQLite-Verzeichnis),
  Aufruf-Muster für Paketskripte; vorhandene `test-*.py` bleiben unverändert.
- Besitz: `internal/httpapi/features.go` (neu), kleine Änderung an `server.go` (einzige erlaubte, Registrierung der Liste),
  `internal/dbplan/` (neu), `scripts/dbfix.py`, `docs/handoffs/W-P01.md`.
- Abhängigkeit: keine. Blockiert alle späteren Backendpakete (nicht W-P02 bis W-P05).
- Abnahme: Bestehende Tests (`go test -race ./...`, Vertragswächter v1 bis v6) unverändert grün; neue Hilfen mit Unit-Tests;
  `dbfix.py` startet die drei Container und räumt nur eigene Ressourcen auf; ein Referenzlauf mit v6-Test.
- Risiken: Refactoring darf keine Vertragsbytes oder Routenverhalten ändern (Bytewächter). Kein Umbau der Mutationspfade.

**W-P02 Web-Shell und Routing** (W, Sonnet; Haiku für Textkatalog `de`)
- Matrix: `UX-007` (Grundlage Tastatur/Touch), `NEU-NAV-006`, `NEU-NAV-007` (Startseite mit W-P06).
- API: keine neue. Ziel: `App.svelte` in Shell zerlegen: Hash-Router (`#/server`, `#/db/<name>/struktur` usw.), Navigationsbereich
  (Server → Datenbank → Tabelle mit Reitern), Funktionsregistry `src/lib/features/index.ts` (Paket meldet Route, Reiter,
  Fähigkeitsschlüssel), Verbindungs- und Sitzungsstore aus `App.svelte` herausgelöst (weiterhin nur Arbeitsspeicher),
  zentraler Textkatalog `src/lib/i18n/de.ts`, Bestätigungs-Dialog-Komponente für zerstörende Aktionen (getippter Name),
  Plan-Vorschau-Komponente (SQL-Anzeige, Warnliste), Fehler-/Ladezustände, Tastaturkürzel-Grundlage.
- Besitz: `src/App.svelte`, `src/main.ts`, `src/lib/shell/**`, `src/lib/features/index.ts`, `src/lib/i18n/**`,
  `src/lib/components/shared/**`. **Alle bestehenden Komponenten bleiben fachlich unverändert**, sie werden nur in die
  Registry eingehängt.
- Abhängigkeit: keine. Blockiert alle Webpakete ab W-P05.
- Abnahme: Bestehende Vitest- und Playwright-Fälle (Mock und echt, SQLite/PG) unverändert grün; Deep-Link per Hash stellt
  Datenbank/Tabelle/Reiter her (Credentials werden nie in der URL geführt); 320-px-Layout; CSP-Test grün; keine neue
  Laufzeitabhängigkeit.
- Risiken: Regressionen im Sitzungs-/Tokenwechsel (Befund QA-F3-01 darf nicht zurückkehren). Kein Passwort/Token in
  Hash, Verlauf oder localStorage.

**W-P03 PWA-Basis** (W und I, Sonnet; Haiku für Icons und Texte)
- Matrix: keine (Produktanforderung dieses Plans).
- Web (Besitz `public/**`, `src/sw.ts` oder `public/sw.js`, `src/lib/pwa/**`, `index.html`-Kopf, Vite-Konfiguration): Manifest
  (`name` „DB Manager“, `short_name`, `lang: de`, `display: standalone`, `start_url: ./`, `scope: ./`, Farben), Icons 192/512 und
  maskable als PNG sowie SVG, Favicon, Apple-Touch-Icon; Service Worker (nur App-Shell: HTML und gehashte Assets,
  Cache-Name mit Buildkennung, `skipWaiting` nur nach Nutzerklick auf „Neue Version laden“; **`/api` und `/healthz` nie
  abfangen oder cachen**); Offline-Hinweis („Keine Verbindung zum Server“); Installationsangebot optional.
- Root/Integration (Besitz `internal/webui/handler.go`, `handler_test.go`, `scripts/build-web.py` Inventarregeln, `scripts/test-build-web.py`):
  erlaubte Dateien `manifest.webmanifest` (`application/manifest+json`), `sw.js` (`text/javascript`, `no-store`, `Service-Worker-Allowed` nicht
  erweitern), Icons (`image/png`, `image/svg+xml`); CSP um `manifest-src 'self'` und `worker-src 'self'` ergänzen; keine
  Lockerung von `script-src`/`default-src`. Inventar und Manifest (`assets-manifest.json`) um die neuen Dateitypen erweitern.
- Abhängigkeit: keine fachliche; Root-Teil nach Pin-Mechanik W-P04 integrieren (zeitlich gemeinsam).
- Abnahme: Chromium-Lauf: Manifest valide, SW aktiv, Installierbarkeit (Lighthouse-/Chrome-Installabilitätsprüfung oder
  Playwright-Check der Manifest- und SW-Kriterien), Neustart offline zeigt Shell und Hinweis, **kein API-Aufruf wird aus dem
  Cache beantwortet** (Test mit gegenüber Mock verändertem Token/Antwort), Update-Ablauf (neue Version erkannt, Hinweis, Reload),
  Go-Handlertests (MIME, 404 für Unbekanntes, Traversal weiter 404, Cache-Header), CSP-Test ohne Verstoß, Reproduzierbarkeit des
  Builds (zwei frische Builds bytegleich). Hinweis: Installierbarkeit verlangt HTTPS oder localhost; Betrieb hinter TLS-Proxy
  dokumentieren (BETRIEB), keine Behauptung für Klartext-HTTP.
- Risiken: SW-Caching von Antworten mit Token wäre ein Sicherheitsfehler (hartes Negativtest-Kriterium); veralteter SW nach
  Release (Cache-Version an Build-ID binden). Registrierung nur im Produktionsbuild.

**W-P04 Build-Lock-Generalisierung** (I, Sonnet; je Repo getrennte Arbeit, dann gemeinsamer Test)
- Matrix: keine (Voraussetzung für Parallelität).
- Root (Besitz `web-build.lock.json`, `scripts/build-web.py`, `scripts/test-build-web.py`, `docs/WEBBUILD.md`): Lock-Schema 2 mit
  Liste `contracts: [{version, sha256, family}]` statt fester Schlüssel; `build-web.py` iteriert; die fünf geprüften Bestandsverträge
  bleiben in der Liste byte-identisch; Rückwärtsprüfung gegen den heutigen Pin (Assets bleiben unverändert, kein Neubau
  verlangt).
- Web (Besitz `package.json` Skripte, `tests/generate-validators.mjs`): `contract:generate/check` und Validator-Generierung
  iterieren über `src/lib/contract/v*/`; keine Änderung generierter Bestandsbytes.
- Abhängigkeit: keine. Blockiert den ersten neuen Pin (alle Webpakete ab W-P05 integrieren erst danach).
- Abnahme: `python3 scripts/build-web.py --check`, `python3 -B scripts/test-build-web.py` (neue Fälle: fehlende/falsche
  Vertragsliste, Reihenfolge, doppelte Version), Web `npm run contract:check`, `npm test`; bytegleicher Neubau des bisherigen Pins
  (`8ef738d`) als Beweis, dass die Generalisierung nichts verändert.
- Risiken: Provenienzprüfung darf nicht aufgeweicht werden; ein fehlender Vertrag ist weiterhin harter Fehler.

**W-P05 Tabelle anlegen, Oberfläche zu API 0.6** (W, Sonnet; Haiku für Texte und Mock-Fixtures)
- Matrix: `OBJ-007`, `NEU-DDL-001` (reservierte Wörter, nur Hinweis), `DDL-005` (Teil: ENUM/SET bleibt für W-P08).
- API: v6 vorhanden (Backend `3d21e42`). Web kopiert Vertrag nach `src/lib/contract/v6/`, StructureClient-ähnlicher
  `CreateClient`, Komponente `src/lib/features/table-create/**`: Spaltenliste (feste Typen BIGINT, VARCHAR, Binary, Decimal laut
  Vertrag), Nullable, Defaults, PK-Auswahl, Vorschau der Strukturprüfung (`transactional/observed/different/unavailable`),
  bestätigter Abschluss, Ausgang „unbekannt“ ohne Retry-Knopf.
- Abhängigkeit: W-P02, W-P04 (Pin). Kein Backend-Anteil.
- Abnahme: Mock- und Echtfälle SQLite und PostgreSQL (MySQL/MariaDB über Backend-Tests v6 bereits belegt; Browser-Echtlauf dort
  soweit Umgebung); Sondernamen; 409 bei Namenskonflikt zerstört nichts; keine Anfrage ohne Bestätigung; Auswertung der
  Typabbildung wie im Vertrag (SQLite BIGINT-PK kein Rowid-Alias als Hinweistext).
- Risiken: Typ-/Defaultdarstellung nicht über den Vertrag hinaus versprechen.

**W-P06 Katalog, Übersicht, Definition, Serverinfo (API 0.7)** (B+W, Sonnet; Haiku für Texte und Fixtures)
- Matrix: `OBJ-001`, `OBJ-006`, `OBJ-014`, `ADMIN-001`, `ADMIN-008`, `ADMIN-009`, `CON-001` (Serverkontext sichtbar), `CON-004` (Zeichensatz
  anzeigen), `NEU-NAV-007`, `NEU-DDL-003`.
- Backend: neue lesende Routen: Datenbankliste mit Kollation und Größen/Objektzahl (Schätzwerte als Schätzung gekennzeichnet,
  exakte Zählung nur auf Nutzerwunsch mit Limit/Timeout), Tabellenübersicht (Typ Tabelle/View, Zeilen geschätzt, Engine, Kollation,
  Größe), Objektdefinition (SHOW CREATE bzw. katalogbasiert rekonstruiert; bei SQLite `sqlite_master`, PostgreSQL
  Rekonstruktion mit Kennzeichnung „abgeleitet“ wo kein natives Pendant), Serverinfo (Version, Laufzeit, Verbindungsdaten ohne
  Geheimnisse, Zeichensätze, Kollationen, Engines/Plugins). Treiberfähigkeiten.
- Web: `src/lib/features/overview/**`, `.../server-info/**`; Startseite, Datenbankliste, Tabellenübersicht, Definitionsansicht
  (nur lesen, kopierbar).
- Dateibesitz Backend: `internal/core/overview.go`, `internal/database/overview*.go`, `internal/httpapi/overview.go`,
  `contracts/api-v0.7.0.json`, `contracts/OVERVIEW-API.md`, `scripts/check-overview-contract.py`, `scripts/test-overview.py`,
  `.github/workflows/overview.yml`.
- Abhängigkeit: W-P01 (Backend), W-P02 (Web), W-P04 (Pin).
- Abnahme: je Treiber (SQ, MY, MA, PG) gegen Direktabfrage; Schätzwert vs. exakt unterscheidbar (OBJ-006); Datenbankwechsel in
  PostgreSQL/MySQL ohne neue Zugangsdatenübernahme in URLs; leere Datenbank; sehr viele Objekte (Limit, Paging); View vs.
  Tabelle; Definition einer Tabelle mit Sonderzeichen im Namen.
- Risiken: Informationsleck (nur sichtbare Objekte der Verbindung); teure Zählungen (Limits und Timeout), `information_schema`-
  Unterschiede MariaDB/MySQL.

### Welle 2: Struktur ändern

Gemeinsame Regeln: Plan/Ausführen-Muster (2.1), Bestätigung mit getipptem Namen bei zerstörenden Aktionen, Vorschau der SQL-Anweisungen,
Hinweis „DDL ist nicht pauschal transaktional“ (MySQL/MariaDB implizites Commit; SQLite/PostgreSQL transaktionale DDL), Folgen für
abhängige Objekte (Fremdschlüssel, Views, Trigger) vor Ausführung abfragen und anzeigen (`NEU-OBJ-001`). SQLite: Änderungen, die einen
Tabellen-Neuaufbau verlangen, sind in dieser Welle **nicht unterstützt** (Entscheidung E5); unterstützt sind ADD COLUMN, RENAME,
DROP COLUMN (Kapazität des eingebetteten SQLite prüfen), CREATE/DROP INDEX, DROP TABLE, RENAME TABLE.

**W-P07 Datenbank- und Tabellenoperationen (API 0.8)** (B+W, Sonnet)
- Matrix: `OBJ-002`, `OBJ-003`, `OBJ-004` (nur Kopie/Umbenennen innerhalb Server; Verschieben zwischen Servern entfällt), `OBJ-005`, `OBJ-008`, `OBJ-009`,
  `OBJ-010`, `OBJ-011`, `OBJ-012`, `NEU-OBJ-001`.
- Backend: Datenbank anlegen (Name, Kollation; SQLite entfällt, Fähigkeit „nein“), Datenbank löschen (nur MY/MA/PG, nie die
  Verbindungsdatenbank ohne Zweitbestätigung), Tabelle umbenennen, kopieren (Struktur/Daten/beides, Daten als
  `INSERT ... SELECT` ohne Client-SQL), leeren (TRUNCATE bzw. DELETE-Äquivalent; Auto-Inkrement-Folge ausweisen), löschen,
  Optionen (Engine, Kommentar, Kollation, Auto-Inkrement), Abhängigkeitsvorschau (Fremdschlüssel/Views).
- Web: `src/lib/features/object-ops/**` mit Aktionsleiste je Datenbank/Tabelle.
- Besitz: `internal/core/objectops.go`, `internal/database/objectops*.go`, `internal/httpapi/objectops.go`, `contracts/api-v0.8.0.json`,
  `contracts/OBJECT-OPS-API.md`, Web `src/lib/contract/v8/**`, `src/lib/features/object-ops/**`.
- Abhängigkeit: W-P01, W-P06 (Objektliste und Definition für Vorschau), W-P02.
- Abnahme: je Treiber; Abhängige Objekte werden korrekt gewarnt (FK, View); falscher Bestätigungsname verhindert die Aktion;
  Sonderzeichen im Namen; beim Löschen nur das Ziel verschwindet (Nachbartabelle bleibt); Kopie mit Daten erhält Zeilenzahl und
  Prüfsumme der Stichprobe; unbekannter Ausgang ohne Retry.
- Risiken: destruktiv. Zweistufige Bestätigung bei DB-Löschen; Umbenennen bricht Fremdschlüssel/Views (Warnung); `DROP DATABASE`
  nur mit explizitem Namen; kein Schutz vor Rechtefehlern vortäuschen.

**W-P08 Spalten ändern (API 0.9)** (B+W, Sonnet)
- Matrix: `DDL-001`, `DDL-002`, `DDL-003`, `DDL-004`, `DDL-005`, `DDL-006`, `NEU-DDL-002` (Spaltenkommentare: nur DB-eigene Kommentare, nicht
  pma-Speicher).
- Backend: Spalte hinzufügen (Position nach/vor, MY/MA), ändern (Name, Typ, Länge, NULL, Default, Kommentar, Kollation),
  umordnen (MY/MA), entfernen (mit Abhängigkeitsvorschau), ENUM/SET-Werte, generierte Spalten (Ausdruck; Ausdruck ist kein
  freies SQL vom Client im Sinne der Regeln, daher nur über einen engen Ausdrucksvalidator oder Entscheidung E5b: zunächst
  nicht unterstützt, wenn nicht sicher validierbar). Typkatalog je Dialekt aus einer festen Liste, nie freie Typzeichenfolgen.
- Web: `src/lib/features/column-edit/**` (Formular je Spalte, mehrere Spalten in einem Plan, Vorschau, Warnungen bei
  Datenverlust: Typverengung, NOT NULL auf vorhandenen NULLs).
- Besitz: wie W-P07 mit Namen `columns`, `contracts/COLUMNS-API.md`, Web `v9`.
- Abhängigkeit: W-P01, W-P06; teilt den Dialekt-Typkatalog mit W-P09 (Datei `internal/dbplan/types.go` gehört W-P08, W-P09
  liest nur).
- Abnahme: je Treiber Änderung nachlesen über Katalog (Typ, Länge, Nullable, Default, Position); Datenverlustwarnung wird
  ausgelöst und verlangt Bestätigung; PostgreSQL `ALTER COLUMN TYPE ... USING` nur mit vorgegebenem Cast aus fester Liste;
  SQLite nicht unterstütztes meldet Fähigkeit „nein“ statt Fehler.
- Risiken: Datenverlust, Locking großer Tabellen (Hinweis, kein stilles Warten; Timeout), Injektion über Typ-/Defaultfelder
  (feste Auswahllisten, Defaultliterale nur als typisierte Konstanten wie in API 0.6).

**W-P09 Indizes, Schlüssel, Fremdschlüssel (API 0.10)** (B+W, Sonnet)
- Matrix: `DDL-007`, `DDL-008`, `DDL-009`, `DDL-010`, `DDL-011`, `REL-001`, `NEU-REL-001`.
- Backend: Primärschlüssel setzen/entfernen, Index (Spaltenreihenfolge, Präfixlängen MY/MA), Unique, Volltext/Spatial nach
  Fähigkeit, Umbenennen/Entfernen, native Fremdschlüssel (ON UPDATE/DELETE, Referenzspaltenprüfung gegen Metadaten),
  Beziehungsübersicht je Tabelle. Prüfung doppelter Werte vor Unique/PK mit Anzahl-Hinweis.
- Web: `src/lib/features/index-edit/**` (erweitert die Strukturansicht um Aktionen, ohne `StructurePanel.svelte` zu ändern: Einhängen
  über Registry-Slot).
- Besitz: `indexes`, `contracts/INDEXES-API.md`, Web `v10`.
- Abhängigkeit: W-P08 (Typkatalog), W-P06.
- Abnahme: je Treiber; Index erscheint in v5-Strukturmetadaten (Nachlesen über die vorhandene Leseschnittstelle); Unique mit
  Duplikaten wird vorab erkannt und abgelehnt; FK auf nichtpassenden Typ abgelehnt; Index-Entfernen eines PK-gestützten FK
  gewarnt.
- Risiken: Sperren/Laufzeit, falsche Namen bei PostgreSQL (Schema).

**W-P10 Tabellenwartung (API 0.11)** (B+W, Sonnet; Haiku für Ergebnistexte)
- Matrix: `MAINT-001..006`.
- Backend: CHECK, ANALYZE, OPTIMIZE, REPAIR, CHECKSUM nach Treiber/Engine-Fähigkeit (SQLite: `PRAGMA integrity_check`,
  ANALYZE, VACUUM nur wenn ADR-0004-konform: kein `VACUUM INTO`; PostgreSQL: ANALYZE, VACUUM mit Optionen, `REINDEX` nach Fähigkeit),
  Stapel über mehrere Tabellen mit Ergebnis je Objekt, ehrliche Teilerfolge.
- Web: `src/lib/features/maintenance/**` (Mehrfachauswahl in Tabellenübersicht über Registry-Slot).
- Besitz: `maintenance`, `contracts/MAINTENANCE-API.md`, Web `v11`.
- Abhängigkeit: W-P06.
- Abnahme: je Treiber Ergebnis je Tabelle vollständig; Fehler einer Tabelle bricht die anderen nicht ab, Ergebnis nennt Teilerfolg;
  REPAIR nur wo die Engine es kennt; Laufzeitlimit.
- Risiken: Lange Locks auf Produktivdaten (Warnhinweis, Zeitlimit), `VACUUM FULL`/`OPTIMIZE` kopieren Tabellen.

### Welle 3: Daten rein und raus

Hinweis: Import und Export verlangen den gemeinsamen Exportvertrag C-Export1 ([ARBEITSPLAN](ARBEITSPLAN.md), zurückgestellt); er wird
Teil von W-P11 (Entscheidung E4) und gilt dann für Web und später Apple.

**W-P11 Export und SQL-Dump (API 0.12)** (B+W, Sonnet; Haiku für Optionstexte)
- Matrix: `EXP-001`, `EXP-002`, `EXP-004`, `EXP-006`, `EXP-007`, `EXP-008`, `EXP-009`, `DUMP-001`, `DUMP-002`, `DUMP-004`, `DUMP-005`,
  `NEU-EXP-001`, `NEU-EXP-002`, `NEU-EXP-004`, `NEU-ADMIN-007`, `NEU-DUMP-001`.
- Backend: Streaming-Download (kein 8-MiB-Antwortlimit, aber eigene Gesamtgrenze und Zeitlimit konfigurierbar): Umfang Server
  (nur lesbare DBs)/Datenbank/Tabellen/Abfrageergebnis; Formate SQL (Struktur, Daten, beides; DROP/IF NOT EXISTS; vollständige oder
  erweiterte INSERTs; Dialektwahl je Zielsystem; Binär als Hex; TIMESTAMP in UTC; Transaktions-/FK-Rahmen), CSV, Excel-CSV, JSON;
  Kodierung; Dateiname; GZip. Daten über gebundene Leseläufe, Struktur aus Katalog (kein `mysqldump`/`pg_dump`-Prozessaufruf,
  Entscheidung E4). Exportvertrag dokumentiert Treuegrenzen (kein universeller Roundtrip).
- Web: `src/lib/features/export/**` (Schnell/Angepasst, Tabellen-/Zeilenauswahl, Vorschau der Optionen, Download über Blob/Stream
  ohne Zwischenspeicher im DOM-Text; bestehender Ergebnisexport F6 bleibt).
- Besitz: `export`, `contracts/EXPORT-API.md`, Web `v12`; gemeinsamer Vertrag `contracts/EXPORT-FORMAT.md` (Format, nicht Route).
- Abhängigkeit: W-P01, W-P06, E4 (Limits).
- Abnahme: Dump je Treiber in **frischer Datenbank derselben Art** wiederherstellen und Zeilen-/Schemavergleich (Roundtrip für
  Teilmenge dokumentierter Typen); unabhängiger Leser für CSV/JSON; Sonderzeichen, NULL/Leer/Binär, große Zahlen, Unicode; Limit-
  und Abbruchfall liefert eindeutig unvollständig statt stiller Kürzung.
- Risiken: Speicher/Dauer (Streaming, Limits), Leck sensibler Daten in Logs (nie Zelleninhalt loggen), Formel-Injektion in CSV
  (Excel-CSV mit Präfixprofil wie F6), nicht ausgewählte DBs nicht anbieten.

**W-P12 Import SQL und CSV (API 0.13)** (B+W, Sonnet, mit Sicherheitsreview)
- Matrix: `IMP-001`, `IMP-002`, `IMP-004` (GZip/ZIP, falls Lizenz/Zeit; sonst Folgepaket), `IMP-005`, `IMP-006`, `IMP-007`, `IMP-008` (nur wenn Treiber es
  zulässt, sonst „nein“), `IMP-013`, `IMP-014`, `NEU-IMP-001..004`, `NEU-ADMIN-008`.
- Backend: Datei-Upload (eigener Routenklassen-Limit, Entscheidung E4: Vorschlag 64 MiB), SQL-Import mit eigenem Anweisungs-
  trenner (Strings, Kommentare, `DELIMITER`, `$$`-Quoting PG, Routinen), Ausführung anweisungsweise im Ziel-DB-Kontext mit
  Fortschritt/Fehlerposition, wahlweise Abbruch bei Fehler oder Weiterlaufen; FK-Prüfung steuerbar (`NEU-DATA-002` gehört hierher),
  CSV mit Trenner/Einfassung/Escape/Header/Zeichensatz/Spaltenzuordnung, Strukturerkennung zur neuen Tabelle über W-P05-Typen.
  Fortschritt: Antwort-Stream oder Pollroute; Wiederaufnahme nicht zugesagt (Matrixgrenze), nur Fehlerposition und Anzahl.
- Web: `src/lib/features/import/**` mit Drag-and-drop (`NEU-IMP-004`), Fortschritt, Fehlerbericht.
- Besitz: `import`, `contracts/IMPORT-API.md`, Web `v13`.
- Abhängigkeit: W-P11 (Format/Limits), W-P05 (Typen), W-P01.
- Abnahme: je Treiber SQL-Dump aus W-P11 und ein fremder Referenzdump (synthetisch) importierbar; CSV-Fixtures mit Anführungs-/
  Zeilenumbruchfällen; Fehler in Anweisung N liefert Position und lässt davor Ausgeführtes unverändert dokumentiert (kein
  Rollback-Versprechen für DDL); Überlänge und Fehlformate abgewiesen; Speichergrenzen unter Last getestet.
- Risiken: höchstes Sicherheitsrisiko der Welle: Import führt nutzerlieferiges SQL mit den Rechten der Verbindung aus (kein Sandbox-
  Versprechen, ausdrücklich dokumentiert), Zip-Bomben (Größen-/Verhältnislimit), Pfadangaben in Archiven, Zeichensatz-
  Verwechslung, Ressourcenverbrauch. SQLite: `ATTACH`/`VACUUM INTO` weiter durch Sandbox gesperrt.

**W-P13 Mehrzeilen, Dateiwerte, Fremdschlüssel-Auswahl (API 0.14)** (B+W, Sonnet)
- Matrix: `DATA-002` (Spaltenwahl, nur Web), `DATA-006`, `DATA-007`, `DATA-008`, `DATA-009`, `REL-003`, `NEU-DATA-001`, `NEU-DATA-006`.
- Backend: mehrere Zeilen ändern/löschen mit Teilerfolgsbericht je Zeile (auf bestehenden Eignungs-/Snapshotregeln, keine
  Massenänderung ohne Einzelsnapshots), Zeile duplizieren (Insert-Vorlage), Dateiwerte (Upload in BLOB/TEXT mit Größenlimit,
  Download Binärwert), Fremdschlüsselziel-Suche mit Anzeigewert, ENUM/SET-Auswahl.
- Web: `src/lib/features/row-bulk/**`, Spaltenwahl in Tabellenansicht über Registry-Slot.
- Besitz: `rows-bulk`, `contracts/ROWS-BULK-API.md`, Web `v14`.
- Abhängigkeit: Bestand v3/v4, W-P02.
- Abnahme: je Treiber; Teilerfolg wird zeilenweise korrekt berichtet (eine Zeile mit absichtlichem Konflikt); nachlesen; Bytes-
  treue Blobs; FK-Suche liefert nur lesbare Zeilen.
- Risiken: Teilerfolge, ABA/Trigger (Grenzen wie B2b), Blobgrößen.

**W-P14 Suche, Suchen und Ersetzen (API 0.15)** (B+W, Sonnet)
- Matrix: `DATA-010` (Rest: Bereich, LIKE, weitere Operatoren), `DATA-011`, `DATA-012`, `NEU-SQL-003` entfällt (Linter in W-P16).
- Backend: erweiterte Filteroperatoren (gebunden), datenbankweite Suche über gewählte Tabellen/Spalten (Limits, Timeout, nur
  textartige Spalten), Suchen/Ersetzen mit Vorschau (Trefferzahl und Beispiele) und bestätigter Ausführung je Tabelle/Spalte
  (UPDATE mit gebundenem Muster, nie Regex vom Client als SQL; Regex nur wo die Engine es nativ, fähigkeitsabhängig).
- Web: `src/lib/features/search/**`.
- Besitz: `search`, `contracts/SEARCH-API.md`, Web `v15`.
- Abhängigkeit: W-P06, W-P13 (Zeilenansicht für Treffer).
- Abnahme: Vorschau darf keine Zeile verändern (Prüfsumme vor/nach); Ersetzen betrifft genau die angezeigten Treffer; Groß-/
  Kleinschreibung je Kollation dokumentiert; je Treiber.
- Risiken: massenhafte Mutation: harte Obergrenze je Lauf, Bestätigung mit Trefferzahl, ehrlicher unbekannter Ausgang.

### Welle 4: SQL und Administration

**W-P15 SQL-Stapel, mehrere Ergebnisse, EXPLAIN, DML-Simulation (API 0.16)** (B+W, Sonnet)
- Matrix: `SQL-002`, `SQL-003`, `SQL-004` (Rest: Warnungen/Servercodes), `SQL-006`, `SQL-013`, `NEU-SQL-003`, `NEU-NAV-003` (Konsole).
- Backend: Stapelausführung mit Anweisungstrenner (gemeinsam mit W-P12 `internal/sqlsplit/`, Besitz W-P12; dieses Paket nutzt
  nur), je Anweisung eigener Status/Ergebnis/Warnungen; EXPLAIN je Treiber (nur auf die Anweisung, nie `EXPLAIN ANALYZE` ohne
  Bestätigung, da Ausführung); DML-Simulation = Zählung der betroffenen Zeilen per Hilfsabfrage in Transaktion mit Rollback
  (nur wo transaktional sicher, sonst Fähigkeit „nein“).
- Web: `src/lib/features/sql-tools/**`, Anbindung an den bestehenden SQL-Arbeitsbereich über Registry-Slot.
- Besitz: `sqlbatch`, `contracts/SQL-BATCH-API.md`, Web `v16`.
- Abhängigkeit: W-P12 (Trenner), W-P02.
- Abnahme: Stapel mit Fehler in Anweisung 3 meldet 1 und 2 erfolgreich, 3 Fehler, Rest nicht ausgeführt (oder wahlweise
  weiter); Routinenkörper mit Delimiter; mehrere Ergebnissätze (MySQL `CALL`, PG mehrere); DML-Vorschau verändert nichts (Nachlesen).
- Risiken: Nebenwirkungen auch bei „Vorschau“ (z. B. Trigger, Sequenzen): ausdrücklich dokumentieren.

**W-P16 SQL-Editor, Verlauf, Lesezeichen (nur Web, lokal)** (W, Sonnet; Haiku für Texte)
- Matrix: `SQL-005`, `SQL-008`, `SQL-009`, `SQL-010`, `SQL-011` (nur lokale Trennung privat/Export; Teilen entfällt bis Konten), `SQL-012`,
  `NEU-SQL-001`, `NEU-SQL-002`, `NEU-SQL-004`, `NEU-SQL-005`, `CON-006`.
- Web: Editor (CodeMirror 6, MIT, Lizenzinventar `docs/LIZENZEN.md` und Notices ergänzen; Entscheidung E8), Syntaxfarben,
  Vervollständigung aus dem Katalog (W-P06), Formatierung (feste, offline laufende Bibliothek, Lizenz prüfen),
  Verlauf/Lesezeichen/gespeicherte Suchen/Favoriten **browserlokal** (IndexedDB), nie mit Passwort/Token/Ergebnisdaten, Export/Import als
  JSON, Löschfunktion, Schalter „Verlauf speichern“ (Default aus, Entscheidung E1), Lesezeichenvariablen mit expliziter Eingabe.
- Besitz: `src/lib/features/sql-editor/**`, `src/lib/storage/**`.
- Abhängigkeit: W-P02, W-P06; bei serverseitiger Ablage später W-P20.
- Abnahme: Persistenz nur nach Opt-in; Löschen entfernt tatsächlich (IndexedDB geprüft); kein Credential im Speicher
  (Test, der den Speicher nach Verbindung durchsucht); CSP unverändert grün; Editor per Tastatur bedienbar.
- Risiken: Verlauf kann sensible SQL-Texte enthalten (daher Opt-in und Löschfunktion), Bibliothekslizenz.

**W-P17 Serverstatus, Variablen, Prozesse (API 0.17)** (B+W, Sonnet)
- Matrix: `ADMIN-002`, `ADMIN-003`, `ADMIN-004`, `ADMIN-005`, `NEU-ADMIN-001`, `NEU-ADMIN-005`.
- Backend: Statuszähler und Abfragetypen (MY/MA: `SHOW GLOBAL STATUS`; PG: `pg_stat_*`; SQLite: Fähigkeit begrenzt), Systemvariablen
  global/Sitzung lesen, berechtigt ändern (nur aus fester Namensliste dynamischer Variablen, nie freie Namen), Prozessliste und
  Beenden (`KILL`/`pg_terminate_backend`, nur gewählte ID), FLUSH-Aktionen aus fester Liste.
- Web: `src/lib/features/server-admin/**`.
- Besitz: `serveradmin`, `contracts/SERVER-ADMIN-API.md`, Web `v17`.
- Abhängigkeit: W-P06.
- Abnahme: synthetische Last verändert Zähler plausibel; Variable setzen und nachlesen; Prozess nur beenden, wenn ID und
  Benutzer bestätigt; fehlende Rechte liefern verständlichen Fehler; je Treiber.
- Risiken: Änderung globaler Variablen und KILL sind systemweit wirksam: Bestätigung mit Wert/Ziel, kein Selbst-Kill der
  eigenen Verbindung ohne Warnung.

**W-P18 Benutzer und Rechte der Datenbankkonten (API 0.18)** (B+W, Sonnet, mit Sicherheitsreview)
- Matrix: `PRIV-001..007`, `PRIV-009`, `PRIV-010`, `NEU-PRIV-001..003`.
- Backend: Konten mit Host auflisten (MY/MA `mysql.user`/`SHOW GRANTS`; PG Rollen), anlegen, Passwort ändern, kopieren/bearbeiten,
  entfernen, globale/Datenbank-/Tabellen-/Spalten-/Routinenrechte setzen und entziehen, Sperren/Entsperren, Rechte neu laden;
  `GRANT`-Anweisungen nur aus festen Berechtigungsnamen und validierten Objektnamen; Passwörter nur im Request, nie geloggt oder
  in Plan-Hash/Vorschau im Klartext (Vorschau maskiert). SQLite: Fähigkeit „nein“.
- Web: `src/lib/features/accounts/**`.
- Besitz: `accounts`, `contracts/ACCOUNTS-API.md`, Web `v18`.
- Abhängigkeit: W-P06, W-P17 (Fähigkeitsmuster).
- Abnahme: zwei Konten, eingeschränktes Konto sieht Aktionen nicht (Fähigkeit) und scheitert serverseitig ohne Rechte; vergebene
  Rechte per `SHOW GRANTS` nachgelesen; Entfernen eines Kontos mit aktiven Sitzungen; MariaDB-/MySQL-Authentifizierungs-
  unterschiede (Plugin) nur als feste Auswahlliste.
- Risiken: Rechteausweitung durch Fehlbedienung (Vorschau und Bestätigung), Passwortleck, Dialektunterschiede, Rollen
  vs. Benutzer in MariaDB/PG. Das Produkt selbst bleibt beim Bearer-Token, keine Produktkonten (siehe W-P30).

**W-P19 Views, Routinen, Trigger, Events (API 0.19)** (B+W, Sonnet)
- Matrix: `PROG-001..007`, `OBJ-014` (Ergänzung), `NEU-PROG-001`, `NEU-PROG-002`, `NEU-PROG-003` (Eventfehlerliste, nur lesend wo verfügbar).
- Backend: Views anlegen/ändern/löschen (Definition als SQL-Text ist hier unvermeidlich: als **Definition mit Plan-Vorschau und
  Bestätigung**, kein anderer SQL-Weg; Entscheidung E5c: Definitionen/Körper gelten als „Nutzer-SQL mit den Rechten der Verbindung“
  und sind entsprechend zu kennzeichnen), Prozeduren/Funktionen (Parameter, Eigenschaften, Körper, Aufruf mit Parametern),
  Trigger (Zeitpunkt/Ereignis/Körper), Events (Zeitplan/Status), jeweils Auflisten, Definition anzeigen, exportieren, löschen.
- Web: `src/lib/features/programs/**`.
- Besitz: `programs`, `contracts/PROGRAMS-API.md`, Web `v19`.
- Abhängigkeit: W-P15 (Trenner/Delimiter), W-P06.
- Abnahme: je Treiber (SQLite: Views und Trigger; keine Routinen/Events; PG: Funktionen/Trigger, Events entfallen, mit Fähigkeit
  „nein“); Routine mit Parametern aufrufen, mehrere Ergebnissätze; Event-Lauf nur in isolierter Fixture mit eingeschaltetem
  Scheduler; Löschen nur des Ziels.
- Risiken: Körper führt beliebigen Code aus (`DEFINER`/Sicherheitskontext sichtbar), Events laufen zeitversetzt weiter.

### Welle 5: Komfort und Spezialfunktionen

Diese Pakete sind einzeln kleiner, abhängig von Entscheidungen oder Bibliotheken und laufen nach Wellen 1 bis 4 frei
parallelisierbar im Rahmen der Agentobergrenze.

| Paket | Inhalt | Matrix / Neue Funde | Abhängigkeit | Besonderheit |
| --- | --- | --- | --- | --- |
| W-P20 Konfigurationsspeicher (v20) | Serverseitiger Speicher für Komfortdaten (Verlauf, Lesezeichen, Favoriten, Einstellungen, Layout) im Serverzustand nach ADR-0006, konto- bzw. nutzerbezogen; Migration der lokalen W-P16-Daten | `CON-006`, `SQL-008`, `SQL-009`, `UX-003`, `UX-004`, `UX-011`, `NEU-NAV-001`, `NEU-NAV-002`, `NEU-NAV-004`, `NEU-NAV-005`, `NEU-DATA-003`, `NEU-UX-002`, `NEU-UX-003`, `NEU-MAINT-007` | E1 (Ort), ADR-0006-Fragen, SZ1 | Nutzt `RequireState`; ohne Serverzustand bleibt W-P16-Lokalmodus aktiv |
| W-P21 Designer, Beziehungen, QBE | Interne Beziehungen, Integritätsprüfung, grafischer Designer (SVG, Tastatur), Layout speichern, Query-by-example, Mehrtabellenabfrage, Datenwörterbuch | `REL-002`, `REL-004..011`, `NEU-REL-002..004` | W-P09, W-P20 | Diagrammbibliothek vermeiden (eigenes SVG); Schemaexport PDF/SVG in W-P28 |
| W-P22 Transformationen | Spaltenzuordnung, Bild/Link/Text/IP/Regex/Binäransicht/Eingabeeditoren (SQL/JSON/XML), Vorschau | `TRANS-001..011`, `NEU-TRANS-001..033` | W-P20 | Externe Programme (`TRANS-010`, `Text_Plain_External`) nur nach Sicherheitsentscheidung, Default: nicht übernehmen |
| W-P23 Tracking, Zentralspalten, Normalisierung | Änderungsverfolgung nur für App-Änderungen, Berichte, Export, Zentralspalten, 1NF/2NF/3NF-Assistent | `META-001..008`, `NEU-META-001`, `NEU-META-002` | W-P20, W-P08, W-P09 | Tracking erfasst nur App-Änderungen (Matrixvorgabe) |
| W-P24 Diagramme | Ergebnisdiagramme, Optionen, Mehrfachserien | `UX-001`, `UX-002` | W-P02, Bibliothekswahl E8 | Lizenz vor Übernahme inventarisieren |
| W-P25 Geodaten | Geometrie anzeigen/bearbeiten, Zoom-Suche, Shapefile-Import | `DATA-013`, `DATA-014`, `IMP-012`, `NEU-DATA-004`, `NEU-DATA-005` | W-P13, W-P12, E7 | Keine externen Kachel-/Kartendienste (Default); Kartenbibliothek E8 |
| W-P26 Einstellungen, Themen, Sprachen, Tastatur, Touch | Themen (hell/dunkel), Spracheinstellung, weitere Sprachen (Haiku-Übersetzungen), RTL, Tastaturkürzel je Seite, Touch-Nachweis | `UX-005`, `UX-006`, `UX-007`, `NEU-UX-004`, `NEU-NAV-006` | W-P02, W-P20 | Haiku übersetzt aus `de.ts`, Sonnet prüft stichprobenartig; Vollständigkeit je Sprache ausweisen |
| W-P27 Monitor, Advisor, Replikation, Binlog, Profiling | Live-Messwerte, Abfrageanalyse, Advisor-Hinweise, Replikationsstatus und -steuerung, Binärlog, Profiling, Storage-Engine-Details | `ADMIN-006`, `ADMIN-007`, `ADMIN-010..012`, `SQL-007`, `PRIV-008`, `NEU-ADMIN-002`, `NEU-ADMIN-003`, `NEU-ADMIN-006`, `NEU-ADMIN-010..012` | W-P17, W-P24 | Replikationssteuerung nur in isoliertem Replikationspaar testen; Default: nur lesen (E9) |
| W-P28 Erweiterte Formate, Dump-Details | XML/ODS/ODT/Word/PDF/LaTeX/MediaWiki/PHP/YAML/Texy/CodeGen-Export, ODS/XML/MediaWiki-Import, DEFINER/Kompatibilität, Kommentare/Aliase, Views als Tabellen, Serververzeichnisse (nur nach Entscheidung) | `EXP-003`, `EXP-005`, `EXP-010..020`, `IMP-003`, `IMP-009..011`, `DUMP-003`, `DUMP-006..008`, `NEU-EXP-003` | W-P11, W-P12 | Je Format eigener Mini-Auftrag (Haiku für einfache Textformate mit Sonnet-Review); PDF/Office Bibliothekslizenz (E8) |
| W-P29 Partitionen, Präfix-Stapel | Partitionen definieren/warten/leeren/entfernen, Tabellenpräfix-Aktionen | `DDL-012..014`, `OBJ-013` | W-P07, W-P08 | Nur MY/MA/PG, SQLite „nein“ |
| W-P30 Anmeldeverfahren, Teams, Gruppen | Produktkonten, TOTP, Passkey-Login, Sitzungen, Teams/Freigaben, UI-Gruppen, Anmeldeprotokoll, IP-Regeln, Config-Anmeldung, CSRF/Sitzung | `UX-008..010`, `UX-012`, `SQL-011` (Teilen), `PRIV`-Abgrenzung, `NEU-SEC-001..003`, `NEU-SEC-005..009`, `NEU-SEC-011`, `NEU-UX-001` | ADR-0006-Fragen 1 bis 11 entschieden (E2), W-P20 | Größte Sicherheitsänderung: eigenes Bedrohungsmodell, doppelter Review |
| W-P31 Produktinfo, Setup-Ersatz, Fehlerberichte | Lizenz-/Änderungsprotokoll, Hilfelinks, Konfigurationsanzeige (nur lesen), Funktionsschalter; Fehlerbericht und Versionsprüfung nur nach Entscheidung | `NEU-MAINT-003..006`, `NEU-MAINT-008`, `NEU-ADMIN-004`, `NEU-ADMIN-009`, `NEU-SETUP-001..006` | E7, E10 | Bewusst entfallende Funktionen siehe unten |

Bewusst nicht übernommen (nur mit Nutzerentscheidung, Default: entfällt): `NEU-SEC-004` Referer-Schutz, `NEU-SEC-010` Zweitfaktor „Simple“,
`NEU-MAINT-001` phpinfo, `NEU-MAINT-002` Git-Revision, `NEU-ADMIN-004` Systemressourcen des PHP-Hosts (Host- statt DB-Werte). Diese
Entfall-Begründungen werden in der Matrix ausdrücklich dokumentiert (Entscheidung E10).

### Welle 6: Abschluss

**W-P32 Paritätsaudit und Release** (Lead + Reviewer, Sonnet; Haiku für Zählungen)
- Alle 181 Matrixzeilen und 119 neuen Funde auf Status `umgesetzt`, `teilweise`, `entfällt (begründet)` mit Nachweis (Test-ID, Fixture,
  Version, Befehl, Datum) prüfen; Lücken als neue Pakete; kombinierter Release (Images, Pages-Doku, Screenshots mit PWA-Install);
  PARITAET/STATUS/ARBEITSPLAN endgültig. Kein Releasepublish ohne ausdrückliche Nutzerfreigabe (E11).

### Zuordnungskontrolle

Bereits im Bestand teilweise umgesetzt und deshalb keinem neuen Paket zugeordnet (Weiterführung nur über die genannten Pakete):
`CON-002`, `CON-003`, `CON-005`, `CON-007`, `DATA-001`, `DATA-003`, `DATA-004`, `DATA-005`, `SQL-001` (Teilnachweise in [PARITAET](PARITAET.md)),
`NEU-DATA-007` und `NEU-DATA-008` (Inline-Gitter und Mehrspaltensortierung, im Web vorhanden). Ihr Nachweisrest (z. B. weitere
Datenbanksysteme im Browser) wird im Paritätsaudit W-P32 geschlossen.

Alle übrigen der 181 Matrix-IDs und 119 neuen Funde stehen in genau einem Paket der Abschnitte 3 oder in der Liste „Bewusst nicht
übernommen“. Das Ergebnis der maschinellen Prüfung steht am Dokumentende (Abschnitt 10).

## 4. Ablaufschema pro Paket und Parallelität

### 4.1 Stufen

| Stufe | Rolle | Ergebnis | Gate |
| --- | --- | --- | --- |
| S0 Auftrag | Lead | Auftragsblatt (ID, Ziel, Nichtziele, Repo, Branch/Worktree, erlaubte Dateien, API-Nummer, Abhängigkeiten, Abnahme, Implementierer, Reviewer) | |
| S1 Vertrag | Backend-Agent (oder Web-Agent bei reinen Webpaketen: entfällt) | `api-v0.N.0.json`, Verhaltensvertrag, Fähigkeitsmodell, **Fixtures für Antworten und Fehler** in `contracts/fixtures/<name>/` (Haiku-Zuarbeit) | **G1 Vertragsfreeze** nach kurzem Reviewer-Check (Sonnet), Vertragsbytes danach unverändert |
| S2a Backend | Backend-Agent | Core/DB/HTTP, 4-DB-Tests, Wächterskript, Workflow, `docs/handoffs/<ID>-backend.md` | |
| S2b Web | Web-Agent | Vertragskopie + Validatoren, Client, Komponenten, Mock- und Unit-Tests gegen die Vertragsfixtures; **parallel zu S2a** | |
| S3 Review Backend | Reviewer (nicht der Autor) | Befunde/Freigabe als `docs/handoffs/<ID>-backend-review.md`; Fixes durch Autor, Nachprüfung durch Reviewer | **G2 Backend angenommen** |
| S4 Review Web | Reviewer (nicht der Autor) | Befunde/Freigabe im Web-Repo (`docs/handoffs/<ID>-review.md`) | **G3 Web angenommen** |
| S5 Integration | Integrations-Agent | Backend nach main (nach Leadfreigabe), Web-Commit pinnen, `build-web.py` Neubau, Manifest, `test-web-integration.py` (echter Browser→Go→DB), Compose-Prüfung; zweiter frischer Build bytegleich | **G4 Bündel integriert** |
| S6 Integrationsreview | Reviewer | Source-/Artefakt-/Lock-/Pinprüfung und Stichprobe der UI | |
| S7 Dokumentation | Doku-Agent (Haiku für Gerüste, Sonnet für Statuszeilen) | Handoffs, [PARITAET](PARITAET.md)-Teilnachweise, STATUS/ARBEITSPLAN-Zeilen, Vertragsindex, BETRIEB/WEBBUILD-Nachträge; `scripts/check-docs.py` grün | |

### 4.2 Parallelitätsregeln (Rechner begrenzt)

- **Höchstens 4 gleichzeitige Agents**, bevorzugt 3: typisch 1 bis 2 schreibende Backend-Agents, 1 Web-Agent und 1 Reviewer/Integrations-/Doku-Agent.
- **Builds nacheinander:** `build-web.py`, `npm ci`/`vite build`, `test-web-integration.py`, DinD und Compose-Läufe führt nur der
  Integrations-Agent aus oder der Autor nach Absprache; es läuft nie mehr als ein Build gleichzeitig, DB-Suiten höchstens zwei, DinD exklusiv.
  Lead führt eine Warteschlange („Buildslot“) im Auftragsblatt.
- **Ein Pin pro Bündel:** Mehrere Webpakete werden im Web-main gesammelt; Pin/Neubau/Integration erfolgt einmal je Bündel (Ende einer Welle oder
  nach drei angenommenen Paketen), nicht je Paket. Der Webmain trägt dazwischen nur geprüfte, aber nicht gepinnte Quellen.
- **Reine Webpakete** (W-P05, W-P16 usw.) brauchen S1/S2a nicht; **reine Backendpakete** brauchen nur S2a/S3 (Bündel S5 später).
- **Haiku-Pakete** laufen in Lücken und blockieren nie die Slots der Sonnet-Pakete.

### 4.3 Beispielhafte Zeitscheiben für Welle 1 und 2

| Scheibe | Slot 1 | Slot 2 | Slot 3 | Slot 4 |
| --- | --- | --- | --- | --- |
| T1 | W-P01 Backend | W-P02 Web-Shell | W-P04 Lock (Root-Teil) | Haiku: Icons, de-Texte, Mock-Fixtures |
| T2 | W-P06 S1 Vertrag + S2a | W-P03 PWA (Web-Teil) | W-P04 Web-Teil | Review W-P01 / W-P02 |
| T3 | W-P06 S2a/S2b | W-P05 Tabelle anlegen (Web) | Review W-P04, W-P03 | Integration Bündel A (W-P02..W-P05, ohne Buildkonflikte nacheinander) |
| T4 | W-P07 Backend | W-P08 Backend | W-P06 Web / W-P07 Web | Review W-P06 |
| T5 | W-P09 Backend | W-P10 Backend | W-P08 Web | Integration Bündel B (W-P06, W-P07) |
| T6 | Reviews W-P07..W-P10 | Webs W-P09, W-P10 | Integration Bündel C | Doku |

Zeitscheiben sind Reihenfolgen, keine Zusage über Dauer.

### 4.4 Abnahme je Paket (Definition of Done)

Abnahmekriterien der Matrixzeilen erfüllt; Tests je DB-System (2.3) mit konkretem Befehl und Ausgang im Handoff; unabhängige Reviews
ohne offene Befunde; Lizenzen/Notices bei neuen Abhängigkeiten aktualisiert (kein Fremdcode aus GPL-phpMyAdmin kopiert;
Funktionsbeschreibungen sind eigene Formulierungen); Dokumentation aktualisiert; keine Secrets oder privaten Pfade; nicht ausgeführte
Prüfungen ausdrücklich offen. Eine frische Session muss fortsetzen können (AGENTS).

## 5. Dateibesitz und Konfliktvermeidung

| Bereich | Besitzer | Regel |
| --- | --- | --- |
| `internal/core/<name>.go`, `internal/database/<name>*.go`, `internal/httpapi/<name>*.go`, `contracts/api-v0.N.0.json`, `contracts/<NAME>-API.md`, `contracts/fixtures/<name>/`, `scripts/check-<name>-contract.py`, `scripts/test-<name>.py`, `.github/workflows/<name>.yml`, `docs/handoffs/<ID>*.md` | das jeweilige Backendpaket | nur eigene Dateien; neue Datei statt Änderung gemeinsamer Dateien |
| `internal/httpapi/features.go`, `internal/dbplan/**`, `scripts/dbfix.py` | W-P01, danach nur per Änderungswunsch an den Lead | Änderungen als kleine Zusatzpakete, nie parallel |
| `internal/dbplan/types.go` | W-P08 | W-P09 liest nur |
| `internal/sqlsplit/**` | W-P12 | W-P15, W-P19 lesen nur |
| `src/lib/features/<name>/**`, `src/lib/contract/vN/**`, `tests/browser/<name>.spec.ts`, `tests/real/<name>.spec.ts` (Web) | das jeweilige Webpaket | nur eigene Verzeichnisse |
| `src/App.svelte`, `src/lib/shell/**`, `src/lib/features/index.ts`, `src/lib/i18n/de.ts` (nur Anhängen eigener Schlüssel unter `<name>.*` erlaubt; Konflikte löst der Lead beim Merge), `src/lib/components/shared/**` | W-P02, danach Lead | Webpakete melden Registry-Einträge in ihrem Paketverzeichnis (`feature.ts`), die Registry liest sie per `import.meta.glob`; keine manuelle Pflege der Liste |
| `public/**`, `src/lib/pwa/**`, `index.html`, `vite.config.ts` | W-P03 | danach Lead |
| `package.json`, `package-lock.json`, `tests/generate-validators.mjs` | W-P04 (Skripte), jede neue Abhängigkeit nur als eigenes, vom Lead genehmigtes Mini-Paket (Lizenz und Lock-SHA) | nie parallel |
| `web-build.lock.json`, `scripts/build-web.py`, `scripts/test-build-web.py`, `internal/webui/**` (inkl. Assets/Manifest) | Integrations-Agent | einziger Schreiber; Pins nur nach G3 |
| `contracts/README.md` (Index), `docs/STATUS.md`, `docs/ARBEITSPLAN.md`, `docs/PARITAET.md`, `docs/TEAM.md`, `docs/PWA-PLAN.md` | Lead / Doku-Agent | Worker liefern Handoffs; Lead schreibt zentrale Dateien |

Worktrees: `.worktrees/<ID>-be`, `.worktrees/<ID>-web` (im jeweiligen Repo), Branch `feat/<ID>-<name>` bzw. `review/<ID>`;
ein Worktree je Agent. Merge nach main nur durch Lead/Integration nach den Gates.

## 6. Entscheidungen des Nutzers (mit Empfehlung und Default)

Bis zur Entscheidung gilt der Default; er ist so gewählt, dass nichts Unumkehrbares oder Datenschutzkritisches passiert. Mit dem Default
kann gearbeitet werden; Pakete, die vom Gegenteil abhängen, sind unten genannt.

| ID | Frage | Empfehlung | Default bis Entscheidung | Betroffen |
| --- | --- | --- | --- | --- |
| E1 | Wo liegen Verlauf, Lesezeichen, Einstellungen? | Zunächst browserlokal (opt-in), später serverseitig im Serverzustand nach ADR-0006 | Browserlokal, Verlauf standardmäßig aus, nie Passwörter/Token; Export/Löschen vorhanden | W-P16, W-P20 |
| E2 | Produktkonten, 2FA, Passkeys, Teams (ADR-0006, Fragen 1 bis 11)? | Erst nach Wellen 1 bis 4 entscheiden; bis dahin Bearer-Token wie heute | Bearer-Token, keine Produktkonten, kein Teilen | W-P30, `SQL-011` |
| E3 | Zielversionen für die „unterstützt“-Aussage? | Fixtureversionen (MySQL 8.4, MariaDB 12.3, PostgreSQL 18.6, SQLite des Servers) als verbindlich; ältere LTS (MySQL 8.0, MariaDB 10.11, PG 14+) „best effort“ mit Feature-Erkennung | Nur Fixtureversionen zugesagt, Rest ohne Zusage | alle Pakete (Matrix-Grenze 3) |
| E4 | Import/Export: Grenzen und Wege? | Nur Browser-Upload/-Download; Import bis 64 MiB, Export gestreamt mit konfigurierbarer Obergrenze; keine serverseitigen Verzeichnisse, kein Aufruf von `mysqldump`/`pg_dump`; Katalog-basierter Dump | Wie Empfehlung | W-P11, W-P12, W-P28 |
| E5 | SQLite-Strukturänderungen mit Tabellen-Neuaufbau und freie Ausdrücke (generierte Spalten, View-/Routinenkörper)? | SQLite-Neuaufbau später als eigenes opt-in Paket mit Sicherung; Ausdrücke/Körper als gekennzeichnetes „Nutzer-SQL“ mit Vorschau und Bestätigung, ohne Parser-Versprechen | Neuaufbau nicht unterstützt; Körper/Definitionen nur über Plan und Bestätigung; generierte Spalten zunächst nicht | W-P08, W-P19 |
| E6 | Destruktive Aktionen: Bestätigungsmodell und optionaler Schreibschutz je Verbindung? | Getippter Zielname für DROP/TRUNCATE/DELETE-Massenaktionen, Schalter „Nur lesen“ je Verbindung | Bestätigung wie beschrieben; Schalter „Nur lesen“ vorhanden, standardmäßig aus | W-P07 bis W-P19 |
| E7 | Ausgehende Verbindungen (Versionsprüfung, Fehlerbericht, Captcha, Kartenkacheln, Hilfe-Links)? | Keine automatischen Verbindungen nach außen; Links nur auf Nutzerklick | Keine Außenverbindungen; Karten ohne Kacheln | W-P25, W-P31 |
| E8 | Neue Bibliotheken (Lizenzen im Inventar)? | CodeMirror 6 (MIT) für Editor; Diagramme, Karten, PDF/Office erst vor dem jeweiligen Paket einzeln freigeben | Nur CodeMirror 6 vorab freigegeben; übrige Pakete warten | W-P16, W-P24, W-P25, W-P28 |
| E9 | Server-Eingriffe mit Systemwirkung (Variablen setzen, KILL, Replikation steuern, Rechte)? | Freigeben mit Plan/Bestätigung; Replikation steuern nur lesen, bis ein isoliertes Testpaar belegt ist | Setzen/KILL/Rechte ja mit Bestätigung; Replikationssteuerung nur lesend | W-P17, W-P18, W-P27 |
| E10 | Bewusst nicht übernommene Funktionen (phpinfo, Git-Revision, Referer-Schutz, Zweitfaktor „Simple“, Host-Ressourcen, externe Transformationsprogramme)? | Entfallen mit dokumentierter Begründung; „alle Funktionen“ heißt dann „alle sinnvoll übertragbaren“ | Entfallen | W-P22, W-P31, W-P32 |
| E11 | Veröffentlichung: wann wird nach main gemergt, gebaut und veröffentlicht? | Merge nach main je Bündel durch Lead nach Gates; Images und Pages nur mit ausdrücklicher Freigabe je Release | Merge nach Gates nur auf Leadfreigabe; kein automatischer Publish | alle |
| E12 | Name und Icon der PWA („DB Manager“ vs. „GoMyAdmin“) | „DB Manager“ beibehalten | „DB Manager“, generisches neutrales Icon (Haiku, SVG) | W-P03 |
| E13 | PWA-Betrieb: HTTPS-Pflicht und Offline-Umfang | Installierbarkeit nur hinter HTTPS/localhost dokumentieren; Offline nur App-Shell, nie Daten | Wie Empfehlung | W-P03 |

## 7. Risiken (Querschnitt)

1. **Parallelkonflikte durch zentrale Dateien:** deshalb W-P01, W-P02, W-P04 vor allen anderen Paketen; Regeln in Abschnitt 5.
2. **Vertragsvermehrung:** 13 neue Vertragsversionen (0.7 bis 0.19) in Welle 1 bis 4, weitere in Welle 5; Lock/Build-Pin müssen mithalten (W-P04), Bündelung der Pins.
3. **SQL-Injection und Identifier-Quoting:** nur Metadaten-validierte Identifier, feste Listen (Typen, Rechte, Variablen, Optionen),
   gebundene Werte, keine freien Fragmente; Nutzer-SQL (Import, Körper, Editor) ist ausdrücklich „Nutzer-SQL mit Verbindungsrechten“.
4. **Zerstörende Aktionen:** Plan, getippte Bestätigung, unbekannter Ausgang ohne Retry, Negativtests.
5. **Rechtegrenzen:** UI-Ausblendung ist keine Sicherheit; serverseitig entscheiden die DB-Rechte. Keine Produktkonten bis E2.
6. **Dialektunterschiede:** Fähigkeitsabfrage je Treiber, „nicht unterstützt“ ist sichtbarer Zustand und Testfall.
7. **Service Worker:** darf nie API-Antworten oder Token zwischenspeichern; Update-Strategie ohne Datenverlust im Editor.
8. **Ressourcen:** Builds und DB-Container nacheinander, gemeinsamer `dbfix`-Harness, Aufräumen nur eigener Ressourcen.
9. **Lizenzen:** neue Abhängigkeiten vor Einführung im Inventar; phpMyAdmin-Code (GPL-2.0) wird nicht kopiert.
10. **Parität ist nicht Behauptung:** `teilweise` bleibt `teilweise`, bis die Matrixabnahme (Test-ID, Version, Befehl, Datum) vorliegt.

## 8. Anhang A: Agent-Prompts für Welle 1

Gemeinsamer Kopfblock für **jeden** Prompt (wird vor den paketspezifischen Teil kopiert):

```text
Du arbeitest im Projekt DB Manager (Go-Backend im Root-Repo, Svelte/shadcn-svelte-Web in db-manager-web).
Zuerst lesen: AGENTS.md, docs/ENTWICKLUNGSABLAUF.md, docs/STATUS.md, docs/PWA-PLAN.md (Abschnitte 1, 2, 4, 5 und dein Paket
in Abschnitt 3), docs/PARITAET.md (nur deine Matrixzeilen) und die Bereichsdateien in docs/pma-inventar/ zu deinen IDs.
Regeln: Dokumentation Deutsch, Code/Tests/Kommentare/technische Bezeichner Englisch. Eigener Worktree und Branch, nur die
unten genannten Dateien schreiben. Keine Attribution-Zeilen in Commits, Git-Identität nicht ändern, nichts nach main
mergen, keine Secrets oder privaten Pfade, kein Code aus phpMyAdmin (GPL) übernehmen. Nur eigene kurzlebige Container, Volumes
und Netze mit eindeutigem Präfix; fremde Ressourcen und Host-Swarm nie anfassen; vor dem Start prüfen, ob schwere Builds oder
DB-Suiten laufen (höchstens zwei DB-Suiten, ein Build). Vor Abschluss: alle genannten Prüfungen tatsächlich ausführen und mit
Befehl und Ausgang im Handoff nennen; nicht Ausgeführtes ausdrücklich offenlassen. Abnahmekriterien und Risiken aus PWA-PLAN
gelten verbindlich. Ergebnis: Commits auf deinem Branch, Handoff docs/handoffs/<ID>.md (Ergebnis, Dateien/Commit, Prüfungen mit
Ausgang, Einschränkungen, Findings, nächster Schritt). Berichte knapp: Branch, SHA, Prüfstand, offene Punkte.
```

### A.1 W-P01 Backend-Registry und Fixture-Harness (Backend-Agent, Sonnet)

```text
[Kopfblock]
Paket W-P01, Repo db-manager (Root), Worktree .worktrees/w-p01, Branch feat/w-p01-registry, Basis aktueller main.
Ziel: Voraussetzungen für parallele Backendpakete schaffen, ohne Verhalten zu ändern.
Aufgaben:
1. internal/httpapi/features.go: Registrierungsliste (Funktionen vom Typ func(mux *http.ServeMux, a *API)), die server.go
   einmal aufruft; bestehende Routen v1 bis v6 werden dort in derselben Reihenfolge registriert, ohne Pfad-/Auth-/Statusänderung.
2. internal/dbplan/ (neues Paket): Typen für Plan/Ausführen (Anweisungsliste je Treiber, normalisierter SHA-256 mit
   Domain-Präfix, Bestätigungsobjekt mit Aktion und Zielname, Ergebnisstatus completed/rejected/outcome_unknown),
   Fähigkeitstyp je Treiber und Aktion (supported/unsupported/limited mit Grund), Hilfen für das Quoting von Identifiern aus
   bereits validierten Metadaten (vorhandene Funktionen aus internal/database wiederverwenden, nicht duplizieren).
3. Routenklassen-Limits: Konfigurierbare Request-Body-Obergrenze je Routenklasse (Standard bleibt 1 MiB), damit spätere
   Import-Pakete eine größere Klasse erhalten; keine Verhaltensänderung für v1 bis v6.
4. scripts/dbfix.py: wiederverwendbare Fixture-Bibliothek (MySQL 8.4, MariaDB 12.3, PostgreSQL 18.6 mit eigener CA und
   verify-full, SQLite-Verzeichnis, Präfix dbmanager-<paket>-<hex>, Aufräumen nur eigener Ressourcen, Healthcheck) nach dem Muster
   von scripts/test-create.py; test-create.py bleibt unverändert, ein neues scripts/test-dbfix.py belegt die Bibliothek mit einem
   v6-Referenzfall.
5. Handoff docs/handoffs/W-P01.md mit Konventionen für nachfolgende Pakete (Dateinamen, Registrierung, Planmuster).
Erlaubte Dateien: internal/httpapi/features.go (neu), internal/httpapi/server.go (nur der Aufruf der Liste), internal/dbplan/**,
scripts/dbfix.py, scripts/test-dbfix.py, zugehörige Tests, docs/handoffs/W-P01.md. Keine Vertrags-, Web-, Docker-, go.mod-Änderung.
Abnahme: go test -race -count=1 ./... und go vet ./... grün; alle bestehenden Vertragswächter (scripts/check-*-contract.py) und die
Bytevergleiche v1 bis v6 unverändert grün; python3 scripts/test-dbfix.py grün (Container danach entfernt, nachweisen);
python3 scripts/check-docs.py grün. Reviewer: anderer Backend-Agent prüft, dass kein Routenverhalten geändert wurde.
```

### A.2 W-P02 Web-Shell und Routing (Web-Agent, Sonnet)

```text
[Kopfblock]
Paket W-P02, Repo db-manager-web, Worktree .worktrees/w-p02, Branch feat/w-p02-shell, Basis aktueller main (f75bfa1 oder neuer).
Ziel: App.svelte in eine erweiterbare Shell zerlegen, damit spätere Funktionspakete parallel in eigenen Verzeichnissen arbeiten.
Aufgaben:
1. Hash-Router (#/..., ohne neue Abhängigkeit): Server, Datenbank, Tabelle, Reiter (Daten, Struktur, SQL); Deep-Link stellt Auswahl
   her; nie Zugangsdaten, Token oder SQL in der URL.
2. src/lib/shell/: Layout (Navigation links, Hauptbereich, mobil 320 px mit Einblendmenü), Verbindungs-/Sitzungsstore aus App.svelte
   herausgelöst (nur Arbeitsspeicher, Verhalten des Token- und Verbindungswechsels unverändert; QA-F3-01 darf nicht zurückkehren).
3. Funktionsregistry src/lib/features/index.ts: jedes Paket legt src/lib/features/<name>/feature.ts ab (Route, Reiter, Slot, benötigte
   Fähigkeit, Titelschlüssel); Registry liest per import.meta.glob; Slots für Tabellenreiter, Tabellenaktionsleiste, Datenbankreiter,
   Serverreiter. Bestehende Komponenten (TableWorkbench, RowWorkspace, StructurePanel, ExportControls, ResultTable) bleiben fachlich
   unverändert und werden nur über die Registry eingehängt.
4. src/lib/i18n/de.ts: zentraler deutscher Textkatalog (nur Struktur und die schon vorhandenen Texte; Haiku-Agent übernimmt später die
   Verschiebung weiterer Texte).
5. src/lib/components/shared/: ConfirmDestructiveDialog (getippter Zielname, Fokus, Tastatur), PlanPreview (SQL-Anzeige schreibgeschützt,
   Warnliste, plan_sha256 sichtbar), CapabilityNotice (nicht unterstützt mit Grund), ErrorPanel.
Erlaubte Dateien: src/App.svelte, src/main.ts, src/lib/shell/**, src/lib/features/index.ts, src/lib/i18n/**, src/lib/components/shared/**,
zugehörige Tests unter src/ und tests/browser/shell.spec.ts, docs/handoffs/W-P02.md. Keine Vertrags-, package.json-, vite-Änderung.
Abnahme: npm run check, npm test, npm run format:check, npm run test:browser, npm run test:csp und, falls Umgebung verfügbar,
python3 scripts/test-real-tables.py (SQLite und PostgreSQL) bestehen unverändert; neue Tests für Router (Deep-Link, ungültige Hashes,
Browser-Zurück), Registry (Slots, fehlende Fähigkeit), Dialogkomponenten (falscher Name verhindert Bestätigung); 320-px-Screenshot
ohne horizontales Scrollen; keine neue Laufzeitabhängigkeit. Reviewer: anderer Web-Agent.
```

### A.3 W-P03 PWA-Basis (Web-Agent Teil 1, Integrations-Agent Teil 2; Sonnet, Haiku für Icons)

```text
[Kopfblock]
Paket W-P03. Teil 1 Repo db-manager-web (Worktree .worktrees/w-p03, Branch feat/w-p03-pwa). Teil 2 Repo db-manager (Worktree
.worktrees/w-p03-embed, Branch feat/w-p03-embed). Teil 2 darf erst nach Teil-1-Review und nach W-P04 integriert werden; Teil 1 und der
Go-Handler-Entwurf von Teil 2 starten parallel.
Ziel: Die Oberfläche wird installierbare PWA mit Offline-Shell, ohne dass API-Daten oder Token jemals zwischengespeichert werden.
Teil 1 (Web): public/manifest.webmanifest (name "DB Manager", short_name, lang de, display standalone, start_url und scope relativ,
theme/background color passend zu app.css), Icons 192 und 512 PNG, maskable PNG, SVG-Icon, favicon, apple-touch-icon (neutrales
generisches Symbol, eigene Erstellung, kein Fremdmaterial); link- und meta-Tags in index.html; Service Worker (App-Shell: index.html
und gehashte Assets; Cache-Name aus Build-ID; fetch nur für GET same-origin auf diese Dateien; /api, /healthz, /readyz und alles andere
bleiben ungecacht und unverändert durchgereicht; Aktivierung neuer Versionen erst nach Nutzerklick "Neue Version laden");
src/lib/pwa/register.ts nur im Produktionsbuild, Offlinehinweis "Keine Verbindung zum Server"; Vite-Konfiguration so, dass sw.js und
Manifest ohne Sourcemaps mit stabilem Namen erzeugt werden und die bestehende Quell-Provenienzprüfung greift.
Teil 2 (Root): internal/webui/handler.go: zusätzlich erlaubte Dateien manifest.webmanifest (application/manifest+json), sw.js
(text/javascript, Cache-Control no-store), Icons (image/png, image/svg+xml), exakte Namensliste statt Wildcards; CSP um manifest-src
'self' und worker-src 'self' ergänzen, sonst unverändert (kein unsafe-eval, kein Inline-Script); handler_test.go entsprechend;
scripts/build-web.py (ROOT_FILES/Inventarregeln) und scripts/test-build-web.py erweitern; docs/BETRIEB.md: Hinweis HTTPS/localhost.
Erlaubte Dateien: Web: public/**, src/lib/pwa/**, index.html, vite.config.ts, tests/browser/pwa.spec.ts, tests/csp/**; Root: die genannten
Go-/Skript-/Doku-Dateien und docs/handoffs/W-P03*.md. Keine Fachfunktion.
Abnahme: Web: npm run check, test, test:browser, test:csp; Playwright-Fälle: Manifest lädt und ist valide, SW wird aktiv, App startet nach
Offline-Schalten mit Shell und Hinweis, ein API-Aufruf wird bei Offline nie aus dem Cache beantwortet, ein API-Aufruf mit geändertem Token
geht immer ans Netz, Update-Hinweis erscheint bei neuem Build. Root: go test -race ./internal/webui, go vet, python3 -B
scripts/test-build-web.py, nach Pin python3 scripts/build-web.py --check; zwei frische Neubauten bytegleich. Installierbarkeit in
Chromium belegen (Installabilitäts-Kriterien per Playwright/CDP oder Lighthouse; Ausgang im Handoff). Reviewer: anderer Agent mit
Sicherheitsfokus auf Cache-Verhalten.
```

### A.4 W-P04 Build-Lock-Generalisierung (Integrations-Agent, Sonnet)

```text
[Kopfblock]
Paket W-P04, Repos db-manager (Worktree .worktrees/w-p04, Branch feat/w-p04-lock) und db-manager-web (Worktree .worktrees/w-p04,
Branch feat/w-p04-contracts). Beide Teile starten parallel, gemeinsamer Test am Ende.
Ziel: API-Vertragspins skalieren ohne feste Schlüsselnamen; Provenienzprüfung bleibt streng.
Root: web-build.lock.json auf schema_version 2 mit contracts: [{family, version, sha256}] (mindestens die fünf Bestandsverträge 0.1 bis
0.5 mit identischen SHAs wie heute; 0.6 bleibt zunächst ungepinnt, weil das Web ihn nicht konsumiert); scripts/build-web.py liest und
prüft die Liste sowohl gegen contracts/api-v<version>.json im Root als auch gegen src/lib/contract/<dir>/api-v<version>.json im Web-
Checkout; doppelte, fehlende oder abweichende Einträge sind harte Fehler; Rückwärtsprüfung gegen alten Pin. scripts/test-build-web.py
um negative Fälle erweitern. docs/WEBBUILD.md anpassen (Beschreibung, SHA-Tabelle bleibt).
Web: package.json: contract:generate/check und validators:generate/check iterieren über alle src/lib/contract/v*/ (Skript statt
Fünfer-Kette); tests/generate-validators.mjs entsprechend; generierte Bestandsdateien bleiben byte-identisch.
Erlaubte Dateien: Root web-build.lock.json, scripts/build-web.py, scripts/test-build-web.py, docs/WEBBUILD.md, docs/handoffs/W-P04.md;
Web package.json, tests/generate-validators.mjs, ggf. ein kleines scripts/contracts.mjs, docs/handoffs/W-P04.md. Keine Produktdateien,
keine Assets, keine Lockfile-Änderung des npm-Locks.
Abnahme: python3 scripts/build-web.py --check und python3 -B scripts/test-build-web.py grün; Web npm run contract:check, npm test, npm run
build erzeugen dieselben Artefakte wie zuvor (bytegleich zum Pin 8ef738d, per frischem Neubau nachweisen: das ist einer der drei
Buildläufe, nicht parallel zu anderen); go test -race ./internal/webui. Reviewer: unabhängiger Integrations-/Sicherheitsreviewer.
```

### A.5 W-P05 Tabelle anlegen, Oberfläche zu API 0.6 (Web-Agent, Sonnet)

```text
[Kopfblock]
Paket W-P05, Repo db-manager-web, Worktree .worktrees/w-p05, Branch feat/w-p05-table-create. Voraussetzungen: W-P02 in Web-main
(oder Branch gemerged), Vertragsliste aus W-P04 nutzbar.
Ziel: Bestätigte Neuanlage von Tabellen im Web, strikt nach API 0.6 (Referenz: contracts/api-v0.6.0.json,
contracts/TABLE-CREATE-API.md, docs/handoffs/B4a.md im Root).
Aufgaben:
1. Unveränderte Vertragskopie nach src/lib/contract/v6/ (api-v0.6.0.json, SHA-256 b08b6aba858f72a3bad1fce2933e487eed017f5e35b4690cfd9fae3a590a6887,
   provenance.json mit Backend 3d21e42b0806f2a1953bcbad47f1e9fcb2acda93), Typen und Standalone-Validatoren per Generator.
2. src/lib/features/table-create/: CreateClient (Capabilities v6, POST tables/create), Formular: Name, bis 32 Spalten (BIGINT,
   VARCHAR, Binary, Decimal mit Grenzen aus dem Vertrag), Nullable, Defaultmodus mit typisierten Konstanten, geordneter Primärschlüssel
   (auch leer), Vorabprüfung der Grenzen im Client (Server bleibt maßgeblich), ausdrückliche Bestätigung, Anzeige von created und der
   getrennten Strukturprüfung (transactional/observed/different/unavailable), Fehlerfälle (409, 422, outcome_unknown ohne Retry-Knopf),
   Hinweis auf SQLite-Typgrenzen (BIGINT-PK kein Rowid-Alias, VARCHAR/Decimal ohne Präzisionsgarantie) und Zeitlimit.
3. Einhängen über feature.ts (Datenbankreiter "Tabelle anlegen"); nach Erfolg Objektliste aktualisieren.
4. Textschlüssel table-create.* in src/lib/i18n/de.ts (nur Anhängen).
Erlaubte Dateien: src/lib/contract/v6/**, src/lib/features/table-create/**, tests/browser/table-create.spec.ts,
tests/real/table-create.spec.ts, docs/handoffs/W-P05.md und der Textblock in de.ts. Keine Änderung an Bestandsvertragsdateien.
Abnahme: npm run contract:check, check, test, format:check, test:browser, test:csp; echte Fälle gegen Go-Backend (SQLite, PostgreSQL;
MySQL/MariaDB soweit die Fixtures in der Umgebung laufen) mit Namen mit Anführungszeichen/Leerzeichen/SQL-Text; Namenskonflikt
(409) lässt bestehende Tabelle und Daten unverändert; keine Anfrage ohne Bestätigung; Doppelklick sendet einmal; veraltete Antwort
nach Token-/Verbindungswechsel wird verworfen. Reviewer: anderer Web-Agent.
```

### A.6 W-P06 Backend: Katalog, Übersicht, Definition, Serverinfo (API 0.7) (Backend-Agent, Sonnet)

```text
[Kopfblock]
Paket W-P06-be, Repo db-manager (Root), Worktree .worktrees/w-p06-be, Branch feat/w-p06-overview. Voraussetzung: W-P01 gemerged
(Registry, dbplan, dbfix). Zuerst Stufe S1: Vertrag und Fixtures committen und dem Reviewer zum Freeze geben; erst danach
Implementierung (der Web-Agent W-P06-web beginnt nach dem Freeze).
Ziel: Neue lesende API 0.7 für Datenbankliste, Tabellenübersicht, Objektdefinition und Serverinformationen.
Umfang: Routen unter /api/v7/ (capabilities, databases, objects, definition, server-info), alle Bearer-authentifiziert, alle Anfragen mit
Verbindung wie bisher. Datenbankliste: Name, Standardkollation, Objektzahl und Größe als Schätzung (Feld estimated=true) bzw. nur auf
Anforderung exakt, mit Zeilenlimit/Zeitlimit; Tabellenübersicht: Typ (table/view/…), Zeilenschätzung, Engine, Kollation, Größe, Kommentar
soweit der Treiber sie liefert; Definition: SHOW CREATE (MySQL/MariaDB), sqlite_master (SQLite), PostgreSQL rekonstruiert aus Katalog mit Feld
derived=true und Hinweis; Serverinfo: Version, Laufzeit/Uptime soweit verfügbar, Zeichensätze, Kollationen, Engines/Plugins, Sitzungszeichensatz;
keine Geheimnisse, keine Hostpfade. Identifier nur aus Katalogtreffern/validierten Eingaben, alle Werte gebunden. Fähigkeiten je Treiber
(sqlite/mysql/postgres, MariaDB als mysql mit Variante) explizit.
Dateien: internal/core/overview.go, internal/database/overview*.go (je Dialekt eigene Datei), internal/httpapi/overview.go, Registrierung über
features.go (nur eigener Eintrag), contracts/api-v0.7.0.json, contracts/OVERVIEW-API.md, contracts/fixtures/overview/**,
scripts/check-overview-contract.py (Muster der bestehenden Wächter), scripts/test-overview.py (dbfix), .github/workflows/overview.yml,
docs/handoffs/W-P06-backend.md. Vertragsindex contracts/README.md ändert der Lead.
Abnahme: go test -race -count=1 ./... und go vet ./...; python3 scripts/check-overview-contract.py; python3 scripts/test-overview.py gegen
MySQL 8.4, MariaDB 12.3, PostgreSQL 18.6 (TLS verify-full) und SQLite; Ergebnisse gegen Direktabfrage verglichen; leere DB, viele Objekte
(Limit/Paging), View vs. Tabelle, Sondernamen (Quote, Backtick, Leerzeichen, SQL-Text), Schätzung vs. Exakt, fehlende Rechte; bestehende
Vertragswächter v1 bis v6 unverändert. Reviewer: anderer Backend-Agent; Sicherheitsfokus auf Informationslecks und teure Abfragen.
```

### A.7 W-P06 Web: Übersicht, Definition, Serverinfo (Web-Agent, Sonnet)

```text
[Kopfblock]
Paket W-P06-web, Repo db-manager-web, Worktree .worktrees/w-p06-web, Branch feat/w-p06-overview. Startet nach Vertragsfreeze (G1) von W-P06-be;
bis zum Backend-Merge gegen die Vertragsfixtures contracts/fixtures/overview/ (aus dem Root-Repo, unverändert kopieren) entwickeln.
Voraussetzungen: W-P02, W-P04 gemerged.
Ziel: Startseite, Datenbankliste, Tabellenübersicht, Definitionsansicht und Serverinfo auf Basis von API 0.7.
Aufgaben: Vertragskopie src/lib/contract/v7/ (SHA und Provenienz aus dem Root-Vertrag), Typen und Validatoren per Generator; OverviewClient;
Komponenten unter src/lib/features/overview/ und src/lib/features/server-info/: Startseite mit Serverkontext (welche Verbindung, Version,
Zeichensatz), Datenbankliste mit Kennzeichnung Schätzwert/exakt und Aktion "exakt zählen", Tabellenübersicht (Tabelle/View unterscheidbar,
Zeilen geschätzt, Engine, Kollation, Größe) mit Sortierung/Filter, Definitionsansicht (nur lesen, kopierbar, bei derived Hinweis), Serverinfo
(Zeichensätze, Kollationen, Engines); alle über feature.ts in Shell einhängen, Hash-Routen #/server und #/db/<name>/uebersicht; Fähigkeiten je
Treiber auswerten (nicht Unterstütztes erklärt statt versteckt); Texte nach src/lib/i18n/de.ts (Schlüssel overview.*, server-info.*).
Dateien: src/lib/contract/v7/**, src/lib/features/overview/**, src/lib/features/server-info/**, tests/browser/overview.spec.ts,
tests/real/overview.spec.ts, docs/handoffs/W-P06-web.md, Textschlüssel in de.ts.
Abnahme: npm run contract:check, check, test, format:check, test:browser, test:csp; echter Lauf (SQLite, PostgreSQL; MySQL/MariaDB soweit
Fixtures laufen) nach Backend-Merge; 320 px; sehr lange Listen (Paging/Virtualisierung nicht nötig, aber Limit-Hinweis); Ladeabbruch beim
Verbindungswechsel verwirft späte Antworten; kein Credential in URL/Hash. Reviewer: anderer Web-Agent.
```

### A.8 Review-, Integrations- und Doku-Prompts (Vorlagen für jedes Paket)

```text
[Kopfblock]
Rolle: unabhängiger Reviewer für Paket <ID> (Backend/Web). Du hast das Paket nicht geschrieben. Eigener Review-Worktree und Branch
review/<ID>. Lies Auftrag, Vertrag, Handoff, Diff gegen Basis. Prüfe: (1) Dateibesitz eingehalten, keine fremden Dateien; (2) Vertrag
und Implementierung stimmen überein, Vertragsbytes unverändert, Fähigkeiten je Treiber korrekt; (3) Sicherheitsleitplanken: Identifier-
Quoting aus Metadaten, gebundene Werte, keine freien SQL-Fragmente, Plan/Bestätigung bei Schreibaktionen, kein Credential in Logs, URL,
Speicher, Cache; (4) Tests: Abnahmekriterien tatsächlich abgedeckt, je DB-System, negative Fälle; Führe die genannten Prüfungen selbst aus
(nicht nur Handoff lesen) und ergänze ein bis drei eigene Gegenproben; (5) Lizenzen/Notices bei neuen Abhängigkeiten. Ergebnis:
docs/handoffs/<ID>-review.md mit Findings (ID, Schwere, Beleg, Reproduktion), Freigabeentscheidung (frei / frei mit Auflagen / nicht frei)
und exakter geprüfter Commit-SHA. Keine Fixes im Produktbranch; Fixes macht der Autor, du prüfst nach.
```

```text
[Kopfblock]
Rolle: Integrations-Agent für Bündel <Liste der Pakete>. Nur du baust Assets und änderst web-build.lock.json, scripts/build-web.py,
internal/webui/**. Voraussetzung: alle Pakete des Bündels haben G2 (Backend) bzw. G3 (Web) und der Lead hat die Integration
freigegeben. Schritte: Backendbranches nach Leadfreigabe in einen Integrationsbranch mergen (go test -race, go vet, alle
Vertragswächter), Web-Quelle auf einen vollen Commit pinnen (nur geprüfte, im Web-main erreichbare Commits), Lock aktualisieren,
python3 scripts/build-web.py (zwei frische Neubauten, bytegleich), python3 scripts/build-web.py --check, python3 -B scripts/test-build-web.py,
python3 scripts/test-web-integration.py (echter Browser gegen eingebettetes Produkt, SQLite und PostgreSQL), Compose-Prüfung; Buildslot
vorher beim Lead anmelden, nie parallel zu anderen Builds. Ergebnis: docs/handoffs/<Bündel>-integration.md mit allen Befehlen und
Ausgängen, Hashes des Lock/Manifests, offenen Prüfungen. Kein Merge nach main, kein Push auf main, kein Release.
```

```text
[Kopfblock]
Rolle: Doku-Agent (Haiku für Gerüste, Sonnet für Statuszeilen) für Paket <ID>. Nur auf Anweisung des Leads: Nachtrag der Teilnachweise in
docs/PARITAET.md (Test-ID, Fixture, Version, Befehl, Datum, Ergebnis nur aus den Handoffs und Reviews übernehmen, nichts erfinden), Zeile in
docs/STATUS.md und docs/ARBEITSPLAN.md, Eintrag im Vertragsindex contracts/README.md, kurze Nutzerbeschreibung in README/Pages-Quellen
falls betroffen. Ergebnis und offene Prüfungen klar trennen; "umgesetzt" nur mit Review und Integration. python3 scripts/check-docs.py grün.
```

### A.9 Haiku-Kleinpakete der Welle 1

```text
[Kopfblock]
Rolle: Haiku-Agent, Paket W-P01h/W-P02h/W-P03h (jeweils getrennt beauftragen). Rein mechanisch, keine Architekturentscheidung.
(a) Mock-Fixtures: Aus dem Entwurf contracts/fixtures/overview/ (Backend-Agent) zusätzliche synthetische Fälle ableiten: leere Liste, sehr
    lange Namen, Sonderzeichen, Fehlerantworten 400/401/422; keine echten Daten; nur im Verzeichnis contracts/fixtures/overview/.
(b) Textkatalog: Bestehende deutsche Texte aus den Svelte-Komponenten in src/lib/i18n/de.ts überführen (nur Textschlüssel; Komponenten nur
    durch Aufrufe t('schlüssel') anpassen, wenn der Web-Agent von W-P02 es ausdrücklich freigibt); Vitest bleibt grün.
(c) Icons: Aus einem selbst erstellten neutralen SVG-Symbol (Datenbankzylinder, einfache Formen) PNG 192, 512, maskable 512, apple-touch-icon
    180 und favicon rastern; Dateien nur unter public/icons/; keine Fremdgrafiken, keine Markenzeichen.
Abnahme: Formatprüfung (prettier), vorhandene Tests grün, Stichprobe durch den Auftraggeber.
```

## 9. Offene Punkte dieses Plans

1. Die API-Nummern 0.7 bis 0.20 sind Vorschläge; endgültige Vergabe beim Paketstart (Lead), Kollisionen werden in Abschnitt 3 nachgetragen.
2. Der Ist-Stand der Oberfläche wurde aus Quellcode und Routen abgeleitet; Browserläufe wurden für diesen Plan nicht ausgeführt.
3. Aufwand und Dauer sind nicht geschätzt (Inventar-Aufwandsklassen S, M, L gelten nur für den Quellumfang).
4. Die in der Matrix fehlende Zielversionsliste (Grenze 3) wird durch E3 vorläufig gesetzt, nicht endgültig.
5. W-P30 setzt Antworten auf ADR-0006 voraus; ADR-0007 (Apple-MySQL) bleibt außerhalb dieser Linie.
6. Der Vorschlag steht im Konflikt mit dem Wortlaut von ARBEITSPLAN Punkt 4 nur so lange, bis der Lead den Abschnitt „Prio PWA-Parität“ dort bestätigt.

## 10. Tatsächliche Prüfung beim Erstellen

- Abdeckung: Ein Skript zog alle `PMA-*`-Matrix-IDs aus [PARITAET](PARITAET.md) (181) und alle `PMA-NEU-*`-IDs der Bereichsdateien (119) und verglich
  sie mit den in diesem Plan genannten IDs (Bereichsschreibweise `A..B` expandiert). Ergebnis: Alle 181 Matrixzeilen und 119 neue Funde kommen vor;
  die oben genannten elf Bestandsfälle sind als solche ausgewiesen. Es wurden keine unbekannten IDs genannt. Die Prüfung belegt die Nennung, keine
  fachliche Richtigkeit der Zuordnung.
- Ist-Stand: Routen (`internal/httpapi`), Web-Quellen (`src`, `package.json`, `vite.config.ts`, `index.html`), Go-Handler (`internal/webui`) und
  Lock/Buildskript wurden im Code gelesen. Es wurden keine Produkt-, Browser-, Datenbank- oder Buildläufe ausgeführt.
- `python3 scripts/check-docs.py`: siehe Commit-Nachricht und Bericht des Erstellers.
