FROM alpine:3.20

LABEL org.opencontainers.image.source=https://github.com/lanjelin/sishc
LABEL org.opencontainers.image.title="SISHC Tunnel Manager"
LABEL org.opencontainers.image.description="Bash script and WEB GUI to manage sish tunnels."
LABEL org.opencontainers.image.author="Lanjelin"
LABEL org.opencontainers.image.licenses=GPL-3.0

ENV SISHC_OUTPUT_LOG="/config/sishc.log"
ENV SISHC_CONFIG_FILE="/config/config.yaml"
ENV HOME="/config"
ENV PUID=1000
ENV PGID=1000

COPY . .

RUN \
  apk --no-cache add --update \
    tini \
    wget \
    bash \
    tzdata \
    shadow \
    openssh \
    su-exec \
    autossh \
    python3 \
    py3-pip \
    py3-yaml \
    py3-flask && \
  wget https://github.com/mikefarah/yq/releases/latest/download/yq_linux_amd64 -O /usr/bin/yq && \
    chmod +x /usr/bin/yq && \
  apk del wget && \
  adduser -s /bin/bash -D --home /config -u $PUID -g $PGID abc

WORKDIR /config
VOLUME /config

ENTRYPOINT ["/sbin/tini", "--"]
CMD ["/bin/bash", "/entrypoint.sh"]
