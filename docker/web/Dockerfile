FROM node:24.11.0-bookworm-slim@sha256:76d0ed0ed93bed4f4376211e9d8fddac4d8b3fbdb54cc45955696001a3c91152 AS build
WORKDIR /src/apps/web
COPY package.json package-lock.json /src/
COPY services/agent/sdk/typescript /src/services/agent/sdk/typescript
RUN npm --prefix /src/services/agent/sdk/typescript ci --ignore-scripts --no-audit --no-fund
RUN npm --prefix /src/services/agent/sdk/typescript run build
COPY apps/web /src/apps/web
COPY sdk/wallet /src/sdk/wallet
COPY services/wallet/package.json services/wallet/pnpm-lock.yaml services/wallet/pnpm-workspace.yaml /src/services/wallet/
COPY services/wallet/gateway/package.json /src/services/wallet/gateway/
COPY services/wallet/demo/package.json /src/services/wallet/demo/
COPY apps/perps/package.json /src/apps/perps/
RUN npm exec --yes --package=pnpm@10.34.9 -- pnpm --dir /src/services/wallet install --frozen-lockfile --ignore-scripts --filter @paxeer/wallet
RUN npm ci --ignore-scripts --no-audit --no-fund
ARG LAYERX_HUMAN_SERVICE_URL=https://layerx-human.layerx-testnet.svc:9443
RUN LAYERX_HUMAN_SERVICE_URL="$LAYERX_HUMAN_SERVICE_URL" npm run build

FROM node:24.11.0-bookworm-slim@sha256:76d0ed0ed93bed4f4376211e9d8fddac4d8b3fbdb54cc45955696001a3c91152
RUN apt-get update && apt-get install -y --no-install-recommends socat \
    && rm -rf /var/lib/apt/lists/*
ENV NODE_ENV=production PORT=3000 HOSTNAME=0.0.0.0
WORKDIR /app
COPY --from=build /src/apps/web/.next/standalone ./
COPY --from=build /src/apps/web/.next/static ./apps/web/.next/static
USER 65532:65532
CMD ["node", "apps/web/server.js"]
