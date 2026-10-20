# Übergabe P3a – Bedrohungsmodell und Sicherheits-Spike

Stand: 20.10.2026. Branch `docs/p3a-bedrohungsmodell`, eigener Worktree
`.worktrees/p3a`. Paketbearbeiter: delegierter Sicherheits-Agent, Lead koordiniert.
Status: zur unabhängigen Prüfung; kein akzeptiertes Architektur-ADR und keine
produktive Freigabe von Tresor, iCloud, PRF oder Teams.

## Ergebnis und Dateien

- [Bedrohungsmodell](../security/BEDROHUNGSMODELL.md): Schutzgüter, Vertrauensgrenzen,
  Angreifer, Sperren, Verlust, Wiederherstellung, Rechteentzug und Betreibervertrauen.
- [Befunde und Entwurf](../security/TRESOR-UND-SYNC.md): Primärquellen, native
  PRF-API-Untergrenzen, eigenständiges Apple ohne Go, gestufte Implementierung
  S0 bis S4 und Compose-/Swarm-Zielbedingungen.
- [Prüfplan](../security/PRUEFPLAN.md): 36 konkrete Prüfzeilen mit getrenntem
  tatsächlichem Nachweis und offenem Plattform-/Produktteststand.
- [Spike](../../spikes/security/README.md): ausschließlich synthetische Daten,
  AES-GCM-Keyhülle/Payload mit Kontextbindung sowie Compileprobe für native PRF.

Wesentlicher Befund: normale Passkey-Anmeldung liefert keinen Tresorschlüssel.
Das installierte SDK deklariert native Plattform-PRF ab macOS 15/iOS 18 und native
Security-Key-Registration-PRF ab 26.4; der Typecheck bestätigt API-Nutzung mit Guards.
Das belegt weder reale Providerunterstützung noch Offline-/Recoveryverhalten.
Apple-Passkeys benötigen Associated Domains; ein vollständig domainunabhängiger
Passkeybetrieb ist nicht nachgewiesen. Masterpasswort plus Recovery ist deshalb
der erste vorgeschlagene vollständige lokale Weg. Produktinkremente dürfen vorher
flüchtige Credentials verwenden.

Persönlicher iCloud-Sync transportiert anwendungseigenen Ciphertext, Go bleibt
optional. Go benötigt bei dort ausgeführten DB-Verbindungen Klartext; dessen
Betreiber ist hierfür vertrauenswürdig. Teamfreigaben übertragen nie den
persönlichen Root und benötigen gesonderte Berechtigungs-/Rotationsabnahme.
Compose-Dateisecrets und Swarm-Secrets haben unterschiedliche Hostschutzannahmen.

## Tatsächliche Prüfungen

Am 20.10.2026, Toolchains Go 1.27.1 Darwin/arm64 und Apple Swift 6.4:

| Befehl | Ausgang |
| --- | --- |
| `go test -race -count=1 -v ./...` in `spikes/security` | bestanden: drei Haupttests und zehn Unterfälle; zusätzlich byteweise Manipulation/Trunkierung beider Ciphertexte |
| `go vet ./...` dort | bestanden, Exit 0 |
| `swiftc -typecheck -target arm64-apple-macosx15.0 prf-api.swift` dort | bestanden, Exit 0; keine echte Autorisierung/kein OS-Laufzeittest |
| `gofmt -l envelope_test.go` dort | keine Ausgabe nach Formatierung |
| `python3 scripts/check-docs.py` im Hauptrepo-Worktree | 78 Dateien geprüft, 0 Fehler |
| `git diff --check` im Hauptrepo-Worktree | bestanden, keine Ausgabe |

Eigene neue Dateien einzeln geprüft: keine echten Verbindungsdaten, privaten Pfade,
privaten Schlüssel oder produktiven Abfragen. Commit enthält ausschließlich die
oben verlinkten Paketdateien; seine Kennung ist aus dem Branchlog ersichtlich.
Git-Autoridentität unverändert aus vorhandener Konfiguration übernommen;
GitHub-noreply-Adresse, keine private Mailadresse. Keine laufenden Paketjobs nach
Abschluss der oben genannten Prozesse. Keine echten Secrets, kein Netzwerkdienst,
keine Migration und keine Datenbestandsänderung durch den Spike.

## Grenzen und nächster Schritt

P3a beschreibt ein implementierbares Untersuchungs-/Stufendesign und begrenzte
Spikes. P3 ist ausdrücklich nicht abgeschlossen: Passwort-KDF/-Bibliothekswahl,
sprachübergreifendes Format, atomare Dateien, Recovery-Rundlauf, Keychain, reale
Passkeyprovider, signierte App, echte iPads, CloudKit-Zweigerätetest und Team-/Swarm-
Tests fehlen. Keine dieser Prüfungen wird aus dem Go-Rundlauf abgeleitet.

Lead beauftragt unabhängigen Review des Commits vor Integration, entscheidet danach
über ADR/weitere Pakete und führt zentrale Status-/Planungsdateien fort. Dateibesitz
bleibt bis Integration bei diesem Paket. Die neue Docker-Compose-/Swarm-Anforderung
ist im Entwurf und in den Abnahmen enthalten; zentrale Architekturentscheidung
bleibt beim Lead. Keine unintegrierte externe Nebenarbeit außerhalb dieser Dateien.
