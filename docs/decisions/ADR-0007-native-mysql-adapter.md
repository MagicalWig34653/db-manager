# ADR-0007: Nativer MySQL-/MariaDB-Adapter für die Apple-App

Status: **Vorschlag**. Keine Produktfreigabe; Annahme dieses ADR durch den Nutzer steht aus.
Teilentscheidung des Nutzers vom 24.10.2026 siehe Abschnitt 3a (TLS nicht erzwungen, aber ausdrücklich gewählt).
Datum: 23.10.2026. Betrifft PROD-003 in [PARITÄT](../PARITAET.md). Umsetzung im Apple-Repo,
Paket A4 (Branch `feat/mysql-adapter`, Handoff `docs/handoffs/A4-mysql.md` dort).
Unabhängiger Review und Annahme dieses ADR stehen aus.

## 1. Anforderung und Befund

Die Swift-App soll MariaDB/MySQL ohne Go-Backend nutzen können. Sie gilt bisher als
gesperrt, weil der in P2a untersuchte Treiber MySQLNIO 1.9.2 einen TLS-Downgrade erlaubt
([P2a](../handoffs/P2a.md)).

Belegter Befund, nicht neu bewertet:

- `handleHandshake` in `MySQLConnectionHandler.swift` startet TLS nur, wenn sowohl eine
  `tlsConfiguration` gesetzt ist als auch der Server `CLIENT_SSL` anbietet. Sonst läuft der
  `else`-Zweig mit `isTLS: false` und sendet die Anmeldung im Klartext.
- Ein eigens gestarteter MariaDB-Server mit `--skip-ssl` akzeptierte die Verbindung trotz
  Default-TLS-Konfiguration; `Ssl_cipher` war leer.
- Das ist kein Zertifikatsprüfungsfehler, sondern fehlende Pflicht-TLS. Ein aktiver Angreifer
  (oder eine fehlkonfigurierte Zwischenstelle), der `CLIENT_SSL` aus dem Server-Greeting
  entfernt, bekommt Benutzername, Scramble-Antwort und alle Daten im Klartext.
- Zusätzlich scheiterte `caching_sha2_password` gegen MySQL 8.4 mit „Access denied“ (Ursache
  ungeklärt). Eine native Abbruchstrategie und echte Backpressure fehlen.

Das Protokoll selbst erzwingt TLS nicht: Das Greeting ist unauthentifiziert, die Fähigkeit
`CLIENT_SSL` ist nur ein Angebot. Pflicht-TLS muss deshalb der Client durchsetzen, und zwar
**bevor** Anmeldedaten gesendet werden.

## 2. Optionen

| Option | Vorteile | Nachteile |
| --- | --- | --- |
| (a) Eigener minimaler Client auf NIO + NIOSSL | Pflicht-TLS, Prüfreihenfolge, Limits, Abbruch und Fehlertexte vollständig unter eigener Kontrolle; keine neue Abhängigkeit (NIOPosix/NIOTLS sind Produkte des bereits gepinnten swift-nio, Hashing über CryptoKit); gleiche TLS-Konfiguration wie PostgreSQL | Eigener Protokollcode muss gepflegt werden; Funktionsumfang bewusst klein; Fehler im Protokollparser sind unser Risiko |
| (b) MySQLNIO patchen (Fork oder `patch`) | Weniger neuer Code; Prepared Statements, MySQLData vorhanden | Fork-/Patchpflege über jede Version; Upstream-Handler vermischt Handshake und Auth, der Downgrade-Zweig liegt tief im Zustandsautomaten; Callback-API ohne Backpressure und Abbruch bleibt; `caching_sha2`-Befund ungeklärt; Projektregel „Treiberquellcode nicht eigenmächtig umschreiben“ wie bei PostgresNIO |
| (c) Andere Bibliothek | Keine eigene Protokollpflege | Kein gleichwertiger, gepflegter, MIT/Apache-lizenzierter Swift-Client bekannt; C-Bibliotheken (libmysqlclient GPL, MariaDB Connector/C LGPL) sind mit iOS-Distribution und AGPL/App-Store-Weg nicht ohne gesonderte Rechtsprüfung vertretbar, bringen eigenes OpenSSL und verlieren die gemeinsame NIO-Abbruchlogik |

