# Betrieb mit Docker Compose und Swarm

Stand: Grundlage für einen persönlichen Go-Server; noch keine Teamkonten oder Tresore.
API-Zugriff benötigt ein zufälliges Bearer-Token aus einer Datei. Zugangsdaten für
Zieldatenbanken verbleiben nur während der jeweiligen Anfrage im Prozessspeicher.
Es gibt keine Zugangsdatenpersistenz. Der API-Token gewährt volle SQL-Berechtigung
innerhalb der erreichbaren Datenbankkonten und gehört ausschließlich vertrauenswürdigen
Personen. TLS am vorgeschalteten Proxy ist vor Zugriff über nicht vertrauenswürdige Netze erforderlich.

## Oberfläche und Lizenzhinweise

Nach dem Start liegt die deutsche Workbench unter `/`, die API bleibt unter `/api/v1/`
und der Healthcheck unter `/healthz`. Im Browser „Backend-Zugang“ öffnen und das lokale
Token eingeben; anschließend Verbindung anlegen, testen und verbinden. Katalogauswahl
öffnet die schreibgeschützte Tabellenansicht mit Daten/Struktur, Seiten, Sortierung und
getaggten UND-Filtern. „Anwenden“ übernimmt gültige Änderungen ausdrücklich; abgewiesene
Entwürfe ändern die angewendete Anfrage nicht. Die Gesamtzahl wird nicht gezählt, fehlende
stabile Reihenfolge ist gekennzeichnet. „SQL“ öffnet den freien Editor; dort wird nur
auf ausdrückliche Aktion ausgeführt. SQL kann weiterhin Daten verändern.
Zugangsdaten bleiben im Seitenarbeitsspeicher; Seitenneuladen verwirft sie. Ein Wechsel
des Backendtokens beendet den alten Verbindungskontext einschließlich laufender Anfragen.
Ein Abbruch nimmt bereits ausgeführte DB-Mutationen nicht zurück.

UI und API benötigen dieselbe Origin. Der Server setzt eine CSP ohne Inline-Scripts
oder `unsafe-eval`; Layout-Stilattribute sind eng erlaubt. Externe Fonts oder Dienste
sind nicht nötig. Ein vorgeschalteter Proxy muss UI, Assets und API gemeinsam vermitteln.
Originale Weblizenzen: `/LICENSE.txt`, `/THIRD-PARTY-NOTICES.txt`. Der Dialog „Quellcode & Lizenzen“ bietet den exakten Webquellstand und die Originaltexte.
Image-Dateien und Quellherkunft: [Webbuild](WEBBUILD.md). Keine öffentliche Releasefreigabe durch dieses Paket.

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
Port, `DBM_TOKEN_PATH` die Quelldatei, `DBM_IMAGE` den Imagenamen
(Standard `ghcr.io/magicalwig34653/db-manager:latest`, siehe Abschnitt Container-Image). Geheimnisse gehören
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

## Installierbare App (PWA)

Die Oberfläche liefert `manifest.webmanifest`, Symbole und `sw.js` aus. Browser bieten die
Installation und den Service Worker nur in einem sicheren Kontext an: **HTTPS** oder
`localhost`/`127.0.0.1`. Hinter einem TLS-Proxy funktioniert die Installation; über Klartext-HTTP
auf einem anderen Hostnamen wird sie nicht angeboten, das ist keine Fehlfunktion des Servers.
Der Service Worker hält ausschließlich die App-Shell (HTML, gehashte Assets, Symbole) für den
Offlinestart vor. `/api`, `/healthz` und `/readyz` werden nie abgefangen oder zwischengespeichert;
Datenbankdaten, Tokens und Zugangsdaten bleiben ohne Offlinekopie. Ohne Verbindung zeigt die Shell
den Hinweis „Keine Verbindung zum Server“. `sw.js` und Manifest werden mit `no-store` ausgeliefert;
eine neue Version wird erst nach Klick auf „Neue Version laden“ aktiv.

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

## Container-Image

Der Workflow `.github/workflows/image.yml` baut das `Dockerfile` für `linux/amd64` und
veröffentlicht es als `ghcr.io/magicalwig34653/db-manager`. Gepusht wird nur bei Push auf
`main` und bei Tags `v*`; Pull Requests bauen ohne Push. Es gibt keine Provenienz-,
SBOM- oder Signaturaussage; `arm64` wird nicht gebaut.

