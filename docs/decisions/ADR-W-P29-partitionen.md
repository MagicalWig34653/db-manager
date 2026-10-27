# ADR W-P29: Strukturierte Partitionen und gemeinsame Präfixpläne

Datum: 25.10.2026. Status: angenommen am 27.10.2026. Beleg: [Review B](../handoffs/WAVE4-REVIEW-B.md) (Freigabe, nur Testlücken) und
[Re-Review](../handoffs/WAVE4-REREVIEW.md). Integriert als API 0.22. Kein MySQL/MariaDB-Echtlauf der Weboberfläche.

## Kontext und Entscheidung im Paket

[W-P29](../PWA-PLAN.md) ergänzt Partitionierung und Präfixstapel über den
[W-P07-Objektoperationen](../../contracts/OBJECT-OPS-API.md).
Neue additive [API 0.22](../../contracts/PARTITIONS-API.md) kumulativ auf v16. Die
Präfixplanung ruft die vorhandene Objektplanung auf; ein Gesamt-Hash bindet die
exakte vollständige Anweisungsliste. Kein zweiter SQL-Generator für Objektaktionen.

Partitionierungsdefinitionen akzeptieren eine Katalogspalte und strukturierte
Grenzen als Daten. Freies Ausdrucks-SQL würde den strukturierten Schreibvertrag
unterlaufen und wird nicht eingeführt. Vorhandene Ausdrücke sind lesbar.
PostgreSQL erhält deklaratives CREATE PARTITION OF und ATTACH/DETACH sowie direkte
Kindoperationen. Ein automatischer Umbau einer bestehenden Tabelle zu einer
partitionierten Tabelle erfordert eine eigene Daten-/Abhängigkeitsmigration und
wird ausdrücklich nicht simuliert. SQLite meldet Partitionen als nicht unterstützt.

## Folgen und Grenzen

Keine neue Bibliothek, keine SQLite-Sandboxänderung. PostgreSQL-/SQLite-Stapel
laufen transaktional, MySQL/MariaDB bleiben bei Teilwirkungen unsicher. Der
Ausführungszähler ist kein Nachzustandsbeweis; `observation` bleibt `null`.
Wartungs-Fehlerzeilen werden neben SQL-Protokollfehlern ausgewertet.
Mehrspaltige Definitionen, freie Ausdrücke und MySQL-Subpartitionen sind begrenzt;
keine vollständige Paritätsbehauptung. Unabhängiges Review und Netzwerk-Livetests
bleiben Voraussetzung für Integration, siehe [Handoff](../handoffs/W-P29-backend.md).
