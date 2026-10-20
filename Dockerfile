# SPDX-License-Identifier: AGPL-3.0-only
FROM golang:1.27.1-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/db-manager ./cmd/db-manager

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && addgroup -g 10001 app && adduser -D -u 10001 -G app app && mkdir /data && chown app:app /data
COPY --from=build /out/db-manager /usr/local/bin/db-manager
USER 10001:10001
ENV DBM_ADDRESS=0.0.0.0:8080 DBM_TOKEN_FILE=/run/secrets/api_token DBM_DATA_DIR=/data
EXPOSE 8080
VOLUME ["/data"]
HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 CMD ["db-manager", "healthcheck"]
ENTRYPOINT ["db-manager"]
