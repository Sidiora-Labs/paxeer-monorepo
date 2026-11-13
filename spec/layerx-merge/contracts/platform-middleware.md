# Platform middleware contract

The [platform middleware](../../../services/platform/middleware) and [examples](../../../services/platform/examples) connect applications to payments and agent services. Their exported APIs, configuration names and HTTP handlers are listed below. Payment verification uses transaction and receipt evidence, rather than the existence of a middleware response.

The tables identify code declarations and configuration names. Activation, authentication and transport registration are defined by the linked implementations.

## examples-agent-spend-env-vars

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `LAYERX_TENANT, LAYERX_ACTOR` | [services/platform/examples/agent-spend/services.mjs](../../../services/platform/examples/agent-spend/services.mjs) |
| `env_var` | `LAYERX_DAEMON_PREPARATION_STORE_PATH` | [services/platform/examples/agent-spend/services.mjs](../../../services/platform/examples/agent-spend/services.mjs) |
| `env_var` | `LAYERX_DAEMON_RECEIPT_STORE_PATH` | [services/platform/examples/agent-spend/services.mjs](../../../services/platform/examples/agent-spend/services.mjs) |
| `env_var` | `LAYERX_SESSION_ID` | [services/platform/examples/agent-spend/services.mjs](../../../services/platform/examples/agent-spend/services.mjs) |
| `env_var` | `LAYERX_WALLET_API_URL` | [services/platform/examples/agent-spend/services.mjs](../../../services/platform/examples/agent-spend/services.mjs) |
| `env_var` | `LAYERX_WALLET_PRINCIPAL` | [services/platform/examples/agent-spend/services.mjs](../../../services/platform/examples/agent-spend/services.mjs) |
| `env_var` | `LAYERX_WALLET_SESSION_FILE` | [services/platform/examples/agent-spend/services.mjs](../../../services/platform/examples/agent-spend/services.mjs) |
| `env_var` | `LAYERX_WALLET_STORE_PATH` | [services/platform/examples/agent-spend/services.mjs](../../../services/platform/examples/agent-spend/services.mjs) |
| `env_var` | `LAYERX_WALLET_SUPABASE_ANON_KEY` | [services/platform/examples/agent-spend/services.mjs](../../../services/platform/examples/agent-spend/services.mjs) |
| `env_var` | `LAYERX_WALLET_SUPABASE_URL` | [services/platform/examples/agent-spend/services.mjs](../../../services/platform/examples/agent-spend/services.mjs) |
| `env_var` | `LAYERX_SPEND_REQUEST_JSON` | [services/platform/examples/agent-spend/index.mjs](../../../services/platform/examples/agent-spend/index.mjs) |
| `env_var` | `LAYERX_WALLET_PREPARATION_REF` | [services/platform/examples/agent-spend/index.mjs](../../../services/platform/examples/agent-spend/index.mjs) |
| `env_var` | `LAYERX_WALLET_REVIEW_ID` | [services/platform/examples/agent-spend/index.mjs](../../../services/platform/examples/agent-spend/index.mjs) |

## examples-buyer-agent-config-keys

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `"application": "buyer-agent",` | [services/platform/examples/buyer-agent/layerx.example.json](../../../services/platform/examples/buyer-agent/layerx.example.json) |
| `config_key` | `"environments": {` | [services/platform/examples/buyer-agent/layerx.example.json](../../../services/platform/examples/buyer-agent/layerx.example.json) |
| `config_key` | `"beta": {` | [services/platform/examples/buyer-agent/layerx.example.json](../../../services/platform/examples/buyer-agent/layerx.example.json) |
| `config_key` | `"idempotencyEnvironment": "LAYERX_EMULATOR_PAYMENT_KEY"` | [services/platform/examples/buyer-agent/layerx.example.json](../../../services/platform/examples/buyer-agent/layerx.example.json) |
| `config_key` | `"scheme": "exact",` | [services/platform/examples/buyer-agent/layerx.example.json](../../../services/platform/examples/buyer-agent/layerx.example.json) |
| `config_key` | `"sourceEnvironment": "LAYERX_EMULATOR_SOURCE",` | [services/platform/examples/buyer-agent/layerx.example.json](../../../services/platform/examples/buyer-agent/layerx.example.json) |
| `config_key` | `"tokenEnvironment": "LAYERX_EMULATOR_TOKEN",` | [services/platform/examples/buyer-agent/layerx.example.json](../../../services/platform/examples/buyer-agent/layerx.example.json) |
| `config_key` | `"emulator": {` | [services/platform/examples/buyer-agent/layerx.example.json](../../../services/platform/examples/buyer-agent/layerx.example.json) |
| `config_key` | `"version": 1,` | [services/platform/examples/buyer-agent/layerx.example.json](../../../services/platform/examples/buyer-agent/layerx.example.json) |

## examples-cli-commands

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `if (arguments_.length === 1 && arguments_[0] === "--check") {` | [services/platform/examples/run-reference-apps.mjs](../../../services/platform/examples/run-reference-apps.mjs) |

## examples-config-keys

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `"applications": [` | [services/platform/examples/reference-apps.json](../../../services/platform/examples/reference-apps.json) |
| `config_key` | `"commands": {` | [services/platform/examples/reference-apps.json](../../../services/platform/examples/reference-apps.json) |
| `config_key` | `"beta": ["npm", "run", "start:beta", "--workspace", "@sidiora/layerx-example-buyer-agent"]` | [services/platform/examples/reference-apps.json](../../../services/platform/examples/reference-apps.json) |
| `config_key` | `"emulator": ["npm", "run", "start:emulator", "--workspace", "@sidiora/layerx-example-buyer-agent"],` | [services/platform/examples/reference-apps.json](../../../services/platform/examples/reference-apps.json) |
| `config_key` | `"config": "layerx.example.json",` | [services/platform/examples/reference-apps.json](../../../services/platform/examples/reference-apps.json) |
| `config_key` | `"name": "buyer-agent",` | [services/platform/examples/reference-apps.json](../../../services/platform/examples/reference-apps.json) |
| `config_key` | `"package": "@sidiora/layerx-example-buyer-agent",` | [services/platform/examples/reference-apps.json](../../../services/platform/examples/reference-apps.json) |
| `config_key` | `"path": "services/platform/examples/buyer-agent",` | [services/platform/examples/reference-apps.json](../../../services/platform/examples/reference-apps.json) |
| `config_key` | `"symbol": "platform_ref_buyer",` | [services/platform/examples/reference-apps.json](../../../services/platform/examples/reference-apps.json) |
| `config_key` | `"version": 1,` | [services/platform/examples/reference-apps.json](../../../services/platform/examples/reference-apps.json) |

