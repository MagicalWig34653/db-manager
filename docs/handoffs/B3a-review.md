# B3a: unabhängiger Security-/Backendreview und Fixnachprüfung

23.10.2026. Eigener Branch `review/b3a`, Worktree `.worktrees/b3a-review`.
Geprüfter Erstand `77eccb7add990ad65fc6149103c548a40e9b7f5f`, Handoff `31c86ca`,
Gesamtdiff ab `5cb604a04a524aac0e4cf0ff3e35d1d90c473e74` einschließlich WIP
`a582c45`. Vom Backendautor unabhängig; keine Produktdatei geändert.

## Urteil

Ein mittlerer Befund **B3a-01** blockierte den Erstkandidaten. Fix
**`7c66bb908a635e1de8e212a2003e3e0826de4210`** unabhängig eng nachgeprüft und
Befund geschlossen. Keine weiteren bestätigten fachlichen/securityrelevanten
Befunde im begrenzten Review. Empfehlung für Produktintegration und getrennt
für den0.5-Vertragsfreeze nach erfolgreicher relevanter exakter CI; tatsächliche
Freigabe/Integration ausschließlich durch den Lead bzw. ausdrücklich beauftragten
Releaseagent. Dieser Review friert keinen Vertrag selbst ein.

Kein DDL-/CREATE-Umfang, keine Garantie atomarer Schemasnapshots oder vollständiger
Metadatensichtbarkeit bei beliebigen DBprivilegien. Bestehende Host-/TLS-/VFS-/
SQLite-SingleReplica-Grenzen bleiben maßgeblich.

## B3a-01: Constraintidentität über Typnamespaces vermischt, mittel

Eigene echte MySQL8.4.11-TLS-Fixture akzeptierte diese ausschließlich synthetische
Definition:

```sql
CREATE TABLE peer_namespace(
 id INTEGER PRIMARY KEY,u INTEGER,v INTEGER,
 CONSTRAINT peer_same UNIQUE(u),
 CONSTRAINT peer_same CHECK(v>0)
);
```

