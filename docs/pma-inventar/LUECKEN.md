# Lücken der bestehenden Paritätsmatrix

Status: **Befund, kein Beschluss.** Die Matrix in [PARITAET.md](../PARITAET.md) wurde nicht geändert.
Die Punkte in Abschnitt A sind Vorschläge für den Lead. Grundlage: [README](README.md) und die Bereichsdateien 01 bis 18.

## A. Befunde zur Matrix selbst

| Nr | Befund | Beleg | Vorschlag |
| --- | --- | --- | --- |
| A1 | Die Teilnachweis-Tabelle am Dateiende nutzt zusammengefasste IDs (`PMA-CON-001/002/003/005/007`, `PMA-DATA-001/002/003`, `PMA-DATA-004/005/008`) und führt `PMA-DATA-010`, `PMA-SQL-001` und `PMA-SQL-004` doppelt. In der Hauptmatrix ist jede ID einmal (181 eindeutige IDs). | PARITAET.md, Abschnitt Teilnachweise (Zeilen 425 bis 430) | Teilnachweise je ID auflösen; Sammelzeilen nicht als Nachweis einer Einzel-ID verwenden. |
| A2 | `PMA-SQL-013` (DML-Simulation) steht im Abschnitt SQL, die Quelle liegt im Importbereich. | Bereich 08, Quelle `Controllers/Import/SimulateDmlController.php` | Zuordnung prüfen: Import oder SQL. |
| A3 | `PMA-UX-010` bündelt Anmeldeverfahren (HTTP, Signon, Cookie, Konfiguration). Die Konfigurations-Anmeldung (`PMA-NEU-SEC-001`) fehlt als eigene Zeile. | Bereich 02 | Eigene Zeile oder Entscheidung dokumentieren. |
| A4 | Sicherheitsfunktionen ohne eigene Zeile: CSRF-Token-Prüfung, IP-Regeln, Anmeldeprotokoll, Referer-Schutz, Ausgabe-Sanitisierung, Sitzungsverwaltung, Geheimnis für Cookie-Verschlüsselung. | Bereich 02 (`PMA-NEU-SEC-002` bis `-008`) | Als Sicherheitsanforderungen aufnehmen (eigene Zeilen oder PROD-Ergänzung). |
| A5 | Engine-Detailseiten und Mehrquellen-Replikation fehlen in den Zeilen `PMA-ADMIN-008` und `PMA-ADMIN-011`. | Bereich 04 (`PMA-NEU-ADMIN-001`, `-007`) | Ergänzen oder den Zeilen zuordnen. |
| A6 | Serverstatus-Details fehlen: Abfragestatistik, Log-Ansichten, Abfrageanalyse im Monitor, Systemressourcen des PHP-Hosts, FLUSH-Aktionen. | Bereich 03 | Ergänzen; Systemressourcen als Entscheidung (Host statt Datenbankserver). |
| A7 | Matrix-Vollständigkeitsgrenze 5 nennt phpinfo, Git-Revision, Versionsprüfung und Fehlerbericht als offen. Alle vier Funktionen existieren im Quellcode; Die Versionsprüfung ruft Daten von einem externen Dienst ab; der Fehlerbericht sendet Daten an einen externen Dienst. | Bereich 13 (`PMA-NEU-MAINT-001` bis `-004`) | Je Funktion entscheiden; externe Verbindungen nur mit Datenschutzprüfung. |
| A8 | Setup und Einrichtung (Assistent, Konfigurationsdatei, Serverprofile, Konfigurationsschreiben im Browser) hat keine Matrixzeile. Die Matrix nennt nur die Grenze 2. | Bereich 18 | Entscheidung: Parität, Ersatz durch Produktkonfiguration oder Ausschluss (ADR). |
| A9 | Die Mehrtabellenabfrage (Multi-Table-Query) fehlt; die Matrix kennt nur Beispiel-Abfragen (`PMA-REL-010`). | Bereich 14 (`PMA-NEU-REL-003`) | Ergänzen. |
| A10 | Zwei Rechteseiten (Datenbank- und Tabellenebene) sind im Dienstregister eingetragen, aber nicht geroutet. | Bereich 05 (`PMA-NEU-PRIV-001`, `-002`) | Prüfen, ob Funktion erreichbar ist; sonst nicht übernehmen. |
| A11 | Quellbefund: Der Docblock von `Controllers/NormalizationController.php` nennt die Normalisierung "temporarily specific to 1NF". Der Code enthält 2NF- und 3NF-Schritte. Die Matrixzeilen `PMA-META-007` und `-008` stimmen. | Bereich 16 | Keine Matrixänderung; im Abnahmefall den Code, nicht den Docblock, als Referenz nehmen. |
| A12 | Das Zwei-Faktor-Plugin "Simple" ist laut Quellbeschreibung nur für Tests gedacht ("For testing purposes only!"). | Bereich 02 (`PMA-NEU-SEC-010`) | Nicht als Produktfunktion führen. |
| A13 | Die Serververzeichnis-Funktionen (Export `PMA-EXP-005`, Import `PMA-IMP-003`) sind im Quellcode durch die Schalter `SaveDir` und `UploadDir` belegt. | Bereiche 10 und 11, `libraries/config.default.php` | Keine Änderung; Matrixangaben sind korrekt markiert (W/A?). |
| A14 | Versions- und Rechteangaben fehlen in der Matrix; Beispiel: `ALTER USER ... ACCOUNT LOCK` (`PMA-PRIV-009`) ist im Quellcode belegt, die Serverversion nicht. | Bereich 05 | Versionsmatrix nach Matrix-Grenze 3 ergänzen. |

