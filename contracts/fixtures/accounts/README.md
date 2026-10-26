# W-P18-Vertragsfixtures

Alle JSON-Dateien in diesem Verzeichnis sind **synthetische Platzhalter**, keine
aufgezeichneten Live-Antworten. `FIXTURE_*_PLACEHOLDER` bezeichnet frei erfundene
Konten, Ziele und Geheimnisse; Nullhashes sind nur syntaktisch gültige Platzhalter.
Sie dürfen nicht für eine echte Ausführung verwendet werden. `.invalid.json`
kennzeichnet absichtlich schemawidrige Werte. Der Wächter prüft beide Klassen.

Echte temporäre Antworten erzeugt [test-accounts.py](../../../scripts/test-accounts.py)
mit eigenen dbfix-Datenbanken; diese werden gegen dieselben Schemata geprüft und
anschließend gelöscht. Der tatsächliche Laufstand steht im
[Backend-Handoff](../../../docs/handoffs/W-P18-backend.md).

Ergänzung Integration (Welle 3): Dateien `*.real-*.json` sind echte Antworten aus `scripts/test-accounts.py <dir>` gegen MySQL 8.4, MariaDB 12.3 und
PostgreSQL 18.6 (TLS verify-full, Wegwerf-Container nacheinander), 26.10.2026. Das Skript prüft, dass das Testpasswort in keiner Antwort vorkommt.
