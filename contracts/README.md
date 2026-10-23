# Gemeinsame Verträge

API `0.1.0`: [OpenAPI](api-v0.1.0.json), nach unabhängigem QA-Review am 20.10.2026
als Clientvertrag freigegeben. Herkunftscommit `80f203b3e0c7fa3f7f4aa5ec06825cb83bc0c5d0`,
SHA-256 `4b598dc69a1904685e67a717997412654440ac10f47c17f7a025ccd114e294be`.
Die Freigabe bestätigt den Vertrag; tatsächliche Engine- und Integrationsnachweise
stehen in [STATUS](../docs/STATUS.md).
HTTP-JSON mit Bearer-Token, kein Cookie-Login. Jede DB-Anfrage trägt die Verbindung
inklusive flüchtigem Passwort; keine Persistenz und keine Rückgabe der Zugangsdaten.
UI hält Token und Passwörter ausschließlich im Arbeitsspeicher. TLS für entfernte
Verbindungen und authentifizierter HTTPS-Zugang zum Backend sind erforderlich.

Additive Tabellen-API `0.2.0`: [OpenAPI](api-v0.2.0.json),
[Verhaltensvertrag](TABLE-API.md), unabhängig freigegeben am 20.10.2026.
Herkunft `427444d7bcb8e30c9563ded2000ec3d5e19646c4`, SHA-256
`c15a848f01bb779429fd68a2b6eb6bba1575d3e07a4c68e6796cf7840a661838`.
Eigene v2-Routen für Fähigkeiten, Metadaten und begrenztes Browsen; v1 bleibt
unverändert. Keine Mutation oder Snapshotgarantie aus dieser Freigabe ableiten.

Additive Einzelzeilenmutationen `0.3.0`: [OpenAPI](api-v0.3.0.json),
[Verhaltensvertrag](ROW-MUTATION-API.md), unabhängig freigegeben am 20.10.2026.
Herkunft `946241d7f10f7444e9e5efebbe8b3a38a8120a9b`, SHA-256
`ec77c9739cda11df3f4cb9fe756a632e67840fdc0f6190deee1aa1d1333dc308`.
SQLite-Mutationen benötigen mindestens den unabhängig geprüften Konfliktfix
`ea304ef0cfe229f6ea7f4a09c4d64ab0b3e8f73b`; frühere Implementierungen können
tabelleneigenes REPLACE beim UPDATE übernehmen. Schema und APIbytes unverändert.
Eigene v3-Fähigkeiten/Eignung und bestätigtes UPDATE/DELETE mit typisiertem
Erwartungssnapshot. Kein Insert, Massenänderung oder Exactly-once-Versprechen;
v1/v2 bleiben unverändert. Clientintegration benötigt ein eigenes geprüftes Paket.

Additives einzelnes INSERT `0.4.0`: [OpenAPI](api-v0.4.0.json),
[Verhaltensvertrag](ROW-INSERT-API.md), unabhängig freigegeben am 20.10.2026.
Herkunft `72855cf57ceb6620dead42856ee6c9d52d0612bd`, SHA-256
`4763ebf8632a9ef977cf7d3a6037299fc4dafde0a581fe10cb34d844f8356fbf`.
Explizite Bestätigung, begrenzte sichere Identitätspfade, tatsächliche Nachherzeile
und ehrlicher unbekannter Commitausgang; kein Upsert/Replace/Ignore/Batch.
Dieser Backendstand enthält beide nachträglichen v3-Schutzkorrekturen für
SQLite-Konfliktstrategien und PostgreSQL-Rewrite-Regeln. Neue Schreibclients pinnen
diesen geprüften gemeinsamen Implementierungsstand oder eine nachgeprüfte Nachfolge.

