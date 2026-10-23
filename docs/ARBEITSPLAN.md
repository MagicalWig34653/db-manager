# Arbeitsplan

Stand: 23.10.2026. Autonom bis vollständig umgesetzt oder heute **10:00
Europe/Berlin (08:00 UTC)**. Root koordiniert ausschließlich. Agents übernehmen
Umsetzung, Dokumentation, Prüfungen und konkret freigegebene Integrationsschritte.
Tatsächliche veröffentlichte Ergebnisse, Vertragspins und CI: [STATUS](STATUS.md).

## Aktuelle Nutzerprioritäten

1. Swift-App für TestFlight vorbereiten, Bundle-ID unter `de.linolaske`, echtes
   Icon mit Icon Composer. Bundle-ID `de.linolaske.dbmanager` ist ausdrücklich
   registriert; Nutzer hat ASC-Eintrag „DB Manager“ erstellt und wurde benachrichtigt.
   Signierte lokale iPad-/Mac-Exporte und unabhängiger Review59c7004 ohne Findings
   liegen vor. Die fehlgeschlagene exakte Apple-CI39906842616 gezielt korrigieren
   und offene Betreiber-/Distributionsangaben abschließen; kein Upload ohne eigenen Auftrag.
2. GitHub-Pages-Site mit Landingpage, gerenderter Doku, Roadmap und echten
   Screenshots bauen. Inhalt aus kanonischen Quellen, tatsächliche Features/Grenzen
   zeigen. Unabhängige Abnahme vor Veröffentlichung; keine erfundenen Termine.
3. B4a ist unabhängig ohne Findings geprüft, Integration/0.6-Freeze noch offen.
   F5-Darstellungsbefund gezielt korrigieren und unabhängig nachprüfen.
   Anschließend den neuen akzeptierten Webstand einschließlich F6 als gesondertes
   Root-Webpaket reproduzierbar einbetten und Compose/Swarm erneut prüfen.

Die Anforderungen des vollständigen Projekts bleiben bestehen. Priorisierung ist
keine Reduktion des Endziels oder Behauptung vollständiger Parität.

## Laufende Pakete und Dateibesitz

| Paket / Rolle | Repo, Branch / Worktree | Auftrag und Abnahme |
| --- | --- | --- |
| A-TF1 / Apple Release QA | Apple, Kandidat `b702051`, Handoff `a350f75`, vorhandener Reviewbranch `review/atf1` | Review59c7004 hat Source und tatsächliche IPA/PKG-Artefakte ohne bestätigte Findings geprüft. Exakte Apple-CI39906842616 ist inzwischen bei CompileAssetCatalogVariant/AppIcon.icon fehlgeschlagen; Ursache und enger Fix durch zuständigen Apple-Agent, danach unabhängige Nachprüfung/neue exakte CI. Keine Integration oder Uploadfreigabe aus lokalem Export ableiten. |
| W1 / Site Engineering | Root, `feat/pages-site`, `.worktrees/pages-site`, Basis `8aab52b` | Besitz ausschließlich neues site/, neuer Pagesworkflow, eigene Build-/Browser-/Link-/A11y-/Lizenzprüfungen und Handoff W1. Landing/Doku/Roadmap aus kanonischem Markdown, echte synthetische F4b-Webscreenshots mit ehrlichen Desktop-/Tablet-Bildunterschriften. Keine Go/API/Docker/Webassets/RootSTATUS-/Plan-/Appledateien. Öffentlicher Kandidat a972fb0 liegt beim unabhängigen Sitepeer; Abnahme vor Pagesaktivierung/Mainintegration/Deployment, Pages bis dahin HTTP 404. |
| F5 / Frontend QA | Web, `feat/schema-workbench`, `.worktrees/schema-workbench`; Kandidat `87751cc`, Handoff `6d8aa5e` | Lesende Index-/Constraintansicht auf freigegebenem API0.5 prüfen: geordnete Terme/Rollen, Partial/Expression/Include/FK/Check, Unknown/Coverage, ältere-Backend-Fallbacks, Sessiongrenzen und F6-/CRUD-/CSP-/Lizenzregressionen. Echte UI-Belege SQLite/PG; MySQL/Maria nur Unit-Testdaten. Eigener Reviewbranch/-handoff, keine Autorfixes. Alle drei bisherigen exakten CIs erfolgreich. Genau QA-F5-01 niedrig offen: Darstellung leerer Constraintnamen. Autor behebt eng, unabhängige Fixnachprüfung und passende neue CI vor Annahme. |
| B4a / Backend QA | Root, `feat/table-create`, `.worktrees/table-create`; Produkt `40defdc`, Handoff `464e647` | API0.6-Kandidat für ausschließlich neue Tabellen unabhängig prüfen: feste Typen/konstante Defaults/PK/Nullable, sichere Identifier/Encoder, Auth/Bounds, vier reale Treiber, Rechte-/Name-/Lock-/Abbruch-/Commit-/Beobachtungsraces. Bestätigter Abschluss und Strukturbeobachtung getrennt; kein universelles DDL-Rollback/Retry/Drop. Reviewf36463d ohne bestätigte Findings abgeschlossen, alle neun exakten CIs erfolgreich. Noch keine Mainintegration oder0.6-Freeze; Clientverbrauch erst nach konkreter Releasefreigabe und integriertem Vertragscheckpoint. |
| Projektgedächtnis / Dokumentation | Root, `docs/project-status-20261023`, `.worktrees/project-status` | Exklusiv docs/STATUS.md, docs/ARBEITSPLAN.md und zusätzlich docs/TEAM.md zu aktuellem Snapshot konsolidieren; historische Evidenz über Handoffs erhalten. Keine Produktdateien. Dokumentations-/Diff-/Autorprüfung, reiner Docscommit mit `[skip ci]`, Leadreview vor Integration/Push. Releasekoordinator erhält STATUS/Plan erst nach explizitem Transfer für den kurzen B4a-Integrationscheckpoint. |

