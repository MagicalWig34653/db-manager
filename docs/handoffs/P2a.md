# Übergabe P2a: Treiber- und Plattformuntersuchung

Stand: 20.10.2026. Bearbeiter: delegierter Worker; Branch `docs/p2a-treiberplan`.
Dateibesitz: diese Übergabe und `spikes/drivers/**`; zentrale Dokumente unverändert.
Vertragsstand: noch keiner freigegeben. Unabhängiger Review und Integration durch Lead offen.
Keine Migrationen, keine produktiven Daten, kein Tresor.

## Ergebnis und Empfehlung zur Entscheidung

**Go ist für alle vier ersten Datenbankfamilien praktisch verwendbar.** Die isolierten
Proben bestanden gegen MariaDB 12.3.3, MySQL 8.4.11, PostgreSQL 18.6 und SQLite mit
Go 1.27.1 auf macOS arm64. Dieselben Go-Module kompilieren ohne cgo für Linux amd64/arm64;
das passt zur Nutzeranforderung Docker Compose und Swarm. Dies ist kein Nachweis eines
bereits implementierten Containerbetriebs.

**Eigenständiges Swift kann direkt auf SQLite, MariaDB und PostgreSQL zugreifen.**
Swift 6.4/Xcode 27.1 bauen die NIO-Adapter auf macOS; tatsächliche lokale Netzwerkabfragen
gegen MariaDB und PostgreSQL bestanden. Swift MySQLNIO 1.9.2 scheitert dagegen in der
untersuchten MySQL-8.4-Konstellation mit `caching_sha2_password` an Authentifizierung.
Ursache ungeklärt; keine Freigabe daraus ableiten.

**MySQLNIO erzwingt TLS nicht.** Der reale Downgrade-Versuch mit einem eigens gestarteten
MariaDB-Server `--skip-ssl` verband erfolgreich mit unveränderter Default-TLS-Konfiguration.
`SHOW SESSION STATUS LIKE 'Ssl_cipher'` lieferte einen leeren Cipher. Produktadapter dürfen
Credentials bei „TLS erforderlich“ erst nach gesichertem TLS-Handshake senden; eine Prüfung
nach erfolgreicher Anmeldung wäre zu spät. Upstream-Korrektur oder geprüfter eigener
Handshake-Pfad ist vor sicherer nativer MySQL-/MariaDB-Netzwerkfreigabe erforderlich.

Empfohlene erste Implementierungsziele (Vorschlag, kein ADR):

- Go: MySQL 8.4 LTS und MariaDB 11.4 LTS als erste Paritätsziele; aktuelle Zweige zusätzlich
  als CI-Kompatibilitätsmatrix. MariaDB 11.4 ist in dieser Probe noch nicht getestet.
- PostgreSQL 17/18 und SQLite 3 als zusätzliche Engines mit separat benanntem Funktionsumfang;
  tatsächliche Netzprobe bisher PostgreSQL 18.6, keine 17er-Abnahme.
- Apple: zunächst macOS 14+/iPadOS 17+ mit SQLite-Dateiverbindungen; direkte PostgreSQL-
  Verbindung nach TLS-/Abbruch-/Gerätetest. MySQL/MariaDB erst nach TLS-Befundbehebung.
  Mindestversionen sind eine Empfehlung, keine nachgewiesene Altgerätekompatibilität.
- Go-Treiber `database/sql` plus go-sql-driver/mysql, pgx/stdlib und modernc.org/sqlite;
  Engine-spezifische Fähigkeiten hinter getrennten Adaptern. Kein einheitliches SQL-Dialektversprechen.
- Swift: SQLite3 aus dem SDK reicht für den ersten schmalen Adapter. GRDB ist eine MIT-Option
  für sichere Zugriffsschichten, aber im Spike nicht eingebunden. PostgreSQL über PostgresNIO;
  MySQLNIO bedingt geeignet, mit den oben dokumentierten Blockern.

## Nachvollziehbare Befunde

Spikes und vollständige Start-/Test-/Cleanup-Befehle stehen in
[spikes/drivers/README.md](../../spikes/drivers/README.md), Abhängigkeiten in
[LIZENZEN.md](../../spikes/drivers/LIZENZEN.md). Die Lockdateien fixieren konkrete Versionen.
Eigener Projektcode AGPL-3.0-only, Fremdcode behält Original-Lizenz.

