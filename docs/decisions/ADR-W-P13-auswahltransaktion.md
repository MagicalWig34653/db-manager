# ADR W-P13: Auswahltransaktion und begrenzte Dateiwerte

Status: Vorschlag, im W-P13-Reviewkandidaten implementiert; Annahme und endgültige
Nummer durch Lead/Review offen. Datum: 25.10.2026.

## Kontext

Die bestehende v3/v4-Eignung schützt Einzelzeilen unter Lock und meldet Commitfehler
konservativ als unbekannt. Mehrere einzeln bestätigte HTTP-Mutationen könnten dennoch
ungewollte Teilerfolge erzeugen. Ein reiner SQL-Plan-Hash bindet keine Parameterwerte
oder Zeilenstabilität. Die vorhandenen Treiber binden Binärwerte vollständig; eine
unbegrenzte Dateiübertragung wäre damit kein ehrliches Streamingversprechen.

## Vorgeschlagene Entscheidung

Alle ausgewählten Zeilen bilden eine gemeinsame Transaktion. Jede Zeile braucht ein
vollständiges kanonisches Abbild und ein serverseitig berechnetes Stabilitätsmerkmal.
Der domänengetrennte Planhash bindet außerdem Ziel, Metadaten, Reihenfolge, Aktion und
gebundene Werte. Vor der ersten Änderung werden alle Zeilen gesperrt und geprüft.
Constraint-/Snapshotfehler rollen den gesamten Batch zurück; zeilenweise Ergebnisse
melden durchgehend denselben Transaktionsausgang. Es gibt keinen Modus mit Teilerfolg.

Die geprüften v3/v4-Schreibblöcke werden in interne Helfer mit übergebener Session
extrahiert. Die Einzelzeilenrouten behalten ihre Validierung und eigenen Commit.
Dadurch verwenden Mehrzeilenoperationen dieselben Konflikt-, Identitäts-, Rule-,
SQLite-REPLACE- und Nachherprüfungen. Der neue Planhash ist bewusst ein Datenplanhash
und kein `dbplan.Digest` ausschließlich über SQL-Anweisungen.

Dateiwerte bleiben auf 512 KiB begrenzt und erhalten keinen Dateisystempfad.
HTTP liest/schreibt Streams; DB-Parameter und kanonische Snapshots sind begrenzte
Speicherpuffer. Binärdownloads bleiben Attachments. Vorschauen dekodieren ausschließlich
Rasterformate und kodieren sie neu als begrenztes PNG. Aktive Formate sind ausgeschlossen.

## Folgen und Grenzen

Zeilenkonflikte verhindern einen ganzen Batch; Clients müssen neu laden und bestätigen.
ABA, externe Triggerwirkungen, Sequenzlücken und unbekannte Commits bleiben Grenzen.
Kein einmalig verwendbarer Plan oder automatischer Retry. Ein großer alter Dateiwert
kann zusammen mit dem neuen Wert die unveränderte 1-MiB-Anfragegrenze überschreiten.
Keine neuen Abhängigkeiten oder Änderung der SQLite-Betriebsgrenzen aus ADR-0004.

Verhalten und Prüfgrenzen: [Vertrag](../../contracts/ROWS-BULK-API.md),
[Handoff](../handoffs/W-P13-backend.md).
