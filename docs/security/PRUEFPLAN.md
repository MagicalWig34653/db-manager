# Sicherheitsprüfplan und Freigabegates

Stand: 20.10.2026. Jede Zeile ist ein auszuführender Abnahmetest, soweit nicht
ausdrücklich als geprüft markiert. Referenzen: [Entwurf](TRESOR-UND-SYNC.md),
[Bedrohungsmodell](BEDROHUNGSMODELL.md), [Spike](../../spikes/security/README.md).
Nur synthetische Zugangsdaten und isolierte Wegwerf-Datenbanken verwenden.

Fortschreibung S1: [Interop-/Formatbefunde](VAULT-FORMAT-V1.md) ergänzen SEC-005
mit tatsächlichem gegenseitigem Go-/Swift-Rundlauf einschließlich Argon2id,
Passwort-/Recoverywegen und Negativfällen. SEC-006 hat eine konkrete gepinnte
Bibliotheksgrundlage und Desktopmessungen; vollständige RFC-Vektor-/Mindestgeräte-
und Ressourcenabnahme bleibt offen. SEC-009 ist auf Codecebene in neuen Prozessen
belegt, aber nicht als App-Erstinstallation/Keychain-/Recoveryoberfläche. Kein
S1-Befund ersetzt Geräte-, Sync-, Persistenz- oder Teamabnahmen der Tabelle.

