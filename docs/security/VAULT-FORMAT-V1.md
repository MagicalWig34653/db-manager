# Persönliches Tresorformat v1

Stand: 20.10.2026. **Reviewkandidat S1**, keine produktive Speicherfreigabe.
[Schema](../../contracts/vault-v1.schema.json),
[synthetische Vektoren](../../contracts/fixtures/vault-v1/README.md),
[Interoperabilitätsprobe](../../spikes/vault-interop/README.md),
[ADR-Vorschlag](../decisions/ADR-0005-vault-format.md).
Die [P3-Vertrauensgrenzen](BEDROHUNGSMODELL.md) bleiben maßgeblich.

## Zweck und unveränderliche Suite

Ein persönlicher, lokal entsperrter Tresor besitzt einen zufälligen 256-Bit-Root-
Schlüssel. Jeder Datensatz bekommt je Revision einen neuen zufälligen 256-Bit-
Datenschlüssel. AES-256-GCM verschlüsselt die Nutzdaten unter diesem Schlüssel
und verpackt ihn unter dem Root. Zwei unabhängige Hüllen verpacken denselben Root:
eine unter dem Masterpasswortschlüssel, eine unter einem Schlüssel aus einem
zufälligen 256-Bit-Wiederherstellungsgeheimnis. Alle GCM-Operationen nutzen frische
96-Bit-Zufallsnonces und vollständige 128-Bit-Tags; keine gekürzten Tags.

