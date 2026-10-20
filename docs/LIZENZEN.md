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

## Produktabhängigkeiten B1

`go.mod`/`go.sum` fixieren diese tatsächlich in das Go-Programm eingebundenen Module.
Originaltexte stehen unter `licenses/`; eigener Adaptercode bleibt AGPL-3.0-only.

| Modul | Version | Ursprungslizenz / Originaltext |
| --- | --- | --- |
| `filippo.io/edwards25519` | `v1.2.0` | [BSD-3-Clause](../licenses/filippo.io-edwards25519.txt) |
| `github.com/go-sql-driver/mysql` | `v1.10.1` | [MPL-2.0](../licenses/github.com-go-sql-driver-mysql.txt) |
| `github.com/jackc/pgpassfile` | `v1.0.0` | [MIT](../licenses/github.com-jackc-pgpassfile.txt) |
| `github.com/jackc/pgservicefile` | `v0.0.0-20240606120523-5a60cdf6a761` | [MIT](../licenses/github.com-jackc-pgservicefile.txt) |
| `github.com/jackc/pgx/v5` | `v5.11.0` | [MIT](../licenses/github.com-jackc-pgx-v5.txt) |
| `github.com/jackc/puddle/v2` | `v2.2.2` | [MIT](../licenses/github.com-jackc-puddle-v2.txt) |
| `github.com/ncruces/go-sqlite3` | `v0.35.6` | [MIT](../licenses/github.com-ncruces-go-sqlite3.txt) |
| `github.com/ncruces/go-sqlite3-wasm/v6` | `v6.3.35304` | [MIT-0](../licenses/github.com-ncruces-go-sqlite3-wasm-v6.txt) |
| `github.com/ncruces/julianday` | `v1.0.0` | [MIT](../licenses/github.com-ncruces-julianday.txt) |
| `golang.org/x/sync` | `v0.23.0` | [BSD-3-Clause](../licenses/golang.org-x-sync.txt) |
| `golang.org/x/sys` | `v0.48.0` | [BSD-3-Clause](../licenses/golang.org-x-sys.txt) |
| `golang.org/x/text` | `v0.42.0` | [BSD-3-Clause](../licenses/golang.org-x-text.txt) |

Der generierte SQLite-Anteil behält seine Ursprungslizenzen. SQLite selbst ist
[Public Domain](https://sqlite.org/copyright.html); ncruces-go-sqlite3-wasm kennzeichnet
nur zusätzliche eigene Bestandteile als MIT-0. Keine optionalen Fremdextensions importiert.
MySQL-Driver bleibt unverändert MPL-2.0; seine Originaldateien wurden nicht umgeschrieben.
Eine vollständige Release-SBOM einschließlich Laufzeitimage, Drittanteilen und notwendigen
Quellbereitstellungs-/Lizenzpflichten bleibt vor Distribution zu prüfen. Dieses Inventar
ist keine juristische Distributionsfreigabe.

Fixtures: MySQL 8.4, MariaDB 12.3 und PostgreSQL 18.6 werden ausschließlich als isolierte
Testwerkzeuge ausgeführt und nicht in das Produktimage eingebunden. Ihre Images behalten
ihre jeweiligen Originalbedingungen. Eine erfolgreiche Treiberprobe ist kein Nachweis
für beliebige alte Datenbankversionen.
