# phpMyAdmin-Paritätsmatrix

Referenzrecherche: 20.10.2026, Paket P1a unabhängig geprüft und integriert.
Implementierungscheckpoint: 21.10.2026, siehe Teilnachweise unten.
**Keine erreichte Parität und keine freigegebene Implementierungsreihenfolge.**

## Referenz und Geltung

Als konkrete Arbeitsreferenz wird **phpMyAdmin 5.2.3**, veröffentlicht am 08.10.2025,
vorgeschlagen. Die [offizielle Downloadseite](https://www.phpmyadmin.net/downloads/)
führt am Abrufdatum 5.2.3. Die [versionsgebundene Einführung](https://docs.phpmyadmin.net/en/release_5_2_3/intro.html)
weist ausdrücklich 5.2.3 aus. `latest` zeigt dagegen 5.2.4-dev und ist keine stabile Referenz.
Der offizielle Git-Tag `RELEASE_5_2_3` wurde mit `git ls-remote` aufgelöst:
Tagobjekt `da8def9bce141e3fa08d38612fd3dfdc4c2ae59a`, Commit
`962857e4f63d42e38f11ff4d63f5e722018add76`. Quellcodeverweise unten verwenden diesen
unveränderlichen Commit. Eine kryptografische Signaturprüfung des Tags wurde nicht durchgeführt.

Das Produktziel aus der [Projektübergabe](../PROJEKTUEBERGABE.md) bleibt mindestens
der gesamte phpMyAdmin-Funktionsumfang. Dieses Inventar macht den recherchierten Umfang
prüfbar; eine Referenzfestlegung als Architekturentscheidung und die endgültige
Vollständigkeitsabnahme stehen aus. Die neu bestätigten Betriebsziele **Docker Compose
und Docker Swarm** ergänzen die Produktanforderungen, sind keine aus phpMyAdmin
abgeleiteten Funktionen. Die eigenständige Apple-App darf für lokale DB-Funktionen
kein Go-Backend voraussetzen.

## Leseschlüssel und gemeinsame Abnahme

- IDs `PMA-<Bereich>-<Nummer>` bleiben stabil. Ergänzungen erhalten neue IDs; Aufteilungen
  dokumentieren die Vorgänger-ID. Kein Eintrag verschwindet durch eine niedrigere Priorität.
- DB `M` bedeutet MySQL und MariaDB, jeweils nur soweit die konkrete Serverversion,
  Engine und Kontorechte die Operation unterstützen. Versionsgrenzen müssen vor
  Implementierung durch P2 und konkrete DB-Matrizen ergänzt werden. PostgreSQL/SQLite
  erhalten eigene native Fähigkeiten; MySQL-SQL wird dort nicht blind wiederverwendet.
- Plattform `W/A` meint Web über Go sowie die eigenständige App auf macOS und iPadOS
  mit direkter Verbindung. `W/A?` markiert einen noch zu entscheidenden funktionalen
  Ersatz für server- oder browserspezifische Mechanismen; das ist keine genehmigte Ausnahme.
- Prioritäten sind ausschließlich **Vorschläge**: `V1` durchgehender Grundpfad,
  `V2` persönlicher Client, `V3` Administration, `V4` erweiterte Parität. Alle gehören
  weiterhin zum Endziel; weder Termin noch Releaseumfang folgt daraus.
- Stand `O` heißt: recherchiertes Soll, vollständige gemeinsame Abnahme über alle
  genannten Varianten und Plattformen offen. Einzelne Teilfunktionen sind inzwischen
  implementiert und geprüft; sie stehen in den Teilnachweisen unten und STATUS.
  Ein Go-/Web-Teilnachweis erfüllt keine native MySQL-/iPad-Abnahme derselben Zeile.
- Die Soll- und Testtexte sind eigene Anforderungsableitungen aus den angegebenen
  Primärquellen, keine Behauptung über identisches Verhalten aller phpMyAdmin-Konfigurationen.
  Ein vorhandener Route-Name allein belegt nur den Einstieg; Details sind bei Umsetzung
  gegen Implementierung und eine laufende Referenz nachzuprüfen.
- Jeder Abnahmefall wird mit synthetischen Daten gegen jede zugesagte DB-Version und
  getrennt auf W, macOS direkt und iPadOS direkt ausgeführt. Backendabhängige Zusatzfälle
  werden gesondert belegt. Ein Go-Test belegt keine Swift-/Gerätefunktion.
- Mutationen werden durch unabhängiges Nachlesen in der DB geprüft. Fehlende Rechte,
  ungültige Eingaben und nicht unterstützte Fähigkeiten ergeben verständliche Fehler
  und keinen erfundenen Erfolg. Mehrteilige Operationen melden Teilerfolge; DDL ist
  nicht pauschal transaktional. Destruktive Aktionen zeigen Ziel und Umfang vor Ausführung.
- Abnahmeevidenz muss künftig Test-ID, Fixture, DB-/OS-/Client-/Vertragsversion,
  Befehl oder reproduzierbaren Bedienablauf, Datum und Ergebnis enthalten. Erst diese
  Evidenz erlaubt `bestanden`; nicht anwendbar braucht begründete akzeptierte Entscheidung.

## Primärquellen

Alle Quellen wurden am 20.10.2026 abgerufen. Dokumentation wurde über die offiziellen
versionsgebundenen Seiten gelesen; zusätzlich offizieller Tag in temporärem Checkout,
Routen, Controller, Exportoptionen und Plugininventar untersucht. Kein Fremdcode
wurde in das Projekt übernommen. Dokumentation und Implementierung ergänzen sich;
Konfigurationsspeicher und optionale Abhängigkeiten begrenzen manche Referenzfunktionen.

| Kürzel | Quelle / Bedeutung |
| --- | --- |
| INTRO | [Einführung 5.2.3](https://docs.phpmyadmin.net/en/release_5_2_3/intro.html): Funktionsüberblick, Tastatur und DB-Konten |
| CONFIG | [Konfiguration 5.2.3](https://docs.phpmyadmin.net/en/release_5_2_3/config.html): Navigation, Konsolen-/Anzeigeoptionen und Konfigurationsspeicher |
| SETUP | [Installation 5.2.3](https://docs.phpmyadmin.net/en/release_5_2_3/setup.html): mehrere Server, Authentifizierungsarten, TLS, erweiterter Speicher |
| IO | [Import/Export 5.2.3](https://docs.phpmyadmin.net/en/release_5_2_3/import_export.html): Formate und Übertragungswege |
| REL | [Beziehungen 5.2.3](https://docs.phpmyadmin.net/en/release_5_2_3/relations.html): native/interne Beziehungen und Designer |
| BOOK | [Lesezeichen 5.2.3](https://docs.phpmyadmin.net/en/release_5_2_3/bookmarks.html): Abfragen, Variablen, Sichtbarkeit |
| PRIV | [DB-Benutzerverwaltung 5.2.3](https://docs.phpmyadmin.net/en/release_5_2_3/privileges.html): Konten und Rechte, keine eigene DB-Rechteschicht |
| TRANS | [Transformationen 5.2.3](https://docs.phpmyadmin.net/en/release_5_2_3/transformations.html): Darstellung und Eingabe getrennt |
| CHART | [Diagramme 5.2.3](https://docs.phpmyadmin.net/en/release_5_2_3/charts.html): Typen, Achsen und Serien |
| AUTH | [Zweiter Faktor 5.2.3](https://docs.phpmyadmin.net/en/release_5_2_3/two_factor.html): TOTP, U2F und Wiederherstellung |
| FAQ | [FAQ 5.2.3](https://docs.phpmyadmin.net/en/release_5_2_3/faq.html): Grenzen, GIS, Exporte und große Dateien |
| ROUTES | [Routen am Referenzcommit](https://github.com/phpmyadmin/phpmyadmin/blob/962857e4f63d42e38f11ff4d63f5e722018add76/libraries/routes.php): Einstiegspunkte, kein Laufzeitnachweis |

In den Tabellen bezeichnet ein verlinktes Quellkürzel die konkret passende Datei am
Referenzcommit. Allgemeine Kürzel ohne Link verweisen auf die Quellentabelle oben.

## Verbindungen und Navigation

Quelle: SETUP, CONFIG, INTRO. Gemeinsamer Stand aller Zeilen: `O`.

| ID | Referenzfunktion | Sollverhalten | DB; Plattform | Prio-Vorschlag | Abnahmekriterium | Stand |
| --- | --- | --- | --- | --- | --- | --- |
| PMA-CON-001 | Mehrere Server | Server auswählen und Verwechslungen durch sichtbaren Kontext vermeiden | M; W/A | V1 | Zwei Server mit gleichnamigen Tabellen zeigen nach Wechsel ihre unterschiedlichen Fixturewerte. | O |
| PMA-CON-002 | Verbindung und DB-Anmeldung | Host, Port, Socket soweit verfügbar und Konto verwenden; Geheimnis bei Bedarf abfragen | M; W/A | V1 | Gültige Anmeldung liest einen Wert; falsches Passwort liefert keinen Datenzugriff. | O |
| PMA-CON-003 | TLS-Verbindung | CA und Serveridentität prüfen; explizite Clientzertifikate anbieten; bei vorgeschriebenem TLS niemals auf Klartext zurückfallen | M; W/A | V1 | Vertrauenswürdiges Zertifikat verbindet; falscher Host und unbekannte CA scheitern. Ein TLS-unfähiger Server wird bei vorgeschriebenem TLS vor Übertragung von Credentials abgewiesen; ein Protokollmitschnitt der isolierten Fixture belegt, dass keine Zugangsdaten oder Authentifizierungsantwort gesendet werden. | O |
| PMA-CON-004 | Verbindungszeichensatz | Zeichensatz/Kollation der Sitzung anzeigen und wählbar setzen | M; W/A | V2 | UTF-8-Text einschließlich Emoji kommt unverändert zurück; aktive Sitzungseinstellung ist nachlesbar. | O |
| PMA-CON-005 | Objektbaum | Datenbanken und Objekte nachladen, suchen und gruppiert darstellen | M; W/A | V1 | Fixture mit zwei Datenbanken und mehreren Objekttypen wird vollständig und korrekt zugeordnet geladen. | O |
| PMA-CON-006 | Favoriten und zuletzt genutzt | Tabellen markieren und zuletzt geöffnete Tabellen wiederfinden | M; W/A | V2 | Favorit bleibt nach Neustart; getrennte Konten sehen keine fremde Liste. | O |
| PMA-CON-007 | Abmelden | Sitzung beenden und Zugriff ohne erneute Anmeldung unterbinden | M; W/A | V1 | Nach Abmeldung gelingt ein erneuter Datenzugriff erst nach Authentifizierung. | O |
## Datenbanken und Tabellen

Quelle: [ROUTES](https://github.com/phpmyadmin/phpmyadmin/blob/962857e4f63d42e38f11ff4d63f5e722018add76/libraries/routes.php). Gemeinsamer Stand aller Zeilen: `O`.

| ID | Referenzfunktion | Sollverhalten | DB; Plattform | Prio-Vorschlag | Abnahmekriterium | Stand |
| --- | --- | --- | --- | --- | --- | --- |
| PMA-OBJ-001 | Datenbankliste | Name, Kollation und Größen-/Objektinformationen darstellen | M; W/A | V1 | Zwei Fixture-Datenbanken erscheinen; Schätzwerte sind als solche erkennbar. | O |
| PMA-OBJ-002 | Datenbank anlegen | Name und Kollation verwenden | M; W/A | V2 | Neue Datenbank erscheint im unabhängigen Metadatenabruf mit gewählter Kollation. | O |
| PMA-OBJ-003 | Datenbank entfernen | Explizit gewählte Datenbank löschen | M; W/A | V3 | Nach bestätigtem Löschen fehlt nur die gewählte Wegwerf-Datenbank. | O |
| PMA-OBJ-004 | Datenbank kopieren/umbenennen | Struktur/Daten/Objekte nachvollziehbar übertragen; Servergrenzen beachten | M; W/A | V3 | Kopie enthält gewählte Objekte und Daten; Umbenennen benennt Teilschritte und verbleibende Quelle korrekt. | O |
| PMA-OBJ-005 | Datenbankkollation | Standard und wahlweise betroffene Tabellen ändern | M; W/A | V3 | Metadaten belegen gewählten Umfang; nicht gewählte Tabellen bleiben unverändert. | O |
| PMA-OBJ-006 | Tabellenübersicht | Objekttyp, Zeilen, Engine, Kollation und Größe zeigen | M; W/A | V1 | Tabelle und View sind unterscheidbar; exakte Zählung und Schätzung werden getrennt. | O |
| PMA-OBJ-007 | Tabelle anlegen | Spalten, Primärschlüssel, Engine und Kollation festlegen | M; W/A | V2 | SHOW CREATE der Fixture enthält alle gewählten Attribute. | O |
| PMA-OBJ-008 | Tabelle umbenennen/verschieben | Tabelle innerhalb oder zwischen Datenbanken übertragen | M; W/A | V3 | Daten und Definition sind am Ziel lesbar; altes Objekt fehlt nach erfolgreichem Verschieben. | O |
| PMA-OBJ-009 | Tabelle kopieren | Nur Struktur, nur Daten oder beides auswählbar kopieren | M; W/A | V2 | Jeder der drei Modi erzeugt die vereinbarte Definition und Zeilenanzahl. | O |
| PMA-OBJ-010 | Tabelle leeren | Daten entfernen und DDL-/Auto-Inkrement-Folgen ausweisen | M; W/A | V3 | Zeilenzahl ist null; Tabelle existiert weiter; beobachtete Auto-Inkrement-Folge wird korrekt angezeigt. | O |
| PMA-OBJ-011 | Tabelle entfernen | Gewähltes Objekt nach Bestätigung löschen | M; W/A | V3 | Gewählte Tabelle fehlt; zweite Fixturetabelle besteht unverändert. | O |
| PMA-OBJ-012 | Tabellenoptionen | Engine, Kommentar, Kollation und Auto-Inkrement einstellen | M; W/A | V3 | Metadaten spiegeln jede unterstützte Einstellung; ungültige Engine liefert Fehler. | O |
| PMA-OBJ-013 | Präfix-Stapelaktionen | Tabellenpräfix hinzufügen, ersetzen und beim Kopieren verwenden | M; W/A | V4 | Drei ausgewählte Tabellen erhalten erwartete Namen; ungewählte Namen bleiben erhalten. | O |
| PMA-OBJ-014 | Definition anzeigen | CREATE-Anweisung von Tabelle oder View abrufen | M; W/A | V2 | Anweisung lässt sich in leerem Wegwerfschema ausführen und erzeugt äquivalente Struktur. | O |
## Spalten, Indizes und Partitionierung

Quelle: [Struktur-Controller](https://github.com/phpmyadmin/phpmyadmin/tree/962857e4f63d42e38f11ff4d63f5e722018add76/libraries/classes/Controllers/Table/Structure). Gemeinsamer Stand aller Zeilen: `O`.

| ID | Referenzfunktion | Sollverhalten | DB; Plattform | Prio-Vorschlag | Abnahmekriterium | Stand |
| --- | --- | --- | --- | --- | --- | --- |
| PMA-DDL-001 | Spalten anlegen | Typ, Länge, NULL, Default und Position setzen | M; W/A | V2 | Metadaten stimmen für NULL-, Literal- und servergestützte Defaultfälle. | O |
| PMA-DDL-002 | Spalten ändern | Name, Typ, Attribute, Kommentar und Kollation ändern | M; W/A | V2 | Änderung ist nachlesbar; ungültige Konvertierung meldet Warnung/Fehler und tatsächlichen Datenstand. | O |
| PMA-DDL-003 | Spalten umordnen | Reihenfolge ändern | M; W/A | V3 | Metadaten liefern die gewählte Reihenfolge bei unveränderten Werten. | O |
| PMA-DDL-004 | Spalten entfernen | Gewählte Spalten samt Folgen für abhängige Objekte behandeln | M; W/A | V3 | Nur gewählte Spalte fehlt oder DB verweigert mit nachvollziehbarem Abhängigkeitsfehler. | O |
| PMA-DDL-005 | ENUM und SET | Wertelisten definieren und passend bearbeiten | M; W/A | V2 | Werte mit Komma und Apostroph lassen sich verlustfrei definieren und lesen. | O |
| PMA-DDL-006 | Generierte Spalten | Ausdruck und unterstützten Speichermodus verwalten | M; W/A | V3 | Server berechnet Fixtureausdruck; Versuch direkter Änderung wird korrekt abgewiesen. | O |
| PMA-DDL-007 | Primärschlüssel | Ein- und mehrspaltige Primärschlüssel setzen/entfernen | M; W/A | V2 | Duplikat wird abgewiesen; Schlüsselreihenfolge ist nachlesbar. | O |
| PMA-DDL-008 | Sekundärindex | Index mit Spaltenreihenfolge und Präfixlängen erstellen | M; W/A | V2 | Metadaten bestätigen Art, Reihenfolge und Präfixlänge. | O |
| PMA-DDL-009 | Unique-Index | Eindeutigkeit mit mehreren Spalten definieren | M; W/A | V2 | Doppelte Schlüsselkombination scheitert serverseitig. | O |
| PMA-DDL-010 | Volltext-/Spatial-Index | Spezielle Indexarten nach Enginefähigkeit anbieten | M; W/A | V3 | SHOW INDEX weist gewählte Art aus; nicht geeigneter Typ führt zu verständlichem Fehler. | O |
| PMA-DDL-011 | Index ändern/entfernen | Umbenennen und Löschen unterstützen | M; W/A | V3 | Neuer Name ist sichtbar; gelöschter Index fehlt ohne Datenverlust. | O |
| PMA-DDL-012 | Partitionierung definieren | Methode, Ausdruck und Partitionen verwalten | M; W/A | V4 | Metadaten stimmen mit Definition überein; Beispielwerte liegen in erwarteten Partitionen. | O |
| PMA-DDL-013 | Partition warten | ANALYZE, CHECK, OPTIMIZE, REBUILD, REPAIR nach Fähigkeit ausführen | M; W/A | V4 | Jede unterstützte Aktion meldet Serverresultat pro gewählter Partition; Ablehnungen bleiben sichtbar. | O |
| PMA-DDL-014 | Partition leeren/entfernen | TRUNCATE/DROP auf gewählte Partition begrenzen | M; W/A | V4 | Nur Daten/Partition der gewählten Fixture werden entfernt; andere Partition bleibt korrekt. | O |
## Daten anzeigen, bearbeiten und suchen

Quelle: [Tabellen-Controller](https://github.com/phpmyadmin/phpmyadmin/tree/962857e4f63d42e38f11ff4d63f5e722018add76/libraries/classes/Controllers/Table). Gemeinsamer Stand aller Zeilen: `O`.

| ID | Referenzfunktion | Sollverhalten | DB; Plattform | Prio-Vorschlag | Abnahmekriterium | Stand |
| --- | --- | --- | --- | --- | --- | --- |
| PMA-DATA-001 | Begrenzte Tabellenansicht | Spalten und paginierte Zeilen laden | M; W/A | V1 | Fixture mit mehr Zeilen als Seitenlimit wird lückenfrei bei stabiler eindeutiger Sortierung durchblättert. | O |
| PMA-DATA-002 | Sortieren und Spaltenansicht | Sortierreihenfolge sowie sichtbare Spalten wählen | M; W/A | V2 | Auf-/absteigende Mehrspaltensortierung und Ein-/Ausblenden passen zur Fixture. | O |
| PMA-DATA-003 | NULL und Binärwerte | NULL, Leerstring, Nullbyte und Binärwerte unterscheidbar anzeigen | M; W/A | V1 | Vier unterschiedliche Fixturewerte bleiben auch nach erneutem Lesen unterscheidbar. | O |
| PMA-DATA-004 | Zeile einfügen | Formularwerte, Defaults, NULL und serverseitige Funktionen einsetzen | M; W/A | V2 | Neue Zeile enthält exakte Werte und erwarteten automatisch erzeugten Schlüssel. | O |
| PMA-DATA-005 | Zeile bearbeiten | Formular und direkte Zellenänderung ermöglichen | M; W/A | V2 | Änderung betrifft nur eindeutig gewählte Zeile; Tabelle ohne Schlüssel behauptet keine Eindeutigkeit. | O |
| PMA-DATA-006 | Mehrere Zeilen bearbeiten | Auswahl bearbeiten und tatsächliche Teilerfolge melden | M; W/A | V2 | Zwei gültige und eine ungültige Änderung liefern nachvollziehbaren Endstand ohne falsche Atomaritätszusage. | O |
| PMA-DATA-007 | Zeile duplizieren | Vorhandene Zeile als bearbeitbare Einfügevorlage verwenden | M; W/A | V2 | Duplikat erhält neuen Schlüssel und gewählte Änderungen; Quellzeile bleibt erhalten. | O |
| PMA-DATA-008 | Zeilen löschen | Einzel- und Mehrfachauswahl gezielt entfernen | M; W/A | V2 | Nur bestätigte Fixturezeilen fehlen; Restzahl stimmt. | O |
| PMA-DATA-009 | Lange Texte und Dateien | Volltext öffnen, Daten hochladen und Binärwert herunterladen | M; W/A | V2 | Datei mit Nullbytes hat nach Upload/Download dieselbe Prüfsumme. | O |
| PMA-DATA-010 | Datensatzsuche | Spaltenoperatoren und Wertbedingungen kombinieren | M; W/A | V2 | Gleichheit, Bereich, LIKE und NULL liefern die vorab definierten Fixturetreffer. | O |
| PMA-DATA-011 | Datenbankweite Suche | In gewählten Tabellen/Spalten suchen | M; W/A | V3 | Nur ausgewählte Tabellen liefern Treffer; Zahl und Drilldown stimmen. | O |
| PMA-DATA-012 | Suchen und Ersetzen | Vorschau und gezielte Textersetzung anbieten | M; W/A | V3 | Vorschau zählt erwartete Treffer; bestätigte Ersetzung ändert nur gewählte Spalte/Zeilen. | O |
| PMA-DATA-013 | Zoom-Suche | Zwei Spalten visuell untersuchen und Punkte zu Datensätzen zuordnen | M; W/A | V4 | Fixturepunkte liegen korrekt; Auswahl öffnet den zugehörigen Datensatz. | O |
| PMA-DATA-014 | Räumliche Daten | Geometrie bearbeiten und räumlich visualisieren | M; W/A | V4 | Punkt und Polygon werden mit passender Geometrie/SRID gespeichert und korrekt dargestellt. | O |
## SQL, Ergebnisse und gespeicherte Abfragen

Quelle: [SQL-Ausführung](https://github.com/phpmyadmin/phpmyadmin/blob/962857e4f63d42e38f11ff4d63f5e722018add76/libraries/classes/Sql.php), BOOK. Gemeinsamer Stand aller Zeilen: `O`.

| ID | Referenzfunktion | Sollverhalten | DB; Plattform | Prio-Vorschlag | Abnahmekriterium | Stand |
| --- | --- | --- | --- | --- | --- | --- |
| PMA-SQL-001 | SQL ausführen | Abfrage im expliziten Server-/Datenbankkontext ausführen | M; W/A | V1 | SELECT des Datenbanknamens und eines Fixturewerts zeigt den gewählten Kontext. | O |
| PMA-SQL-002 | SQL-Stapel | Mehrere Anweisungen und Routine-Delimiter korrekt verarbeiten | M; W/A | V2 | Stapel mit Semikolon im String und Routinekörper wird ohne falsche Trennung ausgeführt. | O |
| PMA-SQL-003 | Mehrere Ergebnissätze | Ergebnisse und Status jeder Anweisung zuordnen | M; W/A | V2 | SELECT, UPDATE, SELECT zeigt zwei getrennte Tabellen und korrekte Änderungszahl. | O |
| PMA-SQL-004 | Fehler und Warnungen | Servercode, verständlichen Fehler und betroffene Anweisung zeigen | M; W/A | V1 | Ungültiges SQL zeigt Fehler; Warnungsfixture liefert separat sichtbare Warnung. | O |
| PMA-SQL-005 | SQL-Editor | Syntaxfarben, Vervollständigung und Formatierung anbieten | M; W/A | V2 | Tabellen-/Spaltennamen mit Sonderzeichen werden korrekt ergänzt; Formatierung ändert Stringinhalt nicht. | O |
| PMA-SQL-006 | EXPLAIN | Ausführungsplan einer geeigneten Abfrage darstellen | M; W/A | V3 | Plan zeigt benutzten Fixtureindex und passt zum direkten DB-Ergebnis. | O |
| PMA-SQL-007 | Profiling | Unterstützte Laufzeit-/Profildaten anzeigen | M; W/A | V4 | Server mit Unterstützung liefert Messwerte; fehlende Unterstützung wird explizit kenntlich gemacht. | O |
| PMA-SQL-008 | SQL-Verlauf | Ausgeführte Abfragen wiederfinden und erneut öffnen | M; W/A | V2 | Abfrage erscheint im richtigen Konto-/Serverkontext; Verlauf enthält keine fremden Kontoeinträge. | O |
| PMA-SQL-009 | Lesezeichen verwalten | Abfragen benennen, öffnen und löschen | M; W/A | V2 | Gespeicherte Abfrage ist nach Neustart verfügbar und nach Löschen nicht mehr auswählbar. | O |
| PMA-SQL-010 | Lesezeichenvariablen | Wiederverwendbare Abfrage mit expliziten Eingaben ausführen | M; W/A | V3 | Zwei Eingaben erzeugen erwartete unterschiedliche Resultate; gespeicherte Vorlage bleibt erhalten. | O |
| PMA-SQL-011 | Lesezeichensichtbarkeit | Private und ausdrücklich gemeinsame Vorlagen trennen | M; W/A? | V3 | Zweites Konto sieht private Vorlage nicht; freigegebene Vorlage ist im festgelegten Bereich zugänglich. | O |
| PMA-SQL-012 | Ergebnisoperationen | Ergebnis exportieren, drucken und als View verwenden | M; W/A | V3 | Export enthält gewählte Spalten/Zeilen; angelegte View liefert dieselben Fixturewerte. | O |
| PMA-SQL-013 | DML-Simulation | Geplante UPDATE-/DELETE-Wirkung vor Mutation untersuchen | M; W/A | V3 | Vorschau stimmt für Fixture mit tatsächlicher späterer Änderung überein und verändert bei Vorschau keine Zeile. | O |
## Views, Routinen, Trigger und Events

Quelle: [Datenbank-Controller](https://github.com/phpmyadmin/phpmyadmin/tree/962857e4f63d42e38f11ff4d63f5e722018add76/libraries/classes/Controllers/Database), INTRO. Gemeinsamer Stand aller Zeilen: `O`.

| ID | Referenzfunktion | Sollverhalten | DB; Plattform | Prio-Vorschlag | Abnahmekriterium | Stand |
| --- | --- | --- | --- | --- | --- | --- |
| PMA-PROG-001 | Views anlegen/ändern | Definition, Sicherheitskontext und unterstützte Optionen bearbeiten | M; W/A | V2 | View liefert berechnete Fixturewerte; geänderte Definition ist nachlesbar. | O |
| PMA-PROG-002 | Views entfernen | Gewählte View löschen | M; W/A | V3 | View fehlt, ihre Basistabelle bleibt unverändert. | O |
| PMA-PROG-003 | Prozeduren verwalten | Definition, Parameter und Eigenschaften anlegen, ändern, exportieren, löschen | M; W/A | V3 | Prozedur mit IN/OUT-Parametern übersteht Export/Import und wird gezielt entfernt. | O |
| PMA-PROG-004 | Funktionen verwalten | Rückgabetyp und Funktionskörper verwalten | M; W/A | V3 | Fixturefunktion liefert erwarteten Wert; geänderte Definition und Export sind reproduzierbar. | O |
| PMA-PROG-005 | Routine aufrufen | Parameter übergeben und Rückgaben/mehrere Ergebnisse anzeigen | M; W/A | V3 | Prozedur liefert zwei Ergebnissätze und OUT-Wert korrekt. | O |
| PMA-PROG-006 | Trigger verwalten | Zeitpunkt, Ereignis und Körper erstellen, ändern, exportieren, löschen | M; W/A | V3 | INSERT erzeugt erwartete Audit-Fixture; nach Entfernung entfällt nur diese Triggerwirkung. | O |
| PMA-PROG-007 | Events verwalten | Zeitplan, Status und Körper erstellen, ändern, exportieren, löschen | M; W/A | V3 | Metadaten enthalten richtigen Zeitplan/Status; isolierter Schedulerlauf zeigt erwartete einmalige Wirkung. | O |
## DB-Konten und Rechte

Quelle: PRIV, [Privilegien](https://github.com/phpmyadmin/phpmyadmin/blob/962857e4f63d42e38f11ff4d63f5e722018add76/libraries/classes/Server/Privileges.php). Gemeinsamer Stand aller Zeilen: `O`.

| ID | Referenzfunktion | Sollverhalten | DB; Plattform | Prio-Vorschlag | Abnahmekriterium | Stand |
| --- | --- | --- | --- | --- | --- | --- |
| PMA-PRIV-001 | DB-Konten auflisten | Konto und Hostzuordnung sowie Rechte anzeigen | M; W/A | V3 | Zwei gleichnamige Konten verschiedener Hosts bleiben unterscheidbar. | O |
| PMA-PRIV-002 | DB-Konten anlegen | Benutzer, Host, Authentifizierung und optional eigene Datenbank setzen | M; W/A | V3 | Neues Wegwerfkonto kann sich anmelden und sieht nur eingeräumte Datenbankrechte. | O |
| PMA-PRIV-003 | DB-Konten bearbeiten/kopieren | Host/Kontoeigenschaften gezielt ändern oder Rechte als Vorlage nutzen | M; W/A | V3 | Neue Zuordnung und Kopie besitzen die gewählten Rechte; Ausgangskonto bleibt wie vorgesehen. | O |
| PMA-PRIV-004 | Passwort ändern | Eigenes oder berechtigt fremdes DB-Passwort ändern | M; W/A | V3 | Altes Passwort scheitert, neues funktioniert; unberechtigter Benutzer kann fremdes Passwort nicht ändern. | O |
| PMA-PRIV-005 | DB-Konten entfernen | Konto samt wählbaren Folgedaten entfernen | M; W/A | V3 | Gelöschtes Konto kann nicht neu anmelden; vereinbarter Datenbankumfang ist eingehalten. | O |
| PMA-PRIV-006 | Globale Rechte | Globale Privilegien und GRANT OPTION verwalten | M; W/A | V3 | Frische Sitzung kann genau die neu gewährte Aktion; Entzug wird in neuer Sitzung wirksam. | O |
| PMA-PRIV-007 | Objektrechte | Datenbank-, Tabellen-, Spalten- und Routineprivilegien setzen | M; W/A | V3 | Je Ebene gibt es einen erlaubten und verweigerten Fixturefall. | O |
| PMA-PRIV-008 | Ressourcen-/TLS-Vorgaben | Unterstützte Limits und TLS-Anforderungen pro Konto einstellen | M; W/A | V4 | SHOW GRANTS/Metadaten stimmen; unverschlüsselte Anmeldung scheitert bei REQUIRE SSL. | O |
| PMA-PRIV-009 | Konto sperren/entsperren | Serverseitige Kontosperre verwalten | M; W/A | V3 | Neue Anmeldung scheitert während Sperre und gelingt nach Entsperren. | O |
| PMA-PRIV-010 | Rechte neu laden | Berechtigt Privilegien neu laden | M; W/A | V3 | Server bestätigt Aktion; fehlendes Recht liefert konkreten Fehler. | O |
## Serverzustand, Monitoring und Wartung

Quelle: [Server-Controller](https://github.com/phpmyadmin/phpmyadmin/tree/962857e4f63d42e38f11ff4d63f5e722018add76/libraries/classes/Controllers/Server). Gemeinsamer Stand aller Zeilen: `O`.

| ID | Referenzfunktion | Sollverhalten | DB; Plattform | Prio-Vorschlag | Abnahmekriterium | Stand |
| --- | --- | --- | --- | --- | --- | --- |
| PMA-ADMIN-001 | Serverinformationen | Version, Laufzeit und relevante Verbindungsdaten darstellen | M; W/A | V1 | Versions-/Serverwerte stimmen mit direkter DB-Abfrage überein. | O |
| PMA-ADMIN-002 | Statusmetriken | Zähler und Abfragetypen darstellen | M; W/A | V3 | Synthetische Last erhöht passenden Zähler; Messzeitpunkt ist sichtbar. | O |
| PMA-ADMIN-003 | Systemvariablen | Globale/Sitzungswerte zeigen und berechtigt ändern | M; W/A | V3 | Gewählte änderbare Variable ist nach Änderung direkt nachlesbar; schreibgeschützte wird abgewiesen. | O |
| PMA-ADMIN-004 | Prozessliste | Aktive Verbindungen/Abfragen anzeigen und aktualisieren | M; W/A | V3 | Laufende lange Fixtureabfrage erscheint mit korrekter Verbindung. | O |
| PMA-ADMIN-005 | Prozess/Abfrage beenden | Gewählten Prozess berechtigt abbrechen | M; W/A | V3 | Nur identifizierte Fixtureabfrage endet; zweite Verbindung bleibt verwendbar. | O |
| PMA-ADMIN-006 | Live-Monitor | Messwerte im Zeitverlauf und Abfrage-/Loganalyse anbieten | M; W/A | V4 | Vorhersehbare Lastspitze erscheint im gewählten Zeitfenster; fehlende Logrechte werden ausgewiesen. | O |
| PMA-ADMIN-007 | Advisor | Zustandsbasierte Hinweise mit Begründung anzeigen | M; W/A | V4 | Definierte Regel-Fixture löst passenden Hinweis mit zugrunde liegendem Messwert aus. | O |
| PMA-ADMIN-008 | Engines und Plugins | Fähigkeiten, Enginezustand und Pluginliste zeigen | M; W/A | V3 | Listen stimmen mit SHOW ENGINES/PLUGINS überein. | O |
| PMA-ADMIN-009 | Zeichensätze und Kollationen | Serverfähige Zeichensätze/Kollationen auffinden | M; W/A | V3 | Gewählte Kollation ist im direkten Serverinventar vorhanden. | O |
| PMA-ADMIN-010 | Binärlog | Verfügbare Logs und deren Ereignisse durchsuchen | M; W/A | V4 | Bekannte synthetische Änderung ist im gewählten Log auffindbar; kein Log liefert klaren Zustand. | O |
| PMA-ADMIN-011 | Replikationsstatus | Primary-/Replica-Zustand und Konfiguration anzeigen | M; W/A | V4 | Isoliertes DB-Paar zeigt passenden Status und dokumentierte Versionsbegriffe. | O |
| PMA-ADMIN-012 | Replikation steuern | Replica konfigurieren/starten/stoppen und Position wählen | M; W/A | V4 | Wegwerf-Replikationspaar übernimmt Fixture nach Start und hält nach Stop; Teilschritte werden protokolliert. | O |
## Tabellenwartung

Quelle: [Wartungs-Controller](https://github.com/phpmyadmin/phpmyadmin/tree/962857e4f63d42e38f11ff4d63f5e722018add76/libraries/classes/Controllers/Table/Maintenance). Gemeinsamer Stand aller Zeilen: `O`.

| ID | Referenzfunktion | Sollverhalten | DB; Plattform | Prio-Vorschlag | Abnahmekriterium | Stand |
| --- | --- | --- | --- | --- | --- | --- |
| PMA-MAINT-001 | CHECK TABLE | Tabellenprüfung starten und pro Objekt berichten | M; W/A | V3 | Resultat entspricht direktem CHECK der gewählten Tabelle. | O |
| PMA-MAINT-002 | ANALYZE TABLE | Statistikanalyse starten | M; W/A | V3 | Serverstatus wird vollständig gezeigt; fehlende Rechte führen zu keinem Erfolg. | O |
| PMA-MAINT-003 | OPTIMIZE TABLE | Unterstützte Optimierung ausführen | M; W/A | V3 | Engineabhängige Hinweise/Ergebnissätze werden vollständig dargestellt. | O |
| PMA-MAINT-004 | REPAIR TABLE | Reparatur nur nach Enginefähigkeit anbieten | M; W/A | V4 | Unterstützte Wegwerftabelle liefert Serverresultat; InnoDB-Ablehnung ist sichtbar. | O |
| PMA-MAINT-005 | CHECKSUM TABLE | Prüfsumme anzeigen | M; W/A | V4 | Angezeigte Prüfsumme entspricht direkter DB-Abfrage einschließlich NULL-Fall. | O |
| PMA-MAINT-006 | Stapelwartung | Mehrere ausgewählte Tabellen bearbeiten | M; W/A | V4 | Zwei Tabellen und ein Fehlerfall liefern drei korrekt zugeordnete Resultate. | O |
## Importwege und Formate

Quelle: IO, [Importplugins](https://github.com/phpmyadmin/phpmyadmin/tree/962857e4f63d42e38f11ff4d63f5e722018add76/libraries/classes/Plugins/Import). Gemeinsamer Stand aller Zeilen: `O`.

| ID | Referenzfunktion | Sollverhalten | DB; Plattform | Prio-Vorschlag | Abnahmekriterium | Stand |
| --- | --- | --- | --- | --- | --- | --- |
| PMA-IMP-001 | Dateiimport | Datei zum Server-, DB- oder Tabellenkontext zuordnen | M; W/A | V2 | Gleiche Testdatei wird im ausgewählten gültigen Zielkontext importiert. | O |
| PMA-IMP-002 | Drag-and-drop | SQL-Datei per Ziehen oder gleichwertiger Plattformaktion importieren | M; W/A? | V3 | Dateiauswahl und Ziehen/Teilen führen zum gleichen explizit bestätigten Importziel. | O |
| PMA-IMP-003 | Serverseitiges Importverzeichnis | Administrativ angebotene Dateien auswählbar machen | M; W/A? | V4 | Nur freigegebene Fixturedateien erscheinen; Pfadüberschreitung erreicht keine fremde Datei. | O |
| PMA-IMP-004 | Komprimierter Import | GZip, BZip2 und ZIP nach verfügbarer Referenzfähigkeit lesen | M; W/A | V3 | Komprimierte und unkomprimierte Fixture erzeugen dieselben Daten; defektes Archiv meldet Fehler. | O |
| PMA-IMP-005 | SQL-Import | DDL/DML, Routinen und Delimiter aus Dump verarbeiten | M; W/A | V2 | SQL-Dump stellt Tabellen, Werte, Routine und Trigger mit korrekter Wirkung wieder her. | O |
| PMA-IMP-006 | CSV-Import | Trenner, Einfassung, Escape, Header, Zeichensatz und Spaltenzuordnung wählen | M; W/A | V2 | Fixture mit Umlaut, Zeilenumbruch, Trennzeichen und Leerwerten wird nach gewählten Optionen korrekt eingelesen. | O |
| PMA-IMP-007 | CSV-Strukturerkennung | Aus CSV im DB-/Serverkontext Tabelle erzeugen | M; W/A | V3 | Header und Datentypvorschlag sind prüfbar; erzeugte Tabelle enthält alle Fixturewerte. | O |
| PMA-IMP-008 | LOAD DATA | CSV über native Ladefunktion nach Fähigkeit importieren | M; W/A | V3 | Ergebnis stimmt mit normalem CSV-Import überein; deaktiviertes LOCAL INFILE ergibt konkreten Fehler. | O |
| PMA-IMP-009 | ODS-Import | Mehrere Tabellenblätter und Kopfzeilen zuordnen | M; W/A | V4 | Zwei benannte Blätter ergeben passende Tabellen; gespeicherte Formelwerte werden korrekt behandelt. | O |
| PMA-IMP-010 | XML-Import | Referenz-XML mit Struktur und Daten übernehmen | M; W/A | V4 | Referenzexport stellt Fixturestruktur und Daten wieder her; ungültige Struktur wird abgelehnt. | O |
| PMA-IMP-011 | MediaWiki-Import | Referenzformat als Tabelleninhalt einlesen | M; W/A | V4 | Kleine Referenztabelle wird mit erwarteten Spalten und Werten importiert. | O |
| PMA-IMP-012 | ESRI-Shapefile-Import | Geodaten und nötige Begleitdateien importieren | M; W/A | V4 | Fixture mit Attributen und Geometrie erzeugt erwartete Werte; fehlende Begleitdatei meldet Fehler. | O |
| PMA-IMP-013 | Große/unterbrochene Importe | Fortschritt, Fehlerposition und Fortsetzungsgrenzen ausweisen | M; W/A | V3 | Abbruch nach Teilimport verschweigt keine bereits übernommenen Zeilen; Fortsetzung erzeugt keine unbemerkten Duplikate. | O |
| PMA-IMP-014 | Importoptionen | Zeichensatz, Überspringen von Anweisungen und FK-Prüfung explizit steuern | M; W/A | V3 | Jede Option wird mit eigenem Fixturefall geprüft; Sitzungseinstellungen werden nach Abschluss wiederhergestellt. | O |
## Exportwege und Formate

Quelle: IO, [Exportplugins](https://github.com/phpmyadmin/phpmyadmin/tree/962857e4f63d42e38f11ff4d63f5e722018add76/libraries/classes/Plugins/Export). Gemeinsamer Stand aller Zeilen: `O`.

| ID | Referenzfunktion | Sollverhalten | DB; Plattform | Prio-Vorschlag | Abnahmekriterium | Stand |
| --- | --- | --- | --- | --- | --- | --- |
| PMA-EXP-001 | Exportumfang | Server, Datenbank, Tabelle, Zeilenauswahl oder Abfrageergebnis wählen | M; W/A | V2 | Jeder Umfang enthält nur explizit gewählte Fixtureobjekte/-zeilen. | O |
| PMA-EXP-002 | Schnell/angepasst | Schnellexport und detaillierte Formateinstellungen anbieten | M; W/A | V2 | Schnellpfad hat nachvollziehbare Defaults; angepasste Option verändert nachweisbar Ausgabe. | O |
| PMA-EXP-003 | Exportvorlagen | Benannte Optionen speichern, laden, ändern und löschen | M; W/A | V3 | Geladene Vorlage reproduziert Einstellungen; andere Konten erhalten keinen unbeabsichtigten Zugriff. | O |
| PMA-EXP-004 | Dateiziel/Kompression | Download, Dateiname, Zeichensatz und verfügbare Kompression wählen | M; W/A | V3 | Dateiname/Kodierung stimmen; entpackte Ausgabe entspricht unkomprimierter Fixture. | O |
| PMA-EXP-005 | Serverseitiges Exportverzeichnis | In administrativ freigegebenes Ziel speichern | M; W/A? | V4 | Export landet ausschließlich im erlaubten Ziel; Pfadüberschreitung scheitert. | O |
| PMA-EXP-006 | SQL-Export | Struktur, Daten oder beides als wiederherstellbaren Dump liefern | M; W/A | V2 | Import in leere Datenbank rekonstruiert gewählten Struktur-/Datenumfang. | O |
| PMA-EXP-007 | CSV-Export | Spalten, Trenner, Einfassung, Escape und NULL-Darstellung einstellen | M; W/A | V2 | Parser liest Sonderzeichen-Fixture mit gewählten Optionen ohne Wertverlust zurück. | O |
| PMA-EXP-008 | Excel-CSV | Für Tabellenkalkulation vorkonfigurierte CSV anbieten | M; W/A | V3 | Referenzprofil erzeugt erwarteten Trenner, Header und Kodierung. | O |
| PMA-EXP-009 | JSON-Export | Daten und referenzbezogene Metadaten ausgeben | M; W/A | V3 | JSON-Parser liest Ausgabe; DB-/Tabellenzuordnung und Unicode stimmen. | O |
| PMA-EXP-010 | XML-Export | Struktur-/Datenoptionen als Referenz-XML liefern | M; W/A | V4 | Ausgabe ist wohlgeformt; Referenzimport stellt gewählten Umfang wieder her. | O |
| PMA-EXP-011 | ODS-Export | Tabelle als OpenDocument Spreadsheet exportieren | M; W/A | V4 | Unabhängiger ODS-Leser bestätigt Blätter, Header und Werte. | O |
| PMA-EXP-012 | ODT-Export | Tabelle als OpenDocument Text exportieren | M; W/A | V4 | Unabhängiger ODT-Leser findet erwartete Tabellenspalten und Unicodewerte. | O |
| PMA-EXP-013 | Word-Export | Referenzformat für Word-2000-kompatible Tabellen liefern | M; W/A | V4 | Kompatibler Leser öffnet Tabelle mit korrekten Headern und Inhalten. | O |
| PMA-EXP-014 | PDF-Export | Daten als lesbare PDF-Tabelle ausgeben | M; W/A | V4 | PDF öffnet; mehrseitige Fixture enthält alle Zeilen und lesbare Überschriften. | O |
| PMA-EXP-015 | LaTeX-Export | Struktur/Daten mit notwendigen Escape-Regeln ausgeben | M; W/A | V4 | Fixture mit TeX-Sonderzeichen lässt sich im dokumentierten Wrapper kompilieren. | O |
| PMA-EXP-016 | MediaWiki-Export | Struktur/Daten als Wiki-Tabelle liefern | M; W/A | V4 | Parser/Referenzimport erhält erwartete Spalten und Werte. | O |
| PMA-EXP-017 | PHP-Array-Export | Daten als PHP-Array repräsentieren | M; W/A | V4 | Isolierter Leser ohne Fremdcodeausführung bestätigt Schlüssel und Fixturewerte. | O |
| PMA-EXP-018 | YAML-Export | Daten als gültiges YAML ausgeben | M; W/A | V4 | Unabhängiger sicherer YAML-Parser liest Typ-/Unicode-Fixture erwartungsgemäß. | O |
| PMA-EXP-019 | Texy!-Export | Tabelle im Referenz-Markup liefern | M; W/A | V4 | Referenz-kompatibler Parser rekonstruiert erwartete Tabellenzellen. | O |
| PMA-EXP-020 | CodeGen-Export | Tatsächlich vorhandene NHibernate-Variante ausgeben | M; W/A | V4 | Ausgabe entspricht Referenzfixture; bloß geplante Generatorvarianten gelten nicht als vorhanden. | O |
## SQL-Dumpoptionen

Quelle: [SQL-Exportplugin](https://github.com/phpmyadmin/phpmyadmin/blob/962857e4f63d42e38f11ff4d63f5e722018add76/libraries/classes/Plugins/Export/ExportSql.php). Gemeinsamer Stand aller Zeilen: `O`.

| ID | Referenzfunktion | Sollverhalten | DB; Plattform | Prio-Vorschlag | Abnahmekriterium | Stand |
| --- | --- | --- | --- | --- | --- | --- |
| PMA-DUMP-001 | Objektdefinitionen | CREATE/DROP, IF NOT EXISTS, Views, Routinen, Events und Trigger wählbar einbeziehen | M; W/A | V3 | Jede Objektklasse wird einzeln ein-/ausgeschaltet und nach Wiederherstellung unabhängig geprüft. | O |
| PMA-DUMP-002 | INSERT-Syntax | Vollständige/erweiterte INSERTs und maximale Anweisungslänge wählen | M; W/A | V3 | Varianten erzeugen gleiche Werte; konfigurierte Längengrenze wird soweit für Einzelzeilen möglich eingehalten. | O |
| PMA-DUMP-003 | Datenanweisung | INSERT, UPDATE, REPLACE sowie IGNORE/DELAYED nach Serverfähigkeit wählen | M; W/A | V4 | Fixture mit vorhandenem Schlüssel zeigt die erwartete Konfliktwirkung jeder unterstützten Variante. | O |
| PMA-DUMP-004 | Binär-/Zeitwerte | Binärdaten hexadezimal und TIMESTAMP in UTC exportieren | M; W/A | V3 | Wiederherstellung bei anderer Zeitzone erhält Zeitpunkt und Binärprüfsumme. | O |
| PMA-DUMP-005 | Dump-Transaktion/FK | Transaktionsrahmen und temporäre FK-Prüfungsoptionen ausgeben | M; W/A | V3 | Dump stellt relationale Fixture wieder her; keine Zusage eines serverweit konsistenten Snapshots ohne Nachweis. | O |
| PMA-DUMP-006 | DEFINER/Kompatibilität | DEFINER entfernen und vorhandene Kompatibilitätsoptionen bewusst anwenden | M; W/A | V4 | Dump ohne fremden DEFINER wird mit berechtigtem Zielkonto geladen; unpassende Kompatibilität wird nicht behauptet. | O |
| PMA-DUMP-007 | Kommentare/Metadaten/Aliase | Wählbare Kommentare, Metadaten und Namensersetzung exportieren | M; W/A | V4 | Nur gewählte Metadaten erscheinen; umbenannte Referenzobjekte sind konsistent oder Grenzen klar ausgewiesen. | O |
| PMA-DUMP-008 | Views als Tabellen | Viewdaten wahlweise als Tabellendump exportieren | M; W/A | V4 | Ziel enthält Tabelle mit Fixturewerten und keine versehentlich verbleibende View. | O |
## Beziehungen, Designer und visuelle Abfragen

Quelle: REL, [Designer](https://github.com/phpmyadmin/phpmyadmin/blob/962857e4f63d42e38f11ff4d63f5e722018add76/libraries/classes/Controllers/Database/DesignerController.php). Gemeinsamer Stand aller Zeilen: `O`.

| ID | Referenzfunktion | Sollverhalten | DB; Plattform | Prio-Vorschlag | Abnahmekriterium | Stand |
| --- | --- | --- | --- | --- | --- | --- |
| PMA-REL-001 | Native Fremdschlüssel | Referenzen mit ON UPDATE/DELETE verwalten | M; W/A | V2 | Server verweigert verwaiste Zeile; CASCADE-/RESTRICT-Fixtures wirken wie eingestellt. | O |
| PMA-REL-002 | Interne Beziehungen | App-interne Beziehungen von DB-erzwungenen Constraints unterscheiden | M; W/A | V4 | Interne Relation ermöglicht Navigation; externe DB-Sitzung wird durch sie nicht fälschlich eingeschränkt. | O |
| PMA-REL-003 | Fremdschlüsselwerte | Referenzdatensatz auswählen, beschreibenden Wert zeigen und navigieren | M; W/A | V2 | Auswahl zeigt Label und speichert Schlüssel; Navigation öffnet richtige referenzierte Zeile. | O |
| PMA-REL-004 | Integritätsprüfung | Verwaiste Werte interner/MyISAM-Beziehungen finden | M; W/A | V4 | Fixture mit genau einem verwaisten Wert meldet genau diesen Datensatz. | O |
| PMA-REL-005 | Grafischer Designer | Tabellen/Beziehungen ansehen und per gleichwertiger Geste bearbeiten | M; W/A | V3 | Zwei Tabellen und Relation erscheinen; grafisch erzeugte native Relation ist in DB nachlesbar. | O |
| PMA-REL-006 | Designerlayout | Positionen, Seiten und Anzeigeeinstellungen speichern | M; W/A | V4 | Layout bleibt nach Neustart erhalten und kann einem Export zugeordnet werden. | O |
| PMA-REL-007 | Schemaexport PDF | Layout mit Tabellen/Beziehungen als PDF ausgeben | M; W/A | V4 | Datei öffnet und enthält alle ausgewählten Tabellen und Beziehungslinien. | O |
| PMA-REL-008 | Schemaexport SVG/EPS/Dia | Weitere vorhandene Schemaformate ausgeben | M; W/A | V4 | Jedes Format öffnet im passenden unabhängigen Leser mit korrekter Objektzahl. | O |
| PMA-REL-009 | Datenwörterbuch | Spalten, Typen, Kommentare und Beziehungen dokumentieren | M; W/A | V4 | Wörterbuch entspricht Fixturemetadaten und gewähltem Objektumfang. | O |
| PMA-REL-010 | Query-by-example | Spalten, Verknüpfungen, Filter und Sortierung visuell in SQL überführen | M; W/A | V3 | Zwei verbundene Fixturetabellen liefern erwartete Treffer; erzeugtes SQL ist sichtbar und reproduzierbar. | O |
| PMA-REL-011 | Visuelle Abfrage speichern | QBE-Vorlagen speichern und wieder öffnen | M; W/A | V4 | Wiedergeöffnete Vorlage bewahrt Join, Filter und ausgewählte Spalten. | O |
## Transformationen und Spezialeditoren

Quelle: TRANS, [Transformationsplugins](https://github.com/phpmyadmin/phpmyadmin/tree/962857e4f63d42e38f11ff4d63f5e722018add76/libraries/classes/Plugins/Transformations). Gemeinsamer Stand aller Zeilen: `O`.

| ID | Referenzfunktion | Sollverhalten | DB; Plattform | Prio-Vorschlag | Abnahmekriterium | Stand |
| --- | --- | --- | --- | --- | --- | --- |
| PMA-TRANS-001 | Transformationszuordnung | MIME-Typ, Eingabe-/Ausgabetransformation und Optionen je Spalte speichern | M; W/A | V4 | Zwei Spalten behalten unabhängige Einstellungen; Ausgabetransformation verändert DB-Werte nicht. | O |
| PMA-TRANS-002 | Bilder und Links | JPEG/PNG inline, Bild-/Textlink und Download darstellen | M; W/A | V4 | Kleine Referenzdateien werden korrekt angezeigt/heruntergeladen; gefährliche Linkinhalte führen nicht zu Codeausführung. | O |
| PMA-TRANS-003 | Binäransicht | Hexdarstellung und Download bereitstellen | M; W/A | V3 | Hexansicht entspricht bekannten Bytes; Download besitzt identische Prüfsumme. | O |
| PMA-TRANS-004 | Textdarstellung | Bool-Label, Datumsformat, formatierten Text, Teilstring und Prä-/Suffix anwenden | M; W/A | V4 | Für jede benannte Variante ergibt Optionen-Fixture erwartete Anzeige bei unverändertem Rohwert. | O |
| PMA-TRANS-005 | IP-Transformationen | IPv4-Zahl und binäre IP in Darstellung/Eingabe umwandeln | M; W/A | V4 | IPv4-Randwerte und unterstützte binäre IP-Fixture bestehen Hin-/Rückumwandlung. | O |
| PMA-TRANS-006 | SQL/JSON/XML-Darstellung | Strukturierte Inhalte passend darstellen | M; W/A | V3 | Verschachtelte gültige Inhalte bleiben vollständig; ungültige Daten erzeugen keinen stillen Wertverlust. | O |
| PMA-TRANS-007 | SQL/JSON/XML-Eingabeeditor | Spezialeditor vor Speicherung anbieten | M; W/A | V3 | Bearbeitete Fixture kommt unverändert an; Syntax-/Validierungsfehler sind sichtbar. | O |
| PMA-TRANS-008 | Datei-/Bildeingabe | Textdatei und Bild als Eingabetransformation übernehmen | M; W/A | V4 | Upload-Fixture wird entsprechend gewählter Transformation gespeichert und korrekt wieder gelesen. | O |
| PMA-TRANS-009 | Regex-Validierung | Eingabe nach konfiguriertem Muster prüfen | M; W/A | V4 | Gültige Eingabe wird gespeichert, ungültige bleibt ohne unbeabsichtigte Mutation abgewiesen. | O |
| PMA-TRANS-010 | Externe Transformation | Referenzfähigkeit externer Programme gesondert bewerten | M; W/A? | V4 | Vor Umsetzung Bedrohungs-/Plattformentscheidung dokumentieren; isolierte freigegebene Fixture zeigt definierten Ersatz oder begründete Abweichung. | O |
| PMA-TRANS-011 | Erweiterbarkeit | Transformationen und verfügbare Optionen auffindbar machen | M; W/A? | V4 | Inventar zeigt jede unterstützte Transformation samt Optionen; Fremd-PHP-Plugin-Kompatibilität wird nicht behauptet. | O |
## Tracking, Zentralspalten und Normalisierung

Quelle: [Tracking](https://github.com/phpmyadmin/phpmyadmin/blob/962857e4f63d42e38f11ff4d63f5e722018add76/libraries/classes/Tracking.php), [Normalisierung](https://github.com/phpmyadmin/phpmyadmin/blob/962857e4f63d42e38f11ff4d63f5e722018add76/libraries/classes/Normalization.php). Gemeinsamer Stand aller Zeilen: `O`.

| ID | Referenzfunktion | Sollverhalten | DB; Plattform | Prio-Vorschlag | Abnahmekriterium | Stand |
| --- | --- | --- | --- | --- | --- | --- |
| PMA-META-001 | Änderungsverfolgung aktivieren | Versionierten Struktur-/Datenstand für gewählte Objekte beginnen/beenden | M; W/A | V4 | Aktiviertes Fixtureobjekt erhält Startversion; deaktiviertes erzeugt keine weiteren App-Trackingeinträge. | O |
| PMA-META-002 | Änderungsbericht | Erfasste DDL/DML nach Version, Zeitpunkt und Konto filtern | M; W/A | V4 | Bekannte App-Änderungen erscheinen korrekt; externe DB-Änderung wird nicht als lückenlos erfasst behauptet. | O |
| PMA-META-003 | Trackingexport | Gewählten Bericht oder erfasste SQL-Änderungen exportieren | M; W/A | V4 | Export enthält genau gewählte Fixtureversionen und nachvollziehbare Reihenfolge. | O |
| PMA-META-004 | Zentralspalten verwalten | Wiederverwendbare Spaltendefinitionen sammeln und entfernen | M; W/A | V4 | Definition kann aus Tabelle übernommen und in neuer Tabelle passend genutzt werden. | O |
| PMA-META-005 | Zentralspalten abgleichen | Abweichende Definitionen erkennen und explizit angleichen | M; W/A | V4 | Zwei abweichende Spalten werden erkannt; bestätigte Auswahl wird passend geändert. | O |
| PMA-META-006 | Normalisierung 1NF | Assistent für Schlüssel, redundante Felder und Wiederholungsgruppen anbieten | M; W/A | V4 | Synthetisches Beispiel wird nach expliziten Nutzerangaben umgebaut; Werte bleiben rekonstruierbar. | O |
| PMA-META-007 | Normalisierung 2NF | Partielle Abhängigkeiten mit Nutzereingaben zerlegen | M; W/A | V4 | Fixture mit zusammengesetztem Schlüssel wird in erwartete Tabellen zerlegt; Join rekonstruiert Ausgangsdaten. | O |
| PMA-META-008 | Normalisierung 3NF | Transitive Abhängigkeiten mit Nutzereingaben zerlegen | M; W/A | V4 | Fixturezerlegung ergibt erwartete Tabellen; Join rekonstruiert Ausgangsdaten ohne verlorene Zeile. | O |
## Diagramme, Einstellungen und Authentifizierung

Quelle: CHART, CONFIG, AUTH, INTRO. Gemeinsamer Stand aller Zeilen: `O`.

| ID | Referenzfunktion | Sollverhalten | DB; Plattform | Prio-Vorschlag | Abnahmekriterium | Stand |
| --- | --- | --- | --- | --- | --- | --- |
| PMA-UX-001 | Ergebnisdiagramme | Balken, Säulen, Linien, Spline, Fläche, Kreis und Timeline nach Dateneignung anbieten | M; W/A | V4 | Jeder geeignete Diagrammtyp stellt bekannte Fixturewerte korrekt dar. | O |
| PMA-UX-002 | Diagrammoptionen | Achsen, mehrere Serien, Titel, Beschriftung und Zeilenbereich wählen | M; W/A | V4 | Optionen ändern nur erwartete Darstellung; gewählter Zeilenbereich stimmt. | O |
| PMA-UX-003 | Persönliche Einstellungen | Navigation, Tabellenansicht, SQL/Konsole und Defaults pro Konto verwalten | M; W/A | V2 | Zwei Konten behalten unterschiedliche Optionen nach Neustart. | O |
| PMA-UX-004 | Einstellungen übertragen | Optionen exportieren, importieren und zurücksetzen | M; W/A | V3 | Export/Import reproduziert Optionen; Reset stellt dokumentierte Defaults her. | O |
| PMA-UX-005 | Themen und Layout | Anzeigeeinstellungen und verfügbare Themen wählbar machen | M; W/A? | V4 | Verfügbare Themen bleiben bedienbar; Auswahl bleibt gespeichert; native Gleichwertigkeit ist ausdrücklich bewertet. | O |
| PMA-UX-006 | Sprache und Richtung | Deutsche Oberfläche sowie zusätzliche Referenzsprachen/RTL abdecken | M; W/A? | V4 | Deutscher Kernpfad ist vollständig; Sprachinventar und RTL-Layout werden gegen festgelegte Referenzliste geprüft. | O |
| PMA-UX-007 | Tastatur und Touch | Wichtige Aktionen ohne Maus und auf iPad per Touch erreichbar machen | M; W/A | V2 | Verbindung, SQL-Ausführung, Tabellenwechsel und Bearbeitung bestehen dokumentierten Tastatur-/Touchablauf. | O |
| PMA-UX-008 | Optionale TOTP-Anmeldung | Zweiten Faktor einrichten, verwenden und entfernen | M; W/A? | V4 | Ungültiger Code scheitert; gültiger Code eröffnet nur autorisierte Sitzung; Wiederherstellung ist separat geprüft. | O |
| PMA-UX-009 | Hardwarefaktor und Recovery | Referenz-U2F-Fähigkeit funktional bewerten und Wiederherstellung abdecken | M; W/A? | V4 | Dokumentierte Plattformentscheidung und Gerätefall belegen zweiten Faktor/Recovery; Passkey-Login allein gilt nicht als Tresorentsperrung. | O |
| PMA-UX-010 | Auth-Integrationen | Konfigurations-, Cookie-, HTTP- und Signon-Wege als Referenzintegrationen bewerten | M; W/A? | V4 | Für jeden Referenzweg wird funktionaler Produktpfad oder begründete Abweichung akzeptiert und isoliert getestet. | O |
| PMA-UX-011 | Konfigurationsspeicher | Persistenzabhängige Funktionen und fehlende Speichermöglichkeiten sichtbar behandeln | M; W/A | V3 | Ohne verfügbaren Speicher entsteht kein falscher Speicherungserfolg; aktivierter Speicher hält Kontoisolation ein. | O |
| PMA-UX-012 | UI-Benutzergruppen | Referenz-Menüsichtbarkeit von tatsächlichen DB-Rechten trennen | M; W/A? | V4 | Ausgeblendete Aktion ist keine Sicherheitsgarantie; direkte Operation bleibt DB-/API-Rechten unterworfen. | O |

## Ergänzende Produktanforderungen außerhalb der phpMyAdmin-Parität

Diese Punkte sind verbindliche Nutzeranforderungen beziehungsweise ausdrücklich markierte
Planungsfragen aus der Projektübergabe, keine nachgewiesenen phpMyAdmin-Funktionen.
Sie dürfen bei der Paritätsabnahme nicht untergehen und benötigen eigene spezifizierte Testfälle:

| ID | Ziel | Plattform | Abnahmerichtung | Tatsächlicher Stand |
| --- | --- | --- | --- | --- |
| PROD-001 | Docker Compose und Swarm als Betriebsziele | Go/Web | Beide Deploymentpfade starten aus sauberem Checkout; Konfiguration, persistente Daten, Healthcheck, Update und Wiederanlauf praktisch prüfen | Teilnachweis F2b/F3b: Compose und isolierter echter DinD-Swarm mit Auth/Health/Originalassets/SQLitepersistenz nach Neustart und Taskersatz unabhängig geprüft. Kein Multinode-/HA- oder umfassender Upgrade-/Migrationsnachweis |
| PROD-002 | Vollständiger persönlicher Betrieb ohne Go | macOS/iPadOS | Direkte DB-Verbindung, Metadaten, SQL und weitere zugesagte lokale Matrixfunktionen bei unerreichbarem Go prüfen | A1 SQLite/A2 PostgreSQL direkt ohne Go integriert, Mac-/Simulatorbuilds und synthetische DB-Fälle geprüft. Weitere lokale Matrixfunktionen, echte iPads, Signing/OS-UI offen |
| PROD-003 | Mehrere zusätzliche DB-Systeme | Go/Apple | DB-Ziele zuerst durch ADR festlegen; native Fähigkeiten und Abweichungen pro System separat abnehmen | B1 SQLite/MySQL/MariaDB/PostgreSQL echt geprüft; Apple SQLite/PostgreSQL. Konkrete Fixtureversionen in Handoffs; umfassende unterstützte Versionsmatrix und native MySQL/MariaDB offen |
| PROD-004 | Optionale verschlüsselte Zugangsdaten, Masterpasswort, Passkey-Entsperren | Alle | Bedrohungsmodell, Verlust-/Recoveryfälle und tatsächlich verfügbare Entsperrwege vor Geheimnisspeicherung prüfen | P3a Bedrohungsmodell/Compile-Spikes und S1-Formatinterop geprüft; S2a-Core aktiv. Kein produktiver Tresor-/Recovery-/Passkey-/Gerätenachweis |
| PROD-005 | Persönlicher iCloud-Sync mit Offlinebetrieb | Apple | Geräteaufnahme, Offlineänderung, Konflikte und verschlüsselte Synchronisierung auf echten Geräten prüfen | Umfang teilweise Vorschlag; keine Implementierung |
| PROD-006 | Optionale Backend-Teams und explizite Freigaben | Go/Web/Apple mit Backend | Rollen, Kontentrennung, bewusste Freigabe und Entzug/Rotation separat spezifizieren und prüfen | Teamziel verbindlich; Detailmodell offen |

## Vollständigkeitsgrenzen und nächster Nachweis

Diese Erhebung deckt die offiziellen Überblicks-/Handbuchkapitel und erkennbaren
fachlichen Einstiegspunkte sowie Import-, Export-, Schema- und Transformationspluginfamilien
ab. Sie ist **kein Beweis**, dass jedes Unterformular, jede Option und jede Kombination
vollständig erhoben ist. Insbesondere offen:

1. Kein interaktiver Vergleich mit laufendem phpMyAdmin 5.2.3. SQL-Delimiter,
   Zeileneditor, Fehler-/Warnungsverhalten, Fortschritt und Fortsetzung benötigen
   Referenzfixtures und tatsächliche Vergleichsläufe.
2. Keine vollständige Zuordnung aller Konfigurationsdirektiven, Tastenkürzel,
   Sprachkataloge, Authentifizierungsplugins und Export-Unteroptionen. Jede Optionsfamilie
   wird vor Implementierung weiter in parametrisierte Fälle zerlegt. Eine Sammelzeile
   ist erst bestanden, wenn alle dort genannten Varianten belegt sind.
3. Versions-/Engine-/Rechteunterschiede MySQL/MariaDB sind hier als Bedingung benannt,
   nicht praktisch vermessen. Keine Zusage für jede historische von phpMyAdmin
   unterstützte Serverversion; konkrete Produktversionen sind noch zu entscheiden.
4. Native Ersatzabläufe für Web-Dateiverzeichnisse, externe Programme, U2F/HTTP/Signon,
   Themen und Konfigurationsintegration sind mit `W/A?` sichtbar offen. Keine pauschale
   Gleichsetzung von WebAuthn/Passkey und älteren U2F-Verfahren. Eine Abweichung darf
   nur mit dokumentierter Entscheidung und funktionaler Bewertung ausgenommen werden.
5. Interne phpMyAdmin-Installationstechnik (PHP/mysqli, phpinfo, Paketverwaltung,
   Git-Revisionsseite, Fehlerbericht an das Fremdprojekt) ist nicht automatisch ein
   Produktfeature. Welche administrativen Diagnose-/Updatefunktionen funktional
   übernommen werden, ist noch ausdrücklich zu entscheiden. Lizenz-/Update- und
   Sicherheitsinformationen des eigenen Produkts gehören zur Releaseprüfung.
6. Keine Bytegleichheit mit allen Fremdexporten zugesagt: Austauschkompatibilität und
   verlustarme Datenwerte sind getrennt zu prüfen. XML-Schema ist im Handbuch selbst
   unvollständig dokumentiert; dafür müssen Referenzdateien erzeugt werden.
7. Zusätzliche Produktideen (konkurrierende Bearbeitung, Schema-Diff, Tab-Wiederherstellung,
   SSH, Streaming) sind nicht pauschal als phpMyAdmin-Funktion behauptet. Sie bleiben
   separat nach Projektplan und Treiberbefunden zu spezifizieren.

Inventar P1a ist nach unabhängiger Prüfung integriert. Referenzvergleich mit laufendem
phpMyAdmin, umfassende DB-/OS-Versionsentscheidung und plattformübergreifende
Abnahmedatensätze bleiben offen. Aktuelle begrenzte Pakete stehen in ARBEITSPLAN.
Nach späterem Funktionszuwachs von phpMyAdmin muss der Referenzumfang bewusst neu bewertet
werden; `latest` darf die historische Abnahmegrundlage nicht stillschweigend verändern.

## Teilnachweise des implementierten Zwischenstands

Diese Zuordnung ändert keine offene Gesamtzeile in bestanden. Sie verbindet konkrete
Produktinkremente mit dem Soll und benennt wesentliche verbleibende Lücken. Details,
exakte Fixtureversionen, Befehle/Commits/CIs: verlinkte Handoffs und [STATUS](STATUS.md).

| Soll-IDs | Geprüfter Teilumfang | Weiterhin offene gemeinsame Abnahme |
| --- | --- | --- |
| PMA-CON-001/002/003/005/007 | B1/F2b/F3b: flüchtige Verbindung, Katalog, echter Webpfad, Sessionwechsel und verifiziertes TLS. A1/A2: SQLite/PG direkt ohne Go. | Gleichzeitige gespeicherte Profile, vollständiger Objektbaum/Clientzertifikate/Socketvarianten, alle MySQL-Apple-/Gerätefälle; kein Teamlogin/Tokenwiderruf |
| PMA-DATA-001/002/003 | B2a/F3: Pagination mit konservativem Stabilitätsmerkmal, Mehrspaltensortierung, exakte typisierte NULL-/Leer-/Binärwerte, gebundene Filter | Sichtbare Spaltenwahl, vollständiger Plattform-/MySQLvergleich und alle nativen Tabellenabläufe; kein Snapshot über Seiten |
| PMA-DATA-004/005/008 | B2b/B2c samt beiden Integritätsfixes: einzelne bestätigte Zeilenmutationen gegen vier Backendtreiber, Default/NULL/PK/Trigger/Conflict/Unknown geprüft | F4-Webeditor noch unintegriert; serverseitige Formularfunktionen, direkte Zelländerung, Mehrfachauswahl und native strukturierte Editoren fehlen |
| PMA-DATA-010 | B2a/F3 gebundene Gleichheit/is-null/is-not-null-AND-Filter und Sortierung | Bereich/LIKE und vollständige native Suchabläufe fehlen |
| PMA-SQL-001 | B1/F2b und A1/A2: einzelne SQL-Anweisung im gewählten DBkontext, begrenzte präzise Ergebnisse | Vollständiger Referenz-/Versions-/Gerätenachweis über alle zugesagten DBsysteme offen |
| PMA-SQL-004 | Sichere verständliche Fehler, begrenzte Ergebnisse und ehrliche Abbruch-/Commitunsicherheit | Detaillierte Warnungsanzeige/Servercodes und vollständiger Referenzvergleich fehlen |

Backendnachweise: [B1](handoffs/B1.md), [B2a](handoffs/B2a.md),
[B2b](handoffs/B2b.md), [SQLitefix](handoffs/B2b-konfliktstrategie.md),
[PGfix](handoffs/B2b-rewrite-regeln.md), [B2c](handoffs/B2c.md).
Web-/Betriebsnachweise: [F2b](handoffs/F2b.md), [F3b](handoffs/F3b.md),
[unabhängiger F3b-Review](handoffs/F3b-backend-review.md).
Native Nachweise im öffentlichen Apple-Repo: docs/handoffs/A1.md und A2.md.
Kein fertiges Paritätsprozent aus Teilnachweisen ableiten.
