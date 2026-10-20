# Tresor und Sync: Befunde und gestufter Entwurf

Stand und Recherche: 20.10.2026. Status: Vorschlag aus P3a, kein akzeptiertes ADR.
Fortschreibung S1: konkretes [Format v1](VAULT-FORMAT-V1.md) und tatsächliche
Go-/Swift-Interoperabilität liegen jetzt als getrennter Reviewkandidat vor.
Die nachfolgenden P3a-Befunde bleiben Ausgangspunkt; S1 ist keine Produktfreigabe.
Verbindliche Ausgangsanforderungen liegen in der [Projektübergabe](../../PROJEKTUEBERGABE.md).
Bedrohungen: [Modell](BEDROHUNGSMODELL.md); Abnahme: [Prüfplan](PRUEFPLAN.md).

## Belastbare Befunde und offene Plattformfragen

| Thema | Nachgewiesen | Nicht nachgewiesen |
| --- | --- | --- |
| Normale Passkey-Anmeldung | WebAuthn beweist Credentialbesitz; die PRF-Erweiterung liefert separat credentialbezogene 32-Byte-Ausgaben [Q1] | Kein Tresorschlüssel aus einer gewöhnlichen Signatur oder aus dem öffentlichen Schlüssel |
| Native Apple-Plattform-PRF | Lokales SDK deklariert `ASAuthorizationPublicKeyCredentialPRFAssertionInput`, Registration-Supportcheck und `.prf` ab macOS 15.0/iOS 18.0; Compileprobe vorhanden [Q2] | Keine ausgeführte Autorisierungszeremonie, keine Aussage über jeden Credentialprovider, Synchronisierung oder tatsächliche PRF-Stabilität nach Recovery |
| Native externe Security-Keys | Aktuelles SDK deklariert Registration-PRF für `ASAuthorizationSecurityKeyPublicKeyCredentialRegistrationRequest` ab macOS/iOS 26.4; Compileprobe vorhanden | Kein Geräte-/Providerbeleg und kein Beleg für Browser-PRF auf älteren Systemen |
| Apple-Passkeys ohne Go | Native Requests benötigen `webcredentials` Associated Domains [Q3]; statische Domainzuordnung braucht keinen Go-Ausführungsdienst | Lokale Einrichtung mit selbst erzeugter Challenge sowie Offlinebenutzung nach Einrichtung sind gesondert praktisch zu validieren; keine Zusage völlig ohne Domain-/Netzbezug |
| CloudKit | Private Datenbank und verschlüsselte Felder sind dokumentiert; Account-/Schlüsselreset und nicht indexierbare verschlüsselte Felder haben konkrete Folgen [Q4] | Keine ausgeführte Zwei-Geräte-Synchronisierung; CloudKit-Verschlüsselung ersetzt nicht den geforderten eigenen Tresor |
| Keychain-Sync | Apps müssen `kSecAttrSynchronizable` ausdrücklich setzen [Q5] | Kein automatischer neuer Entsperrweg durch bloß vorhandene CloudKit-Daten; keine pauschale Secure-Enclave-Speicherung symmetrischer Tresorschlüssel |
| Passwort-KDF | RFC 9106 beschreibt Argon2id samt Testvektoren; kleinere Referenzkonfiguration 64 MiB, drei Durchläufe, vier Lanes, 16-Byte-Salt [Q6] | Noch keine ausgewählte plattformübergreifende Bibliothek, Laufzeit-/Speichermessung oder Mindestgeräteprüfung |
| AES-GCM | Go-Standardbibliothek bietet Zufallsnonce-API mit 96-Bit-Nonce, 128-Bit-Tag und dokumentierter Nachrichtengrenze [Q7] | Kein Nachweis eines vollständigen Tresors, einer Migration oder Go-/CryptoKit-Interoperabilität |

Empfehlung für die erste PRF-Untersuchungsmatrix: macOS 15 und iPadOS 18 als
technische Untergrenze der nativen Plattform-PRF, plus aktuelle unterstützte
Versionen. Das ist keine Aussage über Supportdauer oder Betriebssicherheit alter
OS-Versionen und kein endgültiges Produktminimum. Für einen früheren lokalen
Client können Treiber/API-Nutzung andere Mindestversionen erlauben; P2 entscheidet
mit. Externe native PRF-Security-Keys separat ab 26.4 untersuchen. Browser und
Drittanbieterprovider brauchen eigene Prüfzeilen; Feature-Erkennung und erfolgreicher
Entschlüsselungsrundlauf entscheiden über die angebotene Funktion.

## Stufe S0: flüchtige Verbindung vor Tresorfreigabe

