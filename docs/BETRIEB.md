# Betrieb mit Docker Compose und Swarm

Stand: Grundlage für einen persönlichen Go-Server; noch keine Teamkonten oder Tresore.
API-Zugriff benötigt ein zufälliges Bearer-Token aus einer Datei. Zugangsdaten für
Zieldatenbanken verbleiben nur während der jeweiligen Anfrage im Prozessspeicher.
Es gibt keine Zugangsdatenpersistenz. Der API-Token gewährt volle SQL-Berechtigung
innerhalb der erreichbaren Datenbankkonten und gehört ausschließlich vertrauenswürdigen
Personen. TLS am vorgeschalteten Proxy ist vor Zugriff über nicht vertrauenswürdige Netze erforderlich.

## Compose

```sh
python3 scripts/create-dev-token.py
docker compose up --build -d
docker compose ps
curl --fail http://127.0.0.1:8080/healthz
```

Das Token liegt in einem Verzeichnis mit Modus 0700, die Datei mit Modus 0444
ist für den unprivilegierten Container lesbar. Compose-Dateisecrets behalten die
Host-Dateirechte; `uid`/`gid`/`mode` werden bei bind-gemounteten Dateien nicht umgesetzt.
Das Token wird weder ausgegeben noch überschrieben; `secrets/` bleibt lokal ignoriert.
Compose bindet standardmäßig ausschließlich an Loopback. `DBM_PORT` ändert den lokalen
Port, `DBM_TOKEN_PATH` die Quelldatei, `DBM_IMAGE` den Imagenamen. Geheimnisse gehören
nicht in `.env` oder Kommandozeilenargumente. Das Volume `data` bleibt bei
`docker compose down` erhalten; `down --volumes` löscht es ausdrücklich.

`DBM_DATA_DIR` bezeichnet das ausschließlich serverseitig gewählte Datenverzeichnis,
im Container `/data`, bei direktem Start standardmäßig `./data`. Die Engine legt es bei
Bedarf an und bricht bei Initialisierungsfehlern ab. SQLite-ID `beispiel` verwendet
`beispiel.sqlite`, keine frei wählbaren Pfade oder DSN-URIs. Das VFS verankert Dateizugriffe
im Root und verweigert Symlinks, Hardlinks und externe SQL-Dateioperationen.

SQLite verlangt **eine Instanz mit exklusivem lokalem Volume**. Jede Anfrage öffnet eine
kurze Verbindung; parallel geöffnete gleiche Datenbank liefert `database_busy`.
Rollback-Journal im DELETE-Modus, kein WAL und keine anderen SQLite-Prozesse auf diesem
Volume. Bestehende WAL-/SHM-Dateien verhindern das Öffnen, statt Änderungen zu ignorieren.
Neue Journal-/Dateianlagen synchronisieren Datei und Verzeichnis; beim Löschen des
Journals wird das Verzeichnis nach SQLite-Vorgabe synchronisiert. Prozessabbruch-Recovery
ist geprüft; dies ist kein Nachweis gegen jeden Stromausfall oder Hardwarefehler.

## Netzwerkziele und DB-TLS

`DBM_ALLOWED_HOSTS` enthält kommagetrennte **exakte** Hostnamen/IPs, keine Wildcards,
URLs oder Unix-Sockets. Standard: `localhost,127.0.0.1,::1`. Im Container bezeichnet
Loopback den Container selbst. Für eine externe DB muss der Betreiber deren Zielnamen
explizit konfigurieren, beispielsweise `db.internal.example` in der Compose-/Swarm-
Umgebung. Der API-Aufrufer kann diese Liste nicht erweitern. Einmalige DNS-Auflösung,
Prüfung auf verbotene Link-Local-/Metadaten-, Multicast- und unspecified-Adressen sowie
gepinntes Dialing vermeiden eine zweite ungeprüfte Namensauflösung. Private Netzadressen
können für ausdrücklich freigegebene DB-Ziele verwendet werden.

Netzwerkverbindungen verwenden standardmäßig TLS mit vollständiger Zertifikatsketten-
und Hostnamenprüfung, ohne Klartextfallback. `tls_mode=disable` ist eine ausdrückliche
Entwicklungsoption; sie benötigt weiterhin ein erlaubtes Ziel und soll nicht für
Produktivzugriffe verwendet werden. TLS am HTTP-Proxy bleibt zusätzlich erforderlich.

`DBM_DB_CA_FILE` kann optional ein administrativ gewähltes PEM-Bundle zusätzlicher
DB-Vertrauenswurzeln laden; Systemwurzeln bleiben bestehen. Diese Wurzeln gelten für
alle erlaubten DB-Ziele, keine Zertifikat-Pinning-Zusage. Ein unlesbares/leeres oder
ungültiges Bundle verhindert den Start. Die API akzeptiert keine CA-Dateipfade.
Für Container muss das Bundle separat lesbar und schreibgeschützt eingebunden werden,
zum Beispiel über einen Compose-Override als Bind-Mount oder als Docker-Swarm-Config.
`DBM_DB_CA_FILE` bezeichnet dann den **Pfad im Container**, etwa `/run/db-ca/ca.pem`.
Ein gesetzter Pfad allein mountet keine Hostdatei. Keine privaten Schlüssel in das
CA-Bundle oder Image kopieren. Backend-HTTPS-Vertrauen wird dadurch nicht verändert.

