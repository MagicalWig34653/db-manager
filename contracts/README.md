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
