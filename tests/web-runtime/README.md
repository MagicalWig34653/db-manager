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
