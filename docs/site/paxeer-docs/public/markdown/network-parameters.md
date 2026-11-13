<!-- Source: https://docs.paxeer.app/network-parameters/ -->

# Network Parameters

Chain identity, denominations, source defaults, and governance controls for Paxeer mainnet-beta.

## Unified execution and application parameters

Paxeer mainnet-beta uses EVM chain ID 125. The kernel runs inside `paxd`, and a receipt is final when its block is committed. The address precompile’s `getUnifiedAccount` returns an EVM address, associated Cosmos address, bound DID public key, and kernel main account identifier. An absent association returns an empty Cosmos address; an absent DID binding returns zero key and account fields. See the [network model](https://docs.paxeer.app/unified-network).

| Parameter family | Controlled behavior |
| --- | --- |
| Bridge | Chain registry, attestor threshold, per-asset caps, and pause state |
| Launchpad | Quote token, virtual reserves, creation fee, fee bounds, decay, volatility and concentration weights, and protocol fee share |
| Fee tokens | Approved fee denominations, rate age, spread limit, enablement, and distribution policy |
| XWeb | Attestors, signature threshold, request fee, delivery limits, and pause controls |
| Tenant domains | `modules/layerxchains` namespaces state within the shared network. Creating a domain does not require new node infrastructure. |

Governance writes remain subject to each module’s active handlers and authority checks. Bridge registration, launchpad configuration, fee-token policy, and XWeb settings have distinct validation rules. Consensus commits establish finality for the integrated network.

`docs/system/chain-modules.md` in the monorepo lists parameter fields and `docs/system/governance.md` lists proposal types. Query the target network’s active values for financial decisions. See [native interfaces](https://docs.paxeer.app/precompiles), [asset units](https://docs.paxeer.app/assets), and [bridge controls](https://docs.paxeer.app/bridge).

## Chain Identifiers

Use these identifiers when configuring a wallet or client. Confirm the connected node's chain ID before signing a transaction.

| Parameter | Value |
| --- | --- |
| EVM chain ID | `125` / `0x7d` |
| Consensus chain ID | `hyperpax_125-1` |
| Account prefix | `pax1…` |
| Validator operator prefix | `paxvaloper1…` |
| Binary / default home | `paxd` / `~/.paxeer` |
| Node source | `daemon/paxd/` and `node/` at the monorepo root |
| Module manifest | `go.mod` at the monorepo root |

```
# Set EVM_RPC_URL to the node JSON-RPC URL and CONSENSUS_NODE to its consensus RPC URL.
curl -sS "$EVM_RPC_URL" -H 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
paxd status --node "$CONSENSUS_NODE"
```

Public node JSON-RPC uses the root path. The expected `eth_chainId` result is `0x7d`. Public node namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `test` and `txpool`; availability depends on node configuration. Compare `eth_blockNumber` across configured nodes when selecting one near the head. See [Public RPC](https://docs.paxeer.app/public-rpc).

**Source:** `tools/hpx/publish.sh`, `node/params/config.go`, `node/const.go`, `rpc/info.go`.

## Native Denominations

The native asset pays EVM gas. Node parameters name its human unit `hpx`. Cosmos CLI and bank APIs use `uhpx` for base-unit amounts. EVM clients use 18-decimal native amounts.

```
1 hpx = 1,000,000 uhpx
1 uhpx = 1,000,000,000,000 EVM wei
1 hpx = 1,000,000,000,000,000,000 EVM wei
```

Use `uhpx` amounts for Cosmos CLI transactions and 18-decimal native amounts for EVM transactions. Address association joins the supported wallet representations; see [address precompiles](https://docs.paxeer.app/precompiles).

**Source:** `node/params/config.go` — `UhpxExponent`; `modules/evm/state/utils.go` — `UhpxToSweiMultiplier`.

## Query Current Parameters

Query the connected network for its active governance, staking, slashing, and consensus parameters. Cosmos queries use the consensus RPC endpoint; EVM requests use the EVM JSON-RPC endpoint. The examples use `CONSENSUS_NODE` for the CLI RPC URL and `CONSENSUS_HTTP_URL` for the consensus HTTP origin.

```
paxd query staking params --node "$CONSENSUS_NODE"
paxd query slashing params --node "$CONSENSUS_NODE"
paxd query gov params --node "$CONSENSUS_NODE"
curl -sS "${CONSENSUS_HTTP_URL:?Set the consensus HTTP origin}/consensus_params"
```

| Default parameter | Value | Meaning |
| --- | --- | --- |
| Unbonding time | 21 days | Default staking unbonding duration |
| Unbonding / redelegation entries | 7 | Limit for an individual delegation pair |
| Standard governance period | 2 days | Default deposit and voting periods |
| Expedited governance period | 1 day | Default expedited voting duration |
| Governance minimum deposit | 10,000,000 base units | Query the active governance deposit requirement |
| Expedited minimum deposit | 20,000,000 base units | Query the active governance deposit requirement |
| Slashing fractions | 0 by default | Configured by chain governance |

Governance approval requires a yes-vote ratio strictly greater than the configured threshold after abstentions are excluded. Validator jailing and equivocation handling apply alongside the configured slashing fractions.

**Source:** `sdk/x/staking/types/params.go`, `sdk/x/gov/types/params.go`, `sdk/x/gov/keeper/tally.go`, `sdk/x/slashing/types/params.go`.

## Gas and Block Limits

Query `eth_gasPrice`, `eth_feeHistory`, and `eth_maxPriorityFeePerGas` for EVM fee information. The node also has local admission prices and simulation limits. Consensus `block.max_bytes` is a block limit; `mempool.max-tx-bytes` is a separate transaction limit.

See [JSON-RPC](https://docs.paxeer.app/json-rpc) and [Configuration](https://docs.paxeer.app/configuration) for the distinct EVM, consensus, and node settings.

## Performance and Endpoints

Track block interval using block timestamps, throughput using transactions per block, and RPC latency using request metrics. Record the node revision and workload when comparing measurements.

Use the RPC endpoint supplied by your node operator. The [HPX distribution and peer registry](https://docs.paxeer.app/admin-hpx) supplies chain metadata and peer information for its configured network. The hosted gateway `px_` API is separate from public node JSON-RPC. Obtain its URL and credentials from the operator.

## Kernel Integration

The node executes EVM transactions and kernel activities within one BFT network. `modules/lxphost/` connects kernel execution to the chain. See [Paxeer and LayerX](https://docs.paxeer.app/paxeer-vs-layerx) for the integrated architecture.