## examples-env-vars

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `LAYERX_BIN` | [services/platform/examples/run-reference-apps.mjs](../../../services/platform/examples/run-reference-apps.mjs) |
| `env_var` | `LAYERX_CREDENTIAL_PASSPHRASE` | [services/platform/examples/run-reference-apps.mjs](../../../services/platform/examples/run-reference-apps.mjs) |
| `env_var` | `LAYERX_CREDENTIAL_STORE` | [services/platform/examples/run-reference-apps.mjs](../../../services/platform/examples/run-reference-apps.mjs) |
| `env_var` | `LAYERX_EMULATOR_MERCHANT_CHECKOUT_KEY` | [services/platform/examples/run-reference-apps.mjs](../../../services/platform/examples/run-reference-apps.mjs) |
| `env_var` | `LAYERX_EMULATOR_PRICE` | [services/platform/examples/run-reference-apps.mjs](../../../services/platform/examples/run-reference-apps.mjs) |
| `env_var` | `LAYERX_EMULATOR_SEED_FILE` | [services/platform/examples/run-reference-apps.mjs](../../../services/platform/examples/run-reference-apps.mjs) |
| `env_var` | `LAYERX_EMULATOR_TOKEN` | [services/platform/examples/run-reference-apps.mjs](../../../services/platform/examples/run-reference-apps.mjs) |
| source | — | [services/platform/examples/build-reference-apps.mjs](../../../services/platform/examples/build-reference-apps.mjs) |
| source | — | [services/platform/examples/qualify-reference-apps.mjs](../../../services/platform/examples/qualify-reference-apps.mjs) |

## examples-marketplace-config-keys

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `"application": "marketplace",` | [services/platform/examples/marketplace/layerx.example.json](../../../services/platform/examples/marketplace/layerx.example.json) |
| `config_key` | `"environments": {` | [services/platform/examples/marketplace/layerx.example.json](../../../services/platform/examples/marketplace/layerx.example.json) |
| `config_key` | `"beta": {` | [services/platform/examples/marketplace/layerx.example.json](../../../services/platform/examples/marketplace/layerx.example.json) |
| `config_key` | `"assetEnvironment": "LAYERX_EMULATOR_ASSET",` | [services/platform/examples/marketplace/layerx.example.json](../../../services/platform/examples/marketplace/layerx.example.json) |
| `config_key` | `"cliBinaryEnvironment": "LAYERX_BIN",` | [services/platform/examples/marketplace/layerx.example.json](../../../services/platform/examples/marketplace/layerx.example.json) |
| `config_key` | `"idempotencyEnvironment": "LAYERX_EMULATOR_MARKETPLACE_KEY"` | [services/platform/examples/marketplace/layerx.example.json](../../../services/platform/examples/marketplace/layerx.example.json) |
| `config_key` | `"listingIdEnvironment": "LAYERX_EMULATOR_MARKETPLACE_LISTING_ID",` | [services/platform/examples/marketplace/layerx.example.json](../../../services/platform/examples/marketplace/layerx.example.json) |
| `config_key` | `"priceEnvironment": "LAYERX_EMULATOR_PRICE",` | [services/platform/examples/marketplace/layerx.example.json](../../../services/platform/examples/marketplace/layerx.example.json) |
| `config_key` | `"programIdEnvironment": "LAYERX_EMULATOR_MARKETPLACE_PROGRAM_ID",` | [services/platform/examples/marketplace/layerx.example.json](../../../services/platform/examples/marketplace/layerx.example.json) |
| `config_key` | `"receiptDigestEnvironment": "LAYERX_EMULATOR_MARKETPLACE_RECEIPT_DIGEST",` | [services/platform/examples/marketplace/layerx.example.json](../../../services/platform/examples/marketplace/layerx.example.json) |
| `config_key` | `"sellerEnvironment": "LAYERX_EMULATOR_MERCHANT",` | [services/platform/examples/marketplace/layerx.example.json](../../../services/platform/examples/marketplace/layerx.example.json) |
| `config_key` | `"tokenEnvironment": "LAYERX_EMULATOR_TOKEN",` | [services/platform/examples/marketplace/layerx.example.json](../../../services/platform/examples/marketplace/layerx.example.json) |
| `config_key` | `"emulator": {` | [services/platform/examples/marketplace/layerx.example.json](../../../services/platform/examples/marketplace/layerx.example.json) |
| `config_key` | `"version": 1,` | [services/platform/examples/marketplace/layerx.example.json](../../../services/platform/examples/marketplace/layerx.example.json) |

## examples-marketplace-env-vars

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `LAYERX_CREDENTIAL_PASSPHRASE` | [services/platform/examples/marketplace/index.mjs](../../../services/platform/examples/marketplace/index.mjs) |
| `env_var` | `PAXEER_X_REFERENCE_APP_ARTIFACTS` | [services/platform/examples/marketplace/index.mjs](../../../services/platform/examples/marketplace/index.mjs) |

