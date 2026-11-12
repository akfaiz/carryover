# syntax=docker/dockerfile:1
#
# Builds the Linux (Steam Deck / desktop Linux) version of Carryover and
# packages it into an image that can display its window on a Windows host
# through WSLg. Use scripts/linux-docker.ps1 rather than calling this directly.

FROM golang:1.26-bookworm AS build
RUN apt-get update && apt-get install -y --no-install-recommends \
        gcc libgl1-mesa-dev xorg-dev libxkbcommon-dev libwayland-dev wayland-protocols \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o /out/carryover .

# AppImage packaging (see packaging/appimage/build.sh). Built on demand:
#   docker build --target appimage-out --output type=local,dest=bin .
FROM build AS appimage
ARG APPIMAGETOOL_VERSION=1.9.1
RUN apt-get update \
    && apt-get install -y --no-install-recommends curl ca-certificates file desktop-file-utils zsync appstream \
    && rm -rf /var/lib/apt/lists/* \
    && curl -fsSL -o /usr/local/bin/appimagetool \
        https://github.com/AppImage/appimagetool/releases/download/${APPIMAGETOOL_VERSION}/appimagetool-x86_64.AppImage \
    && dd if=/dev/zero of=/usr/local/bin/appimagetool bs=1 count=3 seek=8 conv=notrunc \
    && chmod +x /usr/local/bin/appimagetool
# The dd above zeroes the AppImage magic ("AI\x02", ELF header bytes 8-10): QEMU's binfmt
# handler rejects non-zero padding, so appimagetool fails with "Exec format error" when this
# builds under emulation (arm64 host). Harmless on a native x86-64 host.
# No FUSE inside a build container: let appimagetool unpack itself.
ENV APPIMAGE_EXTRACT_AND_RUN=1
# Update information for AppImageUpdate / Gear Lever (empty for local builds);
# when set, <APPIMAGE>.zsync is written next to the AppImage. Releases set
# APPIMAGE to the versioned name: the .zsync file refers to the AppImage by name.
ARG UPDATE_INFO=""
ARG APPIMAGE=carryover-linux-amd64.AppImage
RUN UPDATE_INFO="$UPDATE_INFO" OUT="/appimage/$APPIMAGE" bash packaging/appimage/build.sh /out/carryover

FROM scratch AS appimage-out
COPY --from=appimage /appimage/ /

# The bare binary, for the release tarball:
#   docker build --target binary-out --output type=local,dest=bin .
FROM scratch AS binary-out
COPY --from=build /out/carryover /carryover

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
        libgl1 libgl1-mesa-dri libegl1 libx11-6 libxcursor1 libxrandr2 libxinerama1 libxi6 libxxf86vm1 \
        libxkbcommon0 libwayland-client0 libwayland-cursor0 libwayland-egl1 fontconfig fonts-dejavu-core ca-certificates xdg-utils \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/carryover /usr/local/bin/carryover
# Keep settings, library, rclone.conf and state in a volume, away from any real config.
ENV CARRYOVER_HOME=/data
VOLUME /data
ENTRYPOINT ["carryover"]
