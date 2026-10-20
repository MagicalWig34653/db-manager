# Übergabe P2b: SQLite-Dateisandbox

Stand: 20.10.2026. Rolle: Backend & Platform Engineer. Branch
`spikes/p2b-sqlite-sandbox`, eigener Worktree `.worktrees/p2b`, Ausgangsstand `15e2573`.
Dateibesitz ausschließlich `spikes/sqlite-sandbox/**`, diese Übergabe und
[ADR-0004-Vorschlag](../decisions/ADR-0004-sqlite-adapter.md). Keine P4a-/Produktdatei geändert.
Unabhängiger Review und Entscheidung sind offen; noch kein Produktadapter ausgewählt.

## Ergebnis

ncruces/go-sqlite3 **0.35.6** bietet im Gegensatz zur untersuchten öffentlichen
modernc-API einen Authorizer-Hook und einen austauschbaren VFS. Im Spike greifen zwei
Schichten: SQLite-Aktionscodes verweigern unerlaubte SQL-Operationen; ein eigener VFS
verankert Hostdateioperationen per `os.Root` und erlaubt nur die gewählte Hauptdatei
samt Rollback-Journal. Keine SQL-Präfixfilterung.

Reale Tests bestanden auf macOS arm64 und Linux arm64. Ein sicherer erster Adapter
ist unter ausdrücklich eingeschränktem Betrieb plausibel: einzelnes Service-Replikat,
eine Verbindung, exklusives lokales Volume, kein WAL und keine gleichzeitigen fremden
SQLite-Prozesse. Architektur/Sicherheit hat diesen Untersuchungsmodus grundsätzlich
befürwortet; das ist noch kein Review des fertigen Codes oder akzeptierter ADR.

Der Spike und Original-Lizenztexte liegen unter
[spikes/sqlite-sandbox](../../spikes/sqlite-sandbox/README.md). `go.mod`/`go.sum` pinnen
Go 1.26 als Mindeststand, ncruces 0.35.6, generierten SQLite-Code 6.3.35304,
julianday 1.0.0 und x/sys 0.48.0. Tatsächlich benutzt wurde Go 1.27.1.
Der aktuelle `wasm`-Modulname bezeichnet generierten Go-Code, keinen automatisch
abgesicherten separaten WASM-Prozess. Hostdateioperationen benötigen weiterhin den VFS.

## Tatsächliche Prüfungen

Aus `spikes/sqlite-sandbox` am 20.10.2026:

| Prüfung | Ergebnis |
| --- | --- |
| `go test -v ./...` | bestanden: legitime CREATE/INSERT/SELECT/Index/View/Schema, BEGIN/COMMIT/ROLLBACK und Wiederöffnen |
| Dateibezogenes SQL | ATTACH normal/mit Kommentar/mit Ausdruck/URI sowie VACUUM INTO abgewiesen; kein außerhalb liegendes Ziel erzeugt |
| PRAGMA/Extensions | temp/data_store_directory, WAL, writable_schema, temp_store=FILE, File-/Extensionfunktionen, CSV-VTable und indirekte Filefunktion abgewiesen beziehungsweise nicht eingebunden |
| ID-/VFS-Negativfälle | Pfade, absolute Namen, URI, fremde DB-Datei, Slash/Backslash, zu lange/nicht-ASCII-ID abgewiesen; Open/Access/Delete/FullPathname separat geprüft |
| Links und WAL-Artefakte | bestehende Symlinks an Hauptdatei/Journal und Hardlink-Hauptdatei abgewiesen; externe Sentineldatei unverändert; vorhandenes WAL/SHM verhindert Open |
| Deadline | endlose rekursive Abfrage nach 30 ms unterbrochen; Context tatsächlich DeadlineExceeded und `errors.Is(err, sqlite.INTERRUPT)`; unter 2 Sekunden |
| Größenlimit | 1 MiB überschreitender BLOB abgewiesen; Verbindung danach weiter nutzbar |
| Exklusiver Lock | zweiter Handle liefert unmittelbar SQLITE_BUSY; kein blockierendes flock |
| Prozessabbruch/Recovery | eigener Testkindprozess in offener 1000-Zeilen-UPDATE-Transaktion getötet, Hot-Journal vorhanden, Wiederöffnung erhält alle ursprünglichen Null-BLOBs vollständig |
| `go test -race ./...`, `go vet ./...` | bestanden |
| Linux-Crossbuild, `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go test -c` | bestanden |
| Linux-Crossbuild, `CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go test -c` | bestanden |
| Linux arm64 im isolierten Container ohne Netz | alle Tests bestanden, Exit 0 einschließlich Prozessabbruch/Recovery |
| Linux amd64 lokale Emulation | Exit 139 vor Testausgabe; kein bestandener amd64-Lauf behauptet |

