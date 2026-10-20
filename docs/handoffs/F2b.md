# F2b – eingebettete Workbench mit echter Go-Engine

Stand: 20.10.2026. Rolle: Frontend & UX Engineer. Branch `feat/web-runtime`,
Basis `21bd46a`. Implementierung zur unabhängigen QA; keine eigene Integrations-
oder Releasefreigabe. Zentralen Status und Arbeitsplan pflegt der Lead.

## Auftrag, Besitz und Ergebnis

Der bestehende Server umschließt seine reale B1-Engine/API jetzt mit `webui.Wrap`.
CLI und Container liefern die deutsche Oberfläche und API von derselben Origin.
Keine Änderung an DB-Engine, Core, HTTP-Vertrag, Go-Modulen oder Webartefakten.
API bleibt 0.1.0; Frontendquelle bleibt exakt
`4e5abee074d34853b266f20c344f28517d7924d1`, Vertrags-SHA256
`4b598dc69a1904685e67a717997412654440ac10f47c17f7a025ccd114e294be`.

Geändert: `cmd/db-manager/main.go`, `Dockerfile`, `.gitignore`,
`scripts/test-deployment.py`, neue `scripts/test-web-integration.py` und
`scripts/web_test_support.py`, eigenständige Testwerkzeuge/Fixtures unter
`tests/web-runtime/`, `.github/workflows/web-runtime.yml` und passende Einrichtung/
Betriebs-/Webbuild-/Lizenzdokumentation. Compose und Swarmkonfigurationen benötigten
keine Änderung: bestehende Token-/Datenvolume-/Nonroot-/Read-only-Regeln gelten weiter.

Das Image baut allein aus dem Hauptcheckout, prüft das eingebettete Assetmanifest
im Go-Paket und enthält kein Node/npm. Originaler Root-AGPL-Text, alle zwölf bestehenden
Go-Lizenztexte, Weblock und Manifest liegen lesbar im Laufzeitimage. Die originalen
Webtexte bleiben eingebettet und per HTTP verfügbar. Datei-SHAs werden im tatsächlich
gebauten Image und ausgeliefertem HTTP verglichen. OCI-Labels nennen öffentliches
Hauptrepository, Betriebsdoku, AGPL und exakten Frontendquellcommit.

## Tatsächliche lokale Prüfungen

Am 20.10.2026 auf macOS/arm64 mit Docker-Linux/arm64, Go1.27.1,
Node26.5.0/npm11.17.0, Chromium über Playwright1.64.1:

- `python3 scripts/test-web-integration.py`: bestanden. Reales CLI gebaut/gestartet;
  Root/Assets/Notices/MIME/Cache/CSP/Auth/404 sowie SQLite-SQL und PostgreSQL mit
  operatorseitiger Test-CA/`verify-full` geprüft. Falscher Zertifikatname wird im CLI
  und Compose verweigert. Danach echtes Image und Compose gestartet, keine Mock-API.
- Zwei echte Chromiumtests: SQLite und PostgreSQL-TLS. Verbindungstest und Katalog,
  Katalogauswahl ohne automatische Query, danach explizite SQL-Ausführung. Exaktes
  Integermaximum, 40-stellige Dezimaldarstellung, NULL, Leertext, Binärwert Base64,
  Zeilenbegrenzung und PostgreSQL-SSL-Sitzung geprüft. Desktop-/iPadbreite,
  Dialogbedienung, Escape-Abbruch und null CSP-Verletzungen; Browserrequests nur
  an dieselbe Origin. Token/Passwort nicht in Local-/SessionStorage oder Cookies.
- Abbruch und Tokenwechsel erfolgen erst bei nachweislich laufender SQL-Arbeit:
  PostgreSQL `pg_stat_activity` meldet die aktive `pg_sleep`-Abfrage; SQLite meldet
  parallel `database_busy`. Nach Browserabbruch/Tokenwechsel wird bis zur tatsächlichen
  DB-Freigabe gewartet. Eine neue SELECT-Abfrage funktioniert. Ein UI-Abort allein
  gilt ausdrücklich nicht als ausreichender Nachweis. Tokenwechsel leert Verbindung,
  SQL, Katalog und Ergebnisse. Fixture-/Beobachtungsanfragen verwenden dieselbe echte API.
- `python3 scripts/test-deployment.py`: bestanden. Reales Image, Compose und eigener
  privilegierter Docker29.5.2-DinD-Testdaemon mit isoliertem Swarm. Root/alle originalen
  fünf Assets/Notices/CSP/API-Auth/Health/MIME geprüft. Compose läuft UID10001 mit
  Read-only-Root; echte SQLite-Fixture überlebt Containerneustart. Swarm erfüllt
  Secretübergabe, Placement, Healthcheck und Routing Mesh; dieselbe SQLite-Fixture
  überlebt erzwungenen Taskersatz. Kein Host-Swarm initialisiert oder geändert.

Zusätzlich bestanden: `go test -race -count=1 ./...`, `go vet ./...`,
`python3 scripts/build-web.py --check`, `python3 -B scripts/test-build-web.py`
(sechs Tests), `python3 scripts/check-docs.py` (174 Dateien, null Fehler) und
`git diff --check`. Netzwerk-Engine-Go-Tests ohne Fixturevariablen werden durch deren
bestehende Tests übersprungen; die oben genannten echten Runtimeproben ersetzen diese
nicht stillschweigend. Der folgende Nachtrag erfasst den frischen Checkout und die öffentliche Linux-CI.

