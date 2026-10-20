# Bedrohungsmodell für Zugangsdaten, Synchronisierung und Teams

Stand: 20.10.2026. Paket P3a, Vorschlag zur unabhängigen Prüfung; keine freigegebene
Produktkryptografie und kein Nachweis vollständiger P3-Abnahme.
Anforderungen: [Projektübergabe](../../PROJEKTUEBERGABE.md),
[Projektplan](../PROJEKTPLAN.md). Umsetzbare Schritte und Quellen:
[Entwurf und Befunde](TRESOR-UND-SYNC.md). Prüffälle: [Prüfplan](PRUEFPLAN.md).

## Schutzgüter und getrennte Identitäten

Schutzgüter sind DB-Passwörter, SSH-Schlüssel samt Passphrasen, TLS-Clientschlüssel,
Tresorschlüssel, Wiederherstellungsmaterial, Zugriffstoken sowie Datenbankinhalte.
Auch Hostnamen, Benutzernamen, Schema-/Tabellennamen, SQL-Texte, Ergebnisse,
Verbindungsnamen und Ausführungszeitpunkte können vertraulich sein.

Vier Prüfungen bleiben unabhängig: Apple-Account/iCloud-Zugriff, Backend-Anmeldung,
lokales Tresorentsperren und DB-Authentisierung/-Autorisierung. Ein gültiger
Backend-Login oder ein Kontoreset gewährt keinen persönlichen Tresorschlüssel.
Die DB entscheidet über SQL-Rechte; eine UI-Rolle ist kein Ersatz dafür.
Eine gelesene Verbindung bedeutet nicht automatisch das Recht zur Weitergabe,
Geheimnisanzeige, Ausführung oder Anlage eines Hintergrundjobs.

## Vertrauensgrenzen und Datenwege

| Grenze | Erlaubter Datenfluss | Sicherheitsbedingung |
| --- | --- | --- |
| Apple-UI → lokaler Swift-Kern → DB | Nach Entsperren kurzzeitig Klartext im Prozess; direkte verschlüsselte DB-Verbindung | Kein Go-Server, Backend-Konto oder iCloud für lokale Nutzung vorausgesetzt; TLS mit verifiziertem Ziel |
| Lokaler Prozess → Dateisystem/Backup | Verschlüsselte persönliche Datensätze; Schlüsselhüllen und notwendige Versionsmetadaten | Klartextpasswörter und SQL-Ergebnisse werden nicht automatisch persistiert; Dateischutz zusätzlich zur AEAD |
| Lokaler Prozess → Keychain | Explizit zugelassene lokale Entsperrhülle | Gerätebindung, Zugriffsschutz und Verlustverhalten dokumentieren; kein stilles Sync-Flag |
| Lokaler Prozess → CloudKit private Datenbank | Bereits durch eigenen Tresor verschlüsselte persönliche Nutzdaten | CloudKit ist Transport, kein eigener Entsperrweg; keine Schlüssel oder Passwörter im Klartext hochladen |
| Browser → Go-Ausführungsdienst → DB | Bewusst zur Ausführung übergebenes DB-Geheimnis über HTTPS; im Go-Prozess Klartext | Backend und Betreiber werden für diese Ausführung vertraut; keine Behauptung, Server könne solche Geheimnisse nicht sehen |
| Persönlicher Tresor → Teamdienst | Nur ausdrücklich gewählte Verbindung, freigegebene Felder und Geheimnisse | Eigene Teamkopie/Schlüsselhülle, keine Übertragung des persönlichen Root-Schlüssels |
| Go-Dienst → Team-Speicher | Verschlüsselte Teamgeheimnisse, ACLs, Audit-Metadaten | Server-Schlüssel getrennt vom Datenspeicher; Betreiber bleibt bei serverseitiger Ausführung vertrauenswürdig |
| Go-Dienst → beliebiges Verbindungsziel | SQL/TLS/SSH zu vom Benutzer gewähltem Host | Netzwerkzielrichtlinie, DNS-Auflösung, Rebinding- und Weiterleitungsprüfung; Freigabe interner Ziele ist eine Betreiberentscheidung |
| Build/Container → Host/Swarm | Versionierte Binärdateien, nur nötige Secrets | Image/Repository enthalten keine echten Secrets; Containerisolation ersetzt kein Vertrauen zum Hostadministrator |

Die optionale Go-Instanz ist für die alleinstehende Apple-App keine
Verfügbarkeitsabhängigkeit. Ein späterer Web-Tresor kann clientseitig entschlüsseln,
doch ein kompromittierter Server kann verändertes JavaScript liefern. Schutz vor
einem aktiv bösartigen Betreiber lässt sich daher im ausgelieferten Webclient
nicht allein durch verschlüsselte Speicherung behaupten.

## Angreifer und Gegenmaßnahmen

