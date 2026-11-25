# syntax=docker/dockerfile:1

# The binary is built on the runner's own platform and cross-compiled (pure Go,
# no cgo), and the final image only copies files. Nothing runs under emulation,
# so a multi-arch build needs no QEMU.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS TARGETARCH
# timetzdata embeds the time zone database, so the image needs no tzdata package.
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -buildvcs=false -tags timetzdata -ldflags "-s -w" -o /out/twitch-miner ./cmd/twitch-miner \
    && mkdir /out/data

FROM alpine:3.20
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build --chown=1000:1000 /out/data /data
COPY --from=build /out/twitch-miner /usr/local/bin/twitch-miner
# The same user id earlier images ran as, so existing volumes keep working.
USER 1000:1000
WORKDIR /data
# config.json, cookies/, log/ and analytics/ live in /data.
# The image is the unit of update, so the self-updater is forced off.
ENV TCPM_DATA_DIR=/data \
    TCPM_DISABLE_AUTO_UPDATE=1 \
    TCPM_ANALYTICS_ADDR=0.0.0.0:5000
VOLUME /data
EXPOSE 5000
ENTRYPOINT ["/usr/local/bin/twitch-miner"]
