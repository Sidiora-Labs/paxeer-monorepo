# Release v6.12.0 address table

Addresses, module accounts and tokens of release v6.12.0. The plan constant is `V6120Upgrade = "v6.12.0"` in [upgrades_v6120.go](../../../node/upgrades_v6120.go). [Precompile selection](../../../node/precompiles_v6120.go) and [application wiring](../../../node/app.go) define constructors and activation.

## Precompiles

| Address | Name | Constructor | Module | Activation | Contract |
|---|---|---|---|---|---|
| `0x0000000000000000000000000000000000001015` | Perpetuals | `precompiles/perps.NewPrecompile`, selected by `exchangeVersionSwitch` | `perps`; existing exchange activation entry | Existing exchange below the v6.12.0 done height; perpetuals at and above it | [Perps](perps.md) |
| `0x0000000000000000000000000000000000001017` | Launchpad | `precompiles/launchpad.NewPrecompile`, selected by `launchpadVersionSwitch` | `launchpad` | Legacy launchpad below the done height; generation 2 at and above it | [Launchpad](launchpad.md) |
| `0x0000000000000000000000000000000000001020` | PriceFeed | `precompiles/pricefeed.NewPrecompile` | `pricefeed` | `v6120PrecompileGate` | [Pricefeed](pricefeed.md) |
| `0x0000000000000000000000000000000000001021` | PECOR market rail | `precompiles/spot.NewServedSet` | `spot` | `v6120PrecompileGate` | [Spot](spot.md) |
| `0x0000000000000000000000000000000000001022` | PECOROrders | `precompiles/spot.NewServedSet` | `spot` | `v6120PrecompileGate` | [Spot](spot.md) |
| `0x0000000000000000000000000000000000001023` | PECORStopOrders | `precompiles/spot.NewServedSet` | `spot` | `v6120PrecompileGate` | [Spot](spot.md) |
| `0x0000000000000000000000000000000000001024` | PECORQuoter | `precompiles/spot.NewServedSet` | `spot` | `v6120PrecompileGate` | [Spot](spot.md) |
| `0x0000000000000000000000000000000000001025` | PECORVault | `precompiles/spot.NewServedSet` | `spot` | `v6120PrecompileGate` | [Spot](spot.md) |
| `0x0000000000000000000000000000000000001026` | Reserved vault-share slot | None; share tokens have dynamic native pointers | `spot` | No code at this fixed slot | [Spot](spot.md) |
| `0x0000000000000000000000000000000000001027` | Reserved router slot | None | `spot` | No code at this fixed slot | [Spot](spot.md) |
| `0x000000000000000000000000000000000000100A` | Pointer view | Existing [pointer view](../../../precompiles/pointerview/pointerview.go) | `evm` | Existing activation | This table |

The v6.12.0 activation gate withholds pricefeed and spot code below the plan's done height. The exchange and launchpad retain their existing activation entries and use versioned maps to select the implementation for historical reads. `LatestUpgrade` comes from `node/tags`; the named v6.12.0 handler selects its set explicitly. [gigaV6120FailFast](../../../node/giga_v6120.go) extends `AllCustomPrecompilesFailFastMerge` with pricefeed and the five spot rails, routing calls through the ordinary execution path.

## Module accounts

Every module account address is authtypes.NewModuleAddress(name); its EVM address, the module EVM address, is the address GetEVMAddressOrDefault (modules/evm/keeper/address.go) returns for it. Every maccPerms address is a blocked receiver unless it is in allowedReceivingModAcc ([node/app.go](../../../node/app.go)).

| Account | Module | maccPerms | Holds | Contract file |
|---|---|---|---|---|
| perps | perps | none | Every USDL of the perpetuals venue as the USDL ERC20 balance of the perps module EVM address, including the balances behind perps_fees and perps_insurance; moved in by transferFrom under the user's allowance to 0x1015 and out by transfer through CallEVM | perps.md |
| perps_fees | perps | none | Fee recipient; a perps account in the perps store over the perps module EVM address balance | perps.md |
| perps_insurance | perps | none | Insurance fund; a perps account in the perps store over the perps module EVM address balance | perps.md, perps-parameters.md |
| spot | spot | {Minter, Burner}, the spot keeper refusing any mint or burn outside spot/share/* denoms; in allowedReceivingModAcc | uhpx and SID as bank balances; USDL and every registered ERC20 as ERC20 balances of the spot module EVM address; book reserves, escrows, vault pools and fee accruals are keeper records over those balances | spot.md |
| launchpad | launchpad | none (the rule of node/app.go) | authtypes.NewModuleAddress(ModuleName) (modules/launchpad/keeper/keeper.go, modules/launchpad/types/keys.go); the USDL of every pool as the USDL ERC20 balance of the launchpad module EVM address, including the first-generation pool USDL the v6.12.0 handler moves there by ERC20 transfer; no mint right | launchpad.md |
| launchpad_treasury | launchpad | none | Existing treasury account (modules/launchpad/types/keys.go), unchanged | launchpad.md |
| pricefeed | pricefeed | none | Nothing; the module keeps observations and aggregates only | pricefeed.md |

Pool addresses are ordinary accounts and never enter maccPerms. Spot fees have no account of their own: they are accruals inside the spot module account.

## Tokens

| Token | Kind | Decimals | Address | Use | Contract file |
|---|---|---|---|---|---|
| USDL | existing ERC20 contract; no bank denom, no pointer, never minted | 6 (USDL_TOKEN_DECIMALS, contracts/libraries/Constants.sol) | 0x85FcD13735F4309833A503EE804ea32395851479 (USDL_TOKEN, contracts/libraries/Constants.sol) | Launchpad quote asset in ERC20 units read by balanceOf, the only perpetuals collateral and the spot quote asset; moved by transferFrom and transfer | launchpad.md, perps.md, spot.md |
| PAX | bank denom uhpx (BaseDenom, modules/evm/keeper/params.go) | 6, the exponent of the uhpx bank metadata | ERC20 native pointer of uhpx, created by the first v6.12.0 hook | Base denom, pricefeed market 11, spot base asset; moved by bank transfer | spot.md, pricefeed.md |
| SID | bank denom layerxbridgetypes.SidioraDenom() (modules/layerxbridge/types/keys.go) | 6 (SidioraDecimals, modules/layerxbridge/types/keys.go) | ERC20 native pointer of the SID denom, created by the v6.12.0 pointer hook | Pricefeed market 12, spot base asset; receives the legacy SID balances of the SID proxy (node/upgrades_v611.go) through the SID holder migration hook of the v6.12.0 handler; moved by bank transfer | spot.md, pricefeed.md |

Pointer rule: the PAX and SID token addresses are read with getNativePointer of 0x100A (precompiles/pointerview/pointerview.go), the view over GetERC20NativePointer (precompiles/utils/expected_keepers.go), and resolved back to the denom with GetNativePointee (modules/evm/keeper/pointer.go); decimals come from the bank metadata of the denom, never from a constant in the reader. USDL has no pointer: getNativePointer of any denom never returns its address, and its decimals are the ERC20 decimals().