## examples-merchant-checkout-config-keys

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `"application": "merchant-checkout",` | [services/platform/examples/merchant-checkout/layerx.example.json](../../../services/platform/examples/merchant-checkout/layerx.example.json) |
| `config_key` | `"environments": {` | [services/platform/examples/merchant-checkout/layerx.example.json](../../../services/platform/examples/merchant-checkout/layerx.example.json) |
| `config_key` | `"beta": {` | [services/platform/examples/merchant-checkout/layerx.example.json](../../../services/platform/examples/merchant-checkout/layerx.example.json) |
| `config_key` | `"accountEnvironment": "LAYERX_EMULATOR_MERCHANT_ACCOUNT",` | [services/platform/examples/merchant-checkout/layerx.example.json](../../../services/platform/examples/merchant-checkout/layerx.example.json) |
| `config_key` | `"assetEnvironment": "LAYERX_EMULATOR_ASSET",` | [services/platform/examples/merchant-checkout/layerx.example.json](../../../services/platform/examples/merchant-checkout/layerx.example.json) |
| `config_key` | `"currencyEnvironment": "LAYERX_EMULATOR_CURRENCY",` | [services/platform/examples/merchant-checkout/layerx.example.json](../../../services/platform/examples/merchant-checkout/layerx.example.json) |
| `config_key` | `"payToEnvironment": "LAYERX_EMULATOR_MERCHANT",` | [services/platform/examples/merchant-checkout/layerx.example.json](../../../services/platform/examples/merchant-checkout/layerx.example.json) |
| `config_key` | `"port": 8082` | [services/platform/examples/merchant-checkout/layerx.example.json](../../../services/platform/examples/merchant-checkout/layerx.example.json) |
| `config_key` | `"priceEnvironment": "LAYERX_EMULATOR_PRICE",` | [services/platform/examples/merchant-checkout/layerx.example.json](../../../services/platform/examples/merchant-checkout/layerx.example.json) |
| `config_key` | `"scheme": "exact",` | [services/platform/examples/merchant-checkout/layerx.example.json](../../../services/platform/examples/merchant-checkout/layerx.example.json) |
| `config_key` | `"stateDirectory": "state",` | [services/platform/examples/merchant-checkout/layerx.example.json](../../../services/platform/examples/merchant-checkout/layerx.example.json) |
| `config_key` | `"tokenEnvironment": "LAYERX_EMULATOR_TOKEN",` | [services/platform/examples/merchant-checkout/layerx.example.json](../../../services/platform/examples/merchant-checkout/layerx.example.json) |
| `config_key` | `"webhookKeysEnvironment": "LAYERX_EMULATOR_WEBHOOK_PUBLIC_KEYS_JSON",` | [services/platform/examples/merchant-checkout/layerx.example.json](../../../services/platform/examples/merchant-checkout/layerx.example.json) |
| `config_key` | `"emulator": {` | [services/platform/examples/merchant-checkout/layerx.example.json](../../../services/platform/examples/merchant-checkout/layerx.example.json) |
| `config_key` | `"version": 1,` | [services/platform/examples/merchant-checkout/layerx.example.json](../../../services/platform/examples/merchant-checkout/layerx.example.json) |

## examples-merchant-shop-config-keys

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `"application": "merchant-shop",` | [services/platform/examples/merchant-shop/layerx.example.json](../../../services/platform/examples/merchant-shop/layerx.example.json) |
| `config_key` | `"environments": {` | [services/platform/examples/merchant-shop/layerx.example.json](../../../services/platform/examples/merchant-shop/layerx.example.json) |
| `config_key` | `"beta": {` | [services/platform/examples/merchant-shop/layerx.example.json](../../../services/platform/examples/merchant-shop/layerx.example.json) |
| `config_key` | `"accountEnvironment": "LAYERX_EMULATOR_MERCHANT_ACCOUNT",` | [services/platform/examples/merchant-shop/layerx.example.json](../../../services/platform/examples/merchant-shop/layerx.example.json) |
| `config_key` | `"assetEnvironment": "LAYERX_EMULATOR_ASSET",` | [services/platform/examples/merchant-shop/layerx.example.json](../../../services/platform/examples/merchant-shop/layerx.example.json) |
| `config_key` | `"currencyEnvironment": "LAYERX_EMULATOR_CURRENCY",` | [services/platform/examples/merchant-shop/layerx.example.json](../../../services/platform/examples/merchant-shop/layerx.example.json) |
| `config_key` | `"payToEnvironment": "LAYERX_EMULATOR_MERCHANT",` | [services/platform/examples/merchant-shop/layerx.example.json](../../../services/platform/examples/merchant-shop/layerx.example.json) |
| `config_key` | `"port": 8082` | [services/platform/examples/merchant-shop/layerx.example.json](../../../services/platform/examples/merchant-shop/layerx.example.json) |
| `config_key` | `"priceEnvironment": "LAYERX_EMULATOR_PRICE",` | [services/platform/examples/merchant-shop/layerx.example.json](../../../services/platform/examples/merchant-shop/layerx.example.json) |
| `config_key` | `"scheme": "exact",` | [services/platform/examples/merchant-shop/layerx.example.json](../../../services/platform/examples/merchant-shop/layerx.example.json) |
| `config_key` | `"stateDirectory": "state",` | [services/platform/examples/merchant-shop/layerx.example.json](../../../services/platform/examples/merchant-shop/layerx.example.json) |
| `config_key` | `"tokenEnvironment": "LAYERX_EMULATOR_TOKEN",` | [services/platform/examples/merchant-shop/layerx.example.json](../../../services/platform/examples/merchant-shop/layerx.example.json) |
| `config_key` | `"webhookKeysEnvironment": "LAYERX_EMULATOR_WEBHOOK_PUBLIC_KEYS_JSON",` | [services/platform/examples/merchant-shop/layerx.example.json](../../../services/platform/examples/merchant-shop/layerx.example.json) |
| `config_key` | `"emulator": {` | [services/platform/examples/merchant-shop/layerx.example.json](../../../services/platform/examples/merchant-shop/layerx.example.json) |
| `config_key` | `"version": 1,` | [services/platform/examples/merchant-shop/layerx.example.json](../../../services/platform/examples/merchant-shop/layerx.example.json) |

## examples-paid-api-config-keys

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `"application": "paid-api",` | [services/platform/examples/paid-api/layerx.example.json](../../../services/platform/examples/paid-api/layerx.example.json) |
| `config_key` | `"environments": {` | [services/platform/examples/paid-api/layerx.example.json](../../../services/platform/examples/paid-api/layerx.example.json) |
| `config_key` | `"beta": {` | [services/platform/examples/paid-api/layerx.example.json](../../../services/platform/examples/paid-api/layerx.example.json) |
| `config_key` | `"accountEnvironment": "LAYERX_EMULATOR_SELLER_ACCOUNT",` | [services/platform/examples/paid-api/layerx.example.json](../../../services/platform/examples/paid-api/layerx.example.json) |
| `config_key` | `"assetEnvironment": "LAYERX_EMULATOR_ASSET",` | [services/platform/examples/paid-api/layerx.example.json](../../../services/platform/examples/paid-api/layerx.example.json) |
| `config_key` | `"currencyEnvironment": "LAYERX_EMULATOR_CURRENCY",` | [services/platform/examples/paid-api/layerx.example.json](../../../services/platform/examples/paid-api/layerx.example.json) |
| `config_key` | `"fulfillmentDirectory": "state/fulfillments",` | [services/platform/examples/paid-api/layerx.example.json](../../../services/platform/examples/paid-api/layerx.example.json) |
| `config_key` | `"payToEnvironment": "LAYERX_EMULATOR_SELLER",` | [services/platform/examples/paid-api/layerx.example.json](../../../services/platform/examples/paid-api/layerx.example.json) |
| `config_key` | `"port": 8081` | [services/platform/examples/paid-api/layerx.example.json](../../../services/platform/examples/paid-api/layerx.example.json) |
| `config_key` | `"priceEnvironment": "LAYERX_EMULATOR_PRICE",` | [services/platform/examples/paid-api/layerx.example.json](../../../services/platform/examples/paid-api/layerx.example.json) |
| `config_key` | `"resourceFile": "resource.json",` | [services/platform/examples/paid-api/layerx.example.json](../../../services/platform/examples/paid-api/layerx.example.json) |
| `config_key` | `"scheme": "exact",` | [services/platform/examples/paid-api/layerx.example.json](../../../services/platform/examples/paid-api/layerx.example.json) |
| `config_key` | `"emulator": {` | [services/platform/examples/paid-api/layerx.example.json](../../../services/platform/examples/paid-api/layerx.example.json) |
| `config_key` | `"forecast": "clear",` | [services/platform/examples/paid-api/resource.json](../../../services/platform/examples/paid-api/resource.json) |
| `config_key` | `"service": "LayerX metered weather",` | [services/platform/examples/paid-api/resource.json](../../../services/platform/examples/paid-api/resource.json) |
| `config_key` | `"units": "protocol-integer"` | [services/platform/examples/paid-api/resource.json](../../../services/platform/examples/paid-api/resource.json) |
| `config_key` | `"version": 1,` | [services/platform/examples/paid-api/layerx.example.json](../../../services/platform/examples/paid-api/layerx.example.json) |