Additive lesende Strukturmetadaten `0.5.0`: [OpenAPI](api-v0.5.0.json),
[Verhaltensvertrag](STRUCTURE-API.md), nach unabhängigem Review und Fixnachprüfung
am 23.10.2026 als Clientvertrag freigegeben. Geprüfter Fixstand
`7c66bb908a635e1de8e212a2003e3e0826de4210`, Review
`570f5a0097afd772009b4556565f645b99835290`, SHA-256
`b5225339511d80fe4f52f09118fdad2060d8a92f31d44b73dd89fe9f76178c95`.
Referenzbackend `ac9542d81e6f096626c24bc1e0cb9bccb3a62a7c` enthält den
Namespacefix und die Reviewprovenienz. Neue Clients pinnen diesen Stand oder
eine nachgeprüfte Nachfolge; die früheren identischen0.5-Schemabytes beweisen
keinen gefixten Backendstand. Eigene v5-Fähigkeiten/Strukturroute, konservative
Katalog-/unknown-Angaben und ausdrückliche SQLite-CHECK-Lücke. Keine DDL,
Mutation, atomare Schemasnapshot- oder automatische DDL-Generierungszusage.
V1–v4 bleiben unverändert; eine Produktclientbindung benötigt eigene Prüfung.

Additive strukturierte Tabellenanlage `0.6.0`: [OpenAPI](api-v0.6.0.json),
[Verhaltensvertrag](TABLE-CREATE-API.md), nach unabhängigem Review am 23.10.2026
als Clientvertrag freigegeben. Geprüfter Produktstand
`40defdc0ac4bf31f8e1778f55c22b1431de9c827`, unabhängiger Review
`f36463d7dfac633fb72f99039a8108f4aa6fea13`, SHA-256
`b08b6aba858f72a3bad1fce2933e487eed017f5e35b4690cfd9fae3a590a6887`.
Referenzbackend `3d21e42b0806f2a1953bcbad47f1e9fcb2acda93` enthält die geprüfte
Implementation und Reviewprovenienz. Neue Clients pinnen diesen Stand oder eine
nachgeprüfte Nachfolge. Eigene v6-Fähigkeiten und ausdrücklich bestätigtes CREATE
für neue Tabellen mit festen Typen, konstanten Defaults und konservativer PK-Auswahl.
Bestätigter Abschluss und spätere Strukturbeobachtung sind getrennt; unbekannter
Ausgang erlaubt kein automatisches Retry oder Entfernen. Keine Veränderung bestehender
Tabellen, keine allgemeine DDL-Rollback-, Schemaidentitäts- oder Zustandsnach-Antwort-
Garantie. V1–v5 bleiben unverändert; eine Produktclientbindung benötigt eigene Prüfung.

Persönliches Tresorformat `1`: [Schema](vault-v1.schema.json),
[Byte-/Verhaltensvertrag](../docs/security/VAULT-FORMAT-V1.md),
[synthetische Fixtures](fixtures/vault-v1/README.md). Herkunft `1a4eb3c`/`03bd3f4`,
Schema-SHA-256 `b688656056fe2e23848ce17149bc817609c7369f230a3f49dd91f05db4d3f4dd`.
Unabhängig als Formatgrundlage freigegeben; produktive Persistenz benötigt eigene
S2-Abnahme. API und Tresor haben getrennte Versionsräume.

## Verhalten

Treiberkennungen: `sqlite`, `mysql` (auch MariaDB), `postgres`. Die verfügbaren
Treiber meldet `/api/v1/capabilities`; keine Fähigkeit allein aus dem Schema ableiten.
Netzwerk-TLS standardmäßig `verify-full`; `disable` ist eine ausdrückliche lokale
Entwicklungsoption. SQLite-`database` bezeichnet ausschließlich eine vom Server
verwaltete Datenbankkennung unter seinem Datenverzeichnis, keinen freien Dateipfad.
Die Kennung besteht aus 1 bis 64 ASCII-Zeichen: erstes Zeichen alphanumerisch,
danach alphanumerisch, Unterstrich oder Bindestrich. Der Engine ordnet sie intern
der Datei `<kennung>.sqlite` unter dem konfigurierten Root zu. Keine absoluten
Pfade, `..`, Schrägstriche, `file:`-/DSN-URIs, Queryparameter, Symlinks oder SQL-
`ATTACH`-/`VACUUM INTO`-Dateizugriffe außerhalb dieses Bereichs zulassen. Ein
Stringpräfixvergleich allein ist kein Pfadsicherheitsnachweis.
Die Apple-App öffnet unabhängig davon lokale, vom Benutzer ausgewählte Dateien.

