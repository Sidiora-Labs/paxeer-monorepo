# syntax=docker/dockerfile:1.7
ARG NODE_IMAGE=node@sha256:ebfe2f90462722a7a4de65e91990e97fe0d401c70e0e762c5b53302f905ec1c1
ARG CHARTING_IMAGE=ghcr.io/sidiora-labs/hyperpax-charting-library@sha256:4a805ca00668766af6085043670140a8f215ccbd29c169e1bc4978d2984bad74

FROM ${CHARTING_IMAGE} AS charting

FROM ${NODE_IMAGE} AS deps
ARG NODE_IMAGE
ARG CHARTING_IMAGE
RUN set -eu; \
    case "${NODE_IMAGE}" in *@sha256:????????????????????????????????????????????????????????????????) ;; *) echo "NODE_IMAGE must be pinned by digest" >&2; exit 1 ;; esac; \
    case "${CHARTING_IMAGE}" in *@sha256:????????????????????????????????????????????????????????????????) ;; *) echo "CHARTING_IMAGE must be pinned by digest" >&2; exit 1 ;; esac
WORKDIR /app
RUN apk add --no-cache libc6-compat && corepack enable
COPY package.json pnpm-lock.yaml ./
COPY pnpm-workspace.yaml .npmrc ./
COPY tsconfig.json ./
COPY apps ./apps
COPY packages ./packages
RUN corepack prepare "$(node -p 'require("./package.json").packageManager')" --activate
RUN pnpm install --frozen-lockfile

