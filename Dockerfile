FROM golang:1.24-alpine AS build

WORKDIR /src

COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal

RUN go build -o /out/sishc ./cmd/sishc

FROM alpine:3.20

LABEL org.opencontainers.image.source=https://github.com/lanjelin/sishc
LABEL org.opencontainers.image.title="SISHC Tunnel Manager"
LABEL org.opencontainers.image.description="Go daemon and web UI to manage sish tunnels."
LABEL org.opencontainers.image.author="Lanjelin"
LABEL org.opencontainers.image.licenses=GPL-3.0

ENV SISHC_USER="sishc"
ENV PUID="1000"
ENV PGID="1000"

ENV HOME="/config"
ENV SISHC_LOG_DIR="${HOME}/logs"
ENV SISHC_CONFIG_FILE="${HOME}/config.yaml"
ENV SISHC_SOCKET="${HOME}/sishc.sock"
ENV SISHC_KNOWN_HOSTS="${HOME}/.ssh/known_hosts"

RUN apk --no-cache add \
  tini \
  openssh-client \
  su-exec \
  ca-certificates

COPY --from=build /out/sishc /usr/local/bin/sishc
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

WORKDIR "${HOME}"
VOLUME "${HOME}"

ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/docker-entrypoint.sh"]
CMD ["/usr/local/bin/sishc"]
