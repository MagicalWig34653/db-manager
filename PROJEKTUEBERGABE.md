# Projektübergabe: Moderne Datenbankverwaltung

Stand: 20. Oktober 2026

## Projektstatus

Wir planen eine moderne Alternative zu phpMyAdmin. Das Projekt befindet sich in der Brainstorming- und Konzeptphase. Es gibt noch keine verbindlich festgelegte Implementierung oder vollständige technische Spezifikation. Anforderungen des Nutzers und bisherige Architekturvorschläge sind unten getrennt gekennzeichnet.

## Verbindliche Ziele und Anforderungen

### Produktziel

- Schnelle, moderne Datenbankverwaltung mit mindestens dem gesamten Funktionsumfang von phpMyAdmin.
- Unterstützung weiterer Datenbanksysteme und mehrerer Server.
- Bedienung und Arbeitsabläufe sollen neu gedacht werden.

### Plattformen und Technologien

- Weboberfläche: Go-Backend mit Svelte und shadcn-svelte.
- Native Apple-App: Swift/SwiftUI für macOS und iPadOS; iPhone möglicherweise später.
- Die Swift-App muss vollständig ohne Go-Backend funktionieren und sich direkt mit Datenbanken verbinden können.
- Das Go-Backend ist für die Swift-App optional.
- Die Weboberfläche verwendet das Go-Backend.

### Betriebsarten

| Betriebsart | Verhalten |
| --- | --- |
| Swift-App ohne Backend | Eigenständiger persönlicher Datenbankclient mit direkten Datenbankverbindungen und iCloud-Synchronisierung |
| Swift-App mit Backend | Zusätzlich zentrale Verwaltung und Teamfunktionen über eine Go-Instanz |
| Weboberfläche mit Backend | Datenbankverwaltung und Multiuser-Betrieb über Go |

### Server und Zugangsdaten

- Mehrere Datenbankserver und Verbindungen unterstützen.
- Zugangsdaten können optional gespeichert werden.
- Ohne gespeicherte Zugangsdaten werden diese bei Bedarf abgefragt.
- Gespeicherte Zugangsdaten sollen verschlüsselt sein.
- Passkey-Anmeldung und Entsperren der Zugangsdaten per Passkey sind gewünscht.
- Ohne Passkey muss es ein Masterpasswort geben.

### Eigenständige Swift-App und iCloud

- Ohne Backend gibt es keine Freigaben an andere Benutzer und keine Teamfunktionen.
- Persönliche Daten werden über iCloud zwischen den eigenen Apple-Geräten synchronisiert.
- Die App muss ohne Go-Backend vollständig nutzbar sein.

### Multiuser

- Multiuser-Betrieb und Teilen sollen über das Go-Backend möglich sein.
- Der genaue Umfang von Rollen, Freigaben und Teamverwaltung ist noch festzulegen.

## Bisherige technische Entwürfe — noch zu validieren

### Verschlüsselter Tresor

- Jeder Benutzer erhält einen zufälligen Tresorschlüssel.
- Zugangsdaten werden mit zufälligen Datenschlüsseln verschlüsselt, die durch den Tresor geschützt werden.
- Ein aus dem Passkey abgeleiteter Schlüssel oder ein aus dem Masterpasswort abgeleiteter Schlüssel schützt den Tresorschlüssel.
- Für Passkey-basierte Schlüsselableitung wurde WebAuthn PRF betrachtet. Ein normaler Passkey-Login allein liefert keinen Verschlüsselungsschlüssel.
- Unterstützung je Betriebssystem, Browser und Passkey-Anbieter muss geprüft werden.
- Insbesondere die Passkey-Einrichtung und Wiederherstellung im eigenständigen Betrieb ohne Go-Backend sind noch zu klären.
- Argon2id für die Schlüsselableitung aus dem Masterpasswort wurde vorgeschlagen.
- Anmeldung und Tresorentsperrung werden technisch getrennt modelliert.
- Mehrere Passkeys, ein optionales zusätzliches Masterpasswort und ein Wiederherstellungsschlüssel wurden als weitere Entsperrwege vorgeschlagen.
- Ein Kontoreset allein darf keinen verschlüsselten Tresor entschlüsseln.

