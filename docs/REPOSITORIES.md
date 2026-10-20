# Repositories

| Name | Inhalt | Lokaler Pfad relativ zum Projektworkspace |
| --- | --- | --- |
| db-manager | Backend, gemeinsame Dokumentation und Verträge | GoLand/db-manager |
| db-manager-web | Weboberfläche | PhpstormProjects/db-manager-web |
| db-manager-apple | Eigenständige Apple-App | Xcode/db-manager-apple |

GitHub-Owner: `MagicalWig34653`, mit `gh api user` am 20.10.2026 bestätigt.
Alle drei lokalen Repos sind auf `main` initialisiert; Clientgerüste installiert.
Alle drei Repos sind öffentlich unter `https://github.com/MagicalWig34653/<name>`
verfügbar und besitzen `origin` mit Tracking von `main`. Anfangscommits und erfolgreiche
Push-CI-Läufe sind im [Status](STATUS.md) nachgewiesen. Frische unabhängige Checkouts
bestehen die Dokumentationsprüfung; noch keine Produktbuilds.
Alle Repos sollen öffentlich sein. Kein automatisches Überschreiben vorhandener Remote-Repos.

Gemeinsame Verträge im Hauptrepo; Clients pinnen Version/Commit und Prüfsumme.
Derzeit kein freigegebener Vertrag. Clientvorlagen unter `bootstrap/repos/` sind ausschließlich
Installationsvorlagen; nach Installation wird Produktcode im jeweiligen Zielrepo gepflegt.
Sie sind kein Ersatz für eigenständige Repositories und werden nicht parallel weiterentwickelt.
