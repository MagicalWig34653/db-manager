# W-P20-Vertragsbeispiele (API 0.24, Konfigurationsspeicher)

Dateien `*.real-<name>.json` sind **unveränderte Antworten** des Livelaufs `scripts/test-config-store.py`
(Handler `httpapi` gegen den echten Serverzustand auf MariaDB 12.3 mit TLS; `real-local-only` ohne konfigurierten
Zustandsspeicher; `real-mariadb-unreachable` und `Error.real-state-unavailable` während eines simulierten Ausfalls
über einen Proxy). Lauf vom 27.10.2026. Alle Zeitstempel, Kennungen und Hashes stammen aus diesem Lauf.

Alle übrigen Dateien ohne `.invalid` sind **gekennzeichnete Platzhalter** (`*placeholder*`): von Hand geschriebene,
schemagültige Formbeispiele, keine beobachteten Antworten; die Nullhashes sind keine ausführbaren Bestätigungen.
`*.invalid.json` sind absichtlich ungültige Schemafälle, die der Wächter ablehnen muss.

Neu erzeugen: `python3 scripts/test-config-store.py --keep <Verzeichnis>`; der Wächter
`python3 scripts/check-config-store-contract.py` prüft alle Dateien gegen `contracts/api-v0.24.0.json`.