| Fall | Erwartete Gegenmaßnahme | Verbleibende Grenze |
| --- | --- | --- |
| Gestohlener Datenträger oder CloudKit-/Speicherabzug | Zufällige Schlüssel, Passwort-KDF, authentifizierte Verschlüsselung, verschlüsselte Metadaten | Schwaches Masterpasswort erlaubt Offline-Raten; Größe, Zeitpunkt und Objektanzahl können sichtbar bleiben |
| Manipulierter, abgeschnittener oder vertauschter Datensatz | AEAD prüft Nutzdaten und Kontext aus Tresor-ID, Datensatz-ID, Version und Zweck | Authentizität beweist keine Aktualität |
| Rückspielen gültiger alter Tresordaten | Vertrauenswürdiger lokaler Versionshöchststand, Konfliktaufbewahrung, Rotationsepochen | Neues Gerät ohne unabhängigen aktuellen Anker kann vollständiges gültiges Backup-Rollback nicht erkennen |
| Netzangreifer oder falsche DB | HTTPS/DB-TLS mit Hostnamenprüfung; SSH-Hostschlüssel prüfen, kein automatisches Akzeptieren geänderter Schlüssel | Benutzer kann einer falschen CA/einem falschen Host bewusst vertrauen; Netzwerkverfügbarkeit bleibt angreifbar |
| Gestohlenes gesperrtes Gerät | OS-Dateischutz, Keychain-Gerätebindung, Sperrpolitik | Auf entsperrtem/kompromittiertem Gerät und bei Prozessspeicherzugriff besteht keine Vertraulichkeitsgarantie |
| XSS/kompromittierte Webabhängigkeit | CSP ohne unnötiges Inline-Skript, kontextgerechtes Escaping, keine Secrets im DOM/Browserstorage, Abhängigkeitskontrolle | Ausführbarer Schadcode im entsperrten Client kann Schlüssel verwenden |
| Fremdes Teammitglied/erratene Objekt-ID | Autorisierung pro Objekt, Mandant und Operation; Geheimnisabruf getrennt von Metadaten | Legitim kopierte Geheimnisse werden durch späteren Rechteentzug nicht vergessen |
| Bösartiger Teamadministrator | Persönlicher Tresor nie per Adminrecht lesbar; nur bewusste Freigaben | Betreiber eines Go-Ausführungsdienstes kann dort ausgeführte Team-/DB-Geheimnisse sehen |
| Rechteentzug während laufendem Job | Rechte vor Start und an definierten Fortsetzungsgrenzen prüfen; Sitzungen/Jobs abbrechen | Laufende externe DB-Wirkungen sind nicht rückholbar; DB-Credentials/DB-Sitzungen separat widerrufen |
| SSRF über DB-/SSH-Verbindung | Betreiberdefinierte Zielnetze/Ports, blockierte Metadaten- und Link-Local-Ziele, IP-Prüfung nach Auflösung | Interne Netze können legitime Ziele sein; keine pauschale Gleichsetzung mit öffentlichen Internetzielen |
| Logs, Crashberichte, Export, Zwischenablage | Strukturierte Fehlercodes; Redaction vor Logger; Export/Secretanzeige nur bewusst; keine automatischen SQL-/Ergebnislogs | Nutzerinitiierte Exporte und Screenshots verlassen den Schutzbereich |
| Ressourcenangriff über KDF/Import/SQL/Sync | Größen-/KDF-Grenzen vor Allokation, Zeitlimits, Abbruch und Rückstau | Ein fachlich gültiger großer Job benötigt trotzdem Ressourcen |
| Host-/Swarm-Manager kompromittiert | Minimalrechte, gesonderte Schlüssel, geprüfte Backups, Secretrotation | Swarm-Secrets sind vor berechtigten Hostadministratoren und dem Empfängerprozess nicht verborgen |

## Lebenszyklus und Fehlersicherheit

Entsperrzustände: `uninitialized`, `locked`, `unlocking`, `unlocked`, `locking`.
Synczustände sind davon getrennt: `disabled`, `offline`, `syncing`, `conflict`,
`accountChanged`, `recoveryRequired`. Ein Syncfehler sperrt keinen bereits lokal
entsperrbaren Tresor. Ein Cloud-Accountwechsel mischt keine vorhandenen Tresore.

Beim Sperren werden neue Operationen abgewiesen, wartende Geheimnisanfragen
abgebrochen und Schlüsselreferenzen freigegeben. Aktive Verbindungen werden
geschlossen oder benötigen eine ausdrücklich gewählte Sitzungsregel. Go-/Swift-
Speicherverwaltung garantiert keine vollständige Löschung aller Kopien; keine
unbelegte Speicherlöschgarantie formulieren. Hintergrundjobs, die nach dem Sperren
weiterlaufen sollen, benötigen gesondert freigegebene Dienstgeheimnisse und gelten
nicht als Zugriff auf einen gesperrten persönlichen Tresor.

Neues Gerät: verschlüsselte Daten laden, dann bestehenden Entsperrweg verwenden.
Verlorenes Gerät: Gerätesitzung und lokale Komforthülle widerrufen, aber annehmen,
dass bereits bekannte DB-Geheimnisse kopiert wurden. Echte Abhilfe erfordert neue
DB-/SSH-Zugangsdaten und gegebenenfalls eine neue Tresorepoche. Ein Sync-Löschbefehl
an ein dauerhaft offline gebliebenes Gerät ist kein sicherer Fernlöschbeleg.

Wiederherstellung: mindestens ein unabhängiger wiederherstellbarer Entsperrweg wird
vor Nutzung persistent gespeicherter echter Geheimnisse geprüft. Bei Verlust aller
Wege ist der Tresor unentschlüsselbar. Support und Backend-Kontoreset dürfen dieses
Ergebnis nicht umgehen. Ein Wiederherstellungsschlüssel im gleichen iCloud-Account
ist kein unabhängiger Schutz vor vollständigem Accountverlust.

## Freigabegrenzen

Die ersten Produktinkremente verwenden ausschließlich flüchtige Zugangsdaten.
Der getrennte [Spike](../../spikes/security/README.md) bestätigt nur begrenztes
AEAD-Verhalten und API-Kompilierbarkeit. Vor echten Tresordaten fehlen unter
anderem KDF-Auswahl samt Bibliotheks-/Lizenzprüfung, Dateiformat, atomare Updates,
Gerätetests, Keychain-Zugriffspolitik, Recovery-Rundlauf, Konflikt-/Rollbacktests,
Interoperabilität und unabhängige Sicherheitsprüfung. Teams und iCloud werden
jeweils separat freigegeben; eine bestandene Go-Unitprüfung ersetzt keine davon.