## examples-paid-api-http-routes

| Kind | Declaration | Source |
|---|---|---|
| `http_route` | `if (request.method !== "GET" \|\| request.url !== "/paid") {` | [services/platform/examples/paid-api/index.mjs](../../../services/platform/examples/paid-api/index.mjs) |

## examples-support-cli-commands

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `if (arguments_.length === 4 && (arguments_[2] !== "--action" \|\| !["deploy", "list", "buy"].includes(arguments_[3]))) {` | [services/platform/examples/support/runtime.mjs](../../../services/platform/examples/support/runtime.mjs) |
| `cli_command` | `if ((arguments_.length !== 2 && arguments_.length !== 4) \|\| arguments_[0] !== "--environment" \|\| !["emulator", "beta"].includes(arguments_[1])) {` | [services/platform/examples/support/runtime.mjs](../../../services/platform/examples/support/runtime.mjs) |

## examples-support-env-vars

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `LAYERX_EXAMPLE_CONFIG` | [services/platform/examples/support/runtime.mjs](../../../services/platform/examples/support/runtime.mjs) |
| `env_var` | `LAYERX_EXAMPLE_ENDPOINT` | [services/platform/examples/support/runtime.mjs](../../../services/platform/examples/support/runtime.mjs) |
| `env_var` | `LAYERX_EXAMPLE_STATE_ROOT` | [services/platform/examples/support/runtime.mjs](../../../services/platform/examples/support/runtime.mjs) |

## examples-support-http-routes

| Kind | Declaration | Source |
|---|---|---|
| `http_route` | `if (request.method === "GET" && request.url?.startsWith("/orders/")) {` | [services/platform/examples/support/merchant-app.mjs](../../../services/platform/examples/support/merchant-app.mjs) |
| `http_route` | `if (request.method !== "POST" \|\| request.url !== "/checkout") {` | [services/platform/examples/support/merchant-app.mjs](../../../services/platform/examples/support/merchant-app.mjs) |
| `http_route` | `if (request.method === "POST" && request.url === "/webhooks/settlement") {` | [services/platform/examples/support/merchant-app.mjs](../../../services/platform/examples/support/merchant-app.mjs) |

## middleware-agent-error-codes

| Kind | Declaration | Source |
|---|---|---|
| `error_code` | `if (result.kind === "exhausted") return { kind: "refused", reason: "budget_refused" };` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `error_code` | `if (reservation.state === "released") return { kind: "refused", reason: "budget_released" };` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |

## middleware-agent-ts-exports

| Kind | Declaration | Source |
|---|---|---|
| `ts_export` | `export class AgentGrantMiddleware implements GrantDrawExecution {` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `ts_export` | `export interface AgentMiddlewareConfig {` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `ts_export` | `export type AgentMiddlewareErrorCode =` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `ts_export` | `export type AgentRefusalRetry = Exclude<RetryClass, "unknown-outcome">;` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `ts_export` | `export interface ApprovalHold {` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `ts_export` | `export interface BudgetTransition {` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `ts_export` | `export function platform_mw_agent(): "budget-aware-receipt-verified-agent" {` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `ts_export` | `export type ReservedBudgetReservation = BudgetReservationBase & {` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `ts_export` | `export interface Submission {` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `ts_export` | `export async function verifyCommittedAgentPayment(` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `ts_export` | `export interface AgentBudgetLedger {` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `ts_export` | `export class AgentMiddleware {` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `ts_export` | `export class AgentMiddlewareError extends Error {` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `ts_export` | `export interface AgentPreparationBudget {` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `ts_export` | `export interface AgentReceiptContext {` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `ts_export` | `AgentReceiptEvidence` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `ts_export` | `export interface AgentReceiptRecording extends AgentReceiptContext {` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `ts_export` | `export interface AgentReceiptResolver {` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `ts_export` | `export interface AgentRefusal {` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `ts_export` | `export interface AgentSigner {` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `ts_export` | `export interface AgentSpendRequest {` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `ts_export` | `export type AgentSpendResult =` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `ts_export` | `export interface BudgetCommitTransition extends BudgetTransition {` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `ts_export` | `export interface BudgetHoldTransition extends BudgetTransition {` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `ts_export` | `export interface BudgetReleaseTransition extends BudgetTransition {` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `ts_export` | `export type BudgetReservation =` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `ts_export` | `export type BudgetReserveResult =` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `ts_export` | `export type CommittedBudgetReservation = BudgetReservationBase & {` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `ts_export` | `export function decodeDaemonPrepared(value: unknown, request: AgentSpendRequest, _protocolVersion: number): PreparedActivity {` | [services/platform/middleware/agent/src/daemon.ts](../../../services/platform/middleware/agent/src/daemon.ts) |
| `ts_export` | `export function decodeDaemonSubmission(value: unknown, expectedTxHash?: string, expectedReference?: string): Submission {` | [services/platform/middleware/agent/src/daemon.ts](../../../services/platform/middleware/agent/src/daemon.ts) |
| `ts_export` | `export type HeldBudgetReservation = BudgetReservationBase & {` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `ts_export` | `export interface OwnerBudgetSpendResult {` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `ts_export` | `export interface PreparedActivity {` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `ts_export` | `export type ReleasedBudgetReservation = BudgetReservationBase & {` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |
| `ts_export` | `export function signedActivityId(prepared: PreparedActivity, signature: string, publicKey: string): string {` | [services/platform/middleware/agent/src/daemon.ts](../../../services/platform/middleware/agent/src/daemon.ts) |
| `ts_export` | `export async function verifyAgentPayment(` | [services/platform/middleware/agent/src/index.ts](../../../services/platform/middleware/agent/src/index.ts) |