| Prüfung am 20.10.2026 | Tatsächliches Ergebnis |
| --- | --- |
| `go test -v ./...` mit vier expliziten Fixture-DSNs | bestanden: SQLite, MySQL, MariaDB, PostgreSQL; UTF-8, SQL NULL, große Integer verlustfrei als Text; kein Float-Zwischenschritt |
| Go-Abbruch langer Abfrage (100 ms Context) | MySQL/MariaDB/PostgreSQL bestanden, unter 2 s; danach Pool wieder ansprechbar; physische Verbindung darf ersetzt werden |
| `go vet ./...` | bestanden |
| `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go test -c` | bestanden; nur Crossbuild, kein Linux-Lauftest |
| `CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go test -c` | bestanden; nur Crossbuild, kein Linux-Lauftest |
| `swift build -j 4` und `swift run -j 4 DriverSpike` | bestanden; beide NIO-Module importiert, echte SQLite-UTF-8-Abfrage |
| `SPIKE_LIVE=1 SPIKE_MYSQL_PORTS=23306 swift run -j 4 DriverSpike` | bestanden: MariaDB UTF-8/UInt64.max, PostgreSQL UTF-8/Int64.max |
| iPadOS-17-Simulator-Crossbuild mit SDK 27.0 und arm64 | bestanden; beide Netzwerkmodule mitkompiliert |
| Lauf im bereits gestarteten iPad-Pro-13-Zoll-(M5)-Simulator, iOS 27.0 | bestanden: SQLite sowie MariaDB/PostgreSQL-Netzwerkprobe; kein echtes iPad |
| `SPIKE_TLS_DOWNGRADE=1 swift run -j 4 DriverSpike` | unerwünschtes Verhalten bestätigt: Server ohne TLS akzeptiert trotz Default-TLS-Konfiguration |
| `SPIKE_LIVE=1 SPIKE_MYSQL_PORTS=23307 swift run -j 4 DriverSpike` | fehlgeschlagen: MySQL 8.4.11, Fixture-Konto `caching_sha2_password`, Access denied; Ursache offen |

