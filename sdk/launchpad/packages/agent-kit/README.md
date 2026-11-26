# Launchpad agent kit

`@kindle/agent-kit` reads launchpad data through `@kindle/sdk` and submits calls
through the agent routes of the wallet gateway. The gateway holds the agent's
transaction-signing wallet. This package holds only an Ed25519 agent identity
seed, signs gateway requests, and never accepts an owner's wallet key.

Build from the launchpad workspace:

```sh
pnpm --dir apps/launchpad install --frozen-lockfile --ignore-scripts
pnpm --dir apps/launchpad -r --filter @kindle/agent-kit... run build
pnpm --dir sdk/launchpad/packages/agent-kit run test
```

## Tools and routes

All tool results contain `ok` and either `data` or `error`. Gateway refusals retain
their error code, HTTP status and response details, including action IDs and retry
instructions. Values in raw units are decimal strings. Quality, coverage,
watermarks, null prices and unknown cost basis remain visible.

| Tool                    | Route                                                                                                |
| ----------------------- | ---------------------------------------------------------------------------------------------------- |
| `list_coins`            | `GET /v2/pools` (`sort`, `limit`, `cursor`)                                                          |
| `search_coins`          | `GET /v2/search` (`q`, type `pool`)                                                                  |
| `coin`                  | `GET /v2/pools/{poolAddress}`                                                                        |
| `trades`                | `GET /v2/pools/{poolAddress}/trades`                                                                 |
| `candles`               | `GET /v2/pools/{poolAddress}/candles`                                                                |
| `holders`               | `GET /v2/pools/{poolAddress}/holders`                                                                |
| `balances`              | `GET /v1/agent/balances?tokens=…`                                                                    |
| `positions`             | `GET /v1/agent/me`, then `GET /v3/wallets/{agentWallet}/portfolio`                                   |
| `quote`                 | Public pool detail, `GET /v1/agent/me`, then `POST /v1/agent/simulate`                               |
| `buy`, `sell`, `launch` | `POST /v1/agent/simulate`, then `POST /v1/agent/send` or `POST /v1/agent/actions/allowance-and-call` |
| `action`                | `GET /v1/agent/actions/{action_id}`                                                                  |
| `agent_identity`        | `GET /v1/agent/me`                                                                                   |
| `provision_agent`       | `POST /v1/agent/provision`                                                                           |
| `claim_agent`           | Local claim signature for the owner's `POST /v1/agents/{did}/claim`                                  |
| `preset`                | Local policy, rules and budget for the owner to apply                                                |

`coin` includes the pool and token, raw spot price, circulating market value and
total-supply valuation. Graduation progress is `null` with `progressStatus`
`unavailable`. Balances and positions belong to the authenticated agent wallet.

## Calls and quote protection

The exported `DEPLOYMENT` and `EXECUTION_CALLS` are shared by calls and the preset.
They use chain 125 and the web application's deployment:

| Contract                       | Address                                            | Function                                                                                                                     |
| ------------------------------ | -------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| Router                         | `0x78FB06f525cD964388f2305A489035263E31C6e4`       | `buy(address,uint256,uint256,uint256)`, `sell(address,uint256,uint256,uint256)`, `createMarket(string,string,uint8,address)` |
| Quoter                         | `0x793006f4F321b8863F26DBD75b11bAa01DA181dc`       | `quoteExactInput(address,uint256,bool)`                                                                                      |
| ProtocolConfig                 | `0xF3669eDc475Cb6E1F0d7635240ABD8e8FB7E20C6`       | `creationFee()`                                                                                                              |
| Launchpad precompile / spender | `0x0000000000000000000000000000000000001017`       | `homeChainOf(address)`; ERC-20 approval spender                                                                              |
| USDL or the pool token         | USDL: `0x85fcd13735f4309833a503ee804ea32395851479` | `approve(address,uint256)`                                                                                                   |

Router, quoter and protocol-config ABIs come from `sidiora-launchpad-v3`. The home
chain fragment and imported function shapes are checked against the web sources.

`quote` takes `pool`, `side` and `amountInRaw`. It returns a wallet-bound ID valid
for 30 seconds in this process. `buy` and `sell` require that ID, the same pool and
amount, `maxSlippageBps`, and a unique `idempotencyKey`. Quotes cannot be reused or
imported from another process. The minimum output is
`floor(outputRaw * (10000 - maxSlippageBps) / 10000)`, must be positive, and the
deadline is 120 seconds after transaction preparation. The maximum is 5000 bps;
above 500 bps requires `acceptHighSlippage: true`, and impact above 1000 bps
requires `acceptHighImpact: true` for that trade. The recipient is the agent wallet.

Pool `homeChain` is the registry number: zero means chain 125, and other values
map to `125000000 + homeChain`. Foreign pools are refused as `foreign_home_chain`
before authenticating or signing a write request. The gateway chain and the
on-chain `homeChainOf` result are checked as well.

