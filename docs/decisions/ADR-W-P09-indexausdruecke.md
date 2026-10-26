# ADR-W-P09: Strukturierte Indexausdrücke und Schlüsselersetzung

Status: Implementierungsentscheidung im Reviewkandidaten, Leadannahme offen.
Datum: 25.10.2026. Geltungsbereich: W-P09 / API 0.13.

Der Auftrag verlangt PostgreSQL-Ausdrucks- und Teilindizes. Die Leitplanken erlauben
keine freien SQL-Fragmente in strukturierten Schreibpfaden. Deshalb verwendet W-P09
nur die feste Funktionsauswahl `lower`/`upper` auf kataloggeprüften Textspalten und
strukturierte NULL-Prädikate. Funktionen werden explizit aus `pg_catalog` aufgerufen.
Ein allgemeiner SQL-Ausdrucksvalidator wäre eine eigene, größere Sicherheitsgrenze.

Änderungen an Indizes/Fremdschlüsseln ersetzen vollständige Definitionen. PostgreSQL
führt DROP/CREATE in einer Transaktion aus, MySQL/MariaDB verwenden eine kombinierte
ALTER-Anweisung. Der bestehende W-P08-Runner regelt Bestätigung und sichere Outcomes;
keine neuen Zusagen über Replay, ABA oder universelles DDL-Rollback. SQLite erhält
nur CREATE/DROP INDEX; kein impliziter Tabellenneuaufbau. Beobachtung der Schlüssel
bleibt getrennt über die vorhandene API v5.

Folge: enger, verständlicher Funktionsumfang mit ehrlicher Fähigkeitsauskunft.
Beliebige Ausdrücke, PostgreSQL CONCURRENTLY/INCLUDE/Opclasses, allgemeine Prädikate
und SQLite-Neuaufbau bleiben außerhalb dieses Pakets. Siehe [Verhalten](../../contracts/INDEXES-API.md).
