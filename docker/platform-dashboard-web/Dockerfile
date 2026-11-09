FROM node:24.6.0-bookworm-slim@sha256:9b741b28148b0195d62fa456ed84dd6c953c1f17a3761f3e6e6797a754d9edff AS build
WORKDIR /src
COPY apps/web/package.json apps/web/package-lock.json /src/apps/web/
COPY apps/web/packages/layerx-ui /src/apps/web/packages/layerx-ui
COPY services/platform/hosted/dashboard/web /src/services/platform/hosted/dashboard/web
COPY scripts/ci/developer-dashboard-lock.mjs /src/scripts/ci/developer-dashboard-lock.mjs
RUN node scripts/ci/developer-dashboard-lock.mjs
RUN npm --prefix /src/apps/web ci --ignore-scripts --no-audit --no-fund && \
    npm --prefix /src/apps/web run build:ui && \
    ln -s ../../../../../apps/web/node_modules /src/services/platform/hosted/dashboard/web/node_modules
WORKDIR /src/services/platform/hosted/dashboard/web
ARG DASHBOARD_API_ORIGIN
RUN npm run test:static && DASHBOARD_API_ORIGIN="$DASHBOARD_API_ORIGIN" npm run build

FROM node:24.6.0-bookworm-slim@sha256:9b741b28148b0195d62fa456ed84dd6c953c1f17a3761f3e6e6797a754d9edff
ENV NODE_ENV=production PORT=3000 HOSTNAME=0.0.0.0
WORKDIR /app
COPY --from=build /src/services/platform/hosted/dashboard/web/.next/standalone ./
COPY --from=build /src/services/platform/hosted/dashboard/web/.next/static ./services/platform/hosted/dashboard/web/.next/static
WORKDIR /app/services/platform/hosted/dashboard/web
USER 65532:65532
CMD ["node", "server.js"]
