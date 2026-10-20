# Echte Web-Laufzeitprüfung

Dieses Verzeichnis enthält ausschließlich Testwerkzeuge und synthetische SQL-Fixtures.
Start über `python3 scripts/test-web-integration.py` im Hauptrepo; die Tests benötigen
die vom Orchestrator privat erzeugte Konfiguration. Keine Mock-API und keine echten
Zugangsdaten. Node/npm und Chromium sind keine Bestandteile des Produktimages.

`package-lock.json` fixiert `@playwright/test`, `playwright` und `playwright-core`
auf 1.64.1, jeweils [Apache-2.0](https://github.com/microsoft/playwright/blob/v1.64.1/LICENSE).
Die durch Playwright installierte Chromium-Testbinärdatei behält ihre Chromium-/Dritt-
Lizenzen; sie wird ausschließlich lokal oder im CI für Tests heruntergeladen.
Eigene Fixtures und Tests: AGPL-3.0-only.

Die Integration verwendet einen Worker, `npm ci` und die offizielle Chromium-Installation:
[Playwright CI](https://playwright.dev/docs/ci). Serverbeobachtung und Fixture-Anlage laufen
über echte Anfragen, gemäß [APIRequestContext](https://playwright.dev/docs/api/class-apirequestcontext).
Quellen am 20.10.2026 geprüft. Traces, Videos und Screenshots sind ausgeschaltet,
damit keine im Dialog eingegebenen kurzlebigen Zugangsdaten als Artefakte verbleiben.


F3b prüft fünf Fälle im tatsächlichen Go-Embed: SQL/Cancel/Session jeweils SQLite und
PostgreSQL-TLS, Tabellen/Paging/Filter/Metadaten einschließlich QA-F3-01 jeweils für
beide Engines sowie den sichtbaren Quellen-/Original-Lizenzdialog unter Server-CSP.
Die Tabellenregression wurde aus dem festen geprüften öffentlichen
[Webquellcommit](https://github.com/MagicalWig34653/db-manager-web/tree/6c6eb33c93eed355a10cd114f61ce27be7f3f55e)
übernommen und ausschließlich auf die privaten Root-Fixtureangaben angepasst.
Die PostgreSQL-Startbarriere in `scripts/web_test_support.py` beweist explizit,
dass erfolgreiche Unix-Socket-Readiness während des Bootstraps kein fertiger TCP-
Server ist. Erst endgültige TCP-Bereitschaft und positive verify-full-Verbindung
ermöglichen Setup/Browsertests; ein falscher Hostname wird erst anschließend geprüft.
