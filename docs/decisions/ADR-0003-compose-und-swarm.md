# ADR-0003: Docker Compose und Swarm

Status: akzeptiert durch Nutzeranweisung vom 20.10.2026.

Kontext: Der Nutzer verlangt beide Docker-Betriebsarten für das fertige Produkt.
Entscheidung: Go-Backend und Weboberfläche erhalten ein reproduzierbar gebautes Image,
eine Compose-Konfiguration und eine Swarm-Stack-Konfiguration. Persistenz, Secretdateien,
Healthchecks und geordneter Shutdown werden in beiden Betriebsarten berücksichtigt.
Compose ersetzt keinen praktischen Swarm-Nachweis. Tests verwenden isolierte Ressourcen.

Die Apple-App bleibt ohne Go oder Docker nutzbar. Keine Veröffentlichung von Images,
Cloudressourcen oder echten Zugangsdaten ist für lokale Integrationsproben nötig.
Details wie Replikationsmodell, Persistenzbackend und Imageversionierung folgen
als überprüfbare Implementierungsentscheidungen.
