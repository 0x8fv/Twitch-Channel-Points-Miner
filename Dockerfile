# Build: docker buildx build . -t tcpm:latest

# 2 stage Dockerfile; a compile stage and a runner stage.
# this works because the current repo produces a static binary that doesnt
# need any libraries & isn't dependent on any native system toolsy

# heavy builder stage; this stage can be bloated, its only needed for the 
FROM debian:trixie-20261005 AS builder

RUN mkdir -p /data/build /data/dist
WORKDIR /data/build

# prepare data dir for scratch container with correct permissions
RUN mkdir /data/dist/data_folder && \
  chown 10001:10001 /data/dist/data_folder && \
  chmod 600 /data/dist/data_folder

# install build dependencies & updating CA certificates
RUN apt update && \
  apt upgrade -y && \
  apt install -y --no-install-recommends \
  golang \
  ca-certificates && \
  update-ca-certificates && \
  rm -rf /var/lib/apt/lists/*

COPY ./go.mod ./go.sum ./main.go /data/build/
COPY ./TwitchChannelPointsMiner/ /data/build/TwitchChannelPointsMiner

RUN CGO_ENABLED=0 go build \
  -trimpath -buildvcs=false -ldflags="-s -w" \
  -o "/data/dist/TwitchChannelPointsMiner" \
  "/data/build"


# runner stage; completely blank -> extremely small & lightweight docker image!
FROM scratch AS runner 

# copy the built binary
COPY --from=builder /data/dist/TwitchChannelPointsMiner \
  /app/TwitchChannelPointsMiner
# copy the trusted CA certificates (don't exist & not installable via apt in scratch image)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt \
  /etc/ssl/certs/
# copy data folder with correct permissions (we cant create due to scratch image)
COPY --from=builder --chmod=2700 \
  /data/dist/data_folder /app/data

WORKDIR /app
ENV TCPM_DATA_DIR=/app/data

# starting the binary
ENTRYPOINT [ "/app/TwitchChannelPointsMiner" ]