# Umsetzungsvorschlag (Entwurf, nicht beschlossen)

Status: **Vorschlag.** Keine Entscheidung, keine Zusage für Termine oder Umfang. Ein Lead entscheidet über Reihenfolge und Pakete.
Grundlage: Matrixprioritäten V1 bis V4 aus [PARITAET.md](../PARITAET.md), neue Funde aus [LUECKEN.md](LUECKEN.md) und die Eignung je Zeile.
Umsetzungsstände aus `docs/STATUS.md` und `docs/ARBEITSPLAN.md` wurden nicht gegengeprüft; der Lead muss den Vorschlag damit abgleichen.

## Grundsätze (Vorschlag)

1. **Größter Hebel zuerst:** Der Grundpfad V1 (Verbindung, Objektbaum, Tabelle lesen, SQL ausführen, Fehler anzeigen) ist Voraussetzung für fast alle weiteren Funktionen.
2. **Entscheidungen vor Paketen:** Zielversionen je Datenbanksystem (Matrix-Grenze 3), Sicherheitsmodell (Sitzung, CSRF, Geheimnisse, IP-Regeln),
   Modell des Konfigurationsspeichers (pma__-Tabellen), externe Verbindungen (Versionsprüfung, Fehlerbericht), Setup-Ersatz. Je Entscheidung ein ADR.
3. **Zwei Linien:** Web/Go (HTTP-Backend) und Apple (direkte Verbindung ohne Go, PROD-002). Jedes Paket wird je Linie getrennt abgenommen.
4. **Abnahme nach Matrix:** Jedes Paket nennt die Matrixzeilen. Abnahme gilt nur mit dem Abnahmekriterium der Zeile, je DB-Version und Plattform, mit unabhängigem Nachlesen in der Datenbank.

## Umfang je Priorität

| Priorität | Matrixzeilen | Neue Funde (Vorschlag) |
| --- | --- | --- |
| V1 | 12 | 10 |
| V2 | 37 | 16 |
| V3 | 64 | 24 |
| V4 | 68 | 62 |

Die Zahlen für neue Funde beziehen sich auf die Vorschlag-Priorität im Inventar, nicht auf eine Entscheidung.

## Phasen (Vorschlag)

### Phase 0: Entscheidungen und Vorbedingungen

- **Inhalt:** Zielversionen (MySQL/MariaDB, PostgreSQL, SQLite); ADR Sicherheitsmodell (`PMA-NEU-SEC-001` bis `-010`, insbesondere CSRF, Sitzung, Geheimnis); ADR Konfigurationsspeicher, weil Verlauf, Lesezeichen, Favoriten, Tracking, Designer und Benutzereinstellungen darauf aufbauen (`PMA-CON-006`, `PMA-SQL-008`, `PMA-SQL-009`, `PMA-UX-003`, `PMA-REL-006`, `PMA-META-001`); Entscheidung zu Produktinformationen (`PMA-NEU-MAINT-001` bis `-004`).
- **Abhängigkeiten:** Keine; blockiert alle Phasen.
- **Abnahme:** Akzeptierte ADR; dokumentierte Versionsmatrix.

### Phase 1: Lesender Grundpfad (V1)

- **Paket 1.1 Verbindung und Navigation:** `PMA-CON-001` bis `PMA-CON-007` (ohne Favoriten `CON-006`, das zu Phase 0 gehört), `PMA-NEU-NAV-007` (Startseite), `PMA-ADMIN-001` (Serverinfo).
- **Paket 1.2 Objektbaum und Lesen:** `PMA-OBJ-001`, `PMA-OBJ-006`, `PMA-DATA-001`, `PMA-DATA-003`.
- **Paket 1.3 SQL-Ausführung:** `PMA-SQL-001`, `PMA-SQL-004`; Sicherheitsbasis `PMA-NEU-SEC-005` (CSRF) und `PMA-NEU-SEC-007` (Sitzung).
- **Abhängigkeiten:** Phase 0 (Sicherheitsmodell, Zielversion MySQL/MariaDB).
- **Hebel:** Höchster Hebel; jede weitere Funktion setzt Verbindung, Baum, Lesen und SQL voraus.
- **Linien:** Web/Go ist der bestehende Pfad in der Matrix. Apple: SQLite und PostgreSQL direkt sind laut PROD-002 (Teilnachweise A1 und A2) bereits belegt; MySQL/MariaDB direkt ohne Go ist als eigenes Paket offen.

