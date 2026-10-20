# Konzept

Bestätigte Ausgangsanforderungen: [Projektübergabe](../PROJEKTUEBERGABE.md).
Aktuelle Ergänzungen und Umsetzungsvorschlag: [Projektplan](PROJEKTPLAN.md).
Die Anweisung „Dann los“ autorisiert den beschriebenen Bootstrap mit drei Repos.
Technische Optionen aus der Übergabe bleiben unvalidiert, bis ein ADR sie festlegt.

Web verwendet Go; Apple arbeitet eigenständig mit direkten DB-Verbindungen und optionalem
Backend. Persönliche iCloud-Synchronisierung, optionale verschlüsselte Zugangsdaten,
Passkeys/Masterpasswort sowie Teams über Go gehören zum Zielumfang.
Vollständige phpMyAdmin-Parität ist das Endziel, keine Eigenschaft des ersten Zwischenstands.

## Ergänzung vom 20.10.2026

Verbindlich durch Nutzeranweisung: Das Go-/Web-Produkt muss am Ende sowohl mit
Docker Compose als auch als Docker-Swarm-Stack betreibbar sein. Beide Wege benötigen
versionierte Konfiguration, persistente Daten, Secretübergabe, Healthchecks und
nachvollziehbare Prüfungen. Die eigenständige Apple-App bleibt davon unabhängig.

Die autonome Umsetzung mit Agents ist bis zum vollständigen Projektabschluss oder
21.10.2026 um 04:00 Uhr Europe/Berlin autorisiert. Das entspricht 02:00 UTC.
Bei Erreichen der Zeitgrenze laufende eigene Jobs geordnet beenden, tatsächlichen
Stand, offene Prüfungen, Branches und nächsten Schritt dokumentieren.
