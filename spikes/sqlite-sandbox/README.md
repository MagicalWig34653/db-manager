# SQLite-Dateisandbox: Untersuchung P2b

Experimenteller eigener Code AGPL-3.0-only. Kein Produktadapter, keine Freigabe für
beliebige SQLite-Dateien oder mehrere Service-Replikate. [Übergabe](../../docs/handoffs/P2b.md)
und [ADR-Vorschlag](../../docs/decisions/ADR-0004-sqlite-adapter.md) sind maßgeblich.

## Prüfungen

Im Verzeichnis dieses verschachtelten Go-Moduls:

```sh
go test -v ./...
go test -race ./...
go vet ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go test -c -o /tmp/dbmanager-p2b-linux-amd64.test
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go test -c -o /tmp/dbmanager-p2b-linux-arm64.test
```

Die Tests benutzen ausschließlich automatisch entfernte temporäre Dateien. Ein Kindprozess
wird gezielt während einer offenen Schreibtransaktion beendet und danach aus dem Hot-Journal
wiederhergestellt. Keine fremden Prozesse oder Dateien werden angefasst.

Für einen Linux-Lauf zuerst `docker ps -a` prüfen, der folgende eigene Name muss frei sein.
Hier wird ein vorhandenes **arm64**-Image nur als Prozessumgebung verwendet; PostgreSQL
wird durch den expliziten Entrypoint nicht gestartet. `docker cp` vermeidet Annahmen darüber,
welche Hostpfade Colima einbindet:

```sh
docker create --name dbmanager-p2b-linux --label dbmanager.task=p2b --network none --entrypoint /probe postgres:18.6-alpine -test.v
docker cp /tmp/dbmanager-p2b-linux-arm64.test dbmanager-p2b-linux:/probe
docker start -a dbmanager-p2b-linux
docker inspect dbmanager-p2b-linux --format '{{.State.ExitCode}}'
docker rm -v dbmanager-p2b-linux
```

Alle Tests bestanden auf macOS arm64 und Linux arm64. Linux amd64 wurde erfolgreich
kompiliert; ein lokaler amd64-Emulationsversuch endete mit Exit 139 vor Testausgabe.
Ein nativer amd64-Lauftest bleibt offen.

## Schutzmodell und Grenzen

- ID: 1–64 ASCII-Zeichen, erstes Zeichen alphanumerisch, danach zusätzlich `_`/`-`.
  Das API akzeptiert weder Pfade noch URIs; ID `probe` wird `probe.sqlite`.
- Eigener VFS erlaubt genau diese Datei und ihr Rollback-Journal. `os.Root` verankert
  sämtliche Hostdateioperationen im geöffneten Verzeichnis. `O_NOFOLLOW` lehnt Symlinks
  beim Öffnen ab; Fstat prüft reguläre Datei und genau einen Hardlink. `Access` prüft
  per Lstat. `Delete` entfernt ausschließlich den erlaubten Verzeichniseintrag.
- SQL-Autorisierung erfolgt beim SQLite-Compile durch `SetAuthorizer`, unabhängig von
  Kommentaren, Großschreibung oder Dateiname als Ausdruck. ATTACH/DETACH, externe
  virtuelle Tabellen und gefährliche Funktionen sind gesperrt; PRAGMAs haben eine
  kleine Positivliste für Metadaten und `query_only` des Treibers.
- Temporäre SQLite-Daten bleiben im Speicher; keine WAL-Unterstützung. Vorhandene
  `-wal`-/`-shm`-Artefakte führen zur Ablehnung statt ignorierter Änderungen. Ein Lock
  wird sofort mit `flock(LOCK_EX|LOCK_NB)` für die gesamte Datenbankverbindungsdauer
  erworben. Der Pool besitzt genau eine Verbindung. Ein zweiter Open liefert BUSY.
- Ein einziges Server-Replikat besitzt das Volume exklusiv. Gleichzeitige fremde
  SQLite-Prozesse mit anderen Lockprotokollen sind nicht unterstützt. Der VFS ersetzt
  SQLite-Seitenlocks durch den lebenslangen exklusiven Lock; WAL, gemeinsame Leser,
  Netzdateisysteme und beliebige bestehende Fremddateien sind nicht freigegeben.
- Der lokale Administrator und Prozesse mit Schreibzugriff auf Root sind vertrauenswürdig.
  Gegen SQL-Eingaben und ausgewählte bestehende Linkangriffe bestehen konkrete Tests;
  gegen einen laufenden privilegierten Host-Angreifer wird keine Sandbox behauptet.
- Zell-/SQL-Limit je 1 MiB und SQLite-Heaplimit 32 MiB sind experimentell gesetzt.
  Dateigröße, gesamte Prozessressourcen und vollständige JSON-Antwortgröße sind hier
  nicht begrenzt. Der Produktadapter benötigt zusätzliche API-/Betriebsgrenzen.
- Ohne zusätzliche Fehlerabbildung können diese internen Fehler Pfade enthalten.
  Ein Produktadapter muss ausschließlich feste öffentliche Fehlercodes/-texte ausgeben.

## Lizenzen und festgehaltener Graph

`go.mod`/`go.sum` fixieren die tatsächlich eingebundenen Module. `go list -deps` ergibt:

| Modul | Version | Lizenz / übernommener Originaltext |
| --- | --- | --- |
| ncruces/go-sqlite3 | v0.35.6 | MIT, [Text](licenses/ncruces-go-sqlite3.txt) |
| ncruces/go-sqlite3-wasm/v6 | v6.3.35304 | eigener Generatoranteil MIT-0, [Text](licenses/ncruces-go-sqlite3-wasm.txt); ursprünglicher SQLite-Code behält Public Domain |
| ncruces/julianday | v1.0.0 | MIT, [Text](licenses/ncruces-julianday.txt) |
| golang.org/x/sys | v0.48.0 | BSD-3-Clause, [Text](licenses/golang-x-sys.txt) |

SQLite wird als generierter Go-Code eingebunden; der Name `wasm` bedeutet in dieser
Version keinen separaten WASM-Laufzeitprozess. Das Modul-README erklärt ausdrücklich,
dass Ursprungslizenzen weitergelten. Hier sind keine optionalen FTS/Vec-/CSV-Extensions
importiert. `golang.org/x/text` steht als transitive Modulprüfsumme in `go.sum`, gehört
laut tatsächlichem `go list -deps` nicht zum kompilierten Spike. Weitere optionale
Modulgraphknoten des Upstreams sind kein Beleg für eingebundenen Runtimecode.
Vor einer Produktdistribution bleiben vollständiger Binär-/Drittanteilscan und
unabhängiger Lizenzreview erforderlich; dieser Spike erklärt keine juristische Freigabe.

## Primärquellen

Abruf/Quellprüfung: 20.10.2026.

- [SQLite Authorizer](https://sqlite.org/c3ref/set_authorizer.html): Compile-Hook und Grenzen.
- [SQLite VACUUM](https://sqlite.org/lang_vacuum.html): INTO erzeugt eine neue Datei; URI möglich.
- [Go os.Root](https://pkg.go.dev/os#Root): Dateizugriff relativ zum verankerten Root.
- [ncruces v0.35.6 config.go](https://github.com/ncruces/go-sqlite3/blob/v0.35.6/config.go): `SetAuthorizer`, `Limit`.
- [ncruces v0.35.6 VFS-Vertrag](https://github.com/ncruces/go-sqlite3/blob/v0.35.6/vfs/api.go).
- [Generierter SQLite-Code](https://github.com/ncruces/go-sqlite3-wasm/tree/v6.3.35304): Herkunft und Lizenztrennung.
