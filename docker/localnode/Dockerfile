FROM node:24.11.0-bookworm-slim@sha256:76d0ed0ed93bed4f4376211e9d8fddac4d8b3fbdb54cc45955696001a3c91152 AS node-dist
FROM golang:1.25.8-bookworm@sha256:4557cf171e3cdf5053a298d5171b1a5f5734d920260c25f22c79e94760eb2084 AS go-dist
FROM docker.io/rust:1.91.1-bookworm@sha256:c1e5f19e773b7878c3f7a805dd00a495e747acbdc76fb2337a4ebf0418896b33 AS rust-dist

FROM ubuntu:24.04@sha256:fe81560990fae86dc4743202b2cb4d3df98d0bcf949173cecef15ce1127ef852
ENV HOME="/root" RUSTUP_HOME=/usr/local/rustup CARGO_HOME=/usr/local/cargo RUSTUP_TOOLCHAIN=1.91.1 PATH="/usr/local/cargo/bin:/root/go/bin:/pax-protocol/pax-chain/tests/integration/upgrade_module/scripts/:$PATH"
RUN set -eux; \
    export DEBIAN_FRONTEND=noninteractive; \
    for attempt in 1 2 3 4 5; do \
      if apt-get -o Acquire::Retries=5 -o APT::Update::Error-Mode=any update && \
         apt-get -o Acquire::Retries=5 install -y --no-install-recommends make build-essential libssl-dev git jq python3 curl ca-certificates vim uuid-runtime; then \
        update-ca-certificates; \
        rm -rf /var/lib/apt/lists/*; \
        break; \
      fi; \
      rm -rf /var/lib/apt/lists/*; \
      if [ "${attempt}" = "5" ]; then \
        exit 1; \
      fi; \
      sleep $((attempt * 10)); \
    done
COPY --from=go-dist /usr/local/go /usr/local/go
COPY --from=rust-dist /usr/local/rustup /usr/local/rustup
COPY --from=rust-dist /usr/local/cargo /usr/local/cargo
RUN chmod -R a+rwX /usr/local/cargo
RUN set -eux; \
    for attempt in 1 2 3 4 5; do \
      if curl -fsSL https://foundry.paradigm.xyz -o /tmp/foundry.sh && \
         bash /tmp/foundry.sh && \
         /root/.foundry/bin/foundryup; then \
        rm -f /tmp/foundry.sh; \
        break; \
      fi; \
      rm -f /tmp/foundry.sh; \
      if [ "${attempt}" -eq 5 ]; then \
        echo "foundry install failed after retries"; \
        exit 1; \
      fi; \
      sleep 2; \
    done
COPY --from=node-dist /usr/local/bin/node /usr/local/bin/node
COPY --from=node-dist /usr/local/lib/node_modules /usr/local/lib/node_modules
RUN ln -s ../lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm \
    && ln -s ../lib/node_modules/npm/bin/npx-cli.js /usr/local/bin/npx
RUN mkdir -p /root/go/pkg/mod && \
    mkdir -p /root/.cache && \
    chmod -R a+rwX /root
SHELL ["/bin/bash", "-c"]

WORKDIR /pax-protocol/pax-chain

EXPOSE 26656 26657 26658 9090 9091 7171

CMD ["/usr/bin/deploy.sh"]

STOPSIGNAL SIGTERM

COPY docker/localnode/scripts/deploy.sh /usr/bin/deploy.sh
COPY docker/localnode/scripts/step0_build.sh /usr/bin/build.sh
COPY docker/localnode/scripts/step1_configure_init.sh /usr/bin/configure_init.sh
COPY docker/localnode/scripts/step2_genesis.sh /usr/bin/genesis.sh
COPY docker/localnode/scripts/step3_add_validator_to_genesis.sh /usr/bin/add_validator_to_gensis.sh
COPY docker/localnode/scripts/step4_config_override.sh /usr/bin/config_override.sh
COPY docker/localnode/scripts/step5_start_pax.sh /usr/bin/start_pax.sh
