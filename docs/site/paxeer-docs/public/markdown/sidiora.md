<!-- Source: https://docs.paxeer.app/sidiora/ -->

# Sidiora

The SID asset on Paxeer X: native balances, its stable ERC-20 address, bridge-controlled supply, and sponsored or native gas payments.

## Asset Identity

Sidiora is the second official coin of Paxeer X. Its native denomination and ERC-20 interface represent one asset, giving wallets, contracts and Cosmos applications a consistent balance model.

| Property | Value |
| --- | --- |
| Symbol | `SID` |
| Decimals | `6`; one SID is `1,000,000` base units |
| ERC-20 address | `0x21f7b20a555199fa73A238B1a91FD0f549068fEe` |
| Native denom | `factory/pax1dzfx9mk4fl9kl2mysjmtvk2xp75ljumk6nynhf/usid` |
| Mint and burn authority | The `x/layerxbridge` module account |
| Proxy administration | Governance through a restricted upgrade timelock |

PAX uses the base denom `uhpx` and eighteen decimals. Keep SID and PAX scaling separate in displays, approvals, transfer amounts, fee quotations and accounting. Read supply from the native bank state or ERC-20 `totalSupply` for a current value.

## One Asset Across Native and EVM Interfaces

The native ERC-20 implementation reads `balanceOf` from the bank balance of the SID denom and `totalSupply` from its bank supply. Transfers move that denom through the bank precompile. A failed bank operation reverts the contract call. The pointer record resolves the denom to the same stable ERC-20 address.

The proxy address remains constant through implementation upgrades. Native token metadata and allowances use an ERC-7201 storage namespace that preserves legacy implementation slots. Applications query current allowances before building spending transactions and request the approval required by the current interface.

### Bridge-Controlled Supply

The bridge module account administers the tokenfactory denom. Attested bridge-in credits mint SID and bridge-out operations burn the departing amount. Denom creation verifies the bridge-derived creator, expected admin and remote asset mapping. Ordinary holders and application contracts have no minting authority.

## Two Ways to Pay Gas in SID

| Property | Sponsored gas | Native fee preference |
| --- | --- | --- |
| Execution | EIP-7702 account batch and quote service | Chain fee processing |
| SID payment | Atomic transfer to the sponsor inside the batch | Fee debit in SID before execution |
| Pricing | Signed quote checked against the paymaster rate | Governance-controlled conversion rate |
| Replay protection | Account batch nonce and sponsor quote nonce | Normal transaction replay protection and account preference |
| Unused gas | Governed by the signed sponsorship quote | Refunded in the selected fee denom |
| PAX transaction value | The account still supplies value it sends | The account still supplies value it sends |

## Sponsored Transaction Workflow

1. Build the application calls and request a sponsor quote for the declared gas cost.
2. Verify the configured sponsor, SID token, six-decimal units, chain ID, deadline, quoted amount and maximum.
3. Sign the account batch, covering its nonce, calls hash and quote digest, and authorize the EIP-7702 execution.
4. Execute through `BatchCallAndSponsor`; the account runs the calls and repays the sponsor in SID in the same transaction.

The sponsor signature binds chain ID, paymaster, sponsor, token, maximum amount, offered amount, deadline, quote nonce and declared gas cost. The account signature must recover to the account itself. Used quote nonces and account batch nonces prevent replay.

### Pricing and Atomicity

The paymaster checks a positive, sufficiently fresh rate and requires the offered amount to fall within five percent of the amount implied by the declared gas cost. Expired quotes, zero amounts, an incorrect token or invalid sponsor are refused. A failed application call or SID repayment reverts the whole batch.

Quote services apply their own budget and rate-freshness limits before signing a quote. Missing or invalid pricing inputs produce a refusal. Validate the returned quote and signatures locally before building `executeSponsored`.

## Native Fee Preferences

The fee-token precompile at `0x0000000000000000000000000000000000001018` exposes the account preference:

```
function setFeeDenom(string calldata denom) external;
function getFeeDenom(address account) external view returns (string memory denom);
function clearFeeDenom() external;
```

Call `setFeeDenom` with the SID native denom to select it for the caller, query with `getFeeDenom`, or clear the preference to return to the network coin. Only allowed fee denoms can be selected. The precompile rejects value and delegatecall; static calls cannot change preferences.

### Charging and Refunding

For EVM transactions, fee processing converts the maximum gas charge, including a blob fee when applicable, into the chosen denom. The account must have sufficient spendable SID for that charge and sufficient PAX for any transferred value. The fee debit replaces the PAX gas purchase.

Unused gas returns in the same denom. Cosmos transactions can offer allowed fee denoms, whose governed conversion is used for minimum-gas-price checks.

## Governed Rates and Fee Routing

Fee conversion uses a governance-controlled rate for each allowed denom. Only allowed fee denoms can be selected through the fee-token precompile. Applications read the current fee parameters before presenting a fee option, and re-estimate when a rate or policy changes.

PAX fees keep their ordinary collection path through the fee collector.

## Upgrade Authority

The Sidiora proxy timelock accepts only governance-scheduled `upgradeToAndCall(address,bytes)` operations targeting the proxy with zero value. The implementation must be a contract distinct from the proxy and timelock. Upgrade and initialization execute together.

A designated executor runs an operation after its delay and within its grace period. A guardian can cancel before readiness. The handover governance proposal specifies a minimum delay of 172,800 seconds (two days), a delay floor of 86,400 seconds (one day) and an execution grace period of 604,800 seconds (one week). These delays are governance and constructor parameters, not constants in the token contract. These controls bound implementation changes while preserving the token address.

## Integration Checklist

- Use six-decimal SID amounts and query the current balance and allowance.

- Resolve the native denom and stable ERC-20 address as the same asset.

- Display the signed sponsorship amount or estimated native fee in SID before approval.

- Distinguish gas from PAX value sent by the transaction.

- Refresh rates and quotes before submission and preserve transaction evidence for accounting.

Explore [Unified Network](https://docs.paxeer.app/unified-network), [Contracts](https://docs.paxeer.app/contracts), [Precompiles](https://docs.paxeer.app/precompiles) and [xweb](https://docs.paxeer.app/xweb), which accepts SID for agent web requests.
