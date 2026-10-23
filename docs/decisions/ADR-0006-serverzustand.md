# ADR-0006: Persistenter Serverzustand für mehrere Replikate

Status: **Vorschlag**, noch nicht akzeptiert. Keine Implementierung, keine Produktfreigabe.
Datum: 23.10.2026. Betrifft PROD-004 und PROD-006 in [PARITÄT](../PARITAET.md).

Dieses Dokument trennt Anforderung, Vorschlag und Entscheidung. Erst der Abschnitt
„Entscheidung“ wird nach Review und Nutzerfreigabe verbindlich; bis dahin gilt der
dokumentierte Ist-Zustand (persönlicher Dienst, ein gemeinsames Bearer-Token, kein
Serverzustand, siehe [ARCHITEKTUR](../ARCHITEKTUR.md) und [BETRIEB](../BETRIEB.md)).

## 1. Anforderung

Aus [KONZEPT](../KONZEPT.md) und [ADR-0003](ADR-0003-compose-und-swarm.md) folgen:

- Passkeys/WebAuthn, optionale verschlüsselte Zugangsdaten und Backend-Teams mit
  expliziten Freigaben gehören zum Zielumfang, sind aber nicht umgesetzt.
- Dafür braucht der Go-Server **eigenen persistenten Zustand**: Konten, Passkey-
  Credentials, Sessions, Rollen/Teams, Freigaben, ggf. verschlüsselte Zugangsdaten
  und Audit-Metadaten.
- Das Produkt muss mit Compose und Swarm betreibbar sein. Der Serverzustand darf
  mehrere Replikate nicht verhindern; ein Replikat darf ausfallen oder neu starten,
  ohne dass Konten, Credentials oder Freigaben verloren gehen oder inkonsistent werden.
- Das [Bedrohungsmodell](../security/BEDROHUNGSMODELL.md) verlangt: Server-Schlüssel
  getrennt vom Datenspeicher, Autorisierung pro Objekt/Mandant/Operation, persönlicher
  Tresor nie per Adminrecht lesbar, Passkey-Login ist kein Entsperrweg
  (PMA-UX-009, [ADR-0005](ADR-0005-vault-format.md)).
- Nutzerwunsch: die Lösung soll nicht unnötig kompliziert werden.

Nicht Gegenstand: SQLite-Nutzerdatenbanken. Sie bleiben gemäß
[ADR-0004](ADR-0004-sqlite-adapter.md) lokal, ein Replikat, exklusives lokales Volume;
SQLite auf NFS ist dort ausgeschlossen. Diese Grenze ändert dieser ADR nicht. Der
Serverzustand ist ein **anderer** Datenbestand als die verwalteten Nutzerdatenbanken
und wird getrennt gespeichert.

## 2. Optionen und Bewertung

### (a) Netzwerk-Volume (NFS o. ä.) mit Dateizustand (z. B. SQLite/bbolt)

- Konsistenz/Locking: Dateisperren über NFS sind je nach Version/Mount unzuverlässig;
  das ist derselbe Grund, aus dem ADR-0004 NFS ausschließt. Mehrere schreibende
  Replikate bräuchten ein eigenes Locking. Gilt auch für SQLite-WAL über Netz.
- Betrieb: NFS-Server wird zum einzelnen Ausfallpunkt und muss selbst gesichert,
  aktualisiert und abgesichert werden. Swarm kennt kein eingebautes verteiltes Volume;
  Volume-Plugins/Mounts sind pro Umgebung verschieden.
- Sicherheit: Dateiberechtigungen und uid-Mapping über NFS fehleranfällig, Zugang
  meist unverschlüsselt im LAN; Zustandsdaten mit Credentials-Hashes liegen als Datei.
- Ausfallverhalten: Hängendes NFS blockiert I/O; Schreiben bei Teilausfall kann
  Daten korrumpieren statt sauber zu fehlschlagen.
- Komplexität: scheinbar niedrig, tatsächlich verlagert in Betrieb und Fehlersuche.
- Fazit: nur als Einzelreplika-Notlösung vertretbar, nicht für mehrere Replikate.