| Tag | Bedeutung |
| --- | --- |
| `sha-<kurz>` | unveränderlicher Quellstand (kurzer Commit-SHA) |
| `latest` | aktueller Stand von `main` |
| `1.2.3`, `1.2` | Versions-Tags bei Git-Tag `v1.2.3` |

OCI-Labels: `source` (Repository) und `licenses` (`AGPL-3.0-only`). Für reproduzierbare
Deployments einen `sha-`- oder Versions-Tag statt `latest` verwenden.

```sh
docker pull ghcr.io/magicalwig34653/db-manager:latest
DBM_IMAGE=ghcr.io/magicalwig34653/db-manager:sha-<kurz> docker compose up -d --no-build
```

Neue ghcr-Pakete sind zunächst privat. Soll das Image ohne Anmeldung abrufbar sein, muss
das Paket nach dem ersten Push einmalig manuell in den Paketeinstellungen auf GitHub
(Package settings, Danger Zone, Change visibility) auf `public` gestellt werden. Bis dahin
ist `docker login ghcr.io` mit einem Token mit `read:packages` nötig. Lokaler Build bleibt
möglich: `docker compose build` (Compose-Datei enthält `build: .`) oder
`docker build -t db-manager:local .` mit `DBM_IMAGE=db-manager:local`.

## Swarm

Der Stack verwendet das von `docker stack deploy` unterstützte Compose-v3-Format,
keine Compose-spezifischen Build-/Startabhängigkeiten. Das Image muss zuvor auf allen
Zielnodes verfügbar sein oder aus einer Registry geladen werden können.
Ein lokales Volume ist kein repliziertes Volume: Die aktuelle Einzelinstanz ist an
Nodes mit Label `dbmanager.data=true` gebunden. Genau einen geeigneten Datennode labeln;
kein Hochverfügbarkeitsversprechen oder horizontale Schreibreplikation.

Speichermodell ([ADR-0004](decisions/ADR-0004-sqlite-adapter.md)): Das Volume `data` ist
explizit ein node-lokales Volume (`driver: local`). Es ist kein verteilter Speicher
(NFS, Gluster, Ceph, Cloud-Volume-Plugin) nötig. Der Dienst läuft mit genau einem Replikat,
per Placement-Constraint auf den einen gelabelten Knoten festgenagelt. Fällt dieser Knoten
aus, startet der Dienst nicht auf einem anderen Knoten mit denselben Daten: keine HA- und
keine Failover-Zusage, keine Replikation von `/data`. Ein Datenverlust des Knotens ist
ein Verlust der SQLite-Dateien; Sicherung liegt beim Betreiber.

Volumenlose Variante (nur Remote-Datenbanken, keine SQLite-Dateien): Die Anwendung startet
auch ohne benanntes Volume (geprüft mit einem einzelnen Container mit `read_only`, tmpfs
`/tmp`, `cap_drop ALL`, Token-Datei: Zustand `running`, Healthcheck `healthy`). Das Image
deklariert `VOLUME /data`, daher legt Docker ohne Eintrag ein anonymes node-lokales Volume an;
dessen Inhalt geht beim Taskersatz verloren und darf nicht für SQLite genutzt werden. In
einer Kopie von `deploy/swarm.yaml` dafür den Eintrag `data:/data` unter `volumes:` des Dienstes,
den Block `volumes:` auf oberster Ebene und den Placement-Constraint entfernen; dann kann
der Dienst auf jedem Knoten laufen. Diese Variante ist nicht als Swarm-Stack getestet und
keine Standardkonfiguration; der Standard bleibt unverändert.

```sh
docker pull ghcr.io/magicalwig34653/db-manager:latest   # oder DBM_IMAGE=<Image> setzen
docker node update --label-add dbmanager.data=true <data-node>
docker secret create dbmanager_api_token_v1 secrets/api_token
docker stack deploy --compose-file deploy/swarm.yaml dbmanager
docker stack services dbmanager
```

Swarm veröffentlicht Port 8080 über das Routing Mesh und damit potenziell auf allen
Nodes. Netzwerkzugang begrenzen und TLS-Proxy konfigurieren. Keine Secrets in Logs
oder den Stack einbetten. Rotation über neues versioniertes Secret und
`DBM_TOKEN_SECRET` mit Stack-Update. Healthcheck und geordneter SIGTERM-Shutdown
sind im Image enthalten. Mehrere Replikate erfordern gemeinsame Persistenz,
Session-/Jobkoordination und getestete Konsistenzregeln; der optionale Serverzustand (Abschnitt
unten) ist dafür erst das Fundament, kein Mehrreplikat-Nachweis.

