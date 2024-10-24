FROM alpine:3.20

ENV SISHC_OUTPUT_LOG="/dev/null"
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
    autossh && \
  wget https://github.com/mikefarah/yq/releases/latest/download/yq_linux_amd64 -O /usr/bin/yq && \
    chmod +x /usr/bin/yq && \
  apk del wget && \
  adduser -s /bin/bash -D --home /config -u $PUID -g $PGID abc

WORKDIR /config
VOLUME /config

ENTRYPOINT ["/sbin/tini", "--"]
CMD ["/bin/bash", "/entrypoint.sh", "/sishc.sh"]
