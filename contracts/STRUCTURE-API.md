# Lesende Strukturmetadaten – Vertrag 0.5.0

B3a ergänzt ausschließlich `GET /api/v5/capabilities` und
`POST /api/v5/tables/structure`. Die Verträge und Routen 0.1 bis 0.4 bleiben
unverändert. Der Vertrag ist nach unabhängiger Fixnachprüfung und exakter grüner CI
als0.5-Clientvertrag freigegeben; Referenzbackend und Hash stehen im Versionsindex. Die Anfrage verwendet denselben strukturierten
Verbindungs- und Tabellenbezug wie Describe; Bearer-Authentifizierung,
TLS-Prüfung, Zielhostfreigabe und flüchtige Zugangsdaten bleiben verbindlich.

Die Antwort enthält Indizes mit geordneten Termen und Constraints mit geordneten
Spalten. Es gibt keine SQL-Fragmente, Ausdrucks-/Prädikattexte, CREATE-Definitionen
oder DDL-Operationen. Ausdrucksterme werden lediglich als `expression` markiert.
Eine Strukturansicht darf daraus keine automatische DDL oder Schreibberechtigung
ableiten. Views, virtuelle Tabellen und fremde PostgreSQL-Tabellen sind gesperrt.

`yes`, `no`, `unknown` und `not_applicable` unterscheiden belegte Eigenschaften,
fehlenden Nachweis und sachlich nicht anwendbare Eigenschaften. `method_known`,
`name_known`, `columns_known` und `referenced_columns_known` unterscheiden auch
legitime leere Namen von unbekannten Angaben. Ein unbekannter Name bleibt leer;
eine unbekannte Spaltenliste bleibt leer. Bekannte leere Bezeichner bleiben
erhalten. Ausgabeobjektbezüge verwenden tatsächliche Katalognamen und können
leere Namen enthalten; Eingabetabellenbezüge behalten ihre bisherigen Grenzen.

`coverage` beschreibt verwendete Katalogabdeckung, keine Garantie vollständiger
Sichtbarkeit unter beliebigen DB-Privilegien. Mehrere Abfragen bilden keinen
atomaren Schemasnapshot. Gleichzeitige DDL kann Fehler oder zeitlich gemischte
Metadaten ergeben. Constraint- und Indexnamen begründen keine Eigenschaft;
Primär-/Unique-/FK-Eigenschaften stammen aus Katalogfeldern.
Constraintnamen sind nicht global eindeutig: insbesondere können MySQL und MariaDB
gleiche Namen in verschiedenen Typnamensräumen zulassen. Jeder Typ-/Namenseintrag
bleibt getrennt; ein Name allein eignet sich nicht als portable Objektidentität.

| Treiber | Belegte Struktur | Ausdrückliche Grenzen |
| --- | --- | --- |
| SQLite | `index_list`/`index_xinfo`: Unique, PK-Ursprung, partielle Indizes, Spalten/Ausdruck/Rowid, Schlüssel-/Hilfsterme, Richtung und Kollation; PK-/Unique-/FK-Spalten | Constraints nur teilweise; CHECK unavailable ohne SQL-Parser; Constraintnamen, Deferrability, Validierung und Enforcement unbekannt. Implizite FK-Zielspalten werden nicht aus einem Eltern-PK geraten. Rowid-PK ohne physischen Index erzeugt keinen erfundenen Index. Indexmethode unbekannt. |
| MySQL/MariaDB | Information Schema: native Indexmethode, Richtung, Spaltenkollation, Präfixlänge, PK-/Unique-/FK-/CHECK-Typen, Zielbezüge und FK-Aktionen | Ausdrucksmarkierung und Enforcement nur bei tatsächlich vorhandenen Katalogspalten; sonst unbekannt. `STATISTICS.COLLATION` A/D ist Richtung, keine Kollation. Präfixlänge bleibt native Zahl mit `prefix_unit: unknown`, keine portable Zeichen-/Bytezusage. Partielle Indizes, Validierung und NULL-Eindeutigkeitssemantik unbekannt. |
| PostgreSQL | pg_index/pg_constraint: Methode, Schlüssel/INCLUDE, Ausdrücke, partielle/valide Indizes, Kollationen, Richtung, PK/Unique/FK/CHECK, Deferrability/Validierung | NULLS-DISTINCT und Enforcement nur bei nachgewiesenen Versionsfeldern, sonst unbekannt. Keine Prädikate oder CHECK-Ausdruckstexte. Katalogspalten unbekannter Constraintarten werden nicht interpretiert. |

Die Antwort ist auf je 1024 Indizes/Constraints, 1024 Terme je Index bzw.
Spalten je Constraintliste und insgesamt 8192 Indexterme plus Quell-/Zielspalten
begrenzt. Einzelne Metadatenstrings dürfen höchstens 1 MiB UTF-8-Rohbytes haben;
die tatsächliche JSON-Antwort einschließlich Escaping und abschließendem Newline
höchstens 8 MiB. Überschreitungen ergeben `result_too_large`, keine still gekürzte
Struktur. Die Engine begrenzt die Laufzeit auf zehn Sekunden und verwendet die
bestehende globale Parallelitätsgrenze und Verbindungsfreigabe. Das HTTP-Bodylimit
bleibt 1 MiB. Abbruch/Fehler liefern keine teilweise erfolgreiche Antwort.

Primärquellen: [SQLite PRAGMA](https://www.sqlite.org/pragma.html),
[MySQL STATISTICS](https://dev.mysql.com/doc/refman/8.4/en/information-schema-statistics-table.html),
[MariaDB STATISTICS](https://mariadb.com/docs/server/reference/system-tables/information-schema/information-schema-tables/information-schema-statistics-table),
[PostgreSQL pg_index](https://www.postgresql.org/docs/18/catalog-pg-index.html),
[PostgreSQL pg_constraint](https://www.postgresql.org/docs/18/catalog-pg-constraint.html).
