# Umsetzungsvorschlag (Entwurf, nicht beschlossen)

Status: **Vorschlag.** Keine Entscheidung, keine Zusage für Termine oder Umfang. Die Reihenfolge und die Pakete entscheidet der Lead.

**Gültigkeit:** Dieser Plan gilt erst nach Abschluss der Übergabe. [ARBEITSPLAN](../ARBEITSPLAN.md) sieht bis dahin keine weitere neue Featureintegration vor (Punkt 4). Vor Abschluss ist der Plan eine Vormerkung und kein Arbeitsauftrag.

Quellen des Stand-Abgleichs: `main`, [STATUS](../STATUS.md) und [ARBEITSPLAN](../ARBEITSPLAN.md) mit Stand 23.10.2026, Matrix [PARITAET.md](../PARITAET.md) (Teilnachweise), ADR-0003 bis ADR-0007. Eine Abnahme eines Pakets ist kein Nachweis vollständiger Parität (STATUS: keine vollständige phpMyAdmin-Parität).

## Bezug zu Entscheidungen

- **ADR-0006 (akzeptiert, Option c):** Der Serverzustand ist ein eigener Datenbestand des Dienstes (Konten, Passkey-Credentials, Sessions, Rollen und Teams, Freigaben, ggf. verschlüsselte Zugangsdaten, Audit). Er liegt in einer externen MariaDB und wird getrennt von den verwalteten Nutzerdatenbanken gespeichert. SZ1 ist das Fundament dafür; die Fragen 1 bis 11 in ADR-0006 bleiben offen.
- **Abgrenzung zum phpMyAdmin-Konfigurationsspeicher (pma__-Tabellen):** Diese Tabellen enthalten Komfortdaten je Konto (Verlauf, Lesezeichen, Favoriten, Tracking, Designerlayout, Benutzereinstellungen, Spaltenkommentare). Sie sind kein Serverzustand im Sinne von ADR-0006 und sollen nach ADR-0004 und ADR-0006 nicht in verwaltete Nutzerdatenbanken geschrieben werden. Ob sie zum Serverzustand gehören (Option a) oder lokal je Client liegen (Option b), ist offen und Gegenstand von P0.2. Die Tabellen pma__users und pma__usergroups (Oberflächenrechte) gehören zur Rollenfrage und sind mit ADR-0006 abzugleichen.
- **ADR-0007 (Vorschlag, nicht akzeptiert):** Nativer MySQL-/MariaDB-Adapter für die Apple-App (Paket A4, Branch `feat/mysql-adapter` im Apple-Repo). Der native Adapter bleibt laut ARBEITSPLAN bis zur Entscheidung gesperrt (TLS-Downgrade). Der Plan führt Apple-MySQL deshalb nicht als eigenes Umsetzungspaket, sondern als Entscheidung in P0.4.
- **ADR-0004 (SQLite):** exklusive lokale Einzelreplik-Datei, DELETE-Journal, kein WAL, kein NFS. Gilt für Web und Apple.
- **ADR-0003 (Compose und Swarm):** Betriebsziel, kein Funktionsumfang; der Betrieb ist in PROD-001 geführt.
- **ADR-0005 (Tresorformat):** Grundlage für verschlüsselte Zugangsdaten. Passkey-Login entsperrt den Tresor nicht (ADR-0006 Abschnitt 1).

## Ist-Stand-Abgleich mit STATUS.md

Ist-Stand heißt: im STATUS als abgenommen oder integriert genannt. Branch- oder Review-Stände sind davon getrennt markiert.

| STATUS-Paket | Stand laut main | Plan-Paket |
| --- | --- | --- |
| B1 Engine SQLite, MySQL, MariaDB, PostgreSQL (Go) | abgenommen | P1.1, P1.4 |
| B2a, B2b, B2c Metadaten, Sortierung, Filter, Einzelzeilen-Mutationen | abgenommen | P1.3, P2.1, P2.2 |
| B2b-Schutzfixes (SQLite OR ABORT, PostgreSQL-Rewrite) | abgenommen | P2.1 |
| B3a lesende Index- und Constraintmetadaten (API0.5) | abgenommen | P2.4 |
| B4a strukturierte neue Tabellen (API0.6) | abgenommen | P2.4 |
| A1, A2 Apple: SQLite und PostgreSQL ohne Go | abgenommen | P1.1, P1.4 |
| A-TF1 Apple TestFlight-Vorbereitung und Erstupload | integriert; Verarbeitung laut STATUS separat zu bestätigen | kein Featurepaket |
| A4 nativer MySQL-Adapter (ADR-0007) | Vorschlag, Branch im Apple-Repo, gesperrt | P0.4 |
| A5 | in den Dokumenten von main nicht nachweisbar | offen, nicht bewertet |
| F1, F1b, F2a, F2b, F3, F3b Web-SQL- und Tabellenarbeitsplatz | abgenommen | P1.1, P1.3, P1.4, P2.3 |
| F4, F4b Web-Einzelzeileneditor | abgenommen | P2.1 |
| F5, F5b Web-Strukturansicht | abgenommen | P2.4 (lesend) |
| F6 Export geladener Ergebnisse (Web) | abgenommen | P2.5, P3.3 |
| W1 Landingpage, Doku, Roadmap | integriert; Live-Nachweis separat | kein Featurepaket |
| S1, S2a Tresorformat und isolierter Core | abgenommen (Format, Core); kein produktiver Anker | P0.3 |
| SZ1 Serverzustand (ADR-0006) | Widerspruch: Merge-Commits auf main, SZ1-Handoff sagt 'nicht integriert', STATUS ohne Eintrag | P0.1 |
| C-Export1, A3 gemeinsamer Exportvertrag | Vorschlag, zurückgestellt | P2.5, P3.6, P4.10 |
| G-S2b Keychain-Anker (Apple) | offen | P0.3 |
| Zurückgestellt laut ARBEITSPLAN: Schemafunktionen, Import/Export, gespeicherte Abfragen, sichere native Netzwerkadapter, Passkeys, CloudKit-Sync, Teams/Rechte/Jobs | zurückgestellt | Phasen 2 bis 4, nicht vorgezogen |

**Folge für die Phasen:** Phase 1 und Teile von Phase 2 sind teilweise bereits umgesetzt. Phase 0 blockiert nur die Pakete, die unten ausdrücklich genannt sind (P2.6, P3.7, P4.6, P4.8 sowie die Apple-MySQL-Entscheidung und Versionsangaben). Der Rest von Phase 1 bis 3 ist nicht von Phase 0 abhängig.

