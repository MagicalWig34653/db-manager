# Einrichtung und Bootstrap

Noch kein Produktcode und kein App-Build. Vorhanden sind Dokumentation, Clientvorlagen
und Python-Skripte ohne Drittanbieterabhängigkeiten. Python 3 ist für die Prüfung erforderlich;
Git und GitHub CLI zusätzlich für Initialisierung und Veröffentlichung.

## Dokumentation prüfen

```sh
python3 scripts/check-docs.py
python3 bootstrap/repos/db-manager-web/scripts/check-docs.py
python3 bootstrap/repos/db-manager-apple/scripts/check-docs.py
```

## Repo-Gerüste installieren

`<workspace>` bezeichnet das Verzeichnis mit GoLand, PhpstormProjects und Xcode.
Zuerst nur den Plan anzeigen:

```sh
python3 scripts/bootstrap-repos.py --workspace '<workspace>'
```

Mit `--apply` werden Clientvorlagen installiert und drei Git-Repos initialisiert.
Vorhandene abweichende oder unbekannte Clientdateien werden nicht übernommen. Bestehende
Repos mit Commits und Änderungen werden abgelehnt. Frisch initialisierte Repos ohne Commit
dürfen anschließend mit `--commit` fortgesetzt werden; unbekannte bereits gestagte Dateien
werden abgelehnt. Main-Dateien werden explizit über `bootstrap/manifest.json` ausgewählt,
Clientdateien über die Vorlagen; weitere lokale Dateien werden nicht gestagt. Es werden keine Zugangsdaten angelegt.

```sh
python3 scripts/bootstrap-repos.py --workspace '<workspace>' --apply
```

Vor einem öffentlichen Commit die bestehende Git-Autoridentität und alle Dateien prüfen.
`--commit` übernimmt die bestehende Identität; das Skript ändert keine Git-Konfiguration.
`--publish` legt öffentliche GitHub-Repos an und pusht. Default-Owner: MagicalWig34653.
Die Anmeldung muss zum Owner passen. Existierende Remote-Repos erfordern manuelle Prüfung.

```sh
gh auth status
python3 scripts/bootstrap-repos.py --workspace '<workspace>' --apply --commit --publish
```

Der Bootstrap ist kein Rollback-System: Bei einem Fehler bleiben erfolgreiche Teilschritte
bestehen. Vor erneutem Lauf tatsächliche Dateien, Git-Status und Remote-Repos prüfen.
Nach Installation Clientvorlagen nicht weiterentwickeln: Produktarbeit im jeweiligen Zielrepo.
Status und Arbeitsplan danach mit echten Commit-/Remote-/Prüfergebnissen aktualisieren.

## Verifizierter Installationsstand

Am 20.10.2026 wurde `--apply` im tatsächlichen Workspace erfolgreich ausgeführt:
drei Git-Repos auf `main`, 38 installierte Clientdateien und drei bestandene lokale
Dokumentationsprüfungen. GitHub-Anmeldung als `MagicalWig34653` bestätigt.
Die frühere Schreib-/DNS-Blockade wurde dabei nicht mehr reproduziert.
Aktueller Commit-, Remote- und CI-Stand: [Status](docs/STATUS.md).

Nach separater Weiterentwicklung installierter Clients kann das Bootstrap-Skript
abweichende Zieldateien absichtlich ablehnen. Dann Änderungen im jeweiligen Git-Repo
prüfen und explizit committen; Vorlagen nicht über die weiterentwickelten Clients kopieren.
