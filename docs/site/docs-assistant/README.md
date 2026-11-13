# Documentation assistant

Shared React UI and Node server for Learn Centra, Learn Workmates and Paxeer X Docs. The UI uses Apps SDK UI components and each site's semantic theme. Every build creates `.generated/assistant-docs.json` from the same content as its documentation pages.

The Node server serves the production site and `POST /api/docs-assistant` on one origin. It retrieves relevant sections, calls Centra gateway, and streams a cited answer. Conversation history stays in the current browser session; the server does not persist chats. Source files and runtime credentials are separate from public assets. Paxeer X inventories include the complete network documentation: EVM and native execution, LayerX, agents, payments, businesses, markets, programs, proofs and operations. The landing page and every documentation route are included in the generated Markdown and assistant inventory. Page descriptions come directly from each page, and assistant excerpts retain section anchors for precise citations. Other sites without heading metadata continue to cite their existing page URLs.

Runtime settings:

| Variable | Purpose |
| --- | --- |
| `CENTRA_GATEWAY_API_KEY` | Required server-only Centra gateway credential. |
| `CENTRA_GATEWAY_URL` | Defaults to `https://gateway.centra.ag/v1`. |
| `DOCS_ASSISTANT_MODEL` | Defaults to `openrouter/sakana/fugu-max`. |
| `PORT` | Defaults to `3000`. |
| `HOST` | Defaults to `0.0.0.0`. |
| `TRUST_PROXY` | Set to `1` behind the configured Dokploy proxy. |
| `RELEASE_ID` | Deployment identifier returned by `/healthz`. |

From a site directory, build and run `npm run serve` with the credential provided through the environment. The local developer environment also accepts the existing `GATEWAY_ROUTER_API_KEY` name. Never use a `VITE_` or `NEXT_PUBLIC_` credential, and never copy a dotenv file into a build or release archive.

Each site's Dockerfile builds from the parent workspace context, including this shared package. Learn sites use their pnpm lockfiles; Paxeer uses its npm lockfile. The release scripts package already-built, verified output for Dokploy without Git access.

The assistant permits up to 30 questions per visitor per ten minutes, two concurrent requests per visitor and twelve per process. Requests have a 120-second upstream timeout and bounded conversation/context sizes. The gateway exposes no tool execution through this endpoint. Cancellation aborts the upstream request, and interrupted or truncated streams are shown as incomplete answers.
