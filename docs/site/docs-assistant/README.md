# Documentation assistant

The `@centra/docs-assistant` package supplies the React chat panel and Node server used by this repository's documentation site in `../paxeer-docs`. The UI uses Apps SDK UI components and the site's semantic theme. `npm run prepare:docs` in the site creates `.generated/assistant-docs.json` from the documentation pages; `npm run build` runs that generation automatically.

The Node server serves the site's `out/` static export and `POST /api/docs-assistant` on one origin. It retrieves relevant sections, calls the Centra gateway and streams a cited answer. Conversation history stays in the current browser session; the server does not persist chats. Page descriptions come directly from each page, and assistant excerpts retain section anchors for citations.

## Build and run

From the repository root:

```sh
cd docs/site/paxeer-docs
npm ci
npm run build
npm run serve
```

Supply the gateway credential through the server environment. The server listens on port 3000 by default. `GET /healthz` returns the site name, document count, assistant configuration state and release identifier.

| Variable | Purpose |
| --- | --- |
| `CENTRA_GATEWAY_API_KEY` | Server-only Centra gateway credential for assistant requests. |
| `CENTRA_GATEWAY_URL` | Defaults to `https://gateway.centra.ag/v1`. |
| `DOCS_ASSISTANT_MODEL` | Defaults to `openrouter/sakana/fugu-max`. |
| `PORT` | Defaults to `3000`. |
| `HOST` | Defaults to `0.0.0.0`. |
| `TRUST_PROXY` | Set to `1` behind a trusted proxy to use the forwarded visitor address for rate limits. |
| `RELEASE_ID` | Deployment identifier returned by `/healthz`. |

The server also accepts `GATEWAY_ROUTER_API_KEY` as the gateway credential. Keep credentials server-side and outside public assets.

The site Dockerfile uses `docs/site` as its build context so it can include this package. From the repository root:

```sh
docker build -f docs/site/paxeer-docs/Dockerfile -t paxeer-docs docs/site
```

The assistant permits up to 30 questions per visitor per ten minutes, two concurrent requests per visitor and twelve per process. Requests have a 120-second upstream timeout and bounded conversation and context sizes. The endpoint provides no tool execution. Cancellation aborts the upstream request, and interrupted or truncated streams are shown as incomplete answers.
