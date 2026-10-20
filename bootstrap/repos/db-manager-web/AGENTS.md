# Einstieg für Coding-Agents

Das Repository ist das Projektgedächtnis. Sessions sind wegwerfbar.

1. `docs/STATUS.md` und `docs/ARBEITSPLAN.md` lesen.
2. `docs/ENTWICKLUNGSABLAUF.md`, relevante Anforderungen und ADRs lesen.
3. Tatsächliche Dateien, Git-Status und Prüfstand verifizieren; Planung ist keine Implementierung.
4. Ein begrenztes Paket mit Dateibesitz und Abnahmekriterien übernehmen.

## Verbindliche Regeln

- Eigener Projektcode: AGPL-3.0-only. Abhängigkeiten behalten ihre Lizenzen und werden inventarisiert.
- Ein Lead koordiniert Agents; jeder schreibende Worker erhält einen eigenen Worktree und Branch.
- Kein paralleles Schreiben in denselben Dateien; gemeinsame Verträge vorab abstimmen.
- Unabhängiges Review vor Integration. Keine festen Anbieter oder Modellquoten.
- Lead pflegt zentrale Status-/Planungsdateien; Worker liefern `docs/handoffs/<paket>.md`.
- Dokumentation und Oberfläche Deutsch; Code, Kommentare, Tests und technische Bezeichner Englisch.
- Anforderungen, Vorschläge, akzeptierte Entscheidungen und tatsächlichen Prüfstand trennen.
- Wesentliche Entscheidungen als ADR festhalten; Dokumentation mit jeder Teilaufgabe aktualisieren.
- Keine Secrets, privaten Pfade, echten Verbindungsdaten, produktiven SQL-Verläufe oder vollständigen Chats veröffentlichen.
- Bestehende Git-Autoridentität nicht eigenmächtig ändern; öffentliche Commit-Metadaten prüfen.
- Passende Prüfungen ausführen; nicht ausgeführte Prüfungen ausdrücklich offenlassen.
- Keine fremden Änderungen verwerfen. Status laufender Jobs vor erneuter Ausführung prüfen.

Vor Kontextwechsel: Dateien, Branch/Commit, Tests, Probleme, unintegrierte Arbeit, laufende Jobs
und nächsten konkreten Schritt dokumentieren. Kein Wissen darf nur im Chat liegen.
Claude-Code-Einstieg: `CLAUDE.md`. Einrichtung: `EINRICHTUNG.md`.
