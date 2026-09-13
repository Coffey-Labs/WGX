# ---- web build ----
FROM node:26-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --ignore-scripts --no-audit --no-fund
COPY web/ ./
RUN npm run build

# ---- go build ----
FROM golang:1.27-alpine AS build
# The version string the binary reports. Worked out by whoever runs the
# build (CI passes the tag); left empty it says "dev".
ARG WGX_VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY --from=web /src/internal/server/static/dist internal/server/static/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X github.com/Coffey-Labs/WGX/internal/engine.Version=${WGX_VERSION}" -o /wgx ./cmd/wgx

# ---- runtime ----
FROM alpine:3.22
# nftables does the NAT; wireguard-go is the fallback data plane for hosts
# without the kernel module; wireguard-tools gives `wg show` for debugging.
RUN apk add --no-cache nftables wireguard-go wireguard-tools ca-certificates tzdata \
    && mkdir -p /data
COPY --from=build /wgx /usr/local/bin/wgx
ENV WGX_DATA_DIR=/data \
    WGX_HTTP_LISTEN=:51821
VOLUME ["/data"]
EXPOSE 51820/udp 51821/tcp
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s \
    CMD wget -qO- http://127.0.0.1:51821/api/health || exit 1
ENTRYPOINT ["wgx"]
