# Repositories

| Name | Inhalt | Lokaler Pfad relativ zum Projektworkspace |
| --- | --- | --- |
| db-manager | Backend, gemeinsame Dokumentation und Verträge | GoLand/db-manager |
| db-manager-web | Weboberfläche | PhpstormProjects/db-manager-web |
| db-manager-apple | Eigenständige Apple-App | Xcode/db-manager-apple |

GitHub-Owner: `MagicalWig34653`, mit `gh api user` am 20.10.2026 bestätigt.
Alle drei lokalen Repos sind auf `main` initialisiert; Clientgerüste installiert.
Noch keine Commits oder Remotes; Veröffentlichung und CI-Prüfung folgen nach Review.
Alle Repos sollen öffentlich sein. Kein automatisches Überschreiben vorhandener Remote-Repos.

Gemeinsame Verträge im Hauptrepo; Clients pinnen Version/Commit und Prüfsumme.
Derzeit kein freigegebener Vertrag. Clientvorlagen unter `bootstrap/repos/` sind ausschließlich
Installationsvorlagen; nach Installation wird Produktcode im jeweiligen Zielrepo gepflegt.
Sie sind kein Ersatz für eigenständige Repositories und werden nicht parallel weiterentwickelt.