Eine lokale Apple-Verbindung fragt das Geheimnis nur für die aktuelle Sitzung ab.
Ein serverseitig ausgeführter Webauftrag überträgt es per HTTPS gezielt an Go,
bewahrt es höchstens in begrenztem Prozess-/Sitzungsspeicher und schließt bei Ablauf
die Verbindung. Keine Wiederverwendung in Logs, Query-URLs, Browserstorage,
Metadatenpersistenz, Crashtexten, Beispieldateien oder Telemetrie. Kein Hintergrundjob
über den autorisierten Sitzungsrahmen hinaus. UI benennt den ausführenden Ort.

S0 ermöglicht die erste echte DB-Abfrage, ohne eine unvalidierte Tresorarchitektur
produktiv festzuschreiben. Vor Netzwerkbetrieb bleiben Authentisierung,
Mandantentrennung, HTTPS, CSRF-/Origin-Prüfung, XSS-Abwehr und Zielnetzrichtlinie
eigene Abnahmekriterien. Ein lokaler Entwicklungsmodus mit Loopback ist keine
Freigabe für unauthentisierte externe Bereitstellung.

## Stufe S1: persönlicher lokaler Tresor

Vorgeschlagene Hierarchie, ausschließlich mit etablierten Bibliotheksprimitiven:

1. Ein zufälliger 256-Bit-Root-Schlüssel pro persönlichem Tresor und Epoche.
2. Ein neuer zufälliger 256-Bit-Datenschlüssel pro Datensatzrevision. AES-256-GCM
   verschlüsselt den Inhalt; der Root-Schlüssel schützt den Datenschlüssel mit
   einer gesonderten AEAD-Operation. Jede Operation benötigt eine frische Nonce.
3. Ein aus dem Masterpasswort per Argon2id abgeleiteter 256-Bit-Schlüssel schützt
   den Root-Schlüssel. Eigenes zufälliges Salt pro Hülle; KDF-Parameter werden
   versioniert gespeichert. Vorschlag: RFC-Referenzkonfiguration zunächst messen,
   nicht ungeprüft als Gerätekompatibilität ausgeben. Grenzen für eingelesene
   Parameter vor KDF-Ausführung verhindern Speicher-/CPU-Angriffe.
4. Ein zufälliger 256-Bit-Wiederherstellungsschlüssel schützt denselben Root in
   einer separaten Hülle. Seine benutzbare Darstellung, Prüfsumme und Importformat
   müssen separat festgelegt werden. Vor Aktivierung Rücklese-/Entschlüsselungstest;
   Wiederherstellungsmaterial bewusst außerhalb dieses Accounts aufbewahren.
5. Optionaler lokaler Komfortzugriff über Keychain-Hülle. Gerätebindung und
   Benutzerbestätigung werden explizit gewählt; kein stilles Synchronisieren.
   Ein gerätegebundener Weg ist keine Wiederherstellung auf einem neuen Gerät.

Passwort und Wiederherstellungsschlüssel verlassen den Client nicht. Der Login-
Verifier eines Backend-Kontos ist kein Tresorschlüssel. Passwortwechsel erneuert
die Passwort-Hülle atomar; der Root bleibt dabei gleich. Bei vermutetem Diebstahl
der alten Hülle/ihres Schlüssels reicht solches Neuverpacken nicht: Root-Epoche und
betroffene Daten-/DB-Geheimnisse müssen nach Bedrohungslage ebenfalls wechseln.

Vorgeschlagene Hüllenfelder: Format-/Algorithmusversion, Tresor-ID, Epoche,
Hüllen-ID/-typ, KDF-ID samt Salt/Parametern oder PRF-Credential-ID/-Input,
Nonce und verschlüsselter Root. Datensätze tragen Tresor-/Datensatz-ID,
Revision/Elternrevisionen, Epoche, Nonce, verschlüsselten Datenschlüssel und
Payload. Zweck, IDs, Epoche und relevante Versionsfelder werden durch Additional
Authenticated Data gebunden. Kein sensibles Feld bleibt für bequemes Suchen im
Klartext; Suchindizes zunächst nur lokal und flüchtig nach Entsperren.

Das endgültige sprachübergreifende Byteformat einschließlich AAD-Kanonisierung,
Größenlimits und unbekannter Felder benötigt einen eigenen Vertrag samt festen
Go-/Swift-Testvektoren. Der Spike verwendet JSON eines festen Go-Structs nur für
lokale Tests und setzt keinen gemeinsamen Serialisierungsvertrag.

Schreibablauf: neue verschlüsselte Revision vollständig erzeugen, atomar speichern,
erst nach erfolgreichem Commit alte Referenz ersetzen. Backup/Restore und abgebrochene
Rotation müssen vollständig alte oder vollständig neue lesbare Stände liefern.
Ungültige Tags oder unbekannte Versionen führen zu Fehlern; kein Klartextfallback.

## Stufe S2: zusätzliche Passkey-Entsperrhülle