**Vertrauensgrenze:** Der Prozess, der die Datenbankverbindung aufbaut, muss die benötigten Zugangsdaten währenddessen entschlüsselt verwenden können. Bei direkten Verbindungen ist das die Swift-App, bei Backend-Verbindungen der Go-Server. Deshalb keine pauschale Zusage machen, dass das ausführende Backend niemals Zugangsdaten sehen kann.

### iCloud-Synchronisierung

Vorgeschlagener Umfang:

- Serververbindungen einschließlich TLS- und SSH-Einstellungen.
- Gespeicherte Zugangsdaten, zusätzlich durch den eigenen Tresor verschlüsselt.
- Gespeicherte SQL-Abfragen, Favoriten und Servergruppen.
- Einstellungen und optional offene Arbeitsbereiche.

Abfrageergebnisse, Exporte und SQL-Verlauf sollen nach bisherigem Vorschlag standardmäßig lokal bleiben. Eine optionale Verlaufssynchronisierung ist denkbar.

iCloud transportiert verschlüsselte Daten; ein gültiger Entsperrweg muss auch auf einem neuen Gerät verfügbar sein. Bereits lokal verfügbare Daten sollen ohne iCloud erreichbar bleiben und spätere Synchronisierung ermöglichen. Konflikte, besonders bei geänderten Zugangsdaten, müssen bewusst behandelt werden.

### Persönliche Bereiche und Teams

- Persönliche und gemeinsame Bereiche getrennt halten.
- Das Hinzufügen eines Backends teilt persönliche Verbindungen nicht automatisch.
- Verbindungen erst durch eine bewusste Aktion in einen Teambereich übernehmen.
- Gemeinsame Server können persönliche Datenbankzugänge je Benutzer oder ausdrücklich geteilte Zugangsdaten verwenden.
- Administratoren sollen nicht automatisch persönliche Tresore entschlüsseln können.
- Geteilte Geheimnisse benötigen eine eigene Schlüssel- und Berechtigungsverwaltung.
- Der Entzug einer Freigabe kann bereits bekannte Zugangsdaten nicht ungeschehen machen; dafür ist eine Rotation der Datenbankzugangsdaten erforderlich.
- Bei direkten Datenbankverbindungen setzt das Datenbankkonto die tatsächlichen Zugriffsrechte durch. Backend-Verbindungen erlauben zusätzliche zentrale Richtlinien.
- Hintergrundjobs nach Abmeldung benötigen ein bewusst definiertes Zugangs- und Freigabemodell.

### Architektur

- Vorgeschlagen ist ein modularer Go-Monolith mit Datenbankadaptern und versionierter API.
- Die Swift-App benötigt lokale Adapter, SQL-Ausführung, Metadatenabfragen und Datentransferfunktionen.
- Noch offen: getrennte Implementierungen in Swift und Go oder eine gemeinsam eingebettete Kernbibliothek.
- Eine getrennte Swift-/Go-Implementierung wurde als vorläufiger Ansatz vorgeschlagen; diese Entscheidung ist nicht verbindlich.
- Gemeinsame Modelle, Verhaltensspezifikationen, Austauschformate und Testfälle sollen Plattformunterschiede begrenzen.
- Native Treiberverfügbarkeit und Einschränkungen auf macOS, iPadOS und gegebenenfalls iOS müssen praktisch geprüft werden.
- Lokale und über Backend-Instanzen bereitgestellte Verbindungen sollen nach bisherigem Vorschlag gleichzeitig nutzbar sein.
- Die Datenbankadapter sollen Fähigkeiten ausdrücklich beschreiben, damit datenbankspezifische Funktionen zugänglich bleiben.

## Funktionsumfang

Das verbindliche Ziel ist vollständige phpMyAdmin-Funktionsparität. Die folgende Liste ist eine erste Gliederung, keine vollständige Paritätsmatrix:

