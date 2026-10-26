# Handoff W-P28 Backend: Erweiterte Textformate und Dump-Details

Stand: 26.10.2026. Reviewkandidat auf `feat/w-p28-backend`, auf main `3a7e1f1` (Wave 3,
API 0.16) gemergt. API-Nummer 0.21 (reserviert: W-P17=0.17, W-P12=0.18, W-P13=0.19,
W-P27=0.20); ein früherer Codex-Lauf nutzte 0.22 und wurde auf 0.21 umnummeriert.
Nichts nach main gemergt. Zentrale STATUS-/ARBEITSPLAN-Dateien, Vertragsindex,
Paritätsmatrix und Webassets bleiben beim Lead; unabhängiger Review steht aus.

## Ergebnis und Dateien

- `internal/core/export.go`, `export_extended.go`, `export_extended_test.go`:
  additive Requestfelder, eigene v21-Normalisierung, strikte Dekodierung mit Erhaltung
  der TLS-Vorgabe, Optionspräsenz für v11 und UTF-8-Prüfung. Die bisherige
  `NormalizeExport`-Funktion und v11-HTTP-Oberfläche weisen neue Optionen ab.
- `internal/database/export.go`, `export_format.go`, `export_text.go`,
  `export_details.go`: zeilenweiser Export von XML, YAML, PHP-Array, LaTeX tabular/
  longtable, MediaWiki, Texy und CodeGen-Text. Vorhandene Limits, Slots, Kontext,
  Snapshots und Trailerpfade bleiben maßgeblich. Views als Tabellen, sichere
  Katalogkommentare, begrenzte Kompatibilität und Datenanweisungen, Katalogaliase.
- `internal/database/export_extended_test.go`: echte SQLite-Parser-/Werte-/
  Speicherklassen-Roundtrips, Sonderzeichen und doppelte Namen, Referenzbytes,
  Limits/Fehler/Abbruch/UTF-8, View-Restore, Aliase und Konfliktanweisungen;
  neue Livefälle für MySQL/MariaDB/PostgreSQL mit unabhängiger Restore-DB.
- `internal/httpapi/export.go`, `export_extended.go`, `export_extended_test.go`:
  authentisierte eigene v21-Routen, ehrliche Treiberfähigkeiten, MIME/Dateiendungen,
  GZip, Trailer, Schutz des alten Optionsumfangs. `features.go` genau ein neuer
  Eintrag, `features_test.go` ausschließlich Anzahl 12 → 13.
- [API 0.21](../../contracts/api-v0.21.0.json),
  [Verhaltensvertrag](../../contracts/EXTENDED-EXPORT-API.md),
  [synthetische Platzhalterfixtures](../../contracts/fixtures/extended-export/README.md),
  ergänzender Link in `contracts/EXPORT-API.md`. Alle v1–v12-Vertragsbytes und
  `export-format-v1.schema.json` unverändert.
- `scripts/check-extended-export-contract.py`: feste SHA256-Pins aller v1–v12,
  Additivität zu v12, Referenzen, Authentisierung, strikte neue Schemas und Fixtures;
  v13–v20 sind optional und werden nicht vorausgesetzt (Lücken 0.17–0.20 bis zu deren Integration; v21 baut nur auf v12 auf und enthält deshalb die Pfade von 0.13–0.16 nicht. Der Lead führt beim Zusammenführen die Vertragsunion).
- `scripts/test-extended-export.py`: SQLite/HTTP mit Race, echte Fähigkeitsantwort
  gegen Schema; optional seriell `dbfix.py` für MySQL 8.4, MariaDB 12.3, PG 18.6.
  `.github/workflows/extended-export.yml`: vorbereitete CI einschließlich Livefällen.
- [ADR W-P28](../decisions/ADR-W-P28-text-export.md): begrenzte Formatprofile und
  Katalogziele als Vorschlag; Annahme durch Lead offen. Keine neue Abhängigkeit,
  eigener Code mit SPDX AGPL-3.0-only, keine Attribution-Zeilen.

## Fachliche Grenzen und Zuordnung

EXP-010/015..020 sind als eigene dokumentierte Textprofile implementiert, keine
Bytekompatibilität mit phpMyAdmin-Plugins. YAML ist JSON-kompatibles YAML 1.2;
PHP-Metadaten stehen als JSON-String, Zellarrays sind typisiert. CodeGen ist ein
nicht ausführbares JSON-Zeilenprofil, kein C#/NHibernate-Generator. Die sichtbaren
Formate erhalten Zelltypen und zeigen Steuerzeichen als Escapenotation. Keine
vollständige universelle Roundtrip- oder Rendering-Paritätsaussage.

