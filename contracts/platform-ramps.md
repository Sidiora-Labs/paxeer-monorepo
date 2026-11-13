# Independent ramp contract

The [reference ramp](../services/platform/ramps/reference/src/main.rs) and [toolkit](../services/platform/ramps/toolkit/src/lib.rs) support an independent operator exchanging external funds for assets on Paxeer. The operator is an ordinary principal with configured inventory, provider and compliance integrations. The toolkit grants no minting power or protocol custody authority. Every customer presentation carries the exact label:

> External custody: this independent market maker controls the off-platform funds and payout.

Paxeer mainnet-beta is one BFT network, chain ID **125** (`0x7d`), whose kernel participates in block proposal. A receipt is final when its block commits. The historical `layerx` and `paxeer` configuration sections name client and operator-inventory adapters; they do not describe two execution and settlement networks. See [payment middleware](platform-middleware.md), [the gateway](platform-gateway.md) and [hosted services](platform-hosted.md).

## Ramp reference HTTP routes

The current reference service dispatches these routes:

| Route | Boundary |
| --- | --- |
| `GET /livez` | Local process liveness |
| `GET /readyz` | **Local journal** readiness, chain ID, custody label and contract/build metadata |
| `POST /v1/orders` | Authenticated customer order creation |
| `GET /v1/orders/{order_digest}` | Customer-authorized order status |
| `POST /v1/provider-callbacks` | Verified, order-bound provider callback |
| `POST /internal/v1/work` | Operator-controlled workflow work |
| `POST /internal/v1/rebalances` | Operator-only inventory rebalance |
| `GET /internal/v1/rebalances/{idempotency_key}` | Operator rebalance status |
| `GET /internal/v1/journal` | Operator journal health |
| `POST /internal/v1/journal/recover` | Verified recovery under operator authorization |
| `POST /internal/v2/source-settlements` | Authenticated source-settlement intake when configured |

`/readyz` reports `readiness_scope: "local-journal"`; a 200 does not prove that identity, compliance, provider or chain dependencies are available. Mutating routes refuse a journal that requires recovery. Customer identity is obtained through the configured identity service; a request cannot select the operator's wallet, signer, vault or custody operation.

## Ramp reference config keys

The strict `Config` in the reference source owns `listen`, `listener`, `journal_path`, `worker_id`, `lease_seconds`, `reconcile_seconds`, `operator`, `quotes`, listener TLS inputs, `client_tls`, `identity`, `compliance`, `provider`, `layerx`, `paxeer`, optional `source_settlement`, `provider_callback_public_key` and `operator_control_token_file`. Unknown fields are rejected. Use the source validators for bounds and required combinations.

Identity configuration is `endpoint`, `service_token_file`, `audience`. Compliance uses `endpoint`, `service_token_file`, `public_key`. The provider uses `endpoint`, `credential_file`, `settlement_path`, `status_path`; callback trust is configured separately. Paths and schemas at these external boundaries are operator-supplied contracts, not invented public Paxeer routes.

<a id="ramps-config-keys"></a>
<a id="ramp-layerx-config"></a>
<a id="ramp-toolkit-config-keys"></a>

## Chain and external boundary configuration

[LayerxConfig](../services/platform/ramps/toolkit/src/clients.rs) uses `rpc_endpoint`, `rpc_trust_anchor_der`, `chain_id`, `operator_address`, `receipt_keys`, `max_fee_per_gas`, `max_priority_fee_per_gas`, `gas_limit_cap`, `signer_endpoint`, `signer_token_file` and `actor_did`. On mainnet use chain ID 125 and a root node RPC origin, with the matching TLS trust anchor. Receipt keys include the key identity, public key and applicable height range. Priority fee cannot exceed maximum fee, the gas cap must be nonzero, and the configured receipt-key list must be nonempty.

Set `rpc_endpoint` to the configured node's HTTPS root origin. Supply receipt-key pins, TLS trust and external service origins in a private deployment configuration. [config.example.json](../services/platform/ramps/config.example.json) shows the configuration shape.

The `paxeer` section binds the operator's `custody_endpoint`, `custody_credential_file`, `broadcast_path`, `status_path`, `operator_account`, `wallet_address`, `vault_id`, `signer_key_handle` and `poll_cadence_seconds`. These are operator inventory controls; a customer's order cannot override them. The declared external versions are `layerx-ramp-compliance-v1`, `layerx-ramp-provider-v1` and `layerx-ramp-paxeer-v1`. The external integration schemas and workflow are described in
[OPERATIONS.md](../services/platform/ramps/OPERATIONS.md).

## Ramp toolkit Rust pub items

`RampOrder::bind` binds an authenticated customer, operator configuration and operator-owned quote into an `OrderDigest`. Direction, principals/accounts, asset, exact integer amounts, rational rate, fees, slippage, payout/provider tokens, expiry, grant and context participate in the binding. Requests select an order and quote; they do not supply trusted money or identity fields.