## Grundsätze (Vorschlag)

1. **Größter Hebel zuerst:** Der Lesepfad (Verbindung, Objektbaum, Tabelle lesen, SQL, Fehler) ist Grundlage für fast alle weiteren Funktionen; die Reste davon stehen in Phase 1.
2. **Entscheidungen vor den Paketen, die von ihnen abhängen:** Zielversionen (Matrix-Grenze 3), Sicherheitsmodell, Ort des Konfigurationsspeichers, externe Dienste, Setup-Ersatz. Je Entscheidung ein ADR.
3. **Zwei Linien:** Web/Go und Apple (direkte Verbindung ohne Go, PROD-002). Jedes Paket wird je Linie getrennt abgenommen.
4. **Abnahme nach Matrix:** Jedes Paket nennt die Matrixzeilen. Die Abnahme gilt nur mit dem Abnahmekriterium der Zeile, je DB-Version und Plattform, mit unabhängigem Nachlesen in der Datenbank und mit Evidenz nach dem Schema aus PARITAET.md (Test-ID, Fixture, Version, Befehl, Datum, Ergebnis).

## Umfang je Priorität

| Priorität | Matrixzeilen | Neue Funde (Vorschlag) |
| --- | --- | --- |
| V1 | 12 | 10 |
| V2 | 37 | 19 |
| V3 | 64 | 26 |
| V4 | 68 | 64 |

Die Prioritäten sind Vorschläge der Matrix (Matrixzeilen) bzw. des Inventars (neue Funde). Die Zählung ist nicht die Reihenfolge.

## Phasen und Pakete

### Phase 0: Entscheidungen und Vorbedingungen

Ohne Implementierung. Klärt Zielsysteme, Sicherheitsmodell, Ort des Konfigurationsspeichers und externe Dienste. Nur die Pakete, die unten ausdrücklich davon abhängen, sind blockiert; der Rest von Phase 1 bis 3 läuft weiter.

#### P0.1 Serverzustand nach ADR-0006

- **Matrix:** keine
- **Neue Funde:** keine
- **Ist-Stand:** ADR-0006 akzeptiert (Option c: externe MariaDB als einziger Speicher des Serverzustands, Nutzerentscheidung 23.10.2026). SZ1 (Fundament, Branch `feat/sz1-server-state`): auf main liegen die Merge-Commits `90044dc` und `4853761`; der SZ1-Handoff (Zeile 5) sagt dagegen 'Nicht nach main integriert'. Dieser Widerspruch ist offen und vom Lead zu klären. STATUS.md führt SZ1 in keiner Tabelle.
- **Abhängigkeit:** Fragen 1 bis 11 in ADR-0006 Abschnitt 4 sind offen (u. a. Sessionmodell, Mandanten, Backup, Passkey-Rolle). Voraussetzung für P2.6 (Option a), P4.6 und PROD-004/PROD-006.
- **Linie:** Go/Web. Apple-App ohne Go ist davon nicht betroffen.
- **Abnahme:** ADR-0006 Abschnitt 5 (SZ1) und unabhängiger SZ1-Review; PROD-004 und PROD-006 bleiben separat offen.

#### P0.2 Ort des Konfigurationsspeichers (pma__-Modell)

- **Matrix:** keine
- **Neue Funde:** keine
- **Ist-Stand:** Nicht umgesetzt. Das phpMyAdmin-Modell (sql/create_tables.sql: pma__bookmark, pma__favorite, pma__history, pma__recent, pma__tracking, pma__designer_settings, pma__column_info, pma__userconfig, pma__savedsearches, pma__export_templates, pma__relation, pma__central_columns, pma__navigationhiding, pma__pdf_pages, pma__table_uiprefs, pma__users, pma__usergroups) legt Komfortdaten in einer Datenbank des Servers ab. Dieses Projekt speichert Dienstdaten nicht in verwalteten Nutzerdatenbanken (ADR-0004, ADR-0006 Abschnitt 1: Serverzustand ist ein anderer Datenbestand).
- **Abhängigkeit:** Entscheidung offen, neuer ADR nötig (Vorschlag). Option (a): Teil des Serverzustands nach ADR-0006, konto-bezogen; Option (b): lokal je Client (Apple-Gerät, Browser). pma__users und pma__usergroups (Oberflächenrechte) fallen unter 'Rollen' und sind mit ADR-0006 abzugleichen. Blockiert P2.6, P3.7 und P4.8.
- **Linie:** Web/Go und Apple getrennt zu entscheiden.
- **Abnahme:** Akzeptierter ADR; danach eigene Paketabnahme je Funktion.

#### P0.3 Sicherheitsmodell des Webs und Geheimnisse

- **Matrix:** keine
- **Neue Funde:** `PMA-NEU-SEC-001` (V4), `PMA-NEU-SEC-002` (V3), `PMA-NEU-SEC-003` (V3), `PMA-NEU-SEC-006` (V1)
- **Ist-Stand:** Session- und CSP-Befunde des Web-Stands sind geschlossen (F1/F1b, STATUS). CSRF und Sitzung: siehe P1.1. Apple-Geheimnisspeicher: G-S2b (Keychain-Anker) offen, kein produktiver Anchor. Verschlüsselte Zugangsdaten folgen S1/S2a (Tresorformat, Core isoliert).
- **Abhängigkeit:** Muss vor P1.1-Restarbeit, P4.6 und jeder Anmeldeerweiterung entschieden sein. Geheimnisfunktion nur mit Bedrohungsmodell (docs/security/BEDROHUNGSMODELL.md).
- **Linie:** Web/Go und Apple getrennt.
- **Abnahme:** Bedrohungsmodell aktualisiert; ADR für IP-Regeln, Anmeldeprotokoll, Config-Anmeldung und Geheimnisverwaltung.

#### P0.4 Zielsysteme und Datenbankadapter

- **Matrix:** keine
- **Neue Funde:** keine
- **Ist-Stand:** SQLite: ADR-0004 akzeptiert (exklusive lokale Einzeldatei); nativ in Go (B1) und Apple (A1/A2) belegt. PostgreSQL: Go (B1) und Apple (A2) belegt. MySQL/MariaDB: Go-Engine (B1) belegt; nativer Apple-Adapter laut ARBEITSPLAN 'gesperrt' wegen belegtem TLS-Downgrade. ADR-0007 (nativer MySQL-/MariaDB-Adapter für Apple) ist ein Vorschlag auf dem Branch `docs/adr-0007-native-mysql`, nicht akzeptiert.
- **Abhängigkeit:** Blockiert alle Apple-MySQL-Pakete (P1.1 Teil Apple) und alle Versionsangaben der Matrix (Matrix-Grenze 3). Abnahme nur für beschlossene Versionen.
- **Linie:** Go/Web: MySQL/MariaDB ist belegt. Apple: offen bis ADR-0007 entschieden ist.
- **Abnahme:** ADR-0007 akzeptiert oder verworfen; dokumentierte Versionsmatrix je Datenbanksystem.

