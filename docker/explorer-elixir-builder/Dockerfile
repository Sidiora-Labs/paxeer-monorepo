# Builder image for every mix command run against services/explorer/backend.
#
# mix-in-builder.sh builds this on demand when its image reference is absent,
# and the explorer build workflow compiles the backend on the same Elixir and
# Erlang versions, so the toolchain moves in one place.
#
# Build context: the repository root
#   docker build --file docker/explorer-elixir-builder/Dockerfile \
#     --tag explorer-elixir-builder:v10.2.6 .
#
# The image carries the dependency sources and the build toolchain, not the
# compiled artefacts: mix-in-builder.sh points MIX_BUILD_PATH at a named volume
# so a rerun reuses the compilation of the environment it runs in. The umbrella
# sources are mounted over this layout at run time, which is why only the mix
# manifests are copied in.

FROM node:22.16.0-alpine3.22 AS node
FROM rust:1.85.1-alpine3.21 AS rust
FROM debian:bookworm-slim AS glibc

FROM hexpm/elixir:1.19.4-erlang-27.3.4.6-alpine-3.22.2

WORKDIR /app

RUN apk --no-cache --update add \
    alpine-sdk gmp-dev automake libtool inotify-tools autoconf python3 \
    file gcompat libstdc++ curl ca-certificates git make bash postgresql-client py3-pip chromium-chromedriver

COPY --from=node /usr/local/bin/node /usr/local/bin/node
COPY --from=node /usr/local/lib/node_modules /usr/local/lib/node_modules
COPY --from=rust /usr/local/cargo /usr/local/cargo
COPY --from=rust /usr/local/rustup /usr/local/rustup
COPY --from=glibc /lib/x86_64-linux-gnu /opt/glibc/lib
COPY --from=glibc /lib64/ld-linux-x86-64.so.2 /opt/glibc/ld-linux-x86-64.so.2

ENV MIX_HOME=/opt/mix \
    CARGO_HOME=/usr/local/cargo \
    RUSTUP_HOME=/usr/local/rustup \
    NODE_PATH=/opt/solc/node_modules \
    PATH=/usr/local/cargo/bin:/opt/vyper/bin:$PATH

RUN ln -s ../lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm && \
    ln -s ../lib/node_modules/npm/bin/npx-cli.js /usr/local/bin/npx && \
    npm install --prefix /opt/solc --no-audit --no-fund --save-exact solc@0.8.17 && \
    npm install --prefix /opt/solc-0.4.24 --no-audit --no-fund --save-exact solc@0.4.24 && \
    python3 -m venv /opt/vyper && \
    /opt/vyper/bin/pip install --no-cache-dir vyper==0.4.3 && \
    mkdir -p /lib/x86_64-linux-gnu && \
    cp -a /opt/glibc/lib/. /lib/x86_64-linux-gnu/ && \
    rm -f /lib64/ld-linux-x86-64.so.2 /lib/ld-linux-x86-64.so.2 && \
    cp /opt/glibc/ld-linux-x86-64.so.2 /lib/ld-linux-x86-64.so.2 && \
    ln -s /lib/ld-linux-x86-64.so.2 /lib64/ld-linux-x86-64.so.2

COPY services/explorer/backend/mix.exs services/explorer/backend/mix.lock ./
COPY services/explorer/backend/apps/block_scout_web/mix.exs ./apps/block_scout_web/
COPY services/explorer/backend/apps/explorer/mix.exs ./apps/explorer/
COPY services/explorer/backend/apps/ethereum_jsonrpc/mix.exs ./apps/ethereum_jsonrpc/
COPY services/explorer/backend/apps/indexer/mix.exs ./apps/indexer/
COPY services/explorer/backend/apps/utils/mix.exs ./apps/utils/
COPY services/explorer/backend/apps/nft_media_handler/mix.exs ./apps/nft_media_handler/

RUN mix local.hex --force && mix local.rebar --force
RUN mix deps.get
RUN cd deps/siwe/native/siwe_native && \
    cargo build --locked --release && \
    cp target/release/libsiwe_native.so ../../priv/native/libsiwe_native.so && \
    target=$(rustc -vV | sed -n 's/^host: //p') && \
    cp ../../priv/native/libsiwe_native.so "../../priv/native/libsiwe_native-v0.6.0-nif-2.15-$target.so"

COPY docker/explorer-elixir-builder/build-web-assets.sh /usr/local/bin/build-web-assets

CMD ["/bin/sh"]
