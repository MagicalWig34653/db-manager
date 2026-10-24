# Arbeitsplan

Stand: 23.10.2026. Neuer ausdrücklicher Arbeitsabschluss: erster Build auf
TestFlight, Pages online, danach vollständige Doku- und Claude-Code-Übergabe.
Root koordiniert ausschließlich; Agents übernehmen die freigegebenen Schritte.
Offene Gesamtproduktfunktionen werden in der Übergabe festgehalten, nicht als fertig
behauptet. Tatsächlicher Stand: [STATUS](STATUS.md).

## Verbindliche Abschlussreihenfolge

1. Geprüfte Apple-TestFlightvorbereitung ist im Main359666f integriert, signierte
   Exporte privat dauerhaft gesichert. Nur der dedizierte Delivery-Agent führt den
   inzwischen ausdrücklich autorisierten iPad-Erstupload0.1.0Build1 aus. Keine
   parallelen Uploads, kein App-Store-Release und keine öffentlichen Einladungen.
   Tatsächliche TestFlightverarbeitung/Buildanzeige statt bloßer Vorbereitung belegen.
2. F5b und W1 sind nach unabhängiger Source-/Artefakt-/CIabnahme gemeinsam integriert.
   Kombinierten Rootstand, kanonische Sitequellen und320px-Layout prüfen; einmal
   main veröffentlichen, neue Produkt-/Pages-CIs erfassen. Danach autorisierte
   Pageskonfiguration und manueller main-Publishdispatch, Live-URL/HTTPS/Assets/
   Navigation/Suche/Doku/Roadmap/Screenshots/Source/Notices tatsächlich prüfen.
3. Wenn TestFlight-Erstbuild und Pages-Livebetrieb belegt sind: sämtliche Doku auf
   tatsächlichen Endstand bringen, verbleibende Gates/WIPs/Jobs/Dateibesitz erfassen
   und eine unmittelbar ausführbare Übergabe an Claude Code bereitstellen.
4. Keine weitere neue Featureintegration. Bisherige Produktanforderungen bleiben
   offen dokumentiert; dieser neue Abschluss bedeutet keine vollständige Parität.

## Prio PWA-Parität (Nutzerfreigabe 24.10.2026)

Der Nutzer hat am 24.10.2026 als höchste Priorität freigegeben, alle phpMyAdmin-Funktionen in der PWA (Svelte-Weboberfläche über den
Go-Server) verfügbar zu machen, und autonomes Weiterarbeiten erlaubt. Für diese Linie ist die Zwischenregel „keine weitere neue
Featureintegration“ (Punkt 4 oben) aufgehoben; die Apple-Linie ist nachrangig. Plan, Wellen, Pakete, Ablauf und Entscheidungsbedarf:
[PWA-PLAN](PWA-PLAN.md) (Entwurf, bis zur Leadfreigabe kein Beschluss). Pakete verwenden eigene Worktrees und Dateibesitz wie dort
beschrieben; Veröffentlichung (Images, Pages) bleibt ohne ausdrückliche Freigabe ausgeschlossen. Vollständige Parität bleibt offen.

## Integrationsstand und Dateibesitz

| Paket | Geprüfter Stand | Abschlussgate |
| --- | --- | --- |
| B4a | Rootmain1b02ea3/API0.6, Reviewf36463d | Alle neun Main-CIs erfolgreich, keine neue Clientfunktion daraus ableiten |
| F5/F6 | Webmainf75bfa1, Source8ef738d, Review469653dd | Alle drei Webmain-CIs erfolgreich |
| F5b | Produkt0dc9557, Handoff83dae46, Review968f375; Source8ef unverändert | Kombinierte Rootchecks/Main-CI, keine erneute Bytebereinigung oder Assetneubau |
| W1 | Produkt08021c1, Handoff07b6b9c, Review983ac36, alle zehn Source-CIs grün | Kombinierter Build/Link-/Notices-/Browsercheck, autorisierte Veröffentlichung und echter Livebeweis |
| A-TF1 | Apple-main359666f, Source645c8c1, Review7e6619e | Neue Main-CI getrennt prüfen; Upload/Verarbeitung durch Delivery-Agent |

RootSTATUS/ARBEITSPLAN liegen ausschließlich für diesen kombinierten Release beim
Releasekoordinator; Dokumentations-Agent schreibt bis erneutem Transfer nicht.
TEAM bleibt unverändert. Site-/Web-/Appproduktbytes sind eingefroren; Änderungen
nur nach eigenem Finding/Fix/Nachreview. Private Originalarchive/Exporte erhalten,
keine Profile/Keys/Artefakte in öffentlichen Git-/Releaseassets.

Nach erfolgreichem Pageslive- und TestFlightnachweis übergibt Root die zentrale
Statuspflege ausdrücklich an den Dokumentations-/Handoff-Agent. Der Siteworkflow
veröffentlicht ausschließlich manuellen main-Dispatch mit publish=true, kein PR- oder
Pushdeploy. Finale reine Dokunachträge dürfen diesen Schutz nicht umgehen.

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