#### P0.5 Externe Dienste und Produktinformationen

- **Matrix:** keine
- **Neue Funde:** `PMA-NEU-SEC-011` (V4), `PMA-NEU-MAINT-003` (V4), `PMA-NEU-MAINT-004` (V4), `PMA-NEU-MAINT-008` (V4), `PMA-NEU-ADMIN-004` (V4), `PMA-NEU-ADMIN-009` (V4)
- **Ist-Stand:** Nicht umgesetzt. Der Quellcode verbindet sich mit phpmyadmin.net (Versionsabruf, Git-Links) und mit reports.phpmyadmin.net (Fehlerbericht); Captcha nutzt einen externen Dienst. Matrix-Grenze 5 nennt diese Punkte ausdrücklich als offen.
- **Abhängigkeit:** Datenschutz- und Netzwerkentscheidung je Funktion; keine stillen Verbindungen nach außen. Systemressourcen (ADMIN-004) sind Host- statt Datenbankwerte.
- **Linie:** Web/Go. Apple: entfällt für Host- und Browserfunktionen.
- **Abnahme:** Je Funktion Entscheidung; bei Übernahme eigene Abnahme mit Netzwerkprotokoll ohne Nutzdaten.

### Phase 1: Lesender Grundpfad

Verbindung, Objektbaum, Tabelle lesen und SQL ausführen. Großteils bereits als Ist-Stand abgenommen (siehe Ist-Stand-Tabelle); hier steht nur der Rest.

#### P1.1 Verbindung, Anmeldung und Sitzung

- **Matrix:** `PMA-CON-001` (V1), `PMA-CON-002` (V1), `PMA-CON-003` (V1), `PMA-CON-004` (V2), `PMA-CON-005` (V1), `PMA-CON-007` (V1)
- **Neue Funde:** `PMA-NEU-SEC-005` (V1), `PMA-NEU-SEC-007` (V1), `PMA-NEU-SEC-008` (V1)
- **Ist-Stand:** Teilweise, Web: B1, F2b, F3b (Verbindung, Katalog, echter Webpfad, Sessionwechsel, verifiziertes TLS). Apple: A1/A2 (SQLite und PostgreSQL direkt, ohne Go, verifizierter TLS-Pfad). Offen: mehrere gespeicherte Server (CON-001) und Zeichensatzwahl (CON-004). Apple MySQL/MariaDB: P0.4.
- **Abhängigkeit:** P0.3 (CSRF- und Sitzungsrest, Ausgabe-Sanitisierung). P0.4 für Apple-MySQL.
- **Linie:** Web/Go: teilweise umgesetzt. Apple: SQLite/PostgreSQL umgesetzt; MySQL/MariaDB offen.
- **Abnahme:** Matrixkriterien CON-001 bis CON-005 und CON-007 je DB-Version und Plattform.

#### P1.2 Objektbaum, Katalog und Startseite

- **Matrix:** `PMA-OBJ-001` (V1), `PMA-OBJ-006` (V1)
- **Neue Funde:** `PMA-NEU-NAV-007` (V1)
- **Ist-Stand:** Teilweise: Katalogabfragen und Tabellenliste im Web-Arbeitsplatz (F1, F2a; Teilnachweis CON-005). Offen: exakte Zählung und Schätzung getrennt, Tabelle und View unterscheidbar (OBJ-006), Datenbankliste mit Schätzwerten (OBJ-001), Startseite (NAV-007).
- **Abhängigkeit:** P1.1.
- **Linie:** Web/Go teilweise; Apple offen.
- **Abnahme:** Matrixkriterien OBJ-001 und OBJ-006; Startseite nach eigenem Bedienablauf.

#### P1.3 Tabelle lesen: Seiten, Sortierung und Werte

- **Matrix:** `PMA-DATA-001` (V1), `PMA-DATA-002` (V2), `PMA-DATA-003` (V1)
- **Neue Funde:** `PMA-NEU-DATA-007` (V2), `PMA-NEU-DATA-008` (V2)
- **Ist-Stand:** Teilweise: B2a (Paginierung mit konservativem Stabilitätsmerkmal, Mehrspaltensortierung, typisierte NULL-, Leer- und Binärwerte, gebundene Filter) und Web F3/F3b. Offen: sichtbare Spaltenwahl (DATA-002). Inline-Gitter und Mehrspaltensortierung (DATA-007/008) sind im Web-Quellcode vorhanden; ob sie im integrierten Web-Stand enthalten sind, ist in STATUS.md nicht belegt.
- **Abhängigkeit:** P1.1, P1.2.
- **Linie:** Web/Go teilweise; Apple: keine Lese-Abnahme für Tabellenansicht in STATUS.md.
- **Abnahme:** Matrixkriterien DATA-001 bis DATA-003 mit Fixture und Vergleich gegen Direktabfrage.

#### P1.4 SQL ausführen und Fehler anzeigen

- **Matrix:** `PMA-SQL-001` (V1), `PMA-SQL-004` (V1)
- **Neue Funde:** keine
- **Ist-Stand:** Teilweise: B1, F2b (Web) sowie A1/A2 (Apple) mit einer einzelnen Anweisung im gewählten Kontext und begrenzten Ergebnissen. Fehlerdarstellung teilweise (Teilnachweis SQL-001/SQL-004). Offen: Warnungsdetails, Servercodes, Referenzvergleich.
- **Abhängigkeit:** P1.1.
- **Linie:** Web/Go und Apple teilweise umgesetzt; MySQL/MariaDB in Apple offen.
- **Abnahme:** Matrixkriterien SQL-001 und SQL-004 je Datenbanksystem.

#### P1.5 Serverinformationen lesen

- **Matrix:** `PMA-ADMIN-001` (V1)
- **Neue Funde:** keine
- **Ist-Stand:** Nicht in STATUS.md belegt; offen.
- **Abhängigkeit:** P1.1.
- **Linie:** Web/Go und Apple offen.
- **Abnahme:** Matrixkriterium ADMIN-001 gegen direkte Versionsabfrage.

### Phase 2: Schreibender Datenpfad und Schema-Basis (V2)