Eine Upstream-Korrektur in MySQLNIO bleibt wünschenswert, ist aber nicht planbar und
würde die Punkte Backpressure und Abbruch nicht lösen.

## 3. Vorschlag: Option (a), klar begrenzt

Eigener Textprotokoll-Client (`COM_QUERY`) im Core, Dateien `MySQLWire.swift`,
`MySQLConnection.swift`, `MySQLDatabase.swift`. Schnittstelle und Fehlerverhalten folgen
dem PostgreSQL-Adapter (ein Actor, je Operation eine physische Verbindung, keine
Sitzung über Requests, getaggte Zellen, `QueryLimits`).

Sicherheitsregeln:

1. **Standard ist erzwungenes, verifiziertes TLS** (TLS ≥ 1.2, `.fullVerification`, Host-
   name als Zertifikatsidentität, optional explizite CA nur für diese Verbindung).
2. **Reihenfolge:** Greeting lesen → fehlt `CLIENT_SSL`, Abbruch mit `mysqlTLSRequired`
   ohne einen einzigen Anmeldebyte → SSLRequest-Paket → TLS-Handshake → erst danach
   Handshake-Response mit Benutzer und Auth-Daten. Es gibt keinen `prefer`-Modus.
3. **Klartext nur über eine ausdrückliche Entwicklungsoption**
   (`plaintextLoopbackDevelopmentOnly`), die nur für Loopback-Hosts akzeptiert wird, nie
   mit CA kombiniert und nie Standard ist. Persistenz/UI-Gestaltung dieser Option ist
   Sache der App, hier nicht entschieden.
4. **Authentifizierung:** `mysql_native_password` und `caching_sha2_password`
   (Scramble-Antwort, Auth-Switch, Fast-Auth-Erfolg). Bei „Full Authentication“ wird das
   Klartextpasswort nur innerhalb des bereits verifizierten TLS gesendet; ohne TLS keine
   RSA-Schlüsselübernahme, sondern Ablehnung. Andere Plugins (`sha256_password`,
   `client_ed25519`, `mysql_clear_password`, Kerberos, PAM) werden abgelehnt.
5. **Grenzen:** Paketgröße vor dem Puffern geprüft, Sequenznummern erzwungen, Zeilen-,
   Zell-, Ergebnisbytes, höchstens 1024 Spalten, SQL ≤ 1 MiB. Leseseite pausiert nach zwei
   gepufferten Chunks (Backpressure). Kein `LOCAL INFILE`, keine Mehrfachanweisungen, keine
   Kompression, keine Multi-Resultsets.
6. **Werte bleiben exakt:** Das Textprotokoll liefert Dezimal-, BIGINT-UNSIGNED- und
   Mikrosekundenwerte als Zeichenketten; sie werden nie über Double/Date geführt. BLOB
   (Zeichensatz 63) wird Base64-Binärzelle, NULL eigene Art, Null-Datumswerte
   (`0000-00-00`) bleiben Text statt erfundener Zeitpunkte. TIME bleibt Text
   (Bereich ±838:59:59). Das Textprotokoll lässt TIMESTAMP-Werte nicht von der
   Sitzungszeitzone trennen; die Sitzung wird auf `+00:00` gesetzt.
7. **Abbruch/Deadline:** Client-Deadline und Cancellation schließen den Socket und senden
   über eine zweite, gleichermaßen verifizierte Verbindung `KILL QUERY <thread>`; erst
   nach Bestätigung gilt `canceled`, sonst `cancellationUnconfirmed`. Serverseitig
   zusätzlich `max_execution_time` (MySQL) bzw. `max_statement_time` (MariaDB) als
   Rückfall. Kein Rollback-Versprechen.
8. **Lesemodus:** `START TRANSACTION READ ONLY` plus Sitzungsmodus; Schreiben nur mit
   ausdrücklicher Freigabe. Das ist wie bei PostgreSQL keine vollständige Sandbox
   (DDL/Implicit Commit, Serverfunktionen mit Nebenwirkungen sind Sache der Rechte).
9. **Fehlertexte** enthalten nur feste deutsche Texte und den numerischen Serverfehlercode;
   nie SQL, Servermeldung, Hostnamen, Pfade oder Passwörter.

