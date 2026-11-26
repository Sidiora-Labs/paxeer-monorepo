# Perps agent kit

`@paxeer-network/agent-kit` exposes perpetual trading reads and writes as TypeScript tools and a Model Context Protocol (MCP) server over standard input and output. It holds only an agent Ed25519 seed. The wallet gateway holds the agent wallet's transaction signing authority.

## Run

From the repository root:

```sh
pnpm --dir apps/perps install --frozen-lockfile
pnpm --dir apps/perps start:agent
```

Configure the process environment before starting:

| Variable                        | Meaning                                                                                                                   |
| ------------------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| `PAXEER_AGENT_GATEWAY_URL`      | Wallet gateway HTTP(S) base URL serving `/v1/agent`                                                                       |
| `PAXEER_AGENT_NODE_URL`         | Chain JSON-RPC HTTP(S) endpoint                                                                                           |
| `NEXT_PUBLIC_PERPS_GATEWAY_URL` | Trading data gateway bare HTTP(S) origin; uses the web application's `/graphql` queries                                   |
| `PAXEER_AGENT_KEY_FILE`         | Path to a UTF-8 file containing exactly a 32-byte Ed25519 seed as 64 hex characters, with optional surrounding whitespace |
| `PAXEER_AGENT_LABEL`            | Agent identity label, default `perps`; letters, digits, `_` and `-`, at most 128 characters                               |

An MCP client can launch `node --import tsx packages/agent-kit/src/mcp-cli.ts` with its working directory set to `apps/perps` and these environment variables. Stdout carries MCP messages only. The server loads configuration at startup and authenticates lazily when a gateway operation needs a read token. Listing tools does not contact the network.

Library imports use the same `NEXT_PUBLIC_PERPS_GATEWAY_URL` configuration as the backend builders. Set it before importing the library. `PerpsDataClient` also takes explicit `nodeUrl` and `dataUrl` options.

## Tools and calls

Tool monetary inputs are exact decimal strings, without units or exponent notation. USDL inputs accept up to six decimal places; USD notionals, prices and leverage accept up to eighteen. IDs are uint256 decimal strings. Chain read results retain raw units; MCP serializes bigint values to decimal strings. Leverage is an ordinary multiple in tool inputs (`"2"` means 2x).

`P` below is the perps precompile address returned by the backend builders (`0x0000000000000000000000000000000000001015`). `U` is the backend's configured USDL token. All writes use the existing backend builders and plans.

