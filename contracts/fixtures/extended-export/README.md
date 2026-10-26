# W-P28-Vertragsfixtures

Sämtliche `placeholder`-Kennungen sind markierte synthetische Platzhalter, keine
Verbindungsdaten oder Nachweise einer Live-Datenbank. Die JSON-Dateien prüfen gültige
Textformatanfragen und ungültige Optionen gegen API 0.21. Tatsächliche SQLite-Ausgaben
werden durch `scripts/test-extended-export.py` in einem temporären Verzeichnis erzeugt
und unabhängig als XML beziehungsweise strikte JSON-Teilmenge von YAML 1.2 gelesen.
PHP-/LaTeX-/Wiki-Escaping wird gegen feste Referenzbytes geprüft. Die temporäre echte
Fähigkeitsantwort wird gegen das Vertragsschema validiert. Livefixtures werden nur
mit `--live` über `scripts/dbfix.py` erzeugt, seriell und ausschließlich zum Wegwerfen.

Eigener Code: SPDX-License-Identifier: AGPL-3.0-only.
