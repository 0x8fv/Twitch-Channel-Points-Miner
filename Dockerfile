# syntax=docker/dockerfile:1

# Build on the runner's native platform and cross-compile (pure Go, no cgo).
FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -buildvcs=false -ldflags "-s -w" -o /out/twitch-miner ./cmd/twitch-miner

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 1000 miner \
    && mkdir /data && chown miner:miner /data
COPY --from=build /out/twitch-miner /usr/local/bin/twitch-miner
USER miner
WORKDIR /data
# config.json, cookies/, log/ and analytics/ live in /data.
# The image is the unit of update, so the self-updater is forced off.
ENV TCPM_DATA_DIR=/data \
    TCPM_DISABLE_AUTO_UPDATE=1 \
    TCPM_ANALYTICS_ADDR=0.0.0.0:5000
VOLUME /data
EXPOSE 5000
ENTRYPOINT ["/usr/local/bin/twitch-miner"]