| Tool                  | Read source or wallet gateway route                                                   | Contract and function                                                                          |
| --------------------- | ------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| `markets`             | Trading `/graphql`, then node `eth_call`                                              | P: `getMarketConfig`, `getMarginTiers`, `getMarkPriceDetails`; indexed mark and funding data   |
| `account`             | `/v1/agent/me`, then node `eth_call`                                                  | P: `getAccount`; includes collateral, equity, initial/maintenance margin and withdrawable USDL |
| `positions`           | `/v1/agent/me`, then node `eth_call`                                                  | P: `getUserPositionIds`, `getPosition`, `getPositionDetails`, `getPositionMargin`              |
| `open_orders`         | `/v1/agent/me`, then node `eth_call`                                                  | P: `getUserOrderIds`, `getOrder`; returns active orders                                        |
| `history`             | `/v1/agent/me`, then trading `/graphql`                                               | The web application's trades and closed-position queries, with `limit` and `offset`            |
| `deposit`             | `/v1/agent/simulate`, then `/v1/agent/send` or `/v1/agent/actions/allowance-and-call` | P: `deposit(U, amount)`; U: `approve(P, amount)` when allowance is short                       |
| `withdraw`            | `/v1/agent/simulate`, then `/v1/agent/send`                                           | P: `withdraw(U, amount)`                                                                       |
| `open_market`         | Node quote, `/v1/agent/simulate`, then `/v1/agent/send`                               | P: `quoteOpenPosition`, then `placeOrder` MARKET IOC                                           |
| `place_limit`         | `/v1/agent/simulate`, then `/v1/agent/send`                                           | P: `placeOrder` LIMIT                                                                          |
| `place_stop_limit`    | `/v1/agent/simulate`, then `/v1/agent/send`                                           | P: `placeOrder` STOP_LIMIT                                                                     |
| `cancel_order`        | `/v1/agent/simulate`, then `/v1/agent/send`                                           | P: `cancelOrder(orderId)`                                                                      |
| `set_tpsl`            | Node mark read, `/v1/agent/simulate`, then `/v1/agent/send`                           | P: `setPositionTpSl(positionId, takeProfit, stopLoss)`                                         |
| `add_collateral`      | `/v1/agent/simulate`, then `/v1/agent/send` or `/v1/agent/actions/allowance-and-call` | P: `addCollateral(positionId, amount)`; U: `approve(P, amount)` when allowance is short        |
| `remove_collateral`   | `/v1/agent/simulate`, then `/v1/agent/send`                                           | P: `removeCollateral(positionId, amount)`                                                      |
| `close_position`      | Node quote, `/v1/agent/simulate`, then `/v1/agent/send`                               | P: `quoteClosePosition`, then `closePositionWithParams` with full-close size 0                 |
| `partial_close`       | Node quote, `/v1/agent/simulate`, then `/v1/agent/send`                               | P: `quotePartialClose`, then `closePositionWithParams` with the computed USD size              |
| `action_status`       | GET `/v1/agent/actions/:action_id`                                                    | Durable approval/call status                                                                   |
| `transaction_receipt` | GET `/v1/agent/tx/:hash`                                                              | Submitted transaction receipt                                                                  |

Every tool has a strict input schema and a description available through MCP `tools/list`. Read tools are marked read-only; write tools are marked destructive and non-idempotent. A withdrawal goes from the trading account into the agent wallet. The owner uses the owner sweep route to collect those funds.

## Connect from the workspace

Select an agent on the portfolio Agents page to copy its MCP client configuration
and follow the three connection steps. Replace `<path-to-agent-key-file>` with the
path of that agent's seed file on the client machine; the page does not read the
file or receive its contents. Keep the selected DID's label and key together.

The web application serves `/agents/interface.json` as public JSON containing the
package name and version, launch command, environment names, tool descriptions
and JSON input schemas, completed with the workspace's public wallet gateway,
node, data gateway, chain and venue contract configuration. It needs no session
cookie. The address-free package description is available without configuring a
gateway or loading a key:

```ts
import { describeAgentInterface } from "@paxeer-network/agent-kit/interface";
const description = describeAgentInterface();
```

Orders use deposited trading collateral. Limits and stop limits require a nonzero bytes16 `clientOrderId` and support `GTC`, `IOC`, `FOK` and `POST_ONLY`. Market orders use a fresh on-chain quote, IOC, and the backend's 0.5% slippage bound and 60-second deadline. TP/SL prices default to zero for new orders; zero clears a side in `set_tpsl`. Partial closes use `fractionBps` strictly between 0 and 10000, where 2500 closes a quarter.

Before a trading request is signed, chain configuration checks enforce lot size, tick size, market and notional-tier leverage, enabled markets for new orders, and the configured open-interest ceiling for individual order notional. Bound refusals name the bound and carry its raw value. Position and order actions check wallet ownership. The gateway also simulates calls and applies its owner policy and margin checks.

The normal path simulates the exact call and sends it. When a backend plan requires approval, the kit simulates the exact approval and submits its token, spender, amount and builder-derived function signature/arguments to the durable action route. The gateway confirms approval, simulates the final call, sends it and tracks confirmation. Supply a stable `idempotencyKey` for deposits and collateral additions. Persist the returned `action_id` and resume with `action_status`; reuse an intent key only for the same durable action. Direct send operations have no automatic retry. After an ambiguous send outcome, inspect activity/receipts before submitting another transaction.

