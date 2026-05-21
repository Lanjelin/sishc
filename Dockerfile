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

ARG SISHC_USER=sishc
ARG SISHC_UID=1000
ARG SISHC_GID=1000

ENV SISHC_LOG_DIR="/config/logs"
ENV SISHC_CONFIG_FILE="/config/config.yaml"
ENV SISHC_SOCKET="/config/sishc.sock"
ENV SISHC_KNOWN_HOSTS="/config/.ssh/known_hosts"
ENV HOME="/config"

RUN apk --no-cache add \
  tini \
  openssh-client \
  ca-certificates

RUN addgroup -S -g "${SISHC_GID}" "${SISHC_USER}" \
  && adduser -S -D -u "${SISHC_UID}" -G "${SISHC_USER}" -h /config "${SISHC_USER}" \
  && mkdir -p /config \
  && mkdir -p /config/.ssh \
  && chown -R "${SISHC_USER}:${SISHC_USER}" /config

COPY --from=build /out/sishc /usr/local/bin/sishc

WORKDIR /config
VOLUME /config

USER ${SISHC_USER}

ENTRYPOINT ["/sbin/tini", "--"]
CMD ["/usr/local/bin/sishc"]
