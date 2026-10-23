# B3a – lesende Index- und Constraintmetadaten

Stand 23.10.2026. Branch `feat/schema-metadata`, Worktree
`.worktrees/schema-metadata`, Basis `5cb604a04a524aac0e4cf0ff3e35d1d90c473e74`.
Fortsetzung unter dem ausdrücklich verlängerten Zeitfenster bis heute
23.10.2026 10:00 Europe/Berlin. Unabhängiges Review und native CI bleiben vor
Integration und 0.5-Clientfreigabe erforderlich.

## Umsetzung und Besitz

Additive Core-/Engine-/HTTP-Strukturdateien, enge Routenregistrierung und nur das
zusätzliche lesende SQLite-PRAGMA `index_xinfo`; eigene Regressionen,
`contracts/api-v0.5.0.json`, `contracts/STRUCTURE-API.md`, Vertragswächter,
TLS-Liveskript und `.github/workflows/structure.yml`. Keine Änderungen an
GoMod/GoSum, Main, Docker/Compose, Web, Vault oder den alten Vertragsdateien.
Eigener Code AGPL-3.0-only; keine neue Abhängigkeit und damit keine Änderung am
vorhandenen Lizenzinventar. Lizenztexte der vorhandenen Treiber bleiben gültig.

Umgesetzt sind `/api/v5/capabilities` und `/api/v5/tables/structure` für reale
Basistabellen. Alle vier bestehenden APIs bleiben unverändert. Metadatenwerte
werden gebunden, SQL-Ausdruckstexte nicht ausgegeben. Katalog-/Unknown-Semantik,
SQLite-CHECK-Lücke und die fehlende atomare Schemasnapshotgarantie stehen im
Verhaltensvertrag. Namen werden nicht als Beweis für PK/Unique interpretiert.
Ausgabe bewahrt leere Namen, andersschema-/selbstreferenzierende FK-Ziele,
Ausdrucks-/INCLUDE-/Hilfsterme und tatsächliche Kollationen. Implizite FK-Ziele
werden nicht geraten. Grenzen: zehn Sekunden, bestehende globale Parallelität,
1024 Objekte je Abschnitt, 1024 Terme je Index/Constraintliste, 8192 Gesamtterme,
1 MiB Rohstring und tatsächlich 8 MiB encodierte JSON-Antwort inklusive Newline.

## Tatsächlich ausgeführte Prüfungen

- `go test -race -count=1 ./...` und `go vet ./...` vollständig grün.
- `python3 scripts/test-structure.py`: eigene frische MySQL 8.4,
  MariaDB 12.3 und PostgreSQL 18.6 mit echter TLS-/Hostnamenprüfung; alle drei
  Treiber unter Race grün. SQLite wurde im vollständigen lokalen Racelauf geprüft.
- Reale Fälle: zusammengesetzter PK, Unique, FK-Aktionen, Selbst-/andersschema-FK,
  Sonderzeichen und leere Bezeichner, implizite SQLite-FK-Ziele, CHECK-Abdeckung,
  partielle/Ausdrucksindizes, PostgreSQL INCLUDE/NULLS NOT DISTINCT/NOT VALID und
  deferred FK, MySQL/Maria-Präfix und tatsächliche Richtung/Kollation. MySQL-
  Ausdrucksfelder werden nur bei nachgewiesener Katalogunterstützung verwendet.
- HTTP: Auth, alle fünf Capabilityversionen, echte Strukturantwort, No-Store,
  ungültige/unbekannte Felder, Bodylimit und Abbruch; bestehende Query danach
  unverändert nutzbar. Sandbox erlaubt lesendes gebundenes index_xinfo und
  sperrt weiterhin mutierende/sachfremde PRAGMAs und kombinierte SQL-Ausbrüche.
- Größenregressionen prüfen Objektanzahl und tatsächliches JSON-Escaping;
  eine echte SQLite-Datei mit vier jeweils 400000 Steuerzeichen langen
  Indexnamen wird mit `result_too_large` abgewiesen, obwohl Rohsummen kleiner
  als das Antwortlimit sind. Keine still gekürzten Erfolgsergebnisse.
- Der erste erweiterte PostgreSQL-Test wählte versehentlich den ersten von zwei
  Unique-Constraints. Die Fixtureassertion wurde auf die tatsächlichen Spalten
  eingegrenzt; erneuter vollständiger Race-/Vet-/TLS-Lauf grün. Kein Produktfix
  wurde daraus abgeleitet.
- Vertragswächter: v1–v4 byteidentisch, alte Pfade/Definitionen in v5 unverändert,
  genau zwei additive Routen und alle lokalen Referenzen auflösbar. Keine externe
  JSON-Schema-Validatorabhängigkeit installiert; deren Vollvalidierung ist damit
  nicht behauptet.

## Provenienz, Ressourcen und nächster Schritt

Der unterbrochene Stand wurde zuvor ausschließlich lokal als `a582c45` gesichert;
dieser WIP behauptete ausdrücklich keine Tests/Freigabe. Die vorliegende
Fortsetzung ersetzt diesen unfertigen Befund durch obige reale Prüfungen.
Vertrags-SHA256: `b5225339511d80fe4f52f09118fdad2060d8a92f31d44b73dd89fe9f76178c95`.
Native Linux-amd64-CI wird nach dem Produktcommit auf dessen exaktem SHA gestartet;
lokale macOS-Resultate ersetzen diesen Nachweis nicht. Unveränderte Web-/Deploy-
oder Insert-/Update-Proben wurden für das rein lesende Paket nicht wiederholt.

Liveskript besitzt ausschließlich `dbmanager-b3a-*` Container/Volumes und eigene
kurzlebige CA-Dateien. Es entfernt diese im Finally-Pfad. Fremde Container und der
Host-Swarm werden nicht verändert. Abschlussinventar: keine Container mit Label `dbmanager.task=B3a`, keine
`dbmanager-b3a-*` Container oder Volumes und keine eigenen lokalen Prüfjobs mehr.
Native CI und unabhängiger P0-Review folgen.
Keine Integration, Vertragsfreeze oder Clientfreigabe durch den Autor.

Die exklusiv übertragenen ROOT-STATUS-/Planänderungen zum neuen Zeitfenster wurden
separat als reine Dokumentation `4044341526e9cd1f0cc2484916b4922eb0ce4958`
veröffentlicht. Lead koordiniert Reviews und Integration; Agents übernehmen
Umsetzung, Dokumentation, Prüfungen und freigegebene Integrationsschritte.

## Öffentlicher Kandidat und laufende CI

Produktcommit `77eccb7add990ad65fc6149103c548a40e9b7f5f` ist öffentlich und für
unabhängiges Review eingefroren. Keine weiteren Produktänderungen ohne Finding.
Exakt daran gestartet: Struktur39901687640, Engine39901687425,
Backend39901687412, Tabellen39901687408, Mutationen39901687543,
Insert39901687391, Web39901687396 und Docs39901687478. Bei diesem reinen
Nachweischeckpoint noch aktiv/queued, nicht als bestanden behauptet.
Nächster Schritt: unabhängiger Review und CI-Ergebnisse; erst danach dürfen
Integration und 0.5-Vertragsfreeze durch freigegebene Agents erfolgen.
