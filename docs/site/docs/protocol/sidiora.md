# Sidiora

Sidiora (`SID`) is the second official coin of Paxeer X Network. It has six decimals and two implemented paths for paying gas: sponsored account execution and native fee-denom selection.

Paxeer mainnet-beta is one BFT chain, run by `paxd`, with an embedded kernel that participates in block proposal and execution. Its EVM chain id is `125` (`0x7d`), and a receipt is final when its block is committed. Public node JSON-RPC uses the root HTTPS URLs in [Public RPC endpoints](../reference/public-rpc.md). Hosted gateway methods have a separate contract in `services/platform/hosted/gateway/openrpc.json`; do not assume its hosted `px_` methods are served by every node. [Data availability](data-availability.md) explains committed data and retention.

---

## What it is

| Field | Value |
| --- | --- |
| Symbol | `SID` |
| Decimals | `6`: one SID is `1000000` base units |
| ERC-20 proxy address | `0x21f7b20a555199fa73A238B1a91FD0f549068fEe` |
| Native denom | `factory/pax1dzfx9mk4fl9kl2mysjmtvk2xp75ljumk6nynhf/usid` |
| Denom creator and admin | The `layerxbridge` module account; the subdenom is `usid` |
| Supply | Read the bank denom supply, or `totalSupply()` when the native-backed implementation is active |
| Proxy administration | The existing owner; the repository contains a proposed governance timelock handover |

The network coin is PAX. Its bank denom `uhpx` uses six decimals: one PAX is `1000000 uhpx`. EVM values and gas costs use eighteen-decimal wei units: one PAX is `10^18` wei, and one `uhpx` is `10^12` wei. `modules/evm/state/utils.go` implements that conversion. SID base units, bank `uhpx` and EVM wei must be identified explicitly when pricing a fee.

### One address, a new implementation behind it

`contracts/src/SidioraNativeERC20.sol` provides a native-backed implementation for the existing proxy. `balanceOf` reads the bank balance of the native denom, `totalSupply` reads its bank supply, and transfers call the bank precompile at `0x0000000000000000000000000000000000001001`. A failed bank call or a false result reverts the transfer.

The implementation's initialization flag, denom, name, symbol, decimals and allowances use the ERC-7201 namespace `paxeer.storage.SidioraNativeERC20`. Its allowances start empty. Upgrade authorization reads the existing OpenZeppelin Ownable namespace and requires the proxy owner. The implementation must be initialized atomically with its upgrade, after checking that the new namespace is unused in the deployed layout.

The native denom is derived from the bridge module account in both the contract and `modules/layerxbridge/types/keys.go`. `contracts/test/sidiora_pointer_proposal.json` proposes binding that denom to the existing SID address in the native-pointer registry. A proposal file does not prove that the binding or proxy upgrade has been executed.

The current tree also includes the `v6.12.0` chain upgrade in `node/upgrades_v6120.go` and SID migration helpers in `node/upgrades_v611.go`. That handler moves the denom's admin to the bridge, migrates listed holders using their actual legacy proxy balance slots, clears migrated slots, records the migration height and any unminted supply shortfall, and ensures SID bank metadata. Its pointer hook keeps an existing native pointer or creates one if absent; inspect the registry to establish the actual pointer address. The contract's `migrateLegacy` method clears a remaining legacy balance slot after a migration record exists; it does not mint bank coins. These code paths do not establish live upgrade or handover status.

### Who may mint it

For normal tokenfactory operations, minting and burning require the denom's admin. `modules/layerxbridge/keeper/sidiora.go` requires that admin and creator to be the bridge module account, checks the remote-asset mapping and writes six-decimal SID metadata. It refuses a conflicting admin, creator or asset mapping.

The bridge mints when crediting an attested bridge-in and burns on bridge-out. Chain upgrade migration is a distinct minting path: the `v6.12.0` handler uses the bank keeper to mint the legacy balances it migrates. The native-backed ERC-20 exposes bank balances and transfers; it does not give ordinary holders a minting right.

### Who may change the contract behind the address

`contracts/governance/SidioraProxyGovernanceProposal.json` proposes moving ownership to `SidioraProxyTimelock`. Passage of this text proposal authorizes a foundation handover; it does not itself execute an EVM ownership transfer. The deployed role holders and timelock parameters must be bound in the proposal before submission.

`contracts/governance/SidioraProxyTimelock.sol` restricts that administration:

- Only the chain governance authority may schedule, targeting the configured proxy's `upgradeToAndCall(address,bytes)` with zero value. Other targets, selectors, value, oversized calldata and non-canonical encodings are refused.
- The new implementation and initialization data are scheduled together. The implementation must contain code and may be neither the proxy nor the timelock.
- Only the designated executor may execute, after the delay and within the grace period. The proposal specifies a two-day delay, a one-day floor and a one-week execution window.
- Only the guardian may cancel, before the operation becomes ready.

---

## Two ways to pay gas in Sidiora

| | Sponsored | Native |
| --- | --- | --- |
| Where it runs | Delegated account contract and quote service | Chain fee checks and EVM state accounting |
| Consensus change | No new fee rule | Requires the fee-token implementation and governed parameters |
| How SID is paid | Account transfers SID to the sponsor inside its batch | Gas debit and refund use the selected denom |
| Network coin needed by the account | For PAX value sent by its calls | For PAX value sent by its transaction |

The sponsored design uses EIP-7702 delegation, an account signature and a sponsor signature. It requires the corresponding chain transaction support, deployed implementation, funded sponsor and quote service. Native payment requires the fee-token chain code to be active and SID enabled in the governed allowlist. Source code availability alone does not establish either deployment's status.

### The sponsored path

`contracts/src/BatchCallAndSponsor.sol` executes the account's batch and repays the sponsor in SID within the same transaction.

- **The account signs the batch.** The digest covers its batch nonce, the hash of its calls and the quote digest, and the signature must recover to the delegated account.
- **The sponsor signs the quote.** The quote binds chain id, the delegated account address, sponsor, token, maximum token amount, offered token amount, deadline, quote nonce and declared gas cost. Under EIP-7702, `address(this)` is the account whose code is delegated; the digest binds that account address. Both digests use ABI encoding and the Ethereum signed-message prefix for 32 bytes.
- **Replay protection covers both.** The account batch nonce advances, and quote nonces are tracked per sponsor in the account's storage. Failed execution reverts those changes with the batch.
- **The quote is bounded.** Execution rejects a passed deadline, an amount above the maximum, zero gas cost or token amount, a token other than SID, or a sponsor that is zero or the account itself.
- **The price is checked on chain.** `currentRate()` reads the implementation's immutable `rateSource`, including when called through delegated account code. The source contract's owner sets its positive rate with `setRate`; `rateUpdatedAt` and `maxRateAge` bound freshness in seconds. The expected SID amount is `ceil(gasCost * rate / 10^18)`, with `gasCost` in PAX wei and `rate` in SID base units per PAX. The offered amount must fall within the contract's five-percent spread, using the contract's integer rounding.
- **Repayment is atomic.** After executing the calls, the account transfers exactly the quoted SID amount to the sponsor. A failed call or an ERC-20 transfer that fails, returns the wrong length or returns a value other than one reverts the batch. The sponsor still bears transaction gas costs if execution reverts.

The quote service lives in `services/interop/crates/layerx-gas-station`. `src/price.rs` implements `PaymasterRateSource`: it reads `rateUpdatedAt()` and `currentRate()` from the configured paymaster through node `eth_call`. This is a served contract rate source. Missing, malformed, invalid, stale, overflowing or out-of-spread data refuses a quote; the service does not substitute a price. Configuration bounds `max_rate_age` to at most 300 seconds, `spread_bps` to at most 500 and `margin_bps` to the configured spread.

`GAS_STATION_PAYMASTER` is required and must match the configuration's `paymaster` address. `relayer_key_env` names the sponsor signing-key source. The production station also requires a positive decimal `GAS_STATION_DID_GAS_LIMIT`, the maximum sponsored gas for one DID per UTC day. It enforces that identity budget in addition to per-account, per-interval and per-quote SID limits and a sponsor balance floor. The station journal restores reservations and usage across restart.

The rate publisher in `src/rate.rs` sends owner-signed `setRate(uint256)` transactions from the rate file, journals publication before broadcast and confirms the result. Its owner key is separate from the relayer key; publication is bounded by cadence, fees, balance and daily budget. The binary exposes `paxeer-gas-station rate --config PATH --journal PATH --rate-file PATH`. A configured source still needs an authorized publisher to keep its rate fresh.

`services/agent/sdk/typescript/src/gas-station.ts` implements quote requests, quote and batch digests, EIP-7702 authorization and construction of `executeSponsored`. It validates the returned quote against the configured sponsor, token, decimals, chain and deadline, and verifies signatures before building execution calls. See [SDKs](../agents/sdks.md).

### The native path

The chain's fee-token implementation charges the fee in the account's selected denom:

