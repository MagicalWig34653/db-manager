# Arbeitsplan

Stand: 23.10.2026. Vom Nutzer ausdrücklich verlängert bis vollständig umgesetzt oder
heute 23.10.2026 10:00 Europe/Berlin (=08:00 UTC). Lead koordiniert Reviews und Integration;
Agents übernehmen Umsetzung, Dokumentation, Prüfungen und freigegebene Integrationsschritte. Die beiden zentralen Hauptrepo-
Status-/Planungsdateien sind für dieses Fenster exklusiv Backend & Platform übertragen.
Tatsächliche Ergebnisse und feste Vertragspins: [STATUS](STATUS.md).

## Aktive Pakete und Dateibesitz

| ID / Rolle | Repo / Branch / Worktree | Ziel und Abnahme |
| --- | --- | --- |
| F4b / Frontend & UX | Hauptrepo / feat/row-web-runtime / .worktrees/row-web-runtime, Basis4044341 | F4-Quelle3a4a3fd nach unabhängigem Review01933d5 im Web-main bf75748 integriert. Jetzt separate reproduzierbare Rootasset-Einbettung; Besitz Root-Webassets/Manifest/Lock, Build-/Runtime-/Deploytests/enge CI, README/Einrichtung/Betrieb/Webbuild/Lizenzen/Handoff. Kein DB/Core/HTTP/GoMod/API/STATUS/Plan. Produkt3770c4e plus Handoff46e4bf1 und unabhängiger Review533348a ohne Findings angenommen. Zwei bytegleiche öffentliche Builds, Embeddedbrowser9/9, Compose-/isolierte-Swarm-CRUD/409/Persistenz und sauberer öffentlicher Go-/Imagecheckout ohne Node/npm/Siblings unabhängig grün; alle sieben Produkt-CIs success. Freigegebene Agentintegration und Main-Gesamtcheck folgen. |
| S2a / Apple & Security | Apple / feat/vault-core / .worktrees/vault-core, Basis7cbeb38 | Eigenständiger Core im neuen Packages/DBManagerVault, eigene Tests/Fixtures/Originaltexte/CI/Skripte, ADR-A003/Sicherheitsdokumente/Handoff. S1-Format exakt, Actor-Locksession, verwaltetes privates Root mit festen Zufallsnamen und sicheren Dateideskriptoren, flock/CAS/temp0600/fsync/rename/dirfsync. Unabhängiger Anchor reserviert lebenslange Root-AEAD-Nutzung dauerhaft vor Verschlüsselung, bestätigt Hash/Generation erst nach dauerhafter Datei; Pending nur bekannte alte/neue Zustände. Fehlender/zurückrollbarer importierter Anchor erlaubt keine schreibende Produktsession. Nur Testanker in Testtarget, kein stiller InMemory-Produktfallback. Recovery darf mit einem überlebenden alten Faktor in neuen Tresor mit beiden neuen rückgelesenen Faktoren migrieren; Original erhalten. Keine App-/Xcode-/Rootpackage-/A2-/zentralen Statusänderungen. Unabhängige Dateisystem-/Fehler-/Concurrency-/Bounds-/Interop-QA vor Integration. |
| B3a / Backend & Platform | Hauptrepo / feat/schema-metadata / .worktrees/schema-metadata, Basis5cb604a | Öffentlicher Prüfkandidat77eccb7 eingefroren: lesende additive0.5-Strukturmetadaten, keine DDL/Ausdruckstexte; unknown/CHECK-Lücke und Bounds explizit. Code/Vertrag/Tests/Handoff fertig; volle Go-Race/Vet, SQLite+echte3TLS-DBs, Auth/Negativ-/Größen-/Authorizerfälle und Docs270/0 bestanden. Alle acht exakten CIs success; unabhängiger Architecture-/Securitypeerreview und separate0.5-Freigabe folgen. Besitz DB/neueCoreHTTP/engeRouteAuthorizer/eigeneVerträgeCIHandoff, zusätzlich exklusiv RootSTATUSPlan; kein Index/GoMod/Main/Web/Docker. |

A2 ist nach unabhängiger Kalenderfixprüfung und exakter grüner nativer
CI39503396485 an b72e62c im Apple-main7e97aae integriert. Bestehender eigener Worktree
Apple/feat/apple-postgres/.worktrees/apple-postgres bleibt Provenienz. NIOssl-Pin
ist begründete eingeschränkte Abhängigkeitsentscheidung mit dokumentiertem Risiko.
Signing, echte Geräte und OS-UI bleiben offen.

## Review- und Integrationsqueue

1. F3b ist unabhängig abgenommen und im Hauptrepo integriert. Neue kombinierte
   Haupt-CIs nach Veröffentlichung kontrollieren; lokale Integrationschecks bestanden.
