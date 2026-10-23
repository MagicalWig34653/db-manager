# Gemeinsamer Ergebnisexport 1 – Vorschlag C-Export1

**Status: vorgeschlagen, noch nicht unabhängig abgenommen oder als gemeinsamer
Vertrag eingefroren.** Das Paket beschreibt das bereits unabhängig akzeptierte
Web-F6-Dateiformat. Es implementiert weder native Appfunktionen noch einen Import.
HTTP-API, Tresorformat und Ergebnisexport haben getrennte Versionsräume.

## Herkunft und Geltung

Maßgebliche bestehende Produzentquelle ist
[Web-F6 `1a8adfa`](https://github.com/MagicalWig34653/db-manager-web/blob/1a8adfa4174aafe1850b843909af023b4357ebaa/src/lib/export.ts),
mit [UI-/Downloadgrenzen](https://github.com/MagicalWig34653/db-manager-web/blob/1a8adfa4174aafe1850b843909af023b4357ebaa/src/lib/components/ExportControls.svelte)
und [Autoren-/Fixnachweisen](https://github.com/MagicalWig34653/db-manager-web/blob/1a8adfa4174aafe1850b843909af023b4357ebaa/docs/handoffs/F6.md).
F5-Quelle `8ef738d22a1164291e2c8183d80e72d1f043c0f8` enthält denselben Exporter
bytegleich. [Quellen-/Werkzeuglock](source-lock.json) bindet Originaldateien,
Schemahash, Werkzeugversionen und erhaltene Originalnotices mit SHA256.

Ein Export enthält genau einen bereits geladenen unveränderlichen Ergebnissnapshot
oder eine geladene Tabellenseite. Er löst keine zusätzliche DB-Abfrage aus, ist kein
vollständiger Tabellen-/Datenbankdump und bestätigt keine DB-Mutation. Quellen-,
Token- und Sitzungswechsel sowie Blob-/Dateidialog-Lebenszyklen bleiben zusätzlich
im jeweiligen Produkt zu prüfen. Das Format allein belegt diese Abläufe nicht.

## JSON-Struktur

[JSON Schema 2020-12](result-v1.schema.json) beschreibt die strukturellen Regeln.
Die Datei besteht aus einem Objekt mit ausschließlich folgenden Pflichtfeldern:

| Feld | Bedeutung |
| --- | --- |
| `schema_version` | Ganzzahl `1` |
| `scope` | Objekt `{"kind":"sql_result"}` oder `{"kind":"table_page","offset":25,"page_size":25,"has_more":true}` |
| `truncated` | Tatsächliches Kürzungsmerkmal des geladenen Ergebnisses als Boolean |
| `columns` | Geordnetes Array von Objekten mit exakt `name` und `type`, beide Strings |
| `rows` | Geordnetes Array geordneter Zellarrays |

Zusätzliche Felder werden auf allen Objektebenen abgewiesen. SQL, Verbindung,
Passwort, Token, Profil, Datenbank-/Tabellenname, Zeilenzahlquittung und Dauer sind
keine zusätzlichen Envelopefelder. Inhalte und Spaltennamen sind natürlich
DB-Daten; eine explizite lokale Datei kann vertrauliche Ergebniswerte enthalten.
Die Whitelist ist keine Anonymisierung dieser vom Nutzer gewählten Daten.

Spaltennamen dürfen leer oder doppelt sein; Name und Reihenfolge bleiben erhalten.
`type` ist die ursprüngliche Typbeschreibung, kein verlässlich portables DDL und
keine Aufforderung, Typ-SQL auszuführen. Native Modelle mit `database_type` müssen
explizit nach `type` abbilden. Es gibt keine implizite Objektabbildung nach
Spaltennamen, die Duplikate überschreiben könnte.

NULL ist exakt `{"kind":"null"}` ohne `value`. Jede andere Zelle besitzt exakt
`kind` und einen String `value`; erlaubte Tags sind `text`, `integer`, `decimal`,
`float`, `boolean`, `binary`, `timestamp`. NULL, leerer Text und leerer Binärwert
bleiben verschieden. Integer/Decimal/Float werden niemals für diesen Export in
JavaScript-/IEEE-Zahlen umgewandelt. Vorzeichen, Scale, spezielle Werte und
Zeit-/Era-Darstellungen werden als ursprüngliche Strings erhalten; keine
allgemeine ISO8601-Normalisierung oder numerische Kanonisierung. Binärwerte
verwenden die vorhandene Base64-Darstellung. Das Schema prüft Zellform und Tags,
aber nicht die DB-semantische Gültigkeit beliebiger Zahlen-/Zeit-/Base64strings.

`scope.offset` ist nullbasiert und `page_size` der Seitenkontext, nicht die Zusage,
dass genau so viele Zeilen vorliegen. Beide sind Ganzzahlen innerhalb des
sprachübergreifend exakt darstellbaren Bereichs bis `9007199254740991`;
`offset >= 0`, `page_size >= 1`. Produkt-/APIgrenzen können enger sein.
`has_more` benennt weitere Seiten; auch `has_more:false` bei `offset>0` macht
keinen vollständigen Tabellenexport. `truncated` bleibt ein getrenntes Merkmal
und wird nicht aus `has_more` erraten. Über mehrere Seiten gilt keine
atomare Snapshotgarantie.

## Zusätzliche verbindliche Prüfungen außerhalb des Schemas

1. Jede Zeile hat exakt die Anzahl Zellen aus `columns`, auch bei leeren Arrays.
   Standard-JSON-Schema vergleicht diese unabhängigen Arraylängen hier nicht.
2. Die tatsächlich fertige UTF8-Datei ist **höchstens 8 MiB = 8.388.608 Bytes** groß.
   Envelope, Escapes, Einrückung, Newlines, Präfixe und CSVquotierung zählen mit.
   Exakt die Grenze ist erlaubt, ein weiteres Byte nicht. Keine automatische Kürzung.
3. Ungültiges UTF8 oder syntaktisch ungültiges JSON ist kein gültiger JSON-Export.
   Ein JSON-String-Escape ist nicht dasselbe wie ungültige UTF8-Quelldateibytes.

Es wird **keine bytekanonische JSONdarstellung** verlangt: Whitespace, Reihenfolge
von Objektfeldern und zulässige Escapes können variieren; dieselben dekodierten
Strings, Arrayreihenfolgen und Werte müssen erhalten bleiben. Der bestehende
Webproduzent schreibt UTF8, zwei Leerzeichen Einrückung und ein abschließendes LF.
Der feste angebotene JSONdateiname ist `db-manager-result.json`, Medientyp
`application/json;charset=utf-8`. Native OS-Dateidialoge dürfen den Dateinamen
auf ausdrücklichen Nutzerwunsch ändern.

## CSV-Profil

CSV ist ein zusätzlicher bewusst begrenzter Anzeige-/Austauschweg und enthält
selbst keine Typ-, Scope- oder Trunkierungsmetadaten. Der Produktdialog muss den
wirklichen Umfang vor dem Download zeigen. Für verlustlosen typisierten Austausch
ist JSON zu verwenden. Das Profil ist UTF8 ohne BOM mit Komma und CRLF; es nutzt
RFC4180-Quotierung und die hier ausdrücklich beschriebenen Erweiterungen:

- Eine Headerzeile in Spaltenreihenfolge, danach die geladenen Zeilen.
- Jeder Header und jeder Nicht-NULL-Wert ist doppelt gequotet; eingebettete Quotes
  werden verdoppelt. CR und LF dürfen innerhalb gequoteter Felder erhalten bleiben.
- NULL ist das **ungequotete** `\N`; Text mit Inhalt `\N` bleibt gequotet, leerer
  Text `""`. Gewöhnliche CSVparser verwerfen meist die Information, ob ein Feld
  gequotet war: Die NULL-/Textunterscheidung ist deshalb keine allgemeine
  CSV-Roundtripgarantie. Binärzellen sind ihre Base64strings.
- `csv-raw` lässt die Header und Nicht-NULL-Strings unverändert. Tabellenkalkulationen
  können solche Inhalte als Formeln interpretieren; die bewusste Auswahl muss
  diesen relevanten Hinweis zeigen.
- `csv-spreadsheet` setzt **jedem** Header und Nicht-NULL-Wert zuerst ein Apostroph
  voran, auch Zahlen, Leertext und dem literalen NULLmarker. NULL bleibt unverändert.
  Das verändert die Daten ausdrücklich und ist keine universelle Excel-/
  Sicherheits-/Roundtripgarantie.
- Vor jeder CSV-Dateianlage werden U+0000–U+001F außer LF U+000A und CR U+000D,
  DEL U+007F sowie C1 U+0080–U+009F in Headern und Nicht-NULL-Werten ausdrücklich
  abgewiesen. Keine stille Ersetzung/Abschneidung; JSON erhält diese Zeichen exakt.
  Druckbares Unicode ist die ausdrücklich unterstützte Unicodeerweiterung.

Auch CSV zählt seine fertigen Bytes gegen dieselbe 8-MiB-Grenze. Angeboten wird
`db-manager-result.csv`, `text/csv;charset=utf-8`. Browser-/OS-/Consumerverhalten
ist nur im tatsächlich ausgeführten Prüfumfang belegt.

## Reproduzierbare Konformitätsprüfung

Voraussetzungen: Node26.5.0/npm11.17.0 wie im Webbuild; ein eigenes temporäres
öffentliches Webcheckout mit beiden gepinnten Gitobjekten und dessen installierten
Lockdateiabhängigkeiten. Kein Geschwistercheckout und kein Node im Go-Produkt nötig:

```sh
export_web_check=$(mktemp -d /tmp/dbmanager-export-web.XXXXXX)
git clone https://github.com/MagicalWig34653/db-manager-web.git "$export_web_check"
git -C "$export_web_check" checkout --detach 1a8adfa4174aafe1850b843909af023b4357ebaa
npm --prefix "$export_web_check" ci --ignore-scripts --no-audit --no-fund
node contracts/export/check.mjs --web-checkout "$export_web_check"
```

Alle Zeilen werden aus dem Vertragsrepo ausgeführt; das Webcheckout ist ein
ausschließlich eigenes temporäres Prüfarbeitsverzeichnis. Nach dem Lauf dieses
eigene Verzeichnis bereinigen, keine anderen Checkouts oder Caches löschen.
Ein vollständiger öffentlicher Clone enthält auch den gepinnten F5-Vergleichsstand;
bei einem flachen Clone müssen beide SHAobjekte ausdrücklich geholt werden.
Der Prüfer führt **nur den gehashten akzeptierten** `export.ts`-Inhalt aus;
Typimports werden mit TypeScript5.9.3 entfernt. Ajv8.20.0 prüft das Schema und die
Fallobjekte. Das sind Prüfwerkzeuge, keine neuen Go-/Produktlaufzeitabhängigkeiten.

Sieben [synthetische Vektoren](fixtures/vectors.json) werden gegen gespeicherte
semantische JSON- und zwölf exakte CSVbyte-Fixtures geprüft. Dazu kommen
[negative Form-/Breitenfälle](fixtures/negative-cases.json), fehlerhaftes UTF8/JSON,
alle 63 abgelehnten Steuerzeichen in Header/Zelle für beide CSVvarianten und
kompakte in-memory Rezepte für exakt 8 MiB sowie Überschreitung in allen drei
Formaten. Es werden keine großen Grenzdateien committed. Gleichwertige native
Tests müssen später dieselben Vektoren verwenden; dieses Paket behauptet noch
keinen Swift-/App-/Dateidialognachweis.

## Lizenzen

Eigene Spezifikation, Prüfhelfer und synthetische Fixtures: **AGPL-3.0-only**,
[Projektlizenz](../../LICENSE). Die unveränderten Originaltexte der ausschließlich
zur Validierung verwendeten Werkzeuge liegen in [notices](notices): Ajv8.20.0
MIT sowie TypeScript5.9.3 Apache-2.0 mit eigener ThirdPartyNoticeText.txt.
Hashlock und Prüfer vergleichen ihre Originalbytes mit den tatsächlichen
installierten Paketen. Originalautoren und deren Lizenzen bleiben erhalten.
