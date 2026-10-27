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

Additive Verträge `0.7.0` bis `0.23.0` (PWA-Linie, Wellen 1 bis 4, Stand 27.10.2026). Die Kette ist lückenlos
und kumulativ: jede Version ist ein Superset ihres Vorgängers, v1 bis v16 sind byteidentisch zum jeweils
freigegebenen Stand, `scripts/contract_chain.py` pinnt v1 bis v22 per SHA-256 und die Wächter prüfen Pins und
Additivität. Alle Versionen sind auf `main` (Wellen 1 und 2: bis `6cd15c8`, Welle 3: `3a7e1f1`, Welle 4:
`751dbd1`) integriert; die Web-PWA bindet alle 23 Versionen seit [Bündel 5](../docs/handoffs/PIN-BUNDLE-5.md) im Lock.
Die Tabelle nennt die aktuellen Dateibytes; frühere, in Handoffs genannte SHA-256 der Versionen 0.18 bis 0.23
sind durch Reviewfolgen überholt (siehe unten).

| Version | Paket und Umfang | Verhaltensvertrag | SHA-256 | Unabhängiges Review (Befund) |
| --- | --- | --- | --- | --- |
| [0.7.0](api-v0.7.0.json) | W-P06 Katalog, Übersicht, Definition, Serverinfo (lesend) | [OVERVIEW-API](OVERVIEW-API.md) | `c6fd8c91315e25185d949d309397c7c0d80cc67fcab87ae521e38068e7004d17` | [W-P06](../docs/handoffs/W-P06-backend-review.md): Freigabe |
| [0.8.0](api-v0.8.0.json) | W-P07 Datenbank- und Tabellenoperationen mit Plan und Bestätigung | [OBJECT-OPS-API](OBJECT-OPS-API.md) | `74aa39ed06311f2c45afc274a2412a0ac30912e3a5683ec96b812fd473a28264` | [W-P07](../docs/handoffs/W-P07-backend-review.md): Freigabe nach Nachprüfung |
| [0.9.0](api-v0.9.0.json) | W-P08 Spalten ändern | [COLUMNS-API](COLUMNS-API.md) | `508b9951d102ec44bf4cafc9fe444f7266b31f898d9e91c887f4bef39735e3f8` | [W-P08](../docs/handoffs/W-P08-backend-review.md): Freigabe nach Nachprüfung |
| [0.10.0](api-v0.10.0.json) | W-P10 Tabellenwartung | [MAINTENANCE-API](MAINTENANCE-API.md) | `077a0e0222963b112bd6ea528ef5668d2ff7fbe2b2d3555669487c4beaf55cc5` | [W-P10](../docs/handoffs/W-P10-backend-review.md): Freigabe |
| [0.11.0](api-v0.11.0.json) | W-P11 Export und SQL-Dump (gestreamt, mit Schlussmarke) | [EXPORT-API](EXPORT-API.md), [EXPORT-FORMAT](EXPORT-FORMAT.md) | `b3bcc38b44b020335151aead207e281f9c54bdc7463d6fb1ddd7a6cc22396620` | [W-P11](../docs/handoffs/W-P11-backend-review.md): Freigabe nach Nachprüfung |
| [0.12.0](api-v0.12.0.json) | W-P15 SQL-Stapel, EXPLAIN, DML-Simulation (nur SQLite) | [SQL-BATCH-API](SQL-BATCH-API.md) | `7f0446edccdc368a202e4eaae5689d4b44d641e34bdf77ec8314de9f3e3f2dcd` | [W-P15](../docs/handoffs/W-P15-backend-review.md): Freigabe nach Nachprüfung |
| [0.13.0](api-v0.13.0.json) | W-P09 Indizes, Primär- und Fremdschlüssel | [INDEXES-API](INDEXES-API.md) | `e05a2402a8e6ec6a1901afeac03969e34d4fa1821e50a5c8675a6e20f5c1d3d7` | [W-P09](../docs/handoffs/W-P09-backend-review.md): Freigabe mit niedrigen Auflagen |
| [0.14.0](api-v0.14.0.json) | W-P14 Suche, Suchen und Ersetzen | [SEARCH-API](SEARCH-API.md) | `b57c3e5dcf403c3e16d8564c68d0220453625b25f6f206fdc128d2e249656c1e` | [W-P14](../docs/handoffs/W-P14-backend-review.md): Freigabe mit Auflagen |
| [0.15.0](api-v0.15.0.json) | W-P18 Datenbankbenutzer und Rechte (Plan, Bestätigung, geschützte Passwörter) | [ACCOUNTS-API](ACCOUNTS-API.md) | `cd4681a200045c2fdd5ddf9f1bdc2a356c998b30e640495121925ca8a609dad1` | [W-P18](../docs/handoffs/W-P18-backend-review.md): Freigabe mit Auflagen |
| [0.16.0](api-v0.16.0.json) | W-P19 Views, Routinen, Trigger, Events | [PROGRAMS-API](PROGRAMS-API.md) | `d90ac3fda8295a4b8a7095c72eb459c6433fce90f729d7a6b247760d86de7bf3` | [W-P19](../docs/handoffs/W-P19-backend-review.md): Freigabe mit Auflagen |
| [0.17.0](api-v0.17.0.json) | W-P17 Serververwaltung, **ausschließlich lesend** (Status, Variablen, Prozesse) | [SERVER-ADMIN-API](SERVER-ADMIN-API.md) | `ec2f75e2159f06fcc841c19089ba82fa9e58b998f55dfc27efcabed6771b39f3` | [Review B](../docs/handoffs/WAVE4-REVIEW-B.md): Freigabe, Befund F3 behoben |
| [0.18.0](api-v0.18.0.json) | W-P12 Import SQL und CSV (Upload, Plan für CSV, SQL-Import ohne Plan) | [IMPORT-API](IMPORT-API.md) | `17451221a82e4c0cc64c99ad1d49dbda31846eac104698d7f7ec5db097b103e5` | [Review A](../docs/handoffs/WAVE4-REVIEW-A.md) und [Re-Review](../docs/handoffs/WAVE4-REREVIEW.md): Blocker B1 gefunden, Autorenfix ohne unabhängige Nachprüfung |
| [0.19.0](api-v0.19.0.json) | W-P13 Mehrzeilenaktionen, Dateiwerte, Fremdschlüssel-Nachschlagen | [ROWS-BULK-API](ROWS-BULK-API.md) | `c7a52e69685a1ab88656916071d1e931a500ba4aa9854c9fe0e11f130d0a2d46` | Review A: Freigabe unter Auflagen; Re-Review bestätigt Replay-Schutz über HTTP |
| [0.20.0](api-v0.20.0.json) | W-P27 Monitor, Advisor, Replikations- und Binärlogstatus, **nur lesend** | [MONITOR-API](MONITOR-API.md) | `83b49461222f4d79690bd645fea12cfe2427d9fb0bddb9bd21a51ce0c34b63b0` | Review B: Freigabe mit Auflage F2, im Re-Review bestätigt |
| [0.21.0](api-v0.21.0.json) | W-P28 erweiterte Textformate und Dump-Details | [EXTENDED-EXPORT-API](EXTENDED-EXPORT-API.md) | `68db81f9de234193bb3a85d5004e416651221b8f423783b8f2a7f8715218881c` | Review B: Freigabe mit Auflage F1 (YAML), im Re-Review bestätigt |
| [0.22.0](api-v0.22.0.json) | W-P29 Partitionen und Tabellenpräfix-Stapel | [PARTITIONS-API](PARTITIONS-API.md) | `397db1613129333a755b22093e25bf1443756d252379940661c6c2df9144afd1` | Review B: Freigabe ohne Befunde |
| [0.23.0](api-v0.23.0.json) | Reason-Abgleich: `AccountResult.reason` um `grant_without_effect` erweitert; keine neuen Pfade, keine neue Laufzeit-Versionsnummer (Capabilities melden weiter 0.15/0.22) | [ACCOUNTS-API](ACCOUNTS-API.md), Wächter `check-reasons-contract.py` | `7450a7fc994126beb25799e99dae07d6763e43f3e9ef002b20068cb38e313bd7` | [Re-Review](../docs/handoffs/WAVE4-REREVIEW.md): Enum-Abgleich und Test bestätigt |