With sufficient allowance to the spender, the kit simulates the exact call and
sends it with a fresh request signature. Otherwise it simulates the exact approval
and submits a durable allowance-and-call action. The gateway confirms approval,
simulates the call, then broadcasts it. Simulation evidence lasts at most ten
seconds before submission. The approval amount equals the intended input or
creation fee; it is never unlimited. `launch` uses the standard zero optical,
fee strategy 0–3, and the one-USDL protocol fee. A first purchase is a separate buy.

An action response is acceptance, not confirmation. Poll `action` until terminal.
Keep its action ID and the original operation ID. A transaction response contains
the gateway's transaction result; confirmation can be read through
`GET /v1/agent/tx/{hash}`. After an uncertain send, inspect the gateway before
starting another operation. The kit reserves operation IDs in memory before
dispatch and does not automatically retry writes.

## MCP environment

The MCP entry uses standard input and output. Standard output contains protocol
messages only. All tools publish JSON schemas, descriptions and read/write hints.

| Variable                | Value                                                                                       |
| ----------------------- | ------------------------------------------------------------------------------------------- |
| `KINDLE_GATEWAY_URL`    | Wallet gateway HTTPS origin (HTTP is accepted for loopback development).                    |
| `KINDLE_DATA_URL`       | Public launchpad data HTTPS origin.                                                         |
| `KINDLE_AGENT_KEY_FILE` | Path to a private JSON file containing `seedHex` (a 32-byte hex seed) and optional `label`. |

Use mode `0600` for the key file. The seed is read from the file, never from a
command-line argument, and is never printed. The default label is `launchpad`.
The gateway DID is `did:matrix:<label>:<first 16 hex characters of public key>`.
Request signatures use `PXW:AGENT-REQUEST:v1`, length-prefixed method/path and key
ID, a random 16-byte nonce, unsigned 64-bit expiry and the SHA-256 body digest.
Reads authenticate with challenge/verify and a short-lived read token; writes
carry the Ed25519 signature headers. The token is renewed through a new challenge
when it expires.

Launch the entry with these variables set:

```sh
node sdk/launchpad/packages/agent-kit/dist/mcp.js
# Or from the workspace:
pnpm --dir apps/launchpad agent-kit
```

## From a new key to a first buy

1. Create a private agent key file. This command writes a seed and prints only
   its public DID. Set `KINDLE_AGENT_KEY_FILE` to your chosen file path first.

   ```sh
   node --input-type=module <<'JS'
   import { randomBytes } from 'node:crypto';
   import { writeFileSync } from 'node:fs';
   import { agentDid } from './sdk/launchpad/packages/agent-kit/dist/index.js';
   const seed = randomBytes(32);
   writeFileSync(process.env.KINDLE_AGENT_KEY_FILE,
     JSON.stringify({ seedHex: seed.toString('hex'), label: 'launchpad' }) + '\n',
     { mode: 0o600, flag: 'wx' });
   console.log(agentDid(seed));
   JS
   ```

2. Configure the gateway and data origins in the MCP client's environment and
   start the entry. Call `agent_identity` to authenticate through challenge and
   verify, then `provision_agent`. The tool checks and signs the gateway's binding
   message for chain 125 and the returned agent wallet, and continues provisioning.
   Poll provisioning as needed; continue only once it reports an active wallet.
3. Call `claim_agent` with the signed-in owner's user ID. Within 60 seconds the
   owner submits its `expires` and `signature` to
   `POST /v1/agents/{did}/claim` using the owner's session.
4. The owner chooses caps and calls `preset`, supplying `ownerWallet`,
   `maxTxValueWei`, `maxDailyValueWei`, `maxApproveWei`, `budgetCapWei`, and
   `expiresInSeconds`. For selling, include the selected pool's token in
   `sellTokens`. Apply `policy` with `PUT /v1/agents/{did}/policy`, each rule
   with `POST /v1/agents/{did}/rules`, and `budget` with
   `POST /v1/agents/{did}/budgets`. Only the owner applies or replaces these rules.
5. The owner funds the wallet through `POST /v1/agents/{did}/fund`: native PAX for
   gas, and USDL for buys or creation. Call `balances` with the USDL token address
   to inspect funding. The owner retains freeze, unfreeze and sweep controls at
   `/v1/agents/{did}/freeze`, `/unfreeze` and `/sweep`.
6. Call `list_coins` or `search_coins`, inspect `coin`, then request `quote` with
   side `buy` and a raw USDL amount (`1000000` is one USDL). Within 30 seconds call
   `buy` with that quote ID, amount, pool, maximum slippage and operation ID.
   Poll any returned action until terminal and read balances and positions.

For programmatic use, construct `LaunchpadAgentKit` with `gatewayUrl`, `dataUrl`
and `privateKey: Uint8Array`; optional `fetch` and `now` provide transport and clock
integration. Use `invokeTool(kit, name, input)` for the same result envelope as MCP,
or call typed methods directly. `launchpadPreset(caps)` produces owner payloads
without submitting them.
