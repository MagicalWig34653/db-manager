# ADR W-P12: Begrenzte Browserimporte und bestätigte CSV-Pläne

Datum: 25.10.2026. Status: Implementierungsvorschlag, unabhängiger Sicherheitsreview
und Leadfreigabe offen. API 0.18 ist ausdrücklich für dieses Paket vergeben.

## Kontext

[W-P12](../PWA-PLAN.md) verlangt Browserupload bis 64 MiB, SQL-Stapel und CSV-Vorschau.
W-P15 liefert bereits den Lexer. Serververzeichnisse, externe Importprozesse und
LOAD DATA LOCAL INFILE sind für diese Stufe ausgeschlossen. CSV erzeugt keine Tabelle.

## Entscheidungsvorschlag und Implementierung

Multipart mit getrenntem JSON-Metadatenteil und Binärdatei vermeidet Base64-Aufschlag.
Beide Teile werden begrenzt im Speicher gelesen; weder Multipart-Spooling noch
Archivextraktion oder Serverdateipfade. ClassUpload erhält 64 MiB Datei plus 1 MiB
Rahmen; Standardrouten behalten 1 MiB. GZip wird vor jedem DB-Schreiben vollständig
mit Entpackgrenze und Prüfsumme gelesen. ZIP/BZip2 sind nicht freigegeben.

Höchstens zwei Upload-/Plan-/Ausführungsvorgänge je API-Instanz teilen sich Slots.
Ausführungen werden nach vollständigem Upload als flüchtige Jobs mit kryptografischer
ID gestartet. Bearer-geschützte Status-/Abbruchrouten, maximal 16 gespeicherte Jobs,
fünf Minuten Ergebnisaufbewahrung und maximal fünf Minuten Ausführung begrenzen
Lebensdauer und Speicher. Aktive Jobs halten Datei und Credentials; abgeschlossene
Jobs nur Status und begrenzte Ergebnisse. Kein automatischer Retry, keine Wiederaufnahme,
keine persistente Jobwarteschlange. Ein Prozessende kann unklare Ergebnisse hinterlassen.

CSV prüft alle Zeilen und liefert maximal 20 Vorschauzeilen sowie 100 Fehlerpositionen.
`dbplan` bindet die parametrisierte Anweisung plus einen Bindungsdigest an Datei,
normalisierte Optionen, Verbindungsidentität ohne Passwort und beobachtete Metadaten.
HTTP prüft vor 202; die Engine prüft erneut unter Transaktion und Relations-/Metadatensperre.
Savepoints je Zeile ermöglichen Überspringen auch nach PostgreSQL-Constraintfehlern.
SQLite verwendet INSERT OR ABORT gegen bestehende REPLACE-Konfliktregeln. Ganze Datei
in einer Transaktion, begrenzte Einzelzeilen-Inserts; keine verlustbehaftete Multirow-
Konfliktauflösung. Nichttransaktionale Zieltabellen und unbekannte Typen werden abgewiesen.

SQL erhält denselben Lexer mit expliziten Importgrenzen; die bisherigen Split-Grenzen
bleiben unverändert. Datei vor dem ersten Schreiben trennen/prüfen. Einzelanweisungen
laufen auf einer gepinnten Verbindung. Einfache ausgeglichene BEGIN/COMMIT-Rahmen und
feste W-P11-Sitzungsheader sind erlaubt. Keine pauschale Transaktionszusage für DDL,
MySQL/MariaDB oder externe Effekte. Bei Anweisungsfehlern bleibt der Ausgang konservativ
unbekannt, auch wenn bestimmte Fehler nachweislich keine direkten Änderungen machen.

## Folgen und Alternativen

[Vertrag](../../contracts/IMPORT-API.md) beschreibt Typteilmenge, Ergebnisgrenzen,
Schlussmarken und Einschränkungen. Rohes SQL läuft mit tatsächlichen Verbindungsrechten;
der Lexer ist kein SQL-Sicherheitsparser. Die SQLite-VFS-/Authorizergrenze bleibt bestehen.

Streaming würde weniger Speicher brauchen, könnte aber erst spät erkannte Dateifehler
nach bereits wirksamen SQL-Anweisungen entdecken. Festplatten-Spooling widerspricht der
gewählten Dateisystemgrenze. Der begrenzte Speicheransatz ist bewusst gewählt; maximale
64-MiB-Parallelbelastung und Mehrreplikabetrieb sind eigenständige Abnahmegates. Status
muss bis Jobabschluss dieselbe Instanz erreichen. Kein universelles Backup-/Rollbackversprechen.
