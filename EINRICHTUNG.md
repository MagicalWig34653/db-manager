# Einrichtung und Bootstrap

Vorhanden ist eine Go-API mit Bearer-Token und echten SQLite-/MySQL-/MariaDB-/PostgreSQL-
Adaptern. Der persönliche Server speichert keine DB-Zugangsdaten; jede Anfrage liefert
sie erneut. Eine eingebettete Weboberfläche und vollständige Administrationsparität
sind noch nicht vorhanden. Python 3, Docker/Compose und OpenSSL werden für Liveproben
benötigt; Git/GitHub CLI zusätzlich für Veröffentlichung.

## Go und Container prüfen

Go ab 1.26 gemäß `go.mod`; lokal geprüft mit 1.27.1. Im Repository:

```sh
go test -race -count=1 ./...
go vet ./...
go build -o /tmp/db-manager-check ./cmd/db-manager
```

Der Build allein startet keinen Server. Für Containerprüfungen Docker Engine mit
Compose und die Möglichkeit eines privilegierten isolierten Testcontainers bereitstellen:

```sh
python3 scripts/test-deployment.py
python3 scripts/test-engine.py
python3 scripts/test-engine-container.py
```

`test-deployment.py` baut das Image, prüft Compose und einen eigenen Docker-in-Docker-Swarm
und entfernt seine Ressourcen. Es verändert den Host-Swarm nicht. Für einen
bewusst auf Compose begrenzten Lauf `--skip-swarm` verwenden; dies belegt keinen
Swarmbetrieb. Start, Tokenanlage, Netzwerkgrenzen und Volumeverhalten beschreibt
[Betrieb](docs/BETRIEB.md); tatsächliche Prüfungen und Grenzen stehen im
[P4a-Handoff](docs/handoffs/P4a.md) und [B1-Handoff](docs/handoffs/B1.md).
`test-engine.py` erzeugt eindeutig benannte Wegwerf-DBs und eine eigene Test-CA,
prüft echte Netzwerkabfragen samt TLS-/Fehler-/Größenfällen und entfernt seine Container.
`test-engine-container.py` prüft tatsächliche SQLite-Abfragen, UID, schreibgeschütztes Root
und Neustartpersistenz im eigenen Compose-Projekt; dessen Volume wird danach entfernt.
Einzelne Netzwerk-Go-Tests ohne Fixture-Portvariablen werden ausdrücklich übersprungen.

Direkter Start mit lokalem Datenverzeichnis:

```sh
python3 scripts/create-dev-token.py
DBM_TOKEN_FILE=secrets/api_token go run ./cmd/db-manager
```

Der Server bindet standardmäßig an Loopback; SQLite-ID `beispiel` verwendet intern
`data/beispiel.sqlite`. API-Vertrag und Tokenbedienung siehe
[Verträge](contracts/README.md) und [Betrieb](docs/BETRIEB.md). Externe DB-Ziele benötigen
`DBM_ALLOWED_HOSTS`; eine private CA kann über `DBM_DB_CA_FILE` hinzugefügt werden.
Das CA-Bundle muss im tatsächlich ausführenden Prozess/Container lesbar sein.

Clientvorlagen und historischer Bootstrap folgen
unten; sie bilden keinen aktuellen App-Build ab.

## Dokumentation prüfen

```sh
python3 scripts/check-docs.py
python3 bootstrap/repos/db-manager-web/scripts/check-docs.py
python3 bootstrap/repos/db-manager-apple/scripts/check-docs.py
```

## Repo-Gerüste installieren

`<workspace>` bezeichnet das Verzeichnis mit GoLand, PhpstormProjects und Xcode.
Zuerst nur den Plan anzeigen:

```sh
python3 scripts/bootstrap-repos.py --workspace '<workspace>'
```

Mit `--apply` werden Clientvorlagen installiert und drei Git-Repos initialisiert.
Vorhandene abweichende oder unbekannte Clientdateien werden nicht übernommen. Bestehende
Repos mit Commits und Änderungen werden abgelehnt. Frisch initialisierte Repos ohne Commit
dürfen anschließend mit `--commit` fortgesetzt werden; unbekannte bereits gestagte Dateien
werden abgelehnt. Main-Dateien werden explizit über `bootstrap/manifest.json` ausgewählt,
Clientdateien über die Vorlagen; weitere lokale Dateien werden nicht gestagt. Es werden keine Zugangsdaten angelegt.

```sh
python3 scripts/bootstrap-repos.py --workspace '<workspace>' --apply
```

Vor einem öffentlichen Commit die bestehende Git-Autoridentität und alle Dateien prüfen.
`--commit` übernimmt die bestehende Identität; das Skript ändert keine Git-Konfiguration.
`--publish` legt öffentliche GitHub-Repos an und pusht. Default-Owner: MagicalWig34653.
Die Anmeldung muss zum Owner passen. Existierende Remote-Repos erfordern manuelle Prüfung.

```sh
gh auth status
python3 scripts/bootstrap-repos.py --workspace '<workspace>' --apply --commit --publish
```

Der Bootstrap ist kein Rollback-System: Bei einem Fehler bleiben erfolgreiche Teilschritte
bestehen. Vor erneutem Lauf tatsächliche Dateien, Git-Status und Remote-Repos prüfen.
Nach Installation Clientvorlagen nicht weiterentwickeln: Produktarbeit im jeweiligen Zielrepo.
Status und Arbeitsplan danach mit echten Commit-/Remote-/Prüfergebnissen aktualisieren.

## Verifizierter Installationsstand

Am 20.10.2026 wurde `--apply` im tatsächlichen Workspace erfolgreich ausgeführt:
drei Git-Repos auf `main`, 38 installierte Clientdateien und drei bestandene lokale
Dokumentationsprüfungen. GitHub-Anmeldung als `MagicalWig34653` bestätigt. Alle drei Repos anschließend
öffentlich erstellt und gepusht; erste Dokumentations-CI-Läufe erfolgreich.
Dokumentationsprüfung zusätzlich in drei frischen GitHub-Checkouts bestanden.
Die frühere Schreib-/DNS-Blockade wurde dabei nicht mehr reproduziert.
Aktueller Commit-, Remote- und CI-Stand: [Status](docs/STATUS.md).

Nach separater Weiterentwicklung installierter Clients kann das Bootstrap-Skript
abweichende Zieldateien absichtlich ablehnen. Dann Änderungen im jeweiligen Git-Repo
prüfen und explizit committen; Vorlagen nicht über die weiterentwickelten Clients kopieren.