Einzelzeilen, Suche, SQL-Stapel, einfache Ein- und Ausgabe sowie Schema-Grundoperationen. Gesamtes Matrix-V2-Gerüst.

#### P2.1 Einzelzeilen schreiben

- **Matrix:** `PMA-DATA-004` (V2), `PMA-DATA-005` (V2), `PMA-DATA-006` (V2), `PMA-DATA-007` (V2), `PMA-DATA-008` (V2)
- **Neue Funde:** keine
- **Ist-Stand:** Teilweise: B2b/B2c (bestätigte Einzelzeilen-Insert, -Update, -Delete gegen vier Backendtreiber, Schutzfixes geschlossen); Web F4/F4b (Einzelzeileneditor, exakt reproduzierte Einbettung). Offen: mehrere Zeilen mit Teilerfolgen (DATA-006), Duplizieren (DATA-007). Apple: keine Schreib-Abnahme in STATUS.md.
- **Abhängigkeit:** Mutationsregeln der Matrix (Fehler, Teilerfolge, unbekannter Commit; DDL nicht transaktional).
- **Linie:** Web/Go teilweise; Apple offen.
- **Abnahme:** Matrixkriterien DATA-004 bis DATA-008 mit unabhängigem Nachlesen in der Datenbank.

#### P2.2 Suche, Auswahl und Dateiwerte

- **Matrix:** `PMA-DATA-009` (V2), `PMA-DATA-010` (V2), `PMA-REL-003` (V2)
- **Neue Funde:** `PMA-NEU-DATA-001` (V2), `PMA-NEU-DATA-002` (V2), `PMA-NEU-DATA-006` (V2)
- **Ist-Stand:** Teilweise: B2a mit gebundener Gleichheit, is-null und is-not-null-Filter (DATA-010). Offen: Bereich und LIKE, Dateiwerte (DATA-009), Fremdschlüssel-Auswahl mit Anzeigewert (REL-003).
- **Abhängigkeit:** P1.3, P2.1.
- **Linie:** Web/Go teilweise; Apple offen.
- **Abnahme:** Matrixkriterien DATA-009, DATA-010, REL-003 mit synthetischen Fixtures.

#### P2.3 SQL-Stapel, Ergebnismengen und Konsole

- **Matrix:** `PMA-SQL-002` (V2), `PMA-SQL-003` (V2), `PMA-SQL-005` (V2)
- **Neue Funde:** `PMA-NEU-NAV-003` (V2), `PMA-NEU-SQL-001` (V2), `PMA-NEU-SQL-002` (V2)
- **Ist-Stand:** Teilweise: Web-SQL-Arbeitsplatz F1/F1b und F2a/F2b (Browser, Go, DB, Quellen- und Lizenzdialog). Offen: Stapel mit Trennung in Strings und Routinen (SQL-002), mehrere Ergebnismengen (SQL-003), Editor-Vervollständigung und Formatierung (SQL-005).
- **Abhängigkeit:** P1.4. Konsole und Verlauf: P2.6 für den Verlauf.
- **Linie:** Web/Go teilweise; Apple offen.
- **Abnahme:** Matrixkriterien SQL-002, SQL-003, SQL-005.

#### P2.4 Schema-Basis: Anlegen, Lesen und Grundänderungen

- **Matrix:** `PMA-OBJ-002` (V2), `PMA-OBJ-007` (V2), `PMA-OBJ-009` (V2), `PMA-OBJ-014` (V2), `PMA-DDL-001` (V2), `PMA-DDL-002` (V2), `PMA-DDL-005` (V2), `PMA-DDL-007` (V2), `PMA-DDL-008` (V2), `PMA-DDL-009` (V2), `PMA-REL-001` (V2), `PMA-PROG-001` (V2)
- **Neue Funde:** `PMA-NEU-DDL-003` (V1), `PMA-NEU-OBJ-001` (V2), `PMA-NEU-REL-001` (V2)
- **Ist-Stand:** Teilweise: B4a (Tabelle anlegen mit festen Typen, API0.6), B3a (lesende Index- und Constraintmetadaten, API0.5), F5/F5b (Strukturansicht im Web, lesend). Offen: Spalten ändern (DDL-002), Schlüssel und Indizes schreiben (DDL-007 bis DDL-009), Datenbank anlegen (OBJ-002), Kopie (OBJ-009), Views (PROG-001), native Fremdschlüssel (REL-001).
- **Abhängigkeit:** P2.1 (Mutationsregeln). API0.6 gilt nur für die bestätigte Anlage, keine Änderung bestehender Objekte.
- **Linie:** Web/Go teilweise; Apple offen.
- **Abnahme:** Matrixkriterien je Zeile mit Schema-Fixture und Nachlesen der Metadaten.

#### P2.5 Einfache Ein- und Ausgabe

- **Matrix:** `PMA-EXP-001` (V2), `PMA-EXP-002` (V2), `PMA-EXP-006` (V2), `PMA-EXP-007` (V2), `PMA-IMP-001` (V2), `PMA-IMP-005` (V2), `PMA-IMP-006` (V2)
- **Neue Funde:** `PMA-NEU-EXP-002` (V2), `PMA-NEU-IMP-001` (V2), `PMA-NEU-IMP-003` (V2)
- **Ist-Stand:** Teilweise: F6 (Export geladener Ergebnisse ohne zusätzliche DB-Abfrage; typisiertes JSON und getrennte CSV-Profile, Web) deckt einen Teil von EXP-007 und EXP-009 ab. Offen: Exportumfang (EXP-001), SQL-Export (EXP-006), Dateiimport (IMP-001, IMP-005, IMP-006). C-Export1/A3 (gemeinsamer Exportvertrag) ist ein Vorschlag und zurückgestellt.
- **Abhängigkeit:** P1.2, P2.1. Gemeinsamer Exportvertrag (C-Export1/A3) vor Abnahme neuer Exportformate.
- **Linie:** Web/Go teilweise; Apple offen.
- **Abnahme:** Matrixkriterien je Format mit Referenzdatei und unabhängigem Leser.

#### P2.6 Konfigurationsspeicher umsetzen (nach P0.2)

