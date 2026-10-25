# ADR-0008: Begrenzter vollständiger Konfliktvergleich beim Ersetzen

Status: **Paketentscheidung W-P14, unabhängig noch zu prüfen**. Datum: 24.10.2026.

## Kontext

Die Ersetzungsvorschau muss ohne Schreiben auskommen und darf bei Bestätigung
keinen inzwischen geänderten Bestand mit einem alten Plan ersetzen. Eine reine
Trefferzählung erkennt geänderte Werte bei gleicher Anzahl nicht. Ein offener
massenhafter UPDATE über ein Muster begrenzt weder Speicher noch direkten Umfang.

## Entscheidung

Ersetzungen je Tabelle/Spalte konservativ auf den vollständigen begrenzten
Tabellenbestand beschränken (maximal 1000 Zeilen). Vollständige kanonische Zeilen,
Metadaten und nativ berechnete Nachherwerte in den Plan-Hash aufnehmen. Nur
transaktionale Basistabellen mit stabilem PK zulassen; unter Sperren neu berechnen,
danach gebundene Einzelupdates und vollständige Nachprüfung in derselben Transaktion.
Die [API-Beschreibung](../../contracts/SEARCH-API.md) definiert Details und Grenzen.
Keine Bestands-API erweitern; zusätzliche Filter liegen allein auf v15.

## Folgen und Alternativen

Das ist strenger als ein Limit nur für Treffer: große Tabellen werden abgewiesen,
auch wenn eine einzelne Zeile passen würde. Dafür sind Vorschau und kompletter
Konfliktvergleich begrenzt, unveränderte Nebenfelder werden berücksichtigt und
Triggeränderungen können vor Commit erkannt werden. Unbegrenztes COUNT plus UPDATE
wurde verworfen. Native Funktionen erhalten DB-Semantik; keine clientseitige Regex-
Emulation. ABA, externe Effekte und Netzwerk-Phantome bleiben ausdrücklich begrenzt.
Unabhängige Abnahme und zentrale Nummernprüfung durch den Lead stehen noch aus.