## 3a. Nutzerentscheidung 24.10.2026: TLS nicht erzwungen, aber ausdrücklich

Der Nutzer hat ausdrücklich entschieden: Der native Client erzwingt TLS nicht. Das ersetzt die
Punkte 1 und 3 oben (Pflicht-TLS als einziger Standardpfad, Klartext nur Loopback). Gültig ist:

- Die Transportsicherheit ist eine **ausdrückliche Einstellung pro Verbindung** mit drei Modi:
  (1) TLS verifiziert (Standard), (2) TLS ohne Zertifikat-/Hostnamenprüfung, (3) kein TLS, auch für
  Nicht-Loopback-Hosts. Die Modi (2) und (3) tragen eine ablesbare Warnstufe, die UI muss sie
  anzeigen. Es gibt **keinen** automatischen Wechsel oder stillen Fallback.
- Fehlt `CLIENT_SSL` beim Server und ein TLS-Modus ist gewählt, bleibt der Abbruch vor jedem
  Authentifizierungsbyte.
- Ohne TLS verlässt das Passwort den Client nur als Scramble (`mysql_native_password`,
  `caching_sha2_password` Fast-Auth). Full-Auth von `caching_sha2` (Klartextpasswort) wird ohne TLS
  abgelehnt; ein RSA-Public-Key-Austausch ist nicht umgesetzt, weil er ohne Server-Identität
  keinen Schutz vor einem aktiven Angreifer bietet.

**Sicherheitsfolgen** (vom Nutzer so in Kauf genommen):

- Modus (2): Schutz nur gegen passives Mitlesen. Ein aktiver Angreifer auf dem Pfad kann den Server
  imitieren und liest Benutzername, Scramble-Antwort und Daten; bei Full-Auth zusätzlich das
  Klartextpasswort. Ein Scramble kann zudem für Offline-Ratenangriffe auf schwache Passwörter
  dienen.
- Modus (3): Alles ist mitlesbar und veränderbar (Daten, Ergebnisse, SQL), Sitzungsübernahme ist
  möglich. Angemessen nur in vertrauenswürdigen, isolierten Netzen.
- Der Schutz vor dem ursprünglichen MySQLNIO-Befund (stiller Downgrade) bleibt erhalten, weil der
  Wechsel nie automatisch erfolgt und in den TLS-Modi kein Klartext gesendet wird.

## 4. Nicht Gegenstand

Prepared Statements/Binärprotokoll (geeignet für spätere gebundene Filter, derzeit nicht
nötig), Clientzertifikate, Unix-Sockets, SSH-Tunnel, Kompression, Multi-Statement,
Spaltenkatalog über die vorhandene `DatabaseObject`-Schnittstelle hinaus, MySQL-spezifische
Strukturansichten, Stored-Procedure-Resultsets, UI-Einbindung, Verbindungsprofile. Die
Go-Seite (go-sql-driver/mysql) ist unabhängig davon und von diesem ADR nicht berührt.

## 5. Risiken

- Eigener Parser: Mitigation durch Längen-/Sequenzprüfung vor Pufferung, Unit-Tests mit
  gekürzten Eingaben und Live-Tests; ein Fuzzing- oder externes Audit-Gate steht aus.
- Versionsmatrix: bisher nur je eine MariaDB- und MySQL-Version tatsächlich geprüft.
- `caching_sha2_password` ohne TLS wird bewusst nicht unterstützt (kein RSA-Pfad).
- CryptoKit ist Apple-only; für eine Linux-Portierung des Core wäre swift-crypto nötig.

## 6. Entscheidung

Ausstehend (Annahme durch den Nutzer). Teilentscheidung zu TLS siehe 3a. Akzeptanzkriterien für den Übergang auf „akzeptiert“: unabhängiger Review des
Handshake-Codes (Reihenfolge TLS vor Auth), Wiederholung der Negativtests (unbekannte CA,
falscher Host, Server ohne TLS) auf einer zweiten Umgebung, Versionsmatrix MariaDB 10.11/
11.4/12 und MySQL 8.0/8.4, Entscheidung zur UI-Darstellung der Entwicklungsoption.
