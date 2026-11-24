FROM hexpm/elixir:1.19.4-erlang-27.3.4.6-alpine-3.22.2 AS builder-deps

WORKDIR /app

RUN apk --no-cache --update add \
    alpine-sdk gmp-dev automake libtool inotify-tools autoconf python3 file gcompat libstdc++ curl ca-certificates git make bash

# Cache elixir deps
COPY services/explorer/backend/mix.exs services/explorer/backend/mix.lock ./
COPY services/explorer/backend/apps/block_scout_web/mix.exs ./apps/block_scout_web/
COPY services/explorer/backend/apps/explorer/mix.exs ./apps/explorer/
COPY services/explorer/backend/apps/ethereum_jsonrpc/mix.exs ./apps/ethereum_jsonrpc/
COPY services/explorer/backend/apps/indexer/mix.exs ./apps/indexer/
COPY services/explorer/backend/apps/utils/mix.exs ./apps/utils/
COPY services/explorer/backend/apps/nft_media_handler/mix.exs ./apps/nft_media_handler/

ENV MIX_ENV="prod"
ENV MIX_HOME=/opt/mix
RUN mix local.hex --force
RUN MIX_OS_DEPS_COMPILE_PARTITION_COUNT=$(nproc) mix do deps.get + local.rebar --force + deps.compile --skip-umbrella-children

COPY services/explorer/backend/config ./config
COPY services/explorer/backend/rel ./rel
COPY services/explorer/backend/apps ./apps

##############################################################
FROM builder-deps AS builder

ENV DISABLE_WEBAPP=true
ENV ADMIN_PANEL_ENABLED=false
ARG DISABLE_API=false
ENV DISABLE_API=${DISABLE_API}
ARG API_V1_READ_METHODS_DISABLED=false
ENV API_V1_READ_METHODS_DISABLED=${API_V1_READ_METHODS_DISABLED}
ARG API_V1_WRITE_METHODS_DISABLED=false
ENV API_V1_WRITE_METHODS_DISABLED=${API_V1_WRITE_METHODS_DISABLED}
ARG CHAIN_TYPE=paxeer_x
ENV CHAIN_TYPE=${CHAIN_TYPE}
ARG BRIDGED_TOKENS_ENABLED=false
ENV BRIDGED_TOKENS_ENABLED=${BRIDGED_TOKENS_ENABLED}
ARG API_GRAPHQL_MAX_COMPLEXITY=2000
ENV API_GRAPHQL_MAX_COMPLEXITY=${API_GRAPHQL_MAX_COMPLEXITY}
ARG MUD_INDEXER_ENABLED=false
ENV MUD_INDEXER_ENABLED=${MUD_INDEXER_ENABLED}

# Run backend compilation
RUN mix compile

RUN mkdir -p /opt/release && \
    mix release blockscout && \
    mv _build/${MIX_ENV}/rel/blockscout /opt/release

##############################################################
FROM hexpm/elixir:1.19.4-erlang-27.3.4.6-alpine-3.22.2

WORKDIR /app

ARG BLOCKSCOUT_USER=blockscout
ARG BLOCKSCOUT_GROUP=blockscout
ARG BLOCKSCOUT_UID=10001
ARG BLOCKSCOUT_GID=10001

RUN apk --no-cache --update add jq curl && \
    addgroup --system --gid ${BLOCKSCOUT_GID} ${BLOCKSCOUT_GROUP} && \
    adduser --system --uid ${BLOCKSCOUT_UID} --ingroup ${BLOCKSCOUT_GROUP} --disabled-password --home /app ${BLOCKSCOUT_USER}

ENV DISABLE_WEBAPP=true
ENV ADMIN_PANEL_ENABLED=false
ARG DISABLE_API=false
ENV DISABLE_API=${DISABLE_API}
ARG API_V1_READ_METHODS_DISABLED=false
ENV API_V1_READ_METHODS_DISABLED=${API_V1_READ_METHODS_DISABLED}
ARG API_V1_WRITE_METHODS_DISABLED=false
ENV API_V1_WRITE_METHODS_DISABLED=${API_V1_WRITE_METHODS_DISABLED}
ARG CHAIN_TYPE=paxeer_x
ENV CHAIN_TYPE=${CHAIN_TYPE}
ARG BRIDGED_TOKENS_ENABLED=false
ENV BRIDGED_TOKENS_ENABLED=${BRIDGED_TOKENS_ENABLED}
ARG API_GRAPHQL_MAX_COMPLEXITY=2000
ENV API_GRAPHQL_MAX_COMPLEXITY=${API_GRAPHQL_MAX_COMPLEXITY}
ARG MUD_INDEXER_ENABLED=false
ENV MUD_INDEXER_ENABLED=${MUD_INDEXER_ENABLED}

ARG RELEASE_VERSION=10.2.6
ENV RELEASE_VERSION=${RELEASE_VERSION}
ARG BLOCKSCOUT_VERSION=v10.2.6
ENV BLOCKSCOUT_VERSION=${BLOCKSCOUT_VERSION}

COPY --from=builder --chown=${BLOCKSCOUT_USER}:${BLOCKSCOUT_GROUP} /opt/release/blockscout .
COPY --from=builder --chown=${BLOCKSCOUT_USER}:${BLOCKSCOUT_GROUP} /app/config/config_helper.exs ./config/config_helper.exs
COPY --from=builder --chown=${BLOCKSCOUT_USER}:${BLOCKSCOUT_GROUP} /app/config/config_helper.exs /app/releases/${RELEASE_VERSION}/config_helper.exs
COPY --from=builder --chown=${BLOCKSCOUT_USER}:${BLOCKSCOUT_GROUP} /app/config/assets/precompiles-arbitrum.json ./config/assets/precompiles-arbitrum.json

RUN mkdir dets && mkdir temp && chown -R ${BLOCKSCOUT_USER}:${BLOCKSCOUT_GROUP} /app

USER ${BLOCKSCOUT_USER}:${BLOCKSCOUT_GROUP}

CMD ["/bin/sh", "-c", "bin/blockscout eval 'Elixir.Explorer.ReleaseTasks.create_and_migrate()' && bin/blockscout start"]
