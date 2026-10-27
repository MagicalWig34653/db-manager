# ADR W-P19: Bestätigte Programmdefinitionen ohne Client-Splitting

Datum: 25.10.2026. Status: angenommen am 27.10.2026. Beleg: unabhängiges [Backend-Review](../handoffs/W-P19-backend-review.md) (Freigabe mit Auflagen)
und [Welle-3-Re-Review](../handoffs/WAVE3-REREVIEW.md); offen niedrig: N4 (Wiederherstellung bei unbestimmtem CREATE-Ausgang).
Die Integration vergab API 0.16 (nicht 0.17).

## Kontext

Views, Routinen, Trigger und Events benötigen SQL-Körper. Strukturierte Parameter,
Objektbezüge und Optionen allein können deren Inhalt nicht vollständig ausdrücken.
Der Auftrag erlaubt diese Körper als Nutzer-SQL ausschließlich mit exakter Vorschau
und Bestätigung. Routinenaufrufe, Scheduleränderungen und SQLite-Tabellen-Neuaufbau
sind ausdrücklich ausgeschlossen.

## Umsetzung

[API 0.16](../../contracts/PROGRAMS-API.md) baut additiv auf v12 auf. Katalogbezüge
und feste strukturierte Optionen erzeugen den DDL-Rahmen. Jeder vollständige Körper
bleibt ein Treiberstatement; innere Semikolons werden nicht durch den Client getrennt.
SQLite/PostgreSQL verwenden Prepare, MySQL/MariaDB deaktivierte MultiStatements.
Ein Plan verspricht keine Syntaxprüfung; der Server prüft beim bestätigten Anwenden.
DROP und jedes Ersetzen verlangen zusätzlich zum Plan-Hash den getippten Katalognamen.
Definer-/Eigentümerwechsel und mögliche privilegierte Kontexte werden angezeigt.

SQLite/PostgreSQL führen DDL-Gruppen transaktional aus. MySQL/MariaDB können nach
DROP bei fehlgeschlagenem CREATE ein entferntes Objekt hinterlassen; die Antwort
meldet dann einen unsicheren Teilabschluss, ohne automatischen Retry. Abhängigkeiten
bleiben begrenzte Katalog- oder Texthinweise. Ersetzungspläne zeigen die Vereinigung
bestehender und aus der neuen Definition erkennbarer Beziehungen, keine garantierte
Prognose des späteren Katalogzustands.

## Grenzen und Alternativen

Ein eigener SQL-Parser oder Linter ist nicht implementiert. Freie vollständige
DDL-Wrapper und DELIMITER-Protokolle sind keine zusätzliche Eingabeform. Der
zustandslose Hash ist keine Einmalfreigabe; namensbasierte Katalogprüfung schließt
einen Austausch durch andere Sitzungen nicht aus. SQLite-Triggerattribute stehen
im Definitionstext. Server-Livetests sind geschrieben, in der Sandbox aber wegen
fehlenden Dockerzugriffs offen; Prüfstand im [Handoff](../handoffs/W-P19-backend.md).