2. A2 ist integriert, ADR-A002 und zentrale Apple-Dokumente aktualisiert; lokale
   Integrationsprüfung bestanden. Neue kombinierte Apple-Main-CI kontrollieren.
3. F4 ist unabhängig angenommen und im Webrepo integriert. F4b wurde separat
   durch Backend & Platform ohne Findings angenommen, alle sieben CIs success.
   Produkt3770c4e/Handoff46e4bf1/Review533348a durch freigegebenen Agent integrieren,
   passenden Gesamtcheck und MainCI starten; keine B3a-/S2a-Mitnahme.
4. S2a unabhängig prüfen. Reale Keychain-/signierte Anchorintegration und Appablauf
   sind ein separates S2b-Gate; weder Testspeicher noch Build ist Produktpersistenz.
5. B3a-Prüfkandidat77eccb7 und exakte acht CIs unabhängig prüfen; 0.5-Vertrag erst
   danach für eine Strukturansicht freigeben. Weitere Pakete
   aus der Paritätsmatrix anschließend zuschneiden: Schemafunktionen, Import/Export,
   Verlauf/Favoriten und sichere native Netzwerkadapter. Jede Freigabe separat.

QA & Release p0_publication_review und qualifizierte nicht beteiligte Peers übernehmen
unabhängige Reviews. Bei vier Laufzeitplätzen einschließlich Lead wechseln Worker und
Reviewer geordnet. Nachträglich erhöhte lokale Konfiguration wird hier nicht neu geladen;
kein Neustart oder Umgehungsprozess. Kein paralleles Schreiben derselben Dateien.
Hauptrepo-STATUS/ARBEITSPLAN liegen für dieses Fenster exklusiv bei Backend & Platform,
Apple-STATUS/ARBEITSPLAN exklusiv bei Architecture & Security. Übrige zentrale Dateien
benötigen eine gezielte Zuweisung.

## Verbindliche Grenzen und Übergabe

Eigener Code AGPL-3.0-only, Originaltexte/Abhängigkeitsinventare erhalten. Deutsch für
UI/Dokumentation, Englisch für Code/Tests/Kommentare/technische Bezeichner.
Keine echten Secrets, privaten Pfade oder produktiven SQL-Verläufe veröffentlichen;
vorhandene Git-Autoridentität erhalten. Gemeinsame Verträge vor Clientbindung abnehmen.
Compose und Swarm verpflichtend; SQLite nach ADR0004 eingeschränkt. Native MySQLNIO-
Adapter wegen realem TLS-Downgrade gesperrt. Kein HA-/Powerloss-/Exactly-onceversprechen.

Nur eigene kurzlebige Testcontainer/-volumes/-netze/-simulatoren verwenden und bereinigen;
keine fremden Ressourcen, Host-Swarm oder Systemrechte verändern. Keine produktiven
Migrationen/Dienste. Laufende Jobs vor erneutem Start prüfen. Nicht ausgeführte Prüfungen
explizit offenlassen. Vor Zeitgrenze alle Worker checkpointen: Dateien/Branch/Commits,
Tests/Befunde, unintegrierte Arbeit, Jobs/Ressourcen und nächster konkreter Schritt im
Handoff; zentrale Gesamtübergabe durch Lead. Alte saubere Worktrees bleiben Provenienz.

Abgeschlossene Pakete samt historischen Befunden stehen in STATUS und den verlinkten
Handoffs; Planung und Produkt-/Geräte-/Cloud-/Tresorfreigaben bleiben getrennt.


## Fortsetzungscheckpoint 23.10.2026

B3a ist als 77eccb7 öffentlich eingefroren, Vertragshash und acht CIjobs stehen in
STATUS. Lokale endliche Prüfungen abgeschlossen, keine eigenen Ressourcen/Jobs offen;
jetzt unabhängige P0-QA und native CI vor Integration/0.5-Clientbindung. Gesamtzeitgrenze
bleibt heute 23.10.2026 10:00 Europe/Berlin. F4 ist unabhängig angenommen und im Web-main
bf75748 integriert; F4b bleibt separates Rootpaket. Apple-main3c92f97 ergänzt nur A2-Review
und Status/Plan, Produkt7e97aae und grüne CI39505094304 unverändert. S2a separat ohne
Produktanker-/QAfreigabe. Keine weitere Funktion nach B3a ohne begrenzten Auftrag.

Für genau den freigegebenen F4b-Integrationscheckpoint übernimmt Frontend & UX
vorübergehend exklusiv RootSTATUS/ARBEITSPLAN. Backend & Platform bleibt danach idle
und schreibt diese Dateien bis erneuter gezielter Zuweisung nicht parallel.
Review- und Produktprovenienz bleiben erhalten; alle übrigen Besitzgrenzen unverändert.