Jeder Writer besitzt eigenen Branch/Worktree. Root weist weitere Implementierungs-,
Review- und Releasepakete gezielt zu. Keine parallelen Writer derselben Dateien.
Die beiden zentralen Rootdateien liegen exklusiv beim Dokumentations-Agent;
Apple-/Webstatus nur durch ausdrücklich benannten Agent pflegen. Reviewkandidaten
bleiben eingefroren, bis ein reproduzierbarer Befund an den Autor zurückgegeben wird.

## Review- und Integrationsgates

1. **A-TF1:** Source-/Artefaktreview59c7004 ist ohne Findings abgeschlossen,
   exakte Apple-CI39906842616 aber fehlgeschlagen. Compiler-/Iconfehler gezielt
   diagnostizieren, korrigieren und unabhängig nachprüfen; neue exakte grüne CI
   vor konkreter Apple-Agentintegration samt passendem Gesamtcheck; Originalarchive erhalten.
   Exportcompliance/Privacy/TestFlightmetadaten und Uploadauftrag separat klären.
   Keine TestFlightverarbeitung, physische Geräte oder Runtime-Sandbox allein aus
   Signatur/Entitlements behaupten.
2. **W1:** öffentlichen Kandidaten a972fb0 unabhängig nachbauen und in Browsern
   prüfen. Landingclaims gegen STATUS, Doku-/Basislinks und Screenshots/Alttexte,
   lokale Suche, Zugänglichkeit, Originalnotices und minimale Workflowrechte
   verifizieren. Erst danach autorisierte Agentveröffentlichung und echte Live-URL/
   Basis-/Asset-/Suchprobe. Nutzer hat die Pages-Seite beauftragt; kein neuer Rename.
3. **F5:** QA-F5-01 durch engen Autorfix und unabhängige Nachprüfung schließen;
   passende exakte grüne CI bestätigen, erst danach Webmerge
   mit Reviewprovenienz. F6 ist bereits im Web-main integriert. Das separate
   Rootpaket F5b darf nur den endgültig freigegebenen gemeinsamen Quellpin verwenden;
   zwei bytegleiche öffentliche Builds, Originalnotices/Manifest, echter eingebetteter
   SQL-/Tabellen-/CRUD-/Export-/Sessionpfad und Compose-/isolierte-Swarm-Persistenz.
