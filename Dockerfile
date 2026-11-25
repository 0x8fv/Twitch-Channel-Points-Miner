# Build: docker buildx build --platform linux/amd64,linux/arm64 -t tcpm:latest .

# Two-stage build: a compile stage and a minimal runner stage.
# The miner is a static Go binary, so the final image needs only the binary,
# CA certificates and a writable data directory.

# Build stage. It runs on the build platform's native architecture and
# cross-compiles for the target, which keeps multi-arch builds fast.
FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS builder

RUN mkdir -p /data/build /data/dist/data_folder/cookies && \
  chmod 700 /data/dist/data_folder /data/dist/data_folder/cookies
WORKDIR /data/build

# ca-certificates are copied into the scratch runner stage below.
# hadolint ignore=DL3018
RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY main.go ./
COPY TwitchChannelPointsMiner/ ./TwitchChannelPointsMiner/

ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
  go build -trimpath -buildvcs=false -ldflags="-s -w" \
  -o /data/dist/TwitchChannelPointsMiner .

# Runner stage; completely blank -> extremely small & lightweight image.
# scratch has no shell, package manager or user database, so the runtime user
# is a numeric UID/GID and every persistent path must be copied in.
FROM scratch AS runner

# copy the built binary
COPY --from=builder /data/dist/TwitchChannelPointsMiner \
  /app/TwitchChannelPointsMiner
# copy the trusted CA certificates (not installable via apt in a scratch image)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt \
  /etc/ssl/certs/
# prepare the data folder for the non-root runtime user (scratch cannot mkdir)
COPY --from=builder --chown=10001:10001 \
  /data/dist/data_folder /app/data

WORKDIR /app
# The image is the unit of update, so the in-container updater stays off.
ENV TCPM_DATA_DIR=/app/data \
  TCPM_AUTO_UPDATE=false

USER 10001:10001
ENTRYPOINT [ "/app/TwitchChannelPointsMiner" ]
