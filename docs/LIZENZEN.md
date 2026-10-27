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

SZ1 (Serverzustand) fügt **keine** Abhängigkeit hinzu: `github.com/go-sql-driver/mysql`
(MPL-2.0, oben inventarisiert, unverändert, `go.mod`/`go.sum` unverändert) dient auch dem
Zugriff auf die Serverzustands-MariaDB; die Feldverschlüsselung nutzt nur die Go-Standardbibliothek
(`crypto/aes`, `crypto/cipher`). Der MariaDB-Fixture-Container `mariadb:12.3` ist reines
Testwerkzeug und nicht Teil des Produktimages.

Fixtures: MySQL 8.4, MariaDB 12.3 und PostgreSQL 18.6 werden ausschließlich als isolierte
Testwerkzeuge ausgeführt und nicht in das Produktimage eingebunden. Ihre Images behalten
ihre jeweiligen Originalbedingungen. Eine erfolgreiche Treiberprobe ist kein Nachweis
für beliebige alte Datenbankversionen.


## Eingebettete Weboberfläche F2a/F2b/F3b

Der unveränderte Webbuild stammt aus dem unabhängig geprüften öffentlichen Commit
[3e422dd](https://github.com/MagicalWig34653/db-manager-web/tree/3e422ddd71bd32219f062c39adda468326c59584).
Eigener Webcode ist AGPL-3.0-only; die originalen Drittanbieterhinweise bleiben unter
[THIRD-PARTY-NOTICES.txt](../internal/webui/static/THIRD-PARTY-NOTICES.txt) vollständig
erhalten und werden als `/THIRD-PARTY-NOTICES.txt` ausgeliefert. Der originale
[Web-AGPL-Text](../internal/webui/static/LICENSE.txt) ist als `/LICENSE.txt` erreichbar.
[Quellpin, Hashmanifest und Image-Hinweise](WEBBUILD.md) dokumentieren die Herkunft.
Browserprüfwerkzeuge werden separat im [Testinventar](../tests/web-runtime/README.md)
erfasst und gehören nicht ins Laufzeitimage.


F3b übernimmt ausschließlich die abgenommene öffentliche Webquelle samt unveränderten
Originalnotices; kein neues npm-/Go-Modul. Web- und Tabellenvertrag-SHAs stehen im
Lock/Manifest. Der sichtbare Quellen-/Lizenzdialog verwendet dieselben lokalen Texte.

F4b aktualisiert den eingefrorenen Webquellpin auf die unabhängig freigegebene
Einzelzeilenoberfläche. Keine neuen Go-/npm-Abhängigkeiten; Original-AGPL-/MIT-/
Fremdnotices bleiben bytegleich aus dem Quellbuild. Manifest und Image enthalten
Quellpin/Hashprovenienz, sichtbarer Lizenzdialog verlinkt exakt den gebauten Commit.


F5b übernimmt ausschließlich den unabhängig akzeptierten F5-/F6-Quellstand mit
fünf unveränderten Artefakten und denselben Originalnotices. API1–5 sind im Lock
gebunden; weder Go- noch npm-Produktabhängigkeiten ändern sich. Neue Runtimeproben
verwenden die bereits inventarisierten Playwrightwerkzeuge, keine zusätzliche
Bibliothek im Laufzeitimage.


## Web-Editor W-P16 (CodeMirror 6, Vorabeintrag)

Der Webcode auf dem Branch `feat/w-p16-codemirror` des Web-Repositories (Commit
[3e422dd](https://github.com/MagicalWig34653/db-manager-web/tree/3e422ddd71bd32219f062c39adda468326c59584), Pin Bündel 6; Erstfassung auf `feat/w-p16-codemirror` `b0d0429`)
bindet CodeMirror 6 als eigenen, lazy geladenen Chunk ein (Entscheidung E8, vorab freigegeben). Seit Pin Bündel 6 gepinnt und
eingebettet; der Eintrag erfasste die Lizenzen vor dem Repin. Alle Pakete sind MIT-lizenziert; ihre Originaltexte
stehen in der Web-Datei `THIRD-PARTY-NOTICES.txt` und werden mit dem Build ausgeliefert. Kein neues Go-Modul.

| Paket | Version | Lizenz | Verwendung |
| --- | --- | --- | --- |
| `@codemirror/state` | `6.7.6` | MIT | direkt, Editorzustand |
| `@codemirror/view` | `6.43.15` | MIT | direkt, Editoransicht |
| `@codemirror/language` | `6.13.1` | MIT | direkt, Sprach- und Hervorhebungsinfrastruktur |
| `@codemirror/commands` | `6.11.1` | MIT | direkt, Tastaturbefehle, Verlauf |
| `@codemirror/search` | `6.7.2` | MIT | direkt, Suchen und Ersetzen |
| `@codemirror/autocomplete` | `6.20.4` | MIT | direkt, Vervollständigung |
| `@codemirror/lang-sql` | `6.10.0` | MIT | direkt, SQL-Dialekte (SQLite, PostgreSQL, MariaDB/MySQL) |
| `@lezer/highlight` | `1.2.5` | MIT | direkt, Hervorhebungs-Tags |
| `@codemirror/streamparser` | `6.0.0` | MIT | transitiv |
| `@lezer/common` | `1.5.4` | MIT | transitiv |
| `@lezer/lr` | `1.4.11` | MIT | transitiv |
| `@marijn/find-cluster-break` | `1.0.4` | MIT | transitiv |
| `crelt` | `1.0.7` | MIT | transitiv |
| `style-mod` | `4.1.4` | MIT | transitiv |
| `w3c-keyname` | `2.2.8` | MIT | transitiv |

Der SQL-Formatierer ist eigener Webcode (AGPL-3.0-only); `sql-formatter` (MIT, 3,1 MB entpackt) wurde bewusst nicht aufgenommen.
Gemessener Zuwachs des Precache-Budgets: +0,427 MiB (4,321 auf 4,748 MiB), 39 auf 40 Chunks (Grenzen 8 MiB und 64 Chunks).
Mit Pin Bündel 6 gemessen: Precache 4,91 MiB (Config-Store-Oberfläche eingeschlossen), 42 Chunks. `WEBBUILD.md`, Manifest und dieses Inventar stehen auf `f163862`.