FROM ${NODE_IMAGE} AS builder
WORKDIR /app
COPY --from=deps /app ./
RUN apk add --no-cache libc6-compat && corepack enable && corepack prepare "$(node -p 'require("./package.json").packageManager')" --activate
COPY --from=charting /charting_library apps/web/public/static/charting_library
COPY --from=charting /datafeeds apps/web/public/static/datafeeds
RUN test -f apps/web/public/static/charting_library/charting_library.standalone.js || { echo "charting_library.standalone.js missing from CHARTING_IMAGE" >&2; exit 1; }
ENV NEXT_TELEMETRY_DISABLED=1 NODE_ENV=production PAXEER_NETWORK_BUILD_DIR=.next-production
ARG NEXT_PUBLIC_AUTH_REDIRECT_URL=
ARG NEXT_PUBLIC_DIRECT_PRICE_FEED_ORIGINS=
ARG NEXT_PUBLIC_DISABLE_REALTIME_FOR_RENDER=
ARG NEXT_PUBLIC_LAUNCHPAD_PRECOMPILE=
ARG NEXT_PUBLIC_LOG_LEVEL=
ARG NEXT_PUBLIC_PAXEER_RPC_URL=
ARG NEXT_PUBLIC_PAXEER_RPC_URLS=
ARG NEXT_PUBLIC_PAXEER_WALLET_API=
ARG NEXT_PUBLIC_PAXSCAN_API_URL=
ARG NEXT_PUBLIC_PAXSCAN_BASE_URL=
ARG NEXT_PUBLIC_PAXSCAN_WS_URL=
ARG NEXT_PUBLIC_PECOR_MARKET_PRECOMPILE=
ARG NEXT_PUBLIC_PECOR_ORDERS_PRECOMPILE=
ARG NEXT_PUBLIC_PECOR_QUOTER_PRECOMPILE=
ARG NEXT_PUBLIC_PECOR_STOP_PRECOMPILE=
ARG NEXT_PUBLIC_PECOR_VAULT_PRECOMPILE=
ARG NEXT_PUBLIC_PERPS_GATEWAY_URL=
ARG NEXT_PUBLIC_PERPS_PRECOMPILE=
ARG NEXT_PUBLIC_RELEASE=
ARG NEXT_PUBLIC_RELEASE_SPOT_OPEN=
ARG NEXT_PUBLIC_SENTRY_DSN=
ARG NEXT_PUBLIC_SENTRY_ENV=
ARG NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY=
ARG NEXT_PUBLIC_SUPABASE_URL=
ARG NEXT_PUBLIC_WALLET_IFRAME_ORIGIN=
ENV NEXT_PUBLIC_AUTH_REDIRECT_URL=$NEXT_PUBLIC_AUTH_REDIRECT_URL \
    NEXT_PUBLIC_DIRECT_PRICE_FEED_ORIGINS=$NEXT_PUBLIC_DIRECT_PRICE_FEED_ORIGINS \
    NEXT_PUBLIC_DISABLE_REALTIME_FOR_RENDER=$NEXT_PUBLIC_DISABLE_REALTIME_FOR_RENDER \
    NEXT_PUBLIC_LAUNCHPAD_PRECOMPILE=$NEXT_PUBLIC_LAUNCHPAD_PRECOMPILE \
    NEXT_PUBLIC_LOG_LEVEL=$NEXT_PUBLIC_LOG_LEVEL \
    NEXT_PUBLIC_PAXEER_RPC_URL=$NEXT_PUBLIC_PAXEER_RPC_URL \
    NEXT_PUBLIC_PAXEER_RPC_URLS=$NEXT_PUBLIC_PAXEER_RPC_URLS \
    NEXT_PUBLIC_PAXEER_WALLET_API=$NEXT_PUBLIC_PAXEER_WALLET_API \
    NEXT_PUBLIC_PAXSCAN_API_URL=$NEXT_PUBLIC_PAXSCAN_API_URL \
    NEXT_PUBLIC_PAXSCAN_BASE_URL=$NEXT_PUBLIC_PAXSCAN_BASE_URL \
    NEXT_PUBLIC_PAXSCAN_WS_URL=$NEXT_PUBLIC_PAXSCAN_WS_URL \
    NEXT_PUBLIC_PECOR_MARKET_PRECOMPILE=$NEXT_PUBLIC_PECOR_MARKET_PRECOMPILE \
    NEXT_PUBLIC_PECOR_ORDERS_PRECOMPILE=$NEXT_PUBLIC_PECOR_ORDERS_PRECOMPILE \
    NEXT_PUBLIC_PECOR_QUOTER_PRECOMPILE=$NEXT_PUBLIC_PECOR_QUOTER_PRECOMPILE \
    NEXT_PUBLIC_PECOR_STOP_PRECOMPILE=$NEXT_PUBLIC_PECOR_STOP_PRECOMPILE \
    NEXT_PUBLIC_PECOR_VAULT_PRECOMPILE=$NEXT_PUBLIC_PECOR_VAULT_PRECOMPILE \
    NEXT_PUBLIC_PERPS_GATEWAY_URL=$NEXT_PUBLIC_PERPS_GATEWAY_URL \
    NEXT_PUBLIC_PERPS_PRECOMPILE=$NEXT_PUBLIC_PERPS_PRECOMPILE \
    NEXT_PUBLIC_RELEASE=$NEXT_PUBLIC_RELEASE \
    NEXT_PUBLIC_RELEASE_SPOT_OPEN=$NEXT_PUBLIC_RELEASE_SPOT_OPEN \
    NEXT_PUBLIC_SENTRY_DSN=$NEXT_PUBLIC_SENTRY_DSN \
    NEXT_PUBLIC_SENTRY_ENV=$NEXT_PUBLIC_SENTRY_ENV \
    NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY=$NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY \
    NEXT_PUBLIC_SUPABASE_URL=$NEXT_PUBLIC_SUPABASE_URL \
    NEXT_PUBLIC_WALLET_IFRAME_ORIGIN=$NEXT_PUBLIC_WALLET_IFRAME_ORIGIN
RUN for n in $(env | sed -n 's/^\(NEXT_PUBLIC_[A-Za-z0-9_]*\)=.*/\1/p'); do eval "v=\${$n}"; [ -n "$v" ] || unset "$n"; done; pnpm build

FROM ${NODE_IMAGE} AS runner
WORKDIR /app
RUN apk add --no-cache libc6-compat tini && addgroup -g 1001 -S nodejs && adduser -S -u 1001 -G nodejs nextjs
ENV NODE_ENV=production NEXT_TELEMETRY_DISABLED=1 PORT=3000 HOSTNAME=0.0.0.0
COPY --from=builder --chown=nextjs:nodejs /app/apps/web/public ./apps/web/public
COPY --from=builder --chown=nextjs:nodejs /app/apps/web/.next-production/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/apps/web/.next-production/static ./apps/web/.next-production/static
USER nextjs
EXPOSE 3000
ENTRYPOINT ["/sbin/tini", "--"]
CMD ["node", "apps/web/server.js"]
HEALTHCHECK --interval=15s --timeout=5s --start-period=20s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:${PORT}/api/build-id || exit 1
