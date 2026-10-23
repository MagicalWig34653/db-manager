# Unabhängiger Vertragsreview C-Export1

Stand: 23.10.2026. Eigener Worktree `.worktrees/export-contract-review`, Branch
`review/export-contract`; Reviewer ist kein Vertrags-/Fixtureautor.
Kandidat `e5e973b0a31e9ca7e1ed8d2d52819c849b6a13a2`, nachfolgender reiner
Autorenhandoff `a2ffbfb564c0e19bb1a9644abccdd9e8dae31f62`.

## Urteil

**Kein bestätigter Befund. Der vorgeschlagene Ergebnisexport1 kann nach gesonderter
Leadfreigabe als gemeinsamer Vertrag integriert und eingefroren werden.** Kein
Freeze oder Integration durch den Reviewer. Keine native App-/Importer-/Dateidialog-/
Gerätefreigabe und keine allgemeine CSV-/Excel-/Roundtrip-/Sicherheitsgarantie.

Schema-SHA256 unabhängig bestätigt:
`49e7d53b14ff142148a851753e980ad068b01aad7555b27808f3ffb0f4df85c7`.
Nur neue `contracts/export/**` und Autorenhandoff; keine vorhandenen APIs,
Vertragsindizes, Go-/Runtimeabhängigkeiten, Site-, App- oder zentralen Statusdateien.

## Norm, Herkunft und Grenzen

- JSON Schema2020-12 und Zusatznormen gegen tatsächlichen gehashten Web-F6-Exporter
  `1a8adfa4174aafe1850b843909af023b4357ebaa` gelesen. Der Exporter in akzeptierter
  F5-Quelle `8ef738d22a1164291e2c8183d80e72d1f043c0f8` ist bytegleich.
- Envelope-/Scope-/Column-/Cellfelder sind genau begrenzt. Namen dürfen leer/doppelt
  bleiben, Reihenfolge bleibt erhalten; NULL besitzt kein value, sieben weitere
  Tags ausschließlich Stringwerte. Keine SQL-/Token-/Profil-/Dauerfelder im Export.
- Kein neuer Zahlen-/Datums-/Base64parser und keine numerische Kanonisierung.
  Strings wie -0/NaN/Infinity/BC oder anderweitig opake Inhalte bleiben strukturell
  Strings; DB-semantische Gültigkeit wird nicht vom Schema vorgetäuscht.
- Schema beschreibt Struktur, Safe-Integer-Pagination und Grenzen; gleiche Breite
  aller Zeilen und die tatsächlich fertige UTF8-Dateigröße sind ausdrücklich
  zusätzliche Prüfungen. Keine JSONbytekanonik oder vollständiger Tabellendump.
- CSVprofil stimmt mit dem freigegebenen Producer überein: UTF8 ohne BOM, Komma/CRLF,
  Quoteverdopplung, ungequotetes NULLmarker\\N gegenüber gequotetem Text, bewusste
  Roh-/Apostrophvariante und explizite Steuerzeichenablehnung. Keine stille Ersetzung.
- Drei Originalwerkzeugnotices, Versionen und alle Quellen-/Schemahashes tatsächlich
  mit installierten Originalbytes verglichen. CRLF/Originalleerraum und normative
  CSVbytes bleiben durch lokale enge Gitattributes erhalten. Keine globale
  Whitespaceausnahme oder neue Produktabhängigkeit.

## Tatsächliche unabhängige Prüfung

Eigenes temporäres frisches öffentliches Webcheckout mit exakt F6-/F5-Gitobjekten,
`npm ci --ignore-scripts --no-audit --no-fund` aus dem gepinnten Original-Weblock,
anschließend der unveränderte konkrete Prüfer:

```sh
node contracts/export/check.mjs --web-checkout <eigenes-gepinntes-webcheckout>
```

Bestanden: Schema mit strict Ajv2020, sieben semantische JSONfixtures, zwölf exakte
CSVbyte-Fixtures, 21 negative Form-/Breiten-/Encodingfälle, 254 CSV-Controlablehnungen
und drei tatsächliche inklusive8MiB-Grenzen samt Überschreitung. Keine großen
Grenzdateien committed. Node26.5.0, TypeScript5.9.3, Ajv8.20.0 und alle drei
Originalnotices/Quellenhashes geprüft. Keine cachefreie Netzwerkaussage.

Zusätzlich eigene temporäre Gegenproben, keine Produktänderung:

- Keine Spalten/keine Zeilen und eine leere Zeile zu null Spalten gültig; zusätzliche
  Zelle bei null Spalten als Breitenfehler abgewiesen.
- Leere doppelte Columnnamen, leerer Typ, leerer Text und leerer Binarystring bleiben
  getrennt erhalten. Native database_type ist nicht versehentlich Teil des Formats;
  das eigentliche DisplayResult verwendet type, wie in der Norm ausdrücklich erklärt.
- Offset0 und maximal sichere Ganzzahl gültig; negative, gebrochene oder unsichere
  Pagination und page_size0 abgewiesen. Numerisches Scope-minus0 ist mathematisch0
  und wird vom bestehenden Producer als0 serialisiert; keine neue -0-Verbotsnorm.
  Zellstring "-0" wird davon nicht verändert.
- Zusätzliche profile/sql/connection/duration_ms-Envelopefelder abgewiesen.
  Opake Stringwerte werden nicht als Zahlen/Datum neu interpretiert.
- Überlange fertige JSONbytes trotz syntaktisch gültiger zusätzlicher Leerzeichen
  als Bytegrenze abgewiesen; Kodierung und Größe sind verschiedene Prüfungen.

Eine anfängliche eigene Zusatzprobe verwendete versehentlich database_type statt
DisplayResult.type und scheiterte deshalb korrekt am Schema. Ausschließlich diese
Reviewfixture wurde korrigiert und die Gegenproben danach grün wiederholt; kein
Produktbefund und kein ursprünglicher Fehlversuch als Erfolg dargestellt. Die
Wiederholung nutzte den expliziten gehashten vorhandenen Webprüfcheckout; der
unveränderte Konformitätslauf oben war separat im frischen öffentlichen Checkout.

`python3 scripts/check-docs.py` am Kandidaten320/0, Schemahash, eigener Diffcheck
bestanden; abschließende Prüfung nach Ergänzung dieses Reviews separat ausgeführt.
Keine Docker-/Go-/Runtime-/DB-/Swift-/OSprüfungen erforderlich oder daraus behauptet.
Die vorhandenen Root-CIs sind keine zusätzliche Exportkonformitäts-CI; maßgeblich
ist der tatsächlich ausgeführte manuelle Prüflauf oben.

## Ressourcen und Fortsetzung

Eigener frischer Clone samt node_modules und temporäre Reviewinstrumentierung
entfernt. Keine eigenen laufenden Jobs, Docker-/OSressourcen oder Secrets erzeugt.
Vorhandene Geschwistercheckouts und Caches nicht verändert; ursprüngliche Gitidentität
beibehalten. Nur dieser unabhängige Review wird zusätzlich committed.

Nächster Schritt: durch Lead freigegebener Releaseagent integriert Kandidat,
Autorenhandoff und unabhängigen Review, setzt den getrennten Exportvertrag1-Status
und Herkunftspin im kanonischen Index. Native A3-Produzenten dürfen erst danach
auf die freigegebenen Schema-/Norm-/Fixturebytes bauen und ihre eigene tatsächliche
Interoperabilität nachweisen. Kein Appcode ist Teil dieser Abnahme.
