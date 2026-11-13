<!-- Source: https://docs.paxeer.app/precompiles/ -->

# Precompiles

Call native Paxeer X capabilities from the EVM with the correct addresses, ABIs, permissions, and token units.

**Source:** `precompiles/setup.go` and each precompile's `abi.json` and Go implementation.

## Kernel and native application precompiles

Paxeer X extends the EVM with the LayerX kernel and native financial applications. The tables use shortened addresses; pad each address to 20 bytes for an EVM call.

| Address | Interface | Operations |
| --- | --- | --- |
| `0x1012` | Disabled verification surface | Calls revert with `LXP_MODULE_DISABLED` |
| `0x1013` | Disabled custody surface | Calls revert with `LXP_MODULE_DISABLED` |
| `0x1014` | Anchor reads | Read-only queries use the available anchor store. Writes revert with `LXP_MODULE_DISABLED`. Network finality is block commit |
| `0x1015` | Perpetuals | `addCollateral`, `addSize`, `closePosition`, `executeOrder`, `cancelOrder`, account, balance, and market queries; every method is nonpayable. Use `precompiles/perps/abi/abi.json` and `modules/perps` for the served venue |
| `0x1016` | Bridge | `bridgeIn`, `bridgeOut`, `getChain`, `getAttestors`, `getCap`, `isNullified`, `isPaused` |
| `0x1017` | Launchpad | Market creation, quotes, buys and sells, reserves and price snapshots, fee strategies, fee claims, burns, airdrops, and LP rewards |
| `0x1018` | Fee token | `setFeeDenom`, `getFeeDenom`, `clearFeeDenom` select an approved fee denomination |
| `0x1019` | XWeb | `request`, `fulfil`, `fulfilProgram`, `refund`, `getRequest`, `getResult`, `getProgramResult`, `getKernelNetworkId`, attestors, threshold, fee, and parameters |
| `0x1020` | Price feed | `getPrice`, `getPriceBySymbol`, `getPrices`, market and observation queries, feed-key registration and revocation |
| `0x1021`–`0x1025` | Spot (PECOR) | Market and swap rail (`0x1021`), limit orders (`0x1022`), stop orders (`0x1023`), read-only quoter (`0x1024`), and liquidity vault (`0x1025`); see the [spot exchange guide](https://docs.paxeer.app/spot-exchange) |

### Kernel constructs and agent functions

The kernel constructs and agent functions are listed in `MergeAddressTable` in `precompiles/setup.go`. They are served through registered constructors and are gated by the merge activation height. Use the ABI for the address and queried height.

| Address | Interface |
| --- | --- |
| `0x101a` | `layerxledger` |
| `0x101b` | `layerxpay` |
| `0x101c` | `layerxauth` |
| `0x101d` | `layerxprograms` |
| `0x101e` | `layerxchains` |
| `0x0901`–`0x0908` | Agent functions: `orob`, `clearing`, `oracle aggregator`, `pofq`, `scheduler`, `streams`, `tee attestor`, `eip712` |

Use each precompile's ABI for exact tuple layouts, argument types, mutability, and events. Perpetuals and pricefeed ABIs are under `precompiles/perps/abi/` and `precompiles/pricefeed/abi/`; spot ABIs are under `precompiles/spot/abi/`. Solidity interfaces for the bank, address, JSON, WASM and XWeb surfaces are in `contracts/src/precompiles/`. Verify the operation result and its committed block before treating execution as final.

### One account across execution surfaces

The address precompile also exposes `bindLayerX`, `unbindLayerX`, `layerXBindNonce`, `getLayerXDid`, `getEvmAddrByLayerX`, and `getUnifiedAccount`. The unified account returns EVM, Cosmos, DID public key, and LayerX main account identifiers. Binding requires the account signature and current nonce; applications retain each identifier in its native encoding.

Continue with [XDK](https://docs.paxeer.app/xdk) for client bindings, [settlement](https://docs.paxeer.app/settlement) for receipt verification and block finality, [bridge transfers](https://docs.paxeer.app/bridge), and [assets](https://docs.paxeer.app/assets) for denomination and pointer rules.

## Fixed addresses and interfaces

Paxeer X implements precompiles natively in the node. Call them with their matching ABI rather than expecting deployed Solidity bytecode. Match your client bindings to the node revision you use.

| Address | Capability | Current interface |
| --- | --- | --- |
| `0x1001` | Bank | Native balances, metadata, supply, pointer-authorized sends, native value transfers |
| `0x1002` | CosmWasm | `instantiate`, `execute`, `execute_batch`, `query` |
| `0x1003` | JSON | Extract bytes, byte lists, unsigned integers, and array elements |
| `0x1004` | Address association | `getPaxAddr`, `getEvmAddr`, `associate`, `associatePubKey` |
| `0x1005` | Staking | Delegation and validator transactions, queries, pagination, pool, and parameters |
| `0x1006` | Governance | `vote`, `voteWeighted`, `deposit`, `submitProposal` |
| `0x1007` | Distribution | Reward queries, withdrawals, withdrawal address, and validator commission |
| `0x1008` | Oracle | Both price-query methods revert with an explicit disabled-query error |
| `0x1009` | IBC transfer | `transfer`, `transferWithDefaultTimeout` |
| `0x100a` | Pointer lookup | Native, CW20, CW721, and CW1155 pointer address, version, and existence |
| `0x100b` | Pointer creation | `addNativePointer`, `addCW20Pointer`, `addCW721Pointer`, `addCW1155Pointer` |
| `0x100c` | Solo asset claims | `claim(bytes)`, `claimSpecific(bytes)` using signed Cosmos transaction payloads |
| `0x1011` | P-256 signatures | `verify(bytes)` |

Addresses in the table are shortened for readability. Pad them to 20 bytes, for example `0x0000000000000000000000000000000000001001`. `precompiles/common/` supplies shared gas, payment, ABI, and error handling; it is not another callable precompile.

## Address association

`getPaxAddr(address)` resolves an existing EVM-to-Paxeer association. `getEvmAddr(string)` performs the reverse lookup. Account addresses use `pax1`; validator operator addresses use `paxvaloper1`. These lookups can fail when no association exists; they are not interchangeable with a bech32 encoding of the same 20 bytes.

The manual association methods derive both addresses from a public key or signature. Use the key belonging to the account being linked. Creating a new random wallet would associate a different account. Signature construction must match the exact message bytes expected by `precompiles/addr/addr.go`. Use `layerXBindNonce` when preparing the consent message for `bindLayerX`.

## Bank balances and transfers

Bank amounts are integers in the supplied base denomination. The native bank denomination is `uhpx`: one PAX equals `1,000,000 uhpx`, while EVM transaction value uses `10^18` wei per PAX. The internal human-unit identifier is `hpx`; see [network parameters](https://docs.paxeer.app/network-parameters) for the unit mapping.

```
// Minimal read ABI; the complete interface is precompiles/bank/abi.json.
import { Contract, JsonRpcProvider, formatUnits } from 'ethers'

export async function readBankBalance(rpcUrl, account) {
const provider = new JsonRpcProvider(rpcUrl)
if ((await provider.getNetwork()).chainId !== 125n) throw new Error('Wrong chain')
const bank = new Contract(
  '0x0000000000000000000000000000000000001001',
  ['function balance(address,string) view returns (uint256)',
   'function decimals(string) view returns (uint8)'],
  provider,
)
const amount = await bank.balance(account, 'uhpx')
console.log('bank base units:', amount.toString())
console.log('PAX:', formatUnits(amount, 6))
console.log('bank denomination decimals:', await bank.decimals('uhpx')) // 0
}
```

`bank.decimals(denom)` returns `0` because the ABI represents base-denomination integers. It does not return the display exponent of PAX. The `name` and `symbol` methods require bank denomination metadata.

- `send(address from, address to, string denom, uint256 amount)` is nonpayable and restricted to the registered ERC-20 native pointer for that denomination. An arbitrary wrapper or wallet cannot call it successfully just by supplying a sender.

- `sendNative(string toAddress)` is payable, sends EVM value to a Paxeer bech32 recipient, and preserves both whole `uhpx` and the smaller wei remainder. The caller must be associated; delegatecall is rejected.

- `all_balances(address)` and `supply(string)` also expose base-denomination amounts.

## Staking and distribution

Staking writes act for the immediate caller's associated account. A contract wrapper therefore controls its own position; querying its deployer's delegation does not inspect that position. Delegatecall is rejected.

Payable staking methods receive EVM value and convert it into the native staking denomination; amount arguments and query results are denomination integers or fixed-point share and reward values whose exact scale is defined by each method's handler. Read the scale for a method in `precompiles/staking/` and `precompiles/distribution/` before converting a result for display, and keep delegation shares, pending rewards, and withdrawn amounts as separate quantities.

The staking ABI also exposes validator creation and editing; validator, delegation, unbonding, and redelegation queries; historical information; `pool`; and `params`. Read current parameters for unbonding time and entry limits. Paginated queries return a bytes cursor: with ethers, an empty cursor is `0x`, whose JavaScript string length is two.

Distribution supports `setWithdrawAddress`, `withdrawDelegationRewards`, `withdrawMultipleDelegationRewards`, `withdrawValidatorCommission`, and `rewards`. The withdrawal target must be associated. Some validator addresses are indexed strings in EVM events; an indexed string topic is a hash, not the original bech32 text.

## Governance transactions

The governance ABI exposes writes only: `vote`, `voteWeighted`, `deposit`, and `submitProposal`. Use the native module query APIs to inspect proposals and live deposit or voting parameters. Take the exact argument and message shapes from `precompiles/gov/abi.json` and `precompiles/gov/handler.go` for the node revision you target, and confirm a submission through the native governance queries rather than assuming a particular EVM event.

## JSON extraction

The JSON ABI provides `extractAsBytes(bytes,string)`, `extractAsBytesList(bytes,string)`, `extractAsUint256(bytes,string)`, and `extractAsBytesFromArray(bytes,uint16)`. Keys select a literal top-level field. To read a nested field, first extract its containing object.

Byte-list results preserve each element's JSON representation, including quotes around strings. Decode returned hex bytes before parsing JSON. Integer extraction accepts decimal integers and quoted integer strings, rejects negative or oversized values, and does not implement floating-point conversion.

```
// With json connected to the current precompiles/json/abi.json:
const document = toUtf8Bytes(JSON.stringify({ user: { count: '42' }, tags: ['one'] }))
const user = await json.extractAsBytes(document, 'user')
const count = await json.extractAsUint256(user, 'count') // 42n
const tags = await json.extractAsBytesList(document, 'tags')
const firstTag = JSON.parse(toUtf8String(tags[0])) // 'one'
```

The helper functions `toUtf8Bytes` and `toUtf8String` above are ethers exports. The implementation charges 100 gas per ABI argument byte, excluding the selector; total transaction gas includes other execution and transaction costs.

## P-256 signature verification

At `0x1011`, `verify(bytes)` takes exactly 160 payload bytes: a 32-byte digest followed by 32-byte `r`, `s`, public-key `x`, and public-key `y`. Valid signatures return ABI-encoded `bytes` containing a 32-byte word with value one. Invalid signatures return empty low-level call data rather than an ABI-encoded result; handle that case before decoding. Malformed payload length reverts.

This is an ABI call with a dynamic bytes argument. The 160-byte payload occupies 224 argument bytes after its offset and length words, so the precompile charges `224 × 300 = 67,200` precompile execution gas, before other call or transaction costs. P-256 verification is one part of a passkey flow; challenge, origin, authenticator data, and replay handling belong to the complete application ceremony.

## Cross-runtime and cross-chain calls

The [CosmWasm guide](https://docs.paxeer.app/wasm) covers matching the native coin list to payable EVM value. [WASM bindings](https://docs.paxeer.app/wasmbinding) expose calls in the other direction. The [IBC guide](https://docs.paxeer.app/interchain) covers the transfer ABI, which does not expose channel queries.

Pointer lookup returns the pointer address, a version, and an existence flag for one supplied native denomination or CosmWasm address. It does not provide a list-all query. Solo claims consume signed Cosmos transaction payloads; they are asset-claim operations rather than a general administrative interface.

## Oracle query behavior

`getExchangeRates` and `getOracleTwaps` at `0x1008` revert with `oracle precompile is retired; oracle data queries are disabled`. See the [Oracle module guide](https://docs.paxeer.app/modules/oracle) for native module queries. The EVM pricefeed read interface is at `0x1020`.

## ABI and failure handling

Use the matching `precompiles/<name>/abi.json`, simulate the real caller and value, and inspect revert data before sending a transaction. Module-backed precompiles can meter native execution dynamically; kernel constructs and the perpetuals venue use method gas tables. Use the gas policy of the specific precompile. An off-chain read needs no funded signer, while an on-chain call still consumes gas.

In particular, address lookup is `getPaxAddr` in this tree. An SDK package targeting another chain can carry different selectors, chain IDs, prefixes, or addresses even when its API looks similar.
