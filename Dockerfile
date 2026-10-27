# SPDX-License-Identifier: AGPL-3.0-only
FROM golang:1.27.1-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY web-build.lock.json ./
RUN CGO_ENABLED=0 go test ./internal/webui
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/db-manager ./cmd/db-manager

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && addgroup -g 10001 app && adduser -D -u 10001 -G app app && mkdir /data && chown app:app /data
LABEL org.opencontainers.image.source="https://github.com/MagicalWig34653/db-manager" \
      org.opencontainers.image.documentation="https://github.com/MagicalWig34653/db-manager/blob/main/docs/BETRIEB.md" \
      org.opencontainers.image.licenses="AGPL-3.0-only" \
      org.dbmanager.frontend.source="https://github.com/MagicalWig34653/db-manager-web/tree/f163862cfe8c85c4982f4709ae4c973fac17dc9c"
COPY --from=build /out/db-manager /usr/local/bin/db-manager
COPY LICENSE /usr/share/licenses/db-manager/LICENSE
COPY licenses/ /usr/share/licenses/db-manager/go/
COPY web-build.lock.json /usr/share/db-manager/web-build.lock.json
COPY internal/webui/assets-manifest.json /usr/share/db-manager/web-assets-manifest.json
USER 10001:10001
ENV DBM_ADDRESS=0.0.0.0:8080 DBM_TOKEN_FILE=/run/secrets/api_token DBM_DATA_DIR=/data
EXPOSE 8080
VOLUME ["/data"]
HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 CMD ["db-manager", "healthcheck"]
ENTRYPOINT ["db-manager"]