DUMP-003: INSERT/IGNORE/REPLACE für SQLite und MySQL/MariaDB; PG nur INSERT.
DUMP-006 bleibt begrenzt auf DEFINER-Auslassung durch Viewmaterialisierung und
`native`/`data-only`; keine Erhaltung/Ersetzung fremder DEFINER oder historischen
MySQL-/Cross-Dialektmodi. DUMP-007: zusätzliche sichere Katalognamenskommentare und
Aliase nur zu existierenden strukturgleichen Zielen für SQL-Datenexport; native
DDL-Kommentare bleiben erhalten. Keine Konfigurationsspeichermetadaten, freie
DDL-Aliasumschreibung oder Entfernung von Tabellen-/Spaltenkommentaren.
DUMP-008: Viewdaten als reguläre Tabellen ohne Viewlogik/Defaults/Schlüssel/Eigentümer;
SQLite ohne Typaffinität, Netzwerk nur skalare Typteilmenge. Bei Views wird der
Snapshot konservativ als Best-Effort gemeldet. Externe Quellen und nichttransaktionale
Viewquellen erhalten kein Transaktionsversprechen.

DUMP-001 bleibt bei Triggern/Routinen/Events offen, weil W-P19 in dieser Basis nicht
integriert ist. DEFINER-Objektdefinitionen, Rechte und Restoreabhängigkeiten dieser
Objektklassen sind kein implementierter Nachweis. DUMP-002/004/005 nutzen die
bestehenden begrenzten W-P11-Pfade. EXP-003/NEU-EXP-003 (Vorlagen), EXP-005/IMP-003
(Serververzeichnisse), EXP-011..014 (ODS/ODT/Word/PDF, E8) und IMP-009..011
(ODS/XML/MediaWiki-Import, W-P12) sind außerhalb dieses Auftrags, keine Paritätsabnahme.

Jeder Client muss Format und letzte strukturelle Schlussmarke prüfen; HTTP 200,
GZip-Abschluss und geschlossene Einzeltabelle reichen nicht. Trailer können im Browser
fehlen. Ein Textwert mit Markerinhalt ist kein gültiger Dokumentabschluss. Daten werden
nicht gesammelt und Quelldaten nicht verändert; Restore erst nach Abschlussprüfung.

## Prüfstand (26.10.2026, Docker verfügbar)

- Live: `python3 scripts/test-extended-export.py --live` bestanden; je serielle Wegwerf-
  Container (dbfix) MySQL 8.4, MariaDB 12.3, PostgreSQL 18.6 mit TLS, plus SQLite; alle
  acht Formate PASS, View-Materialisierung und Restore in unabhängige DB. Danach keine
  übrig gebliebenen `dbmanager-`-Container.
- Unabhängige Parser: `scripts/check-extended-export-files.py` prüft echte Ausgaben
  (24 Dateien: 3 Treiber x 8 Formate) mit PHP (`include`), PyYAML, ElementTree, json
  sowie HTML-Entitäts-/LaTeX-Rückwandlung gegen die CodeGen-Zeilen: Typen, NULL,
  leerer Text, Binär, Steuerzeichen und Sonderzeichen stimmen überein; letzte
  Abschlussmarke je Format geprüft. Kein LaTeX-/MediaWiki-/Texy-Renderer vorhanden.
- `gofmt -l .` leer, `go vet ./...` ok, `go test -race -count=1 ./...` alle Pakete ok
  (Listener- und Crash-Tests liefen diesmal), alle 16 `check-*-contract.py` ok,
  `check-docs.py` 1159 Dateien/0 Fehler.
- Korrigiert: Wächter prüfte versehentlich v21 selbst als Vorgänger (range 13..21).
- CSV-Formelinjektion: `excel-csv` präfixiert jedes Feld (auch Kopf) mit `'`; Klartext-`csv`
  bleibt bewusst unverändert (W-P08-Vertrag). Neue Formate escapen alle Daten.
- Parallelität: max. 2 Exporte über die bestehenden Slots; neue Routen nutzen denselben
  Handler. Kein Teilzustand, Abschlussmarker fehlt bei Fehler/Abbruch.

## Offen

Unabhängiger Review, Annahme des ADR, fachliche Abnahme der begrenzten DEFINER-/
Kommentar-/CodeGen-Profile, Vertragsunion mit 0.17–0.20 beim Integrieren (Routen
`/api/v21/*` kollisionsfrei), kein Browser-/Compose-Test, DUMP-001 (Trigger/Routinen)
offen. Offene Teilparität nicht als phpMyAdmin-Parität übernehmen.