- **Matrix:** `PMA-CON-006` (V2), `PMA-SQL-008` (V2), `PMA-SQL-009` (V2), `PMA-UX-003` (V2), `PMA-UX-004` (V3), `PMA-UX-011` (V3)
- **Neue Funde:** `PMA-NEU-NAV-001` (V3), `PMA-NEU-NAV-002` (V4), `PMA-NEU-NAV-004` (V4), `PMA-NEU-NAV-005` (V4), `PMA-NEU-DATA-003` (V3), `PMA-NEU-DDL-002` (V3), `PMA-NEU-SQL-004` (V3), `PMA-NEU-SQL-005` (V3), `PMA-NEU-UX-002` (V4), `PMA-NEU-UX-003` (V3), `PMA-NEU-MAINT-007` (V3)
- **Ist-Stand:** Nicht umgesetzt. Blockiert durch P0.2 (Ort und Konto-Bezug der Komfortdaten).
- **Abhängigkeit:** P0.2; bei Option (a) P0.1.
- **Linie:** Web/Go und Apple getrennt; Apple-Speicher nach P0.2 zu entscheiden.
- **Abnahme:** Matrixkriterien CON-006, SQL-008, SQL-009, UX-003, UX-004, UX-011 mit zwei Konten.

#### P2.7 Bedienung mit Tastatur und Touch

- **Matrix:** `PMA-UX-007` (V2)
- **Neue Funde:** `PMA-NEU-UX-004` (V2)
- **Ist-Stand:** Teilweise: keyhandler.js und shortcuts_handler.js im Quellcode; Umfang je Funktion nicht geprüft. Apple: keine Tastatur- oder Touch-Abnahme in STATUS.md.
- **Abhängigkeit:** P1.1 bis P2.3 (Bedienpfade).
- **Linie:** Web/Go und Apple getrennt; Apple nativ zu entwerfen.
- **Abnahme:** Matrixkriterium UX-007 mit dokumentiertem Tastatur- und Touchablauf.

### Phase 3: Administration (V3)

Konten und Rechte der Datenbank, Serververwaltung, Struktur- und Programmobjekte, Import- und Exportvorlagen.

#### P3.1 Konten, Rechte und Teilen (Datenbankkonten)

- **Matrix:** `PMA-PRIV-001` (V3), `PMA-PRIV-002` (V3), `PMA-PRIV-003` (V3), `PMA-PRIV-004` (V3), `PMA-PRIV-005` (V3), `PMA-PRIV-006` (V3), `PMA-PRIV-007` (V3), `PMA-PRIV-009` (V3), `PMA-PRIV-010` (V3), `PMA-SQL-011` (V3)
- **Neue Funde:** `PMA-NEU-PRIV-001` (V3), `PMA-NEU-PRIV-002` (V3), `PMA-NEU-PRIV-003` (V2), `PMA-NEU-UX-001` (V3)
- **Ist-Stand:** Nicht umgesetzt. Datenbankkonten sind Konten des Datenbankservers und nicht Konten des Produkts. Produktkonten, Teams und Freigaben: P0.1 und PROD-006.
- **Abhängigkeit:** P1.1, P2.1; Rechtequelle (SHOW GRANTS). Produktkonten und Teilen von Lesezeichen hängen an P0.1.
- **Linie:** Web/Go; Apple nur mit eigenem Serverrecht.
- **Abnahme:** Matrixkriterien PRIV-001 bis PRIV-007, PRIV-009, PRIV-010, SQL-011 mit zwei Konten.

#### P3.2 Serververwaltung und Wartung

- **Matrix:** `PMA-ADMIN-002` (V3), `PMA-ADMIN-003` (V3), `PMA-ADMIN-004` (V3), `PMA-ADMIN-005` (V3), `PMA-ADMIN-008` (V3), `PMA-ADMIN-009` (V3), `PMA-MAINT-001` (V3), `PMA-MAINT-002` (V3), `PMA-MAINT-003` (V3)
- **Neue Funde:** `PMA-NEU-ADMIN-001` (V3), `PMA-NEU-ADMIN-005` (V3)
- **Ist-Stand:** Nicht umgesetzt.
- **Abhängigkeit:** P1.5, P2.1.
- **Linie:** Web/Go; Apple nur mit Serverrecht.
- **Abnahme:** Matrixkriterien je Zeile mit Serverfixture.

#### P3.3 Suchen, Ersetzen und SQL-Werkzeuge

- **Matrix:** `PMA-DATA-011` (V3), `PMA-DATA-012` (V3), `PMA-SQL-006` (V3), `PMA-SQL-010` (V3), `PMA-SQL-012` (V3), `PMA-SQL-013` (V3)
- **Neue Funde:** `PMA-NEU-SQL-003` (V3)
- **Ist-Stand:** Teilweise: F6 deckt einen Teil von SQL-012 (Export geladener Ergebnisse). Offen: datenbankweite Suche, Ersetzen, EXPLAIN, Lesezeichen-Variablen, Ergebnisoperationen und DML-Simulation.
- **Abhängigkeit:** P2.2, P2.3, P2.6 (Lesezeichen-Variablen).
- **Linie:** Web/Go; Apple offen.
- **Abnahme:** Matrixkriterien je Zeile; DML-Vorschau darf keine Zeile verändern.

#### P3.4 Struktur ändern (schreibende DDL)

- **Matrix:** `PMA-OBJ-003` (V3), `PMA-OBJ-004` (V3), `PMA-OBJ-005` (V3), `PMA-OBJ-008` (V3), `PMA-OBJ-010` (V3), `PMA-OBJ-011` (V3), `PMA-OBJ-012` (V3), `PMA-DDL-003` (V3), `PMA-DDL-004` (V3), `PMA-DDL-006` (V3), `PMA-DDL-010` (V3), `PMA-DDL-011` (V3)
- **Neue Funde:** `PMA-NEU-DDL-001` (V3)
- **Ist-Stand:** Nicht umgesetzt. B4a und B3a liefern nur Anlage und Lesen.
- **Abhängigkeit:** P2.4; Mutationsregeln (DDL nicht pauschal transaktional). Destruktive Aktionen zeigen Ziel und Umfang vor Ausführung.
- **Linie:** Web/Go; Apple offen.
- **Abnahme:** Matrixkriterien je Zeile mit Schema-Fixture und Nachlesen.

#### P3.5 Programmobjekte

- **Matrix:** `PMA-PROG-002` (V3), `PMA-PROG-003` (V3), `PMA-PROG-004` (V3), `PMA-PROG-005` (V3), `PMA-PROG-006` (V3), `PMA-PROG-007` (V3)
- **Neue Funde:** `PMA-NEU-PROG-001` (V3), `PMA-NEU-PROG-002` (V3)
- **Ist-Stand:** Nicht umgesetzt.
- **Abhängigkeit:** P2.4 (Views); Event-Scheduler im isolierten Test.
- **Linie:** Web/Go; Apple offen.
- **Abnahme:** Matrixkriterien PROG-002 bis PROG-007; Event-Lauf nur isoliert.

