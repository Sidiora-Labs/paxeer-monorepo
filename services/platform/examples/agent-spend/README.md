# Agent spend

Use Node.js **22.18.0 or newer** for installation, setup, launch and qualification.
The example checks this minimum before loading integrations or opening stores.

From the repository root, install with `npm ci`, then build the example's
workspace dependency closure with:

```sh
npm run build:dependencies -- @sidiora/layerx-example-agent-spend
```

That closure contains the SDK, seller middleware, agent middleware and agent
integrations. The bundled original-wallet services also require the wallet
workspace: install its locked dependencies with
`pnpm --dir services/wallet install --frozen-lockfile` and build with
`pnpm --dir sdk/wallet build`. Then launch with
`npm start --workspace @sidiora/layerx-example-agent-spend` using the required
owner session, private store paths and service configuration from `index.mjs`
and `services.mjs`.

Review recovery requires the wallet gateway contract in
[repair-22-33-1364.kvx](repair-22-33-1364.kvx). The client looks up the verified
unsigned transaction hash before creation and uses the same hash as the
idempotency key. A missing lookup route fails explicitly. Reopening the owner
store reconciles a pending preparation to the original review ID; approval
still requires the explicit owner terminal confirmation.

The declared repair gate runs an in-process walkthrough of the real handoff,
SQLite owner store, wallet SDK, authentication and HTTP transport against a
loopback wallet peer. The gate installs the root npm workspace and builds the
agent SDK and middleware. It imports the example in place and loads committed
wallet and integration TypeScript sources in process, preserving dependency
resolution without credentials or an external service.