### (b) Eigener Sync-/Konsensprozess zwischen Servern

- Konsistenz: möglich (z. B. Raft), aber Eigenentwicklung oder Einbettung einer
  Bibliothek; Konten, Sessions und Rechte sind sicherheitskritisch, Fehler in
  Replikation oder Splitbrain wären Autorisierungsfehler.
- Locking/Betrieb: Quorum braucht mindestens drei Knoten, Mitgliedschaftsverwaltung,
  Snapshots, Upgrades, Backup/Restore des Clusters, Zeit-/Netzwerkpartitionen.
- Sicherheit: eigenes Inter-Server-Protokoll mit mTLS und Schlüsselverwaltung.
- Ausfallverhalten: ohne Quorum kein Schreiben; Wiederherstellung nach Totalausfall
  nicht trivial.
- Komplexität: mit Abstand am höchsten; widerspricht dem Wunsch nach Einfachheit
  und dem Ziel eines überschaubaren Betriebs. Prüfaufwand (Jepsen-artige
  Tests) wäre unverhältnismäßig.
- Fazit: abgelehnt für das Zielumfeld.

### (c) Externe MariaDB für den Serverzustand

- Konsistenz/Locking: bewährte ACID-Transaktionen, Zeilenlocks, Unique-Constraints
  (z. B. für Credential-IDs, Session-Rotation); alle Replikate sind zustandslos
  gegenüber einer Quelle der Wahrheit.
- Betrieb: weit verbreitet, Backup/Restore (Dump, Snapshots, PITR) und Replikation/
  Galera existieren als Standardwerkzeuge. Hochverfügbarkeit liegt beim Betreiber
  der DB (kann auch eine einfache Einzelinstanz sein) und ist nicht Pflicht des
  Servers. Compose: ein DB-Dienst im Stack oder externer Host. Swarm: externe DB
  oder Dienst mit eigenem Volume; der App-Dienst kann beliebig skalieren.
- Sicherheit: TLS mit Serverzertifikatsprüfung, eigene Datenbank und eigener User
  mit Mindestrechten (kein DDL im Laufbetrieb, DDL nur für Migrationsuser),
  Passwort per Docker-Secret, sensible Felder zusätzlich auf Anwendungsebene
  verschlüsselt (Server-Schlüssel getrennt von der DB, Bedrohungsmodell).
- Ausfallverhalten: DB nicht erreichbar bedeutet, dass der Server keine neuen
  Logins/Freigaben bedienen kann (fail closed); Healthcheck zeigt Zustand an.
  Bereits laufende Verbindungen zu Nutzerdatenbanken könnten theoretisch weiterlaufen,
  das ist eine offene Frage (siehe 4).
- Komplexität: geringste für Mehrreplikat-Betrieb; zusätzliche Abhängigkeit
  (eine DB), Migrationen und Versionsmatrix.
- Zusätzlich möglich: dieselbe Treiberbasis wird bereits für MySQL/MariaDB-
  Zielverbindungen genutzt, trotzdem Pools und Konfiguration strikt trennen.

### Vergleich

| Kriterium | (a) NFS/Datei | (b) Eigener Konsens | (c) MariaDB |
|---|---|---|---|
| Konsistenz bei mehreren Replikaten | schwach, Locking unsicher | möglich, Eigenbau | stark (ACID) |
| Betriebsaufwand | mittel, Fehlersuche schwer | hoch | mittel, bekannte Werkzeuge |
| Sicherheitsfläche | Dateirechte, NFS | eigenes Protokoll | TLS, DB-Rechte |
| Compose/Swarm | umgebungsabhängig | aufwendig | gut |
| Ausfallverhalten | Hänger, Korruptionsrisiko | Quorumverlust | klarer Fehler, fail closed |
| Komplexität/Prüfaufwand | gering scheinbar | sehr hoch | gering bis mittel |

## 3. Vorschlag

Der Vorschlag ist (c): **externe MariaDB als einziger Speicher des Serverzustands**.
Einzelreplika-Betrieb ohne diese DB bleibt der heutige Zustand (kein Serverzustand,
Bearer-Token) und wird nicht entfernt.