| ID | Fall und konkrete Abnahme | Status in P3a |
| --- | --- | --- |
| SEC-001 | Zufälliger Root schützt zufälligen Datenschlüssel; Payload-Rundlauf erhält alle Bytes; gleiche Eingabe zweimal ergibt verschiedene Hüllen/Payloads | Go-Spike geprüft |
| SEC-002 | Jeweils jedes Byte von Keyhülle/Payload ändern und jede mögliche Trunkierung prüfen; keine Variante liefert Klartext | Go-Spike geprüft |
| SEC-003 | Andere Tresor-ID, Datensatz-ID, Revision, Version, falscher Root, leeres Feld oder vertauschter Zweck wird abgewiesen | Go-Spike geprüft, endgültiges Format offen |
| SEC-004 | Gültiges altes Objekt mit altem Kontext bleibt kryptografisch gültig; vertrauenswürdiger neuer Kontext weist es ab | Go-Spike bestätigt diese AEAD-Grenze; Sync-Rollbacktest offen |
| SEC-005 | Go erzeugt vollständigen versionierten Vektor, Swift entschlüsselt; Swift erzeugt, Go entschlüsselt; manipulierte AAD scheitert beidseitig | offen |
| SEC-006 | Argon2id-Implementierungen bestehen RFC-Vektoren, gemeinsame Passwort-Bytekodierung einschließlich Unicode/Leerzeichen; Zeit-/RAM-Messung am kleinsten Zielgerät | offen |
| SEC-007 | Unbekannte/überhöhte KDF-Parameter, überlange Hüllen und ungültige Längen vor teurer Allokation ablehnen; keine stillen Parameterabsenkungen | offen |
| SEC-008 | Masterpasswort ändern: altes scheitert am neuen Stand, neues öffnet; Stromabbruch an jeder Schreibgrenze erhält genau einen konsistenten lesbaren Stand | offen |
| SEC-009 | Recovery auf frischer Installation ohne alten Keychaininhalt; Recovery-Key plus Ciphertext öffnet; einzelnes davon reicht nicht; falscher Key ergibt generischen Fehler | offen |
| SEC-010 | Verlust aller Entsperrwege bleibt auch nach Backend-Kontoreset unentschlüsselbar; Bestätigung war vor echter Geheimnisspeicherung erforderlich | offen |
| SEC-011 | Keychain-Entsperren nach App-/Gerätesperre, biometrischer Änderung, Passcodeentfernung, Backuprestore und neuem Gerät prüfen; gewählte Policy bleibt nachvollziehbar | offen, echtes Gerät nötig |
| SEC-012 | Sperren verhindert neue Secrets und beendet/widerruft wartende Anfragen; aktive DB-/SSH-Verbindungen folgen expliziter Sitzungsregel | offen |
| SEC-013 | Plattform-PRF-Request mit Availability Guards gegen macOS-15-Ziel kompilieren; Security-Key-PRF separat ab 26.4 | Compileprobe vorhanden/geprüft; keine Zeremonie |
| SEC-014 | Signierte Apple-App mit AASA/RP-Domain: Registrierung, PRF-Supportcheck, Ausgabe beim Entsperren, tatsächliche Root-Entschlüsselung | offen; Team-/Domain-/Signingzugang nötig |
| SEC-015 | Gleiches Credential/gleicher PRF-Input wiederholt liefert denselben Entsperrweg; anderes Credential/Input öffnet nicht; fehlende Extension fällt auf Masterpasswort zurück | offen |
| SEC-016 | Passkey auf zweitem realen Apple-Gerät, nach Providerwechsel und nach Apple-Account-/Passkey-Recovery testen; Ergebnisse je OS/Provider protokollieren | offen |
| SEC-017 | Mehrere Passkeys hinzufügen/entfernen; Ersatzcredential nach Verlust öffnet alte Hülle nicht; letztes Recoverymittel nicht unbemerkt entfernen | offen |
| SEC-018 | Native standalone Erstinstallation offline, nach AASA-Caching offline und erneute Installation offline getrennt prüfen; keine unbelegte vollständige Offline-Passkey-Zusage | offen |
| SEC-019 | Browser-PRF pro Safari/Chromium/Firefox und Plattform-/externem-/Drittanbieterprovider prüfen; Geheimnisresultate nicht in Loginpayload/Logs | offen |
| SEC-020 | Zwei echte Apple-Geräte über private CloudKit-Zone: Ciphertext-Upload, neuer Gerätedownload und unabhängiges Entsperren; Export/Netzaufzeichnung enthält keine eigenen Klartextgeheimnisse | offen |
| SEC-021 | Ohne iCloud oder Go neu starten, lokal entsperren und direkte DB nutzen; Netzverlust/Rate-Limit puffert Änderungen und synchronisiert später genau einmal logisch | offen |
| SEC-022 | Beide Geräte offline ändern dasselbe Passwort; nach Sync beide Revisionen erhalten, Konflikt sichtbar, Auswahl bewusst und danach konvergent | offen |
| SEC-023 | Löschen auf Gerät A, alte Änderung von offline Gerät B; Tombstone verhindert stilles Wiederanlegen; Hüllenrotation gleichzeitig mit Datenänderung bleibt lesbar | offen |
| SEC-024 | iCloud-Logout/Accountwechsel/Zonenverlust/Apple-Keychainreset: lokaler Ciphertext bleibt erhalten; kein ungefragtes Mischen/Löschen/Neuerstellen des Tresors | offen |
| SEC-025 | Gültiges altes Backup auf Gerät mit Versionshöchststand einspielen: Warnung/Abweisung; auf frischem Gerät verbleibende Nichterkennbarkeit dokumentieren | offen |
| SEC-026 | Root-/Dienstschlüsselrotation mit gleichzeitigem Sync und Prozessabbruch: atomare Epoche, alte Backups wiederherstellbar nach dokumentierter Policy | offen |
| SEC-027 | Persönliche Verbindung bleibt für Teamadmin und beliebige fremde Objekt-IDs unlesbar; Übernahme verlangt bewusste Feldauswahl und Zielteam | offen |
| SEC-028 | Getrennte Rechte für Metadaten, Ausführung, Geheimnisanzeige und Freigabe; horizontale/vertikale Rechteausweitung per API abweisen | offen |
| SEC-029 | Mitglied während wartendem/laufendem Job entfernen; neue Anfrage scheitert, Job prüft Berechtigung neu, Wiederanlauf verwendet keinen veralteten Cache | offen |
| SEC-030 | Nach Team-Rechteentzug bekannte alte DB-Credentials demonstrieren; tatsächliche Sperre erst mit DB-Rotation/-Sitzungsentzug bestätigen | offen |
| SEC-031 | Synthetisches eindeutiges Secret durch jeden Fehlerpfad führen; Logs, Crash-/Auditdateien, Browserstorage und Backend-Metadaten enthalten es nicht | offen |
| SEC-032 | TLS-CA-/Hostfehler und geänderter SSH-Hostschlüssel verhindern Verbindungsaufbau; kein automatisches unsicheres Fallback | offen |
| SEC-033 | SSRF-Ziele einschließlich IPv4/IPv6, Link-Local, Metadaten-IP, DNS-Rebinding und SSH-Weiterleitung gegen Betreiberregel prüfen | offen |
| SEC-034 | Compose-Linux: nur zugewiesener Dienst liest Dateisecret; Git/Image/Configausgabe enthalten keine Werte; Quellrechte und Restore dokumentiert | offen |
| SEC-035 | Swarm-Linux mit zwei Replikaten: Secretbereitstellung, kontrollierte Rotation, gemeinsam wirksamer Rechteentzug, genau ein logischer Jobstart nach Rescheduling | offen |
| SEC-036 | Benötigtes Secret fehlt/ist unlesbar/falsch: Dienst wird nicht bereit; keine Klartextfallbacks; bei Rotation nur vollständige gültige Konfiguration aktivieren | offen |

Reale Plattformmatrix mindestens: Mac/iPad mit technischer API-Untergrenze und
aktueller freigegebener OS-Version, jeweils Apple- und tatsächlich unterstützter
Drittanbieterprovider. Simulator/Compiletest belegt keine Keychain-/CloudKit-/Passkey-
Recovery. Jeder Befund nennt Build, OS, Gerätetyp, Provider, Entitlements, Ausgang
und verbleibende Grenzen, ohne Geräte-IDs oder echte Kontodaten zu veröffentlichen.

Freigabefolge: S0 benötigt einschlägige Verbindungs-/Netz-/Anmeldeprüfungen; S1
benötigt SEC-001 bis SEC-012 sowie Speicher-/Fehlerprüfungen; S2 zusätzlich PRF-
Fälle der beworbenen Plattformen; S3 zusätzlich Syncfälle; S4 zusätzlich Team-/Job-
und Deploymentfälle. Nicht angebotene optionale PRF-Provider brauchen kein positives
Testergebnis, müssen aber ehrlich als nicht unterstützt dargestellt werden. Vor
Produktfreigabe Sicherheitsreview und versioniertes ADR für die konkrete Auswahl.
