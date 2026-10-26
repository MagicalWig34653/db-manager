<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Fixtures für die lesende API 0.17

Dateien mit `placeholder` sowie `.invalid.json` sind **synthetische Platzhalter**,
keine Liveantworten und kein Prüfbeleg eines Datenbankservers. Kennungen, Benutzer
und Zeitpunkte sind erfunden. `.invalid.json` muss der Vertragswächter abweisen.
Die Fixtures enthalten keine Zugangsdaten.

`ServerAdminReadResult.real-sqlite.json` stammt aus `TestSQLiteServerAdmin` über
den echten SQLite-Adapter dieses Worktrees (26.10.2026), ohne Datenbankpfad.
MySQL-/MariaDB-/PostgreSQL-Antworten sind bisher ausschließlich Platzhalter.
`scripts/test-server-admin.py` prüft zukünftige echte Antworten in temporären
Dumps gegen API 0.17, jeweils mit einem eigenen kurzlebigen `dbfix.py`-Container.
