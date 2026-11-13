# Multi-arch image for the tank (linux/amd64, linux/arm64, linux/arm/v7).
#
#   docker buildx build --platform linux/arm64,linux/arm/v7,linux/amd64 -t ghcr.io/ttbakiatwoam/asciifishtv-tank .
#
# To draw on a Raspberry Pi's HDMI output the container needs the
# framebuffer and sound devices, e.g.:
#
#   docker run --device /dev/fb0 --device /dev/snd -v /home/pi/music:/music:ro \
#     -e TANK_MUSIC_DIR=/music -e TANK_TOKEN=... -p 8080:8080 ghcr.io/ttbakiatwoam/asciifishtv-tank
FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS build
ARG TARGETOS TARGETARCH TARGETVARIANT
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} \
    go build -trimpath -ldflags="-s -w" -o /out/tank ./cmd/tank

FROM debian:bookworm-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends mpg123 \
 && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/tank /usr/local/bin/tank
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/tank"]
