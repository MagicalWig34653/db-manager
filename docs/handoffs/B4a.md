# B4a – strukturierte neue Tabelle

Prüfkandidat vom 23.10.2026. Eigener Branch `feat/table-create`, Worktree
`.worktrees/table-create`, Basis `8aab52bc9be8ccaf84dbdb0eda61ec46a60181d3`.
Nach delegierter B3a-Integration ausdrücklich auf diesen veröffentlichten Stand
gebracht; keine B3a-Bytes nachgepatcht. Keine Selbstfreigabe/Integration oder
API 0.6-Clientbindung vor unabhängigem Review und exakter grüner nativer CI.

## Ergebnis, Besitz und Grenze

Neue Core-/DB-/HTTPdateien samt enger Routenregistrierung, eigener API 0.6-Kandidat,
[Verhaltensvertrag](../../contracts/TABLE-CREATE-API.md), Regressionen,
`test-create.py`, Vertragswächter und `create.yml`. Keine Web-/Main-/Docker-/GoMod-/
Vault-/Lizenzbestandsänderung; v1–v5-Vertragsbytes und vorhandene Routen unverändert.
Eigener Code AGPL-3.0-only. Keine neue Produkt-/Buildabhängigkeit; vorhandene
Treiberpins und Originallizenzen bleiben unverändert.

Maximal32 Spalten mit festen BIGINT/VARCHAR/Binary/Decimal-Abbildungen, explizit
Nullable und Defaultmodus; geordneter nichtnullable PK mit maximal 8 Int64-/Text-
Spalten, auch leere PKliste zulässig. Feste Größen-/Stellen-/Zeit-/Requestgrenzen,
keine freien SQL-Typ-/Default-/Constraintfragmente. Keine Identity/Auto-ID,
Generated/FK/CHECK/Kollations-/Engineauswahl/DDL-Batches. MySQL/Maria InnoDB/utf8mb4.
SQLite bleibt flexibel typisiert; BIGINT-PK ist kein INTEGER-Rowid-Alias,
VARCHAR-/Decimaldeklarationen garantieren keine Längen-/Dezimalspeicherpräzision.
Keine allgemeine Schemaverwaltung oder Parität.

CREATE ohne IF NOT EXISTS/REPLACE entscheidet Namensraces. Vorablookup ist nur
advisory; bestehende Tabellen werden nicht verändert/entfernt/geleert. Default-
literale stammen ausschließlich aus validierten typisierten Konstanten unter
festgesetztem Sessionmodus. Katalogdefault-SQL wird nie ausgeführt. Keine
permanenten Produktmarker, automatischen Retries, Reparatur oder Drop-Aufräumlogik.

Antwortmodell gemäß ausdrücklich abgestimmtem Vertrag: created bestätigt den
CREATE-Abschluss bei MySQL/Maria bzw. Commit bei SQLite/PG; Strukturprüfung ist
separat transactional/observed/different/unavailable. SnapshotAtomic bedeutet
nur zusammengehörige DDL-/Metadatenprüfung derselben echten SQLite-/PG-Transaktion,
keinen Zustand nach Commit/Antwort und keine fremden/externalen Effekte. MySQL-MDL
schützt nur die nachfolgende separate Beobachtung ab Lockaufnahme, keine physische
Objektidentitätskontinuität seit CREATE. Sicheres Ack plus Lesefehler ergibt
created/unavailable; fehlendes Ack/Commitfehler outcome_unknown, kein blindes Retry.
Rückgelesene Attribute sind begrenzt auf Identität, geordnete Spalten/PK, Typen,
Nullable, bekannte konstante Defaultdarstellungen und fehlende Generated-Eigenschaft.
Unbekannte Katalogdarstellung kann konservativ die Gleichheitsbestätigung verhindern.

## Tatsächliche Prüfungen

- Abschließend `go test -race -count=1 ./...` vollständig grün und `go vet ./...`
  bestanden. V1–v5-Byte-/Definition-/Routenwächter sowie v5-Wächter grün.
- Eigener tatsächlicher TLS-Lauf `python3 scripts/test-create.py`: MySQL8.4,
  MariaDB12.3 und PostgreSQL18.6 unter Race grün. SQLite-Fälle im vollständigen
  Racelauf. Eigene CA/Hostnamenprüfung über bestehende Netzwerkpolicy; keine neuen
  TLS-Ausnahmen oder Downgrades.
- Alle vier Treiber: Composite-PK, alle vier Typen, Default-/NULL-Grenzen,
  signed64-Maximum als echter Default und zurückgelesener Integerstring,
  tatsächliche Binarybytes00FF→AP8=, Sondernamen mit Quote/Backtick und wörtlichem
  DROP-TABLE-Text, injektionsartige Textdefaults mit Apostroph/Backslash. Das
  unbeteiligte Fixtureobjekt bleibt bestehen. Erneute Anlage liefert Konflikt und
  bewahrt vorhandene Daten. Ohne PK und explizite andere Schemas ebenfalls geprüft.