Vorgeschlagene Leitplanken:

- Eigene Datenbank (Schema) und eigener Benutzer für die Anwendung; getrennt von
  allen vom Nutzer verwalteten Zieldatenbanken. Laufzeituser: nur
  SELECT/INSERT/UPDATE/DELETE auf diesem Schema. Ein getrennter Migrationsuser führt
  DDL aus und wird nur beim Migrationsschritt genutzt.
- TLS zur Datenbank verpflichtend mit Zertifikatsprüfung; keine Klartext-Verbindung
  außer ausdrücklich gekennzeichnetem lokalem Entwicklungsmodus.
- Eigene, versionierte, vorwärts gerichtete Migrationen im Repository, mit
  Migrationstabelle und Sperre gegen paralleles Migrieren mehrerer Replikate
  (`GET_LOCK` oder gleichwertig). Kein Auto-DDL durch den Laufzeituser.
- Anwendungsseitige Verschlüsselung sensibler Felder (AEAD, Schlüsselkennung pro
  Datensatz, Rotation möglich). Der Serverschlüssel kommt aus einem Docker-Secret
  und liegt nie in der Datenbank oder im Image. Persönliche Tresorinhalte nach
  ADR-0005 bleiben clientseitig verschlüsselt; der Server speichert dafür nur
  Chiffretext.
- DB-Passwort, Serverschlüssel und ggf. CA per Docker-Secret/Compose-Dateisecret,
  Namensschema wie bisher versioniert (`dbmanager_..._v1`).
- Sessions: Speicherung als gehashte Tokens mit Ablauf in der DB, damit jedes Replikat
  sie prüfen kann; Rotation und Widerruf über Zeilenänderung.
- Healthcheck unterscheidet „Prozess lebt“ und „Zustandsspeicher erreichbar“.
- Die DB ist die einzige Quelle der Wahrheit; kein zusätzlicher Zustand in Dateien.

Begründung kurz: (c) verlagert Konsistenz und Locking an ein ausgereiftes System,
vermeidet verteiltes Locking auf Dateien (a) und Eigenbau-Konsens (b), nutzt
Standardwerkzeuge für Backup und HA und hält den Server zustandslos genug für
Replikate.

Nachteile, die offen bleiben: zusätzliche Betriebsabhängigkeit, Betreiber muss
Backup/HA der DB organisieren, MariaDB-Betreiber sieht Metadaten (nicht
Tresorklartext). Kein HA-Versprechen aus diesem Vorschlag.

## 4. Offene Fragen

1. **WebAuthn RP-ID und Origin:** Welche Domain(s) sind RP-ID? Bei mehreren
   Hostnamen/Umgebungen und Reverse-Proxy: feste Konfiguration, kein Ableiten aus
   Host-Header. Folge für Domainwechsel (Credentials werden ungültig).
2. **Rolle der Passkeys:** Anmeldung am Server und ggf. zweiter Faktor, aber
   kein Tresor-Entsperrweg (PMA-UX-009, ADR-0005). Soll Passkey-PRF je als
   Entsperrhilfe geprüft werden, getrennt entscheiden.
3. **Sessionspeicher:** DB-Sessions vs. signierte, zustandslose Tokens;
   Widerrufbarkeit, Lebensdauer, Last auf der DB, Cookie-/CSRF-Modell für den
   Webclient (bisher kein Cookie, Token nur im Browserspeicher).
4. **Verhältnis zum Bearer-Token:** Bleibt das Datei-Token als Betriebsmodus
   „persönlich/ohne Konten“ erhalten (Vorschlag: ja, getrennt wählbar), und wie wird
   ein Wechsel/Koexistenz verhindert, die zu Rechteausweitung führt?
5. **Backup/Restore:** Konsistenzpunkt für DB plus Server-Schlüssel; Restore ohne
   Schlüssel macht verschlüsselte Felder unbrauchbar; Rotationsplan; Test des
   Wiederherstellens.
