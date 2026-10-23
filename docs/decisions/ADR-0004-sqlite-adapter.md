# ADR-0004: Eingeschränkter SQLite-Adapter mit autorisiertem Dateizugriff

Status: **akzeptiert für die eingeschränkte B1-Implementierung**, keine Produktfreigabe.
Datum: 20.10.2026. Lead-Entscheidung nach unabhängigem QA-Review von P2b und
Zustimmung des Architecture-/Security-Verantwortlichen. Die nachfolgenden Grenzen
sind verpflichtend; eine Produktfreigabe setzt die gesonderten B1-Nachweise voraus.

## Kontext und Befunde

Die Go-API soll IDs auf Dateien unter `DBM_DATA_DIR` abbilden. Eine sichere ID allein
begrenzt SQL nicht: `ATTACH` und `VACUUM INTO` besitzen eigene Dateinamen. Ein SQL-
Präfixfilter ist ungeeignet. Für modernc.org/sqlite 1.60.1 wurde keine öffentliche
Authorizer-API gefunden; dessen erfolgreiche P2a-Abfragen beweisen keine Dateisandbox.

P2b hat ncruces/go-sqlite3 0.35.6 und einen kleinen eigenen VFS praktisch geprüft:
macOS arm64 und Linux arm64 bestanden SQL-Normalfälle, Pfad-/URI-/Symlink-/Hardlink-
Angriffe, ATTACH/VACUUM-INTO-/PRAGMA-/Extension-Fälle, Abbruch, Größenlimit sowie
Recovery nach real getötetem Schreibprozess. Quellen, Code, genaue Befehle und
Grenzen stehen in [P2b](../handoffs/P2b.md) und [Spike](../../spikes/sqlite-sandbox/README.md).
Linux amd64 ist nur kompiliert; lokale Emulation schlug vor Testausgabe fehl.

## Entscheidung

Für den ersten Go-SQLite-Adapter ncruces mit dauerhaft installiertem `SetAuthorizer`
und einem über `os.Root` verankerten eigenen VFS weiterverwenden. Öffnen ausschließlich
per sicherer ASCII-ID, genau eine DB-Datei und ihr Rollback-Journal. Symlinks und
mehrfach verlinkte Dateien ablehnen; externe Datenbank-/Dateioperationen unabhängig
vom SQL-Text über SQLite-Aktionscodes verweigern. Erweiterungen nicht nachladen;
PRAGMAs nur über überprüfte Positivliste anbieten.

Freigabe ausdrücklich auf **ein Replikat, eine Verbindung je aktiver Datenbank und
exklusiven Besitz des lokalen Volumes** beschränken. Hauptdatei für die gesamte
Verbindungsdauer nonblocking exklusiv sperren; zweiter Öffnungsversuch wird als
`database_busy` abgebildet. Rollback-Journal verwenden; WAL, fremde parallele SQLite-
Prozesse, Netzdateisysteme und ungeprüfte Fremddateien nicht als unterstützt ausweisen.
Vorhandene WAL-/SHM-Artefakte ablehnen, statt deren unübernommene Änderungen zu ignorieren.

Der Code bleibt bis unabhängiger Prüfung ein Spike. Der Produktadapter muss Lifecycle,
öffentliche feste Fehlermeldungen, Context-/Connectionfreigabe sowie Zell-/SQL-/Antwort-
und Betriebsgrenzen ergänzen. `Drivers()` darf SQLite erst nach diesen geprüften
Voraussetzungen melden. API-Vertrag 0.1.0 benötigt dafür keine neue Eigenschaft.

## Alternativen

- modernc ohne Authorizer mit „Root-ID genügt“: verworfen als Sicherheitsargument;
  zusätzliche SQLite-Dateioperationen bleiben damit unbeschränkt.
- ncruces mit Standard-OS-VFS und Authorizer allein: einfacher, schützt aber die
  gewählte Hauptdatei nicht ausreichend gegen bestehende Links oder neue Dateipfade.
- Separater SQLite-Workerprozess mit OS-Dateisandbox: weiter mögliche robustere Route,
  verlangt zusätzliche Prozess-/IPC-/Betriebsnachweise. Nicht implementiert.
- SQLite vorerst nicht verfügbar: gültiger Rückfall, falls Reviews oder Betriebsgrenzen
  den eingeschränkten eigenen VFS noch nicht für die Produktintegration freigeben.

## Folgen und verbleibende Grenzen

Ein eingeschränkter früher Funktionsumfang ist offen sichtbar. ATTACH, VACUUM,
externe virtuelle Tabellen und viele administrative PRAGMAs sind gesperrt; das ist
keine vollständige SQLite-Administration. WAL-/Mehrprozessfreigabe verlangt eine
neue Lock-/VFS-Untersuchung. Compose und Swarm müssen eine einzelne Instanz mit
exklusivem beschreibbarem Datenvolume betreiben; ein repliziertes Dateiziel ist keine
automatische SQLite-Hochverfügbarkeit. In Swarm genügt dafür ein node-lokales
Volume (`driver: local`) mit Pinning auf einen Knoten; verteilter Speicher ist weder
nötig noch vorgesehen, und es gibt keine Failover-Zusage (siehe `docs/BETRIEB.md`).

Lokale Administratoren/Prozesse mit Root-Schreibrecht gelten als vertrauenswürdig.
Kein Schutz gegen privilegierte Hostangreifer und keine Zusage zur Stromausfallsicherheit
über das getestete Prozessabbruch-Szenario hinaus. NFS, Disk-full und Crashzeitpunkte
zwischen allen Sync-Schritten sind nicht geprüft. Abhängigkeiten MIT/MIT-0/BSD-3-Clause
und ursprünglicher SQLite-Public-Domain-Anteil sind inventarisiert; endgültige
Distributionsprüfung bleibt erforderlich. Eigener Code bleibt AGPL-3.0-only.

## Zusätzliche Abnahmegates aus dem Review

- Linux amd64 muss tatsächlich nativ laufen; ein erfolgreicher Crossbuild genügt nicht.
- Busy/Abbruch darf nicht in unendlichem Lockwarten enden.
- Journalneuanlage und erforderliche Directory-Syncs im Produktadapter geordnet
  implementieren und prüfen; der Untersuchungsspike allein belegt keine Stromausfalldurabilität.
- Reale JSON-Gesamtantwortgröße, Zellgrenze, Root-/Connection-Lifecycle, öffentliche
  Fehler, Containerbenutzer und Persistenz sind eigene B1-Abnahmekriterien.