#### P3.6 Import, Export, Dump und Serverexport (V3)

- **Matrix:** `PMA-IMP-002` (V3), `PMA-IMP-004` (V3), `PMA-IMP-007` (V3), `PMA-IMP-008` (V3), `PMA-IMP-013` (V3), `PMA-IMP-014` (V3), `PMA-EXP-003` (V3), `PMA-EXP-004` (V3), `PMA-EXP-008` (V3), `PMA-EXP-009` (V3), `PMA-DUMP-001` (V3), `PMA-DUMP-002` (V3), `PMA-DUMP-004` (V3), `PMA-DUMP-005` (V3)
- **Neue Funde:** `PMA-NEU-ADMIN-007` (V3), `PMA-NEU-ADMIN-008` (V3), `PMA-NEU-EXP-001` (V3), `PMA-NEU-EXP-004` (V3), `PMA-NEU-IMP-002` (V2), `PMA-NEU-IMP-004` (V3)
- **Ist-Stand:** Teilweise: C-Export1/A3 als Vorschlag zurückgestellt; F6 nur für Ergebnisexport. Import-Fortschritt, Ziehen-und-Ablegen und Kodierung sind im Quellcode belegt, nicht als Ist-Stand.
- **Abhängigkeit:** P2.5; gemeinsamer Exportvertrag vor Exportpaketen; Kodierung (EXP-004).
- **Linie:** Web/Go teilweise; Apple offen (Dateiauswahl nativ zu entwerfen).
- **Abnahme:** Matrixkriterien je Zeile mit Referenzdatei; Dump-Variante je Option.

#### P3.7 Designer, Beziehungen und visuelle Abfragen

- **Matrix:** `PMA-REL-005` (V3), `PMA-REL-006` (V4), `PMA-REL-010` (V3)
- **Neue Funde:** `PMA-NEU-REL-002` (V3), `PMA-NEU-REL-003` (V3)
- **Ist-Stand:** Nicht umgesetzt. Designerdaten liegen im Konfigurationsspeicher (P0.2).
- **Abhängigkeit:** P0.2, P2.4 (native Beziehungen), P1.4.
- **Linie:** Web/Go; Apple nach Entscheidung zur Oberfläche.
- **Abnahme:** Matrixkriterien REL-005, REL-006, REL-010 mit zwei Tabellen und einer Beziehung.

#### P3.8 Spaltendarstellung und Editoren (V3)

- **Matrix:** `PMA-TRANS-003` (V3), `PMA-TRANS-006` (V3), `PMA-TRANS-007` (V3)
- **Neue Funde:** keine
- **Ist-Stand:** Nicht umgesetzt; Transformationen sind in STATUS.md nicht genannt.
- **Abhängigkeit:** P0.2 für die Zuordnung je Spalte (TRANS-001 in P4.7).
- **Linie:** Web/Go; Apple offen.
- **Abnahme:** Matrixkriterien je Zeile; Editor-Eingabe unverändert gespeichert.

### Phase 4: Erweiterte Parität (V4)

Geodaten, Diagramme, Schemaexport, Replikation und Binärlog, Transformationen, Export- und Importformate. Die Kernfunktionen sind damit nicht eingeschlossen; die Reihenfolge innerhalb von Phase 4 ist frei.

#### P4.1 Geodaten

- **Matrix:** `PMA-DATA-013` (V4), `PMA-DATA-014` (V4), `PMA-IMP-012` (V4)
- **Neue Funde:** `PMA-NEU-DATA-004` (V4), `PMA-NEU-DATA-005` (V4)
- **Ist-Stand:** Nicht umgesetzt. Kartendarstellung im Browser und in Apple: Entscheidung offen.
- **Abhängigkeit:** Karten- und Bibliotheksentscheidung (OpenLayers im Quellcode); GD und TCPDF für Bild- und PDF-Ausgabe.
- **Linie:** Web/Go; Apple offen.
- **Abnahme:** Matrixkriterien DATA-013, DATA-014, IMP-012 mit Geometriefixture (SRID).

#### P4.2 Oberfläche, Themen, Diagramme und Menüs

- **Matrix:** `PMA-UX-001` (V4), `PMA-UX-002` (V4), `PMA-UX-005` (V4), `PMA-UX-006` (V4)
- **Neue Funde:** `PMA-NEU-NAV-006` (V4)
- **Ist-Stand:** Nicht umgesetzt. Themen und Sprachen: Quellbestand vorhanden (Themen, 95 Sprachkataloge); Abdeckung nicht geprüft.
- **Abhängigkeit:** P2.6 für gespeicherte Auswahl; Diagrammbibliothek vor Übernahme lizenzieren.
- **Linie:** Web/Go; Apple offen (Diagramme, Themen, Sprache).
- **Abnahme:** Matrixkriterien UX-001, UX-002, UX-005, UX-006 mit Bedienablauf.

#### P4.3 Beziehungen erweitert und Schemaexport

- **Matrix:** `PMA-REL-002` (V4), `PMA-REL-004` (V4), `PMA-REL-007` (V4), `PMA-REL-008` (V4), `PMA-REL-009` (V4), `PMA-REL-011` (V4)
- **Neue Funde:** `PMA-NEU-REL-004` (V4)
- **Ist-Stand:** Nicht umgesetzt.
- **Abhängigkeit:** P3.7; Schema-Exportformate nach gemeinsamem Exportvertrag.
- **Linie:** Web/Go; Apple offen (PDF und Office benötigen Bibliotheksentscheidung).
- **Abnahme:** Matrixkriterien je Format mit unabhängigem Leser.

#### P4.4 Dump-Details

- **Matrix:** `PMA-DUMP-003` (V4), `PMA-DUMP-006` (V4), `PMA-DUMP-007` (V4), `PMA-DUMP-008` (V4)
- **Neue Funde:** `PMA-NEU-DUMP-001` (V4)
- **Ist-Stand:** Nicht umgesetzt.
- **Abhängigkeit:** P3.6.
- **Linie:** Web/Go; Apple offen.
- **Abnahme:** Matrixkriterien je Zeile mit Referenzdump.

#### P4.5 Server, Replikation, Binärlog und Diagnose

