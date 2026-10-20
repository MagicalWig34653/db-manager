# Arbeitsplan

Stand: 20.10.2026. Lead: aktuelle koordinierende Session; kein dauerhafter Sessionbezug nötig.

| ID | Auftrag | Status | Abhängigkeiten / Abnahme |
| --- | --- | --- | --- |
| P0 | Drei Repo-Gerüste, Lizenz, Prozess, Prüfung und Installation | abgeschlossen: installiert, veröffentlicht, Push-CI grün | Drei öffentliche Repos auf main; unabhängiger Review bestanden; Remote-/CI-Nachweise in STATUS |
| P1 | phpMyAdmin-Referenz und vollständige Paritätsmatrix | offen | P0, Recherche mit Quellen und überprüfbaren Kriterien |
| P2 | Mac-/iPad-/Go-Treiberuntersuchung | offen | erste DB-/OS-Ziele festlegen; echte Plattformbefunde |
| P3 | Tresor, Recovery und verschlüsselter Sync | offen | getrennte Machbarkeitsbefunde und dokumentiertes Bedrohungsmodell |

Lead pflegt zentrale Dokumente. Unabhängiger Gerüst- und Veröffentlichungsreview
abgeschlossen; bestätigte Befunde geschlossen, Ergebnis unter
[Handoff P0](handoffs/P0-review.md). Keine schreibenden Worker, Produktdienste oder Migrationen.

## Nächste begrenzte Pakete

Noch nicht begonnen; keine Produktimplementierung daraus ableiten.
Für schreibende Worker legt der Lead jeweils eigenen Branch und Worktree an.
Ein anderer Agent prüft den Diff vor Integration; zentrale Statusdateien bleiben beim Lead.

| Paket | Ziel und erlaubte Dateien | Abnahme und Abhängigkeiten |
| --- | --- | --- |
| P1a | Im Hauptrepo auf Branch `docs/p1a-paritaet`: `docs/PARITAET.md` und `docs/handoffs/P1a.md`; phpMyAdmin-Referenz mit Primärquellen und Datum festlegen, Funktionsinventar erheben | Referenzversion nachvollziehbar; jede Funktion mit Sollverhalten, DB-/Plattformbezug und prüfbarem Abnahmekriterium; Vollständigkeitsgrenzen benannt; keine Implementierung oder stillschweigende Priorisierung |
| P2a | Im Hauptrepo auf Branch `docs/p2a-treiberplan`: `docs/handoffs/P2a.md`; getrennten Go-/Apple-Prüfplan und offene DB-/OS-Ziele erfassen | Vor praktischer Ausführung erste DB-/OS-Ziele klären; TLS, Auth, Typen, Abbruch, Streaming, SSH, Lizenz und echtes iPad abdecken; keine Treiberauswahl ohne Befunde |
| P3a | Im Hauptrepo auf Branch `docs/p3a-bedrohungsmodell`: `docs/handoffs/P3a.md`; Vertrauensgrenzen und Untersuchungsfälle für Tresor, Recovery und Sync beschreiben | Anmeldung und Entsperren getrennt; Geräteaufnahme, Verlust, Offlinebetrieb, Konflikte und Rotation berücksichtigt; Passkey-/PRF-Annahmen ausdrücklich offen; keine echten Zugangsdaten oder Kryptografieimplementierung |

Gemeinsamer Vertragsstand: keiner freigegeben. P1a ist der nächste konkrete Auftrag;
P2a/P3a können danach getrennt geplant werden. Resultate sind zunächst Vorschläge,
Architekturentscheidungen folgen nach Review als ADR. Keine laufenden Hintergrundjobs.