## middleware-buyer-ts-exports

| Kind | Declaration | Source |
|---|---|---|
| `ts_export` | `export interface BuyerMiddlewareConfig {` | [services/platform/middleware/buyer/src/index.ts](../../../services/platform/middleware/buyer/src/index.ts) |
| `ts_export` | `export interface BuyerRetryPolicy {` | [services/platform/middleware/buyer/src/index.ts](../../../services/platform/middleware/buyer/src/index.ts) |
| `ts_export` | `export interface CapturedSettlement extends PortableReceiptV2 {` | [services/platform/middleware/buyer/src/index.ts](../../../services/platform/middleware/buyer/src/index.ts) |
| `ts_export` | `export interface EvidenceMaterial {` | [services/platform/middleware/buyer/src/index.ts](../../../services/platform/middleware/buyer/src/index.ts) |
| `ts_export` | `export interface EvidenceReference {` | [services/platform/middleware/buyer/src/index.ts](../../../services/platform/middleware/buyer/src/index.ts) |
| `ts_export` | `export function grantPaymentHeader(required: PaymentRequired, accepted: PaymentRequirements, rawTransaction: string): string {` | [services/platform/middleware/buyer/src/index.ts](../../../services/platform/middleware/buyer/src/index.ts) |
| `ts_export` | `export type JourneyState =` | [services/platform/middleware/buyer/src/index.ts](../../../services/platform/middleware/buyer/src/index.ts) |
| `ts_export` | `export interface LayerXHttpTransportConfig {` | [services/platform/middleware/buyer/src/index.ts](../../../services/platform/middleware/buyer/src/index.ts) |
| `ts_export` | `export interface MoveQuoteRequest {` | [services/platform/middleware/buyer/src/index.ts](../../../services/platform/middleware/buyer/src/index.ts) |
| `ts_export` | `export type PaymentPreparation =` | [services/platform/middleware/buyer/src/index.ts](../../../services/platform/middleware/buyer/src/index.ts) |
| `ts_export` | `export function paymentQuoteTerms(requirements: PaymentRequirements): PaymentQuoteTerms {` | [services/platform/middleware/buyer/src/index.ts](../../../services/platform/middleware/buyer/src/index.ts) |
| `ts_export` | `export interface PaymentQuoteTerms {` | [services/platform/middleware/buyer/src/index.ts](../../../services/platform/middleware/buyer/src/index.ts) |
| `ts_export` | `export function platform_mw_buyer(): "sdk-quoted-receipt-verified-x402-buyer" {` | [services/platform/middleware/buyer/src/index.ts](../../../services/platform/middleware/buyer/src/index.ts) |
| `ts_export` | `export async function accountIdentifiers(account: string): Promise<readonly Uint8Array[]> {` | [services/platform/middleware/buyer/src/index.ts](../../../services/platform/middleware/buyer/src/index.ts) |
| `ts_export` | `export class BuyerMiddleware {` | [services/platform/middleware/buyer/src/index.ts](../../../services/platform/middleware/buyer/src/index.ts) |
| `ts_export` | `export interface BuyerSupportedKind {` | [services/platform/middleware/buyer/src/index.ts](../../../services/platform/middleware/buyer/src/index.ts) |
| `ts_export` | `export interface Journey {` | [services/platform/middleware/buyer/src/index.ts](../../../services/platform/middleware/buyer/src/index.ts) |
| `ts_export` | `export class LayerXPaymentHttpTransport implements ProductionTransport {` | [services/platform/middleware/buyer/src/index.ts](../../../services/platform/middleware/buyer/src/index.ts) |
| `ts_export` | `export interface MoveQuote {` | [services/platform/middleware/buyer/src/index.ts](../../../services/platform/middleware/buyer/src/index.ts) |
| `ts_export` | `export type PaidFetchResult =` | [services/platform/middleware/buyer/src/index.ts](../../../services/platform/middleware/buyer/src/index.ts) |
| `ts_export` | `export interface ParsedOffer {` | [services/platform/middleware/buyer/src/index.ts](../../../services/platform/middleware/buyer/src/index.ts) |
| `ts_export` | `export interface PreparedPayment extends PortableReceiptV2 {` | [services/platform/middleware/buyer/src/index.ts](../../../services/platform/middleware/buyer/src/index.ts) |

## middleware-conformance-env-vars

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `LAYERX_ACCOUNT, LAYERX_CURRENCY` | [services/platform/middleware/conformance/service.mjs](../../../services/platform/middleware/conformance/service.mjs) |
| `env_var` | `LAYERX_PRICE, LAYERX_ASSET, LAYERX_PAY_TO` | [services/platform/middleware/conformance/service.mjs](../../../services/platform/middleware/conformance/service.mjs) |
| `env_var` | `LAYERX_SETTLEMENT_TOKEN, LAYERX_PRICE` | [services/platform/middleware/conformance/service.mjs](../../../services/platform/middleware/conformance/service.mjs) |
| `env_var` | `LAYERX_WEBHOOK_KEYS` | [services/platform/middleware/conformance/service.mjs](../../../services/platform/middleware/conformance/service.mjs) |
| source | — | [services/platform/middleware/conformance/connected.mjs](../../../services/platform/middleware/conformance/connected.mjs) |
| source | — | [services/platform/middleware/conformance/pay6/examples.test.mjs](../../../services/platform/middleware/conformance/pay6/examples.test.mjs) |

## middleware-conformance-environment

| Kind | Declaration | Source |
|---|---|---|
| source | — | [services/platform/middleware/conformance/service.mjs](../../../services/platform/middleware/conformance/service.mjs) |

## middleware-conformance-http-routes

