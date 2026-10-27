# Entwicklungsablauf

Maßgeblicher Einstieg: [AGENTS.md](../AGENTS.md).

## Quellen

Anforderungen stehen im Konzept, Gründe in ADRs, tatsächliche Ergebnisse in STATUS,
aktive Aufträge in ARBEITSPLAN, Prüf- und Startbefehle in EINRICHTUNG.
Gemeinsame Verträge gehören dem Hauptrepo; Clientrepos referenzieren einen festen Stand.
Details verlinken, nicht als konkurrierende Projektwahrheiten duplizieren.

## Auftrag und Übergabe

Jedes Paket nennt ID, Ziel, Nichtziele, Repo, Branch/Worktree, erlaubte Dateien,
Vertragsstand, Abhängigkeiten, Abnahme, Implementierer und unabhängigen Reviewer.
Ein Worker schreibt nur im eigenen Worktree. Lead integriert, prüft das Gesamtergebnis
und aktualisiert die zentralen Dokumente. Keine Plugin- oder persönliche Memory-Pflicht.

Übergaben enthalten Ergebnis, Dateien/Commit, Prüfdatum und exakte Befehle mit Ausgang,
Einschränkungen, Reviewbefunde, Daten-/Migrationsfolgen, laufende Jobs und nächsten Schritt.
Vor Sessionende regelmäßige Checkpoints und nachvollziehbare Commits sichern.
Alte Einträge „in Arbeit“ anhand von Dateien, Branches und Jobs verifizieren.

## Abnahme

Abnahmekriterien erfüllt, angemessene Prüfungen ausgeführt, relevante Befunde gelöst,
Lizenzen und öffentliche Daten geprüft, Dokumentation und Status aktualisiert.
Nicht ausgeführte Plattformtests bleiben offen. Eine frische Session muss fortsetzen können.

## Dokumentationsregeln für die Site

`python3 scripts/check-docs.py` muss vor jedem Merge fehlerfrei laufen. Es spiegelt den Pages-Check:
In `docs/**/*.md` keine privaten Pfade oder Build-Marker wörtlich schreiben (Liste in `site/check.py`
und `scripts/check-docs.py`: macOS-/Runner-Home-Pfade, temporäre Ordner, Capture-Konfigurationsvariable),
sondern umschreiben (z. B. „Home-Pfade“). Anker-Links nur ASCII und auf vorhandene Überschriften: Der Site-Slug
entfernt Umlaute ohne Ersatz („Parität“ wird `paritat`, nicht `paritaet`).