- **Matrix:** `PMA-ADMIN-006` (V4), `PMA-ADMIN-007` (V4), `PMA-ADMIN-010` (V4), `PMA-ADMIN-011` (V4), `PMA-ADMIN-012` (V4), `PMA-MAINT-004` (V4), `PMA-MAINT-005` (V4), `PMA-MAINT-006` (V4), `PMA-PRIV-008` (V4), `PMA-SQL-007` (V4)
- **Neue Funde:** `PMA-NEU-ADMIN-002` (V4), `PMA-NEU-ADMIN-003` (V4), `PMA-NEU-ADMIN-006` (V4), `PMA-NEU-ADMIN-010` (V4), `PMA-NEU-ADMIN-011` (V4), `PMA-NEU-ADMIN-012` (V4), `PMA-NEU-PROG-003` (V4)
- **Ist-Stand:** Nicht umgesetzt. Replikationsänderungen verlangen eine Zielversionsentscheidung (P0.4).
- **Abhängigkeit:** P3.2, P0.4; Replikation nur mit isoliertem Replikationspaar.
- **Linie:** Web/Go; Apple: Serverzugriff nötig, offen.
- **Abnahme:** Matrixkriterien je Zeile; Teilschritte protokolliert.

#### P4.6 Anmeldeverfahren, Zweiter Faktor und Teams (nach P0.1 und P0.3)

- **Matrix:** `PMA-UX-008` (V4), `PMA-UX-009` (V4), `PMA-UX-010` (V4), `PMA-UX-012` (V4)
- **Neue Funde:** `PMA-NEU-SEC-009` (V4)
- **Ist-Stand:** Nicht umgesetzt. Passkey-Login ist kein Entsperrweg (ADR-0006, Bezug PMA-UX-009). Teams und Freigaben nicht entschieden (ADR-0006 Abschnitt 6).
- **Abhängigkeit:** P0.1, P0.3; Wiederherstellungsweg vor TOTP oder Passkey.
- **Linie:** Web/Go; Apple: Passkey-Login mit eigener Entscheidung.
- **Abnahme:** Matrixkriterien je Verfahren; Passkey-Entsperren ausgeschlossen.

#### P4.7 Transformationen und Plugins

- **Matrix:** `PMA-TRANS-001` (V4), `PMA-TRANS-002` (V4), `PMA-TRANS-004` (V4), `PMA-TRANS-005` (V4), `PMA-TRANS-008` (V4), `PMA-TRANS-009` (V4), `PMA-TRANS-010` (V4), `PMA-TRANS-011` (V4)
- **Neue Funde:** `PMA-NEU-TRANS-001` (V4), `PMA-NEU-TRANS-002` (V4), `PMA-NEU-TRANS-003` (V4), `PMA-NEU-TRANS-004` (V4), `PMA-NEU-TRANS-005` (V4), `PMA-NEU-TRANS-006` (V4), `PMA-NEU-TRANS-007` (V4), `PMA-NEU-TRANS-008` (V4), `PMA-NEU-TRANS-009` (V4), `PMA-NEU-TRANS-010` (V4), `PMA-NEU-TRANS-011` (V4), `PMA-NEU-TRANS-012` (V4), `PMA-NEU-TRANS-013` (V4), `PMA-NEU-TRANS-014` (V4), `PMA-NEU-TRANS-015` (V4), `PMA-NEU-TRANS-016` (V4), `PMA-NEU-TRANS-017` (V4), `PMA-NEU-TRANS-018` (V4), `PMA-NEU-TRANS-019` (V4), `PMA-NEU-TRANS-020` (V4), `PMA-NEU-TRANS-021` (V4), `PMA-NEU-TRANS-022` (V4), `PMA-NEU-TRANS-023` (V4), `PMA-NEU-TRANS-024` (V4), `PMA-NEU-TRANS-025` (V4), `PMA-NEU-TRANS-026` (V4), `PMA-NEU-TRANS-027` (V4), `PMA-NEU-TRANS-028` (V4), `PMA-NEU-TRANS-029` (V4), `PMA-NEU-TRANS-030` (V4), `PMA-NEU-TRANS-031` (V4), `PMA-NEU-TRANS-032` (V4), `PMA-NEU-TRANS-033` (V4)
- **Ist-Stand:** Nicht umgesetzt. Die 31 Einzelplugins sind nur namentlich erfasst.
- **Abhängigkeit:** P0.2 (Zuordnung je Spalte); TRANS-010 (externe Programme) nur nach Bedrohungsentscheidung.
- **Linie:** Web/Go; Apple offen.
- **Abnahme:** Matrixkriterien; je Plugin Optionenliste nach Einzelprüfung (UNSICHER-Zeilen).

#### P4.8 Metadaten-Werkzeuge: Tracking, Zentralspalten, Normalisierung

- **Matrix:** `PMA-META-001` (V4), `PMA-META-002` (V4), `PMA-META-003` (V4), `PMA-META-004` (V4), `PMA-META-005` (V4), `PMA-META-006` (V4), `PMA-META-007` (V4), `PMA-META-008` (V4)
- **Neue Funde:** `PMA-NEU-META-001` (V4), `PMA-NEU-META-002` (V4)
- **Ist-Stand:** Nicht umgesetzt.
- **Abhängigkeit:** P2.6 (Konfigurationsspeicher); Normalisierung braucht Schreibpfad P3.4.
- **Linie:** Web/Go; Apple offen.
- **Abnahme:** Matrixkriterien je Zeile; Tracking erfasst nur App-Änderungen (Matrixvorgabe).

#### P4.9 Partitionen und Stapeloperationen

- **Matrix:** `PMA-DDL-012` (V4), `PMA-DDL-013` (V4), `PMA-DDL-014` (V4), `PMA-OBJ-013` (V4)
- **Neue Funde:** keine
- **Ist-Stand:** Nicht umgesetzt.
- **Abhängigkeit:** P3.4.
- **Linie:** Web/Go; Apple offen.
- **Abnahme:** Matrixkriterien mit partitionierter Fixture.

#### P4.10 Export- und Importformate erweitert

- **Matrix:** `PMA-EXP-005` (V4), `PMA-EXP-010` (V4), `PMA-EXP-011` (V4), `PMA-EXP-012` (V4), `PMA-EXP-013` (V4), `PMA-EXP-014` (V4), `PMA-EXP-015` (V4), `PMA-EXP-016` (V4), `PMA-EXP-017` (V4), `PMA-EXP-018` (V4), `PMA-EXP-019` (V4), `PMA-EXP-020` (V4), `PMA-IMP-003` (V4), `PMA-IMP-009` (V4), `PMA-IMP-010` (V4), `PMA-IMP-011` (V4)
- **Neue Funde:** `PMA-NEU-EXP-003` (V4)
- **Ist-Stand:** Nicht umgesetzt. Office- und PDF-Formate: Bibliotheksfragen (Lizenz) offen.
- **Abhängigkeit:** P2.5, P3.6; gemeinsamer Exportvertrag (C-Export1/A3). Serververzeichnisse (SaveDir, UploadDir) nach Pfadentscheidung.
- **Linie:** Web/Go; Apple offen.
- **Abnahme:** Matrixkriterien je Format mit unabhängigem Leser (Matrixvorgabe).

