# ADR-0007: Nativer MySQL-/MariaDB-Adapter für die Apple-App

Status: **Akzeptiert** durch ausdrückliche Entscheidung des Nutzers am 24.10.2026: eigener Client,
TLS nicht erzwungen (drei ausdrücklich gewählte Modi, Abschnitt 3a), RSA-Schlüsselaustausch auch ohne TLS.
Keine Produktfreigabe der App; Umsetzung nur Paket A4.
Datum: 24.10.2026 (Entwurf 23.10.2026). Betrifft PROD-003 in [PARITÄT](../PARITAET.md). Umsetzung im
Apple-Repo, Paket A4 (Branch `feat/mysql-adapter`, Handoff `docs/handoffs/A4-mysql.md` dort,
unabhängiger Review `A4-review.md` auf `review/a4-mysql`).

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

## 3. Entscheidung: Option (a), klar begrenzt (Regeln 1 und 3 sind durch 3a ersetzt)

Eigener Textprotokoll-Client (`COM_QUERY`) im Core, Dateien `MySQLWire.swift`,
`MySQLConnection.swift`, `MySQLDatabase.swift`. Schnittstelle und Fehlerverhalten folgen
dem PostgreSQL-Adapter (ein Actor, je Operation eine physische Verbindung, keine
Sitzung über Requests, getaggte Zellen, `QueryLimits`).

Sicherheitsregeln:

1. **Standard ist verifiziertes TLS** (nicht mehr einziger Pfad, siehe 3a; TLS ≥ 1.2, `.fullVerification`, Host-
   name als Zertifikatsidentität, optional explizite CA nur für diese Verbindung).
2. **Reihenfolge:** Greeting lesen → fehlt `CLIENT_SSL`, Abbruch mit `mysqlTLSRequired`
   ohne einen einzigen Anmeldebyte → SSLRequest-Paket → TLS-Handshake → erst danach
   Handshake-Response mit Benutzer und Auth-Daten. Es gibt keinen `prefer`-Modus.
3. **Ohne TLS nur über einen ausdrücklich gewählten Modus** (ursprünglich: Loopback-Entwicklungsoption, ersetzt durch 3a)
   (`plaintextLoopbackDevelopmentOnly`), die nur für Loopback-Hosts akzeptiert wird, nie
   mit CA kombiniert und nie Standard ist. Persistenz/UI-Gestaltung dieser Option ist
   Sache der App, hier nicht entschieden.
4. **Authentifizierung:** `mysql_native_password` und `caching_sha2_password`
   (Scramble-Antwort, Auth-Switch zwischen diesen beiden, Fast-Auth-Erfolg). Bei „Full
   Authentication“ gilt: innerhalb einer TLS-Sitzung (verifiziert oder nicht) erwartet der Server das
   Klartextpasswort, es wird durch die TLS-Sitzung gesendet; ohne TLS wird der öffentliche RSA-Schlüssel
   des Servers angefordert und das Passwort (mit der Nonce verknüpft) RSA-OAEP-verschlüsselt gesendet
   (wie die offiziellen MySQL-Clients, Security-Framework, Schlüssel ≥ 2048 Bit). Andere Plugins
   (`sha256_password`, `client_ed25519`, `mysql_clear_password`, Kerberos, PAM) werden abgelehnt.
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

Klarstellung des Nutzers am selben Tag: Wer TLS nicht wählt, hat einen Grund und soll nicht
ausgesperrt werden; dass ohne TLS unverschlüsselt übertragen wird, ist ihm klar. Daher kann
`caching_sha2_password` Full-Auth auch in den Modi (2) und (3) abgeschlossen werden (siehe Regel 4).
Warnungen sind Hinweise, keine Sperren.

**Sicherheitsfolgen** (vom Nutzer so in Kauf genommen, hier ungeschönt):

- Modus (2) TLS ohne Prüfung: Schutz nur gegen passives Mitlesen. Ein aktiver Angreifer kann den
  Server imitieren und erhält Benutzername, Scramble-Antwort und Daten, bei Full-Auth das
  Klartextpasswort (es geht in die ungeprüfte TLS-Sitzung).
- Modus (3) kein TLS: Alles ist mitlesbar und veränderbar (Daten, Ergebnisse, SQL),
  Sitzungsübernahme ist möglich. Der RSA-Schlüssel kommt über denselben ungeschützten Kanal; ein
  aktiver Angreifer kann seinen eigenen Schlüssel unterschieben und erhält das Passwort. Der RSA-
  Austausch schützt nur gegen passive Beobachter.
- **Offline-Ratenangriffe:** In **beiden** Modi (2) und (3) liefert ein Mitschnitt oder aktiver
  Angreifer Scramble-Material, das offline gegen schwache Passwörter geraten werden kann.
- **Plugin-Wechsel:** Weil der Auth-Switch zwischen den beiden unterstützten Plugins in allen Modi
  erlaubt ist, kann ein aktiver Angreifer in den Modi (2) und (3) das schwächere
  `mysql_native_password` (SHA-1-Scramble) erzwingen, was die Offline-Angriffe erleichtert. Es
  entsteht dadurch kein Klartextleck; `mysql_clear_password` und andere Plugins bleiben abgelehnt. In
  Modus (1) ist dies nur durch einen kompromittierten, aber zertifikatsgültigen Server möglich.
- Der Schutz vor dem ursprünglichen MySQLNIO-Befund (stiller Downgrade) bleibt erhalten, weil der
  Wechsel nie automatisch erfolgt: Fehlt `CLIENT_SSL` und ein TLS-Modus ist gewählt, bricht der Client
  vor jedem Authentifizierungsbyte ab.

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
- `caching_sha2_password` ohne TLS nutzt den RSA-Austausch mit unauthentifiziertem Schlüssel (siehe 3a).
- CryptoKit ist Apple-only; für eine Linux-Portierung des Core wäre swift-crypto nötig.

## 6. Entscheidung

**Akzeptiert am 24.10.2026 durch den Nutzer:** eigener Client (Option a), TLS nicht erzwungen, drei
ausdrücklich gewählte Modi pro Verbindung (verifiziertes TLS als Standard, TLS ohne Prüfung, kein TLS),
RSA-Schlüsselaustausch für `caching_sha2_password` ohne TLS, kein stiller Fallback. Offene Folgearbeit
(keine Bedingung der Annahme): unabhängiger Review der Umsetzung (erfolgt; geprüfte Stände `026d7d7`, `4faa2e6`, `227b44d`, `293a949`,
Nachreview offen für spätere Änderungen), Versionsmatrix MariaDB 10.11/11.4/12 und MySQL 8.0/8.4,
UI-Darstellung der Warnstufen (`MySQLSecurityWarning`) und Moduswahl.
