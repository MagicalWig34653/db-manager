# S1-Lizenz- und Herkunftsinventar

Eigene Binding-/Format-/Testdateien: `AGPL-3.0-only`. Vendorte Quellen wurden
nicht unter diese Lizenz umgeschrieben. Keine Produktmodule im Hauptrepo geändert.

| Anteil | Exakter Stand/Herkunft | Lizenz und Verwendung |
| --- | --- | --- |
| Go-Toolchain/Standardbibliothek | geprüft Go 1.27.1, [Go-Projekt](https://go.dev/), [Originaltext](licenses/go-LICENSE) | BSD-3-Clause; AES-GCM, HMAC, HKDF, Zufall, JSON; kein eigener Primitivecode |
| `golang.org/x/crypto` | v0.58.0, [Argon2-Paket](https://pkg.go.dev/golang.org/x/crypto/argon2@v0.58.0), `go/go.sum` | BSD-3-Clause, [unveränderter Originaltext](licenses/golang-x-crypto-LICENSE); Argon2id |
| `golang.org/x/sys` | v0.49.0, transitive CPU-/Plattformabhängigkeit, `go/go.sum` | BSD-3-Clause, [Originaltext](licenses/golang-x-sys-LICENSE) |
| Offizielle Argon2-C-Referenz | [P-H-C/phc-winner-argon2](https://github.com/P-H-C/phc-winner-argon2/tree/f57e61e19229e23c4445b85494dbf7c07de721cb), Commit `f57e61e19229e23c4445b85494dbf7c07de721cb` | Wahlweise CC0-1.0 oder Apache-2.0; hier Apache-2.0 gewählt. Beide unveränderten Texte in [Upstream-LICENSE](swift/Vendor/argon2/LICENSE); sämtliche vorhandenen Datei-Copyrights beibehalten |
| Apple CryptoKit/Security/Foundation | Systemframeworks aus vorhandener Xcode-27.1-/Swift-6.4-Toolchain | Apple-SDK-/Runtime-Bedingungen; Frameworks nicht vendort und nicht im Repository weiterverteilt |
| Swift/Swift Testing | vorhandene Swift-6.4-Toolchain | Swift-/Apple-Bedingungen; kein externes Swift-Testpaket aufgenommen |

Die 14 übernommenen Argon2-Dateien sind einschließlich LICENSE bytegleich im
[Upstream-Lock](swift/Vendor/argon2/upstream-lock.json) mit SHA-256 inventarisiert.
`check.py` prüft jeden Hash. Originaler Quellstand und Buildauswahl werden getrennt:
die eigene `Package.swift` kompiliert die portable Referenz und deaktiviert Threads,
ändert aber keine vorgelagerte Kryptoprimitive. Kein Swift-`Package.resolved`, weil
das Paket keine entfernten Swift-Paketabhängigkeiten auflöst; der Vendor-Lock ist
die feste C-Herkunftsbindung. Das Go-Modul besitzt eigene `go.mod`/`go.sum`.

Originale fremde Lizenz-/Quelltexte können Whitespace enthalten, den ein pauschaler
Diffcheck beanstandet; nicht zur kosmetischen Bereinigung umschreiben. Hashgleichheit
ist hier der Übernahmenachweis. Diese Bestandsliste und synthetische Tests sind keine
allgemeine Rechts-/Lieferketten- oder externe Kryptografieprüfung. Releasehinweise,
signierter Vertrieb und Einbindung in tatsächliche Produktmodule bleiben vor S2
gesondert zu prüfen.