Die Linux-Befehle samt exaktem Entrypoint stehen im Spike-README. Verwendet wurde das
vorhandene arm64-Image `postgres:18.6-alpine` ausschließlich als Prozessumgebung, ohne
DB-Dienst. Der eigene Container `dbmanager-p2b-linux` und sein anonymes Volume wurden
entfernt. Fremde Container nur vorab gelesen. Ein erster Host-Bindmountversuch scheiterte
an Colimas nicht identischem `/tmp`; der erfolgreiche Lauf verwendet `docker cp`.

## Grenzen, Review und nächste Schritte

- Kein Produktionsschutz gegen privilegierte Hostangreifer oder bösartige lokale Prozesse
  mit Root-Schreibrecht. `os.Root`/O_NOFOLLOW sind tatsächlich implementiert, ein reiner
  String-Präfixvergleich ist keine Schutzgrundlage. Bestehende Links werden abgewiesen.
- Das experimentelle VFS ist absichtlich auf exklusives Volume/Single-Connection und
  Rollback-Journal begrenzt. Keine volle Standard-VFS-/Mehrprozess-/WAL-Interoperabilität.
  Ein nicht kooperierender anderer SQLite-Prozess kann anderes Lockprotokoll verwenden.
- Prozessabbruch-Recovery ist geprüft; Stromausfall, NFS, Disk-full und sämtliche
  Crashpunkte zwischen Sync-Schritten bleiben offen. Dateigröße und Prozessressourcen
  sind nicht vollständig begrenzt. Kein vollständiger Datenbankintegritätsbeweis.
- Produktadapter muss feste öffentliche Fehlertexte liefern, DB/Root/Registrierungen
  zuverlässig schließen und 1-MiB-Zell-/8-MiB-**encodierte JSON**-Antwortgrenze beachten.
  Base64 und JSON-Escaping gehören zum tatsächlichen Antwortumfang.
- Architektur-/QA-Review von Code, Einschränkungen und Original-Lizenzinventar erforderlich.
  SQLite erst danach unter diesen Betriebsbedingungen als Fähigkeit melden; andernfalls
  vorerst aus `Drivers()` auslassen. Modernc-Dateisandboxversprechen ausdrücklich verwerfen.
- Anschließend B1 auf freigegebenem P4a-Vertrag in neuem Paket: kurze Verbindung pro
  HTTP-Request, keine über Requests vorgetäuschten Transaktionen; große Integer/Decimal,
  NULL/Leerstring/Binary/Zeit verlustfrei behandeln; Container UID 10001/readonly-root mit
  beschreibbarem `/data` real prüfen. Netzwerkadapter prüfen TLS unabhängig.

Keine produktiven Daten oder Migrationen; alle Fixturedateien automatisch entfernt.
Eigene temporäre Crossbuild-Binärdateien werden vor Übergabe gelöscht. Keine laufenden
eigenen Prozesse oder Container. Eigenarbeit bleibt bis Lead-Integration unintegriert.