## B. Neue Funde (fehlen in der Matrix oder sind nur teilweise abgedeckt)

Alle neuen Funde mit Matrixbezug, Priorität-Vorschlag und Eignung. Sortiert nach ID.
Matrixbezug: "fehlt" = keine Matrixzeile; "teilweise" = Ergänzung zu der genannten Zeile.

| ID | Funktion | Matrixbezug | Priorität (Vorschlag) | Eignung Web/Go · Apple |
| --- | --- | --- | --- | --- |
| PMA-NEU-ADMIN-001 | Abfragestatistik des Servers | teilweise: PMA-ADMIN-002 | V3 | Web/Go ja · Apple ja |
| PMA-NEU-ADMIN-002 | Slow-Query- und General-Log-Ansicht | teilweise: PMA-ADMIN-006 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-ADMIN-003 | Abfrageanalyse im Monitor | teilweise: PMA-SQL-006 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-ADMIN-004 | Systemressourcen des PHP-Hosts | fehlt | V4 | Web/Go teilw. · Apple offen |
| PMA-NEU-ADMIN-005 | Flush-Aktionen im Status | teilweise: PMA-PRIV-010 | V3 | Web/Go ja · Apple ja |
| PMA-NEU-ADMIN-006 | Detailseite je Storage Engine | teilweise: PMA-ADMIN-008 | V4 | Web/Go ja · Apple ja |
| PMA-NEU-ADMIN-007 | Serverweiter Export | fehlt | V3 | Web/Go ja · Apple ja |
| PMA-NEU-ADMIN-008 | Serverweiter Import | fehlt | V3 | Web/Go ja · Apple ja |
| PMA-NEU-ADMIN-009 | Hilfelinks zur Serverdokumentation | fehlt | V4 | Web/Go ja · Apple ja |
| PMA-NEU-ADMIN-010 | Replikationsbenutzer auf dem Quellserver anlegen | teilweise: PMA-PRIV-002 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-ADMIN-011 | Replikationsfehler überspringen | teilweise: PMA-ADMIN-012 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-ADMIN-012 | Mehrquellen-Replikationsstatus | teilweise: PMA-ADMIN-011 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-DATA-001 | Auswahllisten für ENUM, SET und Fremdschlüssel beim Bearbeiten | teilweise: PMA-REL-003 | V2 | Web/Go ja · Apple ja |
| PMA-NEU-DATA-002 | Fremdschlüsselprüfung beim Bearbeiten umschalten | fehlt | V2 | Web/Go ja · Apple ja |
| PMA-NEU-DATA-003 | Spaltenpräferenzen je Tabelle speichern | teilweise: PMA-DATA-002 | V3 | Web/Go ja · Apple ja |
| PMA-NEU-DATA-004 | GIS-Visualisierung für Geodaten | teilweise: PMA-DATA-014 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-DATA-005 | GIS-Datenbearbeitungseditor | teilweise: PMA-DATA-014 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-DATA-006 | Fremdschlüssel-Auswahlfenster | teilweise: PMA-REL-003 | V2 | Web/Go ja · Apple ja |
| PMA-NEU-DDL-001 | Prüfung reservierter Wörter | fehlt | V3 | Web/Go ja · Apple ja |
| PMA-NEU-DDL-002 | Spaltenkommentare im Konfigurationsspeicher | fehlt | V3 | Web/Go ja · Apple offen |
| PMA-NEU-DDL-003 | Strukturübersicht einer Tabelle | teilweise: PMA-DDL-001 | V1 | Web/Go ja · Apple ja |
| PMA-NEU-DUMP-001 | Dump-Kompatibilitätsmodus je Zielsystem | teilweise: PMA-DUMP-006 | V4 | Web/Go ja · Apple ja |
| PMA-NEU-EXP-001 | Exportprüfung auf Zeitlimit | fehlt | V3 | Web/Go ja · Apple offen |
| PMA-NEU-EXP-002 | Exportumfang Tabelle aus Tabellenseite | teilweise: PMA-EXP-001 | V2 | Web/Go ja · Apple ja |
| PMA-NEU-EXP-003 | Exportvorlage für Serverexport | teilweise: PMA-EXP-003 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-IMP-001 | Datenbankimport über die Datenbankseite | teilweise: PMA-IMP-001 | V2 | Web/Go ja · Apple ja |
| PMA-NEU-IMP-002 | Import-Fortschritt als Statusdienst | teilweise: PMA-IMP-013 | V2 | Web/Go ja · Apple offen |
| PMA-NEU-IMP-003 | Upload-Fortschritt und Upload-Sitzung | teilweise: PMA-IMP-013 | V2 | Web/Go ja · Apple offen |
| PMA-NEU-MAINT-001 | phpinfo-Seite | fehlt (Entscheidung) | V4 | Web/Go nicht übernehmen · Apple entfällt |
| PMA-NEU-MAINT-002 | Git-Revisionsanzeige | fehlt (Entscheidung) | V4 | Web/Go ja · Apple entfällt |
| PMA-NEU-MAINT-003 | Versionsprüfung gegen die Projektwebsite | fehlt (Entscheidung) | V4 | Web/Go ja · Apple offen |
| PMA-NEU-MAINT-004 | Fehlerbericht an den Projektanbieter | fehlt (Entscheidung) | V4 | Web/Go nicht übernehmen · Apple entfällt |
| PMA-NEU-MAINT-005 | Lizenzanzeige | fehlt (Pflicht) | V1 | Web/Go ja · Apple ja |
| PMA-NEU-MAINT-006 | Änderungsprotokoll anzeigen | fehlt | V2 | Web/Go ja · Apple ja |
| PMA-NEU-MAINT-007 | Prüfung des Konfigurationsspeichers | teilweise: PMA-UX-011 | V3 | Web/Go ja · Apple offen |
| PMA-NEU-META-001 | Versionsansicht einer verfolgten Tabelle | teilweise: PMA-META-002 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-META-002 | Tracking-Umfang je Objektklasse | teilweise: PMA-META-001 | V4 | Web/Go ja · Apple ja |
| PMA-NEU-NAV-001 | Navigationselemente ausblenden | teilweise: PMA-CON-005 | V3 | Web/Go ja · Apple ja |
| PMA-NEU-NAV-002 | Einstellungen Navigationsbaum | teilweise: PMA-CON-005 | V4 | Web/Go ja · Apple ja |
| PMA-NEU-NAV-003 | SQL-Konsole am Seitenfuß | fehlt | V2 | Web/Go ja · Apple ja |
| PMA-NEU-NAV-004 | Konfigurierbare Menüs | fehlt | V4 | Web/Go ja · Apple ja |
| PMA-NEU-NAV-005 | Hauptpanel-Einstellungen | fehlt | V4 | Web/Go ja · Apple ja |
| PMA-NEU-NAV-006 | Menüleiste mit Ebenen-Tabs | fehlt | V4 | Web/Go ja · Apple ja |
| PMA-NEU-NAV-007 | Startseite der Oberfläche | teilweise: PMA-ADMIN-001 | V1 | Web/Go ja · Apple ja |
| PMA-NEU-OBJ-001 | Abhängigkeitsprüfung bei Tabellenoperationen | teilweise: PMA-OBJ-008 | V2 | Web/Go ja · Apple ja |
| PMA-NEU-PRIV-001 | Datenbankrechte-Seite (nicht geroutet) | teilweise: PMA-PRIV-007 | V3 | Web/Go ja · Apple ja |
| PMA-NEU-PRIV-002 | Tabellenrechte-Seite (nicht geroutet) | teilweise: PMA-PRIV-007 | V3 | Web/Go ja · Apple ja |
| PMA-NEU-PRIV-003 | Ermittlung der eigenen Rechte für die Oberfläche | fehlt | V2 | Web/Go ja · Apple ja |
| PMA-NEU-PROG-001 | Trigger aus der Tabellenansicht verwalten | teilweise: PMA-PROG-006 | V3 | Web/Go ja · Apple ja |
| PMA-NEU-PROG-002 | Linter für Routinen-, Trigger- und Event-Definitionen | teilweise: PMA-SQL-004 | V3 | Web/Go ja · Apple offen |
| PMA-NEU-PROG-003 | Event-Fehlerliste | fehlt | V4 | Web/Go ja · Apple offen |
| PMA-NEU-REL-001 | Beziehungsübersicht je Tabelle | teilweise: PMA-REL-001 | V2 | Web/Go ja · Apple ja |
| PMA-NEU-REL-002 | Beziehungsreparatur des Konfigurationsspeichers | fehlt | V3 | Web/Go ja · Apple offen |
| PMA-NEU-REL-003 | Mehrtabellenabfrage | teilweise: PMA-REL-010 | V3 | Web/Go ja · Apple ja |
| PMA-NEU-REL-004 | Tabellen- und Beziehungsstatistik im Schemaexport | teilweise: PMA-REL-007 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-SEC-001 | Anmeldung ohne Login-Formular (Config-Auth) | fehlt (Entscheidung) | V4 | Web/Go offen · Apple offen |
| PMA-NEU-SEC-002 | IP-Zugriffsbeschränkung (Allow/Deny) | fehlt | V3 | Web/Go ja · Apple entfällt |
| PMA-NEU-SEC-003 | Anmeldeprotokoll | fehlt | V3 | Web/Go ja · Apple offen |
| PMA-NEU-SEC-004 | Referer-Schutz über Weiterleitungsendpunkt | fehlt | V2 | Web/Go ja · Apple entfällt |
| PMA-NEU-SEC-005 | CSRF-Tokenprüfung bei POST | fehlt | V1 | Web/Go ja · Apple entfällt |
| PMA-NEU-SEC-006 | Geheimnis für Sitzungs- und Cookie-Verschlüsselung | fehlt | V1 | Web/Go ja · Apple offen |
| PMA-NEU-SEC-007 | Sitzungsverwaltung | fehlt | V1 | Web/Go ja · Apple offen |
| PMA-NEU-SEC-008 | Ausgabe-Sanitisierung | fehlt | V1 | Web/Go ja · Apple ja |
| PMA-NEU-SEC-009 | FIDO U2F als Hardware-Faktor (Key) | teilweise: PMA-UX-009 | V4 | Web/Go offen · Apple offen |
| PMA-NEU-SEC-010 | Zweiter Faktor "Simple" (nur Testzweck) | fehlt (Entscheidung) | V4 | Web/Go nein · Apple nein |
| PMA-NEU-SETUP-001 | Einrichtungsassistent | fehlt | V1 | Web/Go ja · Apple entfällt |
| PMA-NEU-SETUP-002 | Konfigurationsdatei erzeugen | fehlt | V1 | Web/Go ja · Apple entfällt |
| PMA-NEU-SETUP-003 | Serverprofile im Setup verwalten | teilweise: PMA-CON-001 | V1 | Web/Go ja · Apple ja |
| PMA-NEU-SETUP-004 | Konfigurationsprüfung des Servers | fehlt | V2 | Web/Go ja · Apple ja |
| PMA-NEU-SETUP-005 | Konfiguration im Browser lesen und ändern | fehlt (Entscheidung) | V3 | Web/Go ja · Apple offen |
| PMA-NEU-SETUP-006 | Funktionsschalter für Benutzeroberfläche | teilweise: PMA-UX-003 | V4 | Web/Go ja · Apple ja |
| PMA-NEU-SQL-001 | Tabellen- und Spaltenvorschläge im SQL-Editor | teilweise: PMA-SQL-005 | V2 | Web/Go ja · Apple ja |
| PMA-NEU-SQL-002 | SQL-Formatierung | teilweise: PMA-SQL-005 | V2 | Web/Go ja · Apple ja |
| PMA-NEU-SQL-003 | Abfrage-Linter | fehlt | V3 | Web/Go ja · Apple offen |
| PMA-NEU-SQL-004 | SQL-Editor-Einstellungen | teilweise: PMA-UX-003 | V3 | Web/Go ja · Apple ja |
| PMA-NEU-SQL-005 | Gespeicherte Suchen | teilweise: PMA-REL-011 | V3 | Web/Go ja · Apple offen |
| PMA-NEU-TRANS-001 | Transformationsvorschau im Ausgabefenster | teilweise: PMA-TRANS-001 | V4 | Web/Go ja · Apple ja |
| PMA-NEU-TRANS-002 | Gespeicherter Transformationsvorschlag für Dateiupload | teilweise: PMA-TRANS-008 | V4 | Web/Go ja · Apple ja |
| PMA-NEU-TRANS-003 | Transformation Bool2TextTransformationsPlugin | teilweise: PMA-TRANS-011 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-TRANS-004 | Transformation DateFormatTransformationsPlugin | teilweise: PMA-TRANS-011 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-TRANS-005 | Transformation DownloadTransformationsPlugin | teilweise: PMA-TRANS-011 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-TRANS-006 | Transformation FormattedTransformationsPlugin | teilweise: PMA-TRANS-011 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-TRANS-007 | Transformation InlineTransformationsPlugin | teilweise: PMA-TRANS-011 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-TRANS-008 | Transformation LongToIPv4TransformationsPlugin | teilweise: PMA-TRANS-011 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-TRANS-009 | Transformation PreApPendTransformationsPlugin | teilweise: PMA-TRANS-011 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-TRANS-010 | Transformation SubstringTransformationsPlugin | teilweise: PMA-TRANS-011 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-TRANS-011 | Transformation TextImageLinkTransformationsPlugin | teilweise: PMA-TRANS-011 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-TRANS-012 | Transformation TextLinkTransformationsPlugin | teilweise: PMA-TRANS-011 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-TRANS-013 | Transformation Image_JPEG_Upload | teilweise: PMA-TRANS-011 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-TRANS-014 | Transformation Text_Plain_Iptolong | teilweise: PMA-TRANS-011 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-TRANS-015 | Transformation Text_Plain_JsonEditor | teilweise: PMA-TRANS-011 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-TRANS-016 | Transformation Text_Plain_RegexValidation | teilweise: PMA-TRANS-011 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-TRANS-017 | Transformation Text_Plain_SqlEditor | teilweise: PMA-TRANS-011 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-TRANS-018 | Transformation Text_Plain_XmlEditor | teilweise: PMA-TRANS-011 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-TRANS-019 | Transformation Application_Octetstream_Download | teilweise: PMA-TRANS-011 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-TRANS-020 | Transformation Image_JPEG_Link | teilweise: PMA-TRANS-011 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-TRANS-021 | Transformation Image_PNG_Inline | teilweise: PMA-TRANS-011 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-TRANS-022 | Transformation Text_Octetstream_Sql | teilweise: PMA-TRANS-011 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-TRANS-023 | Transformation Text_Plain_Binarytoip | teilweise: PMA-TRANS-011 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-TRANS-024 | Transformation Text_Plain_Bool2Text | teilweise: PMA-TRANS-011 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-TRANS-025 | Transformation Text_Plain_External | teilweise: PMA-TRANS-011 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-TRANS-026 | Transformation Text_Plain_Formatted | teilweise: PMA-TRANS-011 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-TRANS-027 | Transformation Text_Plain_Imagelink | teilweise: PMA-TRANS-011 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-TRANS-028 | Transformation Text_Plain_Sql | teilweise: PMA-TRANS-011 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-TRANS-029 | Transformation Text_Plain_Xml | teilweise: PMA-TRANS-011 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-TRANS-030 | Transformation Text_Plain_Link | teilweise: PMA-TRANS-011 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-TRANS-031 | Transformation Text_Plain_Longtoipv4 | teilweise: PMA-TRANS-011 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-TRANS-032 | Transformation Text_Plain_PreApPend | teilweise: PMA-TRANS-011 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-TRANS-033 | Transformation Text_Plain_Substring | teilweise: PMA-TRANS-011 | V4 | Web/Go ja · Apple offen |
| PMA-NEU-UX-001 | Benutzerrechte-Gruppen zuweisen | teilweise: PMA-UX-012 | V3 | Web/Go ja · Apple offen |
| PMA-NEU-UX-002 | Schaltflächen-Konfiguration und Tabellenreiter | teilweise: PMA-UX-003 | V4 | Web/Go ja · Apple ja |
| PMA-NEU-UX-003 | Anzeige-Einstellungen für Daten | teilweise: PMA-UX-003 | V3 | Web/Go ja · Apple ja |