## SQL, Grenzen und Fehlerfolgen

Pro Anfrage wird genau ein vorbereitetes Statement ausgeführt. SQL-Batches werden vor
Ausführung abgewiesen; `BEGIN`/`COMMIT` über getrennte HTTP-Anfragen bilden keine gemeinsame
Transaktion. Single-Statement-Routine-/Triggerkörper werden vom jeweiligen DB-Parser
beurteilt. Mehrfachergebnisse werden höchstens bis zu 16 Resultsets verarbeitet; eine
vollständige Stored-Procedure-/Warnungs-/`rows_affected`-Abdeckung ist nicht zugesagt.
SQLite-ATTACH, VACUUM und externe virtuelle Tabellen sowie die meisten administrativen
PRAGMAs sind ausdrücklich gesperrt. Es gibt keinen behaupteten SQL-Lesemodus.

Höchstens vier aktive DB-Anfragen insgesamt; das Warten auf einen Platz ist über den
Anfragekontext abbrechbar. Höchstens 1 MiB Rohdaten pro Zelle und 8 MiB tatsächliche
JSON-Antwort einschließlich Base64/JSON-Escaping. Zu große Ergebnisse liefern
`result_too_large` ohne scheinbar vollständige Teilergebnisse. Ein Zeilenlimit liefert
`truncated=true` und bricht das weitere Einlesen aktiv ab; dabei kann der Treiber die
Verbindung schließen. Bereits ausgeführte DB-Effekte können trotz Abbruch oder Fehler
bestehen bleiben; es gibt keine Rollbackzusage. Verbindungen werden nach jeder Anfrage
geschlossen. Große Wire-Pakete können bereits im Treiber gepuffert werden, bevor die
Zellgrenze geprüft wird. Die Ausgabegrenzen und vier Plätze sind kein vollständiger
DoS-/Prozessspeicher-/Festplattenschutz; Betreiber müssen Ressourcen separat begrenzen.

Dezimalzahlen und große Integer bleiben Strings, NULL/Leertext/Leerbinär bleiben getrennt.
Binärwerte sind Base64. Zeitwerte folgen der verlustarmen Treiberdarstellung; PostgreSQL-
TIMESTAMPTZ erhält den Zeitpunkt, nicht eine ursprüngliche nicht gespeicherte Zeitzone.
SQLite-Dynamic-Typing erzeugt keine zusätzliche exakte DECIMAL-Speichersemantik.
Fehlertexte sind feste öffentliche Meldungen ohne rohe DSNs, SQL oder Passwörter.

## Swarm

Der Stack verwendet das von `docker stack deploy` unterstützte Compose-v3-Format,
keine Compose-spezifischen Build-/Startabhängigkeiten. Das Image muss zuvor auf allen
Zielnodes verfügbar sein oder aus einer Registry geladen werden können.
Ein lokales Volume ist kein repliziertes Volume: Die aktuelle Einzelinstanz ist an
Nodes mit Label `dbmanager.data=true` gebunden. Genau einen geeigneten Datennode labeln;
kein Hochverfügbarkeitsversprechen oder horizontale Schreibreplikation.

```sh
docker build -t db-manager:local .
docker node update --label-add dbmanager.data=true <data-node>
docker secret create dbmanager_api_token_v1 secrets/api_token
docker stack deploy --compose-file deploy/swarm.yaml dbmanager
docker stack services dbmanager
```

Swarm veröffentlicht Port 8080 über das Routing Mesh und damit potenziell auf allen
Nodes. Netzwerkzugang begrenzen und TLS-Proxy konfigurieren. Keine Secrets in Logs
oder den Stack einbetten. Rotation über neues versioniertes Secret und
`DBM_TOKEN_SECRET` mit Stack-Update. Healthcheck und geordneter SIGTERM-Shutdown
sind im Image enthalten. Mehrere Replikate erfordern erst gemeinsame Persistenz,
Session-/Jobkoordination und getestete Konsistenzregeln.

## Verifikation und Quellen

Lokal reproduzierbare Prüfungen und tatsächliche Ergebnisse stehen im jeweiligen Handoff.
Ein erfolgreiches `docker compose config` ersetzt keinen tatsächlichen Containerlauf.
Ein Compose-Test ersetzt keinen Swarm-Test. Fremde Docker-Dienste niemals für Tests stoppen.

- [Docker: Stack deploy](https://docs.docker.com/engine/swarm/stack-deploy/)
- [Docker: Secrets](https://docs.docker.com/engine/swarm/secrets/)
- [Docker: Compose Secrets](https://docs.docker.com/reference/compose-file/secrets/)
- [Go: HTTP-Server](https://pkg.go.dev/net/http)
