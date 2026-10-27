# ADR W-P11: Begrenzter Exportstream mit Formatabschluss

Status: angenommen am 27.10.2026. Beleg: unabhängiges [Backend-Review](../handoffs/W-P11-backend-review.md) mit Nachprüfung (Freigabe, keine Blocker),
integriert als API 0.11 ([Welle 2](../handoffs/WAVE2-INTEGRATION.md)). Die ADR behält ihren paketbezogenen Namen. Datum: 24.10.2026.

## Kontext

Der lokale F6-Export umfasst nur bereits geladene Ergebnisse. W-P11 benötigt neue
Datenbankleseläufe und darf deren Daten nicht vollständig puffern. Nach dem ersten
HTTP-Ausgabebyte kann der Server Fehler nicht mehr in eine JSON-Fehlerantwort umwandeln.
Browser stellen HTTP-Trailer nicht überall bereit. Ein 200 oder ein syntaktisch gültiger
CSV-Teil wäre deshalb ohne zusätzlichen Rahmen ein falscher Vollständigkeitsnachweis.
E4 verlangt Browserdownload ohne Serverdateiziele oder externe Dump-Programme.

## Vorgeschlagene Entscheidung

Die Engine schreibt zeilenweise in einen begrenzten Writer, hält nur beschränkte
Metadaten, die aktuelle Zeile und optional einen begrenzten INSERT-Stapel. Eine feste
Verbindung und Read-only-Transaktion je DB verhindern wechselnde Lesesitzungen.
Gesamtzahl, unkomprimierte Bytes, Einzelzelle, Zeile, INSERT-Länge und Dauer sind begrenzt.
Clientabbruch beendet Kontext und Datenbankleselauf; Rollback und Verbindungsschließen
laufen auch nach Fehlern. Keine temporäre Exportdatei und kein automatischer Retry.

Jedes neue Streamformat hat einen positiven Abschluss: JSON `complete:true`, SQL eine
separate Endmarke, CSV eine Kontrollspalte und einen Abschlusssatz. Trailer ergänzen dies,
ersetzen es aber nicht. Fehlender Abschluss macht ein gespeichertes Fragment erkennbar.
GZip muss zusätzlich vollständig entpackbar sein. Bestehende lokale F6-Profile bleiben
unverändert und werden im gemeinsamen Formatvertrag getrennt beschrieben.

## Folgen und Grenzen

CSV benötigt vor regulärem Datenimport Entfernung der Kontrollspalte und Endzeile. Das
ist dem Risiko unerkannter Teildownloads vorzuziehen. Ein Streamingclient muss vor einem
SQL-Restore Vollständigkeit prüfen; der Server kann bereits ausgegebene Bytes nicht
zurückholen. Bei MySQL-DDL ist BEGIN kein verlässlicher Rollbackrahmen.

Es gibt keinen atomaren Snapshot über mehrere Datenbanken. SQLite bleibt vollständig
an ADR-0004 gebunden, MySQL auf InnoDB begrenzt. Katalogbasierte PostgreSQL-DDL ist keine
vollständige Backupdarstellung. UTF-8 und derselbe SQL-Dialekt sind die ehrliche erste
Teilmenge; weitere Formate und Kodierungen benötigen eigene geprüfte Erweiterungen.

Verträge und genaue Grenzen: [EXPORT-API](../../contracts/EXPORT-API.md),
[EXPORT-FORMAT](../../contracts/EXPORT-FORMAT.md). Prüfstand und offene Gates:
[Handoff](../handoffs/W-P11-backend.md).