### Phase 2: Schreiben und Datenpfad (V1-Rest, V2)

- **Paket 2.1 Zeilen:** `PMA-DATA-004` bis `PMA-DATA-008` (V2), Suche `PMA-DATA-010`, Sortierung `PMA-DATA-002`, Fremdschlüsselauswahl `PMA-REL-003` mit `PMA-NEU-DATA-001` und `PMA-NEU-DATA-002`.
- **Paket 2.2 Schema:** `PMA-OBJ-007`, `PMA-OBJ-014`, `PMA-DDL-001`, `PMA-DDL-002`, `PMA-DDL-005`, `PMA-DDL-007` bis `PMA-DDL-009` (V2).
- **Paket 2.3 Verlauf und Lesezeichen:** `PMA-SQL-002`, `PMA-SQL-003`, `PMA-SQL-008`, `PMA-SQL-009` (V2); braucht Phase 0 (Konfigurationsspeicher).
- **Paket 2.4 Einfache Ein- und Ausgabe:** Export `PMA-EXP-001`, `PMA-EXP-002`, `PMA-EXP-006`, `PMA-EXP-007`; Import `PMA-IMP-001`, `PMA-IMP-005`, `PMA-IMP-006` (V2).
- **Abhängigkeiten:** Phase 1 (Lesepfad), Phase 0 (Konfigurationsspeicher für Verlauf).
- **Hinweis:** Schreibfunktionen folgen der Matrixvorgabe zu Fehlern, Teilerfolgen und DDL ohne Transaktion (PARITAET.md, Abschnitt Leseschlüssel).

### Phase 3: Administration (V3)

- **Paket 3.1 Konten und Rechte:** `PMA-PRIV-001` bis `PMA-PRIV-010`; Rechteermittlung `PMA-NEU-PRIV-003` als Voraussetzung für Oberflächenaktionen.
- **Paket 3.2 Server:** `PMA-ADMIN-003`, `PMA-ADMIN-008`, `PMA-ADMIN-009`, `PMA-ADMIN-004`, `PMA-ADMIN-005`; Wartung `PMA-MAINT-001` bis `PMA-MAINT-006`.
- **Paket 3.3 Struktur:** `PMA-DDL-003`, `PMA-DDL-004`, `PMA-DDL-010`, `PMA-DDL-011`; `PMA-OBJ-003`, `PMA-OBJ-004`, `PMA-OBJ-005`, `PMA-OBJ-008`, `PMA-OBJ-010` bis `PMA-OBJ-012`.
- **Paket 3.4 Programmobjekte:** `PMA-PROG-001` bis `PMA-PROG-006`; Import und Export `PMA-IMP-004`, `PMA-IMP-007`, `PMA-IMP-008`, `PMA-IMP-013`, `PMA-IMP-014`, `PMA-EXP-003`, `PMA-EXP-004`.
- **Paket 3.5 Tracking und Designer:** `PMA-META-001` bis `PMA-META-003`, `PMA-REL-005`.
- **Abhängigkeiten:** Phase 2 (Schreib- und DDL-Pfad, Mutationsregeln), Phase 0.
- **Sicherheit:** Destruktive Aktionen (DROP, TRUNCATE, KILL, Konto löschen) zeigen Ziel und Umfang vor Ausführung (Matrixvorgabe).
- **Apple:** Konten und Serverstatus benötigen Serverrechte; Abnahme getrennt. Monitor und Logs (`PMA-ADMIN-006`) gehören zu Phase 4.

### Phase 4: Erweiterte Parität (V4)