## Optionaler Serverzustand in externer MariaDB (SZ1)

Grundlage: [ADR-0006](decisions/ADR-0006-serverzustand.md) (Option c akzeptiert). SZ1 liefert
nur das Fundament (Verbindung, Migration, Feldverschlüsselung, Health, fail closed), **keine**
Nutzerfunktion: keine Passkeys, Konten, Teams oder Sessions. Der Standardbetrieb ohne diese
Variablen ist unverändert (Bearer-Token, kein Serverzustand, `/healthz` liefert `{"status":"ok"}`).
Die MariaDB ist extern und wird vom Betreiber betrieben; sie ist kein Bestandteil von
`compose.yaml`/`deploy/swarm.yaml` und kein verteilter Speicher der Anwendung. Sicherung,
Hochverfügbarkeit und Updates der DB liegen beim Betreiber; es gibt kein HA-Versprechen.
Der Pool ist vollständig getrennt von den Verbindungen zu verwalteten Zieldatenbanken
(`DBM_ALLOWED_HOSTS`/`DBM_DB_CA_FILE` gelten dafür nicht und umgekehrt).

| Variable | Bedeutung |
| --- | --- |
| `DBM_STATE_DB_HOST` | Hostname oder IP der MariaDB. Schaltet den Serverzustand ein. Fehlt er, aber andere `DBM_STATE_*` sind gesetzt, bricht der Start ab. |
| `DBM_STATE_DB_PORT` | Standard `3306` |
| `DBM_STATE_DB_NAME` | eigene Datenbank für den Serverzustand (`[A-Za-z0-9_]`) |
| `DBM_STATE_DB_USER` | Laufzeitbenutzer |
| `DBM_STATE_DB_PASSWORD_FILE` | Pfad der Passwortdatei (Docker-Secret) |
| `DBM_STATE_SERVER_KEY_FILE` | Pfad der Serverschlüssel-Datei: 32 Byte als Base64 oder Hex (Docker-Secret) |
| `DBM_STATE_DB_CA_FILE` | optional: zusätzliches PEM-CA-Bundle nur für diese DB (Systemwurzeln bleiben) |
| `DBM_STATE_DB_TLS` | leer/`verify` (Standard: TLS mit Zertifikatskette und Hostnamenprüfung, kein Klartextfallback) oder `disable` (nur lokale Entwicklung) |
| `DBM_STATE_DB_MIGRATE_USER`, `DBM_STATE_DB_MIGRATE_PASSWORD_FILE` | optional, nur gemeinsam: separater Benutzer mit DDL-Rechten. Standard: nur für den Einmalbefehl `state-migrate` setzen, nicht im dauerhaft laufenden Dienst. Nur Ausnahme: Migration beim Serverstart |

Passwörter und Serverschlüssel gibt es nur als Datei, nie als Variable, Argument oder Log.
Fehlermeldungen sind feste Texte ohne Host, Benutzer, Passwort oder Treibertext.
Den Serverschlüssel mit `openssl rand -base64 32` erzeugen und sichern: Ohne ihn sind
verschlüsselte Felder unbrauchbar (Backup/Restore-Verfahren ist eine offene Frage von ADR-0006).

Einrichtung der Datenbank (Beispiel, Namen anpassen, Passwörter nicht in die Shell-Historie):

```sql
CREATE DATABASE dbmanager_state CHARACTER SET utf8mb4;
CREATE USER 'dbmanager_runtime'@'%' IDENTIFIED BY '...' REQUIRE SSL;
GRANT SELECT, INSERT, UPDATE, DELETE ON dbmanager_state.* TO 'dbmanager_runtime'@'%';
CREATE USER 'dbmanager_migrate'@'%' IDENTIFIED BY '...' REQUIRE SSL;
GRANT ALL PRIVILEGES ON dbmanager_state.* TO 'dbmanager_migrate'@'%';
```

Der Laufzeitbenutzer führt nie DDL aus. **Standard:** Migration als Einmalschritt (Init-Container
oder Job) vor dem Rollout, mit Migrationsbenutzer; der dauerhaft laufende Dienst bekommt dann
keine DDL-Zugangsdaten:

