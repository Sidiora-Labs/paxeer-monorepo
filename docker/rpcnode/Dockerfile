FROM golang:1.25.8-bookworm AS go-dist

FROM ubuntu:latest
ENV HOME="/home/pax" GOPATH="/home/pax/go" GOCACHE="/home/pax/.cache/go-build" PATH="/home/pax/go/bin:/usr/local/go/bin:$PATH"
RUN apt-get update && \
    apt-get install -y make git build-essential jq python3 curl vim uuid-runtime
COPY --from=go-dist /usr/local/go /usr/local/go
RUN groupadd --gid 10001 pax && \
    useradd --uid 10001 --gid pax --home-dir /home/pax --shell /usr/sbin/nologin --no-create-home pax && \
    install -d -o pax -g pax -m 0700 \
        /home/pax /home/pax/.pax /home/pax/.pax/config /home/pax/.pax/data \
        /home/pax/go /home/pax/go/bin /home/pax/go/pkg /home/pax/go/pkg/mod \
        /home/pax/.cache /home/pax/.cache/go-build && \
    install -d -o root -g root -m 0755 /pax-protocol /pax-protocol/pax-chain && \
    install -d -o pax -g pax -m 0700 \
        /pax-protocol/pax-chain/build /pax-protocol/pax-chain/build/generated \
        /pax-protocol/pax-chain/build/generated/logs
SHELL ["/bin/bash", "-c"]

WORKDIR /pax-protocol/pax-chain

EXPOSE 26656 26657 26658 9090 9091

CMD ["/usr/bin/deploy.sh"]

STOPSIGNAL SIGTERM

COPY --chmod=0555 docker/rpcnode/scripts/deploy.sh /usr/bin/deploy.sh
COPY --chmod=0555 docker/rpcnode/scripts/step0_build.sh /usr/bin/build.sh
COPY --chmod=0555 docker/rpcnode/scripts/step1_configure_init.sh /usr/bin/configure_init.sh
COPY --chmod=0555 docker/rpcnode/scripts/step2_start_pax.sh /usr/bin/start_pax.sh

USER pax:pax
