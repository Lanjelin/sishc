FROM golang:1.24-alpine AS build

WORKDIR /src

COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal

RUN go build -o /out/sishc ./cmd/sishc

FROM alpine:3.20

LABEL org.opencontainers.image.source=https://github.com/lanjelin/sishc
LABEL org.opencontainers.image.title="SISHC Tunnel Manager"
LABEL org.opencontainers.image.description="Go web UI to manage sish tunnels."
LABEL org.opencontainers.image.author="Lanjelin"
LABEL org.opencontainers.image.licenses=GPL-3.0

ENV SISHC_LOG_DIR="/config/logs"
ENV SISHC_CONFIG_FILE="/config/config.yaml"
ENV SISHC_SOCKET="/config/sishc.sock"
ENV HOME="/config"

RUN apk --no-cache add \
  tini \
  openssh-client \
  ca-certificates

COPY --from=build /out/sishc /usr/local/bin/sishc

WORKDIR /config
VOLUME /config

ENTRYPOINT ["/sbin/tini", "--"]
CMD ["/usr/local/bin/sishc"]
