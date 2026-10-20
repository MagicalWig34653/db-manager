# Lizenzinventar der Treiberproben

Stand: 20.10.2026. Nur Untersuchungsabhängigkeiten, keine Freigabe für Produktverteilung.
Eigener Code: AGPL-3.0-only. Die Paketversionen werden durch Lockdateien festgehalten.
Die folgenden Dateinamen stammen tatsächlich aus den aufgelösten Quellen; sie ersetzen
nicht deren Lizenztexte. Bei Distribution müssen sämtliche Lizenz-/Notice-Texte einschließlich
eingebetteter C-Bibliotheken übernommen und der vollständige Graph erneut geprüft werden.

## Go

| Modul | Version | Lizenzkennung des Wurzeltexts | Lizenzdateien im Modulwurzelverzeichnis |
| --- | --- | --- | --- |
| `filippo.io/edwards25519` | `v1.2.0` | BSD-3-Clause | LICENSE |
| `github.com/davecgh/go-spew` | `v1.1.1` | offen | LICENSE |
| `github.com/dustin/go-humanize` | `v1.0.1` | MIT | LICENSE |
| `github.com/go-sql-driver/mysql` | `v1.10.1` | MPL-2.0 | LICENSE |
| `github.com/google/pprof` | `v0.0.0-20260802141513-ef3492d7dac3` | Apache-2.0 | LICENSE |
| `github.com/google/uuid` | `v1.6.0` | BSD-3-Clause | LICENSE |
| `github.com/hashicorp/golang-lru/v2` | `v2.0.7` | MPL-2.0 | LICENSE |
| `github.com/jackc/pgpassfile` | `v1.0.0` | MIT | LICENSE |
| `github.com/jackc/pgservicefile` | `v0.0.0-20240606120523-5a60cdf6a761` | MIT | LICENSE |
| `github.com/jackc/pgx/v5` | `v5.11.0` | MIT | LICENSE |
| `github.com/jackc/puddle/v2` | `v2.2.2` | MIT | LICENSE |
| `github.com/kr/pretty` | `v0.3.0` | offen | kein Wurzeltext im lokalen Modulcache; Prüfung offen |
| `github.com/mattn/go-isatty` | `v0.0.24` | MIT | LICENSE |
| `github.com/ncruces/go-strftime` | `v1.0.0` | MIT | LICENSE |
| `github.com/pmezard/go-difflib` | `v1.0.0` | BSD-2-Clause | LICENSE |
| `github.com/remyoudompheng/bigfft` | `v0.0.0-20230129092748-24d4a6f8daec` | BSD-3-Clause | LICENSE |
| `github.com/stretchr/objx` | `v0.1.0` | offen | kein Wurzeltext im lokalen Modulcache; Prüfung offen |
| `github.com/stretchr/testify` | `v1.11.1` | MIT | LICENSE |
| `golang.org/x/mod` | `v0.41.0` | BSD-3-Clause | LICENSE |
| `golang.org/x/sync` | `v0.23.0` | BSD-3-Clause | LICENSE |
| `golang.org/x/sys` | `v0.48.0` | BSD-3-Clause | LICENSE |
| `golang.org/x/text` | `v0.29.0` | BSD-3-Clause | LICENSE |
| `golang.org/x/tools` | `v0.50.0` | BSD-3-Clause | LICENSE |
| `gopkg.in/check.v1` | `v1.0.0-20201130134442-10cb98267c6c` | offen | kein Wurzeltext im lokalen Modulcache; Prüfung offen |
| `gopkg.in/yaml.v3` | `v3.0.1` | Apache-2.0 | LICENSE, NOTICE |
| `modernc.org/cc/v4` | `v4.29.7` | BSD-3-Clause | LICENSE |
| `modernc.org/ccgo/v4` | `v4.36.1` | BSD-3-Clause | LICENSE |
| `modernc.org/fileutil` | `v1.4.0` | BSD-3-Clause | LICENSE |
| `modernc.org/gc/v2` | `v2.6.5` | BSD-3-Clause | LICENSE |
| `modernc.org/gc/v3` | `v3.1.5` | BSD-3-Clause | LICENSE |
| `modernc.org/goabi0` | `v0.2.0` | BSD-3-Clause | LICENSE |
| `modernc.org/libc` | `v1.77.1` | BSD-3-Clause | LICENSE, LICENSE-3RD-PARTY.md |
| `modernc.org/mathutil` | `v1.7.1` | BSD-3-Clause | LICENSE |
| `modernc.org/memory` | `v1.12.1` | BSD-3-Clause | LICENSE, LICENSE-GO, LICENSE-LOGO, LICENSE-MMAP-GO |
| `modernc.org/opt` | `v0.2.0` | BSD-3-Clause | LICENSE |
| `modernc.org/sortutil` | `v1.2.1` | BSD-3-Clause | LICENSE |
| `modernc.org/sqlite` | `v1.60.1` | BSD-3-Clause | LICENSE, LICENSE-3RD-PARTY.md, LICENSE-SQLITE, LICENSE-SQLITE_VEC |
| `modernc.org/strutil` | `v1.2.1` | BSD-3-Clause | LICENSE |
| `modernc.org/token` | `v1.1.0` | BSD-3-Clause | LICENSE |

