FROM node:24.11.0-bookworm-slim@sha256:76d0ed0ed93bed4f4376211e9d8fddac4d8b3fbdb54cc45955696001a3c91152
WORKDIR /app
ENV PORT=8080
COPY apps/inference-peer/package.json apps/inference-peer/index.html apps/inference-peer/NOTICE apps/inference-peer/LICENSE-ENGINE ./
COPY apps/inference-peer/scripts ./scripts
COPY apps/inference-peer/src ./src
COPY apps/inference-peer/engine ./engine
USER 65532:65532
EXPOSE 8080
CMD ["node", "scripts/serve.mjs"]
