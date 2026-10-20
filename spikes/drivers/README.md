# Isolierte Treiberproben

Experimenteller AGPL-3.0-only-Projektcode; keine Produktimplementierung und keine
Produktabhängigkeitsentscheidung. Untersuchungsstand und Primärquellen:
[Übergabe P2a](../../docs/handoffs/P2a.md).

Die verschachtelten Module verändern weder das spätere Go-Hauptmodul noch das Apple-Repo.
`go/go.mod`, `go/go.sum` und `swift/Package.resolved` fixieren den untersuchten Graphen.
Build-Caches gehören nicht ins Repository. iPad-Crossbuilds in einen separaten temporären
Pfad legen, weil die Dokumentationsprüfung nur den Standardcache `.build` ausnimmt.

## Reproduktion

Zuerst laufende Container prüfen (`docker ps`). Die vier Namen unten dürfen noch nicht
existieren. Ports binden ausschließlich an Loopback. Alle Daten sind Wegwerfdaten.
Das sichtbare Passwort `public-spike-fixture` ist ausschließlich eine öffentliche Testfixture.
Keine echten Verbindungsdaten einsetzen.

```sh
docker run --name dbmanager-p2a-maria --label dbmanager.task=p2a -e MARIADB_ALLOW_EMPTY_ROOT_PASSWORD=1 -e MARIADB_DATABASE=probe -p 127.0.0.1:23306:3306 -d mariadb:12.3
docker run --name dbmanager-p2a-postgres --label dbmanager.task=p2a -e POSTGRES_HOST_AUTH_METHOD=trust -e POSTGRES_DB=probe -p 127.0.0.1:25432:5432 -d postgres:18.6-alpine
docker run --name dbmanager-p2a-mysql --label dbmanager.task=p2a -e MYSQL_ALLOW_EMPTY_PASSWORD=yes -e MYSQL_ROOT_HOST=% -e MYSQL_DATABASE=probe -p 127.0.0.1:23307:3306 -d mysql:8.4
docker run --name dbmanager-p2a-no-tls --label dbmanager.task=p2a -e MARIADB_ALLOW_EMPTY_ROOT_PASSWORD=1 -e MARIADB_DATABASE=probe -p 127.0.0.1:23308:3306 -d mariadb:12.3 --skip-ssl
```

Nach Startbereitschaft die ausschließlich taskeigenen Fixture-Konten anlegen:

```sh
docker exec dbmanager-p2a-maria mariadb -e "CREATE USER 'spike'@'%' IDENTIFIED BY 'public-spike-fixture'; GRANT SELECT ON probe.* TO 'spike'@'%';"
docker exec dbmanager-p2a-no-tls mariadb --skip-ssl -e "CREATE USER 'spike'@'%' IDENTIFIED BY 'public-spike-fixture'; GRANT SELECT ON probe.* TO 'spike'@'%';"
docker exec dbmanager-p2a-mysql mysql -e "CREATE USER 'spike'@'%' IDENTIFIED BY 'public-spike-fixture'; GRANT SELECT ON probe.* TO 'spike'@'%';"
```

Aus `go/`:

```sh
SPIKE_MYSQL8_DSN='root@tcp(127.0.0.1:23307)/probe?timeout=3s' SPIKE_MYSQL_DSN='root@tcp(127.0.0.1:23306)/probe?timeout=3s' SPIKE_POSTGRES_DSN='postgres://postgres@127.0.0.1:25432/probe?sslmode=disable' go test -v ./...
go vet ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go test -c -o /tmp/dbmanager-p2a-linux-amd64.test
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go test -c -o /tmp/dbmanager-p2a-linux-arm64.test
```

Ohne DSN werden Netzwerkprüfungen ausdrücklich übersprungen; SQLite wird trotzdem geprüft.
Unverschlüsselte lokale Testverbindungen sind keine TLS-Abnahme.

Aus `swift/`:

```sh
swift run -j 4 DriverSpike
SPIKE_LIVE=1 SPIKE_MYSQL_PORTS=23306 swift run -j 4 DriverSpike
SPIKE_TLS_DOWNGRADE=1 swift run -j 4 DriverSpike
SPIKE_LIVE=1 SPIKE_MYSQL_PORTS=23307 swift run -j 4 DriverSpike
```

Die ersten drei Aufrufe bestanden. Der vierte reproduziert im untersuchten Stand einen
Authentifizierungsfehler gegen MySQL 8.4.11 mit `caching_sha2_password`; Ursache offen.
Der Downgrade-Aufruf bestätigt einen unerwünschten Befund: Default-TLS-Konfiguration
verbindet sich mit einem Server ohne TLS, `Ssl_cipher` bleibt leer. Er ist kein Vorbild
für einen sicheren Produktadapter. Das Fixture benutzt bewusst einen Server ohne TLS.

Die Probe wurde außerdem für den iPad-Simulator gebaut und im bereits laufenden
Simulator (iOS 27.0, iPad Pro 13 Zoll M5) ausgeführt. Aus `swift/` reproduzierbar:

```sh
swift build --triple arm64-apple-ios17.0-simulator --sdk "$(xcrun --sdk iphonesimulator --show-sdk-path)" --scratch-path /tmp/dbmanager-p2a-ipad -j 4
xcrun simctl list devices booted
# Set SPIKE_IPAD_ID to an already booted disposable iPad simulator identifier.
SIMCTL_CHILD_SPIKE_LIVE=1 SIMCTL_CHILD_SPIKE_MYSQL_PORTS=23306 xcrun simctl spawn "$SPIKE_IPAD_ID" /tmp/dbmanager-p2a-ipad/out/Products/Debug-iphonesimulator/DriverSpike
```

Das ist ein Simulatorbefund ohne App-Sandbox-/Signing-/Berechtigungsabnahme und ohne echtes Gerät.

Nach den Prüfungen ausschließlich die vier selbst erstellten Container und ihre anonymen
Volumes entfernen:

```sh
docker rm -f -v dbmanager-p2a-maria dbmanager-p2a-postgres dbmanager-p2a-mysql dbmanager-p2a-no-tls
```

Keine globale Container-/Volume-Bereinigung durchführen. Abhängigkeiten behalten ihre
Lizenzen; Inventar siehe [Lizenzübersicht](LIZENZEN.md).
