# phpMyAdmin-Quellcode-Inventar (Referenz 5.2.3)

Status: **Analyse, kein Beschluss.** Dieses Inventar erfasst den Funktionsumfang des Referenzstands
systematisch und ordnet ihn der Paritätsmatrix zu. Es ergänzt [PARITAET.md](../PARITAET.md) und ändert sie nicht.
Umsetzungsvorschlag: [FEATURE-PLAN.md](FEATURE-PLAN.md). Lücken der Matrix: [LUECKEN.md](LUECKEN.md).

## Referenz und Prüfung des Quellstands

- Quelle: `https://github.com/phpmyadmin/phpmyadmin`, Tag `RELEASE_5_2_3`, Commit
  `962857e4f63d42e38f11ff4d63f5e722018add76` (Referenzcommit laut PARITAET.md).
- Klon mit `--depth 1` in ein leeres Scratchpad-Verzeichnis. Geprüft: `git rev-parse HEAD` liefert den Referenzcommit,
  `git describe --tags` liefert `RELEASE_5_2_3`.
- Der Quellcode wurde nur gelesen. Es wurde nichts aus dem Checkout ausgeführt, kein Composer und kein npm aufgerufen.
  Hilfsskripte (Python) liefen nur über Dateien als Daten.
- Lizenz des Referenzcodes: GPL-2.0 (`LICENSE` im Checkout). Es wurde kein Code übernommen; alle
  Funktionsbeschreibungen sind eigene Formulierungen mit Quellverweis.

## Methodik

1. **Gliederung über Routen:** `libraries/routes.php` nennt alle HTTP-Einstiegspunkte. Jede dort
   referenzierte Controller-Datei ist ein Prüfpunkt.
2. **Controller-Digest:** Alle Dateien unter `libraries/classes/Controllers/` mit Zeilenzahl, Docblock und
   erkannten SQL-Verben (Hilfsskript, nur Lesen).
3. **Plugin-Familien:** Export, Import, Schema, Auth, TwoFactor, Transformations, jeweils per Dateiliste.
4. **Fachklassen:** `Server/`, `Engines/`, `Gis/`, `Partitioning/`, `Database/`, `Table/`, `Navigation/`,
   `ConfigStorage/`, Replikations-, Tracking-, Normalisierungs-, Advisor-, Logging-, IP-, Redirect-, Versions-,
   Fehlerbericht- und Git-Klassen.
5. **Abgleich mit der Matrix:** Jede Matrixzeile erhält mindestens eine Inventarzeile. Neue Funde erhalten
   `PMA-NEU-<Bereich>-<n>`. Matrix-IDs bleiben unverändert.
6. **Maschinelle Prüfung:** Jeder Quellverweis wurde gegen den Checkout geprüft (Datei vorhanden, Muster
   gefunden); die Zeilennummer ist der erste Treffer des Musters. Die Routen-Abdeckung wurde per Dateipfad
   geprüft. Die Matrixabdeckung wurde per ID geprüft.

## Abdeckung

| Bereich | Geprüft | Nicht im Detail geprüft |
| --- | --- | --- |
| Routen (`libraries/routes.php`) | 177 Controller-Einstiege; 177 per Dateipfad im Inventar genannt | Routen-Präfixe nur über Klassennamen, nicht als HTTP-Test |
| Controllers/ (189 Dateien) | Alle Routen-Controller; Basisklassen nur als Basis | Hilfsklassen ohne eigene Funktion |
| Plugins Export, Import, Schema, Auth, TwoFactor | Dateiliste je Familie, Docblock und Hauptmethoden | Optionen je Plugin nicht je Option ausgezählt |
| Plugins Transformations | Dateiliste; Plugins ohne Lektüre markiert (Hinweis je Zeile) | Optionen und Eingabe-/Ausgabelogik je Plugin |
| Fachklassen Server, Engines, Gis, Partitioning, Replication, Navigation | Klassenebene und Hauptmethoden | Zeile für Zeile |
| Oberfläche (`templates/`, 279 Dateien; `js/`, 113 Dateien) | Stichprobe: Tastatursteuerung (`js/src/keyhandler.js`), Template-Namen | Vollständige Bedienabläufe und Darstellung |
| Sprachen (`po/`, 95 Dateien) | Anzahl; Kataloge nicht gelesen | Vollständigkeit je Sprache, Rechtsschreibung, RTL-Layout |
| Themes (`themes/`) | Ordnernamen | Theme-Inhalt |
| Setup (`setup/`, `libraries/classes/Setup/`) | Setup-Controller und Konfigurationsgenerator | Installationsablauf im Detail |
| Konfiguration (`libraries/config.default.php`) | Ausgewählte Schalter (SaveDir, UploadDir, Servers, blowfish_secret, ssl, Konfigurationsschema) | Alle Direktiven (Matrix-Grenze 2) |
| Konfigurationsschema (`sql/create_tables.sql`) | Tabellennamen | Spaltendetails |
| Tests (`test/`) | Nicht ausgewertet | Testfälle als Verhaltensbelege |
| Dbal, Html, Query, Utils, Crypto, Charsets, Http, Image, Command | Nur Klassenebene oder gar nicht | Details |
| Handbuch (`docs.phpmyadmin.net`) | Nicht erneut abgerufen; Belege stammen aus PARITAET.md | Versions- und Abschnittsvergleich |

