<!-- SPDX-License-Identifier: AGPL-3.0-only -->
# Synthetische Wartungsfixtures

Alle JSON-Dateien hier sind **handgeschriebene Platzhalter**, keine Antworten aus
realen Engines. Namen und Daten sind erfunden. Insbesondere `aaaa…` als Planhash
ist absichtlich kein echter Digest der Beispielanweisung. Echte Enginefixtures
kommen nach dem separat auszuführenden Live-Gate; nichts davon ist im Web-Lock
gepinnt.

Dateinamen: `<Schema>.<Fall>.json`; `*.invalid.json` muss der Wächter ablehnen.
Positive Fälle decken die drei Treiber, Vorschau, einzelne Ergebnisse, Teilerfolg,
Prüfsumme als Zeichenfolge und sichere Fehler ab. Negative Fälle prüfen fehlende
Pflichtfelder, unbekannte Felder, Nullwerte, Grenzen, Hashformat und widersprüchliche
Planstatus. Prüfung: `python3 scripts/check-maintenance-contract.py`.

`scripts/test-maintenance.py` erzeugt echte Antworten nur temporär und validiert sie
gegen denselben Vertrag. Diese Läufe ersetzen weder die Platzhalter stillschweigend
noch die unabhängige Reviewfreigabe.