6. **Mandantentrennung:** Mandanten-Spalte in jeder Tabelle plus erzwungene
   Prüfung im Repository-Layer oder getrennte Schemas je Team; Auswirkung auf
   Migrationen und Rechte.
7. **MariaDB-Versionen:** Unterstützte Mindest- und LTS-Versionen, Zeichensatz
   (utf8mb4), Isolationsstufe, Verhalten bei Galera/Replikation; Kompatibilität
   mit MySQL ausdrücklich ja/nein.
8. **Verfügbarkeit bei DB-Ausfall:** Dürfen bestehende Sessions für reine
   Lese-/Abfrageaktionen kurz weiterlaufen oder fail closed? Vorschlag: fail closed.
9. **Audit:** Wo liegt das Auditprotokoll und wie wird Manipulation erschwert.
10. **Betrieb ohne externe DB:** Soll ein Einzelinstanz-Modus mit lokalem Speicher
    (Datei) für Konten überhaupt angeboten werden oder nur MariaDB?
11. **Treiber/Lizenz:** Auswahl und Inventarisierung des Go-Treibers
    (LIZENZEN), Pooling und Zeitlimits.

## 5. Vorgeschlagenes erstes Umsetzungspaket (SZ1, begrenzt)

Ziel: tragfähige, geprüfte Grundlage ohne Nutzerfunktion (keine Passkeys, keine Teams).
Dateibesitz: neues internes Paket für Zustandsspeicher und Migrationen, Konfiguration,
Compose-/Swarm-Beispiel als optionales Profil, Betriebsdoku. Bestehende API-Verträge
bleiben unverändert.

Inhalt:

- Konfiguration der Verbindung (Host, Datenbank, User, CA, TLS-Pflicht) aus Dateien/
  Docker-Secrets, nie aus Argumenten oder Logs.
- Migrationsrunner mit Versionstabelle, Sperre und trockenem Planmodus; erste
  Migration enthält nur Schemaversion und eine Testtabelle für Schlüsselkennungen.
- Feldverschlüsselung (AEAD) mit Schlüsselkennung, Roundtrip- und Fehlerfalltests.
- Healthcheck-Erweiterung: Zustandsspeicher erreichbar oder nicht (optional, wenn
  konfiguriert).
- Optionales Compose-Profil mit MariaDB-Dienst und Swarm-Beispiel; Betriebsdoku.

Abnahmekriterien:

1. Zwei Replikate gegen dieselbe MariaDB starten; Migration läuft genau einmal,
   zweiter Start wartet oder erkennt Stand (praktischer Nachweis mit Protokoll).
2. Verbindung ohne gültiges TLS-Zertifikat wird abgelehnt; Laufzeituser kann kein
   DDL und nichts außerhalb seines Schemas (Negativtests).
3. Neustart und Löschen eines Replikats verlieren keinen Zustand; DB-Ausfall führt
   zu fail closed und `unhealthy`, Wiederkehr führt zu Erholung ohne Neustart.
4. Verschlüsselte Felder sind im DB-Dump nicht im Klartext; falscher oder fehlender
   Schlüssel führt zu sauberem Fehler, kein Panic, kein Klartext im Log.
5. Secrets erscheinen nicht in Logs, Fehlermeldungen, Imageebenen oder
   Stack-Konfiguration (Prüfskript).
6. Ohne Konfiguration verhält sich der Dienst unverändert wie heute
   (Bearer-Token, kein Zustandsspeicher); bestehende Tests bleiben grün.
7. Unabhängiges Review, Prüfstand und offene Punkte im Handoff dokumentiert; Swarm
   nur auf isoliertem Daemon wie in BETRIEB; kein Multinode-/HA-Anspruch ohne Nachweis.

Nicht in SZ1: WebAuthn, Konten-UI, Teams, Freigaben, Sessionumstellung.
Diese folgen nach Annahme dieses ADR und Klärung der offenen Fragen 1–4 in eigenen Paketen.

## 6. Entscheidung

Offen. Dieser ADR wird erst mit Review und ausdrücklicher Freigabe auf
„akzeptiert“ gesetzt; bis dahin sind PROD-004/PROD-006 unverändert offen.