Direkte Treiber: go-sql-driver/mysql MPL-2.0; jackc/pgx MIT; modernc.org/sqlite BSD-3-Clause.
modernc enthält zusätzlich SQLite (Public Domain) und Erweiterungen mit eigenen Lizenzen.
Die vollständig fixierten transitiven Module stehen oben und in `go/go.sum`.

## Swift

| Paket | Version | Lizenzkennung des Wurzeltexts | Lizenz-/Notice-Dateien im Checkout-Wurzelverzeichnis |
| --- | --- | --- | --- |
| [mysql-nio](https://github.com/vapor/mysql-nio) | `1.9.2` | MIT | LICENSE |
| [postgres-nio](https://github.com/vapor/postgres-nio) | `1.33.1` | MIT | LICENSE, NOTICE.txt |
| [swift-algorithms](https://github.com/apple/swift-algorithms) | `1.2.1` | Apache-2.0 mit Swift-Runtime-Ausnahme | LICENSE.txt |
| [swift-asn1](https://github.com/apple/swift-asn1) | `1.7.3` | Apache-2.0 | LICENSE.txt, NOTICE.txt |
| [swift-async-algorithms](https://github.com/apple/swift-async-algorithms) | `1.1.7` | Apache-2.0 mit Swift-Runtime-Ausnahme | LICENSE.txt |
| [swift-atomics](https://github.com/apple/swift-atomics) | `1.3.1` | Apache-2.0 mit Swift-Runtime-Ausnahme | LICENSE.txt |
| [swift-collections](https://github.com/apple/swift-collections) | `1.7.3` | Apache-2.0 mit Swift-Runtime-Ausnahme | LICENSE.txt |
| [swift-crypto](https://github.com/apple/swift-crypto) | `4.5.2` | Apache-2.0 | LICENSE.txt, NOTICE.txt |
| [swift-log](https://github.com/apple/swift-log) | `1.16.1` | Apache-2.0 | LICENSE.txt, NOTICE.txt |
| [swift-metrics](https://github.com/apple/swift-metrics) | `2.12.0` | Apache-2.0 | LICENSE.txt, NOTICE.txt |
| [swift-nio](https://github.com/apple/swift-nio) | `2.104.0` | Apache-2.0 | LICENSE.txt, NOTICE.txt |
| [swift-nio-ssl](https://github.com/apple/swift-nio-ssl) | `2.37.5` | Apache-2.0 | LICENSE.txt, NOTICE.txt |
| [swift-nio-transport-services](https://github.com/apple/swift-nio-transport-services) | `1.28.0` | Apache-2.0 | LICENSE.txt |
| [swift-numerics](https://github.com/apple/swift-numerics) | `1.1.1` | Apache-2.0 mit Swift-Runtime-Ausnahme | LICENSE.txt |
| [swift-service-lifecycle](https://github.com/swift-server/swift-service-lifecycle) | `2.13.0` | Apache-2.0 | LICENSE.txt, NOTICE.txt |
| [swift-system](https://github.com/apple/swift-system) | `1.8.1` | Apache-2.0 mit Swift-Runtime-Ausnahme | LICENSE.txt |

MySQLNIO und PostgresNIO: MIT. Apple-Swift-Pakete überwiegend Apache-2.0 mit Runtime-Ausnahme;
maßgeblich ist der jeweilige Originaltext. SwiftNIO SSL/Swift Crypto enthalten BoringSSL und weitere
Drittanteile; deren Notices im Quellbaum sind zusätzlich erforderlich. SQLite3 wird aus dem Apple-SDK
gebunden und im Spike nicht mitkopiert. GRDB/NIOSSH werden nur untersucht, nicht eingebunden.
Containerbilder sind Werkzeuge der lokalen Untersuchung und werden nicht verteilt.

Primärquellen: [MySQL](https://github.com/go-sql-driver/mysql/blob/v1.10.1/LICENSE),
[pgx](https://github.com/jackc/pgx/blob/v5.11.0/LICENSE),
[modernc](https://github.com/modernc-org/sqlite),
[SwiftNIO SSL](https://github.com/apple/swift-nio-ssl/blob/main/NOTICE.txt).