Integrationsnachweise: [Welle 2](../docs/handoffs/WAVE2-INTEGRATION.md) (0.8 bis 0.12), [Welle 3](../docs/handoffs/WAVE3-INTEGRATION.md)
mit [Re-Review](../docs/handoffs/WAVE3-REREVIEW.md) (0.13 bis 0.16), [Welle 4](../docs/handoffs/WAVE4-INTEGRATION.md) (0.17 bis 0.23).
Die API-Nummern wichen teils von den Planvorschlägen ab: W-P09 ist 0.13, W-P18 0.15, W-P19 0.16, W-P17 0.17, W-P12 0.18, W-P13 0.19,
W-P27 0.20, W-P28 0.21, W-P29 0.22. Ältere Handoffs und ADRs nennen teils die ursprünglichen Nummern.

Ehrliche Grenzen der Freigaben 0.7 bis 0.23:

- **Die Wellen-3-Freigaben sind Freigaben mit Auflagen.** Offen aus dem Re-Review: Wiederherstellung bei unbestimmtem CREATE-Ausgang
  (N4) und Umbenennen auf einen Systemkontonamen auf MariaDB (N5); PostgreSQL-SCRAM mit SASLprep ist umgesetzt, aber nur mit
  libpq-konformen Clients nutzbar, wenn das Passwort unter NFKC abweicht.
