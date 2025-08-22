FROM --platform=linux/amd64 alpine

ARG tag

# print build tag for debugging

ENV WORKDIR="/srv/bililive"
ENV OUTPUT_DIR="/srv/bililive" \
    CONF_DIR="/etc/bililive-go" \
    PORT=8080

ENV PUID=0 PGID=0 UMASK=022

RUN mkdir -p $OUTPUT_DIR && \
    mkdir -p $CONF_DIR && \
    apk update && \
    apk --no-cache add ffmpeg libc6-compat curl su-exec tzdata && \
    cp -r -f /usr/share/zoneinfo/Asia/Shanghai /etc/localtime

RUN set -eux; \
    cd /tmp; \
    curl -sSLO https://github.com/bulai0408/bililive-go/releases/download/${tag}/bililive-linux-amd64.tar.gz; \
    tar -zxvf bililive-linux-amd64.tar.gz || true; \
    BIN_PATH=""; \
    if [ -f bililive-linux-amd64 ]; then BIN_PATH=bililive-linux-amd64; fi; \
    if [ -z "$BIN_PATH" ]; then BIN_PATH=$(ls -1 bililive* 2>/dev/null | head -1 || true); fi; \
    if [ -z "$BIN_PATH" ]; then echo "binary not found in archive"; exit 2; fi; \
    chmod +x "$BIN_PATH"; \
    mv "$BIN_PATH" /usr/bin/bililive-go; \
    rm -f bililive-linux-amd64.tar.gz; \
    /usr/bin/bililive-go --version || true

COPY config.docker.yml $CONF_DIR/config.yml

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

VOLUME $OUTPUT_DIR

EXPOSE $PORT

WORKDIR ${WORKDIR}
ENTRYPOINT [ "sh" ]
CMD [ "/entrypoint.sh" ]
