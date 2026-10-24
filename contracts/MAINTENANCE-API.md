<!-- SPDX-License-Identifier: AGPL-3.0-only -->
# Tabellenwartung (API 0.11.0, vorläufig)

Die Nummer ist ein Planvorschlag; der Lead vergibt sie vor Integration endgültig.
[Schema](api-v0.11.0.json) erweitert den im Paket vorhandenen Vertrag v7 additiv.
V1 bis v7 bleiben unverändert; parallel entstehende v8–v10 werden hier nicht kopiert.
Kein Pin im Web-Lock. Alle Routen verlangen das Bearer-Token und JSON; Verbindung
und Geheimnisse bleiben flüchtig. Unbekannte Felder und explizite Nullwerte werden
in Anfragen abgewiesen. Keine freien SQL-Fragmente oder Optionsnamen.

| Route | Zweck |
| --- | --- |
| `GET /api/v11/capabilities` | Verfügbarkeit der Engine, statische Fähigkeiten je Treiber/Aktion und Grenzen |
| `POST /api/v11/tables/maintenance/plan` | Vorschau ohne Ausführung, Ergebnis je Eintrag |
| `POST /api/v11/tables/maintenance` | Bestätigte Ausführung, Ergebnis je Eintrag |

## Anfragen und Vorschau

`connection`, `items` (1 bis 50) und optional `timeout_ms` (0 bis 60 000; 0/fehlend
bedeutet 10 000) bilden die Anfrage. Das Zeitbudget gilt für den gesamten Stapel,
einschließlich Öffnen, Kataloglesen, erneuter Planung und Ausführung. Einträge werden
in Eingabereihenfolge auf derselben dedizierten Verbindung bearbeitet. Jeder enthält:

* `table`: `name` und optional `schema`; Standard `main` (SQLite), Verbindungsdatenbank
  (MySQL/MariaDB) oder `public` (PostgreSQL).
* `action`: `check`, `analyze`, `optimize`, `repair`, `checksum`, `vacuum` oder `reindex`.
* `full`, `analyze`: optionale Boolesche Werte, nur bei `vacuum` wirksam/zulässig;
  `true` bei anderen Aktionen wird je Eintrag abgelehnt.

Eine Vorschau liefert `items` mit `state=planned` oder `rejected`, `reason`,
`table`, `action`, `plan`, `typed_confirmation` und `target_name`. Bei Ablehnung ist
`plan=null`, `target_name=""` und `typed_confirmation=false`. Bei einem gültigen Plan
enthält `plan` die exakten Wartungsanweisungen, deutsche Warnungen zu unvorhersehbarer
Laufzeit, Sperren und Abbruch, betroffene Objekte sowie `plan_sha256` aus `dbplan`.
Die Antwort enthält die Katalogschreibweise; der Request muss exakt dazu passen.
Identifier werden ausschließlich nach Katalogauflösung über `dbplan` gequotet.
Sichten, virtuelle/fremde Tabellen und Systemtabellen sind ausgeschlossen.

## Bestätigung und Ergebnisse

Jeder Ausführungseintrag ergänzt die ursprünglichen Planfelder um
`expected_plan_sha256` und `confirm` mit `action` sowie gegebenenfalls
`target_name`. Vor jeder einzelnen Ausführung wird der Plan erneut aus dem Katalog
berechnet und mit `dbplan.Check` geprüft. `confirm.action` ist immer erforderlich.
OPTIMIZE, REPAIR, REINDEX und VACUUM FULL verlangen zusätzlich den exakt getippten
Katalognamen. Falscher/ungültiger Hash ergibt `rejected/plan_mismatch`, eine falsche
Bestätigung `rejected/confirmation_mismatch`. Diese Eintragsfehler erscheinen bei
HTTP 200, damit andere Einträge weiterlaufen; es gibt keine globale Transaktion.

`items` enthält für jeden Eintrag in derselben Reihenfolge `table`, `action`,
`outcome`, `reason`, `checksum` und `duration_ms`:

* `completed`: Engine bestätigt den Abschluss; leeres `reason`. Keine Zusage über
  dauerhafte Statistiken oder den späteren Tabellenzustand. CHECK meldet bei SQLite
  ausschließlich ein bestätigtes `ok`; MySQL verlangt eine abschließende Statuszeile
  `OK` oder `Table is already up to date` ohne Fehler/Warnung.
* `rejected`: keine Wartung ausgeführt oder lesende Prüfung ohne positives Ergebnis.
  Gründe: ungültige Optionen, Systemobjekt, Objekt nicht gefunden, nicht unterstützte
  Aktion/Engine/Tabelle, fehlende Rechte, Katalogfehler, Abbruch vor Ausführung,
  fehlgeschlagene Sitzungsvorbereitung, Plan-/Bestätigungsfehler,
  `integrity_check_failed` oder `checksum_unavailable`.
* `outcome_unknown/execution_unconfirmed`: Fehler nach dem Absenden, unterbrochene
  Verbindung, Zeitüberschreitung, fehlender Abschlussstatus oder nicht positiv
  bestätigtes Engineergebnis. Auch eine teilweise wirksame REPAIR-Aktion gehört hierher.
  Ausschließlich ausdrückliche Rechte-/Objektfehler des Treibers werden sicher als
  Ablehnung eingeordnet. Keine automatischen Wiederholungen.

