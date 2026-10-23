# GHCR-IMAGE: Container-Image in ghcr.io veröffentlichen

Branch `feat/ghcr-image`. Begrenztes Paket mit Besitz an `.github/workflows/image.yml`,
`compose.yaml`, `deploy/swarm.yaml`, `scripts/test-deployment.py`, `docs/BETRIEB.md`.

## Ergebnis

- Workflow baut das `Dockerfile` (buildx, `linux/amd64`, Actions per Commit-SHA gepinnt),
  pusht nur bei Push auf `main` und Tags `v*` nach `ghcr.io/magicalwig34653/db-manager`
  (Tags `sha-<kurz>`, `latest`, Versionen). Pull Requests bauen ohne Login und Push.
  `packages: write` nur im Image-Job. Provenienz und SBOM sind abgeschaltet; keine Signatur.
- `compose.yaml` und `deploy/swarm.yaml` verwenden Standard
  `${DBM_IMAGE:-ghcr.io/magicalwig34653/db-manager:latest}`; Compose behält `build: .`,
  Swarm hat kein `build:`. `test-deployment.py` ersetzt den neuen Standardstring.
- `arm64` bewusst nicht gebaut: `go test` im Build unter QEMU wäre langsam; Cross-Build
  würde Dockerfile-Änderung erfordern.

- `deploy/swarm.yaml`: Volume `data` explizit `driver: local` mit Kommentar; Doku und
  ADR-0004-Bezug beschreiben node-lokales Volume, Pinning, kein verteilter Speicher, keine
  HA-/Failover-Zusage. Volumenlose Variante (nur Remote-DBs) ist dokumentiert; belegt nur
  durch einen Einzelcontainer-Start (read-only, tmpfs, healthy), nicht als Swarm-Stack.
  Hinweis: `VOLUME /data` im Dockerfile erzeugt dabei ein anonymes, flüchtiges Volume.

## Offene manuelle Schritte

- Paket nach erstem Push auf Wunsch einmalig auf `public` stellen (siehe `docs/BETRIEB.md`).
- Lokal gelaufen: check-docs, gofmt, go vet, actionlint, test-deployment.py (Compose und isolierter Swarm grün). CI-Läufe: siehe Bericht des Leads; hier nicht vorab behauptet.
