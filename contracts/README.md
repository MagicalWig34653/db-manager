# Gemeinsame Verträge

API-Kandidat `0.1.0`: [OpenAPI](api-v0.1.0.json). Vor unabhängiger Prüfung noch nicht freigegeben.
HTTP-JSON mit Bearer-Token, kein Cookie-Login. Jede DB-Anfrage trägt die Verbindung
inklusive flüchtigem Passwort; keine Persistenz und keine Rückgabe der Zugangsdaten.
UI hält Token und Passwörter ausschließlich im Arbeitsspeicher. TLS für entfernte
Verbindungen und authentifizierter HTTPS-Zugang zum Backend sind erforderlich.

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
dieser Grenze gehört zur Treiberintegration; die Infrastruktur mit leerem Engine
belegt sie nicht.
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
DB-Credentials aufrufen. Tokenprüfung ersetzt keine Zielnetzrichtlinie; diese
wird zusammen mit Netzwerk-Treibern implementiert. Der alleinstehende Apple-
Ausführungsweg verwendet diesen HTTP-Dienst nicht.

## Versionsbindung

Clients kopieren die exakte Spezifikation mit SHA-256 und Herkunftscommit ins eigene
Repo; saubere Checkouts brauchen kein Geschwisterrepo. Kompatibilitätsänderungen
erhalten eine neue Vertragsversion. Gleiche Wertfälle in Go und Swift prüfen.
