# Web assets are platform-independent. Build them on the native runner once
# instead of rebuilding them under QEMU for every target image.
FROM --platform=$BUILDPLATFORM node:22-alpine AS web-builder

WORKDIR /src/web
COPY web/ ./
RUN npm ci && npm run build

# Agent artifacts are cross-compiled explicitly below and are also platform-
# independent, so keep this stage on the native build platform.
FROM --platform=$BUILDPLATFORM golang:1.25-bookworm AS agent-builder

WORKDIR /src/agent
ARG KOMARI_VERSION
RUN test -n "$KOMARI_VERSION"
COPY agent/go.mod agent/go.sum ./
RUN go mod download
COPY agent/ ./

RUN set -eux; \
    mkdir -p /out; \
    VERSION="${KOMARI_VERSION}"; \
    printf '%s' "$VERSION" > /out/version; \
    for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do \
      os="${target%/*}"; arch="${target#*/}"; ext=""; \
      if [ "$os" = "windows" ]; then ext=".exe"; fi; \
      CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath \
        -ldflags="-s -w -X github.com/komari-monitor/komari-agent/update.CurrentVersion=$VERSION" \
        -o "/out/komari-agent-$os-$arch$ext" .; \
    done; \
    cp install.sh install.ps1 /out/

FROM golang:1.25-bookworm AS server-builder

ARG KOMARI_VERSION
ARG KOMARI_COMMIT=unknown
RUN test -n "$KOMARI_VERSION"
RUN apt-get update \
    && apt-get install -y --no-install-recommends zstd \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src/server
COPY server/go.mod server/go.sum ./
RUN go mod download

COPY server/ ./
COPY --from=web-builder /src/web/dist/ /src/web-dist/
COPY --from=web-builder /src/web/komari-theme.json web/public/defaultTheme/komari-theme.json

RUN mkdir -p web/public/defaultTheme \
    && tar -cf /tmp/komari-web.tar -C /src/web-dist . \
    && zstd -6 -T0 -f /tmp/komari-web.tar -o web/public/defaultTheme/dist.tar.zst \
    && CGO_ENABLED=1 go build -trimpath -ldflags="-s -w -X github.com/komari-monitor/komari/utils.CurrentVersion=${KOMARI_VERSION} -X github.com/komari-monitor/komari/utils.VersionHash=${KOMARI_COMMIT}" -o /out/komari .

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
COPY --from=agent-builder --chown=komari:komari /out/ /app/agent-dist/

ENV GIN_MODE=release
ENV KOMARI_LISTEN=0.0.0.0:25774

USER komari
EXPOSE 25774
VOLUME ["/app/data"]

HEALTHCHECK --interval=15s --timeout=5s --start-period=20s --retries=5 \
    CMD curl -fsS http://127.0.0.1:25774/ping || exit 1

CMD ["/app/komari", "server"]
