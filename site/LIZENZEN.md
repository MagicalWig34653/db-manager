# Lizenzen und Buildherkunft

Eigene Website, Design, Texte und Skripte: **AGPL-3.0-only**.
[Original-Lizenztext](licenses/AGPL-3.0-only.txt).
Quellen, Buildskripte und Locks sind öffentlich im [Hauptrepository](https://github.com/MagicalWig34653/db-manager/tree/main/site).
Die Produktaufnahmen zeigen ausschließlich eigene synthetische Beispieldaten.
[Exakte Screenshotherkunft](assets/screenshots/provenance.json).

## Abhängigkeiten

MkDocs rendert die kanonischen Markdownquellen beim Build. Die Website verwendet
ein eigenes Theme und eine eigene lokale Suche. Ausgeliefert werden keine
JavaScript-Bibliotheken aus den Build-/Testabhängigkeiten, keine externen Fonts,
Tracker oder CDN-Ressourcen. Build- und Testwerkzeuge behalten ihre Originallizenzen.
Chromium ist ein separat installiertes Testwerkzeug und kein Websiteasset.
GitHub Actions laufen als Build-/Deploymentinfrastruktur und werden per Commit gepinnt.

Pythonpakete sind vollständig mit Versionen und SHA-256-Artefakthashes in
`site/requirements.lock` gepinnt; npm-Testpakete in `site/package-lock.json`
mit Integritätshashes. Originaltexte wurden unverändert aus genau diesen
installierten Distributionen übernommen. Inventare enthalten ihre Einzelhashes:
[Python](licenses/python-inventory.json), [npm](licenses/npm-inventory.json).

| Werkzeug | Version | Verwendung | Originaltexte |
| --- | --- | --- | --- |
| click | 8.5.0 | Build | [LICENSE.txt](licenses/python/click/8.5.0/licenses/LICENSE.txt) |
| ghp-import | 2.1.0 | Build | [LICENSE](licenses/python/ghp-import/2.1.0/LICENSE) |
| Jinja2 | 3.1.6 | Build | [LICENSE.txt](licenses/python/Jinja2/3.1.6/licenses/LICENSE.txt) |
| Markdown | 3.11.2 | Build | [LICENSE.md](licenses/python/Markdown/3.11.2/licenses/LICENSE.md.txt) |
| MarkupSafe | 3.0.4 | Build | [LICENSE.txt](licenses/python/MarkupSafe/3.0.4/licenses/LICENSE.txt) |
| mergedeep | 1.3.4 | Build | [LICENSE](licenses/python/mergedeep/1.3.4/LICENSE) |
| mkdocs | 1.6.1 | Build | [LICENSE](licenses/python/mkdocs/1.6.1/licenses/LICENSE) |
| mkdocs-get-deps | 0.2.2 | Build | [LICENSE.md](licenses/python/mkdocs-get-deps/0.2.2/licenses/LICENSE.md.txt) |
| packaging | 26.3 | Build | [LICENSE](licenses/python/packaging/26.3/licenses/LICENSE), [LICENSE.APACHE](licenses/python/packaging/26.3/licenses/LICENSE.APACHE), [LICENSE.BSD](licenses/python/packaging/26.3/licenses/LICENSE.BSD) |
| pathspec | 1.1.1 | Build | [LICENSE](licenses/python/pathspec/1.1.1/licenses/LICENSE) |
| platformdirs | 4.13.5 | Build | [LICENSE](licenses/python/platformdirs/4.13.5/licenses/LICENSE) |
| python-dateutil | 3.0.0 | Build | [LICENSE](licenses/python/python-dateutil/3.0.0/licenses/LICENSE) |
| PyYAML | 6.0.3 | Build | [LICENSE](licenses/python/PyYAML/6.0.3/licenses/LICENSE) |
| pyyaml_env_tag | 1.1 | Build | [LICENSE](licenses/python/pyyaml_env_tag/1.1/licenses/LICENSE) |
| six | 1.17.0 | Build | [LICENSE](licenses/python/six/1.17.0/LICENSE) |
| tzdata | 2026.5 | Build | [LICENSE](licenses/python/tzdata/2026.5/licenses/LICENSE), [LICENSE_APACHE](licenses/python/tzdata/2026.5/licenses/licenses/LICENSE_APACHE) |
| watchdog | 6.0.0 | Build | [COPYING](licenses/python/watchdog/6.0.0/COPYING), [LICENSE](licenses/python/watchdog/6.0.0/LICENSE) |
| @axe-core/playwright | 4.14.0 | Browser-/Accessibilitytests | [LICENSE](licenses/npm/@axe-core/playwright/4.14.0/LICENSE) |
| @playwright/test | 1.64.1 | Browser-/Accessibilitytests | [LICENSE](licenses/npm/@playwright/test/1.64.1/LICENSE), [NOTICE](licenses/npm/@playwright/test/1.64.1/NOTICE) |
| axe-core | 4.14.0 | Browser-/Accessibilitytests | [LICENSE](licenses/npm/axe-core/4.14.0/LICENSE), [LICENSE-3RD-PARTY.txt](licenses/npm/axe-core/4.14.0/LICENSE-3RD-PARTY.txt) |
| playwright | 1.64.1 | Browser-/Accessibilitytests | [LICENSE](licenses/npm/playwright/1.64.1/LICENSE), [NOTICE](licenses/npm/playwright/1.64.1/NOTICE), [ThirdPartyNotices.txt](licenses/npm/playwright/1.64.1/ThirdPartyNotices.txt), [expect.js.LICENSE](licenses/npm/playwright/1.64.1/lib/matchers/expect.js.LICENSE), [babelBundle.js.LICENSE](licenses/npm/playwright/1.64.1/lib/transform/babelBundle.js.LICENSE), [esmLoader.js.LICENSE](licenses/npm/playwright/1.64.1/lib/transform/esmLoader.js.LICENSE) |
| playwright-core | 1.64.1 | Browser-/Accessibilitytests | [LICENSE](licenses/npm/playwright-core/1.64.1/LICENSE), [NOTICE](licenses/npm/playwright-core/1.64.1/NOTICE), [ThirdPartyNotices.txt](licenses/npm/playwright-core/1.64.1/ThirdPartyNotices.txt), [serverRegistry.js.LICENSE](licenses/npm/playwright-core/1.64.1/lib/serverRegistry.js.LICENSE), [utilsBundle.js.LICENSE](licenses/npm/playwright-core/1.64.1/lib/utilsBundle.js.LICENSE), [webp_codec.LICENSE](licenses/npm/playwright-core/1.64.1/lib/webp_codec.LICENSE) |

## Hosting und Sicherheitsheader

Der Build setzt eine restriktive Content-Security-Policy als HTML-Metatag
(`script-src self`, ohne Inline-Skripte oder `unsafe-eval`) und eine
Referrer-Policy `no-referrer`. GitHub Pages kontrolliert die HTTP-Antwortheader;
`frame-ancestors`, HSTS oder `X-Content-Type-Options` werden hier nicht als
selbst gesetzte HTTP-Header behauptet. Die Seite enthält keine Anmeldung,
Verbindungsdaten oder Produkt-API und benötigt keine Cookies oder lokale Speicherung.

## Gepinnte CI-Infrastruktur

Diese offiziellen Actions laufen ausschließlich in GitHub Actions; ihr Quellcode
wird nicht in das Websiteartefakt kopiert. Commit und Original-Lizenzseite wurden
am 23.10.2026 über die jeweiligen offiziellen Repositories überprüft.

| Action | Commit | Originallizenz |
| --- | --- | --- |
| actions/checkout | `d23441a48e516b6c34aea4fa41551a30e30af803` | [MIT](https://github.com/actions/checkout/blob/d23441a48e516b6c34aea4fa41551a30e30af803/LICENSE) |
| actions/setup-python | `ece7cb06caefa5fff74198d8649806c4678c61a1` | [MIT](https://github.com/actions/setup-python/blob/ece7cb06caefa5fff74198d8649806c4678c61a1/LICENSE) |
| actions/setup-node | `249970729cb0ef3589644e2896645e5dc5ba9c38` | [MIT](https://github.com/actions/setup-node/blob/249970729cb0ef3589644e2896645e5dc5ba9c38/LICENSE) |
| actions/upload-pages-artifact | `7b1f4a764d45c48632c6b24a0339c27f5614fb0b` | [MIT](https://github.com/actions/upload-pages-artifact/blob/7b1f4a764d45c48632c6b24a0339c27f5614fb0b/LICENSE) |
| actions/configure-pages | `983d7736d9b0ae728b81ab479565c72886d7745b` | [MIT](https://github.com/actions/configure-pages/blob/983d7736d9b0ae728b81ab479565c72886d7745b/LICENSE) |
| actions/deploy-pages | `d6db90164ac5ed86f2b6aed7e0febac5b3c0c03e` | [MIT](https://github.com/actions/deploy-pages/blob/d6db90164ac5ed86f2b6aed7e0febac5b3c0c03e/LICENSE) |