Prüfskripte erzeugen ausschließlich zufällige kurzlebige Credentials unter einem
privaten temporären Verzeichnis. Credentials gehen weder in Prozessargumente noch
Testausgaben; Fehlerausgaben werden zusätzlich redigiert. Browsertraces, Screenshots
und Videos sind ausgeschaltet. Nur eindeutig benannte eigene Prozesse/Container/
Netzwerke/Volumes/Images werden entfernt, keine fremden Dienste. Die PG-Fixtures
verwenden separate dynamische Loopbackports und den Prefix `dbmanager-f2b-web-`;
Deploymentproben `dbmanager-f2b-deploy-`. Alle Ressourcen der genannten Läufe entfernt.

## Grenzen und nächster Schritt

Unabhängige QA und öffentliche CI vor Integration erforderlich. Keine vollständige
phpMyAdmin-Parität, kein HA-/Mehrinstanz-SQLite, kein produktiver Tresor/Sync, keine
öffentliche Deployment-/Distributionsfreigabe. MySQL/MariaDB wurden in B1 separat
geprüft, in diesem Paket keine zusätzlichen Browser-Engineproben dafür. Echte iPads,
Safari und beliebige entfernte TLS-Proxys bleiben offen. iPadbreite im Chromium ist
kein Gerätenachweis. Es bestehen weiterhin keine Rollbackgarantien bei SQL-Abbruch.

Ein prominenter Quellcode-/Lizenzenlink in der Produktoberfläche ist für das nächste
separate Frontendticket vorgemerkt; der freigegebene Build wird nicht nachträglich
manipuliert. Bei öffentlicher AGPL-Netzbereitstellung ist vollständiges passendes
Quellangebot sicherzustellen. Release-SBOM und unveränderliche Image-Digests bleiben
gesonderte Releaseaufgaben. F2b installiert keine SDKs und erweitert den API-Vertrag nicht.

Nächster Schritt: öffentliche Branch-CI und unabhängige QA, danach ausschließlich
Integration durch den Lead. Keine laufenden Entwicklungsserver vorgesehen.


## Abschlusscheckpoint und offene CI-Jobs

Implementierungscommit: `f2c92eef840de1228b4464b71a8068cf86c398ec`, öffentlich auf
`feat/web-runtime`. Bestehende öffentliche Autoridentität unverändert.
Ein frischer Clone dieses Commits wurde ohne Geschwisterrepository gebaut:
`CGO_ENABLED=0 GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local go build -trimpath ./cmd/db-manager`,
wobei `PATH` ausdrücklich weder Node noch npm enthielt. Offline-Assetprüfung und
Dokumentationsprüfung bestanden; Checkout sauber und anschließend entfernt.

Öffentliche GitHub-Actions für denselben Implementierungscommit:

| Workflow | Nachweis | Stand bei Übergabe |
| --- | --- | --- |
| Echter Web-Datenbank-Pfad | [39495523677](https://github.com/MagicalWig34653/db-manager/actions/runs/39495523677) | Erfolgreich auf Linux/amd64: reales CLI/Image/Compose, beide Chromiumtests 3,4 s, SQLite/PostgreSQL-TLS und tatsächliche DB-Abbruchfreigabe |
| Documentation | [39495523691](https://github.com/MagicalWig34653/db-manager/actions/runs/39495523691) | Erfolgreich |
| Backend und Container | [39495523685](https://github.com/MagicalWig34653/db-manager/actions/runs/39495523685) | Noch aktiv; Go-Race/Vet/Build und Doku bereits erfolgreich, Compose/Swarm-Job noch nicht abgeschlossen |
| Datenbankengine | [39495523709](https://github.com/MagicalWig34653/db-manager/actions/runs/39495523709) | Noch aktiv; Ergebnis ausdrücklich offen |

Lead übernimmt die Beobachtung der beiden offenen Jobs sowie die unabhängige QA-
Zuweisung. Dieser reine Dokucheckpoint verändert keinen getesteten Produkt-/Testcode;
`[skip ci]` vermeidet eine zweite identische volle Testserie. Doku- und Diffprüfung
für den Nachtrag bestanden. Keine lokal laufenden Server oder Testjobs, keine eigenen
Docker-Ressourcen verblieben; Host-Swarm weiterhin `inactive`. Fremde Dienste blieben
unverändert. Worker gibt den Slot frei und wartet auf QA-Befunde.

## Unabhängige Abnahme und Leadintegration

QA hat `f2c92ee`/`c9ced80` ohne bestätigte Befunde freigegeben. Eigener sauberer
Archivlauf, Go-Race/Vet/Manifest/sechs Pythonprüfungen, echter Browserpfad und
vollständige Compose-/isolierte-Swarm-Persistenzprobe bestanden. Image-Lizenzen,
Assetbytes/Provenienz, Auth/CSP/Session-/Abbruchfreigabe unabhängig bestätigt.
Alle vier öffentlichen CIs erfolgreich. Keine Ressourcen oder Jobs zurückgelassen.
Lead integrierte als `df0b291`/`c7e1a1d`; integrierte Go-Race/Vet-, Manifest-/
Python- und Dokumentationsprüfungen ebenfalls bestanden. Begrenzter Zwischenstand,
keine vollständige Parität, Tresor-, HA- oder Distributionsfreigabe.
