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

ENV HOME="/config"
ENV SISHC_LOG_DIR="${HOME}/logs"
ENV SISHC_CONFIG_FILE="${HOME}/config.yaml"
ENV SISHC_SOCKET="${HOME}/sishc.sock"
ENV SISHC_KNOWN_HOSTS="${HOME}/.ssh/known_hosts"

RUN apk --no-cache add \
  tini \
  openssh-client \
  ca-certificates

RUN addgroup -S -g "${SISHC_GID}" "${SISHC_USER}" \
  && adduser -S -D -u "${SISHC_UID}" -G "${SISHC_USER}" -h "${HOME}" "${SISHC_USER}" \
  && mkdir -p "${HOME}" \
  && mkdir -p "${HOME}/.ssh" \
  && chown -R "${SISHC_USER}:${SISHC_USER}" "${HOME}"

COPY --from=build /out/sishc /usr/local/bin/sishc

WORKDIR "${HOME}"
VOLUME "${HOME}"

USER ${SISHC_USER}

ENTRYPOINT ["/sbin/tini", "--"]
CMD ["/usr/local/bin/sishc"]