### Phase 5: Betrieb, Einrichtung und Web-Eigenlinie

Funktionen, die an PHP-Laufzeit, Webserver oder Browser gebunden sind, sowie Produktinformationen. Keine Kern-Parität; Übernahme nur nach Entscheidung.

#### P5.1 Einrichtung und Konfiguration (Web-Eigenlinie)

- **Matrix:** keine
- **Neue Funde:** `PMA-NEU-SETUP-001` (V1), `PMA-NEU-SETUP-002` (V1), `PMA-NEU-SETUP-003` (V1), `PMA-NEU-SETUP-004` (V2), `PMA-NEU-SETUP-005` (V3), `PMA-NEU-SETUP-006` (V4)
- **Ist-Stand:** Nicht umgesetzt. Der Betrieb über Docker Compose und Swarm (ADR-0003, PROD-001) ist davon getrennt und bereits Ist-Stand.
- **Abhängigkeit:** Entscheidung A8 (Setup-Ersatz durch Produktkonfiguration).
- **Linie:** Web/Go; für Apple entfällt.
- **Abnahme:** Nur nach Entscheidung; Konfigurationsschreiben nur mit Rechteprüfung (SETUP-005).

#### P5.2 Produktinformationen des eigenen Produkts

- **Matrix:** keine
- **Neue Funde:** `PMA-NEU-MAINT-005` (V1), `PMA-NEU-MAINT-006` (V2)
- **Ist-Stand:** Lizenz: docs/LIZENZEN.md vorhanden; Anzeige im Produkt nicht belegt. Änderungsprotokoll des Produkts nicht vorhanden.
- **Abhängigkeit:** Projektregel: Lizenz- und Updateinformationen des eigenen Produkts gehören zur Releaseprüfung (PARITAET.md, Vollständigkeitsgrenze 5).
- **Linie:** Web/Go und Apple.
- **Abnahme:** Lizenzanzeige (Pflicht) und Änderungsprotokoll im Release; kein Fremdtext.

#### P5.3 Nicht übernehmen oder nur mit Entscheidung

- **Matrix:** keine
- **Neue Funde:** `PMA-NEU-MAINT-001` (V4), `PMA-NEU-MAINT-002` (V4), `PMA-NEU-SEC-004` (V2), `PMA-NEU-SEC-010` (V4)
- **Ist-Stand:** Nicht vorgesehen. phpinfo und Git-Revision sind Installationsdetails des Referenzprodukts; Referer-Schutz entfällt für die App; der Zweitfaktor 'Simple' ist laut Quelle nur für Tests.
- **Abhängigkeit:** Entscheidung; ohne Entscheidung keine Umsetzung.
- **Linie:** Web/Go; Apple entfällt.
- **Abnahme:** Nicht übernehmen, außer die Entscheidung verlangt es.

## Zuordnungskontrolle

- Matrix-IDs im Inventar: 181; jede genau einem Paket zugeordnet: 181 von 181; doppelt: 0; nicht zugeordnet: 0.
- Neue Funde im Inventar: 119; jeder genau einem Paket zugeordnet: 119 von 119; doppelt: 0; nicht zugeordnet: 0.
- Zurückgestellt ohne Paket (bewusst 'später'): keine. Zurückgestellte Bereiche laut ARBEITSPLAN sind in Phase 2 bis 4 eingeordnet, aber nicht vorgezogen.

## Linien Web/Go und Apple

- **Web/Go:** alle Phasen. Die Betriebsziele Docker Compose und Swarm sind bestätigt (ADR-0003, PROD-001) und nicht Teil dieses Plans. Setup und Einrichtung (P5.1) sind eine eigene Linie.
- **Apple:** Phasen 1 bis 4 direkt, soweit die Quelle keine Server- oder Browserabhängigkeit verlangt. Ausgenommen sind Funktionen mit 'Apple entfällt' (Setup, Konfigurationsdatei, phpinfo, Git-Revision, Referer-Schutz, Versionsprüfung, Fehlerbericht). Offen für Apple sind unter anderem MySQL/MariaDB direkt (P0.4), Kartendarstellung (P4.1), Diagramme und Themen (P4.2), Replikation und Binärlog (P4.5, Serverzugriff) sowie Exporte mit Bibliotheksfragen (PDF, Office; P4.10).
- Zahl der Funde je Eignung: siehe [README](README.md), Abschnitt Ergebnis.

## Offene Punkte und Widersprüche

1. **SZ1:** Die Merge-Commits `90044dc` und `4853761` liegen auf `main`. Der SZ1-Handoff (Zeile 5) sagt 'Nicht nach main integriert', und STATUS.md führt SZ1 nicht. Der Lead klärt den Widerspruch, bevor P0.1 und P2.6 Option (a) verwendet werden.
2. **A4 und A5:** A4 ist ADR-0007 (Vorschlag). A5 ist in den Dokumenten von main nicht nachweisbar und wird nicht bewertet.
3. **C-Export1/A3:** gemeinsamer Exportvertrag ist ein Vorschlag und zurückgestellt; er ist Voraussetzung für neue Exportformate (P2.5, P3.6, P4.3, P4.10).
4. **G-S2b:** Keychain-Anker auf geeignetem Host nicht belegt; betrifft Apple-Geheimnisspeicher (P0.3).
5. **Zielversionen:** Matrix-Grenze 3 fehlt; ohne Versionsmatrix ist keine Abnahme eindeutig.
6. **Lizenzen:** Vor Übernahme sind externe Bibliotheken zu inventarisieren: TCPDF (PDF), OpenLayers (Karten), CodeMirror (Editoren), Diagrammbibliothek, TOTP- und WebAuthn-Bibliotheken (AGENTS.md: Abhängigkeiten behalten ihre Lizenzen).
7. **Matrix-Teilnachweise:** Sammel-IDs (siehe LUECKEN A1) sind je ID aufzulösen, bevor Teilnachweise als Abnahme gelten.
8. **Web-Stand:** Der Plan prüft den tatsächlichen Web-Stand nicht erneut. Abweichungen zwischen STATUS.md und dem Quellcode der Web-Repos sind beim Lead abzugleichen.
