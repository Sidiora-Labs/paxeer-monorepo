FROM node:22-bookworm-slim AS build
WORKDIR /docs/site/paxeer-docs
COPY docs/site/docs-assistant /docs/site/docs-assistant
COPY docs/site/paxeer-docs/package.json docs/site/paxeer-docs/package-lock.json docs/site/paxeer-docs/.npmrc ./
RUN npm ci
COPY docs/site/paxeer-docs/src ./src
COPY docs/site/paxeer-docs/public ./public
COPY docs/site/paxeer-docs/scripts ./scripts
COPY docs/site/paxeer-docs/next.config.js docs/site/paxeer-docs/postcss.config.mjs docs/site/paxeer-docs/tsconfig.json ./
COPY docs/release/7.0.0-systems.kvx /docs/release/7.0.0-systems.kvx
ARG NEXT_PUBLIC_SITE_URL
RUN npm run build

FROM node:22-bookworm-slim AS runtime
WORKDIR /app
ENV NODE_ENV=production PORT=3000 TRUST_PROXY=1
COPY docs/site/docs-assistant/server /docs-assistant/server
COPY --from=build /docs/site/paxeer-docs/out ./out
COPY --from=build /docs/site/paxeer-docs/.generated/assistant-docs.json ./.generated/assistant-docs.json
COPY docs/site/paxeer-docs/scripts/serve.mjs ./scripts/serve.mjs
USER node
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 CMD node -e "fetch('http://127.0.0.1:'+process.env.PORT+'/healthz').then(r=>{if(!r.ok)process.exit(1)}).catch(()=>process.exit(1))"
CMD ["node", "scripts/serve.mjs"]