Für Version 1 ist die Suite fest: Argon2id 1.3 (`version=19`), 65.536 KiB
Gesamtspeicher, drei Durchläufe, vier Lanes, 16-Byte-Salt und 32-Byte-Ausgabe.
Dies ist die zweite Referenzkonfiguration aus [RFC 9106](https://www.rfc-editor.org/rfc/rfc9106.html#section-4).
Der erste RFC-Vorschlag benötigt 2 GiB und passt nicht ungeprüft zum mobilen Ziel.
Andere Parameter oder Algorithmen sind in v1 ungültig; keine Passwortableitung
mit ungeprüften Dateiwerten, keine stille Abschwächung für langsame Geräte.
Eine spätere Änderung benötigt explizite Format-/Profilentscheidung und Tests.

Passwörter werden als unveränderte UTF-8-Bytes mit 1 bis 1.024 Bytes verarbeitet,
einschließlich Leerzeichen und eingebettetem NUL. Kein Trimmen, keine Unicode-
Normalisierung und keine automatische Groß-/Kleinschreibung. Die Mindestlänge
des Codecs ist keine Stärkeempfehlung: S2 muss starke Masterpasswörter verständlich
anfordern. Verschiedene Unicode-Normalformen ergeben unterschiedliche Schlüssel.

Die Wiederherstellung besteht aus genau 32 zufälligen Bytes, als kanonisches
Standard-Base64 dargestellt. Sie wird getrennt vom Tresor aufbewahrt. Es gibt
noch keine produktive Eingabe-/Druck-/Prüfsummenoberfläche. Das Format bietet
genau eine Passwort- und eine Recoveryhülle; Passkey-, Geräte- und Teamhüllen
sind nicht stillschweigend vorgesehen.

## Transport, Grenzen und Parsing vor Kryptografie

Die Datei ist **kanonisches ASCII-JSON ohne Whitespace, BOM, Escapes oder Newline**:
Objektschlüssel lexikografisch nach ASCII sortiert, Zahlen als positive dezimale
Integer ohne führende Nullen. Arrays behalten ihre Reihenfolge. Alle vorkommenden
Stringwerte sind feste ASCII-Bezeichner, Kleinbuchstaben-Hex-IDs oder Base64;
es besteht kein Bedarf für JSON-Escapes. Veränderte Darstellung wird abgewiesen,
auch wenn sie ein allgemeiner JSON-Parser als gleichwertig betrachtet.

Beide Codecs begrenzen den Dateiinput vor JSON-Decoding auf 8 MiB, prüfen ASCII,
verbotene Backslashes und höchstens vier Verschachtelungsebenen. Nach typisiertem
Decoding müssen die erneut kanonisch serialisierten Bytes exakt übereinstimmen.
Damit sind unbekannte/fehlende/doppelte Schlüssel, Nullwerte, `1.0`, zusätzliche
Dokumente und beliebige Feldschreibweisen ungültig. Diese Regel ist zusätzlicher
Vertrag zum JSON-Schema, das allein keine kanonischen Dateibytes beweist.

Danach folgen alle strukturellen Prüfungen, **vor Argon2/HKDF/AES**:

- `format="db-manager-vault"`, `version=1`; Epoch, Generation und Revisionen
  ganzzahlig von 1 bis 4.294.967.295. Überlauf wird abgewiesen, nie umgebrochen.
- Tresor-/Datensatz-IDs: genau 16 Bytes, dargestellt als 32 kleine Hexzeichen.
- Genau das oben genannte Argon2id-Profil und ein kanonisches 16-Byte-Salt.
- Nonces genau 12 Bytes, Root-/Datenschlüsselciphertexte genau 48 Bytes
  einschließlich Tag, Manifest-MAC genau 32 Bytes.
- Höchstens 64 Datensätze, eindeutige IDs und keine doppelte `key_nonce` unter
  demselben Root; Nutzdaten höchstens 65.536 Bytes je Eintrag. Der verschlüsselte
  Payload besteht aus 0 bis 65.536 Bytes Ciphertext plus 16-Byte-Tag.
- Kanonisches gepaddetes RFC-4648-Standard-Base64, keine Zeilenumbrüche,
  URL-Alphabetvarianten oder nichtkanonischen Restbits.

Große oder tief verschachtelte falsche Dateien werden somit vor teurer Ableitung
abgewiesen. Das ist kein vollständiger Prozess-Heap-/DoS-Nachweis. S2 muss parallele
Entsperrvorgänge begrenzen. Die verwendeten KDF-APIs sind nicht kooperativ abbrechbar;
ein abgebrochener UI-Auftrag darf das Ergebnis des begrenzten laufenden KDF-Aufrufs
verwerfen, aber keinen sofortigen Berechnungsstopp behaupten.

## Binäre Kontexte und Schlüsselableitung

`||` bedeutet Bytekonkatenation, `BE32(x)` einen vorzeichenlosen Vierbytewert in
Netzwerkreihenfolge. IDs/Salts sind die decodierten Bytes, keine Textdarstellung.

```text
COMMON(p) = ASCII("DBMV1") || 00 || BYTE(p) || vault_id[16] || BE32(epoch)
PASSWORD_CONTEXT = BE32(65536) || BE32(3) || BE32(4) || salt[16]
ENTRY_CONTEXT = entry_id[16] || BE32(revision)

password_root_AAD = COMMON(1) || PASSWORD_CONTEXT
recovery_root_AAD = COMMON(2)
entry_key_AAD = COMMON(3) || ENTRY_CONTEXT
entry_payload_AAD = COMMON(4) || ENTRY_CONTEXT
```

Der gemeinsame Kopf hat exakt 27 Bytes. Die vollständigen festen AAD-Beispiele
stehen in [aad.json](../../contracts/fixtures/vault-v1/aad.json); beide Codecs
prüfen zusätzlich denselben festen binären Entry-Key-Vektor.

Der Passwort-Hüllenschlüssel ist direkt die 32-Byte-Argon2id-Ausgabe. Weitere
Schlüssel verwenden HKDF-SHA-256 mit `salt=vault_id[16]`, Länge 32:

```text
recovery_KEK = HKDF(recovery_secret, salt,
                   ASCII("DBMV1") || 00 || ASCII("recovery") || 00 || BE32(epoch))
manifest_key = HKDF(root_key, salt,
                   ASCII("DBMV1") || 00 || ASCII("manifest") || 00 || BE32(epoch))
```

`wrapped_root`/`wrapped_key`/`payload` enthalten jeweils Ciphertext gefolgt vom
16-Byte-GCM-Tag. Die Nonce liegt im eigenen Feld. Root und Datenschlüssel werden
nie als Klartext in der Datei gespeichert. Ein normaler Login liefert keinen
dieser Schlüssel.

## Vollständiger Manifest-Schutz

Nur einzelne AEAD-Datensätze würden stilles Entfernen gültiger Datensätze nicht
erkennen. Deshalb schützt `mac` den gesamten Inhalt als HMAC-SHA-256 unter dem
separat abgeleiteten Manifest-Schlüssel. Der binäre HMAC-Input lautet exakt:

```text
COMMON(5) || BE32(generation) || PASSWORD_CONTEXT ||
password_nonce[12] || password_wrapped_root[48] ||
recovery_nonce[12] || recovery_wrapped_root[48] || BE32(entry_count) ||
für jeden Eintrag in Arrayreihenfolge:
  entry_id[16] || BE32(revision) || key_nonce[12] || wrapped_key[48] ||
  payload_nonce[12] || BE32(payload_ciphertext_length) || payload_ciphertext
```

Damit sind auch Slotmetadaten, Anzahl, Reihenfolge, Nonces und alle Ciphertexte
authentifiziert. Bei Entsperren erst Root-Hülle authentifiziert entschlüsseln, dann
Manifest-MAC konstantzeitlich mit der Bibliotheksfunktion prüfen, dann Entry-Key-
und Payload-Hüllen entschlüsseln. **Kein Eintrag wird vor erfolgreichem gesamten
Rundlauf zurückgegeben.** Fehler geben keinen Teilklartext und keine Rohfehler
mit Geheimnissen zurück. Ein beschädigter Slot kann die Gesamtintegrität zerstören;
Recovery ersetzt keine unbeschädigte Sicherung der Ciphertexte.

## Aktualität, Rotation, Verlust und Löschung

`generation` wächst bei jedem logisch gespeicherten Dateistand; `revision` bei
jeder Datensatzänderung; `epoch` bei Rootwechsel. Änderungen benötigen neue
Datenschlüssel/Nonces und einen neuen Manifest-MAC. Die vollständige persistente
Update-/Rotations-API und atomare Dateischreibfolge gehören zu S2, nicht zum Spike.
Überläufe erfordern eine neue ausdrücklich migrierte Identität statt Zählerrücksprung.

Ein gültiger alter Tresor bleibt ohne unabhängigen aktuellen Anker gültig. Der
Codec akzeptiert einen vom Aufrufer **vertrauenswürdig** bezogenen Mindestwert
`minimumGeneration`; damit wird eine ältere Datei vor KDF abgewiesen. Ein neues
Gerät ohne solchen Anker oder der gemeinsam zurückgespielte Anker kann vollständiges
Rollback nicht erkennen. Eine allein im gleichen Tresor gespeicherte Generation
ist kein Rollbackschutz. Neues Gerät/CloudKit sind noch nicht umgesetzt.
S2 muss einen solchen Anker stets einer unabhängig gepinnten Tresor-ID zuordnen
und diese ID vor Entsperren gegen `Parse` prüfen; die Portable-Datei allein beweist
keine Zuordnung zu einem Benutzerkonto oder lokalen Dateiplatz.

Ein verlorenes Masterpasswort lässt sich mit unabhängig vorhandenem Recoverycode
überbrücken; ein verlorener Recoverycode mit gültigem Masterpasswort. Fehlen beide,
gibt es keinen administrativen Entsperrweg. Ein Recoverycode ohne Ciphertextdatei
rekonstruiert keine gelöschten Daten. Die Tests verwenden falsche/fehlende Wege und
prüfen diese Fälle; sie behaupten keinen mathematischen Beweis aus einem Unit-Test.

Neuverpacken des Root nach Passwort-/Recoverywechsel löscht keine alten Hüllen
oder Backups. Bei kompromittiertem Entsperrweg sind auch dieser Weg zu ersetzen,
Root/Datenschlüssel zu rotieren und betroffene DB-/SSH-Geheimnisse am eigentlichen
Ziel zu ändern. Ein unveränderter kompromittierter Faktor könnte sonst auch den
neu verpackten Root öffnen. Bekannter alter Klartext wird nicht widerrufen.

Zufallsnonces bleiben nur innerhalb der dokumentierten Nachrichtengrenze sicher.
S2 benötigt einen konservativen lebenslangen Root-Nutzungs-/Rotationszähler
(Planungsgrenze 2^20 Root-AEAD-Operationen; Bibliotheksobergrenze für zufällige
96-Bit-Nonces unter einem Schlüssel 2^32). Dieser Spike erzeugt nur kleine
synthetische Dateien und bietet keinen persistenten Zähler. Er garantiert keine
sichere Löschung von Go-/Swift-Kopien, Swap, SSD-Blöcken, Snapshots oder Backups.
Auch die Referenzimplementierung mit internem Wiping erlaubt keine solche
plattformübergreifende Aussage.

## Prüfumfang und Produktgate

Eigene Go-/Swift-Module mit Standardbibliothek/CryptoKit und bewährter Argon2-
Referenz; keine eigene Kryptoprimitive. Gegenseitige Verschlüsselung/Entschlüsselung,
beide Entsperrwege, feste AAD, Manifestmanipulation, gültiger MAC mit falschem Entry-
Kontext, alle Dateitrunkierungen und begrenzte ungültige Parameter sind geprüft.
Exakte Befehle, Messungen und Grenzen stehen im [Handoff](../handoffs/S1.md).

Vor S2: unabhängige QA, akzeptiertes ADR/Formatpin, sichere Passwort-/Recovery-
Einrichtung mit Rücklesetest, atomarer Persistenz-/Crashpfad, Sperren/Key-Lifecycle,
Update-/Rotationszähler, konkrete App-Sandbox-/Backuppolitik und echte Zielgeräte.
Keine produktive Credentialdatei, kein CloudKit, Passkey oder Teamtresor durch S1.
