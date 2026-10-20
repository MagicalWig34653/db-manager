# S1: synthetische Go-/Swift-Tresorinteroperabilität

Dieses getrennte Experiment verwendet ausschließlich öffentlich erkennbare
synthetische Daten. Kein Produktimport, keine echte Credentialdatei, kein
CloudKit-/Passkey-/Teamzugriff. Eigener Code `AGPL-3.0-only`;
[Drittanteile](LIZENZEN.md) behalten ihre Lizenzen.

## Prüfung

Im Hauptrepo:

```sh
python3 spikes/vault-interop/check.py
```

Das Skript prüft unveränderte vendorte C-Dateien gegen den Commit-/SHA-Lock,
führt Go-Race-/Vet- und Swift-Tests aus, baut getrennte Go-/Swift-Release-Codecs
in einem eigenen temporären Verzeichnis und entfernt dieses danach. Es erzeugt
je eine neue synthetische Datei mit beiden Implementierungen. Beide Reader
öffnen beide Writerdateien mit Passwort und unabhängigem Recoverygeheimnis.
Danach folgen beidseitige Negativfälle und die eingecheckten öffentlichen Vektoren.
Es werden ausschließlich Status/Zeitmessungen ausgegeben, keine Schlüssel.

```sh
python3 spikes/vault-interop/check.py --report /tmp/vault-check-report.json
```

`--go-binary`/`--swift-binary` erlauben gezieltes Prüfen bereits gebauter
synthetischer Testcodecs; dabei werden Build-/Unittests ausdrücklich übersprungen.
Der dokumentierte abschließende Lauf verwendet beide Optionen **nicht**.
Einmalige frühere lokale Vorproben mit vorhandenen Binärdateien sind kein Beleg
des endgültigen Quellstands.

Einzelprüfungen in `go` beziehungsweise `swift`:

```sh
go test -race -count=1 ./...
go vet ./...
swift test --scratch-path /tmp/vault-swift-check -j 4
swift build --scratch-path /tmp/vault-ipad-check \
  --triple arm64-apple-ios17.0-simulator \
  --sdk "$(xcrun --sdk iphonesimulator --show-sdk-path)" --target VaultFormat -j 4
```

Der iPad-Aufruf kompiliert Swift, CryptoKit-Anbindung und den C-Referenz-KDF.
Er startet kein Gerät und belegt weder Altgeräte-RAM-/Zeitverhalten noch eine
produktive App-Einrichtung. macOS-Ziel 14/iPadOS-Ziel 17; tatsächlich verwendete
SDK-/Toolchain und Ergebnisse im [Handoff](../../docs/handoffs/S1.md).

## Aufbau

- `go`: eigenes Go-Modul mit `x/crypto/argon2`, Standard-AES-GCM/HKDF/HMAC.
- `swift`: eigenes Swift-Paket mit CryptoKit/Security und unveränderten
  offiziellen Argon2-C-Referenzquellen. Es wird `ref.c` statt SIMD-`opt.c`
  gebaut; `ARGON2_NO_THREADS` serialisiert die vier algorithmischen Lanes.
  Die Ausgabe bleibt identisch zu Go mit vier Lanes; keine Aussage über vier
  aktive C-Workerthreads. Kein selbst implementierter Argon2-/AES-/Hashalgorithmus.
- `check.py`: temporärer Build-/Vektorrundlauf ohne neue globale Tools.
- `results/interop.json`: tatsächlich aufgezeichnete Status-/Zeitwerte einer
  Maschine. Prozess-Walltime enthält Start und Parsing, Emit-Zeit umfasst KDF,
  Verschlüsselung und synthetisches Schreiben. Keine isolierte KDF-, Altgeräte-
  oder p95-/Lastbenchmark behauptet.

CLI `emit <verzeichnis>` erstellt `vault.json` und die ausdrücklich synthetische
`synthetic-secrets.json` nebeneinander. **Das ist ausschließlich ein Testvektor**:
ein Produkt darf Masterpasswort/Recoverycode niemals daneben persistieren.
`verify <vault> <synthetic-secrets> password|recovery [minimumGeneration]` nimmt
keine Geheimnisse über Prozessargumente entgegen und gibt nur Status aus.
Diese CLI ist keine Benutzeroberfläche und soll nie mit echten Geheimnissen
verwendet werden.

Das Format, kanonische JSON, alle Grenzen, binäre AAD/HMAC-Kontexte und
Verlust-/Replaygrenzen stehen normativ im [Formatvertrag](../../docs/security/VAULT-FORMAT-V1.md).
S2 benötigt zusätzlich atomare Speicherung, Einrichtungs-/Recovery-Rücklesetest,
Sperr-/Schlüssellebenszyklus, konservative Nutzungszähler und unabhängige Abnahme.
