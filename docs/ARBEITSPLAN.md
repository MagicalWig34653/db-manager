# Arbeitsplan

Stand: 20.10.2026, 22:00 UTC. Autonom bis vollständig umgesetzt oder
21.10.2026 04:00 Europe/Berlin (=02:00 UTC). Lead koordiniert, organisiert Reviews,
integriert und pflegt zentrale Dokumente. Agents implementieren Produktcode.
Tatsächliche Ergebnisse und feste Vertragspins: [STATUS](STATUS.md).

## Aktive Pakete und Dateibesitz

| ID / Rolle | Repo / Branch / Worktree | Ziel und Abnahme |
| --- | --- | --- |
| F4 / Frontend & UX | Web / feat/row-workbench / .worktrees/row-workbench, Basis ffdd2a0 | Gepinnte API0.3/0.4, verpflichtende v4-Fähigkeiten auf Backend72855cf und v3-Eignung. Bestätigte Einzelzeilen-Inserts/Edits/Deletes mit Default/NULL/Leerwert-/Präzisionsunterscheidung, vollständigem altem typisiertem Snapshot und tatsächlicher Rücklese. Keine automatischen Retries; 409 fordert Neuladen, Unknown/Transportabbruch nach Versand als möglicher Commit. Session-/Quellenwechsel räumt Werte auf, generischer Unknown-Hinweis darf bleiben. Besitz Webprodukt/Vertragskopien/Build/Tests/Lizenzen/README/Einrichtung/Handoff, keine zentralen Statusdateien. Echte SQLite-/PG-TLS-Browserfälle einschließlich konkurrierender Änderung und verloren gegangener Antwort nach tatsächlichem Commit; CSP/Lizenz-/Sessionregressionen. Unabhängige QA vor Webintegration; Rootassetpin F4b separat. |
| S2a / Apple & Security | Apple / feat/vault-core / .worktrees/vault-core, Basis7cbeb38 | Eigenständiger Core im neuen Packages/DBManagerVault, eigene Tests/Fixtures/Originaltexte/CI/Skripte, ADR-A003/Sicherheitsdokumente/Handoff. S1-Format exakt, Actor-Locksession, verwaltetes privates Root mit festen Zufallsnamen und sicheren Dateideskriptoren, flock/CAS/temp0600/fsync/rename/dirfsync. Unabhängiger Anchor reserviert lebenslange Root-AEAD-Nutzung dauerhaft vor Verschlüsselung, bestätigt Hash/Generation erst nach dauerhafter Datei; Pending nur bekannte alte/neue Zustände. Fehlender/zurückrollbarer importierter Anchor erlaubt keine schreibende Produktsession. Nur Testanker in Testtarget, kein stiller InMemory-Produktfallback. Recovery darf mit einem überlebenden alten Faktor in neuen Tresor mit beiden neuen rückgelesenen Faktoren migrieren; Original erhalten. Keine App-/Xcode-/Rootpackage-/A2-/zentralen Statusänderungen. Unabhängige Dateisystem-/Fehler-/Concurrency-/Bounds-/Interop-QA vor Integration. |
| B3a / Backend & Platform | Hauptrepo / feat/schema-metadata / .worktrees/schema-metadata, Basis5cb604a | Lesende Index-/Constraintmetadaten für Strukturansicht, additive API, v1-v4 unverändert. Keine DDL/SQL-Fragmente; konservative Aussagen statt erratener Eigenschaften. Scope freigegeben: kein Ausdrucks-SQL, ehrliche CHECK-Lücke; neue Core-/DB-/HTTPdateien, enge Route/Authorizer, eigene Verträge/CI/Tests/Doku/Handoff; zentrale Statusdateien/Vertragsindex ausgenommen. Echte vier Treiber/TLS, Go-Race/Vet, Vertrags-/Sondernamen-/Ausdrucks-/Partial-/Include-/Kollationsfälle und unabhängiger Review. |

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
3. F4 nach Autorenabschluss unabhängig prüfen; Konflikt-/Unknown-/Sessionfälle und
   reale DB-Semantik sind Abnahmegates. Danach separater reproduzierbarer RootpinF4b.
4. S2a unabhängig prüfen. Reale Keychain-/signierte Anchorintegration und Appablauf
   sind ein separates S2b-Gate; weder Testspeicher noch Build ist Produktpersistenz.
5. B3a im freigegebenen lesenden Scope implementieren und separat prüfen. Weitere Pakete
   aus der Paritätsmatrix anschließend zuschneiden: Schemafunktionen, Import/Export,
   Verlauf/Favoriten und sichere native Netzwerkadapter. Jede Freigabe separat.

QA & Release p0_publication_review und qualifizierte nicht beteiligte Peers übernehmen
unabhängige Reviews. Bei vier Laufzeitplätzen einschließlich Lead wechseln Worker und
Reviewer geordnet. Nachträglich erhöhte lokale Konfiguration wird hier nicht neu geladen;
kein Neustart oder Umgehungsprozess. Kein paralleles Schreiben derselben Dateien.
Lead besitzt zentrale STATUS-/ARBEITSPLAN-Dateien aller Repos.

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
