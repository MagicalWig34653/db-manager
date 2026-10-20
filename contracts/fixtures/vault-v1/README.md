# Öffentliche synthetische Tresorvektoren

Alle Dateien dieses Verzeichnisses sind **synthetische Testvektoren**, keine
Geheimnisse oder Zugangsdaten einer Person/einer echten Datenbank.
`go/` und `swift/` enthalten je eine vom jeweiligen Codec erzeugte Datei samt
öffentlich bekanntem Testpasswort, Recoverymaterial und erwarteten Klartextbytes.
Die Geheimnisdatei daneben ist ausschließlich für Testvektoren zulässig; ein
Produkt darf diese Struktur niemals als Speicherempfehlung übernehmen.

`aad.json` ist ein eigenständig deterministisch erzeugtes Bytebeispiel für alle
vier AEAD-Zwecke. Die beiden Unittest-Suiten prüfen denselben festen Entry-Key-
AAD-Vektor. Der volle mechanische Rundlauf unter `spikes/vault-interop/check.py`
prüft beide Richtungen und beide Entsperrwege, inklusive dieser Dateien.

Payloads decken Unicode/Emoji, eingebettetes NUL, leere Bytes und nichttextuelle
Bytes ab. Datensatz-IDs, Schlüssel, Salts und Nonces der verschlüsselten Beispiele
stammen aus den normalen kryptografischen Zufallsquellen. Dass Testschlüssel hier
öffentlich sind, erlaubt keine Aussage über Geheimhaltung dieser Beispielinhalte.
Kein produktiver SQL-Verlauf, keine echten Verbindungsdaten, kein Cloudkonto.

Normativ: [Formatvertrag](../../../docs/security/VAULT-FORMAT-V1.md),
[Schema](../../vault-v1.schema.json),
[ADR-Vorschlag](../../../docs/decisions/ADR-0005-vault-format.md).
