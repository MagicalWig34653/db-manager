# Unabhängiger B4a-Review

Stand: 23.10.2026. Reviewer ist weder B4a-Autor noch Implementierer.
Eigener Worktree `.worktrees/b4a-review`, Branch `review/b4a`, Ausgangspunkt
`464e64744a67034d64fb07179d03ace9e6e2f89c`. Produkt exakt
`40defdc0ac4bf31f8e1778f55c22b1431de9c827`, Gesamtdiff ab `8aab52b`.
Der nachfolgende Autorenhandoff ändert keine Produktbytes.

## Urteil

**Kein bestätigter Befund. Das begrenzte B4a-Inkrement ist zur Agentintegration und
anschließenden gesonderten API0.6-Freigabe durch die Projektleitung geeignet.**
Keine Selbstintegration oder Vertragsfreigabe durch diesen Reviewer. Kein allgemeines
DDL-Rollback-, Schemaidentitäts-, Zustandsnach-Antwort-, Trigger-/Externaleffekt-,
Powerloss-, UI-, Betriebs- oder Paritätsversprechen.

Der bestätigte CREATE-Abschluss und die spätere Strukturbeobachtung sind getrennt.
Insbesondere bleibt MySQL/MariaDB `created/unavailable` nach sicherem Ack ein
bestätigter Abschluss; eine nachgelagerte unbestätigte/abweichende Struktur ist kein
Grund für automatisches Retry oder Entfernen. `snapshot_atomic` gilt ausschließlich
für die zusammengehörige DDL-/Metadatenprüfung derselben SQLite-/PG-Transaktion.

## Quell- und Vertragsprüfung

- Dateigrenze geprüft: neue Core-/DB-/HTTP-/Vertrags-/Testdateien und enge zusätzliche
  Route; keine Änderungen an v1–v5-Vertragsbytes, GoMod/GoSum, Enginegrundlage,
  vorhandenen Webassets oder Betriebsdateien. Keine neue Abhängigkeit.
- API0.6-SHA256 tatsächlich
  `b08b6aba858f72a3bad1fce2933e487eed017f5e35b4690cfd9fae3a590a6887`.
- Compiler: feste Typen, begrenzte Präzision/Skala/Längen/Spalten/PK-Budgets,
  validierte signed64-/Decimal-/Textkonstanten, korrekt gequotete Identifier und
  Literale unter gesetztem Sessionmodus; keine freien SQLfragmente oder Auswertung
  von Katalogdefault-SQL. Nullable/Default/PK und konservative Namensduplikate geprüft.
- CREATE ohne IF NOT EXISTS/REPLACE, advisory Lookup und tatsächliche Namensarbitrierung,
  keine Retry-/Drop-/Reparatur-/Markierungslogik. SQLite/PG verifizieren vor Commit;
  MySQL-MDL schützt nur die gesonderte Beobachtung nach Lockaufnahme.
- Fehler-/Abbruch-/Commitgrenzen, tatsächliches JSON-Antwortbudget inklusive Newline,
  Authentifizierung vor Engine, Body-/Zeit-/Connectiongrenzen und sichere Fehler geprüft.
- Eigene fünf HTTP-Gegenproben für verbotene Null-Zusatzfelder (`length:null`,
  `scale:null`, `precision:null`, `none/value:null`, `null/value:null`) ergaben jeweils
  korrekt HTTP400. Der bestehende globale Token-Nullguard verhindert die zunächst
  vermutete Pointer-Presenzlücke; **kein Finding** daraus.

## Unabhängig tatsächlich ausgeführt

Im unveränderten Reviewcheckout:

```sh
go test -race -count=1 ./...
go vet ./...
python3 scripts/check-create-contract.py
python3 scripts/check-docs.py
python3 scripts/test-create.py
```

