# ADR W-P15: Nutzer-SQL und ehrliche Simulation

Datum: 24.10.2026. Status: Implementierungsvorschlag für W-P15, Leadfreigabe und
Integration offen. Keine zentrale ADR-Nummer vorweggenommen.

## Kontext

[W-P15](../PWA-PLAN.md) verlangt SQL-Stapel, EXPLAIN und DML-Vorschau.
Freies Nutzer-SQL besitzt die tatsächlichen Verbindungsrechte. Transaktionale
Tabellen allein beweisen keine Rücknahme von Sequenzen, Triggern oder externen Effekten.
Der bestehende Einzelstatementvertrag darf sich nicht ändern. W-P12s gemeinsamer
Splitter ist auf dieser Basis noch nicht vorhanden.

## Vorschlag und Implementierung

Neue additive vorläufige API0.16 mit eigener ausdrücklicher Schreibzustimmung für
jede Ausführung. Stapel sind begrenzte Einzelaufrufe auf derselben Verbindung;
Ergebnisfehler und Abbruch bleiben konservativ unsicher. Eigenständiger getesteter
Splitter unter `internal/sqlsplit`, bei Integration mit W-P12 abzugleichen.

EXPLAIN ohne implizites ANALYZE. Simulation führt ausschließlich begrenzte einfache
SQLite-DML in der unveränderten Sandbox aus und rollt explizit zurück. Die anderen
Adapter melden die allgemeine freie Simulation als nicht unterstützt. Keine neue
Bibliothek und keine Lockerung von VFS/Authorizer.

## Folgen und Alternativen

[Vertrag](../../contracts/SQL-BATCH-API.md) dokumentiert Zählerlücken bei
RETURNING/WITH/CALL, begrenzte Warncodes und fehlende PostgreSQL-Hinweise.
Metadatengeprüfte Netzwerk-Simulation wäre umfangreicher und bleibt ein separates
Paket; ein pauschaler Rollbackversuch würde eine falsche Sicherheitsgarantie vermitteln.
Der Splitter prüft Lexik, keine vollständige Grammatik; ein vollwertiger Linter und die
Webkonsole sind durch dieses Backendpaket nicht umgesetzt.