```sh
db-manager state-migrate --plan   # zeigt ausstehende Migrationen, ändert nichts
db-manager state-migrate          # wendet sie an (Migrationsbenutzer erforderlich)
```

**Schema 2 (W-P20, Konfigurationsspeicher):** Migration 2 legt `config_owners`, `config_items` und `config_meta` an und prüft danach
Primär- und Eindeutigkeitsschlüssel; eine vorhandene, abweichend definierte Tabelle bricht die Migration ab (`Migration fehlgeschlagen`).
`state-migrate` muss **vor** dem Start der neuen Programmversion laufen: ohne Migration meldet der Dienst `Schema ist veraltet` und
startet nicht. Umgekehrt startet ein Altprogramm (nur Schema 1) nicht gegen Schema 2 (`Schema passt nicht zu dieser Programmversion`);
eine Programm-Rücknahme braucht daher eine Datenbankwiederherstellung. Der Serverschlüssel schützt auch den Index-Schlüssel in
`config_meta` (Hashes von Scope und Schlüsseln); wird er verloren, sind Konfigurationsdaten unbrauchbar. Beim Rotieren bleibt der alte
Schlüssel als zweiter Eintrag erforderlich. Gelöschte Einträge verschwinden in Datenbanksicherungen erst mit der nächsten Sicherung.

Beispiel Compose: `docker compose -f compose.yaml -f compose.state.yaml -f compose.state-migrate.yaml run --rm --no-deps app state-migrate`
(danach den Dienst ohne `compose.state-migrate.yaml` starten). Der Sperrmechanismus (GET_LOCK)
macht auch parallele Läufe sicher: einer migriert, die anderen warten und erkennen den Stand.
**Ausnahme:** Mit gesetzten `DBM_STATE_DB_MIGRATE_*` im Dienst migriert dieser beim Start; dann liegt
das DDL-Passwort während der gesamten Laufzeit im Containerprozess. Nur für einfache Einzelumgebungen.

Ohne Migrationsbenutzer im Dienst migriert der Server nicht; ist das Schema veraltet oder passt es
nicht zur Programmversion, startet er nicht (Meldung ohne Details). Migrationen sind
vorwärts gerichtet, mit Prüfsumme je Version; nachträglich geänderte Migrationen werden erkannt.

Compose (externe DB, Dateien wie oben unter `secrets/`, die Dateien sind per Modus 0444 lesbar):

```sh
DBM_STATE_DB_HOST=db.internal.example \
  docker compose -f compose.yaml -f compose.state.yaml up -d
# Ausnahme (DDL-Passwort im Dienst, Migration beim Start): zusätzlich -f compose.state-migrate.yaml
```

Pfade und Namen lassen sich über `DBM_STATE_DB_PASSWORD_PATH`, `DBM_STATE_SERVER_KEY_PATH`,
`DBM_STATE_DB_CA_PATH`, `DBM_STATE_DB_NAME`, `DBM_STATE_DB_USER` (und `..._MIGRATE_...`)
ändern. Das Compose-Netz muss die DB erreichen (gleiches Netz oder erreichbarer Hostname).
Swarm: `deploy/swarm.state.yaml` (und `swarm.state-migrate.yaml`) nach `swarm.yaml` mergen;
Secrets vorher als externe Swarm-Secrets `dbmanager_state_db_password_v1`,
`dbmanager_state_server_key_v1`, `dbmanager_state_db_ca_v1` (Migration:
`dbmanager_state_db_migrate_password_v1`) anlegen und die DB im Overlay erreichbar machen.
`swarm.yaml` bleibt auf ein Replikat mit lokalem Volume festgelegt (ADR-0004); das
Beispiel ändert das nicht und ist nur als gerendertes Stack-Dokument geprüft
(`docker stack config`), nicht als Swarm-Deployment.

Verhalten bei Betrieb und Ausfall:

- Start: DB, TLS und Schema müssen innerhalb von 30 Sekunden stimmen, sonst Abbruch
  (Zertifikats- und Anmeldefehler sofort). Der Server loggt nur feste Statuszeilen.
- `/healthz` ist reine **Liveness** ohne DB-Abhängigkeit und liefert immer `{"status":"ok"}`;
  die Docker-/Swarm-Healthchecks nutzen sie unverändert. Damit ersetzt ein Orchestrator bei einer
  DB-Störung keine Tasks (keine Austauschschleife durch `unhealthy`).