| Kind | Declaration | Source |
|---|---|---|
| `http_route` | `if (path === "/budget") {` | [services/platform/middleware/conformance/service.mjs](../../../services/platform/middleware/conformance/service.mjs) |
| `http_route` | `if (path === "/receipt") {` | [services/platform/middleware/conformance/service.mjs](../../../services/platform/middleware/conformance/service.mjs) |
| `http_route` | `if (path === "/signer") {` | [services/platform/middleware/conformance/service.mjs](../../../services/platform/middleware/conformance/service.mjs) |
| `http_route` | `if (request.method !== "POST" \|\| request.url !== "/settle") {` | [services/platform/middleware/conformance/service.mjs](../../../services/platform/middleware/conformance/service.mjs) |

## middleware-conformance-ts-exports

| Kind | Declaration | Source |
|---|---|---|
| `ts_export` | `export async function buildSignedReceipt(` | [services/platform/middleware/conformance/src/receipts.ts](../../../services/platform/middleware/conformance/src/receipts.ts) |
| `ts_export` | `export function buyerPayload(offer: OfferFixture, receipt: SignedReceipt, key: string): PaymentPayload` | [services/platform/middleware/conformance/src/receipts.ts](../../../services/platform/middleware/conformance/src/receipts.ts) |
| `ts_export` | `export class ConformanceSequencer` | [services/platform/middleware/conformance/src/receipts.ts](../../../services/platform/middleware/conformance/src/receipts.ts) |
| `ts_export` | `export function encodeBase64(value: Uint8Array): string` | [services/platform/middleware/conformance/src/receipts.ts](../../../services/platform/middleware/conformance/src/receipts.ts) |
| `ts_export` | `export function fixedBytes(seed: number): Uint8Array` | [services/platform/middleware/conformance/src/receipts.ts](../../../services/platform/middleware/conformance/src/receipts.ts) |
| `ts_export` | `export function offerFixture(payTo: Uint8Array, asset: Uint8Array, amount: bigint): OfferFixture` | [services/platform/middleware/conformance/src/receipts.ts](../../../services/platform/middleware/conformance/src/receipts.ts) |
| `ts_export` | `export interface OfferFixture` | [services/platform/middleware/conformance/src/receipts.ts](../../../services/platform/middleware/conformance/src/receipts.ts) |
| `ts_export` | `export interface ReceiptFacts` | [services/platform/middleware/conformance/src/receipts.ts](../../../services/platform/middleware/conformance/src/receipts.ts) |
| `ts_export` | `export interface SignedReceipt` | [services/platform/middleware/conformance/src/receipts.ts](../../../services/platform/middleware/conformance/src/receipts.ts) |
| `ts_export` | `export function toHex(value: Uint8Array): string` | [services/platform/middleware/conformance/src/receipts.ts](../../../services/platform/middleware/conformance/src/receipts.ts) |
| `ts_export` | `export async function runScenarios(): Promise<Suite>` | [services/platform/middleware/conformance/src/scenarios.ts](../../../services/platform/middleware/conformance/src/scenarios.ts) |
| `ts_export` | `export async function assertServiceAccountPayTo(account, payTo)` | [services/platform/middleware/conformance/service.mjs](../../../services/platform/middleware/conformance/service.mjs) |
| `ts_export` | `export async function runServiceScenarios(suite)` | [services/platform/middleware/conformance/service.mjs](../../../services/platform/middleware/conformance/service.mjs) |
| `ts_export` | `export const SERVICE_PAYEE_ACCOUNT = "agent:did:layerx:conformance-seller:main";` | [services/platform/middleware/conformance/service.mjs](../../../services/platform/middleware/conformance/service.mjs) |
| `ts_export` | `export async function servicePayee(account)` | [services/platform/middleware/conformance/service.mjs](../../../services/platform/middleware/conformance/service.mjs) |
| `ts_export` | `export function assert(condition: boolean, message: string): void` | [services/platform/middleware/conformance/src/support.ts](../../../services/platform/middleware/conformance/src/support.ts) |
| `ts_export` | `export interface CheckResult` | [services/platform/middleware/conformance/src/support.ts](../../../services/platform/middleware/conformance/src/support.ts) |
| `ts_export` | `export async function expectThrows(` | [services/platform/middleware/conformance/src/support.ts](../../../services/platform/middleware/conformance/src/support.ts) |
| `ts_export` | `export class FixedReceiptKeys {` | [services/platform/middleware/conformance/src/support.ts](../../../services/platform/middleware/conformance/src/support.ts) |
| `ts_export` | `export class InMemoryDeliveryStore implements WebhookDeliveryStore` | [services/platform/middleware/conformance/src/support.ts](../../../services/platform/middleware/conformance/src/support.ts) |
| `ts_export` | `export class InMemoryFulfillmentRepository<T> implements FulfillmentRepository<T>` | [services/platform/middleware/conformance/src/support.ts](../../../services/platform/middleware/conformance/src/support.ts) |
| `ts_export` | `export class Suite` | [services/platform/middleware/conformance/src/support.ts](../../../services/platform/middleware/conformance/src/support.ts) |

## middleware-examples-cli

| Kind | Declaration | Source |
|---|---|---|
| source | — | [services/platform/middleware/examples/public-rpc.mjs](../../../services/platform/middleware/examples/public-rpc.mjs) |
| source | — | [services/platform/middleware/examples/public_rpc.py](../../../services/platform/middleware/examples/public_rpc.py) |

## middleware-examples-cli-commands

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `parser.add_argument("--activity")` | [services/platform/middleware/examples/public_rpc.py](../../../services/platform/middleware/examples/public_rpc.py) |
| `cli_command` | `parser.add_argument("--claim-key")` | [services/platform/middleware/examples/public_rpc.py](../../../services/platform/middleware/examples/public_rpc.py) |
| `cli_command` | `parser.add_argument("--did", default=os.environ.get("LAYERX_DID"))` | [services/platform/middleware/examples/public_rpc.py](../../../services/platform/middleware/examples/public_rpc.py) |
| `cli_command` | `parser.add_argument("--faucet", default=os.environ.get("LAYERX_FAUCET_URL"))` | [services/platform/middleware/examples/public_rpc.py](../../../services/platform/middleware/examples/public_rpc.py) |
| `cli_command` | `parser.add_argument("--offer")` | [services/platform/middleware/examples/public_rpc.py](../../../services/platform/middleware/examples/public_rpc.py) |
| `cli_command` | `parser.add_argument("--payer")` | [services/platform/middleware/examples/public_rpc.py](../../../services/platform/middleware/examples/public_rpc.py) |
| `cli_command` | `parser.add_argument("--public-key")` | [services/platform/middleware/examples/public_rpc.py](../../../services/platform/middleware/examples/public_rpc.py) |
| `cli_command` | `parser.add_argument("--rpc", default=os.environ.get("LAYERX_RPC_URL"))` | [services/platform/middleware/examples/public_rpc.py](../../../services/platform/middleware/examples/public_rpc.py) |