4. **B4a:** Reviewf36463d und alle neun exakten CIs sind ohne Findings/grün.
   Nach konkreter Freigabe Agentintegration, API0.6-Vertragsindexfreeze und
   kombinierte Mainprüfungen. Der Releasekoordinator erhält den kurzen zentralen
   Integrationscheckpoint erst nach explizitem Dateibesitztransfer.
   Erst danach neue Tabellenanlage im Client als eigenes Ticket beginnen.
5. **Checkpoint:** vor der Zeitgrenze tatsächliche Dateien/Commits, Tests/Grenzen,
   unintegrierte Arbeit, Prozesse/Ressourcen und nächsten Schritt pro Paket sichern.
   Fremde Änderungen und Originalartefakte erhalten; keine pauschale Bereinigung.

Unabhängige Reviews dürfen qualifizierte Peers übernehmen; kein Autor nimmt sein
eigenes Paket allein ab. Bei bestätigten Findings gehen Repro/Schweregrad an den
Eigentümer, Fix und Nachprüfung bleiben getrennt. Grüne Branch-CI ersetzt weder
Review noch die passenden kombinierten Integrationschecks.

## Zurückgestellte oder separat offene Arbeit

- G-S2b: damaliger DP-Keychain-Gateversuch mit ad-hoc-Host `-34018`, keine Items.
  Atomarer CAS, Lock-/Unsicherheitsfälle und Vertrauensgrenze auf tatsächlich
  geeignetem signierten/provisionierten Host separat belegen. Kein produktiver
  InMemory-/Dateifallback und keine App-Geheimnisspeicherung aus den bisherigen Tests.
- C-Export1/A3: gemeinsamen Root-/Web-/Apple-Exportvertrag mit Schema/Fixtures/
  Provenienz unabhängig freigeben, anschließend nativen Ergebnisexport als eigenes
  Paket. Bisher nur Vorschlag im Gatebranch, zugunsten Pages/TestFlight zurückgestellt.
- Weitere Schemafunktionen, Import/Export, gespeicherte Abfragen, sichere native
  Netzwerkadapter, Passkeys, CloudKit-Sync, Teams/Rechte/Jobs und übrige
  [Paritätsmatrix](PARITAET.md) schrittweise mit eigenen Abnahmekriterien zuschneiden.
  Vollständige Parität bleibt offen, keine pauschalen Ausnahmen durch Priorisierung.

## Verbindliche Betriebs- und Arbeitsgrenzen

Laufzeit jetzt **31 gleichzeitige Plätze einschließlich Root**; bis zu 30 Worker
möglich, keine festen Anbieter-/Modellquoten. Fachrollen und aktive Slots bleiben
getrennt. Anforderungen/Entscheidungen/Pläne und tatsächliche Prüfungen klar trennen.
AGPL-3.0-only für eigenen Code; Drittanbietertexte und Inventare erhalten. Deutsch
für UI/Dokumentation, Englisch für Code/Tests/Kommentare/technische Bezeichner.
Bestehende Git-Autoridentität erhalten, keine Secrets/privaten Pfade/produktiven
SQLdaten oder vollständigen Chats veröffentlichen.

Compose und Swarm verbindlich. SQLite gemäß ADR0004 nur exklusive lokale
SingleReplica-Datei mit einer aktiven Verbindung und DELETE-Journal; kein
WAL/NFS/fremder SQLiteprozess/HA-/Powerlossversprechen. Native MySQLNIO-Adapter
wegen belegtem TLS-Downgrade gesperrt. Abbruch kein garantiertes Rollback,
unbekannter Commit kein automatischer Retry, keine Exactly-once-Zusage.

Nur eigene kurzlebige Fixtures/Container/Volumes/Netze/Scratchs verwenden; fremde
Ressourcen, Host-Swarm und Systemrechte unverändert lassen. Laufende Jobs vor
Neustarts tatsächlich prüfen, Beobachtungstimeout nicht als Prozessende behandeln.
Nicht ausgeführte Prüfungen offenlassen. Dieser Dokumentations-Worker hat keine
Produkt-/DB-/Browser-/Signing-/Deployjobs gestartet; nach seinem Docscommit ist der
nächste Schritt Leadreview der beiden geänderten Dateien und konkrete Integration.
