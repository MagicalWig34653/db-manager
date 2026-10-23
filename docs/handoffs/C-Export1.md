# C-Export1 – gemeinsamer Ergebnisexport als Vertragsvorschlag

23.10.2026. Autor: Contracts Architect; unabhängiger Reviewer wird durch Root
zugewiesen. Eigener Branch `docs/export-contract`, Worktree
`.worktrees/export-contract`, Basis `1b02ea30447f6ff100ef00a24b7e174bb8618cd0`.
**Noch nicht unabhängig abgenommen, eingefroren oder in main integriert.**

## Ergebnis und Dateibesitz

Ausschließlich neue Dateien unter `contracts/export/` und dieser Handoff.
JSON Schema 2020-12, deutscher Byte-/Verhaltensvertrag, unveränderte Originalnotices,
Quellen-/Werkzeughashlock, kompakte synthetische positive/negative Fixtures sowie
konkreter Konformitätsprüfer. Keine bestehenden APIs/Vertragsindizes/GoMod-/Go-/
Web-/Apple-/Site-/zentralen STATUS-/Planungsdateien geändert.

Vorherige Planung aus Apple `a6df2dc`, tatsächliche Produzentquelle Web-F6
`1a8adfa4174aafe1850b843909af023b4357ebaa`; exporter.ts ist im akzeptierten
F5 `8ef738d22a1164291e2c8183d80e72d1f043c0f8` bytegleich. Konkrete Pfade,
Hashes und erhaltene Werkzeuglizenzen stehen im [Lock](../../contracts/export/source-lock.json).
Native `database_type` muss später ausdrücklich nach `columns[].type` abgebildet
werden. Kein Swift-/Appcode oder Import wird in diesem Paket implementiert.

Schema und Norm ergänzen sich: keine unbekannten Felder, richtige Tags/Stringwerte,
beide tatsächlichen Scopeobjekte und Pagination im sicheren Ganzzahlbereich;
Zeilenbreite und fertige UTF8-Bytegrenze zusätzlich außerhalb JSON Schema.
Keine Zahlen-/Zeit-/Base64-Neuinterpretation, keine JSONbytekanonik. CSV bleibt
bewusst typverlustbehaftet mit exakt F6s NULL-/Quote-/Apostroph-/Steuerzeichenprofil.
Die normativen CSVbyte-Fixtures bewahren CRLF ausdrücklich per .gitattributes.
Originalnotices behalten ebenfalls ihre ursprünglichen CRLF/Leerraumbytes; nur
diese drei unveränderten Originaldateien sind vom eigenen Whitespacecheck ausgenommen.

## Tatsächlich ausgeführte lokale Prüfung

```sh
node contracts/export/check.mjs --web-checkout /tmp/eigenes-web-pruefcheckout
python3 scripts/check-docs.py
git diff --check
```

Der erste Lauf nutzte bereits vorhandene, version-/hashgeprüfte Werkzeuge des
Arbeitsplatzes, keinen behaupteten frischen oder cachefreien npm-Installationslauf.
Der finale öffentliche frische Clone wird im folgenden Checkpoint getrennt belegt.

- Schema mit Ajv2020 aus Ajv8.20.0 unter strict/allErrors kompiliert.
- Sieben JSONvektoren und zwölf gespeicherte exakte CSVbyte-Fixtures gegen den
  gehashten tatsächlichen TypeScript-Exporter mit TypeScript5.9.3 geprüft.
- 19 negative Struktur-/Breitenfälle plus ungültiges UTF8/JSON abgewiesen.
- 254 CSV-Steuerzeichenablehnungen, JSONwerte unverändert, je drei Formate genau
  an 8.388.608 Bytes akzeptiert und darüber abgewiesen. Kein großer Fixtureblob.
- Sechs Originaldateihashes, unveränderter F5-Exporter, Werkzeugversionen und drei
  Originalnotices mit tatsächlichen installierten Originalbytes geprüft.
- Dokumentationsprüfung nach Ergänzung des Handoffs vor Commit erneut ausführen;
  keine Produkt-/Go-/DB-/Docker-/OS-/Swift-/Geräteprüfungen daraus ableiten.

Die anfängliche Dateiermittlung nahm irrtümlich ExportControls direkt unter lib
an; tatsächlicher Pfad lib/components wurde gelesen und der Quellenlock korrekt
erst danach erstellt. Kein solcher fehlgeschlagener Lookup ist ein Prüferfolg.

## Grenzen und Fortsetzung

Nur Vertragsvorschlag aus bereits akzeptiertem Webverhalten. Keine eigenständig
freigegebene Austauschkanonik und keine native Interoperabilitätsbehauptung.
Source-UI-Whitelist ist keine Anonymisierung echter Ergebnisdaten. Kein vollständiger
Tabellendump, keine automatische zusätzliche DBabfrage, kein CSV-/Excel-Roundtrip-
oder allgemeines Sicherheitsversprechen. Keine neue Root-Laufzeitabhängigkeit;
Ajv/TypeScript werden nur aus explizit übergebenem gepinntem Prüfcheckout geladen.

Nächster Schritt: unabhängiger Vertrags-/Fixture-/Produzentenreview, dann gesonderter
Rootfreeze/Vertragsindex durch freigegebenen Releaseagent. Erst danach A3-Apphelper/
ViewModel/fileExporter und gemeinsame synthetische Vergleichsvektoren; echte
OS-Dateidialoge und Geräte bleiben separate Nachweise. Keine eigenen Docker-/OS-
Ressourcen oder Datenbankjobs angelegt. Laufende Jobs werden im finalen Checkpoint
benannt; keine fremden Änderungen oder Caches bereinigen.