[evm.rs](../services/platform/ramps/toolkit/src/evm.rs) implements the chain leg using EIP-1559 transactions and the ledger precompile at **`0x000000000000000000000000000000000000101a`**. The on-ramp uses `payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)`; the off-ramp uses `draw(bytes32,bytes32,uint128,bytes32,uint64,bytes32,bytes32)` under the customer's payer grant. Locally verified receipts must bind the exact raw transaction, order accounts, asset, amount and context. The [engine](../services/platform/ramps/toolkit/src/engine.rs), [clients](../services/platform/ramps/toolkit/src/clients.rs) and [migration intake](../services/platform/ramps/toolkit/src/migration.rs) own workflow, external transport and source-settlement verification respectively.

Public aggregate statuses are `pending`, `unknown`, `refused`, `manual_review`, `reversed` and `done`. An unknown submission is reconciled against its recorded transaction or provider idempotency identity rather than blindly resubmitted. `done` requires the workflow's verified chain leg and, for an off-ramp, settled external payout. A provider reversal can remove `done`. The status is a ramp workflow outcome; inspect the receipt's rung to distinguish signed evidence from block commitment.

## Ramp toolkit file formats

[journal.rs](../services/platform/ramps/toolkit/src/journal.rs) defines an append-only hash-chained JSONL journal with current domain `LXP/market-maker-ramp/journal/v2\0`. It persists bound orders, stages, work leases, planned side effects, raw transactions/hashes, receipt evidence, provider callbacks and rebalance observations. A stage such as `layerx_submission_planned` or `provider_submission_planned` precedes the financial side effect. Startup replay and recovery preserve unknown outcomes.

The implementation recognizes legacy journal material for inspection; this does not authorize v1 writers. Preserve its exclusive writer lock, fsync ordering, integrity checks and recovery checks. A truncated or inconsistent journal must not be replaced with a fresh successful state.

## Ramp reference error codes

The reference dispatcher returns typed JSON errors. Examples include `callback_invalid` (400), `journal_busy` (503) and `journal_recovery_required` (503). The toolkit's `RampError` distinguishes order binding, payer-grant requirements, receipt verification, conflicts, lease ownership, illegal transitions, compliance, provider, chain and configuration failures. Preserve these distinctions when reporting or retrying; an unavailable provider is not proof of a refused or settled payout.

## Ramp reference Rust pub items

The executable exposes `platform_ramp_toolkit` metadata from the toolkit and runs the configured request listener plus reconciler. The library types are the reusable integration boundary; the reference service's private `Config`, `State` and HTTP dispatcher are not a generic provider SDK. No exported type proves that a provider sandbox or a custody integration is deployed.

## Ramp reference CLI commands

The reference binary takes a protected configuration filename:

```sh
layerx-reference-ramp /run/ramp/config.json
```

It also accepts `--verify-receipt CONFIG.json ORDER.json TRANSACTION_HASH`; that command reads the bound order, resolves the configured chain receipt and refuses unverified results. The dedicated toolkit verifier below has its own flag and exit-code contract.

<a id="ramp-toolkit-cli-commands"></a>

## Ramp verify receipt

[ramp-verify-receipt.rs](../services/platform/ramps/toolkit/src/bin/ramp-verify-receipt.rs) takes exactly these flag/value pairs in this order:

```sh
ramp-verify-receipt --config-file /run/ramp/verifier.json --order-file /run/ramp/order.json --transaction-hash 0xTRANSACTION_HASH --output-file /run/ramp/proof.json
```

Replace the transaction placeholder with 64 hexadecimal digits after `0x`. The verifier configuration contains `client_tls` and `layerx`.
`client_tls.timeout_seconds` must be from 1 through 120. Protected inputs must meet file ownership, link, size and permission checks. The output parent must be private and owned by the caller, and output creation is exclusive with mode `0600`.

Exit codes are **0** for verified output, **2** for input/configuration/output-open refusal, **3** for pending, **4** for unknown, **5** for refused or invalid proof, and **6** for output write failure. Success requires authenticated committed inclusion and receipt re-verification
against the bound order; successful output has `rung: "committed"`. The output records order/transaction/receipt identity, height, rung, chain ID, custody label and optional build identity; it is not a claim that an external payout settled.

<a id="ramp-reference-env-vars"></a>
<a id="ramp-toolkit-env-vars"></a>
<a id="ramp-container-environment"></a>
<a id="ramps-env-vars"></a>

## Environment and deployment

`LAYERX_RAMP_BUILD_REVISION` and `LAYERX_RAMP_BUILD_SOURCE_DIGEST` are compile-time provenance fields read through `option_env!`. Service runtime settings come from the protected JSON configuration. [env-init.sh](../services/platform/ramps/env-init.sh) and [deployment.yaml](../services/platform/ramps/deployment.yaml) define the container bootstrap mapping; they do not create trusted providers, keys or nodes. [sandbox-journey.sh](../services/platform/ramps/sandbox-journey.sh) is qualification tooling, not a production order API. Keep mounted credentials private and keep signer key handles distinct from secret key material.
