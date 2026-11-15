FROM node:24.11.0-bookworm-slim@sha256:76d0ed0ed93bed4f4376211e9d8fddac4d8b3fbdb54cc45955696001a3c91152 AS build
WORKDIR /src
ENV COREPACK_HOME=/corepack \
    COREPACK_ENABLE_DOWNLOAD_PROMPT=0
RUN corepack enable && corepack prepare pnpm@10.15.1 --activate

COPY apps/names/package.json apps/names/pnpm-lock.yaml apps/names/pnpm-workspace.yaml apps/names/tsconfig.json apps/names/next.config.ts ./apps/names/
COPY sdk/names ./sdk/names
COPY services/wallet/tsconfig.base.json ./services/wallet/tsconfig.base.json
RUN pnpm --dir sdk/names install --no-lockfile
RUN pnpm --dir apps/names install --frozen-lockfile --ignore-scripts
RUN pnpm --dir apps/names run prepare
COPY apps/names ./apps/names
RUN pnpm --dir apps/names run build

FROM node:24.11.0-bookworm-slim@sha256:76d0ed0ed93bed4f4376211e9d8fddac4d8b3fbdb54cc45955696001a3c91152
ENV NODE_ENV=production PORT=3000 HOSTNAME=0.0.0.0
WORKDIR /app
COPY --from=build /src/apps/names/.next/standalone ./
COPY --from=build /src/apps/names/.next/static ./apps/names/.next/static
USER 65532:65532
EXPOSE 3000
CMD ["node", "apps/names/server.js"]