PRF-Support zunächst prüfen und die tatsächliche PRF-Ausgabe anfordern. Aus dieser
Ausgabe über HKDF-SHA-256 mit getrenntem Anwendungskontext und Tresor-/Hüllenbindung
einen Hüllenschlüssel ableiten [Q8]. Zufälligen PRF-Input und Credential-ID samt
Hülle speichern; weder PRF-Ausgabe noch Hüllenschlüssel an den Loginserver senden.
Jeder weitere Passkey erhält nach bestehendem Entsperren eine eigene Root-Hülle.
Ein neu erstellter Ersatzpasskey ergibt einen neuen Schlüssel; bloßer Kontoreset
entschlüsselt alte Hüllen nicht.

Für eigenständiges Apple ist Masterpasswort plus Recovery der erste vollständige
Weg. Passkey-Komfort bleibt optional, bis signierte App, RP-Domain/AASA,
Providerwechsel, neues Gerät, gelöschtes Credential und Offlinefälle getestet sind.
Eine projektbetriebene statische AASA-Domain könnte Go unabhängig vermeiden,
bleibt aber eine Domain-/Betriebsabhängigkeit und benötigt ein späteres ADR.
Ohne erfolgreiche PRF-Evaluation keinen Passkey als Tresorentsperrweg anzeigen.
Backend-Login und Vault-PRF-Resultat haben getrennte Datenmodelle, auch wenn dieselbe
Zeremonie beides auslöst; keine vollständigen Extension-Resultate zum Server kopieren.

## Stufe S3: persönliche iCloud-Synchronisierung

Nur persönliche Daten in einer privaten CloudKit-Zone speichern, mit
anwendungseigener AEAD bereits vor dem Upload. Erkennbar bleiben mindestens
Objekt-IDs, Größen, Transportzeitpunkte und Zugriffsmuster. Optional zusätzlich
CloudKit-verschlüsselte Felder nutzen, ohne daraus eine stärkere plattformunabhängige
Schlüsselkontrolle abzuleiten. Apple dokumentiert eigene Schlüsselverlustfälle und
Bedingungen für Advanced Data Protection [Q4].

Anmeldung an iCloud erlaubt Download, aber die App entsperrt auf dem neuen Gerät
über Masterpasswort/Recovery oder nachgewiesene PRF-Hülle. CloudKit-Token sind keine
Tresorschlüssel. Kein eigener Go-Server im persönlichen Syncpfad. iCloud deaktiviert,
abgemeldet, Netz weg oder Rate-Limit: lokale verschlüsselte Daten und Entsperrwege
bleiben nutzbar; ausstehende Änderungen landen atomar in einer lokalen Warteschlange.

Konflikte werden durch Elternrevisionen und serverseitige bedingte Änderungen
erkannt. Konkurrierende Passwortänderungen erzeugen zwei verschlüsselte Varianten;
keine stille Wahl nach Geräteuhr. Benutzer wählt bzw. ersetzt die Verbindung und
prüft sie bewusst. Tombstones verhindern Wiederauferstehen gelöschter Einträge.
Schlüsselhüllen und Rotationsepochen benötigen dieselbe Konfliktbehandlung.
Lokaler Versionshöchststand hilft bei Rollback-Erkennung; ein neues Gerät ohne
unabhängigen aktuellen Anker kann vollständiges gültiges Zurückspielen nicht
beweisen. Diese Grenze bleibt sichtbar im Bedrohungsmodell.

Accountwechsel pausiert Sync und trennt lokalen Datenbestand vom neuen Account.
CloudKit-Zonenverlust/Schlüsselreset darf keinen ungefragten lokalen Löschlauf
auslösen. Wiederhochladen vorhandener Ciphertexte erfolgt erst nach konsistenter
Account-/Tresorzuordnung. Vollständiger Accountverlust lässt sich nur mit vorhandenem
lokalem Bestand/Export plus unabhängigen Entsperrmitteln überstehen; der Recovery-
Schlüssel allein rekonstruiert keinen verlorenen Ciphertext.

## Stufe S4: explizite Teamfreigaben und Go-Betrieb

Teamfähigkeit benötigt Backend-Anmeldung und Objektberechtigungen. Vorschlag:
getrennte Rechte `connection.readMetadata`, `connection.execute`,
`connection.revealSecret`, `connection.share`, `job.create`, `job.execute` und
`team.manageMembers`; finale Rollenmatrix folgt vor Implementierung. Eine
persönliche Verbindung bleibt privat, bis die Person Zielteam, enthaltene Daten,
Ausführungsort und Geheimnisfreigabe bestätigt. Keine persönliche Root-Hülle für
Teammitglieder oder Backendadministratoren erzeugen.