Vollständig bestanden; Dokumentation am Kandidaten 288 Dateien/0 Fehler. Die
Vertragsprüfung bestätigt eingefrorene v1–v5-Bytes und additive v6-Definitionen/Routen.
Der Livehelfer startete ausschließlich eigene zufällige TLS-Fixtures für MySQL8.4,
MariaDB12.3 und PostgreSQL18.6. Die originale vollständige Autoren-Livematrix wurde
unabhängig wiederholt: positive Anlage/Defaults/Binary/Sondernamen, Konflikte,
CREATE-only-User, MDL/Timeout/Connectionfreigabe, PG-Namensrace/fehlende Rechte,
wirklicher deferred COMMITfehler, separate nach-Ack-Strukturänderung und keine
Reparatur. Kein negativer TLS-Fall wird ohne erfolgreiche positive Verbindung als
Verschlüsselungsnachweis ausgegeben.

Zusätzliche eigene temporäre Peerproben wurden im Reviewworktree erstellt,
ausgeführt und anschließend entfernt; keine Produktdatei geändert:

- Auf SQLite und allen drei TLS-Zielen neue Tabellen mit Defaultwerten
  `-9223372036854775808`, einem Supplementary-Unicodezeichen in VARCHAR(1),
  Tab/CR/LF/Ctrl-Z/Apostroph/Backslash als Text und Decimal `-0.0` angelegt.
- Danach echte INSERT-Defaultausführung und unabhängiges SELECT über die reale
  Engine: Integer-/Textwerte blieben exakt erhalten. Decimal wurde entsprechend
  der tatsächlichen Treiberart gelesen; insbesondere SQLite darf `-0.0` als
  Integer0 speichern, wie im Vertrag ausdrücklich begrenzt.
- Bestehende View mit einem Fixturewert17 war bei Anlageversuch ein echter409-
  Konflikt und lieferte danach unverändert dieselbe Spalte/denselben Wert.
- MySQL/Maria lieferten bei einzelnen gültig gespeicherten Unicode-/Steuertextdefaults
  konservativ `created/different`, weil die bekannte Katalogdarstellung nicht als
  gleich bestätigt wurde. Die getrennte reale Defaultausführung lieferte die
  angeforderten Werte exakt. Dies ist die dokumentierte konservative Grenze und
  wurde weder als Datenverlust noch als garantierte Gleichheitsbestätigung dargestellt.

Die zusätzliche Netzwerkrunde lief erst nach abgeschlossenem Finally-Cleanup der
vorherigen Runde; niemals zwei eigene Drei-DB-Suiten gleichzeitig. Die temporären
Peerproben wurden nicht als neue Produktregressionen eingeschleust.

## Exakte öffentliche CI

Alle folgenden Läufe wurden per GitHub-API am richtigen Produkt-SHA `40defdc` als
`completed/success` geprüft:

- Anlage39905577680, Struktur39905577665, Engine39905577655,
  Backend39905577678, Tabellen39905577617, Mutationen39905577615,
  Insert39905577609, Web39905577711 und Dokumentation39905577731.

Native Linux-CI und lokale macOS-Prüfungen sind getrennte Nachweise. Unveränderte
Compose-/Swarm-/Webbrowserpfade wurden lokal nicht nochmals ausgeführt; die vorhandenen
exakten Kandidaten-CIs bestätigen diese unveränderten Bereiche zusätzlich.

## Öffentliche Daten, Ressourcen und Fortsetzung

Eigener Code/Review AGPL-3.0-only, ursprüngliche Gitidentität unverändert. Keine
produktiven Verbindungsdaten, privaten Profile, Schlüssel oder Maschinenpfade im
Review. Alle Fixtures verwendeten ausdrücklich synthetische Credentials.

Nach Abschluss nur die vier fremden laufenden Container vorhanden; keine eigenen
B4a-Container/Volumes oder lokalen Prüfprozesse verbleiben. Host-Swarm `inactive`.
Kein globales Prune und keine fremden Ressourcen/OSrechte geändert. Reviewworktree
enthält anschließend ausschließlich diese zusätzliche Reviewdokumentation.

Nächster Schritt: Projektleitung erteilt konkrete Agentintegrations-/Freezefreigabe;
Integrationsagent übernimmt geprüfte Produkt-/Handoff-/Reviewprovenienz und prüft den
kombinierten Hauptstand. Ein Clientpaket benötigt den danach freigegebenen festen
Vertrag/Implementierungspin, keine zufällige Geschwisterdatei.