`partial=true` bedeutet mindestens ein `completed` und mindestens einen anderen
Ausgang. Auch ohne Teilerfolg werden alle Eintragsergebnisse ausgegeben.
`checksum` ist nur bei erfolgreichem CHECKSUM eine Dezimalzeichenfolge bis 20 Stellen,
sonst `null`; keine Rundung über JavaScript-Zahlen. Prüfsummen sind enginespezifisch,
kein treiberübergreifender Inhaltsvergleich. Rohe Engine-Meldungen, SQL aus Fehlern,
Hostpfade und Zugangsdaten werden nie ausgegeben. Korruptionsdetails werden ebenfalls
unterdrückt. Rohmeldungen werden weder übernommen noch in feste Gründe eingebettet.

## Tatsächliche Fähigkeiten

| Aktion | SQLite | MySQL/MariaDB (Treiber `mysql`) | PostgreSQL ab 17 |
| --- | --- | --- | --- |
| CHECK | `PRAGMA main.integrity_check(<Katalogname>)`, nur Zieltabellenprüfung | InnoDB, MyISAM, Aria | nein |
| ANALYZE | `ANALYZE` der Tabelle | InnoDB, MyISAM, Aria | `ANALYZE` |
| OPTIMIZE | nein | InnoDB, MyISAM, Aria; Neuaufbau möglich | nein, gesondertes VACUUM |
| REPAIR | nein | nur MyISAM, Aria, ARCHIVE, CSV | nein |
| CHECKSUM | nein | InnoDB, MyISAM, Aria; NULL ergibt Ablehnung | nein |
| VACUUM | nein: ADR-0004 sperrt VACUUM und VACUUM INTO | nein | `VACUUM`, optional `FULL`, `ANALYZE` |
| REINDEX | nein | nein | `REINDEX TABLE`, getippte Bestätigung wegen Sperre |

Die Engine muss tatsächlich installiert und das notwendige DB-Recht vorhanden sein;
Fähigkeiten sind keine Rechteerteilung. Nicht aufgelistete MySQL-Engines werden
konservativ abgewiesen. PostgreSQL erlaubt hier nur lokale Basistabellen ohne
Vererbung oder Partitionierung (auch keine Partitionen als Ziel), um ungenannte
Kindtabellen auszuschließen; vorab Eigentümer-/MAINTAIN-Prüfung verhindert stilles
Überspringen ohne Recht. Ältere PostgreSQL-Versionen werden nicht als geprüft oder
unterstützt zugesagt. Index- und TOAST-Arbeit gehört zur gewählten Basistabelle.

SQLite erlaubt ausschließlich auf dedizierten Wartungsverbindungen das lesende
`integrity_check` mit Tabellenargument auf der PRAGMA-Positivliste. Normale
SQL-Verbindungen behalten ihre bisherige Positivliste unverändert. Keine Änderung an ATTACH-, VFS-, Journal- oder Dateiregeln;
VACUUM bleibt gesperrt. Ein eigener Sandboxtest prüft diese Grenze.

## Grenzen und Clientpflichten

HTTP 400 bei ungültiger Anfrage/fehlender Pflichtbestätigung, 401 ohne Token,
413 über 1 MiB, 415 ohne JSON, 422 ohne Enginefähigkeit. Verbindungsfehler vor der
Eintragsverarbeitung verwenden das bestehende sichere Fehlermodell. Eine defekte
Verbindung vor dem Stapel erzeugt keine erfundenen Eintragsergebnisse.

Sitzungsinitialisierung ist keine Wartungsanweisung: PostgreSQL bekommt vor jeder
Ausführung `statement_timeout` aus dem Restbudget und `lock_timeout` bis 1 s;
MySQL bekommt `lock_wait_timeout` aus dem auf Sekunden aufgerundeten Restbudget.
Diese festen, serverseitig erzeugten Einstellungen stehen nicht im Planhash.
MySQL hat kein allgemeines serverseitiges Zeitlimit für diese Wartungsbefehle;
Kontextabbruch schließt die Verbindung, serverseitige Arbeit kann noch weiterlaufen.
SQLite nutzt den Kontextabbruch des bestehenden Adapters. Höchstens 64 Statuszeilen
werden ausgewertet; Ergebnisse sind auf feste Statusfelder reduziert.

Der zustandslose Planhash deckt Treiber und normalisierte Anweisungen ab, keine
Objektidentität, Warnungen oder einmalige Nutzung. Zwischen Katalogprüfung und SQL
können andere Sitzungen Objekte austauschen; kein atomarer Schemasnapshot oder
Exactly-once-Versprechen. Nach `completed` oder `outcome_unknown` müssen Clients den
Eintrag sperren; weitere Wartung verlangt neue Vorschau und Bestätigung. Doppelte
Einträge werden separat bearbeitet; Clients sollen sie vermeiden. Nach Abbruch darf
kein automatischer Retry erfolgen. Nachfolgende Einträge werden weiter geprüft;
bei aufgebrauchtem Budget ohne weitere Wartung als abgelehnt ausgewiesen.

## Fixtures und Nachweise

[Fixtures](fixtures/maintenance/README.md) sind ausdrücklich handgeschriebene,
synthetische Platzhalter. Ihre Hashwerte belegen keine realen Enginepläne.
`scripts/check-maintenance-contract.py` prüft Altbytes, Additivität, Referenzen,
strikte neue Objekte und positive/negative Fixtures ohne neue Abhängigkeit.
`scripts/test-maintenance.py` startet über `dbfix.py` jeweils nur ein eigenes
DB-System, führt die Livefälle aus und validiert echte temporäre Engineantworten
gegen das Schema. Lokal nicht ausgeführte Systeme stehen im
[Handoff](../docs/handoffs/W-P10-backend.md); keine vollständige Paritätsabnahme.