SQL wird mit den Rechten der angegebenen Datenbankverbindung ausgeführt. Kein SQL-
Parser darf Lesemodus oder Schreibschutz vortäuschen. Mutationen benötigen im Client
eine ausdrückliche Benutzeraktion; eine Abfrage kann auch bei Ergebnisbegrenzung
Nebenwirkungen haben. Stapel-/Mehrfachergebnisfähigkeiten werden erst nach echten
Treibertests zugesagt. Kein Transaktionsversprechen über mehrere HTTP-Anfragen.

Anfragen sind auf 1 MiB begrenzt; unbekannte JSON-Felder werden abgewiesen.
`max_rows`: 0 bedeutet Standard 1000, höchstens 10000 je Resultset. `timeout_ms`:
0 bedeutet Standard 10000, höchstens 60000. Ein übergroßes Ergebnis wird ausdrücklich
als `truncated` markiert; Treiber müssen zusätzlich Zell-/Gesamtbytes begrenzen.
Für diese Vertragsversion: höchstens 1 MiB Rohdaten je Zelle und 8 MiB serialisierte
JSON-Antwort insgesamt. Überschreiten führt zu `result_too_large`, ohne bereits
gelesene Teilergebnisse als vollständiges Resultat auszugeben. Die Implementierung
dieser Grenze ist in B1 unabhängig geprüft; Grenzen und tatsächlicher Prüfstand
stehen in [STATUS](../docs/STATUS.md).
Abbruch folgt HTTP-Kontext und Zeitlimit; Datenbankeffekte können bereits erfolgt sein.
`duration_ms` misst serverseitige Ausführung, keine Netzwerklatenz.

Zellen sind getaggt: NULL ist `{"kind":"null"}`; andere Werte tragen `value` als String.
Integer und Dezimalzahlen bleiben dadurch auch außerhalb der JavaScript-Zahlenpräzision
erhalten. Binärwerte sind Base64; Zeitwerte enthalten die vom Treiber verlustarm
gelieferte Darstellung. Spaltenreihenfolge, doppelte Namen und Zeilenreihenfolge
bleiben erhalten. `rows_affected` ist optionaler Dezimalstring, kein Erfolgskriterium
für die Atomarität mehrerer Statements.
Ein leeres Text-/Binärfeld trägt ausdrücklich `"value":""`; `null` trägt kein
`value`. Boolesche Werte werden als `"true"` oder `"false"` dargestellt. Zahlen
verwenden keine lokalisierte Darstellung. Leere Ergebnis-/Spalten-/Zeilenlisten
sind `[]`, nicht JSON-`null`.

Fehler enthalten `error.code` und eine deutsche öffentliche Meldung; rohe Treiberfehler,
Passwörter, DSNs und SQL werden nicht geloggt oder im Fehlertext zurückgegeben.
Kein CORS und keine Browserpersistenz von Credentials. Authentifizierung erfolgt vor
DB-Aufrufen. Statuscodes: 400 Validierung, 401 Anmeldung, 408 Abbruch, 413 Größe,
415 Medientyp, 422 Datenbank-/Fähigkeitsfehler. `/healthz` ist ohne Auth erreichbar.

Die erste Bereitstellung ist ein persönlicher administrativer Dienst mit einem
gemeinsamen Token, ohne Benutzerkonten, Teamrechte oder Mandantentrennung. Wer das
Token besitzt, darf alle implementierten Endpunkte mit selbst gelieferten
DB-Credentials aufrufen. Tokenprüfung ersetzt keine Zielnetzrichtlinie; die B1-Engine erzwingt ihre
dokumentierte Host-/TLS-Policy. Der alleinstehende Apple-
Ausführungsweg verwendet diesen HTTP-Dienst nicht.

## Versionsbindung

Clients kopieren die exakte Spezifikation mit SHA-256 und Herkunftscommit ins eigene
Repo; saubere Checkouts brauchen kein Geschwisterrepo. Kompatibilitätsänderungen
erhalten eine neue Vertragsversion. Gleiche Wertfälle in Go und Swift prüfen.