Der Abgleich der Plugin-Familien ergibt (Dateien mit Namen im Inventar genannt, ohne Abstract-, Interface- und Properties-Klassen):

- Export: 16 von 17; Import: 11 von 11; Schema: 23 von 23; Auth: 4 von 4; TwoFactor: 4 von 5; Transformations: 45 von 45.
- Nicht namentlich genannte Dateien stehen in [LUECKEN.md](LUECKEN.md), Abschnitt C.

## Ergebnis

- **Matrix:** 181 Zeilen; alle IDs sind im Inventar referenziert (Matrix-Deckung 181 von 181).
- **Neue Funde:** 112 Zeilen `PMA-NEU-*`. Vorschlag-Priorität: V1 10, V2 16, V3 24, V4 62.
- **Einordnung der neuen Funde:** 78 ergänzen eine bestehende Matrixzeile (Teilweise), 27 sind neue Vorschläge, 7 benötigen eine Produkt- oder Sicherheitsentscheidung, bevor sie übernommen werden.
- **Eignung für die Apple-App:** 8 Funktionen sind an PHP-Laufzeit, Webserver oder Browser gebunden (Apple entfällt); 60 sind für Apple offen.
- **Quellverweise:** 509 Permalinks auf den Tag-Commit (430 unterschiedliche Stellen).
- **Aufwand:** Vergeben sind S, M und L (Schätzung nach Quellumfang, nicht Projektaufwand). XL wurde nicht vergeben; große Funktionsgruppen sind über mehrere Zeilen zerlegt.

## Dateien

- Bereiche: [01 Verbindung und Navigation](01-verbindung-navigation.md),
  [02 Sicherheit und Authentifizierung](02-sicherheit-auth.md),
  [03 Serverstatus, Monitor, Advisor](03-server-status-monitor.md),
  [04 Serververwaltung](04-server-verwaltung.md),
  [05 Konten und Rechte](05-benutzerkonten-rechte.md),
  [06 Datenbanken, Tabellen, Struktur](06-datenbank-tabellen-struktur.md),
  [07 Daten, Suche, Geodaten](07-daten-suche-gis.md),
  [08 SQL und Ergebnisse](08-sql-ergebnisse.md),
  [09 Views, Routinen, Trigger, Events](09-views-routinen-trigger-events.md),
  [10 Import](10-import.md),
  [11 Export](11-export.md),
  [12 SQL-Dumpoptionen](12-sql-dump.md),
  [13 Wartung und Diagnose](13-wartung-diagnose.md),
  [14 Beziehungen, Designer, Schema-Export](14-beziehungen-designer.md),
  [15 Transformationen](15-transformationen.md),
  [16 Tracking, Zentralspalten, Normalisierung](16-tracking-zentralspalten-normalisierung.md),
  [17 Oberfläche und Einstellungen](17-oberflaeche-einstellungen.md),
  [18 Einrichtung und Konfiguration](18-setup-konfiguration.md).
- [LUECKEN.md](LUECKEN.md): Befunde zur Matrix und Liste der neuen Funde.
- [FEATURE-PLAN.md](FEATURE-PLAN.md): Umsetzungsvorschlag in Phasen (nicht beschlossen).

## Spaltenlegende

- **Priorität (Vorschlag):** Für Matrixfunktionen übernommen aus PARITAET.md. Für neue Funde ein Vorschlag des Inventars.
- **Einordnung:** `Anforderung (Matrix)` = Matrixzeile. `Teilweise: <ID>` = neue Funktion ergänzt diese Zeile.
  `Vorschlag neu` = neue Funktion, nicht in der Matrix. `Entscheidung offen` = Produkt- oder Sicherheitsentscheidung nötig.
  `Pflicht` = aus den Projektregeln.
- **Eignung Web/Go · Apple:** `Apple ja` = direkt in der App umsetzbar, `Apple offen` = Umsetzung oder Ersatz ist zu entscheiden,
  `Apple entfällt` = an PHP-Laufzeit, Webserver oder Browser gebunden. Die Einschätzung ist eine Analyse, keine Abnahme.
- **Voraussetzung:** Abhängigkeiten, Konfigurationsspeicher und Rechte. Versions- und Rechteangaben stammen aus dem Quellcode,
  soweit belegt; sonst aus allgemeiner Datenbankkenntnis und sind damit Annahmen, die vor Implementierung gegen die Serverdokumentation zu prüfen sind.
- **Hinweis:** Einschränkungen, Unsicherheiten, Abgrenzungen. `UNSICHER` markiert eine nicht geprüfte Aussage.

## Grenzen

- Kein Laufzeitvergleich mit phpMyAdmin (PARITAET.md, Vollständigkeitsgrenze 1). Keine Abnahmeevidenz.
- Prioritäten und Aufwandsklassen sind Vorschläge.
- Keine Übernahme von Quellcode; GPL-2.0-Code wird nicht in das AGPL-3.0-Projekt kopiert.
- Keine Änderung an PARITAET.md, STATUS.md oder ARBEITSPLAN.md.
