# Einrichtung und Bootstrap

Vorhanden ist eine Go-API mit Bearer-Token und echten SQLite-/MySQL-/MariaDB-/PostgreSQL-
Adaptern. Der persönliche Server speichert keine DB-Zugangsdaten; jede Anfrage liefert
sie erneut. Die deutsche Weboberfläche ist im Go-Programm eingebettet und unter `/`
erreichbar; vollständige Administrationsparität bleibt offen. Python 3, Docker/Compose und OpenSSL werden für Liveproben
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

`test-deployment.py` baut das Image lokal (unabhängig vom veröffentlichten ghcr-Image, siehe `docs/BETRIEB.md`), prüft Compose und einen eigenen Docker-in-Docker-Swarm
mit originalen Webassets/CSP und echten SQLite-Abfragen samt Neustartpersistenz
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

Echte Browserintegration mit Node26.5.0/npm, Chromium, Docker und OpenSSL:

```sh
python3 scripts/test-web-integration.py
# Linux-CI kann erforderliche Chromium-Systempakete installieren:
python3 scripts/test-web-integration.py --browser-deps
```

Der Test baut das CLI und Image aus diesem Checkout, installiert Browserwerkzeuge
per eigenem Lockfile in einem temporären Verzeichnis und erzeugt SQLite/PostgreSQL-
Fixtures, Test-CA und zufällige Zugangsdaten. PostgreSQL verwendet `verify-full`;
falsche Zertifikatnamen werden verweigert. Der echte Browser prüft Katalog/SQL,
getaggte Werte, Begrenzung, nachweislich laufende Abfragen vor/nach Abbruch,
Token-Kontextwechsel und CSP. Alle eigenen Ressourcen werden danach entfernt.
Es gibt keinen Frontend-Neubau oder Nachbarrepo-Zugriff. Node wird zum Ausführen
dieser Browsertests benötigt, nicht zum Bauen oder Betreiben des Go-Produkts.
Details und Grenzen: [F3b-Handoff](docs/handoffs/F3b.md).

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
Nach dem Start `http://127.0.0.1:8080/` öffnen, unter „Backend-Zugang“ das lokale Token
einfügen und eine Verbindung anlegen. Token und DB-Zugangsdaten bleiben ausschließlich
im Arbeitsspeicher der Seite; ein Tokenwechsel räumt den bisherigen Arbeitskontext ab.

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


## Eingebetteter Tabellen-/Zeilenarbeitsplatz F5b

Objektauswahl öffnet die read-only Tabellenansicht mit Daten und Struktur.
Mehrspaltensortierung und getaggte UND-Filter werden über „Anwenden“ aktiviert;
Tippen erzeugt keine Browse-Anfrage. Nach ungültigem Entwurf bleiben alte angewendete
Filter/Sortierung/Seitengröße erhalten. Seitennavigation enthält kein COUNT-Versprechen.
PK-/Generated-/Default-/Transaktions-/Read-only-Metadaten und fehlende stabile Ordnung
werden angezeigt. Unter „SQL“ bleiben explizite freie Abfragen, Begrenzung und Abbruch
verfügbar; keine Rollbackzusage und kein allgemeiner Schreibschutz der SQL-API.
„Quellcode & Lizenzen“ führt zum exakt gebauten Webcommit und zu lokalen Originaltexten.

`python3 scripts/test-web-integration.py` prüft den tatsächlich eingebetteten Build
mit 13 Chromiumfällen gegen SQLite und PostgreSQL-TLS, einschließlich QA-F3-01-
Pagingregression, SQL/Cancel/Sitzungswechsel und Quelllink-/Lizenzdialog. Das Skript
hält den socketbasierten PostgreSQL-Bootstrap deterministisch an, beweist dessen
fehlende TCP-Bereitschaft und wartet anschließend auf den endgültigen TCP-Server.
Erst nach positivem `verify-full`-Test wird der falsche Hostname geprüft. Keine bloße
Socket-Readiness und kein zufälliges Warten durch Builddauer als Bereitschaftsnachweis.
Compose/isolierter Swarm werden zusätzlich mit `python3 scripts/test-deployment.py`
geprüft. Node/npm sind lediglich für Neubau und Browsertests nötig; Go-CLI/Image und
Betrieb benötigen sie nicht. Aktueller Nachweis: [F5b-Handoff](docs/handoffs/F5b.md).

Für geeignete transaktionale Basistabellen mit PK erscheinen bestätigte Einzelzeilen-
aktionen. INSERT unterscheidet ausgelassenen Default, NULL und Leerwert; UPDATE sendet
den vollständigen Originalsnapshot. PK/generated-Zielspalten bleiben gesperrt. Nach
409 aktuelle Daten laden und eine neue Erwartung bestätigen. Nach Antwortverlust
kann ein Commit bereits erfolgt sein: erst den Datenstand prüfen, nicht blind erneut
senden. Der Client wiederholt Mutationen nicht automatisch. Sitzungswechsel verwirft
Entwürfe; ein versendeter Request kann trotzdem gespeichert sein. Compose/isolierter
Swarm prüfen v4-INSERT und v3-UPDATE/DELETE inklusive Konflikt und typisierter
SQLitepersistenz nach Neustart/Taskersatz. Grenzen und Rootabnahme stehen im Handoff.


F5b ergänzt im Reiter „Struktur“ die katalogbelegte Index-/Constraintansicht samt
Abdeckung/unbekannten Eigenschaften und eindeutig zitierten leeren Bezeichnern.
„Ergebnis herunterladen“ speichert nur den geladenen SQL-Ergebnis-/Seiten-Snapshot als
JSON oder ausdrücklich gewählte CSV-Variante; keine neue DB-Abfrage. JSON ist der
verlustlose Weg für TaggedCells. CSV-Typ-/NULL-/Quoteregeln, Apostrophvariante,
8 MiB-Endbytegrenze und Steuerzeichenablehnung stehen in [WEBBUILD](docs/WEBBUILD.md).
Der F5b-Prüfkandidat verlangt vor Integration eigene unabhängige Root-QA.