- Datenbanken, Tabellen, Spalten, Indizes, Constraints und Views verwalten.
- Daten anzeigen, suchen, filtern, einfügen, bearbeiten und löschen.
- SQL-Editor, Stapelabfragen, mehrere Ergebnissätze, Verlauf und gespeicherte Abfragen.
- Benutzer, Rechte, Status, Prozesse und Wartung.
- Prozeduren, Funktionen, Trigger und Events.
- Import, Export und Dumps einschließlich relevanter Formate und Optionen.
- Beziehungsdesigner, Fremdschlüssel und visuelle Abfragen/Query-by-example.
- Transformationen, Änderungsverfolgung, Vorlagen, Einstellungen und Übersetzungen.

Für jede Funktion müssen Umfang, Verhalten und überprüfbare Erfüllung erfasst werden. MySQL-/MariaDB-spezifische Funktionen werden für andere Systeme durch passende native Funktionen ergänzt, soweit sinnvoll.

## Bisherige Produktideen — noch nicht verbindlich beschlossen

- Tabs mit wiederherstellbarem Zustand und globale Befehlssuche.
- Gute Tastaturbedienung sowie passende Touch-Bedienung auf dem iPad.
- Tabelleneditor mit direkter Zellenbearbeitung und Formularansicht.
- Mehrere Änderungen sammeln, prüfen und gemeinsam speichern; Atomarität abhängig von Datenbank und Operation.
- SQL-Vorschau und Vorher-/Nachher-Ansicht.
- Visuelle Filter mit sichtbarer SQL-Entsprechung.
- Fremdschlüsselnavigation und Spezialeditoren für JSON, Binärdaten, Datum und lange Texte.
- Erkennung konkurrierender Datenänderungen.
- Schema-Vergleich und überprüfbare Änderungsskripte.
- Hintergrundaufgaben mit Fortschritt, Protokollen und Abbruchmöglichkeiten, soweit unterstützt.
- Verständliche Ausführungspläne.
- Klare Kennzeichnung von Entwicklungs-, Test- und Produktionsverbindungen.
- Bedarfsgerechtes Metadatenladen, begrenzte Datenabfragen, virtualisierte Tabellen und Streaming großer Ergebnisse und Exporte.
- Einfache Bereitstellung des Go-Backends als ausführbare Datei mit eingebettetem Webfrontend sowie Container.

## Noch offene Entscheidungen

1. Datenbanksysteme und Priorisierung: Vorgeschlagen wurden MySQL/MariaDB, PostgreSQL und SQLite, später weitere relationale Systeme.
2. Referenzversion und vollständige Funktionsmatrix für phpMyAdmin-Parität.
3. Lokaler Datenbankkern: getrennte Implementierungen oder gemeinsame Bibliothek.
4. Mindestversionen der Apple-Betriebssysteme und tatsächliche Treiberunterstützung.
5. Passkey-/PRF-Kompatibilität sowie Einrichtung ohne eigene Go-Instanz.
6. Wiederherstellung, Geräteaufnahme und Schlüsselverfügbarkeit über mehrere Geräte.
7. iCloud-Technik, genauer Synchronisierungsumfang und Konfliktauflösung.
8. Rollen, Freigaben, Team-Schlüsselverwaltung und Rotation.
9. Funktionsumfang einer möglichen iPhone-Version.

## Empfohlene nächste Schritte

1. Anforderungen und phpMyAdmin-Funktionsmatrix vervollständigen.
2. Verbindungs-, Benutzer-, Arbeitsbereichs- und Tresormodell entwerfen.
3. Technische Prototypen für direkte Datenbankverbindungen auf Mac und iPad erstellen.
4. Passkey-Entsperrung, Masterpasswort-Fallback und Wiederherstellung prüfen.
5. Verschlüsselte iCloud-Synchronisierung einschließlich Geräteaufnahme prototypisch validieren.
6. Darauf basierend Architektur und Umsetzungsetappen festlegen.

Diese Übergabe dient als Projektkontext. Vorschläge und offene Entscheidungen dürfen nicht stillschweigend als bereits bestätigte Anforderungen behandelt werden.