- **Paket 4.1 Geodaten:** `PMA-DATA-013`, `PMA-DATA-014`, `PMA-NEU-DATA-004`, `PMA-NEU-DATA-005`; Kartendarstellung ist für Apple offen.
- **Paket 4.2 Schema und Dokumentation:** `PMA-REL-007` bis `PMA-REL-009`, `PMA-REL-011`; Diagramme `PMA-UX-001`, `PMA-UX-002`.
- **Paket 4.3 Transformationen:** `PMA-TRANS-001` bis `PMA-TRANS-011` samt neuer Einzelplugins (`PMA-NEU-TRANS-*`); `PMA-TRANS-010` (externe Programme) nur nach Bedrohungsentscheidung.
- **Paket 4.4 Dump-Optionen:** `PMA-DUMP-001` bis `PMA-DUMP-008`.
- **Paket 4.5 Server erweitert:** Replikation `PMA-ADMIN-011`, `PMA-ADMIN-012`; Binärlog `PMA-ADMIN-010`; Monitor und Advisor `PMA-ADMIN-006`, `PMA-ADMIN-007`; Partitionen `PMA-DDL-012` bis `PMA-DDL-014`.
- **Paket 4.6 Normalisierung und Zentralspalten:** `PMA-META-004` bis `PMA-META-008`; Oberfläche `PMA-UX-005`, `PMA-UX-006`.
- **Abhängigkeiten:** Phasen 2 und 3; für Replikation und Binärlog zusätzlich die Zielversionsentscheidung.

### Phase 5: Nur Web/Go oder Entscheidung (kein Teil der Kern-Parität)

- Setup und Einrichtung `PMA-NEU-SETUP-001` bis `-006`; phpinfo `PMA-NEU-MAINT-001`; Git-Revision `PMA-NEU-MAINT-002`; Versionsprüfung `PMA-NEU-MAINT-003`; Fehlerbericht `PMA-NEU-MAINT-004`.
- Referer-Schutz `PMA-NEU-SEC-004` (Apple entfällt); Simple-2FA `PMA-NEU-SEC-010` (nicht übernehmen).
- Dateiverzeichnisse `PMA-IMP-003`, `PMA-EXP-005` (W/A?) nur nach Entscheidung zu Pfadrechten.

## Getrennt nach Linie

- **Web/Go:** Alle Phasen. Serverbetrieb (Docker Compose und Swarm, PROD-001) und Setup (Phase 5) sind eigene Linie.
- **Apple:** Phasen 1 bis 4 direkt. Ausgenommen sind Funktionen mit "Apple entfällt" (Setup, Konfigurationsdatei, phpinfo, Referer, externe Versionsprüfung). Offen für Apple sind unter anderem Kartendarstellung, Diagramme, Themen, Sprache, Replikation und Binärlog (Serverzugriff) sowie Exporte mit Bibliotheksfragen (PDF, Office, ODS, ODT).
- Die Zahl der Funde je Eignung steht in [README](README.md) (Ergebnis).

## Abhängigkeitsübersicht

```
Phase 0 (Entscheidungen, Konfigurationsspeicher) --> Phase 1 (Lesepfad) --> Phase 2 (Schreiben) --> Phase 3 (Administration) --> Phase 4 (Erweitert)
Phase 0 --> Phase 2.3 (Verlauf, Lesezeichen) und Phase 3.5 (Tracking, Designer)
Phase 5 unabhängig, nur nach Entscheidung
```

## Offene Punkte

1. Zielversionen je Datenbanksystem (Matrix-Grenze 3) fehlen; ohne sie ist keine Abnahme möglich.
2. Lizenzprüfung externer Bibliotheken vor Übernahme: TCPDF (PDF), OpenLayers-Karte (GIS), CodeMirror (Editoren), Diagrammbibliothek, TOTP- und WebAuthn-Bibliotheken. AGENTS.md verlangt, dass Abhängigkeiten ihre Lizenzen behalten und inventarisiert werden.
3. Der Abgleich mit dem tatsächlichen Umsetzungsstand (`STATUS.md`, `ARBEITSPLAN.md`) fehlt im Inventar und muss beim Lead erfolgen.
4. Entscheidungen mit Sicherheits- oder Datenschutzbezug: `PMA-NEU-SEC-001`, `PMA-NEU-SETUP-005`, `PMA-NEU-MAINT-003`, `PMA-NEU-MAINT-004`, `PMA-TRANS-010`.