- **Welle 4 (0.17 bis 0.23) ist nicht uneingeschränkt freigegeben.** Das [Re-Review](../docs/handoffs/WAVE4-REREVIEW.md) endete mit
  „Nicht freigegeben“ (Blocker B1: bestätigter CSV-Import war über HTTP wiederholbar). Der Autorenfix `9f73efa` (Vorabprüfung
  schaltet den Plan nicht erneut frei, PostgreSQL-Abbruch, Fixture, Meldungstext) ist auf `main`, wurde aber **nicht erneut unabhängig
  geprüft**. Der Re-Review-Hinweis N1 bleibt offen: Importausführung benötigt etwa 500 MiB bei zwei Slots (Plan etwa 320 MiB), echtes
  Streaming fehlt; empfohlen ist ein Containerlimit von mindestens 1 GiB.
- **Einmal-Pläne** (Plan-Ledger) gibt es nur für CSV-Import und Mehrzeilen-Kopieren, nur im Speicher, für 1 Stunde und höchstens
  4096 Einträge; nach Neustart und über mehrere Instanzen sind sie nicht wirksam. Partitionen, Struktur, Konten, Programme, Indizes,
  Suche/Ersetzen und Wartung nutzen keinen Einmal-Plan. Der **SQL-Import hat bewusst keinen Plan** (Dumps sind nicht planbar); die
  Antwort warnt mit einer Klassenzählung, die Oberfläche muss die Warnung vor der Ausführung zeigen.
- **Echtläufe:** Die Backendpakete wurden gegen SQLite, MySQL 8.4, MariaDB 12.3 und PostgreSQL 18.6 geprüft (Integrationen und Reviews).
  Im Web-Echtlauf (Browser gegen Backend) fehlen MySQL/MariaDB für Import, Mehrzeilenaktionen, Exportformate und Partitionen; Konten
  und Wartung haben keinen vollständigen Echtlauf im Browser.
- Verträge ohne eigenes Nachpinnen: Wächter älterer Pakete (v1 bis v16) pinnen die neueren Versionen nicht. Die SHA-256 von 0.18 bis
  0.23 in den Integrationshandoffs wurden durch Reviewkorrekturen (`rolled_back`, Beschreibungen, `reason`-Enum) ersetzt; maßgeblich
  sind die Bytes der Dateien und `scripts/contract_chain.py`.

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