Results have `{ ok: true, data }` or `{ ok: false, error: { code, message, details } }`. Gateway refusal codes, including durable action error envelopes, reach the agent unchanged. An accepted action with `terminal: false` is pending. A transaction hash is submission evidence; use `transaction_receipt` to check confirmation. MCP sets `isError` on refusals.

## New key to first position

1. Create the agent's key file outside the repository with owner-only permissions. For example, with `PAXEER_AGENT_KEY_FILE` set:

   ```sh
   node --input-type=module -e 'import {randomBytes} from "node:crypto"; import {writeFileSync} from "node:fs"; writeFileSync(process.env.PAXEER_AGENT_KEY_FILE, randomBytes(32).toString("hex"), {mode:0o600,flag:"wx"})'
   ```

2. Register the key through the gateway's challenge/verify flow. The client derives `did:matrix:<label>:<first-16-public-key-hex>` and signs the exact `paxeer-agent-auth:<did>:<nonce>` challenge. Its writes sign the SDK-compatible `PXW:AGENT-REQUEST:v1` digest over method/path, DID, random nonce, expiry and body hash. The read token authorizes gateway reads and simulation; only signed requests authorize writes.

   ```ts
   const { AgentGatewayClient } = await import("@paxeer-network/agent-kit");
   const { readFile } = await import("node:fs/promises");
   const gateway = new AgentGatewayClient({
     url: process.env.PAXEER_AGENT_GATEWAY_URL!,
     label: process.env.PAXEER_AGENT_LABEL ?? "perps",
     privateKey: Buffer.from(
       (await readFile(process.env.PAXEER_AGENT_KEY_FILE!, "utf8")).trim(),
       "hex",
     ),
   });
   await gateway.authenticate();
   // Give gateway.did and its public key to the owner claiming the agent.
   ```

3. In an owner-authenticated application, claim the DID using `POST /v1/agents/:did/claim` and the expiring claim signature from `sdk/wallet`'s `buildClaim`. The owner session stays in the owner application. Call `gateway.provision()` to provision the agent wallet; when the wallet gateway requests a binding signature, use the SDK's agent-key `signBinding` for the returned chain/address/nonce and supply that signature to `gateway.provision(signature)`. Continue only when provisioning reports active.

4. The owner calls `buildTradingPreset(caps)` with `ownerAddress`, raw bigint `maxTxValueWei`, `maxDailyValueWei`, `maxApproveWei`, `budgetWei`, and integer `expiresInSeconds` and `rateLimitPerMin`. It returns `policy`, `rules`, `budget` and `approvalSpenders`, derived from the write builders. Apply these through the owner policy, rules and budget routes under `/v1/agents/:did`. The preset selects `trade_only`, disables native transfers, lists only the owner's wallet for withdrawal, includes the trading contracts/selectors and exact perps approval spender, and gives the budget an expiry. Value caps are in the gateway's raw wei units; USDL approval/budget values use USDL base units. Owners can freeze/unfreeze, inspect activity and sweep through those same owner routes.

5. Fund the agent wallet through owner `POST /v1/agents/:did/fund` with USDL and enough native gas. Check the effective policy and allowance before depositing. The gateway returns the policy's approval/withdrawal decision; continue only after that decision permits the deposit. Call `deposit` with `{ "amount": "100", "idempotencyKey": "first-deposit" }`. If it returns an action, poll until confirmed. Read `account` to confirm trading collateral.

6. Read `markets` to choose an enabled market and its current limits. For a market ID of 1, call `open_market` with `{ "marketId": "1", "isLong": true, "collateralAmount": "100", "leverage": "2" }`. Check the returned receipt, then read `positions` and `account`. The gateway can refuse the operation according to the owner's caps and rules.

7. Use `set_tpsl`, `partial_close` or `close_position` as needed. After confirmation, `withdraw` releases withdrawable trading collateral into the agent wallet; the owner can sweep it to the owner wallet and freeze the agent.

The agent process never receives an owner wallet key or owner session token. Startup diagnostics contain no key content or key-file path.