- **Preference is per account.** The fee-token precompile at `0x0000000000000000000000000000000000001018` exposes `setFeeDenom(string)`, `getFeeDenom(address)` and `clearFeeDenom()`. Set and clear affect only the caller. The precompile is non-payable, rejects delegatecall and rejects state changes in static calls. Setting a preference requires the switch to be enabled and the denom to be allowed. No preference selects the network coin.
- **The fee check converts before charging.** `modules/evm/ante/fee.go` computes the gas purchase, including the applicable blob fee, and converts it to the fee denom with ceiling rounding. The payer needs sufficient spendable SID for that charge and sufficient PAX for transferred value.
- **Gas accounting uses SID.** `modules/evm/state/balance.go` intercepts gas-buy, gas-return and fee-reward balance changes. The debit rounds up, the refund rounds up in the payer's favor, and the fee credit rounds down. Transaction value still uses the network coin.
- **Cosmos fees can also use allowed denoms.** `node/antedecorators/feedenom.go` converts allowed fee tokens for the ordinary fee check and charges the offered coins. It rejects denoms that are neither fee-token allowlisted nor otherwise approved by the auth fee policy.

When the switch is off, the ordinary network-coin path remains active. `GetFeeTokenCharge` also selects that path for a stored preference whose denom is no longer allowed. For a selected, allowed fee denom, a bad or stale rate is an error rather than a substituted rate.

### How a Sidiora fee is priced

The native path reads governed `x/evm` parameters, independently of the sponsored contract's rate:

- An `allowed_fee_denoms` entry carries a denom, a positive rate in denom base units per whole PAX and `rate_update_height`.
- `max_fee_token_rate_age` bounds age in blocks. A future or negative update height, a non-positive rate or invalid age bound returns `ErrFeeTokenRateInvalid`; an expired rate returns `ErrFeeTokenRateStale`; an absent denom returns `ErrFeeTokenRateUnavailable`.
- `ConvertFeeToDenom` converts PAX wei to SID base units, dividing by `10^18`; inverse conversion uses the same rate. Callers choose ceiling or floor as required for charge, refund or credit.
- `InitialSidioraBaseUnitsPerPax` documents `3114000` base units, or 3.114 SID per PAX, as a possible governed initial rate. It does not populate or enable the allowlist. Rate updates are also bounded by `max_fee_token_spread` relative to the stored rate.

---

## The governance surface

The fee-token parameters live in the `x/evm` parameter store. Their defaults leave native SID fee payment disabled; the migration preserves values governance has already stored.

| Parameter | Default | What its validator refuses |
| --- | --- | --- |
| `fee_token_enabled` | `false` | A non-boolean value |
| `allowed_fee_denoms` | Empty | Invalid or duplicate denoms, `uhpx`, non-positive or unset rates, negative update heights |
| `max_fee_token_rate_age` | `1000` blocks | A non-integer or non-positive age |
| `max_fee_token_spread` | `0.05` | An unset, negative or non-decimal value, or a value at or above one |
| `fee_token_distribution` | `false` | A non-boolean value |

`modules/evm/keeper/abci.go` sweeps spendable allowed fee tokens from transaction coinbase accounts to the block fee destination when fee tokens are enabled, as well as sweeping the network coin. `RouteCollectedFeeTokens` then applies the collection policy:

- **Held**, the default: allowed non-base denoms at the fee collector move to the `fee_token_holding` module account. A missing holding account is an error.
- **Distributed**: fee tokens stay in the collector for its ordinary distribution path.

This policy skips the base denom. It changes the destination of collected fee tokens, independently of the payer's preference and rate calculation.

---

## Where it stands

- Sponsored payment is implemented in the account contract, gas station and SDK. Its served rate comes from the configured contract, with freshness maintained by an owner-authorized publisher and enforced DID and sponsor budgets.
- Native fee payment is implemented in the chain. Defaults are off with an empty allowlist; deployment and governed state determine whether SID can actually pay gas.
- The repository contains a pointer-binding proposal and a proxy timelock handover proposal. Neither file establishes applied state. The `v6.12.0` handler additionally implements legacy-balance migration, metadata and pointer setup; inspect committed state to determine which upgrades and bindings have taken effect.
- Fee-token collection includes the EVM end-of-block sweep and governed holding or distribution policy.

The implementation paths above are in the [repository](https://github.com/Sidiora-Labs/Paxeer-X-Network). Treat deployed contract state, the pointer registry, upgrade records and governed parameters as the evidence for activation on a particular network.