Vor Fix meldete Structure nur einen `peer_same` vom Typ `unique` mit
`columns:["u","u"]`; CHECK fehlte trotz `coverage.checks=catalog`.
Ein eigener zweiter Fall mit gleichnamigem UNIQUE(u) und FK(f), separatem FK-
Stützindex, lieferte `columns:["u","f","u","f"]` und verlor den FK.
Ursache: Katalogjoins und Gruppierung in `structure_mysql.go` identifizierten nur
den Namen; MySQL hat getrennte Constrainttyp-Namespaces. [Primärquelle](https://dev.mysql.com/doc/refman/8.4/en/create-table-check-constraints.html).

MariaDB12.3.3 lehnte den ersten UNIQUE/CHECK-Fall nativ mit Error1826
`Duplicate CHECK constraint name` ab; dies ist **kein** Produktfehler dieses
Mariafalls. Gleichnamiges UNIQUE/FK mit eigenem Stützindex ist dort zulässig und
wurde im unabhängigen Nachlauf ebenfalls tatsächlich ausgeführt. Versions- und
Treibergrenzen werden nicht aus MySQL auf MariaDB übertragen.

Fix trennt Gruppierung und Sortierung nach Name **und Typ**. KCU-Zuordnung trennt
FK über vorhandenen Referenztabellenbezug von PK/Unique; CHECK erhält keine
fremden KCU-Spalten, Referential-Join nur für FK. Unabhängige identische Proben
nun grün: MySQL UNIQUE/CHECK und UNIQUE/FK, MariaDB UNIQUE/FK, mit je korrekter
Typ-/Spalten-/Zielidentität. Zusätzlich neue Autorenregressionen aller drei
MySQL-Paare einschließlich FK/CHECK und der tatsächlichen Mariaabweichung grün.

## Reproduzierbare Prüfung und tatsächliche Belege

Beide Codekandidaten wurden aus frischen `git archive`-Kopien geprüft. Eigene
zusätzliche Tests wurden nur in diesen temporären Reviewkopien ergänzt, nicht
im Produktworktree. Basiskommandos:

```sh
go test -race -count=1 ./...
go vet ./...
python3 scripts/check-structure-contract.py
python3 scripts/test-structure.py
```

- Erststand: vollständige Race-/Vet- und Vertragsprüfung grün. Originale echte
  TLS-Matrix auf MySQL8.4.11/MariaDB12.3.3/PostgreSQL18.6 grün; eigene unabhängige
  Namensraumfälle rot. Die vollständigen Tests wurden nicht wegen bloß
  übersprungener Netzwerkfälle als Live-Erfolg ausgegeben.
- Fix: gezielter Race-/Vet-Lauf für Datenbank/HTTP/Sandbox, unveränderter Vertrag
  und erneut vollständiger echter Drei-DB-TLS-Lauf einschließlich unabhängiger
  Namensraumproben grün. Keine Wiederholung unveränderter Deployment-/Web-/Vault-
  oder voller TLS-Angriffsmatrix ohne Anlass.
- Eigener SQLite-Fall mit WITHOUT-ROWID, zusammengesetztem PK(b,a), Unique und
  CHECK: echte PK-Termreihenfolge, keine erfundene Rowid, CHECK-Coverage korrekt
  unavailable. Eigene Negativfälle sperren writable_schema, WAL, integrity_check,
  Extensionladen und angehängtes ATTACH; danach Struktur erneut nutzbar.
- Eigene HTTP-Proben: Auth401; Root-/Nestednull einschließlich durch doppelten
  Schlüssel verdecktem Nullwert, unbekannte Felder und mehrere JSONdokumente400;
  Abbruch408 und anschließende erfolgreiche Anfrage; No-Store; ein synthetischer
  CHECK-/Partial-Index-Ausdrucksmarker erscheint nicht in der Strukturantwort.
- Autorenfälle unter unabhängiger Wiederholung: Compositekeys, Unique/FK-Aktionen,
  Selbst-/andersschema-FK, Sonder-/Leernamen, implizite SQLite-FK-Ziele ausdrücklich
  unbekannt, expression/partial/include/prefix/direction/collation, PK nicht nach
  Namen geraten, PostgreSQL NOT VALID/deferrable/NULLS NOT DISTINCT, Views gesperrt.
- Quellprüfung: neue Katalogbezüge gebunden; keine Ausdrucks-/Prädikat-/CREATEtexte
  im Ausgabeformat. PostgreSQL-Foreign/Views und SQLite-virtuelle Tabellen werden
  nach tatsächlichem Relationstyp gesperrt. Nur index_xinfo zur PRAGMA-Allowlist
  hinzugefügt, keine mutierenden PRAGMAs freigeschaltet. Body-/Anzahl-/Term-/
  Rohstring-/tatsächliche JSONbytegrenzen und Kontext/Cleanup kontrolliert;
  Größen-/Escaping-/Cancelregressionen unter Race wiederholt.

v1–v4 sind bytegleich; alte Pfade und Definitionen bleiben in v5 unverändert.
Schema0.5-SHA256 vor/nach Fix identisch:
`b5225339511d80fe4f52f09118fdad2060d8a92f31d44b73dd89fe9f76178c95`.
Keine neue Vollvalidierung mit externem JSON-Schema-Metaschemavalidator behauptet.

## CI und Ressourcen

Alle acht ursprünglichen CIs auf77eccb7 unabhängig als erfolgreich kontrolliert.
Am exakten Fix7c66bb9 abschließend alle acht CIs erfolgreich: Struktur39902565413,
Tabellen39902565315, Mutation39902565373, Insert39902565367, Docs39902565357,
Engine39902565466, Backend39902565392 und Web39902565363. [Exakte Struktur-CI](https://github.com/MagicalWig34653/db-manager/actions/runs/39902565413).

Eigenes Liveskript verwendet ausschließlich zufällige `dbmanager-b3a-*` Container,
dynamische Loopbackports und eigene synthetische Zertifikate. Finally-Cleanup
erfolgreich; keine eigenen Container/Volumes oder lokalen Prüfjobs verbleiben.
Temporäre Quell-/Probeverzeichnisse anschließend entfernt. Fremde bestehende Dienste,
Host-Swarm und OS-Rechte unverändert. Keine Produktintegration durch diesen Review.
