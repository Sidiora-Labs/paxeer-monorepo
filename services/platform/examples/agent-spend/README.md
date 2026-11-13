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

## Review recovery

The [wallet transport](wallet-review-transport.mjs) looks up the verified unsigned
transaction hash before creation and uses the same hash as the idempotency key.
An unavailable lookup route returns an explicit error. Reopening the owner store
reconciles a pending preparation to the original review ID; approval requires
explicit owner terminal confirmation. The [service bindings](services.mjs)
retain the owner context and review state in SQLite.
