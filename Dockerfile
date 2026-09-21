FROM node:22-alpine AS web-builder

WORKDIR /src/web
COPY web/ ./
RUN npm ci && npm run build

FROM golang:1.25-bookworm AS server-builder

RUN apt-get update \
    && apt-get install -y --no-install-recommends zstd \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src/server
COPY server/go.mod server/go.sum ./
RUN go mod download

COPY server/ ./
COPY --from=web-builder /src/web/dist/ /src/web-dist/

RUN mkdir -p web/public/defaultTheme \
    && tar -cf /tmp/komari-web.tar -C /src/web-dist . \
    && zstd -19 -T0 -f /tmp/komari-web.tar -o web/public/defaultTheme/dist.tar.zst \
    && CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /out/komari .

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl tzdata \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 10001 komari \
    && useradd --uid 10001 --gid komari --no-create-home --home-dir /app --shell /usr/sbin/nologin komari \
    && mkdir -p /app/data \
    && chown -R komari:komari /app

WORKDIR /app
COPY --from=server-builder --chown=komari:komari /out/komari /app/komari

ENV GIN_MODE=release
ENV KOMARI_LISTEN=0.0.0.0:25774

USER komari
EXPOSE 25774
VOLUME ["/app/data"]

HEALTHCHECK --interval=15s --timeout=5s --start-period=20s --retries=5 \
    CMD curl -fsS http://127.0.0.1:25774/ping || exit 1

CMD ["/app/komari", "server"]
