# Tatsächlicher Projektstand

Stand: 23.10.2026; letzter CI-Prüfcheckpoint 05:52 UTC (07:52 Europe/Berlin). Die autonome Arbeitsphase ist
vom Nutzer bis zum vollständigen Abschluss oder heute **10:00 Europe/Berlin
(08:00 UTC)** autorisiert. Root koordiniert ausschließlich; Agents übernehmen
Implementierung, Dokumentation, Prüfungen und freigegebene Integrationsschritte.
Docker Compose und Docker Swarm bleiben verbindliche Betriebsziele (ADR-0003).

## Veröffentlichte Hauptstände

| Repository | Geprüfter Hauptstand | Tatsächlicher Umfang |
| --- | --- | --- |
| [db-manager](https://github.com/MagicalWig34653/db-manager) | B4a-Release ab `8708c3e`; Produkt-/Reviewcheckpoint `3d21e42` | Go mit SQLite/MySQL/MariaDB/PostgreSQL, SQL, Tabellenansicht, bestätigten Einzelzeilen-Inserts/Updates/Deletes, lesender Index-/Constraint-API0.5 und bestätigter neuer Tabellenanlage über API0.6. Eingebetteter Webstand bleibt F4b, Quelle `3a4a3fd`. |
| [db-manager-web](https://github.com/MagicalWig34653/db-manager-web) | `204cb07` | F6-Ergebnisexport nach unabhängiger QA integriert: typisiertes JSON sowie klar getrennte CSVvarianten, explizite Steuerzeichenablehnung vor CSV-Download. Noch nicht im Go-/Docker-Webbuild enthalten. |
| [db-manager-apple](https://github.com/MagicalWig34653/db-manager-apple) | `e50d745`; Produktcheckpoint `d0dea67` | Eigenständige SwiftUI-App mit SQLite und direktem PostgreSQL ohne Go; isolierter geprüfter Vault-Core zusätzlich im Repository. Keine Appverdrahtung oder produktive Credentialpersistenz aus dem Core. TestFlight-/Iconänderungen liegen separat im Reviewkandidaten. |

Branch-/Handoffstände sind von Integration und Veröffentlichung getrennt. Eine grüne
CI ersetzt keine unabhängige Abnahme oder einen tatsächlichen Geräte-/Uploadnachweis.

## Abgenommene Inkremente und historische Nachweise

| Paket | Tatsächliches Ergebnis | Nachweis |
| --- | --- | --- |
| P0–P4a, P2b | Drei öffentliche Repos, Paritätsreferenz/Anforderungen, Go-/Swift-/Securityspikes, HTTP/Auth-/Containergrundlage und eingeschränkte SQLite-VFS-Entscheidung | [P0](handoffs/P0-review.md), [P1a](handoffs/P1a.md), [P2a](handoffs/P2a.md), [P3a](handoffs/P3a.md), [P4a](handoffs/P4a.md), [P2b](handoffs/P2b.md) |
| B1 | Reale SQLite-/MySQL-/MariaDB-/PostgreSQL-Engine; vier Reviewbefunde geschlossen | [B1](handoffs/B1.md) |
| A1/A2 | Native SQLite-/PostgreSQL-App, präzise Werte und verifizierter TLS-/Abbruchpfad; Lifecycle-/Kalenderbefunde geschlossen | Apple-Handoffs [A1](https://github.com/MagicalWig34653/db-manager-apple/blob/e50d745/docs/handoffs/A1.md), [A2](https://github.com/MagicalWig34653/db-manager-apple/blob/e50d745/docs/handoffs/A2.md), [unabhängiger A2-Review](https://github.com/MagicalWig34653/db-manager-apple/blob/e50d745/docs/handoffs/A2-review.md) |
| F1/F1b, F2a/F2b, F3/F3b | Deutscher Web-SQL-/Tabellenarbeitsplatz mit Quellen-/Originallizenzdialog, reproduzierbaren Assets und echtem Browser→Go→DB-Pfad; Session-/CSP-/Filterzustandsbefunde geschlossen | [F2a](handoffs/F2a.md), [F2b](handoffs/F2b.md), [F3b](handoffs/F3b.md), [F3b-Review](handoffs/F3b-backend-review.md) |
| B2a/B2b/B2c | Metadaten, Sortierung, gebundene Filter und Pagination sowie bestätigte einzelne Inserts/Updates/Deletes mit konservativer Eignung und ehrlichem unbekanntem Commitausgang | [B2a](handoffs/B2a.md), [B2b](handoffs/B2b.md), [B2c](handoffs/B2c.md) |
| B2b-Schutzfixes | SQLite OR ABORT verhindert fremde REPLACE-Löschungen; PostgreSQL-Rewrite-Tabellen werden gesperrt. Beide nachträglichen Integritätsbefunde unabhängig geschlossen | [SQLite](handoffs/B2b-konfliktstrategie.md), [PostgreSQL](handoffs/B2b-rewrite-regeln.md) |
| F4/F4b | Web-Einzelzeileneditor und exakt reproduzierte Go-/Docker-Einbettung. Produkt `3770c4e`, Handoff `46e4bf1`, unabhängiger Review `533348a`; als `ffcf519`/`2ad78b2` integriert | [F4b](handoffs/F4b.md), [F4b-Review](handoffs/F4b-backend-review.md) |
| B3a | Lesende Index-/Constraintmetadaten. Namespacebefund B3a-01 im Fix `7c66bb9` unabhängig geschlossen; Feature/Review als `fddc574`/`ac9542d` integriert, API0.5 freigegeben | [B3a](handoffs/B3a.md), [B3a-Review](handoffs/B3a-review.md) |
| B4a | Strukturierte neue Tabellen mit festen Typen, konstanten Defaults und konservativem PK/Nullable-Modell. Produkt `40defdc` und unabhängiger Review `f36463d` ohne Findings, alle neun Produkt-CIs erfolgreich; Feature/Review als `77c4c41`/`3d21e42` integriert, API0.6 freigegeben | [B4a](handoffs/B4a.md), [B4a-Review](handoffs/B4a-review.md), [Releasecheckpoint](handoffs/B4a-integration.md) |
| F6 | Lokaler Export geladener Ergebnisse ohne zusätzliche DB-Abfrage. QA-F6-01 geschlossen, Quelle `1a8adfa` mit Review `8301b0c` im Web-main integriert | [F6](https://github.com/MagicalWig34653/db-manager-web/blob/204cb07/docs/handoffs/F6.md), [F6-Review](https://github.com/MagicalWig34653/db-manager-web/blob/204cb07/docs/handoffs/F6-review.md) |
| S1/S2a | Interoperables Tresorformat und isolierter nativer Core: Reservierung/CAS/Pending/Recovery und atomare Dateiablage geprüft. Kein produktiver Anchor oder App-Geheimnisspeicher | [S1](handoffs/S1.md), [Format](security/VAULT-FORMAT-V1.md), Apple [S2a](https://github.com/MagicalWig34653/db-manager-apple/blob/e50d745/docs/handoffs/S2a.md), [S2a-Review](https://github.com/MagicalWig34653/db-manager-apple/blob/e50d745/docs/handoffs/S2a-review.md) |

Die Handoffs erhalten vollständige historische Commits, Repros, Prüfungen und Grenzen.
Überholte Zwischenstände werden hier nicht als weiterhin aktive Blocker wiederholt.

## Freigegebene Verträge

| Vertrag | Herkunft / Referenzbackend | SHA256 |
| --- | --- | --- |
| API0.1.0 | `80f203b3e0c7fa3f7f4aa5ec06825cb83bc0c5d0` | `4b598dc69a1904685e67a717997412654440ac10f47c17f7a025ccd114e294be` |
| API0.2.0 | `427444d7bcb8e30c9563ded2000ec3d5e19646c4` | `c15a848f01bb779429fd68a2b6eb6bba1575d3e07a4c68e6796cf7840a661838` |
| API0.3.0 | `946241d7f10f7444e9e5efebbe8b3a38a8120a9b` (historische Herkunft) | `ec77c9739cda11df3f4cb9fe756a632e67840fdc0f6190deee1aa1d1333dc308` |
| API0.4.0 | `72855cf57ceb6620dead42856ee6c9d52d0612bd` enthält beide v3-Schutzfixes | `4763ebf8632a9ef977cf7d3a6037299fc4dafde0a581fe10cb34d844f8356fbf` |
| API0.5.0 | `ac9542d81e6f096626c24bc1e0cb9bccb3a62a7c` enthält Namespacefix und Review | `b5225339511d80fe4f52f09118fdad2060d8a92f31d44b73dd89fe9f76178c95` |
| API0.6.0 | `3d21e42b0806f2a1953bcbad47f1e9fcb2acda93` enthält geprüfte Tabellenanlage und Review | `b08b6aba858f72a3bad1fce2933e487eed017f5e35b4690cfd9fae3a590a6887` |
| Tresorformat1 | `1a4eb3c`/`03bd3f4`, ADR0005 | `b688656056fe2e23848ce17149bc817609c7369f230a3f49dd91f05db4d3f4dd` |

Neue Schreibclients benötigen den korrigierten Backendstand `72855cf` oder eine
nachgeprüfte Nachfolge. Gleiche API0.3-Vertragsbytes beweisen keinen Serverfix; die
frühere Implementierung war für SQLite-REPLACE-/PostgreSQL-Rewrite-Fälle unsicher.
API0.5 ist ausschließlich lesend und verspricht keinen atomaren Schemasnapshot.
API0.6 bestätigt ausschließlich die neue Tabellenanlage; bestätigter Abschluss und
Strukturbeobachtung bleiben getrennt, keine bestehenden Objekte ändern und keine
universelle DDL-Rollback-/Zustandsnach-Antwortgarantie ableiten.
API- und Tresorversionen haben getrennte Versionsräume. [Vertragsindex](../contracts/README.md).

## Aktuelle Reviewkandidaten und Nutzeraufträge

| Paket | Tatsächlicher Stand | Noch erforderliches Gate |
| --- | --- | --- |
| F5 – Web-Strukturansicht | `feat/schema-workbench`, Kandidat `87751cceed47332f2164f2c3c584ddcd0ef2edd8`, Handoff `6d8aa5e`; geordnete Index-/Constraintdetails, Coverage/Unknown und ältere-Backend-Fallbacks. Alle drei exakten CIs inzwischen erfolgreich. Frühere Linux-CDP-Testbeobachtung scheiterte; eng ohne Produktänderung korrigiert. | Unabhängiger Review mit genau QA-F5-01 (niedrig) offen: leere Constraintnamen werden unklar dargestellt. Autor korrigiert gezielt, unabhängige Nachprüfung erforderlich. Echte Browsernachweise SQLite/PG; MySQL/Maria-UI bislang nur Unit-Testdaten. Kein Rootpin vor Annahme. [Handoff](https://github.com/MagicalWig34653/db-manager-web/blob/6d8aa5e/docs/handoffs/F5.md) |
| W1 – GitHub Pages | `feat/pages-site`, öffentlicher Kandidat `a972fb0`; Landingpage, gerenderte kanonische Doku, Roadmap und vier echte synthetische Webscreenshots. Unabhängige Site-QA läuft; keine Pages-Veröffentlichung behauptet. | Unabhängige Site-/Build-/Browser-/Link-/Lizenzprüfung vor Integration und Deployment; Pages-API am aktuellen Checkpoint HTTP 404. |
| A-TF1 – TestFlight und Icon | `release/testflight-preparation`, Kandidat `b7020517d6431a852f61188f70c90e495534bdd2`, Handoff `a350f75`. Bundle-ID **de.linolaske.dbmanager** explizit registriert; Nutzer hat ASC-Eintrag „DB Manager“ angelegt und wurde zum Fortsetzen benachrichtigt. Signierte iPad-/Universal-Macarchive und lokale IPA-/PKG-Exporte belegt, echte editierbare Icon-Composerquelle und Rendernachweise vorhanden. | Unabhängiger Artefakt-/Sourcereview `59c7004` ohne bestätigte Findings abgeschlossen. Docs-/Vault-CI erfolgreich; exakte Apple-CI39906842616 inzwischen fehlgeschlagen (CompileAssetCatalogVariant für AppIcon.icon mit CI-Xcode26.3). Konkrete Ursache und Korrektur noch offen, Integrationsgate nicht erfüllt. Kein Upload oder Einreichen. Betreiberangaben zu Exportcompliance/Privacy und TestFlightmetadaten bleiben vor Upload offen. [Handoff](https://github.com/MagicalWig34653/db-manager-apple/blob/a350f75/docs/handoffs/A-TF1.md) |
| G-S2b – OS-Anchor | Separater Spike `4953399`/`07c284a`, Vorschlagscheckpoint `a6df2dc`: damaliger eigener ad-hoc-Host ohne Team-ID erhielt bei DP-Keychain-SecItemAdd `-34018`; kein Item angelegt, keine Dialoge oder Fallbacks. | Atomarer CAS, OS-Lock und Unsicherheitsfälle auf geeignetem signierten/provisionierten Host nicht belegt. Die inzwischen vorbereitete Appsignierung schließt dieses unabhängige Plattformgate nicht automatisch. Kein produktiver Anchor/App-Geheimnisspeicher. |
| C-Export1/A3 | Gemeinsamer Root-/Web-/Apple-Exportvertrag und nativer Ergebnisexport als Vorschlag gesichert; zugunsten Pages/TestFlight noch nicht implementiert. | Gemeinsamer Vertrag unabhängig freigeben, erst danach native Implementierung. |

„GoMyAdmin“ wurde als Name diskutiert, aber nicht beschlossen. Kein Repository- oder
Produkt-Rename; der vom Nutzer angelegte App-Store-Connect-Eintrag heißt „DB Manager“.

## Verifizierte Prüfungen und CI

- Root-Produktstand `4de72bc`: alle sieben kombinierten CIs erfolgreich; IDs
  Docs39902300560, Web39902300600, Backend39902300593, Engine39902300588,
  Tabellen39902300594, Mutationen39902300556, Insert39902300614.
- Root-Stand `8aab52b`: alle acht kombinierten CIs erfolgreich; IDs
  Docs39903385427, Struktur39903385487, Engine39903385441, Backend39903385549,
  Tabellen39903385422, Mutationen39903385437, Insert39903385476, Web39903385529.
- Web-main `204cb07`: [Frontend39905016492](https://github.com/MagicalWig34653/db-manager-web/actions/runs/39905016492), [echte Integration39905016530](https://github.com/MagicalWig34653/db-manager-web/actions/runs/39905016530) und [Docs39905016391](https://github.com/MagicalWig34653/db-manager-web/actions/runs/39905016391) erfolgreich.
- Apple-S2a-Produktstand `d0dea67`: [Vault39903661136](https://github.com/MagicalWig34653/db-manager-apple/actions/runs/39903661136), [Apple39903661074](https://github.com/MagicalWig34653/db-manager-apple/actions/runs/39903661074) und [Docs39903661079](https://github.com/MagicalWig34653/db-manager-apple/actions/runs/39903661079) erfolgreich.
- B4a `40defdc`: alle neun CIs erfolgreich; IDs Anlage39905577680,
  Struktur39905577665, Engine39905577655, Backend39905577678, Tabellen39905577617,
  Mutationen39905577615, Insert39905577609, Web39905577711, Docs39905577731.
- F5 `87751cc`: [Frontend39906977846](https://github.com/MagicalWig34653/db-manager-web/actions/runs/39906977846), [echte Integration39906977849](https://github.com/MagicalWig34653/db-manager-web/actions/runs/39906977849) und [Docs39906977828](https://github.com/MagicalWig34653/db-manager-web/actions/runs/39906977828) erfolgreich.
- A-TF1 `b702051`: [Docs39906842633](https://github.com/MagicalWig34653/db-manager-apple/actions/runs/39906842633) und [Vault39906842798](https://github.com/MagicalWig34653/db-manager-apple/actions/runs/39906842798) erfolgreich; [Apple39906842616](https://github.com/MagicalWig34653/db-manager-apple/actions/runs/39906842616) beim aktuellen Abruf `completed/failure`. Fehlgeschlagener Befehl: CompileAssetCatalogVariant für Assets/AppIcon.icon im macOS-Build mit Xcode26.3; kein unbelegter Schluss auf die konkrete Ursache.

F4b ist unabhängig mit zwei bytegleichen öffentlichen Builds, neun echten
Browserfällen, Go-Race/Vet, neun Pythonprüfungen, Compose und isoliertem DinD-Swarm
mit CRUD/409/typisierter SQLitepersistenz nach Neustart/Taskersatz geprüft. Frischer
Go-/Imagecheckout benötigt keine Node-/npm-Laufzeit oder Geschwisterrepos.
S1/S2a-Format-/Originalquell-/CAS-/Prozessabbruchnachweise stehen in ihren Handoffs.
Signierte A-TF1-Archive sind noch kein physischer Gerätelauf oder Runtime-Sandboxbeweis.

## Team, Grenzen und Fortsetzung

Die Laufzeit bietet jetzt **31 Plätze einschließlich Root**, also bis zu 30 Worker;
keine feste Modell-/Anbieterquote. Jeder schreibende Worker erhält eigenen Branch/
Worktree und begrenzten Dateibesitz. Root koordiniert und erteilt konkrete
Integrationsfreigaben. Für den begrenzten B4a-Releasecheckpoint liegen diese beiden zentralen Rootdateien
nach ausdrücklichem Transfer exklusiv beim Releasekoordinator; der Dokumentations-Agent
schreibt nicht parallel. Andere Worker liefern paketbezogene Handoffs. Apple-/Web-
Statuspflege erfolgt nur nach eigener ausdrücklicher Zuweisung.

Keine vollständige phpMyAdmin-Parität: [Matrix](PARITAET.md) und [Teilnachweise](PARITAET.md#teilnachweise-des-implementierten-zwischenstands)
trennen Ziele von Implementierung. Offen bleiben insbesondere produktiver Tresor/
Recoveryablauf, Passkeys, CloudKit-Sync, Teams/Rechte, umfassende Schema-/Import-/
Exportfunktionen und sichere native MySQL-/MariaDB-Adapter (belegter TLS-Downgrade).
Echte Geräte, alte OS-Laufzeiten, interaktive OS-UI und signierte Runtime-
Sandboxdurchsetzung bleiben separat offen. Keine Upload-/App-Store-Verfügbarkeit.

SQLite nach ADR0004: SingleReplica, exklusives lokales Volume, eine aktive Verbindung,
DELETE-Journal, kein WAL/NFS/fremder SQLiteprozess/HA-/Powerlossversprechen.
Abbruch garantiert keinen Rollback, unbekannter Commit keinen automatischen Retry;
Trigger-/externe Effekte, ABA und Exactly-once bleiben begrenzt. Nur eigene
Wegwerfressourcen verwenden; fremde Container und Host-Swarm nicht verändern.
Git-Refanomalie wurde reversibel ohne Änderung gültiger Refs quarantänisiert;
[Nachweis](handoffs/Git-Refs-2026-10-23.md), keine belegte Aussage zur Ursache.

Nächste Schritte: die neun neu gestarteten kombinierten B4a-Main-CIs kontrollieren;
A-TF1-CIfehler gezielt klären, F5-Darstellungsfix unabhängig nachprüfen und W1-QA
abschließen. Danach jeweils freigegebene Agentintegration/Gesamtchecks; F5/F6 erst als eigenes geprüftes
Root-Webpaket einbetten. A-TF1 darf erst nach gesonderter Uploadfreigabe hochgeladen
werden. Vor 10:00 Berlin alle eigenen Jobs, Branches, Tests, offenen Gates und
konkreten nächsten Schritte checkpointen. [Arbeitsplan](ARBEITSPLAN.md),
[Teammodell](TEAM.md), [Einrichtung](../EINRICHTUNG.md).

Dokumentationscheckpoint: eigener Branch `docs/project-status-20261023`,
Worktree `.worktrees/project-status`; ausschließlich STATUS/ARBEITSPLAN und das
zusätzlich zugewiesene TEAM-Dokument verändert. Dateibesitz endet erst nach
explizitem Transfer an den Releasekoordinator für dessen Integrationscheckpoint.
Keine eigenen Produkt-, DB-, Browser-, Signing- oder Deployjobs gestartet.
Produktprüfungen für diese reine Dokumentationsänderung nicht erneut ausgeführt.
Dokumentationsprüfung: 275 Dateien, null Fehler; Diffprüfung bestanden, sechs
Vertragsprüfsummen direkt bestätigt. Leadreview vor Integration bleibt offen.

## B4a-Integrationscheckpoint

Nach konkreter Leadfreigabe im eigenen `release/b4a-20261023`-Worktree integriert:
Featuremerge `77c4c41`, unabhängiger Reviewmerge `3d21e42`, angenommene vollständige
Dokumentationsprovenienz `35c151c`/`e554bb0` als `d1d4699`. Geprüfte B4a-Produkt-,
Vertrags- und Workflowbytes unverändert. API0.6 im [Vertragsindex](../contracts/README.md)
ausdrücklich freigegeben; Referenzbackend `3d21e42` enthält Implementation und Review.
Keine Site-, F5- oder Appleproduktänderung in diesem Release.

Kombinierte lokale Checks bestanden: `go test -race -count=1 ./...`, `go vet ./...`,
alle fünf Vertragswächter, tatsächliches Manifest mit fünf Originalassets, neun
Python-Buildprüfungen und Dokumentations-/Diffcheck. Keine neue lokale Docker-,
Live-DB-, Compose- oder Swarmserie für diese unveränderten Betriebsbereiche; die
unabhängigen vier Treiberproben und neun exakten Produkt-CIs stehen im Review.
Die Veröffentlichung dieses Checkpoints startet eine neue kombinierte Mainserie
mit neun CIs; deren Ergebnisse werden noch nicht als erfolgreich behauptet.

STATUS/ARBEITSPLAN wurden nach ausdrücklichem Abschluss/Transfer des Docs-Agents
für diesen begrenzten Release exklusiv durch den Releasekoordinator aktualisiert.
Keine eigenen lokalen Prüfjobs oder Ressourcen verbleiben. Nach Push: exakte
Main-SHA/CI-IDs melden und Dateibesitz durch Root neu zuweisen lassen.
