# Sicherheits-Spike P3a

Stand: 20.10.2026. Ausschließlich synthetische Daten, kein produktiver Tresor,
kein Netzwerkzugriff durch den Spike und keine gespeicherten Zugangsdaten.
Eigener Code: `AGPL-3.0-only` gemäß [Projektlizenz](../../LICENSE).

## Inhalt und Abhängigkeiten

`envelope_test.go` verwendet Go-Standardbibliothek: AES-256-GCM über
`crypto/cipher.NewGCMWithRandomNonce`, Zufallsbytes aus `crypto/rand`, feste
Go-Structs als lokale AAD-Testdarstellung. Keine selbst entwickelte
Kryptoprimitive und kein Drittanbieter-Go-Modul. Go-Standardbibliothek bleibt
[BSD-3-Clause](https://go.dev/LICENSE). `go.mod` verlangt mindestens Go 1.24 wegen
der verwendeten Zufallsnonce-API. Das ist das Minimum dieses Experiments, keine
Toolchainfestlegung für das Produkt.

`prf-api.swift` nutzt Apples vorinstallierte Systemframeworks
`AuthenticationServices` und `Foundation` unter den jeweiligen Apple-SDK-
Bedingungen, keine vendorten SDK-Dateien. Die Probe erzeugt keine Passkeys und
führt keine Autorisierung aus. Sie kompiliert Plattform-PRF mit Availability
Guard ab macOS 15/iOS 18 und Security-Key-Registration-PRF ab 26.4.

## Prüfung

Im Verzeichnis `spikes/security`:

```sh
go test -race -count=1 -v ./...
go vet ./...
swiftc -typecheck -target arm64-apple-macosx15.0 prf-api.swift
```

Go-Tests prüfen erfolgreichen Rundlauf, frische Schlüssel/Nonces als
unterschiedliche Ausgaben, jedes manipulierte Byte und jede Trunkierung beider
Ciphertexte, falsche Root-/Kontextwerte und Zweckvertauschung. Ein eigener Test
zeigt, dass gültige alte Ciphertexte mit ihrem alten Kontext erneut akzeptiert
werden: AEAD liefert allein keinen Schutz vor vollständigem gültigem Rollback.
Ein vertrauenswürdiger neuer Revisionskontext weist dieselbe alte Hülle ab.

Geprüfte Toolchain: Go 1.27.1 auf Darwin/arm64 und Apple Swift 6.4 mit macOS-SDK
der installierten Xcode-Toolchain. Die lokale öffentliche Framework-
Swiftinterface deklarierte die untersuchten API-Untergrenzen; der macOS-15-
Typecheck prüft die Guards, ohne ein macOS-15-Gerät auszuführen. Exakte Ergebnisse
und offene Prüfungen im [Handoff](../../docs/handoffs/P3a.md).

## Grenzen

Nicht enthalten: Passwort-KDF, Root-Hüllen für Masterpasswort/Recovery/PRF,
dauerhafte Dateien, atomare Migration, Lizenz-/Sicherheitsauswahl einer Argon2-
Bibliothek, CloudKit, Keychainzugriffe, Interoperabilität, echte Geräte, UI,
Teamfreigaben, Docker- oder Swarm-Ausführung. Keine Speicherlöschgarantie. Kein
Produktimportpfad und kein gemeinsamer Serialisierungsvertrag.

Die Go-API begrenzt die sichere Zahl zufälliger Nonces pro Schlüssel auf höchstens
2^32 Nachrichten [Dokumentation](https://pkg.go.dev/crypto/cipher#NewGCMWithRandomNonce).
Der Spike führt nur kleine Tests aus und implementiert keinen dauerhaften
Schlüsselzähler. Das spätere Produkt benötigt konservative Rotation und eine
über alle Geräte konsistente Strategie, bevor es diese API dauerhaft nutzt.
Auch bestandene Tests stellen keine unabhängige Kryptografieprüfung dar.