## middleware-examples-env-vars

| Kind | Declaration | Source |
|---|---|---|
| source | — | [services/platform/middleware/examples/public-rpc.mjs](../../../services/platform/middleware/examples/public-rpc.mjs) |
| source | — | [services/platform/middleware/examples/examples-live-check.sh](../../../services/platform/middleware/examples/examples-live-check.sh) |

## middleware-examples-http-routes

| Kind | Declaration | Source |
|---|---|---|
| `http_route` | `url.pathname !== "/v1/faucet/claims" \|\|` | [services/platform/middleware/examples/public-rpc.mjs](../../../services/platform/middleware/examples/public-rpc.mjs) |

## middleware-merchant-ts-exports

| Kind | Declaration | Source |
|---|---|---|
| `ts_export` | `export interface CartLine {` | [services/platform/middleware/merchant/src/index.ts](../../../services/platform/middleware/merchant/src/index.ts) |
| `ts_export` | `export interface CatalogItem {` | [services/platform/middleware/merchant/src/index.ts](../../../services/platform/middleware/merchant/src/index.ts) |
| `ts_export` | `export interface CatalogProvider {` | [services/platform/middleware/merchant/src/index.ts](../../../services/platform/middleware/merchant/src/index.ts) |
| `ts_export` | `export type MerchantCheckoutResult =` | [services/platform/middleware/merchant/src/index.ts](../../../services/platform/middleware/merchant/src/index.ts) |
| `ts_export` | `export type MerchantErrorCode =` | [services/platform/middleware/merchant/src/index.ts](../../../services/platform/middleware/merchant/src/index.ts) |
| `ts_export` | `export interface MerchantMiddlewareConfig {` | [services/platform/middleware/merchant/src/index.ts](../../../services/platform/middleware/merchant/src/index.ts) |
| `ts_export` | `export type MerchantOrderState = "awaiting-payment" \| "paid-verified" \| "refused";` | [services/platform/middleware/merchant/src/index.ts](../../../services/platform/middleware/merchant/src/index.ts) |
| `ts_export` | `export interface MerchantQuote {` | [services/platform/middleware/merchant/src/index.ts](../../../services/platform/middleware/merchant/src/index.ts) |
| `ts_export` | `MerchantReceiptEvidence` | [services/platform/middleware/merchant/src/index.ts](../../../services/platform/middleware/merchant/src/index.ts) |
| `ts_export` | `export interface MerchantReceiptResolver {` | [services/platform/middleware/merchant/src/index.ts](../../../services/platform/middleware/merchant/src/index.ts) |
| `ts_export` | `export interface MerchantSellerFactory {` | [services/platform/middleware/merchant/src/index.ts](../../../services/platform/middleware/merchant/src/index.ts) |
| `ts_export` | `export interface MerchantSettlementWebhooksConfig {` | [services/platform/middleware/merchant/src/index.ts](../../../services/platform/middleware/merchant/src/index.ts) |
| `ts_export` | `export function platform_mw_merchant(): "receipt-backed-merchant-checkout" {` | [services/platform/middleware/merchant/src/index.ts](../../../services/platform/middleware/merchant/src/index.ts) |
| `ts_export` | `export interface QuotedLine extends CartLine {` | [services/platform/middleware/merchant/src/index.ts](../../../services/platform/middleware/merchant/src/index.ts) |
| `ts_export` | `export interface SettlementWebhookEvent {` | [services/platform/middleware/merchant/src/index.ts](../../../services/platform/middleware/merchant/src/index.ts) |
| `ts_export` | `export interface CheckoutOpenRequest {` | [services/platform/middleware/merchant/src/index.ts](../../../services/platform/middleware/merchant/src/index.ts) |
| `ts_export` | `export class MerchantError extends Error {` | [services/platform/middleware/merchant/src/index.ts](../../../services/platform/middleware/merchant/src/index.ts) |
| `ts_export` | `export class MerchantMiddleware {` | [services/platform/middleware/merchant/src/index.ts](../../../services/platform/middleware/merchant/src/index.ts) |
| `ts_export` | `export interface MerchantOrder {` | [services/platform/middleware/merchant/src/index.ts](../../../services/platform/middleware/merchant/src/index.ts) |
| `ts_export` | `export interface MerchantOrderStore {` | [services/platform/middleware/merchant/src/index.ts](../../../services/platform/middleware/merchant/src/index.ts) |
| `ts_export` | `export class MerchantSettlementWebhooks {` | [services/platform/middleware/merchant/src/index.ts](../../../services/platform/middleware/merchant/src/index.ts) |

## middleware-seller-error-codes

| Kind | Declaration | Source |
|---|---|---|
| `error_code` | `const safeReason = /^[a-z][a-z0-9_]{0,63}$/u.test(reason) ? reason : "payment_refused";` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |

## middleware-seller-http-routes

| Kind | Declaration | Source |
|---|---|---|
| source | — | [services/platform/middleware/seller/src/rpc.ts](../../../services/platform/middleware/seller/src/rpc.ts) |

## middleware-seller-ts-exports

