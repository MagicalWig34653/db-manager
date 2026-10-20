# Architektur – Entwurf

Der [Projektplan](PROJEKTPLAN.md) beschreibt die vorgeschlagenen Modulgrenzen.
Go als modularer Monolith, eigenständiger Swift-Kern und versionierte gemeinsame
Verhaltensverträge sind die Arbeitshypothese. Keine Treiber oder Frameworks ausgewählt.

Offen: Datenbank-/OS-Zielversionen, native Treiber, Kernbibliothek, Tresor und Recovery,
Sync-Konflikte, Teamrechte und Netzwerkzugriffsrichtlinie. Vor Festlegung P2/P3 auswerten.
Apple darf zur Laufzeit keinen Go-Server benötigen. Lokale und Backend-Ausführung müssen
Herkunft, Fähigkeiten, Berechtigungen und Umgang mit Geheimnissen ausdrücklich unterscheiden.
