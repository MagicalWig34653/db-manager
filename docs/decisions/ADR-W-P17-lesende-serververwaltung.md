<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# ADR-W-P17: Lesende Serververwaltung

Datum: 26.10.2026. Status: angenommen am 27.10.2026 für die **lesende** Stufe. Beleg: [Review B](../handoffs/WAVE4-REVIEW-B.md) (Freigabe) und
[Re-Review](../handoffs/WAVE4-REREVIEW.md) (Befund F3 behoben, keine Secrets geöffnet). Integriert als API 0.17. Die ursprüngliche
W-P17-Gesamtplanung (schreibende Eingriffe) wird dadurch nicht als umgesetzt ausgewiesen.

## Entscheidung

API 0.17 baut additiv auf v12 auf und bietet ausschließlich Status, Variablenlesen
und Prozessliste. SET/KILL samt Vorschau, Ausführung, Bestätigungstypen und
Identitätstoken werden aus dem WIP entfernt. Stufe 2 benötigt einen neuen Auftrag
und eigenen Vertrag. Weder FLUSH noch Monitor-/Replikationsfunktionen gehören dazu.

Prozessabfragen wählen keine SQL-Texte oder Hosts aus. Variablenwerte folgen einer
konservativen Positivliste numerischer Werte; sensible und unbekannte Werte bleiben
maskiert. SQLite bleibt bei der Version ohne Pfade, gemäß
[ADR-0004](ADR-0004-sqlite-adapter.md). Zeit-, Zeilen- und Bytegrenzen begrenzen die
Beobachtung; Sichtbarkeit wird vom Server bestimmt. Kein atomarer Snapshot.

## Folgen

Die lesende Stufe benötigt keine Schreibvorschau und keine Prozess-Fingerprints.
Die Oberfläche muss eingeschränkte Fähigkeiten und maskierte Werte ehrlich
anzeigen. Vertrag und Prüfstand stehen in [SERVER-ADMIN-API](../../contracts/SERVER-ADMIN-API.md)
und im [Handoff](../handoffs/W-P17-backend.md). Liveabnahme der Netzwerkengines
und unabhängiger Review bleiben getrennte Gates.