Erste serverseitige Teamstufe: je Team eigene Datenschlüsselhierarchie und Epoche;
Root-Hülle unter Betriebs-Schlüssel aus gesondertem Secretstore. Autorisierter Go-
Dienst entschlüsselt nur für konkrete Ausführung/Anzeige. Dies ist verschlüsselte
Speicherung mit serverseitigem Zugriff und kein Schutz vor dem Go-Betreiber.
Ein späteres nur an Mitglieder verschlüsseltes Austauschmodell wäre separat zu
entwerfen; unbeaufsichtigte Go-Jobs brauchen dennoch einen Dienstentsperrweg.

Mitgliederentzug sperrt sofort neue API-/Jobzugriffe und widerruft Sessions,
benötigt eine definierte Behandlung laufender DB-Sitzungen sowie gegebenenfalls
DB-Credentialrotation. Team-Schlüsselrotation schützt zukünftige Stände; bereits
kopierte Geheimnisse/Backups werden dadurch nicht ungültig. Jobrechte vor Start,
nach Wiederanlauf und an festgelegten Fortsetzungsgrenzen erneut prüfen. Audit
enthält Akteur, Objekt-ID, Aktion, Ergebnis und Zeitpunkt, keine Secrets oder
vollständigen SQL-Texte/Ergebnisse.

Docker Compose und Swarm sind bestätigte Deploymentziele. Geheimnisse im Image,
Repository und normalen Environmentwerten vermeiden; Prozess erhält gezielte
Dateisecrets, z. B. über eine dokumentierte `*_FILE`-Konfiguration. Compose-Secrets
werden per Dateimount zugestellt [Q9]; deren Quelldateien brauchen Hostschutz.
Swarm verschlüsselt Secrets im Transport und Raft-Speicher und stellt sie dem
berechtigten Linux-Task im Arbeitsspeicherdateisystem bereit [Q10]. Beide erfordern
Vertrauen zum Host/Manager und Dienstprozess. Mehrere Replikate benötigen gemeinsame
Sitzungs-/Widerrufsdaten, konsistente Rotationsepochen und koordinierte Jobannahme;
ein Prozessspeicher-Lock oder lokale SQLite-Datei ist dafür kein gemeinsamer
Autorisierungs- oder Jobzustand. Geheimnisse versioniert rotieren, alte Hüllen
erst nach überprüftem Rewrap/Backup entfernen. Diese Punkte sind Anforderungen an
die spätere Bereitstellung, keine ausgeführten Container-/Swarmtests.

## Primärquellen

Alle URLs am 20.10.2026 recherchiert; Aussagen sind Befunde, nicht Ergebnisse eigener Gerätetests.

- Q1: [W3C WebAuthn Level 3, PRF](https://www.w3.org/TR/webauthn-3/#prf-extension),
  [W3C-Explainer](https://github.com/w3c/webauthn/blob/main/explainers/prf-extension.md).
- Q2: [Apple PRF Assertion Input](https://developer.apple.com/documentation/authenticationservices/asauthorizationpublickeycredentialprfassertioninput-swift.struct).
  Zusätzlich lokales Xcode-SDK: `AuthenticationServices.framework`, öffentliche
  `arm64e-apple-macos.swiftinterface`; Toolchain und Probe im [Spike](../../spikes/security/README.md).
- Q3: [Apple Supporting passkeys](https://developer.apple.com/documentation/authenticationservices/supporting-passkeys),
  [Associated Domains](https://developer.apple.com/documentation/xcode/supporting-associated-domains).
  Apples dort verlinkte Markdownfassungen wurden nach einem Content-Type-Fehler
  des Browsers lesend per `curl` abgerufen; kein Plattformtest daraus abgeleitet.
- Q4: [Apple Encrypting User Data](https://developer.apple.com/documentation/cloudkit/encrypting-user-data),
  [CKRecord.encryptedValues](https://developer.apple.com/documentation/cloudkit/ckrecord/encryptedvalues).
- Q5: [Apple Secure keychain syncing](https://support.apple.com/en-ie/guide/security/sec0a319b35f/web),
  [kSecAttrSynchronizable](https://developer.apple.com/documentation/security/ksecattrsynchronizable).
- Q6: [RFC 9106, Argon2](https://www.rfc-editor.org/rfc/rfc9106.html).
- Q7: [Go cipher.NewGCMWithRandomNonce](https://pkg.go.dev/crypto/cipher#NewGCMWithRandomNonce).
- Q8: [Go HKDF](https://pkg.go.dev/crypto/hkdf), [RFC 5869](https://www.rfc-editor.org/rfc/rfc5869).
- Q9: [Docker Compose Secrets](https://docs.docker.com/compose/how-tos/use-secrets/).
- Q10: [Docker Swarm Secrets](https://docs.docker.com/engine/swarm/secrets/).
