# ADR W-P28: Begrenzte Textformate und Katalogziele

Status: Vorschlag als implementierter Reviewkandidat; Annahme durch Lead offen.

## Kontext

E4 verlangt begrenzte Browserstreams. E8 erlaubt keine neuen Office-/PDF-Bibliotheken.
Der bestehende Exportvertrag verlangt einen positiv prüfbaren Formatabschluss.
Freie Namen und Werte dürfen weder Quellcode noch Markup erzeugen. Ein universelles
Umschreiben nativer DDL wäre ohne Parser kein sicherer Kompatibilitätsmodus.

## Entscheidungsvorschlag

v21 ergänzt eigene Routen und Schemata additiv zu v12; v1–v12 bleiben bytefest.
Zeilen laufen durch die vorhandenen Limits und zwei Exportplätze. YAML beschränkt
sich auf JSON-kompatibles YAML 1.2, XML kodiert nicht darstellbare Felder Base64.
PHP-Strings kodieren jeden Byte als Hexescape. Wiki/Texy kodieren jedes Zeichen als
Entität, LaTeX verwendet feste Escapes. CodeGen ist ein nicht ausführbares Textprofil.
Alle Formate erhalten eine eigene strukturierte Schlussmarke; die Clientpflicht bleibt.

Views werden ausschließlich als neue Tabellen materialisiert. DEFINER wird daher
nicht ausgegeben; Erhaltung/Ersetzung wartet auf die Objektdefinitionen aus W-P19.
Kompatibilität bleibt `native` oder `data-only`. Aliase werden ausschließlich auf
existierende strukturgleiche Katalogtabellen für Daten-Dumps aufgelöst; dbplan zitiert
sie. Native DDL wird nicht zur Alias-/Kommentar-/DEFINER-Entfernung umgeschrieben.

## Folgen und Grenzen

Keine neue Abhängigkeit. Keine Sprachgenerator-/Office-/Import-/Vorlagenparität.
View-Dumps bewahren keine Viewlogik, Constraints, Defaults oder Berechtigungen.
Bei unbekannten skalaren Typen endet die Vorbereitung ohne Streambeginn. Externe
und nichttransaktionale Viewquellen bleiben ehrlicher Best-Effort. Kommentare sind
zusätzliche sichere Kataloghinweise, kein DDL-Kommentarfilter. Historische
Kompatibilitätsmodi bleiben abgewiesen statt stillschweigend angenähert.

[Vertrag](../../contracts/EXTENDED-EXPORT-API.md),
[Prüfstand und offene Gates](../handoffs/W-P28-backend.md).
