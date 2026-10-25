# ADR W-P18: Geschützte Passwortwerte in Kontoplänen

Status: paketbezogene Implementierungsentscheidung zu W-P18; API-Nummer vorläufig,
keine zentrale Produkt- oder Vertragsfreigabe. Datum: 24.10.2026.

## Anlass

E9 erlaubt bestätigte Rechteverwaltung. E2 lässt Produktkonten offen. Kontenbefehle
mit Passwörtern müssen prüfbar sein, ohne Passwörter in Vorschau, Antworten oder
Plan-Hash aufzunehmen. Universelle gebundene Parameter unterstützt die DDL-Syntax
der beteiligten Treiber nicht.

## Entscheidung

Das Paket verwaltet ausschließlich Datenbankkonten. Nur Ausführungsanfragen tragen
den geschützten Wert `secret`; Plananfragen weisen ihn ab. Die normalisierte
Anweisungsliste enthält einen festen Passwortplatzhalter. Nach Neuberechnung des
Plans und Prüfung von Hash, Aktion, getippter Kennung und zusätzlicher
Eigenkonto-Bestätigung ersetzt die Ausführung nur die vorgesehene letzte Position.
Die Sitzung erzwingt deterministisches Literalquoting. Keine Protokollierung von
Eingaben, expandiertem SQL oder rohen Treiberfehlern; keine Passwort-Hashes lesen.

Kontenkennung enthält Namenslänge, Namen und Host. Destruktive Aktionen verlangen
exaktes Abtippen. Rechte-/Rollenentzug und Löschen verlangen vorsorglich immer die
zweite Bestätigung, weil geerbte Rechte das eigene Konto indirekt betreffen können.
Die tatsächlichen Datenbankrechte bleiben die maßgebliche Autorisierung.

## Folgen

Ein Passwortwechsel verändert den Plan-Hash nicht. Der Hash ist kein
Passwortnachweis, keine Einmalbestätigung und kein Objekt-CAS. Clients müssen nach
Erfolg oder unbekanntem Ausgang sperren und neu planen. Kopie ist eine ausdrücklich
begrenzte Vorlage direkter Rechte/Rollen, keine vollständige Kontenreplikation.
MySQL-Mehrschrittfolgen bleiben nicht atomar. PostgreSQL nutzt eine Transaktion.

Vertrag und Grenzen: [Konten-API](../../contracts/ACCOUNTS-API.md).
Prüfstand: [Handoff](../handoffs/W-P18-backend.md).
