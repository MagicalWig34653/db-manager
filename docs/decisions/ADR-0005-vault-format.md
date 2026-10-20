# ADR-0005: Persönliches Tresorformat und Argon2id-Grundlage

Status: **akzeptiert als Formatgrundlage nach unabhängiger S1-QA**, keine produktive Speicherfreigabe.
Datum: 20.10.2026. Auf [P3a](../handoffs/P3a.md) aufbauend.

Leadentscheidung nach unabhängigem QA-Review von `1a4eb3c`/`03bd3f4`, integriert
als `7969f63`/`bc5dd4c`. Schema-SHA256
`b688656056fe2e23848ce17149bc817609c7369f230a3f49dd91f05db4d3f4dd`.
Kein bestätigter Reviewbefund; die folgenden S2-Produktgates gelten weiter.

## Kontext

Persönliche Zugangsdaten sollen optional verschlüsselt gespeichert werden, Apple
ohne Go-Server funktionieren und ein verlorenes Masterpasswort darf mit unabhängig
vorhandenem Recoverymaterial überbrückbar sein. Ein normaler Login ist kein
Entsperrweg. Vor Speicherung echter Geheimnisse fehlen gemeinsame Dateibytes und
tatsächliche Go-/Swift-Interoperabilität.

## Entscheidung

Den [Formatvertrag v1](../security/VAULT-FORMAT-V1.md) mit festem Argon2id-1.3-
Profil 64 MiB/t3/p4 und AES-256-GCM verwenden. Zufälliger Root verpackt zufällige
Datenschlüssel; separate Passwort- und Recoveryhülle verpacken den Root. HKDF
leitet Recovery- und Manifest-Schlüssel mit unterschiedlichen Kontexten ab.
Ein HMAC schützt den vollständigen Bestand gegen stilles Entfernen/Reihenfolgetausch;
AEAD-AAD bindet zusätzlich Identität, Epoche, Revision und Zweck.

Streng kanonisches, begrenztes ASCII-JSON und explizites binäres AAD/HMAC-Format
vermeiden sprachabhängige JSON-Schlüsselreihenfolge und uneindeutige akzeptierte
Eingaben. Nach typisiertem Decoding muss die kanonische Re-Encodierung bytegleich
sein. Das Schema allein ersetzt diese Parsingregeln nicht. KDF-Profil, Längen,
IDs, Anzahl und Dateigröße werden vor teurer Berechnung geprüft.

Go verwendet `x/crypto/argon2`, Swift CryptoKit und eine kleine Bindung an exakt
gepinnten offiziellen Argon2-C-Referenzcode. Die C-Primitiven bleiben unverändert;
portable `ref.c` vermeidet ungeprüfte SSE2-/arm64-Probleme fremder Wrapperpakete.
Vier algorithmische Lanes bleiben bei `ARGON2_NO_THREADS` erhalten, C berechnet
sie in dieser Untersuchung seriell. Go-/Swift-Ausgaben stimmen praktisch überein.

## Geprüfte Alternativen

- Reines Passwort-PBKDF2 wäre in Apple-APIs einfacher verfügbar, bietet aber nicht
  die gewählte speicherharte Argon2id-Eigenschaft. Kein Grund für eine stille
  Profilabschwächung; Argon2-C-Referenz lässt sich tatsächlich auf beiden Zielen bauen.
- Ungeprüfte reine Swift-Argon2-Implementierungen oder Wrapper mit beweglicher
  Upstream-Branchabhängigkeit wurden nicht als Vertrauensgrundlage aufgenommen.
- Nur AEAD je Eintrag erkennt kein ersatzloses Entfernen gültiger Einträge. Das
  Manifest bindet den gesamten Bestand; Generation allein wäre kein Integritätsschutz.
- Ein zusätzlich verschlüsseltes vollständiges Containerobjekt wäre möglich,
  würde aber pro Änderung sämtliche Nutzdaten neu verschlüsseln. Der vorgeschlagene
  Manifest-MAC hält Entry-Hüllen und gezielte spätere Updates getrennt prüfbar.
- iCloud-Keychain oder Passkey-PRF sind zusätzliche spätere Komfortwege, kein
  Ersatz für nachgewiesene unabhängige Passwort-/Recoverygrundlagen.

## Befunde, Folgen und Freigabegates

Separate [Go-/Swift-Codecs](../../spikes/vault-interop/README.md) verschlüsseln und
entschlüsseln tatsächlich gegenseitig synthetische Dateien. Unit-/Negativtests,
Originalnotices, Versions-/SHA-Locks und Messungen sind dokumentiert. Arm64-iPadOS-
17-Simulatorziel kompiliert KDF und CryptoKit; echte Geräteleistung/Altgeräte/Signing
sind nicht damit belegt. Keine externen Provider-/iCloud-/Passkey-Berechtigungen
wurden für S1 benötigt oder vorgetäuscht.

Vor S2 bleiben unabhängige QA/ADR-Akzeptanz und konkrete Produktpakete erforderlich:
Masterpasswort-/Recoveryeinrichtung mit Rückleseprüfung, atomare Dateien und Crash-
Recovery, gesperrte/entsperrte Sessionzustände, Updates/Rotation/Nutzungszähler,
Backuppolitik und Zielgeräte. Gültige alte Dateien bleiben ohne unabhängigen
aktuellen Anker gültig. Schlüsselneuverpackung löscht keine alten Kopien; komplette
Root-/Faktor-/DB-Credentialrotation kann erforderlich sein. Keine beweisbare
Speicher-/SSD-/Backup-Löschung und kein sofortiger KDF-Abbruch versprochen.

S1 ändert keine produktiven Go-/Swift-Appmodule und schreibt keine echten Secrets.
CloudKit, Passkeys, Teams und Server-Entsperrmodelle bleiben eigene Aufträge.

## Primärquellen

Abruf 20.10.2026:

- [RFC 9106: Argon2id und Parameter](https://www.rfc-editor.org/rfc/rfc9106.html)
- [Offizielle Argon2-Referenz](https://github.com/P-H-C/phc-winner-argon2/tree/f57e61e19229e23c4445b85494dbf7c07de721cb)
- [Go Argon2](https://pkg.go.dev/golang.org/x/crypto/argon2@v0.58.0)
- [Go AES-GCM-Zufallsnoncegrenze](https://pkg.go.dev/crypto/cipher#NewGCMWithRandomNonce)
- [RFC 5869: HKDF](https://www.rfc-editor.org/rfc/rfc5869)