| Kind | Declaration | Source |
|---|---|---|
| `ts_export` | `export const MAX_PAYMENT_ACCOUNT_BYTES = 512;` | [services/platform/middleware/seller/src/commitment.ts](../../../services/platform/middleware/seller/src/commitment.ts) |
| `ts_export` | `export const MAX_PAYMENT_CURRENCY_BYTES = 16;` | [services/platform/middleware/seller/src/commitment.ts](../../../services/platform/middleware/seller/src/commitment.ts) |
| `ts_export` | `export const MAX_PAYMENT_DID_BYTES = 255;` | [services/platform/middleware/seller/src/commitment.ts](../../../services/platform/middleware/seller/src/commitment.ts) |
| `ts_export` | `export interface PaymentCommitmentEvidence {` | [services/platform/middleware/seller/src/commitment.ts](../../../services/platform/middleware/seller/src/commitment.ts) |
| `ts_export` | `export async function verifyPaymentCommitmentEvidence(` | [services/platform/middleware/seller/src/commitment.ts](../../../services/platform/middleware/seller/src/commitment.ts) |
| `ts_export` | `export class GrantPaymentAuthority implements SellerPaymentAuthority {` | [services/platform/middleware/seller/src/grant.ts](../../../services/platform/middleware/seller/src/grant.ts) |
| `ts_export` | `export function platform_mw_seller(): "receipt-gated-x402-seller" {` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export interface PortableReceiptV2 {` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export interface SellerMiddlewareConfig<T> {` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export interface VerifiedWebhookConsumerConfig {` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export interface X402Extension {` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export function decodeLedgerTransferLog(log: LedgerLog): LedgerTransferEvent` | [services/platform/middleware/seller/src/ledger.ts](../../../services/platform/middleware/seller/src/ledger.ts) |
| `ts_export` | `export function decodePaymentDrawnLog(log: LedgerLog): PaymentDrawnEvent` | [services/platform/middleware/seller/src/ledger.ts](../../../services/platform/middleware/seller/src/ledger.ts) |
| `ts_export` | `export function decodePaymentExactLog(log: LedgerLog): PaymentExactEvent` | [services/platform/middleware/seller/src/ledger.ts](../../../services/platform/middleware/seller/src/ledger.ts) |
| `ts_export` | `export function encodeLedgerDraw(draw: LedgerDraw): Uint8Array` | [services/platform/middleware/seller/src/ledger.ts](../../../services/platform/middleware/seller/src/ledger.ts) |
| `ts_export` | `export const LAYERX_LEDGER_ADDRESS = "0x000000000000000000000000000000000000101a";` | [services/platform/middleware/seller/src/ledger.ts](../../../services/platform/middleware/seller/src/ledger.ts) |
| `ts_export` | `export class PaymentRpc extends BoundedRpc {` | [services/platform/middleware/seller/src/rpc.ts](../../../services/platform/middleware/seller/src/rpc.ts) |
| `ts_export` | `export class PaymentRpcError extends Error {` | [services/platform/middleware/seller/src/rpc.ts](../../../services/platform/middleware/seller/src/rpc.ts) |
| `ts_export` | `export function rpcHex(value: unknown, size?: number): Uint8Array {` | [services/platform/middleware/seller/src/rpc.ts](../../../services/platform/middleware/seller/src/rpc.ts) |
| `ts_export` | `export function rpcObject(value: unknown): Record<string, unknown> {` | [services/platform/middleware/seller/src/rpc.ts](../../../services/platform/middleware/seller/src/rpc.ts) |
| `ts_export` | `export async function verifyRpcPayment(` | [services/platform/middleware/seller/src/rpc.ts](../../../services/platform/middleware/seller/src/rpc.ts) |
| `ts_export` | `export function decodePaymentPayloadHeader(value: string): PaymentPayload {` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export function decodePaymentRequiredHeader(value: string): PaymentRequired {` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export function decodeSettlementHeader(value: string): SettlementResponse {` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export function encodePaymentPayloadHeader(value: PaymentPayload): string {` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export function encodePaymentRequiredHeader(value: PaymentRequired): string {` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export function encodeSettlementHeader(value: SettlementResponse): string {` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export interface FulfillmentRepository<T> {` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export interface GrantDrawExecution {` | [services/platform/middleware/seller/src/grant.ts](../../../services/platform/middleware/seller/src/grant.ts) |
| `ts_export` | `export type JsonValue = null \| boolean \| number \| string \| JsonValue[] \| { readonly [key: string]: JsonValue };` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export class MiddlewareError extends Error {` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export type MiddlewareErrorCode =` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export const PAYMENT_REQUIRED_HEADER = "PAYMENT-REQUIRED" as const;` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export const PAYMENT_RESPONSE_HEADER = "PAYMENT-RESPONSE" as const;` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export const PAYMENT_SIGNATURE_HEADER = "PAYMENT-SIGNATURE" as const;` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export function paymentAccount(extra: unknown, required = false): string \| undefined {` | [services/platform/middleware/seller/src/commitment.ts](../../../services/platform/middleware/seller/src/commitment.ts) |
| `ts_export` | `export type PaymentCommitment = "receipted" \| "committed";` | [services/platform/middleware/seller/src/commitment.ts](../../../services/platform/middleware/seller/src/commitment.ts) |
| `ts_export` | `export function paymentCommitment(extra: unknown): PaymentCommitment {` | [services/platform/middleware/seller/src/commitment.ts](../../../services/platform/middleware/seller/src/commitment.ts) |
| `ts_export` | `export interface PaymentCommitmentResolver extends PaymentValidatorResolver {` | [services/platform/middleware/seller/src/commitment.ts](../../../services/platform/middleware/seller/src/commitment.ts) |
| `ts_export` | `export function paymentCurrency(extra: unknown, required = false): string \| undefined {` | [services/platform/middleware/seller/src/commitment.ts](../../../services/platform/middleware/seller/src/commitment.ts) |
| `ts_export` | `export function paymentPayer(extra: unknown, required = false): string \| undefined {` | [services/platform/middleware/seller/src/commitment.ts](../../../services/platform/middleware/seller/src/commitment.ts) |
| `ts_export` | `export interface PaymentPayload {` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export function paymentPurpose(extra: unknown, required = false): string \| undefined {` | [services/platform/middleware/seller/src/commitment.ts](../../../services/platform/middleware/seller/src/commitment.ts) |
| `ts_export` | `export interface PaymentRequired {` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export interface PaymentRequirements {` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export class ReceiptPayloadAuthority implements SellerPaymentAuthority {` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export interface ResourceInfo {` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export type SellerDecision<T> =` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export class SellerMiddleware<T> {` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export interface SellerPaymentAuthority {` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export type SellerSettlementOutcome =` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export interface SellerSettlementRequest {` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export interface SettlementResponse {` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export interface StoredFulfillment<T> extends PortableReceiptV2 {` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export class VerifiedWebhookConsumer {` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export async function verifyPaymentCommitment(` | [services/platform/middleware/seller/src/commitment.ts](../../../services/platform/middleware/seller/src/commitment.ts) |
| `ts_export` | `export async function verifyPaymentReceipt(` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export type WebhookClaimResult = "claimed" \| "processing" \| "completed" \| "conflict";` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export type WebhookConsumeResult = "processed" \| "duplicate" \| "processing";` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export interface WebhookDeliveryClaim {` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export interface WebhookDeliveryStore {` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export interface WebhookRequestHeaders {` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
| `ts_export` | `export const X402_VERSION = 2 as const;` | [services/platform/middleware/seller/src/index.ts](../../../services/platform/middleware/seller/src/index.ts) |
