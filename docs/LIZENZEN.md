# Lizenzen

Eigener Projektcode: `AGPL-3.0-only`, vollständiger Text in [LICENSE](../LICENSE).
Keine zusätzliche Lizenz-Ausnahme. Drittanbieter behalten ihre Lizenzen.
Vor Aufnahme weiterer Bibliotheken: Name, Version, Herkunft,
Lizenz, Verteilungsform, notwendige Hinweise und Kompatibilitätsprüfung erfassen.
Apple-Vertriebsweg und konkrete Treiberlizenzen sind vor Release gesondert zu bewerten.

## Infrastrukturgrundlage P4a

| Bestandteil | Version/Herkunft | Lizenz und Verwendung |
| --- | --- | --- |
| Go-Toolchain und Standardbibliothek | Go 1.27.1, [Go-Projekt](https://go.dev/) | [BSD-3-Clause](https://go.dev/LICENSE); Container-Buildwerkzeug und statisch eingebundene Standardbibliothek; keine externen Go-Module in P4a |
| Build-Basisimage | `golang:1.27.1-alpine`, [offizielles Image](https://hub.docker.com/_/golang) | Go-/Alpine-Komponenten behalten ihre jeweiligen Lizenzen; nur Buildstufe, nicht komplettes Buildimage im Laufzeitimage |
| Laufzeit-Basisimage | `alpine:3.23`, [Alpine](https://alpinelinux.org/about/) | Unterschiedliche Paketlizenzen einschließlich MIT/GPL/BSD; `ca-certificates` zusätzlich installiert; konkrete Release-SBOM und erforderliche Hinweise/Quellangebote vor Verteilung prüfen |
| CI-Go-Setup | `actions/setup-go` v6.5.0, [Repository](https://github.com/actions/setup-go/tree/v6.5.0) | [MIT](https://github.com/actions/setup-go/blob/v6.5.0/LICENSE); Build-/Testwerkzeug, kein Produktbestandteil |
| Docker-in-Docker-Testimage | `docker:29.5.2-dind`, [offizielles Image](https://hub.docker.com/_/docker) | Docker-/Alpine-Komponenten unter ihren jeweiligen Lizenzen; ausschließlich isolierter Testdaemon, kein Produktbestandteil |

Tags fixieren Versionen, jedoch keine unveränderlichen Image-Digests. Das ist eine
offene Release-/Lieferkettenaufgabe. Diese Bestandsliste ersetzt keine vollständige
Laufzeitimage-SBOM oder Prüfung der GPL-Quellbereitstellung bei Imageverteilung.
