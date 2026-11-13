FROM node:22-bookworm-slim AS build
WORKDIR /app
COPY docs-assistant /docs-assistant
COPY paxeer-docs/package.json paxeer-docs/package-lock.json paxeer-docs/.npmrc ./
RUN npm ci
COPY paxeer-docs/src ./src
COPY paxeer-docs/public ./public
COPY paxeer-docs/scripts ./scripts
COPY paxeer-docs/next.config.js paxeer-docs/postcss.config.mjs paxeer-docs/tsconfig.json ./
RUN npm run build

FROM node:22-bookworm-slim AS runtime
WORKDIR /app
ENV NODE_ENV=production PORT=3000 TRUST_PROXY=1
COPY docs-assistant/server /docs-assistant/server
COPY --from=build /app/out ./out
COPY --from=build /app/.generated/assistant-docs.json ./.generated/assistant-docs.json
COPY paxeer-docs/scripts/serve.mjs ./scripts/serve.mjs
USER node
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 CMD node -e "fetch('http://127.0.0.1:'+process.env.PORT+'/healthz').then(r=>{if(!r.ok)process.exit(1)}).catch(()=>process.exit(1))"
CMD ["node", "scripts/serve.mjs"]
