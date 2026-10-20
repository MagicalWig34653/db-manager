# Handoff P1a – phpMyAdmin-Funktionsinventar

Stand: 20.10.2026. Implementierer: beauftragter P1a-Worker; unabhängiger Review ausstehend.
Paket im Hauptrepository, Branch `docs/p1a-paritaet`; eigener Worktree `.worktrees/p1a`.
Ursprünglicher Dateibesitz `docs/PARITAET.md` und dieser Handoff; durch Leadauftrag
um `docs/TEAM.md` erweitert (Nutzerpräzisierung zum professionellen Team).
Gemeinsamer API-Vertrag: weiterhin keiner. Kein Produktcode verändert. `docs/TEAM.md` beschreibt Rollenbesetzung, maximal drei aktive
Worker, Ready-/Done-Kriterien, unabhängige Reviews sowie Integrations-/Releasegates.

## Ergebnis und Abnahme

- Offizielle Downloadseite und versionsgebundene Dokumentation recherchiert;
  konkrete Arbeitsreferenz phpMyAdmin 5.2.3 vom 08.10.2025 vorgeschlagen.
  `latest` ist 5.2.4-dev und bewusst keine Grundlage des Inventars.
- Offizieller Tag `RELEASE_5_2_3`, Tagobjekt
  `da8def9bce141e3fa08d38612fd3dfdc4c2ae59a`, aufgelöster Commit
  `962857e4f63d42e38f11ff4d63f5e722018add76` geprüft. Codequellen verwenden
  unveränderliche Commitlinks. Keine Signaturprüfung des Tags ausgeführt.
- 181 stabile PMA-IDs in 16 Bereichen, jeweils Referenzfunktion, Sollverhalten,
  DB-/Plattformbezug, ausschließlich vorgeschlagene Reihenfolge und überprüfbares
  Abnahmekriterium. Quellen je Bereich und gemeinsame Statussemantik dokumentiert.
- Auch spezielle Funktionen erfasst: Partitionierung, Binärlog, Replikation, GIS,
  Zoom-Suche, Zentralspalten, Normalisierung, Monitoring/Advisor, Tracking,
  Import-/Export-/Schemaformate, Transformationsfamilien und Auth-Integrationen.
- Alle 181 Funktionen stehen ausdrücklich auf `O`: keine Produktimplementierung,
  keine ausgeführte Funktionsabnahme, keine nachgewiesene Parität.
- Sechs zusätzliche Produktziele separat: Compose/Swarm, eigenständige Apple-App,
  zusätzliche DBs, Tresor/Passkey/Masterpasswort, iCloud, Teams. Keine Herkunft aus
  phpMyAdmin behauptet. Keine Freigabe einer Kryptografie- oder Treiberentscheidung.
- Vollständigkeitsgrenzen und nicht entschiedene native Ersatzabläufe explizit benannt.
  Das Inventar ist für Planungsreview geeignet; vollständige P1-/Releaseabnahme
  verlangt Referenzvergleich, konkrete DB-/OS-Ziele und Optionsfixtures.

## Prüfungen am 20.10.2026

Aus dem eigenen Worktree ausgeführt:

1. `git status --short`: bei Übernahme sauber; vor Commit nur die drei eigenen Dateien.
2. `git ls-remote https://github.com/phpmyadmin/phpmyadmin.git refs/tags/RELEASE_5_2_3 'refs/tags/RELEASE_5_2_3^{}'`:
   erfolgreich, Tag und Commit wie oben.
3. Temporärer Sparse-Checkout des offiziellen Tags, `git rev-parse HEAD`:
   exakt Referenzcommit; `git status --short` dort sauber. Controller-/Plugin-/Template-
   und Dokumentationsinventar gelesen. Referenzcode bleibt außerhalb des Repositories.
4. `python3 scripts/check-docs.py`: nach den drei Dokumenten **72 Dateien, 0 Fehler**.
5. Ad-hoc-Strukturprüfung mit Python: **181 eindeutige PMA-IDs**, sieben Spalten,
   jede PMA-Zeile mit Status `O` und Prioritätsvorschlag; alle verlinkten Codepfade
   im Referenzcheckout vorhanden. Keine dauerhafte Testdatei für diese Dokumentprüfung.
6. `git diff --check`: erfolgreich, keine Whitespacefehler.
7. `git config user.name` und `git config user.email`: vorhandene Identität unverändert;
   öffentliche GitHub-noreply-Adresse. Keine Identitätsänderung.

Offizielle Webquellen sind mit Abrufdatum und direkten Links in PARITAET dokumentiert.
Kein phpMyAdmin-Dienst, Produktdienst oder DB-Prozess gestartet. Keine laufenden Jobs,
keine echten Verbindungsdaten, SQL-Verläufe oder Fremdcodedateien aufgenommen.

## Teamauftrag und künftige Zuständigkeit

TEAM benennt Lead, aktuelle P2-/P3-/P1a-Fachrollen und ehrlich noch unbesetzte
Apple-/QA-Rollen. Slots werden sequenziell besetzt; Selbstprüfung ist keine unabhängige
Abnahme. Nach P1a und beauftragtem Fremdreview übernimmt dieser Worker Frontend & UX.
Produktimplementation erfolgt nur in eigenem Client-Worktree auf reviewed Vertragsbasis.

## Offene Grenzen und nächste Schritte

Keine praktischen phpMyAdmin-/DB-/Apple-/Containerprüfungen und keine Referenzfixtures
für Austauschformate. Optionen sind teilweise Familien; jedes genannte Familienmitglied
benötigt später einen eigenen parametrisierten Fall. Native Gegenstücke zu serverseitigen
Dateiverzeichnissen, externen Programmen, Themes und Auth-Integrationen sind offen.
Eine echte vollständige Sprach-/Options- und UI-Erhebung bleibt erforderlich.

Lead beauftragt unabhängigen lesenden Review des Diffs, integriert erst nach Befundklärung
und aktualisiert danach STATUS/ARBEITSPLAN. Die Produktanforderung Compose/Swarm muss
zusätzlich in den zentralen Entscheidungen/Betriebsaufträgen verankert werden.
Anschließend Referenz/DB-Ziele entscheiden und isolierte Referenzfixtures anlegen.
Commit dieses Pakets wird über Branch und Git-Historie identifiziert; keine Integration
oder Veröffentlichung durch diesen Worker vorgenommen.