- PostgreSQL: konkurrierender uncommitteter CREATE hält Namen; zweiter Request
  bricht am Zeitlimit ab. Nach Commit gewinnt ausschließlich die ursprüngliche
  Tabelle, kein Überschreiben. Reale fehlende CREATE-Rechte sicher abgewiesen.
  Eigener DDL-Eventtrigger erzeugt eine deferred FK-Verletzung: echter COMMITfehler
  liefert outcome_unknown; anschließender Adminlookup bestätigt tatsächlichen
  Rollback des Fixtures. Daraus keine universelle Commit-Ausfallphasengarantie.
- MySQL/Maria: eigener CREATE-only-User darf anlegen, aber keine SELECT-MDL nehmen;
  tatsächlich created/unavailable und Tabelle per Admin separat nachgewiesen.
  Eigener Fixture-Globalreadlock blockiert CREATE; Zeitabbruch liefert ohne sicheres
  Ack outcome_unknown, danach Conn-/Slotfreigabe und weitere Anlage erfolgreich.
  Keine Aussage, dass unterbrochenes DDL niemals noch wirksam werden kann.
- Echte separate MySQL/Maria-Lesetransaktion nimmt SELECT-LIMIT0-MDL; konkurrierendes
  ALTER läuft am Zeitlimit aus, während die gehaltene Beobachtung unverändert bleibt.
  Keine Nachfreigabe-Zustandsgarantie. Zusätzlicher Test leitet echten CREATE weiter,
  verändert das Fixture über eine zweite Verbindung vor der Beobachtung und erhält
  created/different samt tatsächlichen zwei Spalten; keine Reparatur/Entfernung.
- Faultproben an der tatsächlichen internen Ausführungsgrenze: verlorenes Ack,
  Metadatenfehler, Commitfehler und zu große spätere JSON-Beobachtung unterscheiden
  unknown/created-unavailable korrekt. Kein Retry; Zähler belegen genau einen
  Statement-/Commitversuch. Diese simulierten Fehler sind von echten DB-Proben
  getrennt; kein vollständiger Netzwerk-/Powerlossnachweis behauptet.
- SQLite-Busy/anschließende Freigabe, Spalten-/Typ-/Skalen-/PK-/Zellvalidation,
  unzulässige Katalogausdrücke, tatsächlich encodierte Antwortgrenze. Ein anfänglich
  roter Busytest hatte nur den lazy sql.DB-Wrapper geöffnet; nach echtem Ping wird
  die Dateisperre tatsächlich gehalten und der Test ist grün. Kein Produktfix daraus.
- HTTP: Auth vor DB-Aufruf; Capabilities1–6, alle neuen Pflichtfelder, explizite
  Null-/fehlende Werte, gültiger Leertext, unzulässige Zusatztypfelder/SQLtyptexte,
  Bodylimit, No-Store, echter SQLite-Erfolg und409 ohne Überschreiben.
- Einmalige zusätzliche Validierung im entfernten temporären venv mit exakt
  jsonschema 4.23.0: alle Komponenten gegen Metaschema, echte lokale HTTP-Capabilities,
  Anfrage mit Decimal-scale0 und tatsächliche Antwort sowie fehlendes Pflichtfeld
  geprüft. Keine neue Repo-/CI-Abhängigkeit daraus; CLI/venv/Token/DB entfernt.
- Docs 288/0 und Diffcheck bestanden. Keine unveränderten Web-/Compose-/Swarmproben
  neu ausgeführt; weder Betriebsdateien noch Dateisandbox geändert. Native CI wird
  am Produktcommit getrennt geprüft, lokale macOS-Ergebnisse ersetzen sie nicht.

## Unterwegs behobene Test-/Darstellungsbefunde

Die erste Compileprobe hatte eine falsche Testhelper-Referenz, korrigiert. Danach
zeigte MariaDB beim normalen Defaultvergleich eine andere SQLliteral-Darstellung:
Textwerte sind gequotet und Backslashes doppelt, MySQL liefert rohe Textwerte.
Die Engine ermittelt den Hersteller auf derselben Session und vergleicht nur dessen
belegte Konstantendarstellung. Erneute komplette Matrix einschließlich der
ursprünglichen Default-/Injectionfälle grün. Unbekannte Ausdrücke werden nicht durch
SQL-Auswertung oder beliebige numerische Ausdrucksinterpretation normalisiert.

## Ressourcen und nächster Schritt

Nur zufällige `dbmanager-b4a-*` Container, dynamische Loopbackports, eigene
kurzlebige CA-/synthetische Credentialdateien und PostgreSQL-Rollen/Eventtrigger
innerhalb dieser ausschließlich eigenen Wegwerf-DBs verwendet. Kein fremder
Container/Host-Swarm/OS-Recht geändert und kein globaler Docker-Prune. Finally-
Cleanup entfernt eigene Container/Volumes/Temporärdateien; Abschlussinventar und
exakte Commit-/CI-IDs folgen im reinen Übergabecheckpoint.

Noch offen: unabhängiger Backend-/Sicherheitsreview, exakte neun nativen CIs und
separater0.6-Freeze vor Clientbindung. Kein Produktclient/Schema-Editor hier.
