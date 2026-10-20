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
im Container `/data`, bei direktem Start standardmäßig `./data`. Der API-Client
kann dieses Root nicht ändern. Die folgende Treiberintegration muss den Root bei
Engine-Erzeugung auflösen/anlegen und alle SQLite-Dateien sowie SQL-Dateizugriffe
darunter einsperren; die aktuelle Platzhalter-Engine öffnet noch keine Datenbank.
Übergabepunkt für den Backend-Agent: `config.Config.DataDir` und
`DBM_DATA_DIR`, Engine-Konstruktion in `cmd/db-manager/main.go`; nach Initialisierung
bei Fehlern sicher abbrechen statt auf ein anderes Verzeichnis auszuweichen.

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