- `/readyz` ist die **Readiness**: ohne Serverzustand `{"status":"ok","state_store":"disabled"}`, mit
  Serverzustand `state_store` `ok` (200) oder `unreachable` (503). Sie liest nur den zwischengespeicherten
  Zustand (Hintergrundprüfung alle 3 Sekunden, ein Ping je Intervall) und löst keine DB-Abfrage pro Aufruf aus.
  Für Load Balancer/Monitoring gedacht, nicht für Docker-Healthchecks. Beide Routen sind
  unauthentifiziert und verraten damit den Zustand des Speichers.
- Fail closed gilt pro Route: `API.RequireState` liefert 503 `state_unavailable`, solange der Speicher
  nicht erreichbar ist. Es gibt in SZ1 noch keine Route, die Serverzustand liest, daher ist **keine
  Route gesperrt**; künftige Konten-/Session-/Freigabe-Routen müssen damit umhüllt werden. Der
  Bearer-Token-Modus läuft bei DB-Ausfall unverändert weiter. Kehrt die DB zurück, erholt sich die
  Readiness ohne Neustart. Ein vom Aufrufer abgebrochener Check ändert den Zustand nicht.

Bekannte Grenzen (SZ1-Review, nicht behoben): `DBM_STATE_DB_CA_FILE` ergänzt die Systemwurzeln statt sie
zu ersetzen (kein Pinning); Dateirechte der Secret-Dateien werden nicht geprüft; Serverschlüssel werden nur
gegen Länge und Nullschlüssel geprüft, nicht auf Entropie; ein kurzer Pingfehler setzt die Readiness sofort
auf `unreachable` (keine Hysterese).

Prüfung: `python3 scripts/test-server-state.py` (nur eigene Wegwerf-MariaDB-Container mit
TLS, ein eigenes Docker-Netz und ein Testimage; räumt alles ab). Ergebnisse und Grenzen:
[SZ1](handoffs/SZ1.md).

## Verifikation und Quellen

Lokal reproduzierbare Prüfungen und tatsächliche Ergebnisse stehen im jeweiligen Handoff.
Ein erfolgreiches `docker compose config` ersetzt keinen tatsächlichen Containerlauf.
Ein Compose-Test ersetzt keinen Swarm-Test. Fremde Docker-Dienste niemals für Tests stoppen.

- [Docker: Stack deploy](https://docs.docker.com/engine/swarm/stack-deploy/)
- [Docker: Secrets](https://docs.docker.com/engine/swarm/secrets/)
- [Docker: Compose Secrets](https://docs.docker.com/reference/compose-file/secrets/)
- [Go: HTTP-Server](https://pkg.go.dev/net/http)

## Bestätigte Einzelzeilenaktionen

F5b-Prüfkandidat: Webclient und API1–5 aus einer Origin, keine Node-/npm-Laufzeit.
Geeignete Tabellen bieten explizit bestätigte Einzelzeilenaktionen; keine Massen-
änderungen oder Retrywarteschlange. Nach Konflikt Daten neu laden; nach unterbrochener
Schreibantwort möglichen Commit zuerst prüfen. Abbruch garantiert kein Rollback.
Tatsächliche Nachherwerte können durch DB-Konversionen von Eingaben abweichen.
Die F4b-Deploymentprobe führt reale strukturierte INSERT-/UPDATE-/DELETE-Anfragen aus
und prüft Default/NULL/Präzision/Binärwerte sowie stale-Snapshot-Konflikte. Eine erhaltene
Zeile wird nach Compose-Neustart und isoliertem Swarm-Taskersatz erneut gelesen.
Dies belegt keine HA-, Powerloss-, Exactly-once- oder öffentliche Releasegarantie.
[Prüfstand und unabhängiges Gate](handoffs/F5b.md).


Die F5b-Deploymentprobe prüft zusätzlich die authentifizierte v5-Fähigkeit und echte
SQLite-Index-/PK-/Abdeckungsmetadaten nach Neustart bzw. Taskersatz. JSON-/CSV-Download
bleibt eine lokale explizite Browserfunktion ohne Serverdatei/zusätzliche DB-Abfrage;
ein Download wird dadurch nicht automatisch zu einem Backup oder vollständigen Dump.