## C. Nicht eigenständig geführte Dateien

Diese Dateien sind keine eigenständigen Produktfunktionen oder wurden nicht namentlich genannt.

- **Basis- und Hilfsklassen** (`AbstractController` in den Controller-Verzeichnissen): keine eigene Funktion.
- **Entwicklungswerkzeuge** `libraries/classes/Command/` (CacheWarmupCommand, FixPoTwigCommand, SetVersionCommand, TwigLintCommand, WriteGitRevisionCommand): keine Produktfunktion.
- **Übersetzungsschnittstelle** `JavaScriptMessagesController` (`js/messages.php`): übergibt Texte an die Oberfläche; gehört zu `PMA-UX-006`.
- **Zweitfaktor-Fallback** `Plugins/TwoFactor/Invalid.php`: Reaktion auf unbekannte Zweitfaktor-Verfahren; keine Funktion.
- **Ausgabe-Hilfsklasse** `Plugins/Export/TableProperty.php`: Eigenschaftsbeschreibung für Exportoptionen; Zuordnung nicht im Detail geprüft (UNSICHER).

Dateien ohne namentliche Nennung im Inventar (automatische Prüfung, Plugin-Familien):
- Export: `TableProperty.php`
- Import: keine
- Schema: keine
- Auth: keine
- TwoFactor: `Invalid.php`
- Transformations: keine