Der Downgrade-Codebeleg liegt im fixierten MySQLNIO-Checkout bei
`Sources/MySQLNIO/MySQLConnectionHandler.swift`: `handleHandshake` startet TLS nur, wenn
sowohl `tlsConfiguration` gesetzt als auch `CLIENT_SSL` vorhanden ist; der `else`-Zweig
ruft `writeHandshakeResponse(... isTLS: false)` auf. Fixierter Quellstand:
[MySQLNIO 1.9.2](https://github.com/vapor/mysql-nio/blob/57fec3f24169ed65365def42e516e579a03172ee/Sources/MySQLNIO/MySQLConnectionHandler.swift).

Vier ausschließlich taskeigene Container benutzt: `dbmanager-p2a-maria`,
`dbmanager-p2a-postgres`, `dbmanager-p2a-mysql`, `dbmanager-p2a-no-tls`.
Vorher laufende fremde Container nur gelesen; nie gestoppt/verändert.
Docker-Images können im Cache bleiben. Die Images wurden tatsächlich abgefragt:

| Image/Server | Lokaler Image-Inhalt |
| --- | --- |
| MariaDB 12.3.3 | `sha256:805c8e104bd563d5bfa24fadd3f31cd419ea859cb5277f32b5dbf2db714f9ed1` |
| PostgreSQL 18.6 | `sha256:77f585114c32fbca283dc835b0596f4e52b51b4c6662d7810b2f4084f60a1873` |
| MySQL 8.4.11 | `sha256:0f51e63e537a1f176423fa03ef559aabe6fa4372a53160a44bc7e75da7d15364` |

## Adaptervergleich mit Primärquellen

Abrufdatum aller Quellen: 20.10.2026. Dokumentierte Eigenschaften sind keine ausgeführten Tests.

| Pfad | Lizenz / Fähigkeiten aus Primärquelle | Offene Abnahme |
| --- | --- | --- |
| [go-sql-driver/mysql 1.10.1](https://github.com/go-sql-driver/mysql/tree/v1.10.1) | MPL-2.0; `database/sql`, TLS-Konfiguration, Context-Abbruch; mysql_native_password und caching_sha2_password | eigene CA, Hostname, Clientzertifikat, abgelaufene Zertifikate, Auth-Pluginmatrix; konkrete Version kann weniger Plugins als aktueller Entwicklungsbranch unterstützen |
| [pgx 5.11.0](https://github.com/jackc/pgx/tree/v5.11.0) | MIT; reine Go-Implementierung, eigener Typcodec und `database/sql`, COPY, SCRAM/TLS | SCRAM/channel binding, Zertifikate, Arrays/Numeric/Infinity/JSONB, COPY-Abbruch |
| [modernc/sqlite](https://github.com/modernc-org/sqlite) | BSD-3-Clause mit Drittanteilen; SQLite ohne cgo | Read-only-Dateien, Locking/WAL, Interrupt, inkrementelle große BLOBs, Dateiauswahl im Container |
| [MySQLNIO 1.9.2](https://github.com/vapor/mysql-nio/tree/1.9.2) | MIT; SwiftNIO/NIOSSL, Text-/Prepared-Protokoll, MySQLData, Callback pro Zeile | TLS-Downgrade beheben, caching_sha2-Befund, native Abbruchstrategie, echte Backpressure; Row-Callback allein begrenzt Speicher nicht |
| [PostgresNIO 1.33.1](https://github.com/vapor/postgres-nio/tree/1.33.1) | MIT; AsyncSequence mit Backpressure, Swift primitive Typen, Apple Network.framework-Unterstützung | TLS, Query-Cancellation bis Server, große Datentypen, Speichergrenze; Future-Callback-API hat ausdrücklich keine Backpressure |
| [GRDB](https://github.com/groue/GRDB.swift) | MIT; Swift-SQLite-Schicht, Cursor und Interrupt APIs | nicht eingebunden; Lizenz-/Build-/Datei-/Concurrency-Probe bei Auswahl |
| [Go SSH](https://pkg.go.dev/golang.org/x/crypto/ssh) / [NIOSSH](https://github.com/apple/swift-nio-ssh) | Go BSD-3-Clause; NIOSSH Apache-2.0; beide erlauben TCP-Forwarding | keine SSH-Probe; Hostkey-Verifikation, Fingerprint-Änderung, Keyformat, Agent, Auth-Abbruch und Tunnel-Lebenszyklus erforderlich |

SSH ist eine eigene Transportschicht mit festen Hostkey-Regeln; DB-TLS bleibt unabhängig.
SwiftNIO SSH liefert Bausteine, keinen fertigen produktionsreifen SSH-Client.
Kein SSH-Passwort oder privater Schlüssel wurde angelegt.

## Noch auszuführende Abnahmen

- TLS pro Netzwerkadapter: vertrauenswürdige eigene CA akzeptieren; falsche CA, falscher Host,
  Ablauf und TLS-unfähiger Server vor Credentials ablehnen; optionales mTLS getrennt prüfen.
- Auth: MySQL caching_sha2 Voll-/Fast-Auth, MariaDB mysql_native_password/ed25519 je Ziel,
  PostgreSQL SCRAM, falsches Passwort, Lockout und Serverwechsel; kein stiller unsicherer Fallback.
- Typen: Decimal als exakter Text, unsigned Grenzwerte, NULL/leer, BIT/Binary/BLOB,
  UTF-8/abweichende MySQL-Zeichensätze, Zeit ohne Zone/mit Zone, zero date, UUID,
  JSON/JSONB, Arrays und unbekannte Typen mit verlustfreiem Rohwert.
- Streaming: eine Million synthetische Zeilen, Konsument künstlich bremsen, RSS messen,
  feste Byte-/Zeilenlimits, Verbindung bei Abbruch zuverlässig freigeben. Aktuelle Probe
  liest jeweils eine Zeile; sie beweist keine begrenzte Speichernutzung.
- Abbruch: Query, Handshake, Tunnel und mitten im Resultset; serverseitigen Abbruch separat
  nachweisen. Go-Test belegt Zeitgrenze und Poolverfügbarkeit, keinen `KILL QUERY`-Nachweis.
- Apple: Simulator und echtes iPad separat; Sandbox-Dateizugriff, Dateiauswahl, WLAN/VPN,
  IPv6, Local-Network-Permission, Hintergrund/Suspend/Resume, Keychain und Speichergrenzen.
  Vorhandene Simulatoren sind kein Nachweis eines angeschlossenen echten Geräts.
- Deployment: Containerbuild und Laufprüfungen auf Linux amd64/arm64; Compose Healthchecks,
  Secrets und Datenträger; Swarm-Replikation, Rolling Update, Restart und Secret-Rotation.
  SQLite-Zieldateien sind lokale Dateien des ausführenden Clients/Containers, keine zentral
  unterstellte DB. Swarm-Replikate dürfen SQLite-Dateien nicht unkoordiniert gemeinsam öffnen.

## Fortsetzung

Lead entscheidet DB-/OS-Ziele und Adapter-ADR nach unabhängigem Review. Danach schmalen
Engine-Vertrag umsetzen und Go-Adapter zunächst an synthetischen Wegwerfdaten prüfen.
Apple zuerst SQLite; PostgreSQL nach TLS-Test; MySQL/MariaDB benötigen Fehlerbehebung.
Abschließender Stand: vier eigene Container samt anonymen Volumes entfernt, temporäre Crossbuild-Dateien gelöscht. Keine laufenden eigenen Jobs. Standard-Swift-Buildcache bleibt lokal ignoriert. Dokumentationsprüfung bestanden. Eigene Änderungen sind bis Lead-Integration unintegriert.
