# Launchpad contract: the launchpad precompile at 0x1017 in release v6.12.0

This page describes the launchpad of the Paxeer X Network using the contracts in `contracts/launchpad` for the manifest-active generation 2, served by the precompile at 0x1017 and its surfaces at the generation 2 role addresses and every pool address. USDL stays the ERC20 contract and the launchpad holds it in custody at the launchpad module EVM address through nested ERC20 calls; pool tokens stay the deployed SidioraERC20 contracts; opticals, ERC721 fee rights and the Treasury are served as the contracts define them; and the v6.12.0 migration brings every generation 1 and generation 2 pool into generation 2.

Source paths refer to this repository. The launchpad backend binds the precompile by manifest role.


## 1. Scope and addresses

| Role | Generation 1 (retired, section 21) | Generation 2 (served by 0x1017) |
|---|---|---|
| SidioraFactory | `0x8a1A09CEe72c1D39dF33B8284E38baeF8371f465` (sdk/launchpad/protocol/addresses.go) | `0x3993D004166738d1069B5C4e0a923f7B9D5D8830` (sdk/launchpad/shared/config/deployment/current.json) |
| Router | `0xCC7298801112682e10ee14b8a520309caD80336d` (sdk/launchpad/protocol/addresses.go) | `0x78FB06f525cD964388f2305A489035263E31C6e4` (sdk/launchpad/shared/config/deployment/current.json) |
| Quoter | `0xB768e183b6EfDeDf8b2AA7af732039D1C3c452d0` (sdk/launchpad/protocol/addresses.go) | `0x793006f4F321b8863F26DBD75b11bAa01DA181dc` (sdk/launchpad/shared/config/deployment/current.json) |
| FeesRouter | `0x02Df12a44F2658080E76fbcF7D6B34Baa97843b6` (sdk/launchpad/protocol/addresses.go) | `0x05681dA6f611C2dfFD3E92611F6dE2d247647927` (sdk/launchpad/shared/config/deployment/current.json) |
| FeeAccumulator | `0x50C69dF6637b3DCE6a7407C5A4b4F99E68514A76` (sdk/launchpad/protocol/addresses.go) | `0x7b14808b72f61664cDb79D25E6d1587655571a66` (sdk/launchpad/shared/config/deployment/current.json) |
| PoolRegistry | `0x7684382c89f79104574D8EF9b31eFf2eD2C2BA0b` (sdk/launchpad/protocol/addresses.go) | `0x1dC357d2B7B850332a91F06A9E66634F46fC4Ba7` (sdk/launchpad/shared/config/deployment/current.json) |
| ProtocolConfig | `0xEeDF5409cFD30bd14D0399318c7d2150265575e5` (sdk/launchpad/protocol/addresses.go) | `0xF3669eDc475Cb6E1F0d7635240ABD8e8FB7E20C6` (sdk/launchpad/shared/config/deployment/current.json) |
| SidioraNFT | `0xDF73b354ed9dcB473cc9D01541c46f507591e190` (sdk/launchpad/protocol/addresses.go) | `0xe8DcC844a8006dbf64f5Bb851bCad90292a6409B` (sdk/launchpad/shared/config/deployment/current.json) |
| Treasury | `0x15405D535ce533BfFb98c83e42f4DD242AA5e079` (sdk/launchpad/protocol/addresses.go) | read from SidioraFactory slot 6 (contracts/launchpad/contracts/core/SidioraFactory.sol), recorded in modules/launchpad/keeper/testdata/v6120/layout.json |
| OpticalRegistry | `0x4CdA6e48632d51Ee4Fa735D81BF09F7543f644a1` (sdk/launchpad/protocol/addresses.go) | `0x0FB5E510372Bd3EddFED16A5Dd766d4473CffcF0` (sdk/launchpad/shared/config/deployment/current.json), retained contract (section 22) |
| EventEmitter | `0x0E10286EE51F99c666CDcAb52451e58AbdA4048F` (sdk/launchpad/protocol/addresses.go), keeps its code | `0x622E1bb90E0E2B6E6e5e9e2DbE1555389E8A3EF6` (sdk/launchpad/shared/config/deployment/current.json), retained contract (section 22) |
| PoolBeacon | `0xf11f08afe33e020Cab22bCaffBbAfC471c75E9d4` (sdk/launchpad/protocol/addresses.go), keeps its code | `0x0c60Ea33B1DD602A17E1EF1875eE912F3fad71BE` (sdk/launchpad/shared/config/deployment/current.json), retained contract (section 22) |

USDL: `0x85FcD13735F4309833A503EE804ea32395851479` (sdk/launchpad/shared/config/deployment/current.json) stays the ERC20 contract with its code and storage; there is no bank denom, no mint and no native pointer for it. The launchpad holds its USDL as the ERC20 balance of the launchpad module EVM address (GetEVMAddressOrDefault, modules/evm/keeper/address.go, of authtypes.NewModuleAddress(ModuleName), modules/launchpad/keeper/keeper.go; spec/release-v6-12-0/contracts/address-table.md, module accounts) (section 5).

- The launchpad is served at 0x0000000000000000000000000000000000001017 (modules/launchpad/types/keys.go), registered at precompiles/setup.go.
- Ten contract roles are served with their Launch-x ABIs: SidioraPool 42 functions, SidioraFactory 23, SidioraNFT 37, Router 23, Quoter 19, FeesRouter 20, FeeAccumulator 54, PoolRegistry 22, ProtocolConfig 27 and Treasury 17 (contracts/launchpad/contracts/protocol/Treasury.sol); the SidioraERC20 file (19 functions) encodes nested calls to the pool-token contracts, which stay deployed (the pool entry point at contracts/launchpad/contracts/core/SidioraPool.sol).
- Direct calls to 0x1017 resolve through abi.json: 113 role selectors in Router, SidioraFactory, Quoter, PoolRegistry, FeesRouter, FeeAccumulator, ProtocolConfig order, followed by 13 chain-launch selectors, for 126 methods. The first role wins for the 21 shared signatures (precompiles/launchpad/run.go).
- Every market reads generation 2 after the upgrade: the generation 2 role addresses of sdk/launchpad/shared/config/deployment/current.json serve every pool, those of generation 1 (sdk/launchpad/protocol/addresses.go) are retired (requirement 2.36), and markets created after the upgrade join the same generation 2 set.
- Fee rights are ERC721 tokens of the generation 2 collection 0xe8Dc…409B (sdk/launchpad/shared/config/deployment/current.json); generation 1 rights are imported under new ids from nextTokenId with an id map, and markets created after the upgrade mint from nextTokenId (base contracts/launchpad/contracts/base/ERC721Base.sol).
- Pool tokens stay the deployed SidioraERC20 contracts with their code and storage; new markets deploy one by CREATE2 from the generation 2 SidioraFactory (contracts/launchpad/contracts/core/SidioraFactory.sol).

## 2. Interface by role

Every function, event and custom error of each role, with its selector (function and error: 4 bytes; event: topic 0). The ABI golden file precompiles/launchpad/testdata/abi_golden.txt records the served role tables and chain-launch extension. The retained EventEmitter and optical contract tables describe their Solidity interfaces.

### SidioraPool

42 functions, 6 events, 19 errors.

| Function | Selector | Mutability | Returns | Declared |
|---|---|---|---|---|
| `PAUSER_ROLE()` | `0xe63ab1e9` | view | `(bytes32)` | contracts/launchpad/contracts/base/Pausable.sol |
| `SNAPSHOT_INTERVAL()` | `0x6d2beef1` | view | `(uint256)` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `SNAPSHOT_MIN_VOLUME_BPS()` | `0x923795a3` | view | `(uint256)` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `acceptGuardian()` | `0xa5f31d61` | nonpayable | `()` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `accumulatedTokenFees()` | `0x1e5a0998` | view | `(uint256)` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `accumulatedUsdlFees()` | `0x61f63bf5` | view | `(uint256)` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `creationTimestamp()` | `0xb3311086` | view | `(uint256)` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `cumulativeBuyUsdlVolume()` | `0x1aa6f435` | view | `(uint256)` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `cumulativeSellUsdlVolume()` | `0x00a8b4fe` | view | `(uint256)` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `cumulativeVolume()` | `0xf4dd4156` | view | `(uint256)` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `currentFeeBps()` | `0x975f8f46` | view | `(uint256)` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `emergencyPaused()` | `0x27c830a9` | view | `(bool)` | contracts/launchpad/contracts/base/Pausable.sol |
| `eventEmitter()` | `0x9ff78c30` | view | `(address)` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `factory()` | `0xc45a0155` | view | `(address)` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `feeAccumulator()` | `0x5eaea35f` | view | `(address)` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `getEffectiveReserves()` | `0x6e7f1e46` | view | `(uint256,uint256)` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `getPoolInfo()` | `0x60246c88` | view | `((address,address,uint256,uint256,uint256,uint256,uint256))` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `getPrice()` | `0x98d5fdca` | view | `(uint256)` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `getPriceSnapshots()` | `0x680aef93` | view | `(uint256[8])` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `getReserves()` | `0x0902f1ac` | view | `(uint256,uint256,uint256)` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `guardian()` | `0x452a9320` | view | `(address)` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `initialize(address,address,address,address,address,address,address,uint256,uint256)` | `0x9bb725be` | nonpayable | `()` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `lastSnapshotTimestamp()` | `0x6f660492` | view | `(uint256)` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `migrateFactory(address)` | `0x308efad5` | nonpayable | `()` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `opticalAddress()` | `0xa2d02927` | view | `(address)` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `pause()` | `0x8456cb59` | nonpayable | `()` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `paused()` | `0x5c975abb` | view | `(bool)` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `pendingGuardian()` | `0x762c31ba` | view | `(address)` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `priceSnapshots(uint256)` | `0xcbd47f88` | view | `(uint256)` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `protocolConfig()` | `0xf5efbb4f` | view | `(address)` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `realUsdlBalance()` | `0xa8c98bde` | view | `(uint256)` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `snapshotCount()` | `0x098ab6a1` | view | `(uint256)` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `snapshotIndex()` | `0x234feaf2` | view | `(uint256)` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `swap(uint256,uint256,bool,address,uint256)` | `0x704c4536` | nonpayable | `(uint256)` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `swapFor(uint256,uint256,bool,address,address,uint256)` | `0x0602dc5b` | nonpayable | `(uint256)` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `syncReserves()` | `0x5249f13e` | nonpayable | `(uint256,uint256)` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `tokenAddress()` | `0x9d76ea58` | view | `(address)` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `tokenReserve()` | `0xcbcb3171` | view | `(uint256)` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `transferGuardian(address)` | `0x091954cd` | nonpayable | `()` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `unpause()` | `0x3f4ba83a` | nonpayable | `()` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `usdlAddress()` | `0x2e10f976` | view | `(address)` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `virtualUsdlReserve()` | `0xe87ca810` | view | `(uint256)` | contracts/launchpad/contracts/core/SidioraPool.sol |

| Event | Topic 0 | Declared |
|---|---|---|
| `FactoryMigrated(address)` | `0xdc152171996ab31ea044d1f083998178a0515971a850651a133f8d13027bb277` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `GuardianTransferStarted(address,address)` | `0x112d7ce5d0717de1e8d403be44596423dc61e85438d8ba49eaf0d31b1992bce0` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `GuardianTransferred(address,address)` | `0x9fe65984b511eb7998b3cd481cfd80c807fcc7f28721a790d33c87dc9c879485` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `Initialized(uint64)` | `0xc7f505b2f371ae2175ee4913f4499e1f2633a7b5936321eed1cdaeb6115181d2` | contracts/launchpad/contracts/base/Initializable.sol |
| `InputAdjustment(uint256,uint256)` | `0xc6dfd6050e713ed5ff35fd38843c7c60822498a3d56ae5dc062e563a5b157d5c` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `PauseToggled(bool)` | `0x9077d36bc00859b5c3f320310707208543dd35092cb0a0fe117d0c6a558b148b` | contracts/launchpad/contracts/base/Pausable.sol |

| Error | Selector | Declared |
|---|---|---|
| `AlreadyInitialized()` | `0x0dc149f0` | contracts/launchpad/contracts/base/Initializable.sol |
| `DeadlineExpired()` | `0x1ab7da6b` | contracts/launchpad/contracts/interfaces/ISidioraPool.sol |
| `DivisionByZero()` | `0x23d359a3` | contracts/launchpad/contracts/libraries/SidioraMath.sol |
| `InsufficientInput()` | `0xf8b3bb61` | contracts/launchpad/contracts/interfaces/ISidioraPool.sol |
| `InsufficientLiquidity()` | `0xbb55fd27` | contracts/launchpad/contracts/interfaces/ISidioraPool.sol |
| `InvalidBalance()` | `0xc52e3eff` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `InvalidConfiguration()` | `0xc52a9bd3` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `InvalidHookResponse()` | `0x1e048e1d` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `NotInitializing()` | `0xd7e6bcf8` | contracts/launchpad/contracts/base/Initializable.sol |
| `NotPaused()` | `0x6cd60201` | contracts/launchpad/contracts/base/Pausable.sol |
| `Overflow()` | `0x35278d12` | contracts/launchpad/contracts/libraries/SidioraMath.sol |
| `Paused()` | `0x9e87fac8` | contracts/launchpad/contracts/base/Pausable.sol |
| `ReentrancyGuardReentrantCall()` | `0x3ee5aeb5` | contracts/launchpad/contracts/base/ReentrancyGuard.sol |
| `SlippageExceeded()` | `0x8199f5f3` | contracts/launchpad/contracts/interfaces/ISidioraPool.sol |
| `TransferFailed()` | `0x90b8ec18` | contracts/launchpad/contracts/libraries/TransferHelper.sol |
| `Unauthorized()` | `0x82b42900` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `VirtualFloorBreached()` | `0x04af467f` | contracts/launchpad/contracts/core/SidioraPool.sol |
| `ZeroAddress()` | `0xd92e233d` | contracts/launchpad/contracts/interfaces/ISidioraPool.sol |
| `ZeroAmount()` | `0x1f2a2005` | contracts/launchpad/contracts/interfaces/ISidioraPool.sol |

### SidioraFactory

23 functions, 6 events, 16 errors.

| Function | Selector | Mutability | Returns | Declared |
|---|---|---|---|---|
| `DEFAULT_ADMIN_ROLE()` | `0xa217fddf` | view | `(bytes32)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `ROUTER_ROLE()` | `0x30d643b5` | view | `(bytes32)` | contracts/launchpad/contracts/core/SidioraFactory.sol |
| `createMarket(string,string,uint8,address)` | `0x57c0d7fd` | nonpayable | `(address,address,uint256)` | contracts/launchpad/contracts/core/SidioraFactory.sol |
| `createMarketFor(address,string,string,uint8,address)` | `0x5db85915` | nonpayable | `(address,address,uint256)` | contracts/launchpad/contracts/core/SidioraFactory.sol |
| `eventEmitter()` | `0x9ff78c30` | view | `(address)` | contracts/launchpad/contracts/core/SidioraFactory.sol |
| `feeAccumulator()` | `0x5eaea35f` | view | `(address)` | contracts/launchpad/contracts/core/SidioraFactory.sol |
| `getNonce(address)` | `0x2d0335ab` | view | `(uint256)` | contracts/launchpad/contracts/core/SidioraFactory.sol |
| `getRoleAdmin(bytes32)` | `0x248a9ca3` | view | `(bytes32)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `grantRole(bytes32,address)` | `0x2f2ff15d` | nonpayable | `()` | contracts/launchpad/contracts/base/AccessControl.sol |
| `hasRole(bytes32,address)` | `0x91d14854` | view | `(bool)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `initialize(address,address,address,address,address,address,address,address,address)` | `0xf5e95acb` | nonpayable | `()` | contracts/launchpad/contracts/core/SidioraFactory.sol |
| `nftContract()` | `0xd56d229d` | view | `(address)` | contracts/launchpad/contracts/core/SidioraFactory.sol |
| `poolBeacon()` | `0xc5a7b2ea` | view | `(address)` | contracts/launchpad/contracts/core/SidioraFactory.sol |
| `poolRegistry()` | `0xafcff50f` | view | `(address)` | contracts/launchpad/contracts/core/SidioraFactory.sol |
| `predictPoolAddress(address,address,address)` | `0x6992a1f5` | view | `(address)` | contracts/launchpad/contracts/core/SidioraFactory.sol |
| `protocolConfig()` | `0xf5efbb4f` | view | `(address)` | contracts/launchpad/contracts/core/SidioraFactory.sol |
| `proxiableUUID()` | `0x52d1902d` | view | `(bytes32)` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `registeredDefaultAdminCount()` | `0xc0cb71b6` | view | `(uint256)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `renounceRole(bytes32,address)` | `0x36568abe` | nonpayable | `()` | contracts/launchpad/contracts/base/AccessControl.sol |
| `revokeRole(bytes32,address)` | `0xd547741f` | nonpayable | `()` | contracts/launchpad/contracts/base/AccessControl.sol |
| `treasury()` | `0x61d027b3` | view | `(address)` | contracts/launchpad/contracts/core/SidioraFactory.sol |
| `upgradeToAndCall(address,bytes)` | `0x4f1ef286` | payable | `()` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `usdlAddress()` | `0x2e10f976` | view | `(address)` | contracts/launchpad/contracts/core/SidioraFactory.sol |

| Event | Topic 0 | Declared |
|---|---|---|
| `Initialized(uint64)` | `0xc7f505b2f371ae2175ee4913f4499e1f2633a7b5936321eed1cdaeb6115181d2` | contracts/launchpad/contracts/base/Initializable.sol |
| `MarketCreated(address,address,address,uint256,address)` | `0xde3c5cd27ce87044b7bc6c8182270f302e537f55f8c943271a34e5163ef2b284` | contracts/launchpad/contracts/interfaces/ISidioraFactory.sol |
| `RoleAdminChanged(bytes32,bytes32,bytes32)` | `0xbd79b86ffe0ab8e8776151514217cd7cacd52c909f66475c3af44e129f0b00ff` | contracts/launchpad/contracts/base/AccessControl.sol |
| `RoleGranted(bytes32,address,address)` | `0x2f8788117e7eff1d82e926ec794901d17c78024a50270940304540a733656f0d` | contracts/launchpad/contracts/base/AccessControl.sol |
| `RoleRevoked(bytes32,address,address)` | `0xf6391f5c32d9c69d2a47ea670b442974b53935d1edc7fd64eb21e047a839171b` | contracts/launchpad/contracts/base/AccessControl.sol |
| `Upgraded(address)` | `0xbc7cd75a20ee27fd9adebab32041f755214dbc6bffa90cc0225b39da2e5c2d3b` | contracts/launchpad/contracts/base/ERC1967Utils.sol |

| Error | Selector | Declared |
|---|---|---|
| `AlreadyInitialized()` | `0x0dc149f0` | contracts/launchpad/contracts/base/Initializable.sol |
| `DuplicateToken()` | `0x464e3f6a` | contracts/launchpad/contracts/interfaces/ISidioraFactory.sol |
| `InsufficientCreationFee()` | `0x535b7470` | contracts/launchpad/contracts/interfaces/ISidioraFactory.sol |
| `InvalidImplementation()` | `0x68155f9a` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `InvalidRoleAccount()` | `0xeccda526` | contracts/launchpad/contracts/base/AccessControl.sol |
| `LastDefaultAdmin()` | `0x9c88180d` | contracts/launchpad/contracts/base/AccessControl.sol |
| `MissingRole(address,bytes32)` | `0x0161a64a` | contracts/launchpad/contracts/base/AccessControl.sol |
| `NonPayableUpgrade()` | `0x93738d49` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `NotInitializing()` | `0xd7e6bcf8` | contracts/launchpad/contracts/base/Initializable.sol |
| `TransferFailed()` | `0x90b8ec18` | contracts/launchpad/contracts/libraries/TransferHelper.sol |
| `TransferFromFailed()` | `0x7939f424` | contracts/launchpad/contracts/libraries/TransferHelper.sol |
| `UUPSNotThroughActiveProxy()` | `0x45a93fc9` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `UUPSNotThroughProxy()` | `0x4f083858` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `UnauthorizedUpgrade()` | `0x3a617a54` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `UpgradeInitializationFailed()` | `0xcc1cf440` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `ZeroAddress()` | `0xd92e233d` | contracts/launchpad/contracts/interfaces/ISidioraFactory.sol |

### SidioraERC20

19 functions, 2 events, 7 errors. Not served by the precompile: these are the functions of the deployed pool-token contracts, which keep their code and storage and which the precompile reaches by nested calls (requirement 2.12).

| Function | Selector | Mutability | Returns | Declared |
|---|---|---|---|---|
| `DOMAIN_SEPARATOR()` | `0x3644e515` | view | `(bytes32)` | contracts/launchpad/contracts/base/ERC20Base.sol |
| `PERMIT_TYPEHASH()` | `0x30adf81f` | view | `(bytes32)` | contracts/launchpad/contracts/base/ERC20Base.sol |
| `allowance(address,address)` | `0xdd62ed3e` | view | `(uint256)` | contracts/launchpad/contracts/base/ERC20Base.sol |
| `approve(address,uint256)` | `0x095ea7b3` | nonpayable | `(bool)` | contracts/launchpad/contracts/base/ERC20Base.sol |
| `balanceOf(address)` | `0x70a08231` | view | `(uint256)` | contracts/launchpad/contracts/base/ERC20Base.sol |
| `bindPool(address)` | `0x5a73fae2` | nonpayable | `()` | contracts/launchpad/contracts/core/SidioraERC20.sol |
| `decimals()` | `0x313ce567` | view | `(uint8)` | contracts/launchpad/contracts/base/ERC20Base.sol |
| `deploymentFactory()` | `0x8ea261cc` | view | `(address)` | contracts/launchpad/contracts/core/SidioraERC20.sol |
| `getPastBalance(address,uint256)` | `0x0df86999` | view | `(uint256)` | contracts/launchpad/contracts/base/ERC20Base.sol |
| `getPastTotalSupply(uint256)` | `0x8e539e8c` | view | `(uint256)` | contracts/launchpad/contracts/base/ERC20Base.sol |
| `getPastVotes(address,uint256)` | `0x3a46b1a8` | view | `(uint256)` | contracts/launchpad/contracts/base/ERC20Base.sol |
| `name()` | `0x06fdde03` | view | `(string)` | contracts/launchpad/contracts/base/ERC20Base.sol |
| `nonces(address)` | `0x7ecebe00` | view | `(uint256)` | contracts/launchpad/contracts/base/ERC20Base.sol |
| `permit(address,address,uint256,uint256,uint8,bytes32,bytes32)` | `0xd505accf` | nonpayable | `()` | contracts/launchpad/contracts/base/ERC20Base.sol |
| `pool()` | `0x16f0115b` | view | `(address)` | contracts/launchpad/contracts/core/SidioraERC20.sol |
| `symbol()` | `0x95d89b41` | view | `(string)` | contracts/launchpad/contracts/base/ERC20Base.sol |
| `totalSupply()` | `0x18160ddd` | view | `(uint256)` | contracts/launchpad/contracts/base/ERC20Base.sol |
| `transfer(address,uint256)` | `0xa9059cbb` | nonpayable | `(bool)` | contracts/launchpad/contracts/base/ERC20Base.sol |
| `transferFrom(address,address,uint256)` | `0x23b872dd` | nonpayable | `(bool)` | contracts/launchpad/contracts/base/ERC20Base.sol |

| Event | Topic 0 | Declared |
|---|---|---|
| `Approval(address,address,uint256)` | `0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925` | contracts/launchpad/contracts/base/ERC20Base.sol |
| `Transfer(address,address,uint256)` | `0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef` | contracts/launchpad/contracts/base/ERC20Base.sol |

| Error | Selector | Declared |
|---|---|---|
| `FutureLookup()` | `0x0297ecc2` | contracts/launchpad/contracts/base/ERC20Base.sol |
| `InsufficientAllowance()` | `0x13be252b` | contracts/launchpad/contracts/base/ERC20Base.sol |
| `InsufficientBalance()` | `0xf4d678b8` | contracts/launchpad/contracts/base/ERC20Base.sol |
| `InvalidPermit()` | `0xddafbaef` | contracts/launchpad/contracts/base/ERC20Base.sol |
| `InvalidPoolBinding()` | `0xf77cf96c` | contracts/launchpad/contracts/core/SidioraERC20.sol |
| `PermitExpired()` | `0x1a15a3cc` | contracts/launchpad/contracts/base/ERC20Base.sol |
| `ZeroAddress()` | `0xd92e233d` | contracts/launchpad/contracts/base/ERC20Base.sol |

### SidioraNFT

37 functions, 10 events, 22 errors.

| Function | Selector | Mutability | Returns | Declared |
|---|---|---|---|---|
| `DEFAULT_ADMIN_ROLE()` | `0xa217fddf` | view | `(bytes32)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `MINTER_ROLE()` | `0xd5391393` | view | `(bytes32)` | contracts/launchpad/contracts/core/SidioraNFT.sol |
| `STRATEGY_SETTER_ROLE()` | `0x7bb5cc58` | view | `(bytes32)` | contracts/launchpad/contracts/core/SidioraNFT.sol |
| `approve(address,uint256)` | `0x095ea7b3` | nonpayable | `()` | contracts/launchpad/contracts/base/ERC721Base.sol |
| `balanceOf(address)` | `0x70a08231` | view | `(uint256)` | contracts/launchpad/contracts/base/ERC721Base.sol |
| `eventEmitter()` | `0x9ff78c30` | view | `(address)` | contracts/launchpad/contracts/core/SidioraNFT.sol |
| `getApproved(uint256)` | `0x081812fc` | view | `(address)` | contracts/launchpad/contracts/base/ERC721Base.sol |
| `getFeeStrategy(uint256)` | `0xdea3579a` | view | `(uint8)` | contracts/launchpad/contracts/core/SidioraNFT.sol |
| `getPoolAddress(uint256)` | `0x00a5ae21` | view | `(address)` | contracts/launchpad/contracts/core/SidioraNFT.sol |
| `getRoleAdmin(bytes32)` | `0x248a9ca3` | view | `(bytes32)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `grantRole(bytes32,address)` | `0x2f2ff15d` | nonpayable | `()` | contracts/launchpad/contracts/base/AccessControl.sol |
| `hasRole(bytes32,address)` | `0x91d14854` | view | `(bool)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `initialize(string,string,address,address)` | `0x8f15b414` | nonpayable | `()` | contracts/launchpad/contracts/core/SidioraNFT.sol |
| `isApprovedForAll(address,address)` | `0xe985e9c5` | view | `(bool)` | contracts/launchpad/contracts/base/ERC721Base.sol |
| `migratePoolIndex(uint256)` | `0x0270182d` | nonpayable | `()` | contracts/launchpad/contracts/core/SidioraNFT.sol |
| `mint(address,address,uint8)` | `0xbd5a3396` | nonpayable | `(uint256)` | contracts/launchpad/contracts/core/SidioraNFT.sol |
| `name()` | `0x06fdde03` | view | `(string)` | contracts/launchpad/contracts/base/ERC721Base.sol |
| `nextTokenId()` | `0x75794a3c` | view | `(uint256)` | contracts/launchpad/contracts/core/SidioraNFT.sol |
| `ownerOf(uint256)` | `0x6352211e` | view | `(address)` | contracts/launchpad/contracts/base/ERC721Base.sol |
| `poolIndexMigrationCursor()` | `0xce808028` | view | `(uint256)` | contracts/launchpad/contracts/core/SidioraNFT.sol |
| `poolIndexReady()` | `0x3e747982` | view | `(bool)` | contracts/launchpad/contracts/core/SidioraNFT.sol |
| `poolTokenId(address)` | `0x8a5d6909` | view | `(uint256)` | contracts/launchpad/contracts/core/SidioraNFT.sol |
| `proxiableUUID()` | `0x52d1902d` | view | `(bytes32)` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `registeredDefaultAdminCount()` | `0xc0cb71b6` | view | `(uint256)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `renounceRole(bytes32,address)` | `0x36568abe` | nonpayable | `()` | contracts/launchpad/contracts/base/AccessControl.sol |
| `revokeRole(bytes32,address)` | `0xd547741f` | nonpayable | `()` | contracts/launchpad/contracts/base/AccessControl.sol |
| `safeTransferFrom(address,address,uint256)` | `0x42842e0e` | nonpayable | `()` | contracts/launchpad/contracts/base/ERC721Base.sol |
| `safeTransferFrom(address,address,uint256,bytes)` | `0xb88d4fde` | nonpayable | `()` | contracts/launchpad/contracts/base/ERC721Base.sol |
| `setApprovalForAll(address,bool)` | `0xa22cb465` | nonpayable | `()` | contracts/launchpad/contracts/base/ERC721Base.sol |
| `setFeeStrategy(uint256,uint8)` | `0x9fcf8f62` | nonpayable | `()` | contracts/launchpad/contracts/core/SidioraNFT.sol |
| `supportsInterface(bytes4)` | `0x01ffc9a7` | view | `(bool)` | contracts/launchpad/contracts/core/SidioraNFT.sol |
| `symbol()` | `0x95d89b41` | view | `(string)` | contracts/launchpad/contracts/base/ERC721Base.sol |
| `tokenByIndex(uint256)` | `0x4f6ccce7` | view | `(uint256)` | contracts/launchpad/contracts/base/ERC721Enumerable.sol |
| `tokenOfOwnerByIndex(address,uint256)` | `0x2f745c59` | view | `(uint256)` | contracts/launchpad/contracts/base/ERC721Enumerable.sol |
| `totalSupply()` | `0x18160ddd` | view | `(uint256)` | contracts/launchpad/contracts/base/ERC721Enumerable.sol |
| `transferFrom(address,address,uint256)` | `0x23b872dd` | nonpayable | `()` | contracts/launchpad/contracts/base/ERC721Base.sol |
| `upgradeToAndCall(address,bytes)` | `0x4f1ef286` | payable | `()` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |

| Event | Topic 0 | Declared |
|---|---|---|
| `Approval(address,address,uint256)` | `0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925` | contracts/launchpad/contracts/base/ERC721Base.sol |
| `ApprovalForAll(address,address,bool)` | `0x17307eab39ab6107e8899845ad3d59bd9653f200f220920489ca2b5937696c31` | contracts/launchpad/contracts/base/ERC721Base.sol |
| `FeeStrategyChanged(uint256,uint8,uint8)` | `0x773a9b32cf9f6ecdff8ee5e6f78159f15beb042f9da471ba934182412ea69568` | contracts/launchpad/contracts/interfaces/ISidioraNFT.sol |
| `Initialized(uint64)` | `0xc7f505b2f371ae2175ee4913f4499e1f2633a7b5936321eed1cdaeb6115181d2` | contracts/launchpad/contracts/base/Initializable.sol |
| `PoolNFTMinted(uint256,address,address)` | `0xa1ff1e0bedf4ae07c5f17ac3f0d64625d9c386b9681a705499c8595ebea1a732` | contracts/launchpad/contracts/interfaces/ISidioraNFT.sol |
| `RoleAdminChanged(bytes32,bytes32,bytes32)` | `0xbd79b86ffe0ab8e8776151514217cd7cacd52c909f66475c3af44e129f0b00ff` | contracts/launchpad/contracts/base/AccessControl.sol |
| `RoleGranted(bytes32,address,address)` | `0x2f8788117e7eff1d82e926ec794901d17c78024a50270940304540a733656f0d` | contracts/launchpad/contracts/base/AccessControl.sol |
| `RoleRevoked(bytes32,address,address)` | `0xf6391f5c32d9c69d2a47ea670b442974b53935d1edc7fd64eb21e047a839171b` | contracts/launchpad/contracts/base/AccessControl.sol |
| `Transfer(address,address,uint256)` | `0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef` | contracts/launchpad/contracts/base/ERC721Base.sol |
| `Upgraded(address)` | `0xbc7cd75a20ee27fd9adebab32041f755214dbc6bffa90cc0225b39da2e5c2d3b` | contracts/launchpad/contracts/base/ERC1967Utils.sol |

| Error | Selector | Declared |
|---|---|---|
| `AlreadyInitialized()` | `0x0dc149f0` | contracts/launchpad/contracts/base/Initializable.sol |
| `AlreadyMinted()` | `0xddefae28` | contracts/launchpad/contracts/base/ERC721Base.sol |
| `DuplicatePool()` | `0xb82a4746` | contracts/launchpad/contracts/core/SidioraNFT.sol |
| `IndexOutOfBounds()` | `0x4e23d035` | contracts/launchpad/contracts/base/ERC721Enumerable.sol |
| `InvalidImplementation()` | `0x68155f9a` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `InvalidRoleAccount()` | `0xeccda526` | contracts/launchpad/contracts/base/AccessControl.sol |
| `InvalidStrategy()` | `0x4e236e9a` | contracts/launchpad/contracts/interfaces/ISidioraNFT.sol |
| `LastDefaultAdmin()` | `0x9c88180d` | contracts/launchpad/contracts/base/AccessControl.sol |
| `MissingRole(address,bytes32)` | `0x0161a64a` | contracts/launchpad/contracts/base/AccessControl.sol |
| `NonERC721Receiver()` | `0x80526d0c` | contracts/launchpad/contracts/base/ERC721Base.sol |
| `NonPayableUpgrade()` | `0x93738d49` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `NotApproved()` | `0xc19f17a9` | contracts/launchpad/contracts/base/ERC721Base.sol |
| `NotInitializing()` | `0xd7e6bcf8` | contracts/launchpad/contracts/base/Initializable.sol |
| `NotTokenOwner()` | `0x59dc379f` | contracts/launchpad/contracts/base/ERC721Base.sol |
| `PoolIndexMigrationRequired()` | `0x9aee8140` | contracts/launchpad/contracts/core/SidioraNFT.sol |
| `TokenNotFound()` | `0xcbdb7b30` | contracts/launchpad/contracts/base/ERC721Base.sol |
| `UUPSNotThroughActiveProxy()` | `0x45a93fc9` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `UUPSNotThroughProxy()` | `0x4f083858` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `UnauthorizedUpgrade()` | `0x3a617a54` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `UnsupportedSnapshotToken()` | `0xe3aa7e40` | contracts/launchpad/contracts/core/SidioraNFT.sol |
| `UpgradeInitializationFailed()` | `0xcc1cf440` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `ZeroAddress()` | `0xd92e233d` | contracts/launchpad/contracts/base/ERC721Base.sol |

### Router

23 functions, 9 events, 21 errors.

| Function | Selector | Mutability | Returns | Declared |
|---|---|---|---|---|
| `DEFAULT_ADMIN_ROLE()` | `0xa217fddf` | view | `(bytes32)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `buy(address,uint256,uint256,uint256)` | `0x1622dbe4` | nonpayable | `(uint256)` | contracts/launchpad/contracts/periphery/Router.sol |
| `buyWithPermit(address,uint256,uint256,uint256,(uint256,uint256,uint8,bytes32,bytes32))` | `0xffafb569` | nonpayable | `(uint256)` | contracts/launchpad/contracts/periphery/Router.sol |
| `createMarket(string,string,uint8,address)` | `0x57c0d7fd` | nonpayable | `(address,address,uint256)` | contracts/launchpad/contracts/periphery/Router.sol |
| `createMarketWithPermit(string,string,uint8,address,(uint256,uint256,uint8,bytes32,bytes32))` | `0xdc614295` | nonpayable | `(address,address,uint256)` | contracts/launchpad/contracts/periphery/Router.sol |
| `factory()` | `0xc45a0155` | view | `(address)` | contracts/launchpad/contracts/periphery/Router.sol |
| `getRoleAdmin(bytes32)` | `0x248a9ca3` | view | `(bytes32)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `grantRole(bytes32,address)` | `0x2f2ff15d` | nonpayable | `()` | contracts/launchpad/contracts/base/AccessControl.sol |
| `hasRole(bytes32,address)` | `0x91d14854` | view | `(bool)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `initialize(address,address,address,address,address)` | `0x1459457a` | nonpayable | `()` | contracts/launchpad/contracts/periphery/Router.sol |
| `multicall(bytes[])` | `0xac9650d8` | nonpayable | `(bytes[])` | contracts/launchpad/contracts/base/Multicall.sol |
| `poolRegistry()` | `0xafcff50f` | view | `(address)` | contracts/launchpad/contracts/periphery/Router.sol |
| `protocolConfig()` | `0xf5efbb4f` | view | `(address)` | contracts/launchpad/contracts/periphery/Router.sol |
| `proxiableUUID()` | `0x52d1902d` | view | `(bytes32)` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `registeredDefaultAdminCount()` | `0xc0cb71b6` | view | `(uint256)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `renounceRole(bytes32,address)` | `0x36568abe` | nonpayable | `()` | contracts/launchpad/contracts/base/AccessControl.sol |
| `revokeRole(bytes32,address)` | `0xd547741f` | nonpayable | `()` | contracts/launchpad/contracts/base/AccessControl.sol |
| `sell(address,uint256,uint256,uint256)` | `0x92cdbac5` | nonpayable | `(uint256)` | contracts/launchpad/contracts/periphery/Router.sol |
| `sellWithPermit(address,uint256,uint256,uint256,(uint256,uint256,uint8,bytes32,bytes32))` | `0x3aa73d77` | nonpayable | `(uint256)` | contracts/launchpad/contracts/periphery/Router.sol |
| `swapTokenForToken(address,address,uint256,uint256,uint256)` | `0x8ef5184b` | nonpayable | `(uint256,uint256)` | contracts/launchpad/contracts/periphery/Router.sol |
| `swapTokenForTokenWithPermit(address,address,uint256,uint256,uint256,(uint256,uint256,uint8,bytes32,bytes32))` | `0x1894fbf8` | nonpayable | `(uint256,uint256)` | contracts/launchpad/contracts/periphery/Router.sol |
| `upgradeToAndCall(address,bytes)` | `0x4f1ef286` | payable | `()` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `usdlAddress()` | `0x2e10f976` | view | `(address)` | contracts/launchpad/contracts/periphery/Router.sol |

| Event | Topic 0 | Declared |
|---|---|---|
| `Buy(address,address,uint256,uint256)` | `0x89f5adc174562e07c9c9b1cae7109bbecb21cf9d1b2847e550042b8653c54a0e` | contracts/launchpad/contracts/interfaces/IRouter.sol |
| `Initialized(uint64)` | `0xc7f505b2f371ae2175ee4913f4499e1f2633a7b5936321eed1cdaeb6115181d2` | contracts/launchpad/contracts/base/Initializable.sol |
| `MarketCreated(address,address,address,uint256)` | `0xfe7e33f239f268e882861cae62e631558d814973588d97738cc084b8b3489599` | contracts/launchpad/contracts/interfaces/IRouter.sol |
| `MultihopSwap(address,address,address,uint256,uint256,uint256)` | `0xef2dce1faa347130cf25c6d059af1f6f181983e69a2fadd79ed8751a532d3d96` | contracts/launchpad/contracts/interfaces/IRouter.sol |
| `RoleAdminChanged(bytes32,bytes32,bytes32)` | `0xbd79b86ffe0ab8e8776151514217cd7cacd52c909f66475c3af44e129f0b00ff` | contracts/launchpad/contracts/base/AccessControl.sol |
| `RoleGranted(bytes32,address,address)` | `0x2f8788117e7eff1d82e926ec794901d17c78024a50270940304540a733656f0d` | contracts/launchpad/contracts/base/AccessControl.sol |
| `RoleRevoked(bytes32,address,address)` | `0xf6391f5c32d9c69d2a47ea670b442974b53935d1edc7fd64eb21e047a839171b` | contracts/launchpad/contracts/base/AccessControl.sol |
| `Sell(address,address,uint256,uint256)` | `0xa082022e93cfcd9f1da5f9236718053910f7e840da080c789c7845698dc032ff` | contracts/launchpad/contracts/interfaces/IRouter.sol |
| `Upgraded(address)` | `0xbc7cd75a20ee27fd9adebab32041f755214dbc6bffa90cc0225b39da2e5c2d3b` | contracts/launchpad/contracts/base/ERC1967Utils.sol |

| Error | Selector | Declared |
|---|---|---|
| `AlreadyInitialized()` | `0x0dc149f0` | contracts/launchpad/contracts/base/Initializable.sol |
| `DeadlineExpired()` | `0x1ab7da6b` | contracts/launchpad/contracts/interfaces/IRouter.sol |
| `InsufficientBalance()` | `0xf4d678b8` | contracts/launchpad/contracts/interfaces/IRouter.sol |
| `InvalidImplementation()` | `0x68155f9a` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `InvalidRoleAccount()` | `0xeccda526` | contracts/launchpad/contracts/base/AccessControl.sol |
| `LastDefaultAdmin()` | `0x9c88180d` | contracts/launchpad/contracts/base/AccessControl.sol |
| `MissingRole(address,bytes32)` | `0x0161a64a` | contracts/launchpad/contracts/base/AccessControl.sol |
| `MulticallFailed(uint256,bytes)` | `0x6cf73d14` | contracts/launchpad/contracts/base/Multicall.sol |
| `NonPayableUpgrade()` | `0x93738d49` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `NotInitializing()` | `0xd7e6bcf8` | contracts/launchpad/contracts/base/Initializable.sol |
| `PoolNotFound()` | `0x76ecffc0` | contracts/launchpad/contracts/interfaces/IRouter.sol |
| `ReentrancyGuardReentrantCall()` | `0x3ee5aeb5` | contracts/launchpad/contracts/base/ReentrancyGuard.sol |
| `SameToken()` | `0x201b580a` | contracts/launchpad/contracts/interfaces/IRouter.sol |
| `TransferFailed()` | `0x90b8ec18` | contracts/launchpad/contracts/libraries/TransferHelper.sol |
| `TransferFromFailed()` | `0x7939f424` | contracts/launchpad/contracts/libraries/TransferHelper.sol |
| `UUPSNotThroughActiveProxy()` | `0x45a93fc9` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `UUPSNotThroughProxy()` | `0x4f083858` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `UnauthorizedUpgrade()` | `0x3a617a54` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `UpgradeInitializationFailed()` | `0xcc1cf440` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `ZeroAddress()` | `0xd92e233d` | contracts/launchpad/contracts/interfaces/IRouter.sol |
| `ZeroAmount()` | `0x1f2a2005` | contracts/launchpad/contracts/interfaces/IRouter.sol |

### Quoter

19 functions, 5 events, 18 errors.

| Function | Selector | Mutability | Returns | Declared |
|---|---|---|---|---|
| `DEFAULT_ADMIN_ROLE()` | `0xa217fddf` | view | `(bytes32)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `getAllPools(uint256,uint256)` | `0xa7753f09` | view | `(address[])` | contracts/launchpad/contracts/periphery/Quoter.sol |
| `getMarketCap(address)` | `0xd146d31d` | view | `(uint256)` | contracts/launchpad/contracts/periphery/Quoter.sol |
| `getPoolPrice(address)` | `0x0a6de2c9` | view | `(uint256)` | contracts/launchpad/contracts/periphery/Quoter.sol |
| `getPoolStats(address)` | `0x8ae97595` | view | `((uint256,uint256,uint256,uint256,uint256,uint256,uint256,uint256))` | contracts/launchpad/contracts/periphery/Quoter.sol |
| `getPoolsByCreator(address)` | `0xc84728ef` | view | `(address[])` | contracts/launchpad/contracts/periphery/Quoter.sol |
| `getRoleAdmin(bytes32)` | `0x248a9ca3` | view | `(bytes32)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `grantRole(bytes32,address)` | `0x2f2ff15d` | nonpayable | `()` | contracts/launchpad/contracts/base/AccessControl.sol |
| `hasRole(bytes32,address)` | `0x91d14854` | view | `(bool)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `initialize(address,address,address)` | `0xc0c53b8b` | nonpayable | `()` | contracts/launchpad/contracts/periphery/Quoter.sol |
| `poolRegistry()` | `0xafcff50f` | view | `(address)` | contracts/launchpad/contracts/periphery/Quoter.sol |
| `protocolConfig()` | `0xf5efbb4f` | view | `(address)` | contracts/launchpad/contracts/periphery/Quoter.sol |
| `proxiableUUID()` | `0x52d1902d` | view | `(bytes32)` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `quoteExactInput(address,uint256,bool)` | `0x4d9f71e4` | view | `((uint256,uint256,uint256))` | contracts/launchpad/contracts/periphery/Quoter.sol |
| `quoteMultihop(address,address,uint256)` | `0x7f1737ce` | view | `((uint256,uint256,uint256,uint256,uint256,uint256,uint256,address,address))` | contracts/launchpad/contracts/periphery/Quoter.sol |
| `registeredDefaultAdminCount()` | `0xc0cb71b6` | view | `(uint256)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `renounceRole(bytes32,address)` | `0x36568abe` | nonpayable | `()` | contracts/launchpad/contracts/base/AccessControl.sol |
| `revokeRole(bytes32,address)` | `0xd547741f` | nonpayable | `()` | contracts/launchpad/contracts/base/AccessControl.sol |
| `upgradeToAndCall(address,bytes)` | `0x4f1ef286` | payable | `()` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |

| Event | Topic 0 | Declared |
|---|---|---|
| `Initialized(uint64)` | `0xc7f505b2f371ae2175ee4913f4499e1f2633a7b5936321eed1cdaeb6115181d2` | contracts/launchpad/contracts/base/Initializable.sol |
| `RoleAdminChanged(bytes32,bytes32,bytes32)` | `0xbd79b86ffe0ab8e8776151514217cd7cacd52c909f66475c3af44e129f0b00ff` | contracts/launchpad/contracts/base/AccessControl.sol |
| `RoleGranted(bytes32,address,address)` | `0x2f8788117e7eff1d82e926ec794901d17c78024a50270940304540a733656f0d` | contracts/launchpad/contracts/base/AccessControl.sol |
| `RoleRevoked(bytes32,address,address)` | `0xf6391f5c32d9c69d2a47ea670b442974b53935d1edc7fd64eb21e047a839171b` | contracts/launchpad/contracts/base/AccessControl.sol |
| `Upgraded(address)` | `0xbc7cd75a20ee27fd9adebab32041f755214dbc6bffa90cc0225b39da2e5c2d3b` | contracts/launchpad/contracts/base/ERC1967Utils.sol |

| Error | Selector | Declared |
|---|---|---|
| `AlreadyInitialized()` | `0x0dc149f0` | contracts/launchpad/contracts/base/Initializable.sol |
| `DivisionByZero()` | `0x23d359a3` | contracts/launchpad/contracts/libraries/SidioraMath.sol |
| `InsufficientInput()` | `0xf8b3bb61` | contracts/launchpad/contracts/interfaces/ISidioraPool.sol |
| `InsufficientLiquidity()` | `0xbb55fd27` | contracts/launchpad/contracts/interfaces/ISidioraPool.sol |
| `InvalidImplementation()` | `0x68155f9a` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `InvalidRoleAccount()` | `0xeccda526` | contracts/launchpad/contracts/base/AccessControl.sol |
| `LastDefaultAdmin()` | `0x9c88180d` | contracts/launchpad/contracts/base/AccessControl.sol |
| `MissingRole(address,bytes32)` | `0x0161a64a` | contracts/launchpad/contracts/base/AccessControl.sol |
| `NonPayableUpgrade()` | `0x93738d49` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `NotInitializing()` | `0xd7e6bcf8` | contracts/launchpad/contracts/base/Initializable.sol |
| `OpticalQuoteUnsupported()` | `0x41d4e1d1` | contracts/launchpad/contracts/periphery/Quoter.sol |
| `Overflow()` | `0x35278d12` | contracts/launchpad/contracts/libraries/SidioraMath.sol |
| `UUPSNotThroughActiveProxy()` | `0x45a93fc9` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `UUPSNotThroughProxy()` | `0x4f083858` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `UnauthorizedUpgrade()` | `0x3a617a54` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `UnexecutableQuote()` | `0xc1becfdb` | contracts/launchpad/contracts/periphery/Quoter.sol |
| `UpgradeInitializationFailed()` | `0xcc1cf440` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `ZeroAddress()` | `0xd92e233d` | contracts/launchpad/contracts/periphery/Quoter.sol |

### FeesRouter

20 functions, 12 events, 16 errors.

| Function | Selector | Mutability | Returns | Declared |
|---|---|---|---|---|
| `DEFAULT_ADMIN_ROLE()` | `0xa217fddf` | view | `(bytes32)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `claimAirdrop(uint256)` | `0xe30d4440` | nonpayable | `(uint256)` | contracts/launchpad/contracts/periphery/FeesRouter.sol |
| `claimAirdropEpoch(uint256,uint256)` | `0x13e615b9` | nonpayable | `(uint256)` | contracts/launchpad/contracts/periphery/FeesRouter.sol |
| `claimFees(uint256)` | `0xac68a748` | nonpayable | `(uint256)` | contracts/launchpad/contracts/periphery/FeesRouter.sol |
| `executeAirdrop(uint256)` | `0x9d4be5b2` | nonpayable | `(uint256)` | contracts/launchpad/contracts/periphery/FeesRouter.sol |
| `executeBurn(uint256)` | `0x15dec3cf` | nonpayable | `(uint256)` | contracts/launchpad/contracts/periphery/FeesRouter.sol |
| `executeLpRewards(uint256)` | `0x90d862a1` | nonpayable | `(uint256)` | contracts/launchpad/contracts/periphery/FeesRouter.sol |
| `feeAccumulator()` | `0x5eaea35f` | view | `(address)` | contracts/launchpad/contracts/periphery/FeesRouter.sol |
| `getRoleAdmin(bytes32)` | `0x248a9ca3` | view | `(bytes32)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `grantRole(bytes32,address)` | `0x2f2ff15d` | nonpayable | `()` | contracts/launchpad/contracts/base/AccessControl.sol |
| `hasRole(bytes32,address)` | `0x91d14854` | view | `(bool)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `initialize(address,address,address,address)` | `0xf8c8765e` | nonpayable | `()` | contracts/launchpad/contracts/periphery/FeesRouter.sol |
| `nftContract()` | `0xd56d229d` | view | `(address)` | contracts/launchpad/contracts/periphery/FeesRouter.sol |
| `poolRegistry()` | `0xafcff50f` | view | `(address)` | contracts/launchpad/contracts/periphery/FeesRouter.sol |
| `proxiableUUID()` | `0x52d1902d` | view | `(bytes32)` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `registeredDefaultAdminCount()` | `0xc0cb71b6` | view | `(uint256)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `renounceRole(bytes32,address)` | `0x36568abe` | nonpayable | `()` | contracts/launchpad/contracts/base/AccessControl.sol |
| `revokeRole(bytes32,address)` | `0xd547741f` | nonpayable | `()` | contracts/launchpad/contracts/base/AccessControl.sol |
| `setFeeStrategy(uint256,uint8)` | `0x9fcf8f62` | nonpayable | `()` | contracts/launchpad/contracts/periphery/FeesRouter.sol |
| `upgradeToAndCall(address,bytes)` | `0x4f1ef286` | payable | `()` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |

| Event | Topic 0 | Declared |
|---|---|---|
| `AirdropClaimed(uint256,address,uint256)` | `0x752d3a5de250652e9111e24fea2886c837dadfe50c8479ad3b5511f4c36f20a2` | contracts/launchpad/contracts/interfaces/IFeesRouter.sol |
| `AirdropExecuted(uint256,uint256)` | `0x0fa3c377438bb545a4c9ecaa3e574d96bb1b2c6416976df85919d2b83a57bd9f` | contracts/launchpad/contracts/interfaces/IFeesRouter.sol |
| `FeeStrategyChanged(uint256,uint8,uint8)` | `0x773a9b32cf9f6ecdff8ee5e6f78159f15beb042f9da471ba934182412ea69568` | contracts/launchpad/contracts/interfaces/IFeesRouter.sol |
| `FeesBurned(uint256,uint256)` | `0x9768e1fe058fe58a8022c0c5a88689f80a64c3a69cc5ba498c518407fc04ced5` | contracts/launchpad/contracts/interfaces/IFeesRouter.sol |
| `FeesClaimed(uint256,address,uint256)` | `0x2227733fc4c8a9034cb58087dcf6995128b9c0233b038b03366aaf30c92b92d6` | contracts/launchpad/contracts/interfaces/IFeesRouter.sol |
| `Initialized(uint64)` | `0xc7f505b2f371ae2175ee4913f4499e1f2633a7b5936321eed1cdaeb6115181d2` | contracts/launchpad/contracts/base/Initializable.sol |
| `LpRewardsExecuted(uint256,uint256)` | `0xf831aaeb2aae759a8843e359900e1b8a3fe4cdefa90fc390825acb66a4374408` | contracts/launchpad/contracts/interfaces/IFeesRouter.sol |
| `MirrorFailed(bytes4)` | `0x8694b6bbbf10ff6faa1b6ae45172474b03b0df0b4d87ac2113bbced3810eebdd` | contracts/launchpad/contracts/periphery/FeesRouter.sol |
| `RoleAdminChanged(bytes32,bytes32,bytes32)` | `0xbd79b86ffe0ab8e8776151514217cd7cacd52c909f66475c3af44e129f0b00ff` | contracts/launchpad/contracts/base/AccessControl.sol |
| `RoleGranted(bytes32,address,address)` | `0x2f8788117e7eff1d82e926ec794901d17c78024a50270940304540a733656f0d` | contracts/launchpad/contracts/base/AccessControl.sol |
| `RoleRevoked(bytes32,address,address)` | `0xf6391f5c32d9c69d2a47ea670b442974b53935d1edc7fd64eb21e047a839171b` | contracts/launchpad/contracts/base/AccessControl.sol |
| `Upgraded(address)` | `0xbc7cd75a20ee27fd9adebab32041f755214dbc6bffa90cc0225b39da2e5c2d3b` | contracts/launchpad/contracts/base/ERC1967Utils.sol |

| Error | Selector | Declared |
|---|---|---|
| `AlreadyInitialized()` | `0x0dc149f0` | contracts/launchpad/contracts/base/Initializable.sol |
| `InvalidImplementation()` | `0x68155f9a` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `InvalidRoleAccount()` | `0xeccda526` | contracts/launchpad/contracts/base/AccessControl.sol |
| `LastDefaultAdmin()` | `0x9c88180d` | contracts/launchpad/contracts/base/AccessControl.sol |
| `MissingRole(address,bytes32)` | `0x0161a64a` | contracts/launchpad/contracts/base/AccessControl.sol |
| `NonPayableUpgrade()` | `0x93738d49` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `NotInitializing()` | `0xd7e6bcf8` | contracts/launchpad/contracts/base/Initializable.sol |
| `NotNftOwner()` | `0x4b64ae47` | contracts/launchpad/contracts/interfaces/IFeesRouter.sol |
| `ReentrancyGuardReentrantCall()` | `0x3ee5aeb5` | contracts/launchpad/contracts/base/ReentrancyGuard.sol |
| `UUPSNotThroughActiveProxy()` | `0x45a93fc9` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `UUPSNotThroughProxy()` | `0x4f083858` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `UnauthorizedUpgrade()` | `0x3a617a54` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `UnsupportedSnapshotToken()` | `0xe3aa7e40` | contracts/launchpad/contracts/interfaces/IFeesRouter.sol |
| `UpgradeInitializationFailed()` | `0xcc1cf440` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `WrongStrategy()` | `0xe181295c` | contracts/launchpad/contracts/interfaces/IFeesRouter.sol |
| `ZeroAddress()` | `0xd92e233d` | contracts/launchpad/contracts/interfaces/IFeesRouter.sol |

### FeeAccumulator

54 functions, 17 events, 23 errors.

| Function | Selector | Mutability | Returns | Declared |
|---|---|---|---|---|
| `DEAD_ADDRESS()` | `0x4e6fd6c4` | view | `(address)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `DEFAULT_ADMIN_ROLE()` | `0xa217fddf` | view | `(bytes32)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `FACTORY_ROLE()` | `0x04a0fb17` | view | `(bytes32)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `FEES_ROUTER_ROLE()` | `0xdfb76d70` | view | `(bytes32)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `OPTICAL_CLAIM_ROLE()` | `0x39df917a` | view | `(bytes32)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `OPTICAL_GRANTER_ROLE()` | `0x85f3d987` | view | `(bytes32)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `POOL_ROLE()` | `0x404ccd07` | view | `(bytes32)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `accountingInitialized()` | `0x2a2593e7` | view | `(bool)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `accountingMigrationCursor()` | `0xad37f674` | view | `(uint256)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `airdropClaimedSupply(address,uint256)` | `0x9deb3b11` | view | `(uint256)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `airdropEligibleSupply(address,uint256)` | `0xd71c7375` | view | `(uint256)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `airdropEpochRemaining(address,uint256)` | `0x81de2866` | view | `(uint256)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `airdropSnapshotBlock(address,uint256)` | `0xae1e143b` | view | `(uint256)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `allocateLegacyAirdrop(address,uint256,bytes32,uint256)` | `0xb01b2210` | nonpayable | `()` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `authorizeOptical(address)` | `0x610edef8` | nonpayable | `()` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `authorizePool(address)` | `0x3059b7c6` | nonpayable | `()` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `burn(address)` | `0x89afcb44` | nonpayable | `(uint256)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `claim(address,address)` | `0x21c0b342` | nonpayable | `(uint256)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `claimAirdrop(address,address)` | `0x66e1b755` | nonpayable | `(uint256)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `claimAirdropEpoch(address,address,uint256)` | `0xd195ba7e` | nonpayable | `(uint256)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `claimLegacyAirdrop(address,uint256,address,uint256,bytes32[])` | `0x05fb334e` | nonpayable | `()` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `claimOpticalSurplus(address,uint256,address)` | `0x613022fd` | nonpayable | `()` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `eventEmitter()` | `0x9ff78c30` | view | `(address)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `getAccumulatedFees(address)` | `0x564201f9` | view | `(uint256)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `getAirdropBalance(address)` | `0xefa94d49` | view | `(uint256)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `getAirdropEpoch(address)` | `0x9eac5b39` | view | `(uint256)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `getLpRewardsBalance(address)` | `0x81fa4ef7` | view | `(uint256)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `getOpticalSurplus(address)` | `0xfc7ea90c` | view | `(uint256)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `getProtocolFeesPending()` | `0x52b2a385` | view | `(uint256)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `getRoleAdmin(bytes32)` | `0x248a9ca3` | view | `(bytes32)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `grantRole(bytes32,address)` | `0x2f2ff15d` | nonpayable | `()` | contracts/launchpad/contracts/base/AccessControl.sol |
| `hasClaimedAirdrop(address,address,uint256)` | `0x2c959e18` | view | `(bool)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `hasRole(bytes32,address)` | `0x91d14854` | view | `(bool)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `initialize(address,address,address,address,address,address)` | `0xcc2a9a5b` | nonpayable | `()` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `legacyAirdropRemaining(address,uint256)` | `0xf10dec34` | view | `(uint256)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `legacyAirdropRoot(address,uint256)` | `0x45f34775` | view | `(bytes32)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `legacyAirdropUnallocated(address)` | `0x8e09d24b` | view | `(uint256)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `migrateAccounting(uint256)` | `0x479db4fd` | nonpayable | `()` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `migrateAdditionalAccounting(address[])` | `0xdb6087ea` | nonpayable | `()` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `poolRegistry()` | `0xafcff50f` | view | `(address)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `protocolConfig()` | `0xf5efbb4f` | view | `(address)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `proxiableUUID()` | `0x52d1902d` | view | `(bytes32)` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `recordFee(address,uint256)` | `0x0a6e0fd4` | nonpayable | `()` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `recordOpticalSurplus(address,uint256)` | `0x9605d79b` | nonpayable | `()` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `registeredDefaultAdminCount()` | `0xc0cb71b6` | view | `(uint256)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `renounceRole(bytes32,address)` | `0x36568abe` | nonpayable | `()` | contracts/launchpad/contracts/base/AccessControl.sol |
| `revokeRole(bytes32,address)` | `0xd547741f` | nonpayable | `()` | contracts/launchpad/contracts/base/AccessControl.sol |
| `sendLpRewards(address)` | `0xa48d7d07` | nonpayable | `(uint256)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `supportsAirdrop(address)` | `0x8c61de83` | view | `(bool)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `totalLiabilities()` | `0xf73579a9` | view | `(uint256)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `treasury()` | `0x61d027b3` | view | `(address)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `triggerAirdrop(address)` | `0xed374d4a` | nonpayable | `(uint256)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `upgradeToAndCall(address,bytes)` | `0x4f1ef286` | payable | `()` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `usdlAddress()` | `0x2e10f976` | view | `(address)` | contracts/launchpad/contracts/data/FeeAccumulator.sol |

| Event | Topic 0 | Declared |
|---|---|---|
| `AccountingMigrated(uint256)` | `0x75b17ca7274960709421f611b666d569ceebe0cbb24821254bfe2e5e4fce9df2` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `AirdropClaimed(address,address,uint256,uint256)` | `0xd399c6e7fad358fc300beda3f056717c94a04c7233ce92683de6500ba509022e` | contracts/launchpad/contracts/interfaces/IFeeAccumulator.sol |
| `AirdropTriggered(address,uint256,uint256)` | `0x8c39ea0d52c00d579184e8070640104228f1a618d77e55429e09cfbb049c2dc1` | contracts/launchpad/contracts/interfaces/IFeeAccumulator.sol |
| `FeeRecorded(address,uint256,uint256,uint256)` | `0xb4d4d3bd2f97a7d6f1657ee69f7191d7aa7dbd5b6864a2d7a9d14efc1322552f` | contracts/launchpad/contracts/interfaces/IFeeAccumulator.sol |
| `FeesBurned(address,uint256)` | `0x0d9575a73e2a7da16cfde907df749d23d901528ff2e7c832b731babdecca000b` | contracts/launchpad/contracts/interfaces/IFeeAccumulator.sol |
| `FeesClaimed(address,address,uint256)` | `0xfe3464cd748424446c37877c28ce5b700222c5bc9f90d908afcc4e5cb22707ff` | contracts/launchpad/contracts/interfaces/IFeeAccumulator.sol |
| `Initialized(uint64)` | `0xc7f505b2f371ae2175ee4913f4499e1f2633a7b5936321eed1cdaeb6115181d2` | contracts/launchpad/contracts/base/Initializable.sol |
| `LegacyAirdropAllocated(address,uint256,bytes32,uint256)` | `0x66f288a74d4384624c871c51e16635a78cbc49fc5c471db184fae9b5860ee216` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `LpRewardsSent(address,uint256)` | `0x86b634da567bed9efe3e74597f5b2f7c9a880ed2e05e28f4c7c2fae1f0a9cfdb` | contracts/launchpad/contracts/interfaces/IFeeAccumulator.sol |
| `MirrorFailed(bytes4)` | `0x8694b6bbbf10ff6faa1b6ae45172474b03b0df0b4d87ac2113bbced3810eebdd` | contracts/launchpad/contracts/data/FeeAccumulator.sol |
| `OpticalSurplusClaimed(address,address,uint256)` | `0x879606fa2db265e0664b2683c3e39e5ca7c90eae54628f7af6b70f239af8d0a6` | contracts/launchpad/contracts/interfaces/IFeeAccumulator.sol |
| `OpticalSurplusRecorded(address,uint256)` | `0xc15bf759695c18fd9ddf60f8638036310ac80c3da55f86b717f102c53a44a9e3` | contracts/launchpad/contracts/interfaces/IFeeAccumulator.sol |
| `ProtocolFeeSwept(uint256)` | `0xe50aaa1965bcc0030c0dedb0b575635e1da17cf4989a55fa0f691ab35ff7ff13` | contracts/launchpad/contracts/interfaces/IFeeAccumulator.sol |
| `RoleAdminChanged(bytes32,bytes32,bytes32)` | `0xbd79b86ffe0ab8e8776151514217cd7cacd52c909f66475c3af44e129f0b00ff` | contracts/launchpad/contracts/base/AccessControl.sol |
| `RoleGranted(bytes32,address,address)` | `0x2f8788117e7eff1d82e926ec794901d17c78024a50270940304540a733656f0d` | contracts/launchpad/contracts/base/AccessControl.sol |
| `RoleRevoked(bytes32,address,address)` | `0xf6391f5c32d9c69d2a47ea670b442974b53935d1edc7fd64eb21e047a839171b` | contracts/launchpad/contracts/base/AccessControl.sol |
| `Upgraded(address)` | `0xbc7cd75a20ee27fd9adebab32041f755214dbc6bffa90cc0225b39da2e5c2d3b` | contracts/launchpad/contracts/base/ERC1967Utils.sol |

| Error | Selector | Declared |
|---|---|---|
| `AirdropNotTriggered()` | `0xceadfa6d` | contracts/launchpad/contracts/interfaces/IFeeAccumulator.sol |
| `AlreadyClaimed()` | `0x646cf558` | contracts/launchpad/contracts/interfaces/IFeeAccumulator.sol |
| `AlreadyInitialized()` | `0x0dc149f0` | contracts/launchpad/contracts/base/Initializable.sol |
| `ApproveFailed()` | `0x3e3f8f73` | contracts/launchpad/contracts/libraries/TransferHelper.sol |
| `InsufficientSurplus()` | `0x94290ab9` | contracts/launchpad/contracts/interfaces/IFeeAccumulator.sol |
| `InvalidImplementation()` | `0x68155f9a` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `InvalidRoleAccount()` | `0xeccda526` | contracts/launchpad/contracts/base/AccessControl.sol |
| `LastDefaultAdmin()` | `0x9c88180d` | contracts/launchpad/contracts/base/AccessControl.sol |
| `MissingRole(address,bytes32)` | `0x0161a64a` | contracts/launchpad/contracts/base/AccessControl.sol |
| `NoFeesAccumulated()` | `0xbe6fbbaf` | contracts/launchpad/contracts/interfaces/IFeeAccumulator.sol |
| `NonPayableUpgrade()` | `0x93738d49` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `NotInitializing()` | `0xd7e6bcf8` | contracts/launchpad/contracts/base/Initializable.sol |
| `ReentrancyGuardReentrantCall()` | `0x3ee5aeb5` | contracts/launchpad/contracts/base/ReentrancyGuard.sol |
| `TransferFailed()` | `0x90b8ec18` | contracts/launchpad/contracts/libraries/TransferHelper.sol |
| `UUPSNotThroughActiveProxy()` | `0x45a93fc9` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `UUPSNotThroughProxy()` | `0x4f083858` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `Unauthorized()` | `0x82b42900` | contracts/launchpad/contracts/interfaces/IFeeAccumulator.sol |
| `UnauthorizedUpgrade()` | `0x3a617a54` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `UnsupportedSnapshotToken()` | `0xe3aa7e40` | contracts/launchpad/contracts/interfaces/IFeeAccumulator.sol |
| `UpgradeInitializationFailed()` | `0xcc1cf440` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `WrongStrategy()` | `0xe181295c` | contracts/launchpad/contracts/interfaces/IFeeAccumulator.sol |
| `ZeroAddress()` | `0xd92e233d` | contracts/launchpad/contracts/interfaces/IFeeAccumulator.sol |
| `ZeroAmount()` | `0x1f2a2005` | contracts/launchpad/contracts/interfaces/IFeeAccumulator.sol |

### PoolRegistry

22 functions, 7 events, 16 errors.

| Function | Selector | Mutability | Returns | Declared |
|---|---|---|---|---|
| `DEFAULT_ADMIN_ROLE()` | `0xa217fddf` | view | `(bytes32)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `FACTORY_ROLE()` | `0x04a0fb17` | view | `(bytes32)` | contracts/launchpad/contracts/data/PoolRegistry.sol |
| `eventEmitter()` | `0x9ff78c30` | view | `(address)` | contracts/launchpad/contracts/data/PoolRegistry.sol |
| `getAllPools(uint256,uint256)` | `0xa7753f09` | view | `(address[])` | contracts/launchpad/contracts/data/PoolRegistry.sol |
| `getNftIdByPool(address)` | `0xd56ed6c4` | view | `(uint256)` | contracts/launchpad/contracts/data/PoolRegistry.sol |
| `getPoolByToken(address)` | `0xbbadc03b` | view | `(address)` | contracts/launchpad/contracts/data/PoolRegistry.sol |
| `getPoolCount()` | `0x8eec5d70` | view | `(uint256)` | contracts/launchpad/contracts/data/PoolRegistry.sol |
| `getPoolCountByCreator(address)` | `0x4598ed01` | view | `(uint256)` | contracts/launchpad/contracts/data/PoolRegistry.sol |
| `getPoolMetadata(address)` | `0x51de9d26` | view | `((address,address,address,uint256,uint256,uint256))` | contracts/launchpad/contracts/data/PoolRegistry.sol |
| `getPoolsByCreator(address)` | `0xc84728ef` | view | `(address[])` | contracts/launchpad/contracts/data/PoolRegistry.sol |
| `getPoolsByCreatorPage(address,uint256,uint256)` | `0xf27495af` | view | `(address[])` | contracts/launchpad/contracts/data/PoolRegistry.sol |
| `getRoleAdmin(bytes32)` | `0x248a9ca3` | view | `(bytes32)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `grantRole(bytes32,address)` | `0x2f2ff15d` | nonpayable | `()` | contracts/launchpad/contracts/base/AccessControl.sol |
| `hasRole(bytes32,address)` | `0x91d14854` | view | `(bool)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `initialize(address,address)` | `0x485cc955` | nonpayable | `()` | contracts/launchpad/contracts/data/PoolRegistry.sol |
| `isRegisteredPool(address)` | `0xc699e633` | view | `(bool)` | contracts/launchpad/contracts/data/PoolRegistry.sol |
| `proxiableUUID()` | `0x52d1902d` | view | `(bytes32)` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `register(address,address,address,address,uint256)` | `0x43a6d1c3` | nonpayable | `()` | contracts/launchpad/contracts/data/PoolRegistry.sol |
| `registeredDefaultAdminCount()` | `0xc0cb71b6` | view | `(uint256)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `renounceRole(bytes32,address)` | `0x36568abe` | nonpayable | `()` | contracts/launchpad/contracts/base/AccessControl.sol |
| `revokeRole(bytes32,address)` | `0xd547741f` | nonpayable | `()` | contracts/launchpad/contracts/base/AccessControl.sol |
| `upgradeToAndCall(address,bytes)` | `0x4f1ef286` | payable | `()` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |

| Event | Topic 0 | Declared |
|---|---|---|
| `Initialized(uint64)` | `0xc7f505b2f371ae2175ee4913f4499e1f2633a7b5936321eed1cdaeb6115181d2` | contracts/launchpad/contracts/base/Initializable.sol |
| `MirrorFailed(bytes4)` | `0x8694b6bbbf10ff6faa1b6ae45172474b03b0df0b4d87ac2113bbced3810eebdd` | contracts/launchpad/contracts/data/PoolRegistry.sol |
| `PoolRegistered(address,address,address,address,uint256,uint256)` | `0xdd85b374a142757b7745e1d5957bddced32d0fae0d5a038c41e7f79dc162b5ce` | contracts/launchpad/contracts/interfaces/IPoolRegistry.sol |
| `RoleAdminChanged(bytes32,bytes32,bytes32)` | `0xbd79b86ffe0ab8e8776151514217cd7cacd52c909f66475c3af44e129f0b00ff` | contracts/launchpad/contracts/base/AccessControl.sol |
| `RoleGranted(bytes32,address,address)` | `0x2f8788117e7eff1d82e926ec794901d17c78024a50270940304540a733656f0d` | contracts/launchpad/contracts/base/AccessControl.sol |
| `RoleRevoked(bytes32,address,address)` | `0xf6391f5c32d9c69d2a47ea670b442974b53935d1edc7fd64eb21e047a839171b` | contracts/launchpad/contracts/base/AccessControl.sol |
| `Upgraded(address)` | `0xbc7cd75a20ee27fd9adebab32041f755214dbc6bffa90cc0225b39da2e5c2d3b` | contracts/launchpad/contracts/base/ERC1967Utils.sol |

| Error | Selector | Declared |
|---|---|---|
| `AlreadyInitialized()` | `0x0dc149f0` | contracts/launchpad/contracts/base/Initializable.sol |
| `DuplicatePool()` | `0xb82a4746` | contracts/launchpad/contracts/interfaces/IPoolRegistry.sol |
| `DuplicateToken()` | `0x464e3f6a` | contracts/launchpad/contracts/interfaces/IPoolRegistry.sol |
| `InvalidImplementation()` | `0x68155f9a` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `InvalidRoleAccount()` | `0xeccda526` | contracts/launchpad/contracts/base/AccessControl.sol |
| `LastDefaultAdmin()` | `0x9c88180d` | contracts/launchpad/contracts/base/AccessControl.sol |
| `MissingRole(address,bytes32)` | `0x0161a64a` | contracts/launchpad/contracts/base/AccessControl.sol |
| `NonPayableUpgrade()` | `0x93738d49` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `NotInitializing()` | `0xd7e6bcf8` | contracts/launchpad/contracts/base/Initializable.sol |
| `PoolNotFound()` | `0x76ecffc0` | contracts/launchpad/contracts/interfaces/IPoolRegistry.sol |
| `UUPSNotThroughActiveProxy()` | `0x45a93fc9` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `UUPSNotThroughProxy()` | `0x4f083858` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `Unauthorized()` | `0x82b42900` | contracts/launchpad/contracts/interfaces/IPoolRegistry.sol |
| `UnauthorizedUpgrade()` | `0x3a617a54` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `UpgradeInitializationFailed()` | `0xcc1cf440` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `ZeroAddress()` | `0xd92e233d` | contracts/launchpad/contracts/interfaces/IPoolRegistry.sol |

### ProtocolConfig

27 functions, 6 events, 13 errors.

| Function | Selector | Mutability | Returns | Declared |
|---|---|---|---|---|
| `DEFAULT_ADMIN_ROLE()` | `0xa217fddf` | view | `(bytes32)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `baseFeeBps()` | `0xbf5a5940` | view | `(uint256)` | contracts/launchpad/contracts/protocol/ProtocolConfig.sol |
| `concentrationWeight()` | `0x2006e14c` | view | `(uint256)` | contracts/launchpad/contracts/protocol/ProtocolConfig.sol |
| `creationFee()` | `0xdce0b4e4` | view | `(uint256)` | contracts/launchpad/contracts/protocol/ProtocolConfig.sol |
| `eventEmitter()` | `0x9ff78c30` | view | `(address)` | contracts/launchpad/contracts/protocol/ProtocolConfig.sol |
| `feeDecayRate()` | `0x155d94c4` | view | `(uint256)` | contracts/launchpad/contracts/protocol/ProtocolConfig.sol |
| `getRoleAdmin(bytes32)` | `0x248a9ca3` | view | `(bytes32)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `grantRole(bytes32,address)` | `0x2f2ff15d` | nonpayable | `()` | contracts/launchpad/contracts/base/AccessControl.sol |
| `hasRole(bytes32,address)` | `0x91d14854` | view | `(bool)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `initialize(address,address,address)` | `0xc0c53b8b` | nonpayable | `()` | contracts/launchpad/contracts/protocol/ProtocolConfig.sol |
| `maxFeeBps()` | `0xbf769a3f` | view | `(uint256)` | contracts/launchpad/contracts/protocol/ProtocolConfig.sol |
| `minFeeBps()` | `0x071de20d` | view | `(uint256)` | contracts/launchpad/contracts/protocol/ProtocolConfig.sol |
| `protocolFeeBps()` | `0x35659fb8` | view | `(uint256)` | contracts/launchpad/contracts/protocol/ProtocolConfig.sol |
| `proxiableUUID()` | `0x52d1902d` | view | `(bytes32)` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `registeredDefaultAdminCount()` | `0xc0cb71b6` | view | `(uint256)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `renounceRole(bytes32,address)` | `0x36568abe` | nonpayable | `()` | contracts/launchpad/contracts/base/AccessControl.sol |
| `revokeRole(bytes32,address)` | `0xd547741f` | nonpayable | `()` | contracts/launchpad/contracts/base/AccessControl.sol |
| `setBaseFeeBps(uint256)` | `0xa389301a` | nonpayable | `()` | contracts/launchpad/contracts/protocol/ProtocolConfig.sol |
| `setCreationFee(uint256)` | `0xb7d86225` | nonpayable | `()` | contracts/launchpad/contracts/protocol/ProtocolConfig.sol |
| `setFeeWeights(uint256,uint256,uint256)` | `0xf1726fe7` | nonpayable | `()` | contracts/launchpad/contracts/protocol/ProtocolConfig.sol |
| `setProtocolFeeBps(uint256)` | `0xc0417e58` | nonpayable | `()` | contracts/launchpad/contracts/protocol/ProtocolConfig.sol |
| `setVirtualDefaults(uint256,uint256)` | `0xa83f1b4a` | nonpayable | `()` | contracts/launchpad/contracts/protocol/ProtocolConfig.sol |
| `upgradeToAndCall(address,bytes)` | `0x4f1ef286` | payable | `()` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `usdlAddress()` | `0x2e10f976` | view | `(address)` | contracts/launchpad/contracts/protocol/ProtocolConfig.sol |
| `virtualTokenDefault()` | `0xd9fce324` | view | `(uint256)` | contracts/launchpad/contracts/protocol/ProtocolConfig.sol |
| `virtualUsdlDefault()` | `0x071ed6b5` | view | `(uint256)` | contracts/launchpad/contracts/protocol/ProtocolConfig.sol |
| `volatilityWeight()` | `0x294ae7f9` | view | `(uint256)` | contracts/launchpad/contracts/protocol/ProtocolConfig.sol |

| Event | Topic 0 | Declared |
|---|---|---|
| `ConfigUpdated(bytes32,uint256,uint256)` | `0xac2ccce3de9c0816ae772598f7f65fe69f9893b637f7c490497378cbb3ea043e` | contracts/launchpad/contracts/interfaces/IProtocolConfig.sol |
| `Initialized(uint64)` | `0xc7f505b2f371ae2175ee4913f4499e1f2633a7b5936321eed1cdaeb6115181d2` | contracts/launchpad/contracts/base/Initializable.sol |
| `RoleAdminChanged(bytes32,bytes32,bytes32)` | `0xbd79b86ffe0ab8e8776151514217cd7cacd52c909f66475c3af44e129f0b00ff` | contracts/launchpad/contracts/base/AccessControl.sol |
| `RoleGranted(bytes32,address,address)` | `0x2f8788117e7eff1d82e926ec794901d17c78024a50270940304540a733656f0d` | contracts/launchpad/contracts/base/AccessControl.sol |
| `RoleRevoked(bytes32,address,address)` | `0xf6391f5c32d9c69d2a47ea670b442974b53935d1edc7fd64eb21e047a839171b` | contracts/launchpad/contracts/base/AccessControl.sol |
| `Upgraded(address)` | `0xbc7cd75a20ee27fd9adebab32041f755214dbc6bffa90cc0225b39da2e5c2d3b` | contracts/launchpad/contracts/base/ERC1967Utils.sol |

| Error | Selector | Declared |
|---|---|---|
| `AlreadyInitialized()` | `0x0dc149f0` | contracts/launchpad/contracts/base/Initializable.sol |
| `FeeOutOfRange()` | `0x0c8249ad` | contracts/launchpad/contracts/interfaces/IProtocolConfig.sol |
| `InvalidImplementation()` | `0x68155f9a` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `InvalidRoleAccount()` | `0xeccda526` | contracts/launchpad/contracts/base/AccessControl.sol |
| `LastDefaultAdmin()` | `0x9c88180d` | contracts/launchpad/contracts/base/AccessControl.sol |
| `MissingRole(address,bytes32)` | `0x0161a64a` | contracts/launchpad/contracts/base/AccessControl.sol |
| `NonPayableUpgrade()` | `0x93738d49` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `NotInitializing()` | `0xd7e6bcf8` | contracts/launchpad/contracts/base/Initializable.sol |
| `UUPSNotThroughActiveProxy()` | `0x45a93fc9` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `UUPSNotThroughProxy()` | `0x4f083858` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `UnauthorizedUpgrade()` | `0x3a617a54` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `UpgradeInitializationFailed()` | `0xcc1cf440` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `ZeroAddress()` | `0xd92e233d` | contracts/launchpad/contracts/interfaces/IProtocolConfig.sol |

### Treasury

17 functions, 8 events, 19 errors. Served at the generation 2 Treasury address over the keeper Treasury ledger (section 20).

| Function | Selector | Mutability | Returns | Declared |
|---|---|---|---|---|
| `DEFAULT_ADMIN_ROLE()` | `0xa217fddf` | view | `(bytes32)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `DEPOSITOR_ROLE()` | `0xa3b0b5a3` | view | `(bytes32)` | contracts/launchpad/contracts/protocol/Treasury.sol |
| `deposit(address,uint256)` | `0x47e7ef24` | nonpayable | `()` | contracts/launchpad/contracts/protocol/Treasury.sol |
| `eventEmitter()` | `0x9ff78c30` | view | `(address)` | contracts/launchpad/contracts/protocol/Treasury.sol |
| `getBalance(address)` | `0xf8b2cb4f` | view | `(uint256)` | contracts/launchpad/contracts/protocol/Treasury.sol |
| `getRoleAdmin(bytes32)` | `0x248a9ca3` | view | `(bytes32)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `grantRole(bytes32,address)` | `0x2f2ff15d` | nonpayable | `()` | contracts/launchpad/contracts/base/AccessControl.sol |
| `hasRole(bytes32,address)` | `0x91d14854` | view | `(bool)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `initialize(address,address)` | `0x485cc955` | nonpayable | `()` | contracts/launchpad/contracts/protocol/Treasury.sol |
| `proxiableUUID()` | `0x52d1902d` | view | `(bytes32)` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `recordDeposit(address,uint256)` | `0x1e578bae` | nonpayable | `()` | contracts/launchpad/contracts/protocol/Treasury.sol |
| `recoverSurplus(address,address,uint256)` | `0x72df7f5b` | nonpayable | `()` | contracts/launchpad/contracts/protocol/Treasury.sol |
| `registeredDefaultAdminCount()` | `0xc0cb71b6` | view | `(uint256)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `renounceRole(bytes32,address)` | `0x36568abe` | nonpayable | `()` | contracts/launchpad/contracts/base/AccessControl.sol |
| `revokeRole(bytes32,address)` | `0xd547741f` | nonpayable | `()` | contracts/launchpad/contracts/base/AccessControl.sol |
| `upgradeToAndCall(address,bytes)` | `0x4f1ef286` | payable | `()` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `withdraw(address,address,uint256)` | `0xd9caed12` | nonpayable | `()` | contracts/launchpad/contracts/protocol/Treasury.sol |

| Event | Topic 0 | Declared |
|---|---|---|
| `Deposited(address,address,uint256)` | `0x8752a472e571a816aea92eec8dae9baf628e840f4929fbcc2d155e6233ff68a7` | contracts/launchpad/contracts/interfaces/ITreasury.sol |
| `Initialized(uint64)` | `0xc7f505b2f371ae2175ee4913f4499e1f2633a7b5936321eed1cdaeb6115181d2` | contracts/launchpad/contracts/base/Initializable.sol |
| `RoleAdminChanged(bytes32,bytes32,bytes32)` | `0xbd79b86ffe0ab8e8776151514217cd7cacd52c909f66475c3af44e129f0b00ff` | contracts/launchpad/contracts/base/AccessControl.sol |
| `RoleGranted(bytes32,address,address)` | `0x2f8788117e7eff1d82e926ec794901d17c78024a50270940304540a733656f0d` | contracts/launchpad/contracts/base/AccessControl.sol |
| `RoleRevoked(bytes32,address,address)` | `0xf6391f5c32d9c69d2a47ea670b442974b53935d1edc7fd64eb21e047a839171b` | contracts/launchpad/contracts/base/AccessControl.sol |
| `SurplusRecovered(address,address,uint256)` | `0x00c571bcd43634ce4043e2bd7088ab9a1d36bd5dc21f5f4246e9eddda3c222f6` | contracts/launchpad/contracts/protocol/Treasury.sol |
| `Upgraded(address)` | `0xbc7cd75a20ee27fd9adebab32041f755214dbc6bffa90cc0225b39da2e5c2d3b` | contracts/launchpad/contracts/base/ERC1967Utils.sol |
| `Withdrawn(address,address,uint256)` | `0xd1c19fbcd4551a5edfb66d43d2e337c04837afda3482b42bdf569a8fccdae5fb` | contracts/launchpad/contracts/interfaces/ITreasury.sol |

| Error | Selector | Declared |
|---|---|---|
| `AlreadyInitialized()` | `0x0dc149f0` | contracts/launchpad/contracts/base/Initializable.sol |
| `InsufficientBalance()` | `0xf4d678b8` | contracts/launchpad/contracts/interfaces/ITreasury.sol |
| `InvalidImplementation()` | `0x68155f9a` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `InvalidRoleAccount()` | `0xeccda526` | contracts/launchpad/contracts/base/AccessControl.sol |
| `InvalidToken()` | `0xc1ab6dc1` | contracts/launchpad/contracts/protocol/Treasury.sol |
| `LastDefaultAdmin()` | `0x9c88180d` | contracts/launchpad/contracts/base/AccessControl.sol |
| `MissingRole(address,bytes32)` | `0x0161a64a` | contracts/launchpad/contracts/base/AccessControl.sol |
| `NonPayableUpgrade()` | `0x93738d49` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `NotInitializing()` | `0xd7e6bcf8` | contracts/launchpad/contracts/base/Initializable.sol |
| `ReentrantCall()` | `0x37ed32e8` | contracts/launchpad/contracts/protocol/Treasury.sol |
| `TransferFailed()` | `0x90b8ec18` | contracts/launchpad/contracts/libraries/TransferHelper.sol |
| `TransferFromFailed()` | `0x7939f424` | contracts/launchpad/contracts/libraries/TransferHelper.sol |
| `UUPSNotThroughActiveProxy()` | `0x45a93fc9` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `UUPSNotThroughProxy()` | `0x4f083858` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `Unauthorized()` | `0x82b42900` | contracts/launchpad/contracts/interfaces/ITreasury.sol |
| `UnauthorizedUpgrade()` | `0x3a617a54` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `UpgradeInitializationFailed()` | `0xcc1cf440` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `ZeroAddress()` | `0xd92e233d` | contracts/launchpad/contracts/interfaces/ITreasury.sol |
| `ZeroAmount()` | `0x1f2a2005` | contracts/launchpad/contracts/interfaces/ITreasury.sol |

### EventEmitter

63 functions, 39 events, 12 errors. The EventEmitter contracts stay deployed (section 22); the twelve launchpad events are emitted at 0x1017 (section 11).

| Function | Selector | Mutability | Returns | Declared |
|---|---|---|---|---|
| `DEFAULT_ADMIN_ROLE()` | `0xa217fddf` | view | `(bytes32)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `EMITTER_ADMIN_ROLE()` | `0xc2d11c38` | view | `(bytes32)` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `EVENT_EMITTER_ROLE()` | `0xd7056c20` | view | `(bytes32)` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `VERSION()` | `0xffa1ad74` | pure | `(string)` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `deregisterToken(address)` | `0xbab2af1d` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitAssetApproval(address,address,address,uint256,bool)` | `0x4f2bdd09` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitBestRouteSwap(address,address,address,bytes32,uint256,uint256,address[],uint256)` | `0xb3defa3c` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitCircuitBreaker(address,bytes32,uint256,uint256,uint256)` | `0xd6278d9c` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitConfigUpdated(bytes32,uint256,uint256)` | `0x85ceceb5` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitEventLog(string,(((string,address)[],(string,address[])[]),((string,uint256)[],(string,uint256[])[]),((string,int256)[],(string,int256[])[]),((string,bool)[],(string,bool[])[]),((string,bytes32)[],(string,bytes32[])[]),((string,bytes)[],(string,bytes[])[]),((string,string)[],(string,string[])[])))` | `0x906c49f6` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitEventLog1(string,bytes32,(((string,address)[],(string,address[])[]),((string,uint256)[],(string,uint256[])[]),((string,int256)[],(string,int256[])[]),((string,bool)[],(string,bool[])[]),((string,bytes32)[],(string,bytes32[])[]),((string,bytes)[],(string,bytes[])[]),((string,string)[],(string,string[])[])))` | `0x24de01e4` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitEventLog2(string,bytes32,bytes32,(((string,address)[],(string,address[])[]),((string,uint256)[],(string,uint256[])[]),((string,int256)[],(string,int256[])[]),((string,bool)[],(string,bool[])[]),((string,bytes32)[],(string,bytes32[])[]),((string,bytes)[],(string,bytes[])[]),((string,string)[],(string,string[])[])))` | `0x63d16363` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitFeeDistributed(bytes32,uint256,uint8,uint256,address)` | `0x06a7cedb` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitFeeFlow(uint8,address,address,uint256,uint256,uint256,uint256)` | `0xfa2e1d5e` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitFeeRecorded(bytes32,uint256,uint256,uint256)` | `0xd6d28200` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitFeeStrategyChanged(bytes32,uint256,uint8,uint8)` | `0xb2c8dc72` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitGovernance(uint8,bytes32,address,bytes)` | `0x98616097` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitMarketCreated(bytes32,address,address,address,address)` | `0xe4617ca5` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitNftMint(uint256,address,address,uint8)` | `0x75c48038` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitNftTransfer(address,address,address,uint256)` | `0xeeaf8bcd` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitOpticalExecuted(bytes32,address,string,bytes)` | `0xd4756325` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitOpticalLifecycle(uint8,address,address,bytes32,bytes)` | `0x82b678c3` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitOracleAdapterLifecycle(bytes32,address,uint8,uint256)` | `0x176af80c` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitPauseToggle(address,bool)` | `0xd4925be5` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitPecorOrderCreated(uint256,address,uint8,uint8,address,address,uint256,uint256,uint256,uint256)` | `0x7944994b` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitPecorOrderLifecycle(uint256,address,uint8,uint8,uint256,bytes)` | `0x1a7a34ef` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitPecorSwap(address,address,address,uint256,uint256,uint256,uint256,uint256,uint256,uint256,uint256,uint8)` | `0xa6df9eea` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitPoolRegistered(address,address,address,address,uint256)` | `0x25b14c33` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitPoolStateUpdated(bytes32,uint256,uint256,uint256,uint256)` | `0xfa78f4ca` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitPriceUpdated(address,uint256,address,uint256,uint256,bytes32)` | `0xd08aa55f` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitRoleChange(uint8,bytes32,address,address,bytes32)` | `0xcdc0fc61` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitRouterTrade(uint8,address,address,address,address,uint256,uint256,uint256)` | `0xe3cdc125` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitSwap(bytes32,address,bool,uint256,uint256,uint256,uint256)` | `0x100fca4f` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitTokenDeployed(address,address,address,bytes32,string,string,uint8,uint256)` | `0x3cced653` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitTokenTransfer(address,address,address,uint256)` | `0xc6770db6` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitTreasuryFlow(uint8,address,address,uint256)` | `0x9de5933b` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitUpgraded(address,address,uint8)` | `0x37114247` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `emitVaultFlow(uint8,address,address,uint256,uint256)` | `0x9e4a40ca` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `getRoleAdmin(bytes32)` | `0x248a9ca3` | view | `(bytes32)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `grantRole(bytes32,address)` | `0x2f2ff15d` | nonpayable | `()` | contracts/launchpad/contracts/base/AccessControl.sol |
| `hasRole(bytes32,address)` | `0x91d14854` | view | `(bool)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `initialize(address)` | `0xc4d66de8` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `isAuthorizedEmitter(address)` | `0x66706fd2` | view | `(bool)` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `isRegisteredToken(address)` | `0x7639a230` | view | `(bool)` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `isTypedEmitter(address,bytes4)` | `0x48c3830a` | view | `(bool)` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `metaAGRouter()` | `0xfa0dced9` | view | `(address)` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `opticalRegistry()` | `0x73b5708c` | view | `(address)` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `poolRegistry()` | `0xafcff50f` | view | `(address)` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `proxiableUUID()` | `0x52d1902d` | view | `(bytes32)` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `registerToken(address,address)` | `0x4739f7e5` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `registeredDefaultAdminCount()` | `0xc0cb71b6` | view | `(uint256)` | contracts/launchpad/contracts/base/AccessControl.sol |
| `reinitializeV2(address)` | `0xbe6b99e3` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `renounceRole(bytes32,address)` | `0x36568abe` | nonpayable | `()` | contracts/launchpad/contracts/base/AccessControl.sol |
| `revokeRole(bytes32,address)` | `0xd547741f` | nonpayable | `()` | contracts/launchpad/contracts/base/AccessControl.sol |
| `setAuthorizedEmitter(address,bool)` | `0x27b81718` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `setMetaAGRouter(address)` | `0xf4795b26` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `setOpticalRegistry(address)` | `0xde2b7764` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `setPoolRegistry(address)` | `0x7b77cd6a` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `setSidioraFactory(address)` | `0xe53122ec` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `setTokenRegistry(address)` | `0x35a5af92` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `setTypedEmitter(address,bytes4,bool)` | `0xb2473503` | nonpayable | `()` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `sidioraFactory()` | `0xb182ba64` | view | `(address)` | contracts/launchpad/contracts/data/EventEmitter.sol |
| `upgradeToAndCall(address,bytes)` | `0x4f1ef286` | payable | `()` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |

| Event | Topic 0 | Declared |
|---|---|---|
| `AssetApproval(address,address,address,uint256,bool,uint256,uint256)` | `0xa9de58179a7ba626557514c856f27ad734eea61c56c3e9acb937f374ec63afe7` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `BestRouteSwap(address,address,address,bytes32,uint256,uint256,address[],uint256,uint256,uint256)` | `0xf5041a4d1e1f497822428a20e208ea0ad2c90eb99518d0d9a1f598f8af04cd72` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `CircuitBreakerTriggered(address,bytes32,uint256,uint256,uint256,uint256,uint256)` | `0xb08f12d79c5828345938b295337ebad6aa44441a33df36e9f766b0b7f253889b` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `ConfigUpdated(bytes32,uint256,uint256,uint256,uint256)` | `0x2903b3642d2105b7fbd127326de888ab50e8d975bf819398337fb58407c1b10e` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `ContractUpgraded(address,address,uint8,uint256,uint256)` | `0xb059389b6a1d06bc1833ca4b78798c2642d02b2d03d0f8fce5f4b8299737d3b7` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `EventLog(address,bytes32,string,(((string,address)[],(string,address[])[]),((string,uint256)[],(string,uint256[])[]),((string,int256)[],(string,int256[])[]),((string,bool)[],(string,bool[])[]),((string,bytes32)[],(string,bytes32[])[]),((string,bytes)[],(string,bytes[])[]),((string,string)[],(string,string[])[])),uint256,uint256)` | `0x3f5c3dc69ea622ef6e129f03f630c1484849db8ae3e8f3d7fa164ca7fae08c27` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `EventLog1(address,bytes32,bytes32,string,(((string,address)[],(string,address[])[]),((string,uint256)[],(string,uint256[])[]),((string,int256)[],(string,int256[])[]),((string,bool)[],(string,bool[])[]),((string,bytes32)[],(string,bytes32[])[]),((string,bytes)[],(string,bytes[])[]),((string,string)[],(string,string[])[])),uint256,uint256)` | `0xe67d7d5cff44135efdb2aa905b4fcd19c700c05482dd08ccb52a0b02b99f788a` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `EventLog2(address,bytes32,bytes32,bytes32,string,(((string,address)[],(string,address[])[]),((string,uint256)[],(string,uint256[])[]),((string,int256)[],(string,int256[])[]),((string,bool)[],(string,bool[])[]),((string,bytes32)[],(string,bytes32[])[]),((string,bytes)[],(string,bytes[])[]),((string,string)[],(string,string[])[])),uint256,uint256)` | `0xfb0f08ac39934a2d9d9120c47f7079c97667c3a5495f5a6733b46b2c87cd2502` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `FeeDistributed(bytes32,uint256,uint8,uint256,address,uint256,uint256)` | `0xf7ab826cedf9d005102eddf95e65f43597aef36a55ff09ee4f452204c83f4afc` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `FeeFlow(uint8,address,address,uint256,uint256,uint256,uint256,uint256,uint256)` | `0x626e3fd5d9323ca4b7ffa3ca51c21c02766b638d15eb8896a36a4e0e97ae6de4` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `FeeRecorded(bytes32,uint256,uint256,uint256,uint256,uint256)` | `0xe80e9e3e09460d716260f5ce6979a59f6cf4fbc1096e20f008d78b5cac7b7179` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `FeeStrategyChanged(bytes32,uint256,uint8,uint8,uint256,uint256)` | `0x337fd9f9304e9da4c04886a5b8bf56e74032b459758212a860bc61d3544353dd` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `Governance(uint8,bytes32,address,bytes,uint256,uint256)` | `0x637d9c5f725b51336e8338a6c85951604e8ee7bc041e8e2be8dc739abe9d2173` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `Initialized(uint64)` | `0xc7f505b2f371ae2175ee4913f4499e1f2633a7b5936321eed1cdaeb6115181d2` | contracts/launchpad/contracts/base/Initializable.sol |
| `MarketCreated(bytes32,address,address,address,address,uint256,uint256)` | `0xb0a9c7605389f1127c2299d4a3ed85f0b2fc19894ab5411d76b453bb29f9f47b` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `NftMint(uint256,address,address,uint8,uint256,uint256)` | `0xd2917d37031c0dfe5016665c7a83360cbf2ff32063096b2caf61ba66c06832e5` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `NftTransfer(address,address,address,uint256,uint256,uint256)` | `0x021f03e8a1315704c74c03e8c022c12bc80c4a2bd4fec8bd36b217322c340ad3` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `OpticalExecuted(bytes32,address,string,bytes,uint256,uint256)` | `0xfdf5ccbb26a02ea4efd5415e00b1d7782ca04cbb6b33a0d0a0d549ba284f9e57` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `OpticalLifecycle(uint8,address,address,bytes32,bytes,uint256,uint256)` | `0xa3faac07b1221710e43ca60800fc18acd83007ed48055def89c1e69b79ba44cb` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `OracleAdapterLifecycle(bytes32,address,uint8,uint256,uint256,uint256)` | `0x9f8c04497ebc19f8f7de480e55fdfa3780212516de2d597e6f793d6629017cec` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `PauseToggle(address,bool,uint256,uint256)` | `0xd079c1af4038467292233cbd5af2e60da5c45ea48670f905aff3da18b483973c` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `PecorOrderCreated(uint256,address,uint8,uint8,address,address,uint256,uint256,uint256,uint256,uint256,uint256)` | `0xa24904fab3b2e5505845a72e1cd3b798ad4fbd48a7e6cb662de4b0d0d5abb298` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `PecorOrderLifecycle(uint256,address,uint8,uint8,uint256,bytes,uint256,uint256)` | `0xdbbb8b49248d8ae078049644637ec9edf159baa1c653f8274ddf3fa26f358d26` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `PecorSwap(address,address,address,uint256,uint256,uint256,uint256,uint256,uint256,uint256,uint256,uint8,uint256,uint256)` | `0x9b4f5f387e6b6c21cc8d67c37c41d9d7b8d8b3f16494c84db2dd1fca812a644f` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `PoolRegistered(address,address,address,address,uint256,uint256,uint256)` | `0xda5568642b67660133b1de538c867c58f3037a5722b0e231a959ee8ca11b2ed7` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `PoolStateUpdated(bytes32,uint256,uint256,uint256,uint256,uint256,uint256)` | `0x8155a4b38a877d212b271811786986f8f7d78074c82fce4526c58defcc6d7ebd` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `PriceUpdated(address,uint256,address,uint256,uint256,bytes32,uint256,uint256)` | `0x8d99dbd92dc7116e69aa285daa09f2da514d6b2ca202545c3d9a0385982a7a69` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `RoleAdminChanged(bytes32,bytes32,bytes32)` | `0xbd79b86ffe0ab8e8776151514217cd7cacd52c909f66475c3af44e129f0b00ff` | contracts/launchpad/contracts/base/AccessControl.sol |
| `RoleChange(uint8,bytes32,address,address,bytes32,uint256,uint256)` | `0xdcb006502b86d8ebfada5d7606648e10555d5df4a42fe9a8fcd00999c25faaf4` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `RoleGranted(bytes32,address,address)` | `0x2f8788117e7eff1d82e926ec794901d17c78024a50270940304540a733656f0d` | contracts/launchpad/contracts/base/AccessControl.sol |
| `RoleRevoked(bytes32,address,address)` | `0xf6391f5c32d9c69d2a47ea670b442974b53935d1edc7fd64eb21e047a839171b` | contracts/launchpad/contracts/base/AccessControl.sol |
| `RouterTrade(uint8,address,address,address,address,uint256,uint256,uint256,uint256,uint256)` | `0x17c1899a3fd431f2b2200b90e738d270bb856a4222040c5650bfa208acb3b650` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `Swap(bytes32,address,bool,uint256,uint256,uint256,uint256,uint256,uint256)` | `0xa234ab85728295548823d8a26077ecc77aa5815f9ed8f4c5e6fd390ac851d4cc` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `TokenDeployed(address,address,address,bytes32,string,string,uint8,uint256,uint256,uint256)` | `0xad187874b793634c55d977f35d4828526eb48170a9531c71d88f8fd10cf9c237` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `TokenTransfer(address,address,address,uint256,uint256,uint256)` | `0xd1630e8dfeca8f80cedf002590087ccd08ffc4a5a1860fc02892098e7aa92de1` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `TreasuryFlow(uint8,address,address,uint256,uint256,uint256)` | `0x589609a9576cbe900d4afeecbd83764a5db719055826446f88c74bee62af7f81` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `TypedEmitterPermission(address,bytes4,bool)` | `0xac8cad594581b394b23d152f0d6fe73d7d6611afa3458ccc51e4659ff7dbc844` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `Upgraded(address)` | `0xbc7cd75a20ee27fd9adebab32041f755214dbc6bffa90cc0225b39da2e5c2d3b` | contracts/launchpad/contracts/base/ERC1967Utils.sol |
| `VaultFlow(uint8,address,address,uint256,uint256,uint256,uint256)` | `0x4a928ce96ccd98c06c706d2dfc9e3be829d83d7df95db10dfe9d1c359ea54e78` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |

| Error | Selector | Declared |
|---|---|---|
| `AlreadyInitialized()` | `0x0dc149f0` | contracts/launchpad/contracts/base/Initializable.sol |
| `InvalidImplementation()` | `0x68155f9a` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `InvalidRoleAccount()` | `0xeccda526` | contracts/launchpad/contracts/base/AccessControl.sol |
| `LastDefaultAdmin()` | `0x9c88180d` | contracts/launchpad/contracts/base/AccessControl.sol |
| `MissingRole(address,bytes32)` | `0x0161a64a` | contracts/launchpad/contracts/base/AccessControl.sol |
| `NonPayableUpgrade()` | `0x93738d49` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `NotInitializing()` | `0xd7e6bcf8` | contracts/launchpad/contracts/base/Initializable.sol |
| `UUPSNotThroughActiveProxy()` | `0x45a93fc9` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `UUPSNotThroughProxy()` | `0x4f083858` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `Unauthorized()` | `0x82b42900` | contracts/launchpad/contracts/interfaces/IEventEmitter.sol |
| `UnauthorizedUpgrade()` | `0x3a617a54` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |
| `UpgradeInitializationFailed()` | `0xcc1cf440` | contracts/launchpad/contracts/base/UUPSUpgradeable.sol |

### LaunchpadOptical

30 functions, 5 events, 10 errors. Not served by the precompile: the optical contracts stay deployed and the precompile calls their hooks by nested calls (section 13).

| Function | Selector | Mutability | Returns | Declared |
|---|---|---|---|---|
| `MAX_CAPITAL_RAISE_BPS()` | `0x87993a4e` | view | `(uint256)` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `MAX_CLIFF_DURATION()` | `0x5300a4a4` | view | `(uint256)` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `MAX_RAISE_DURATION()` | `0xf43ae507` | view | `(uint256)` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `MAX_TEAM_WALLETS()` | `0xd0fe7ca2` | view | `(uint256)` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `MAX_VESTING_DURATION()` | `0x7cd0814b` | view | `(uint256)` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `accumulatedUsdl(address)` | `0xa7746391` | view | `(uint256)` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `afterFeeDistribution(address,uint256)` | `0xae3bd4bd` | nonpayable | `(bytes4)` | contracts/launchpad/contracts/opticals/BaseOptical.sol |
| `afterSwap(address,address,bool,uint256,uint256)` | `0xf74c1c1a` | nonpayable | `(bytes4)` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `beforeFeeDistribution(address,uint256)` | `0xf80d2cc7` | nonpayable | `(uint256)` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `beforeSwap(address,address,bool,uint256)` | `0xae7e71d9` | nonpayable | `(bool,int256)` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `beforeTokenTransfer(address,address,address,uint256)` | `0x37be75a0` | nonpayable | `()` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `capitalRaiseBps()` | `0xc6532ff3` | view | `(uint256)` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `capitalRaiseDuration()` | `0x71f35332` | view | `(uint256)` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `claimCapital(address)` | `0xa9b00985` | nonpayable | `()` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `cliffDuration()` | `0xd85349f7` | view | `(uint256)` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `creator()` | `0x02d05d3f` | view | `(address)` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `feeAccumulator()` | `0x5eaea35f` | view | `(address)` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `getCapitalRaiseInfo(address)` | `0x79ec5214` | view | `(uint256,uint256,uint256,bool)` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `getFlags()` | `0x6cc7149d` | pure | `(uint8)` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `getVestingInfo(address,address)` | `0xffc7b095` | view | `(bool,uint256,uint256,uint256,uint256,uint256)` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `isTeamWallet(address)` | `0x4a5ecb36` | view | `(bool)` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `isVested(address)` | `0x760a2e8a` | view | `(bool)` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `owner()` | `0x8da5cb5b` | view | `(address)` | contracts/launchpad/contracts/opticals/BaseOptical.sol |
| `poolRegistry()` | `0xafcff50f` | view | `(address)` | contracts/launchpad/contracts/opticals/BaseOptical.sol |
| `poolStartTime(address)` | `0x56fcdcb4` | view | `(uint256)` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `teamClaimAddress()` | `0x4ccb398e` | view | `(address)` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `teamWalletCount()` | `0x2c420122` | view | `(uint256)` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `tokensSold(address,address)` | `0x9f1245ad` | view | `(uint256)` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `totalRaised(address)` | `0x30d373f5` | view | `(uint256)` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `vestingDuration()` | `0x1514617e` | view | `(uint256)` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |

| Event | Topic 0 | Declared |
|---|---|---|
| `CapitalRaiseAccumulated(address,uint256,uint256)` | `0x48b3413afdad030150d68e8ed719b41dd32593b330f034158efe3134ffde0dae` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `CapitalRaiseClaimed(address,uint256,address)` | `0xe8067320421baf056a2b16baecdc6fff89d3e90e2642fd0ab434b9845d8702c1` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `PoolStartTimeRecorded(address,uint256)` | `0xe604181e9b1b472a9fd7581a81193c352a32c4a286112bcad619d427b19257c9` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `SellBlockedCliff(address,address,uint256)` | `0x4acfead499d5bfd950d77435a2671732e5e612aba3b219ba1f129c48829b502b` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `SellBlockedVesting(address,address,uint256,uint256)` | `0xb3966d45988315ed5a8dc1f14ca6aa97a4d2c0615f4a43cdc146990a715a4069` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |

| Error | Selector | Declared |
|---|---|---|
| `CapitalRaiseTooHigh()` | `0xf7fc11bd` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `CliffActive()` | `0x1392d261` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `DurationTooLong()` | `0x9529f506` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `NoTeamWallets()` | `0x1985bc9e` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `NotOwner()` | `0x30cd7471` | contracts/launchpad/contracts/opticals/BaseOptical.sol |
| `NotPool()` | `0x6f61f641` | contracts/launchpad/contracts/opticals/BaseOptical.sol |
| `NotTeam()` | `0xe9f3e974` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `NothingToClaim()` | `0x969bf728` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `TooManyTeamWallets()` | `0x1f4941f9` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |
| `VestingExceeded()` | `0x45446fb4` | contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol |

The served interface uses one ABI file per contract role and a golden selector file:

- precompiles/launchpad/abi/ holds the compiled ABI of eleven role files whose sources are in `contracts/launchpad/contracts`: the ten served roles SidioraPool, SidioraFactory, SidioraNFT, Router, Quoter, FeesRouter, FeeAccumulator, PoolRegistry, ProtocolConfig and Treasury (contracts/launchpad/contracts/protocol/Treasury.sol), and SidioraERC20, which is not served but encodes the nested calls the precompile makes to pool-token contracts; each file is loaded with MustGetABI (precompiles/common/precompiles.go) through the embed.FS of precompiles/launchpad/launchpad.go.
- precompiles/launchpad/testdata/abi_golden.txt records 612 lines: role methods, events and distinct errors, 16 EventEmitter events, 13 ChainLaunch methods and the two chain-defined errors. Treasury contributes 44 entries. Each method/error line includes its selector and each event its topic0.
- precompiles/launchpad/abi.json serves direct calls to 0x1017. It combines the 113 distinct role selectors in section 3 with the 13 methods of precompiles/launchpad/abi/ChainLaunch.json. Event and error definitions are included in the ABI. The v6.10 interface is retained under precompiles/launchpad/legacy/v610.
- Two signatures with one selector never appear in one role or at 0x1017; the golden test asserts the selector set of each role and of 0x1017 has no duplicate.

## 3. Direct calls to 0x1017

A call sent to 0x1017 itself resolves the selector in role order Router, SidioraFactory, Quoter, PoolRegistry, FeesRouter, FeeAccumulator, ProtocolConfig; the first role that declares it serves it. This gives 113 distinct role selectors. The 13 chain-launch selectors are resolved after these roles, for 126 direct methods in total. 21 signatures are declared by several roles:

| Signature | Selector | Served by | Also declared by |
|---|---|---|---|
| `DEFAULT_ADMIN_ROLE()` | `0xa217fddf` | Router | SidioraFactory, Quoter, PoolRegistry, FeesRouter, FeeAccumulator, ProtocolConfig |
| `createMarket(string,string,uint8,address)` | `0x57c0d7fd` | Router | SidioraFactory |
| `getRoleAdmin(bytes32)` | `0x248a9ca3` | Router | SidioraFactory, Quoter, PoolRegistry, FeesRouter, FeeAccumulator, ProtocolConfig |
| `grantRole(bytes32,address)` | `0x2f2ff15d` | Router | SidioraFactory, Quoter, PoolRegistry, FeesRouter, FeeAccumulator, ProtocolConfig |
| `hasRole(bytes32,address)` | `0x91d14854` | Router | SidioraFactory, Quoter, PoolRegistry, FeesRouter, FeeAccumulator, ProtocolConfig |
| `poolRegistry()` | `0xafcff50f` | Router | SidioraFactory, Quoter, FeesRouter, FeeAccumulator |
| `protocolConfig()` | `0xf5efbb4f` | Router | SidioraFactory, Quoter, FeeAccumulator |
| `proxiableUUID()` | `0x52d1902d` | Router | SidioraFactory, Quoter, PoolRegistry, FeesRouter, FeeAccumulator, ProtocolConfig |
| `registeredDefaultAdminCount()` | `0xc0cb71b6` | Router | SidioraFactory, Quoter, PoolRegistry, FeesRouter, FeeAccumulator, ProtocolConfig |
| `renounceRole(bytes32,address)` | `0x36568abe` | Router | SidioraFactory, Quoter, PoolRegistry, FeesRouter, FeeAccumulator, ProtocolConfig |
| `revokeRole(bytes32,address)` | `0xd547741f` | Router | SidioraFactory, Quoter, PoolRegistry, FeesRouter, FeeAccumulator, ProtocolConfig |
| `upgradeToAndCall(address,bytes)` | `0x4f1ef286` | Router | SidioraFactory, Quoter, PoolRegistry, FeesRouter, FeeAccumulator, ProtocolConfig |
| `usdlAddress()` | `0x2e10f976` | Router | SidioraFactory, FeeAccumulator, ProtocolConfig |
| `eventEmitter()` | `0x9ff78c30` | SidioraFactory | PoolRegistry, FeeAccumulator, ProtocolConfig |
| `getAllPools(uint256,uint256)` | `0xa7753f09` | Quoter | PoolRegistry |
| `getPoolsByCreator(address)` | `0xc84728ef` | Quoter | PoolRegistry |
| `feeAccumulator()` | `0x5eaea35f` | SidioraFactory | FeesRouter |
| `nftContract()` | `0xd56d229d` | SidioraFactory | FeesRouter |
| `FACTORY_ROLE()` | `0x04a0fb17` | PoolRegistry | FeeAccumulator |
| `treasury()` | `0x61d027b3` | SidioraFactory | FeeAccumulator |
| `initialize(address,address,address)` | `0xc0c53b8b` | Quoter | ProtocolConfig |

No two different signatures share a selector.

The Treasury and SidioraNFT roles are not direct roles: they are served only at their generation 2 surface addresses. Pool functions are served only at pool addresses.

### Chain launch and revenue

Direct calls also expose the chain-launch ABI in precompiles/launchpad/abi/ChainLaunch.json, implemented by precompiles/launchpad/chainlaunch.go. launchChain and launchChainWithPermit accept native value; other methods reject it. The keeper records the pool, token, fee-right id, creator, registry number and chain id. Revenue reads and claims use native amounts expressed in EVM units.

| Method | Selector | Mutability |
|---|---|---|
| `launchChain(string,string,uint8,address,string,uint64,uint64,uint16)` | `0x1022e4cc` | payable |
| `launchChainWithPermit(string,string,uint8,address,string,uint64,uint64,uint16,(uint256,uint256,uint8,bytes32,bytes32))` | `0x0bac6d3e` | payable |
| `openChainRevenueEpoch(uint32)` | `0xc1422f63` | nonpayable |
| `claimChainRevenue(uint32,uint64)` | `0xf31a46ac` | nonpayable |
| `claimChainRevenueBatch(uint32,uint64[])` | `0xae3572e8` | nonpayable |
| `pauseLaunchedChain(uint32)` | `0x378c575a` | nonpayable |
| `resumeLaunchedChain(uint32)` | `0x3d183bdf` | nonpayable |
| `retireLaunchedChain(uint32)` | `0x554c12a4` | nonpayable |
| `launchedChain(uint32)` | `0x49e9a8e7` | view |
| `launchedChainOfPool(address)` | `0x8a092d44` | view |
| `homeChainOf(address)` | `0x81d1d524` | view |
| `chainRevenueEpoch(uint32,uint64)` | `0xcd8063f7` | view |
| `chainRevenueClaimable(uint32,uint64,address)` | `0xaea55ee3` | view |

## 4. Surfaces

- A registered surface selects its role ABI at 0x1017. The precompile accepts an appended original caller only from a surface registered in the keeper; other callers are decoded using their complete calldata and their EVM caller address (`precompiles/launchpad/run.go`).
- `InstallRoleSurfaces` installs the nine generation 2 role addresses from `modules/launchpad/keeper/surfaces.go`. It writes the 51-byte `SurfaceRuntime` defined in `modules/launchpad/keeper/market.go`: `0x3415600957600080fd5b363d3d373360601b36523d3d601436013d3d6110175af13d600060003e602e573d6000fd5b3d6000f3`. This runtime rejects native value, copies calldata, appends CALLER and calls 0x1017, returning or reverting with its data. New market creation installs the same runtime at the pool address.
- `MigrateV6120` installs the fixed 41-byte pool runtime `0x363d3d373360601b36523d3d601436013d3d6110175af13d600060003e6024573d6000fd5b3d6000f3` through `installPoolSurface` in `modules/launchpad/keeper/migrate_v6120.go`, then installs the generation 2 role surfaces. Pool-token contracts and USDL keep their code.
- Retired generation 1 role addresses receive a separate runtime that reverts with `Retired(replacement)` for every call (`precompiles/launchpad/retired.go`). Generation 1 pool addresses remain registered pool surfaces.
- A surface call resolves through its role ABI: pool, collection or the role of the registered address. Generation 2 role surfaces serve every generation's markets. Direct calls to 0x1017 resolve in Router, SidioraFactory, Quoter, PoolRegistry, FeesRouter, FeeAccumulator, ProtocolConfig order, followed by the chain-launch methods.
- Delegatecall into 0x1017 and writes under staticcall revert. View selectors work under staticcall. Nonpayable methods reject attached value; direct `launchChain` and `launchChainWithPermit` calls accept native value (`precompiles/launchpad/run.go`).
- An unknown selector at a surface or at 0x1017 reverts with empty data. Genesis exports and imports the registered surface set (`modules/launchpad/keeper/genesis.go`).

## 5. Quote asset

- The quote asset is the USDL ERC20 contract at 0x85FcD13735F4309833A503EE804ea32395851479 (sdk/launchpad/shared/config/deployment/current.json), six decimals (sdk/launchpad/shared/config/deployment/current.json), named by Params.QuoteToken; it has no bank denom and no native pointer, and its code and storage stay the contract's.
- The launchpad module EVM address holds the launchpad USDL as an ERC20 balance, split in the keeper ledgers HeldQuote per pool, FeeHeld and TreasuryHeld; USDL.balanceOf(launchpad module EVM address) covers their sum.
- USDL moves only by nested ERC20 calls: transferFrom(user, launchpad module EVM address, amount) and transfer(to, amount) with caller the launchpad module EVM address inside the precompile run through evm.Call (precompiles/xweb/xweb.go), and CallEVM (modules/evm/keeper/evm.go) in the upgrade handler; moves between ledgers write no log.
- Router and Factory surfaces pull the caller's USDL with the allowance granted to 0x1017; allowances granted to the old Router contracts stay on USDL but are not spent, so a user grants a new allowance to 0x1017 once.
- Params.QuoteToken (modules/launchpad/types/params.go), encoded as quote_token in api/launchpad/tx.proto, names the USDL contract 0x85FcD13735F4309833A503EE804ea32395851479. MigrateV6120 sets QuoteToken to this contract; USDL has no bank denom, bank metadata or native pointer.
- USDL keeps its code, storage, six decimals, allowances and permit nonces; the launchpad never mints, burns or writes USDL storage, and every USDL movement is a call into the USDL contract: transferFrom(user, launchpad module EVM address, amount) with msg.sender 0x1017 against the allowance the user grants to 0x1017, and transfer(to, amount) with caller the launchpad module EVM address or a pool address, as nested calls of the precompile run (the nested-call form of precompiles/xweb/xweb.go) or through CallEVM in the upgrade handler (modules/evm/keeper/evm.go).
- The keeper holds custody ledgers for the USDL at the launchpad module EVM address: HeldQuote per pool, FeeHeld for the generation 2 FeeAccumulator, and TreasuryLedger and TreasuryHeld per token for the Treasury; USDL.balanceOf(launchpad module EVM address) is at least the sum of every HeldQuote, FeeHeld and TreasuryHeld(USDL), and moves between a pool, the FeeAccumulator and the Treasury are ledger moves inside custody that emit no USDL Transfer log.
- Pay-first input keeps working: before swap, swapFor and syncReserves the precompile sweeps the pool address with a nested call whose caller is the pool address, USDL.transfer(launchpad module EVM address, USDL.balanceOf(pool)) when the balance is nonzero, and adds the amount to HeldQuote(pool), so USDL sent straight to the pool address before a swap counts as prepaid input as at contracts/launchpad/contracts/core/SidioraPool.sol; the generation 2 FeeAccumulator address and the Treasury address are swept the same way before every balance read of theirs.
- Pool tokens stay at the pool address: sell input arrives by token.transferFrom(user, pool) or by a prior transfer to the pool, buy output leaves by token.transfer(recipient, amount) with caller the pool address (contracts/launchpad/contracts/core/SidioraPool.sol), and the pool balance checks read token.balanceOf(pool) through a nested staticcall.
- Router and Factory pulls use allowances to 0x1017: USDL.transferFrom(caller, launchpad module EVM address, amount) for buys and creation fees (contracts/launchpad/contracts/periphery/Router.sol, contracts/launchpad/contracts/periphery/Router.sol, contracts/launchpad/contracts/core/SidioraFactory.sol) and token.transferFrom(caller, pool, amount) for sells and the first leg of a token-for-token swap (contracts/launchpad/contracts/periphery/Router.sol, contracts/launchpad/contracts/periphery/Router.sol); the *WithPermit variants call permit(holder, 0x1017, value, deadline, v, r, s) on the token contract through a nested call and ignore its failure as contracts/launchpad/contracts/periphery/Router.sol does, then pull; the intermediate USDL of a token-for-token swap (contracts/launchpad/contracts/periphery/Router.sol) is a ledger move from HeldQuote of the first pool to HeldQuote of the second.
- Pool and FeeAccumulator USDL payouts are real USDL.transfer calls from the launchpad module EVM address to the recipient: sell output (contracts/launchpad/contracts/core/SidioraPool.sol), claim, burn to `0x000000000000000000000000000000000000dEaD`, airdrop claims, optical surplus claims and legacy airdrop claims (contracts/launchpad/contracts/data/FeeAccumulator.sol, contracts/launchpad/contracts/data/FeeAccumulator.sol, contracts/launchpad/contracts/data/FeeAccumulator.sol, contracts/launchpad/contracts/data/FeeAccumulator.sol, contracts/launchpad/contracts/data/FeeAccumulator.sol); the buy fee is a ledger move from HeldQuote(pool) to FeeHeld (contracts/launchpad/contracts/core/SidioraPool.sol) and sendLpRewards a ledger move from FeeHeld to HeldQuote(pool) (contracts/launchpad/contracts/data/FeeAccumulator.sol).

## 6. Swap

- Swap checks in contract order: deadline (contracts/launchpad/contracts/core/SidioraPool.sol), zero input (contracts/launchpad/contracts/core/SidioraPool.sol), zero recipient (contracts/launchpad/contracts/core/SidioraPool.sol), prepaid input (contracts/launchpad/contracts/core/SidioraPool.sol), then the optical beforeSwap reduction rules (contracts/launchpad/contracts/core/SidioraPool.sol), each reverting with the same custom error.
- The fee is taken on the input with mulDiv(amountIn, feeBps, 10000) (contracts/launchpad/contracts/core/SidioraPool.sol); a buy prices amountIn minus fee against virtual plus real USDL (contracts/launchpad/contracts/core/SidioraPool.sol), moves the fee from HeldQuote(pool) to FeeHeld of the generation 2 FeeAccumulator and records it (contracts/launchpad/contracts/core/SidioraPool.sol); a sell keeps fee tokens in the pool (contracts/launchpad/contracts/core/SidioraPool.sol).
- A sell whose output exceeds real USDL reverts VirtualFloorBreached() (contracts/launchpad/contracts/core/SidioraPool.sol); a buy whose output exceeds the token reserve reverts InsufficientLiquidity() (contracts/launchpad/contracts/core/SidioraPool.sol); zero output reverts InsufficientInput() and output below minAmountOut reverts SlippageExceeded() (contracts/launchpad/contracts/core/SidioraPool.sol).
- USDL volume is paidInput on buys and amountOut on sells, added to cumulativeVolume and to the buy or sell cumulative field (contracts/launchpad/contracts/core/SidioraPool.sol); the price snapshot is taken only when volume is at least 1 bps of effective USDL and 60 seconds passed since the last snapshot (contracts/launchpad/contracts/core/SidioraPool.sol), into an eight-slot ring.
- After the swap the precompile emits InputAdjustment when paidInput differs from amountIn (contracts/launchpad/contracts/core/SidioraPool.sol), calls afterSwap and requires its selector (contracts/launchpad/contracts/core/SidioraPool.sol), reverts InvalidBalance() unless HeldQuote(pool) is at least realUsdlBalance and token.balanceOf(pool) is at least tokenReserve (contracts/launchpad/contracts/core/SidioraPool.sol), and emits Swap at 0x1017 (contracts/launchpad/contracts/core/SidioraPool.sol).
- The dynamic fee uses ProtocolConfig parameters with the holder concentration term fixed at 0 (contracts/launchpad/contracts/core/SidioraPool.sol) through FeeLib (contracts/launchpad/contracts/libraries/FeeLib.sol), which the keeper implements in CalculateDynamicFee (modules/launchpad/types/math.go).
- syncReserves sweeps the pool address, sets realUsdlBalance to HeldQuote(pool) and tokenReserve to token.balanceOf(pool), and emits PoolStateUpdated at 0x1017 (contracts/launchpad/contracts/core/SidioraPool.sol).

## 7. Market creation

- createMarket through the Router or Factory surface or 0x1017 validates name and symbol (modules/launchpad/types/market.go) and the virtual defaults, fee strategy and optical code as contracts/launchpad/contracts/core/SidioraFactory.sol does (reverting ZeroAddress()), and charges the creation fee by USDL.transferFrom(caller, launchpad module EVM address, creationFee) credited to TreasuryLedger and TreasuryHeld of the generation 2 Treasury address read from SidioraFactory slot 6 (contracts/launchpad/contracts/core/SidioraFactory.sol) and recorded in modules/launchpad/keeper/testdata/v6120/layout.json, with Deposited(USDL, depositor, creationFee) emitted at the Treasury address (contracts/launchpad/contracts/periphery/Router.sol, contracts/launchpad/contracts/core/SidioraFactory.sol); a failed pull reverts with the USDL revert data wrapped as TransferFromFailed() as TransferHelper does.
- The pool token is a real SidioraERC20 contract created by a nested CREATE2 with caller the generation 2 SidioraFactory address 0x3993…8830 (sdk/launchpad/shared/config/deployment/current.json), salt keccak256(abi.encodePacked(creator, name, symbol, nonce)) with the per-creator nonce of contracts/launchpad/contracts/core/SidioraFactory.sol, and init code SidioraERC20(name, symbol, virtualToken, factory) (contracts/launchpad/contracts/core/SidioraFactory.sol), so its deploymentFactory is 0x3993…8830 (sdk/launchpad/shared/config/deployment/current.json) and its whole supply starts at that address with six decimals (contracts/launchpad/contracts/core/SidioraERC20.sol).
- The pool address is the exact address predictPoolAddress(token, creator, optical) returns (contracts/launchpad/contracts/core/SidioraFactory.sol): CREATE2 from 0x3993…8830 (sdk/launchpad/shared/config/deployment/current.json) with salt keccak256(abi.encodePacked(token)) over the beacon proxy init code of 0x0c60…71BE (sdk/launchpad/shared/config/deployment/current.json) and initialize calldata built with the generation 2 role addresses (contracts/launchpad/contracts/core/SidioraFactory.sol); the precompile installs the 51-byte SurfaceRuntime there with SetCode (modules/evm/keeper/code.go) and nonce 1, and when the address already holds code or a nonzero nonce the call reverts with empty revert data as contracts/launchpad/contracts/core/SidioraFactory.sol does.
- In the contract order of contracts/launchpad/contracts/core/SidioraFactory.sol the precompile authorizes the pool at the generation 2 FeeAccumulator, mints the fee right in the generation 2 collection at nextTokenId (slot 11) with its strategy (slot 12) and pool (slot 13), writes the PoolRegistry metadata creator, token, optical, nftId, createdAt and createdBlock (contracts/launchpad/contracts/interfaces/IPoolRegistry.sol) and the keeper market record, then runs bindPool(pool) and transfer(pool, virtualToken) on the token with caller 0x3993…8830 (sdk/launchpad/shared/config/deployment/current.json) (contracts/launchpad/contracts/core/SidioraFactory.sol), so the token transfer reads opticalAddress() from a pool that already exists (contracts/launchpad/contracts/core/SidioraERC20.sol).
- Market creation emits Initialized(1) at the pool address, PoolNFTMinted and Transfer at the collection, NftMint, PoolRegistered, TokenDeployed and MarketCreated of the EventEmitter family at 0x1017, and MarketCreated(address,address,address,uint256,address) at 0x3993…8830 (sdk/launchpad/shared/config/deployment/current.json) (contracts/launchpad/contracts/core/SidioraFactory.sol); the token contract emits its own ERC20 Transfer logs; no pointer is registered, so the pointer gas term is 0 for market creation.

## 8. Migration record

- MigrateV6120 runs step 1 (USDL custody), step 2 (pools and pool tokens), step 3 (fee rights, roles, accounting, pause state) and step 4 (surfaces and retirement) in one cached context, once.
- Step 1 moves the USDL of every pool and FeeAccumulator to the launchpad module EVM address by USDL.transfer through CallEVM from the holding address, drains both Treasuries by withdraw and recoverSurplus through CallEVM from the fixture admin, and credits the ledgers; USDL code and storage outside those balances are unchanged.
- Step 2 enumerates pools from each PoolRegistry (contracts/launchpad/contracts/data/PoolRegistry.sol), reads every pool slot, sets reserves to min(book, held) and records the book-held delta with no top-up.
- DEFAULT_ADMIN_ROLE memberships, the factory link of migrateFactory and the old contract storage behind the surfaces are not carried; pool-token storage, USDL allowances and nonces stay in their contracts.

### Step 1, USDL custody

- Step 1 reads USDL.balanceOf of every pool address of generations 1 and 2, of both FeeAccumulator addresses and of both Treasury addresses (0x15405D535ce533BfFb98c83e42f4DD242AA5e079 (sdk/launchpad/protocol/addresses.go) and the generation 2 Treasury address read from SidioraFactory slot 6 (contracts/launchpad/contracts/core/SidioraFactory.sol) and recorded in modules/launchpad/keeper/testdata/v6120/layout.json) by StaticCallEVM (modules/evm/keeper/evm.go) and refuses with "upgrade v6.12.0: launchpad: usdl balance differs from golden" when a balance differs from modules/launchpad/keeper/testdata/v6120/usdl.json.
- For each pool and FeeAccumulator address with a nonzero balance the step calls CallEVM (modules/evm/keeper/evm.go) with from the holding contract and data USDL.transfer(launchpad module EVM address, balance), before any surface is installed, and credits HeldQuote(pool) or FeeHeld; the contracts have no withdraw function, which is why the transfer runs from their address.
- For each Treasury the step calls withdraw(token, 0x1017, getBalance(token)) and then recoverSurplus(token, 0x1017, surplus) (contracts/launchpad/contracts/protocol/Treasury.sol, contracts/launchpad/contracts/protocol/Treasury.sol) through CallEVM from the DEFAULT_ADMIN_ROLE holder named in the fixture, after a hasRole(0x00, holder) staticcall confirms the holding, for every token the fixture lists; the withdrawn ledger amounts are credited to TreasuryLedger of the generation 2 Treasury and the full amounts to TreasuryHeld, so nothing stays behind a retired address.
- USDL allowances and permit nonces are untouched because USDL storage is untouched; users who traded through the Router grant a new allowance to 0x1017, the spender of the pulls, and the migration record of spec/release-v6-12-0/contracts/launchpad.md lists this.
- Step 1 runs first inside MigrateV6120, needs no Minter permission and no MintCoins, and its result is checked: USDL.balanceOf(launchpad module EVM address) after the step equals its value before plus the sum moved, and the custody invariant holds.

### Step 2, pools and pool tokens

- migratePools lists the pools of each generation from the PoolRegistry _allPools array (slot 6, contracts/launchpad/contracts/data/PoolRegistry.sol) of 0x7684…BA0b and 0x1dC3…4Ba7 and refuses when a pool code hash is not the code hash of a generation pool proxy recorded in modules/launchpad/keeper/testdata/v6120/layout.json.
- Each pool state field is read from its slot (tokenAddress 0, usdlAddress 1, opticalAddress 2, feeAccumulator 3, eventEmitter 4, protocolConfig 5, guardian 6, virtualUsdlReserve 7, realUsdlBalance 8, tokenReserve 9, creationTimestamp 10, cumulativeVolume 11, priceSnapshots 12-19, snapshotIndex 20, snapshotCount 21, accumulatedUsdlFees 22, accumulatedTokenFees 23, factory 24, pendingGuardian 25, lastSnapshotTimestamp 26, cumulativeBuyUsdlVolume 27, cumulativeSellUsdlVolume 28) as layout.json records them for the generation implementation, into the keeper Market record.
- Pool tokens stay the deployed SidioraERC20 contracts with code and storage unchanged; step 2 reads token.balanceOf(pool) and totalSupply by StaticCallEVM (modules/evm/keeper/evm.go), checks holders.json against balanceOf with their sum equal to totalSupply, and writes nothing at the token address.
- For each pool the step sets RealQuote to min(realUsdlBalance, the USDL moved from the pool in step 1) and TokenReserve to min(tokenReserve, token.balanceOf(pool)), and records the book-held delta when book and held differ, so InvalidBalance (contracts/launchpad/contracts/core/SidioraPool.sol) holds for every migrated pool.
- Every migrated pool becomes a generation 2 pool: its factory, feeAccumulator, protocolConfig and eventEmitter fields are set to 0x3993…8830 (sdk/launchpad/shared/config/deployment/current.json), 0x7b14…1a66 (sdk/launchpad/shared/config/deployment/current.json), 0xF366…20C6 (sdk/launchpad/shared/config/deployment/current.json) and 0x622E…3EF6 (sdk/launchpad/shared/config/deployment/current.json), its fee right moves to the generation 2 collection, it is authorized at the generation 2 FeeAccumulator and registered at the generation 2 PoolRegistry, and the generation 1 ProtocolConfig values that differ from generation 2 are written to the migration log.
- Pool-token allowances, permit nonces and checkpoints stay in the token contracts, so getPastBalance, getPastTotalSupply, getPastVotes, permit, nonces, DOMAIN_SEPARATOR and PERMIT_TYPEHASH keep answering at the token addresses; Paused (the Pausable slot of contracts/launchpad/contracts/base/Pausable.sol) and guardian state of each pool migrate into the market record.

### Step 3, fee rights, roles and accounting

- Fee rights of the generation 2 collection keep their ids; fee rights of the generation 1 collection 0xDF73…e190 (sdk/launchpad/protocol/addresses.go) are minted again in the generation 2 collection at nextTokenId in generation 1 id order, each with holder from _owners (slot 2), approval from _tokenApprovals (slot 4), operator approvals from _operatorApprovals (slot 5) for the fixture operators, strategy from _feeStrategy (slot 12) and pool from _poolAddress (slot 13), and the collection fields of slots 2-9, 11-13, 15 and 16-17 of contracts/launchpad/contracts/core/SidioraNFT.sol and contracts/launchpad/contracts/base/ERC721Base.sol migrate, among them the per-holder _balances (contracts/launchpad/contracts/base/ERC721Base.sol), the enumeration arrays _allTokens, _allTokensIndex, _ownedTokens and _ownedTokensIndex (contracts/launchpad/contracts/base/ERC721Enumerable.sol) rebuilt over the merged ids, poolTokenId per pool and poolIndexMigrationCursor (contracts/launchpad/contracts/core/SidioraNFT.sol), and poolIndexReady set true once every pool is indexed (contracts/launchpad/contracts/core/SidioraNFT.sol); the fee-right receiver hook is not called on import.
- Role memberships other than DEFAULT_ADMIN_ROLE migrate from each role _roles mapping (slot 0, or slot 10 in the collection) for the accounts listed in the fixture and verified by storage read, generation 1 holders merged into the generation 2 role; DEFAULT_ADMIN_ROLE is not carried, so registeredDefaultAdminCount() returns 0 at every role address and administration runs through requirement 2.40.
- FeeAccumulator accounting of both generations merges into the generation 2 FeeAccumulator: _accumulatedFees, _protocolFeesPending, _airdropBalance, _airdropEpoch, _airdropEpochAmount, _airdropClaimed (for the fixture claimants), _lpRewardsBalance, _opticalSurplus, totalLiabilities, the airdrop snapshot fields (slots 15 to 18), accountingInitialized and the legacy airdrop fields (slots 22 to 24).
- Each pool keeps its optical address; optical contracts stay deployed and unchanged and are called by the precompile with the callers they accept (contracts/launchpad/contracts/opticals/BaseOptical.sol), the PoolRegistry metadata (slot 4) of every pool migrates with nftId (the new id for a generation 1 fee right), createdAt and createdBlock, and OPTICAL_GRANTER_ROLE of LaunchpadOpticalFactory (contracts/launchpad/contracts/opticals/LaunchpadOpticalFactory.sol) migrates to the generation 2 FeeAccumulator role table.
- MigrateV6120 returns its first error wrapped as "upgrade v6.12.0: launchpad: <error>" by the caller in the upgrade handler; it writes nothing outside the launchpad and EVM stores, USDL balances change only through the nested ERC20 calls of step 1, and the whole migration is one cached context committed only on success.
- The map from each generation 1 fee-right id to its generation 2 id is written to the migration log and stored in the keeper, served by a keeper query, and exported and imported by genesis.

### Book-held deltas

- For every migrated pool MigrateV6120 compares the book (realUsdlBalance, tokenReserve) with the held amounts (USDL moved from the pool in step 1, token.balanceOf(pool)), uses the smaller value of each pair, and stores the book-held delta (pool, asset, book, held, delta) in a keeper record, writes it to the migration log and serves it by a keeper query; nothing is minted or topped up.
- modules/launchpad/keeper/testdata/v6120/pools.json lists the known deltas at the survey height: USDL 0x90665CF0… minus 100, 0x67e1b3d8… minus 50 and 0x26Fd86Ef… minus 1,508.850948, and pool tokens 0x67e1b3d8… minus 130,368,747,608 and 0x7bF2f964… minus 5e12 (base units), and the step refuses when a pool not listed at the halt height shows a delta larger than the fixture tolerance.
- The golden fixture includes generation 1 pool 0xEeA16aC55AaE47c484EBef02eb1186aD606e1BF9 and generation 2 pool 0xCE20Da50F92D21d8BeE497910c782E9Aa25b6263 with their reads at H, and the golden pool count is the 45 generation 1 and 4 generation 2 pools of the registries.

### Guards, pause state and legacy replay

- MigrateV6120 refuses with "upgrade v6.12.0: launchpad: native markets exist" when GetMarketCount (modules/launchpad/keeper/keeper.go) is above 0 at the halt height, so no native market created before the upgrade collides with the imported indexes.
- The pause flag of every pool (the Pausable slot at contracts/launchpad/contracts/base/Pausable.sol) and the guardian and pending guardian migrate into the market record, so paused() and emergencyPaused() return the halt-height values and a paused pool refuses swaps after the upgrade.
- The v6.10 keeper and types remain under modules/launchpad/legacy/v610/keeper and modules/launchpad/legacy/v610/types over the same store key. The legacy precompile and keeper accessor replay heights below activation.

### Not migrated

| Item | After the upgrade |
|---|---|
| DEFAULT_ADMIN_ROLE memberships | administration runs through MsgUpdateParams and MsgTreasuryWithdraw (section 23); admin methods revert AccessControlUnauthorizedAccount(address,bytes32) |
| the factory link of migrateFactory | migrateFactory reverts Unauthorized() for every caller |
| the storage of the old contracts behind the surfaces | left in place and no longer read after the migration |
| USDL allowances granted to the old Router contracts | kept on USDL but not spent: pulls run with spender 0x1017 |

### What holders do

- Grant a new allowance to 0x1017 on USDL, or on the pool token for sells, once before the first buy, sell or market creation after the upgrade; the *WithPermit variants take a permit with spender 0x1017.
- Fee rights of generation 1 carry new ids in the generation 2 collection; the id map is served by a keeper query and listed in the migration log.
- Calls to a retired generation 1 address revert Retired(address) carrying the generation 2 address to call instead (section 21).

## 9. Fees and airdrops

- recordFee splits the buy fee into protocol cut protocolFeeBps/10000 and pool cut, applies the optical beforeFeeDistribution adjustment with InsufficientSurplus when it raises the pool cut (contracts/launchpad/contracts/data/FeeAccumulator.sol), moves the protocol cut from FeeHeld to TreasuryLedger and TreasuryHeld of the Treasury with Deposited(USDL, 0x7b14…1a66, cut) emitted at the Treasury address (contracts/launchpad/contracts/data/FeeAccumulator.sol), calls afterFeeDistribution and requires its selector (contracts/launchpad/contracts/data/FeeAccumulator.sol) and emits FeeRecorded (contracts/launchpad/contracts/data/FeeAccumulator.sol) at the generation 2 FeeAccumulator address 0x7b14…1a66 (sdk/launchpad/shared/config/deployment/current.json).
- claim pays the accumulated pool fees to the recipient by USDL.transfer from the launchpad module EVM address and reverts NoFeesAccumulated() at zero (contracts/launchpad/contracts/data/FeeAccumulator.sol); burn transfers them to `0x000000000000000000000000000000000000dEaD` (contracts/launchpad/contracts/data/FeeAccumulator.sol); sendLpRewards moves them from FeeHeld to HeldQuote of the pool reserve (contracts/launchpad/contracts/data/FeeAccumulator.sol).
- The FeesRouter role checks the caller holds the fee right of the pool in the generation 2 collection (ownerOf) and the strategy of that fee right, and emits FeeDistributed at 0x1017 (contracts/launchpad/contracts/periphery/FeesRouter.sol); the keeper requireFeeRightsHolder (modules/launchpad/keeper/fees.go) reads the fee-right store keyed by collection and id.
- The FeeAccumulator keeps one totalLiabilities and requires FeeHeld to cover it plus the new amount (contracts/launchpad/contracts/data/FeeAccumulator.sol); the mirror FeeFlow event is emitted at 0x1017 (contracts/launchpad/contracts/data/FeeAccumulator.sol).
- triggerAirdrop opens epoch e+1 for the pool with the airdrop balance, the eligible supply and the snapshot block block.number-1 as contracts/launchpad/contracts/data/FeeAccumulator.sol does, reading getPastTotalSupply and getPastBalance of the pool and dead addresses from the pool-token contract by nested staticcall; the keeper stores the epoch fields of FeeAccumulator slots 15 to 18 and no entitlement table.
- claimAirdrop pays holder balance at the snapshot times the epoch amount over the eligible supply by USDL.transfer from the launchpad module EVM address, refuses the pool, the `0x000000000000000000000000000000000000dEaD` address and zero (contracts/launchpad/contracts/data/FeeAccumulator.sol), refuses a second claim with AlreadyClaimed() (contracts/launchpad/contracts/data/FeeAccumulator.sol) and gives the rounding residue to the final claimant (contracts/launchpad/contracts/data/FeeAccumulator.sol); the holder balance at the snapshot is getPastBalance(holder, snapshotBlock) read from the pool-token contract by nested staticcall.
- Open airdrop epochs of migrated pools keep their snapshot: MigrateV6120 carries airdropSnapshotBlock and the epoch fields (FeeAccumulator slots 15 to 18), and a claim after the upgrade reads the same token checkpoints the contract read, so it pays what the contract would have paid.
- claimLegacyAirdrop verifies the Merkle proof over keccak256(pool, epoch, holder, amount) (contracts/launchpad/contracts/data/FeeAccumulator.sol) against legacyAirdropRoot (slot 23) and legacyAirdropRemaining (slot 24), both migrated.

## 10. Parameters

- Default parameters match ProtocolConfig.initialize (contracts/launchpad/contracts/protocol/ProtocolConfig.sol): ConcentrationWeight is 0, CreationFee is 1000000 and QuoteToken names USDL (modules/launchpad/types/params.go).
- Validate enforces the ProtocolConfig setter bounds (contracts/launchpad/contracts/protocol/ProtocolConfig.sol, contracts/launchpad/contracts/protocol/ProtocolConfig.sol, contracts/launchpad/contracts/protocol/ProtocolConfig.sol).
- Parameters, role grants and revokes change through MsgUpdateParams only (modules/launchpad/keeper/msg_server.go); Treasury payouts run through MsgTreasuryWithdraw.

| Parameter | ProtocolConfig.initialize | DefaultParams | v6.12.0 |
|---|---|---|---|
| VirtualQuoteDefault | contracts/launchpad/contracts/protocol/ProtocolConfig.sol | modules/launchpad/types/params.go | 10000000000 |
| VirtualTokenDefault | contracts/launchpad/contracts/protocol/ProtocolConfig.sol | modules/launchpad/types/params.go | 1000000000000000 |
| MinFeeBps | contracts/launchpad/contracts/protocol/ProtocolConfig.sol | modules/launchpad/types/params.go | 10 |
| MaxFeeBps | contracts/launchpad/contracts/protocol/ProtocolConfig.sol | modules/launchpad/types/params.go | 300 |
| BaseFeeBps | contracts/launchpad/contracts/protocol/ProtocolConfig.sol | modules/launchpad/types/params.go | 30 |
| ProtocolFeeBps | contracts/launchpad/contracts/protocol/ProtocolConfig.sol | modules/launchpad/types/params.go | 1000 |
| FeeDecayRate | contracts/launchpad/contracts/protocol/ProtocolConfig.sol | modules/launchpad/types/params.go | 500 |
| VolatilityWeight | contracts/launchpad/contracts/protocol/ProtocolConfig.sol | modules/launchpad/types/params.go | 100 |
| ConcentrationWeight | contracts/launchpad/contracts/protocol/ProtocolConfig.sol | modules/launchpad/types/params.go | 0 |
| CreationFee | contracts/launchpad/contracts/protocol/ProtocolConfig.sol | modules/launchpad/types/params.go | 1000000 |

- DefaultParams (modules/launchpad/types/params.go) sets ConcentrationWeight 0 and CreationFee 1000000, the ProtocolConfig values (contracts/launchpad/contracts/protocol/ProtocolConfig.sol).
- MigrateV6120 writes one parameter set read from the generation 2 ProtocolConfig storage (slots 2 to 11) and refuses when it fails Validate (modules/launchpad/types/params.go); the generation 1 ProtocolConfig values that differ are written to the migration log and not applied.
- Validate enforces the ProtocolConfig bounds: concentrationWeight must be 0 (contracts/launchpad/contracts/protocol/ProtocolConfig.sol), decay and volatility weights at most 10000 (contracts/launchpad/contracts/protocol/ProtocolConfig.sol), virtual defaults fit uint112 (contracts/launchpad/contracts/protocol/ProtocolConfig.sol), protocolFeeBps at most 5000 (modules/launchpad/types/params.go) and baseFeeBps within min and max (contracts/launchpad/contracts/protocol/ProtocolConfig.sol).
- Parameters change only through MsgUpdateParams from the governance authority (modules/launchpad/keeper/msg_server.go), which emits ConfigUpdated at 0x1017 for each changed key.

## 11. Events

- Contract-native events are emitted at the generation 2 address of the role, or at the pool address, through EmitEVMLog (precompiles/common/evm_events.go); ERC20 logs of USDL and pool tokens come from those contracts.
- The twelve EventEmitter launchpad events are emitted at 0x1017 with block timestamp and number appended (contracts/launchpad/contracts/data/EventEmitter.sol); the EventEmitter contracts stay deployed for the other systems.
- Logs of one call appear in the contract order of the call, nested contract logs included.
- SidioraPool events are emitted at the pool address; Initialized only at pools created after the upgrade.
- SidioraFactory events at 0x3993…8830; Router events at the address the caller called (0x78FB…C6e4 or 0x1017); FeesRouter events at the address the caller called (0x0568…7927 or 0x1017); FeeAccumulator events at 0x7b14…1a66; PoolRegistry events at 0x1dC3…4Ba7; ProtocolConfig events at 0x1017; SidioraNFT events at 0xe8Dc…409B; Treasury events at the generation 2 Treasury address.
- RoleGranted and RoleRevoked are emitted at the surface of the role only from MsgUpdateParams administration and renounceRole; RoleAdminChanged, Upgraded and MirrorFailed are not emitted after the upgrade.
- ERC20 Approval and Transfer logs of USDL and pool tokens come from those token contracts.
- EventEmitter event `Swap(bytes32,address,bool,uint256,uint256,uint256,uint256,uint256,uint256)` (topic0 0xa234ab85728295548823d8a26077ecc77aa5815f9ed8f4c5e6fd390ac851d4cc, declared at contracts/launchpad/contracts/interfaces/IEventEmitter.sol) is emitted at 0x1017 by the precompile wherever the contract flow called emitSwap (contracts/launchpad/contracts/core/SidioraPool.sol), with the arguments emitSwap adds (block.timestamp and block.number, contracts/launchpad/contracts/data/EventEmitter.sol); the EventEmitter contracts stay deployed and unchanged for the other systems that use them.
- EventEmitter event `PoolStateUpdated(bytes32,uint256,uint256,uint256,uint256,uint256,uint256)` (topic0 0x8155a4b38a877d212b271811786986f8f7d78074c82fce4526c58defcc6d7ebd, declared at contracts/launchpad/contracts/interfaces/IEventEmitter.sol) is emitted at 0x1017 by the precompile wherever the contract flow called emitPoolStateUpdated (contracts/launchpad/contracts/core/SidioraPool.sol), with the arguments emitPoolStateUpdated adds (block.timestamp and block.number, contracts/launchpad/contracts/data/EventEmitter.sol); the EventEmitter contracts stay deployed and unchanged for the other systems that use them.
- EventEmitter event `TokenDeployed(address,address,address,bytes32,string,string,uint8,uint256,uint256,uint256)` (topic0 0xad187874b793634c55d977f35d4828526eb48170a9531c71d88f8fd10cf9c237, declared at contracts/launchpad/contracts/interfaces/IEventEmitter.sol) is emitted at 0x1017 by the precompile wherever the contract flow called emitTokenDeployed (contracts/launchpad/contracts/core/SidioraFactory.sol), with the arguments emitTokenDeployed adds (block.timestamp and block.number, contracts/launchpad/contracts/data/EventEmitter.sol); the EventEmitter contracts stay deployed and unchanged for the other systems that use them.
- EventEmitter event `MarketCreated(bytes32,address,address,address,address,uint256,uint256)` (topic0 0xb0a9c7605389f1127c2299d4a3ed85f0b2fc19894ab5411d76b453bb29f9f47b, declared at contracts/launchpad/contracts/interfaces/IEventEmitter.sol) is emitted at 0x1017 by the precompile wherever the contract flow called emitMarketCreated (contracts/launchpad/contracts/core/SidioraFactory.sol), with the arguments emitMarketCreated adds (block.timestamp and block.number, contracts/launchpad/contracts/data/EventEmitter.sol); the EventEmitter contracts stay deployed and unchanged for the other systems that use them.
- EventEmitter event `NftMint(uint256,address,address,uint8,uint256,uint256)` (topic0 0xd2917d37031c0dfe5016665c7a83360cbf2ff32063096b2caf61ba66c06832e5, declared at contracts/launchpad/contracts/interfaces/IEventEmitter.sol) is emitted at 0x1017 by the precompile wherever the contract flow called emitNftMint (contracts/launchpad/contracts/core/SidioraNFT.sol), with the arguments emitNftMint adds (block.timestamp and block.number, contracts/launchpad/contracts/data/EventEmitter.sol); the EventEmitter contracts stay deployed and unchanged for the other systems that use them.
- EventEmitter event `FeeStrategyChanged(bytes32,uint256,uint8,uint8,uint256,uint256)` (topic0 0x337fd9f9304e9da4c04886a5b8bf56e74032b459758212a860bc61d3544353dd, declared at contracts/launchpad/contracts/interfaces/IEventEmitter.sol) is emitted at 0x1017 by the precompile wherever the contract flow called emitFeeStrategyChanged (contracts/launchpad/contracts/core/SidioraNFT.sol), with the arguments emitFeeStrategyChanged adds (block.timestamp and block.number, contracts/launchpad/contracts/data/EventEmitter.sol); the EventEmitter contracts stay deployed and unchanged for the other systems that use them.
- EventEmitter event `NftTransfer(address,address,address,uint256,uint256,uint256)` (topic0 0x021f03e8a1315704c74c03e8c022c12bc80c4a2bd4fec8bd36b217322c340ad3, declared at contracts/launchpad/contracts/interfaces/IEventEmitter.sol) is emitted at 0x1017 by the precompile wherever the contract flow called emitNftTransfer (contracts/launchpad/contracts/core/SidioraNFT.sol), with the arguments emitNftTransfer adds (block.timestamp and block.number, contracts/launchpad/contracts/data/EventEmitter.sol); the EventEmitter contracts stay deployed and unchanged for the other systems that use them.
- EventEmitter event `PoolRegistered(address,address,address,address,uint256,uint256,uint256)` (topic0 0xda5568642b67660133b1de538c867c58f3037a5722b0e231a959ee8ca11b2ed7, declared at contracts/launchpad/contracts/interfaces/IEventEmitter.sol) is emitted at 0x1017 by the precompile wherever the contract flow called emitPoolRegistered (contracts/launchpad/contracts/data/PoolRegistry.sol), with the arguments emitPoolRegistered adds (block.timestamp and block.number, contracts/launchpad/contracts/data/EventEmitter.sol); the EventEmitter contracts stay deployed and unchanged for the other systems that use them.
- EventEmitter event `RouterTrade(uint8,address,address,address,address,uint256,uint256,uint256,uint256,uint256)` (topic0 0x17c1899a3fd431f2b2200b90e738d270bb856a4222040c5650bfa208acb3b650, declared at contracts/launchpad/contracts/interfaces/IEventEmitter.sol) is emitted at 0x1017 by the precompile wherever the contract flow called emitRouterTrade (contracts/launchpad/contracts/periphery/Router.sol), with the arguments emitRouterTrade adds (block.timestamp and block.number, contracts/launchpad/contracts/data/EventEmitter.sol); the EventEmitter contracts stay deployed and unchanged for the other systems that use them.
- EventEmitter event `ConfigUpdated(bytes32,uint256,uint256,uint256,uint256)` (topic0 0x2903b3642d2105b7fbd127326de888ab50e8d975bf819398337fb58407c1b10e, declared at contracts/launchpad/contracts/interfaces/IEventEmitter.sol) is emitted at 0x1017 by the precompile wherever the contract flow called emitConfigUpdated (contracts/launchpad/contracts/protocol/ProtocolConfig.sol), with the arguments emitConfigUpdated adds (block.timestamp and block.number, contracts/launchpad/contracts/data/EventEmitter.sol); the EventEmitter contracts stay deployed and unchanged for the other systems that use them.
- EventEmitter event `FeeFlow(uint8,address,address,uint256,uint256,uint256,uint256,uint256,uint256)` (topic0 0x626e3fd5d9323ca4b7ffa3ca51c21c02766b638d15eb8896a36a4e0e97ae6de4, declared at contracts/launchpad/contracts/interfaces/IEventEmitter.sol) is emitted at 0x1017 by the precompile wherever the contract flow called emitFeeFlow (contracts/launchpad/contracts/data/FeeAccumulator.sol), with the arguments emitFeeFlow adds (block.timestamp and block.number, contracts/launchpad/contracts/data/EventEmitter.sol); the EventEmitter contracts stay deployed and unchanged for the other systems that use them.
- EventEmitter event `FeeDistributed(bytes32,uint256,uint8,uint256,address,uint256,uint256)` (topic0 0xf7ab826cedf9d005102eddf95e65f43597aef36a55ff09ee4f452204c83f4afc, declared at contracts/launchpad/contracts/interfaces/IEventEmitter.sol) is emitted at 0x1017 by the precompile wherever the contract flow called emitFeeDistributed (contracts/launchpad/contracts/periphery/FeesRouter.sol), with the arguments emitFeeDistributed adds (block.timestamp and block.number, contracts/launchpad/contracts/data/EventEmitter.sol); the EventEmitter contracts stay deployed and unchanged for the other systems that use them.

## 12. Errors

- Every refusal returns the contract custom error selector and ABI arguments as revert data; keeper errors keep their LXP_LAUNCHPAD_ codes (modules/launchpad/types/errors.go).
- DEFAULT_ADMIN_ROLE methods (grantRole, revokeRole, upgradeToAndCall and every admin setter) revert `AccessControlUnauthorizedAccount(address,bytes32)` (0xe2517d3f) carrying the caller and 0x00; non-admin role checks revert MissingRole(caller, role) (0x0161a64a, contracts/launchpad/contracts/base/AccessControl.sol); initialize reverts AlreadyInitialized() (0x0dc149f0).
- Guardian functions revert Unauthorized() (0x82b42900, contracts/launchpad/contracts/core/SidioraPool.sol) for other callers, and migrateFactory reverts it for every caller (contracts/launchpad/contracts/core/SidioraPool.sol).
- A pool-token or USDL error raised inside a TransferHelper call of a surface is wrapped as TransferFromFailed() (0x7939f424) or TransferFailed() (0x90b8ec18) (contracts/launchpad/contracts/libraries/TransferHelper.sol); a direct interface call returns it unchanged.

Chain-defined errors (no Launch-x declaration):

| Error | Selector | Raised |
|---|---|---|
| `AccessControlUnauthorizedAccount(address,bytes32)` | `0xe2517d3f` | every DEFAULT_ADMIN_ROLE method at a surface, with the caller and 0x00 |
| `Retired(address)` | `0xda9d2e31` | every call to a retired generation 1 address, with the generation 2 address of the same role |

Errors added by the custody and Treasury model:

| Error | Selector | Declared | Raised |
|---|---|---|---|
| `ReentrancyGuardReentrantCall()` | `0x3ee5aeb5` | contracts/launchpad/contracts/base/ReentrancyGuard.sol | a second entry while a pool or role lock is held (section 19) |
| `ReentrantCall()` | `0x37ed32e8` | contracts/launchpad/contracts/protocol/Treasury.sol | a second entry while the Treasury lock is held |
| `InvalidToken()` | `0xc1ab6dc1` | contracts/launchpad/contracts/protocol/Treasury.sol | a token without code, or whose balanceOf staticcall fails, at the Treasury |
| `NonERC721Receiver()` | `0x80526d0c` | contracts/launchpad/contracts/base/ERC721Base.sol | a fee-right receiver that does not return 0x150b7a02 |

## 13. Optical hooks

- The precompile calls `getFlags()` (selector 0x6cc7149d, implemented at contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol) on the market optical through evm.Call with caller the pool address, at the point of the contract flow contracts/launchpad/contracts/core/SidioraPool.sol, only when the optical flags carry every flag (contracts/launchpad/contracts/libraries/BitFlag.sol); the optical code stays deployed unchanged and its sender checks pass (contracts/launchpad/contracts/opticals/BaseOptical.sol).
- The precompile calls `beforeSwap(address,address,bool,uint256)` (selector 0xae7e71d9, implemented at contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol) on the market optical through evm.Call with caller the pool address, at the point of the contract flow contracts/launchpad/contracts/core/SidioraPool.sol, only when the optical flags carry BEFORE_SWAP (1) (contracts/launchpad/contracts/libraries/BitFlag.sol); the optical code stays deployed unchanged and its sender checks pass (contracts/launchpad/contracts/opticals/BaseOptical.sol).
- The precompile calls `afterSwap(address,address,bool,uint256,uint256)` (selector 0xf74c1c1a, implemented at contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol) on the market optical through evm.Call with caller the pool address, at the point of the contract flow contracts/launchpad/contracts/core/SidioraPool.sol, only when the optical flags carry AFTER_SWAP (2) (contracts/launchpad/contracts/libraries/BitFlag.sol); the optical code stays deployed unchanged and its sender checks pass (contracts/launchpad/contracts/opticals/BaseOptical.sol).
- The precompile calls `beforeFeeDistribution(address,uint256)` (selector 0xf80d2cc7, implemented at contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol) on the market optical through evm.Call with caller the generation 2 FeeAccumulator address 0x7b14…1a66 (sdk/launchpad/shared/config/deployment/current.json), which serves the markets of every generation, at the point of the contract flow contracts/launchpad/contracts/data/FeeAccumulator.sol, only when the optical flags carry BEFORE_FEE_DISTRIBUTION (4) (contracts/launchpad/contracts/libraries/BitFlag.sol); the optical code stays deployed unchanged and its sender checks pass (contracts/launchpad/contracts/opticals/BaseOptical.sol).
- The precompile calls `afterFeeDistribution(address,uint256)` (selector 0xae3bd4bd, implemented at contracts/launchpad/contracts/opticals/BaseOptical.sol) on the market optical through evm.Call with caller the generation 2 FeeAccumulator address 0x7b14…1a66 (sdk/launchpad/shared/config/deployment/current.json), at the point of the contract flow contracts/launchpad/contracts/data/FeeAccumulator.sol, only when the optical flags carry AFTER_FEE_DISTRIBUTION (8) (contracts/launchpad/contracts/libraries/BitFlag.sol); the optical code stays deployed unchanged and its sender checks pass (contracts/launchpad/contracts/opticals/BaseOptical.sol).
- `beforeTokenTransfer(address,address,address,uint256)` (selector 0x37be75a0, implemented at contracts/launchpad/contracts/opticals/presets/LaunchpadOptical.sol) is called by the pool-token contract itself at contracts/launchpad/contracts/core/SidioraERC20.sol, unchanged, when the optical flags carry BEFORE_TOKEN_TRANSFER (16); the token reads opticalAddress() from the pool surface, which the precompile answers from the keeper market record, so every pool-token transfer, the nested ones the precompile makes included, runs the hook with the token contract as caller and the optical sender checks pass (contracts/launchpad/contracts/opticals/BaseOptical.sol).

## 14. Storage

- Market records extend modules/launchpad/types/market.go under MarketPrefix (modules/launchpad/types/keys.go).
- Surface, role, custody, Treasury, book-held delta, fee-right id map and retired records are new prefixes beside modules/launchpad/types/keys.go.
- Holding history stays under HoldingHistoryPrefix (modules/launchpad/types/keys.go) for the reads of tasks 3.2.10 and 12.3.29; airdrops read past balances from the pool-token checkpoints.
- Pool storage slots read by the migration are those of spec/release-v6-12-0/contracts/launchpad.md section 14, from the Launch-x build of each generation implementation; old storage is left in place.

Storage slots of the generation 2 implementations read by the migration (slot, field):

- SidioraERC20: 0 `name`, 1 `symbol`, 2 `totalSupply`, 3 `balanceOf`, 4 `allowance`, 5 `nonces`, 6 `_balanceCheckpoints`, 7 `_supplyCheckpoints`, 8 `pool`.

- SidioraFactory: 0 `_roles`, 1 `poolBeacon`, 2 `nftContract`, 3 `poolRegistry`, 4 `eventEmitter`, 5 `protocolConfig`, 6 `treasury`, 7 `feeAccumulator`, 8 `usdlAddress`, 9 `_nonces`.

- SidioraNFT: 0 `name`, 1 `symbol`, 2 `_owners`, 3 `_balances`, 4 `_tokenApprovals`, 5 `_operatorApprovals`, 6 `_allTokens`, 7 `_allTokensIndex`, 8 `_ownedTokens`, 9 `_ownedTokensIndex`, 10 `_roles`, 11 `nextTokenId`, 12 `_feeStrategy`, 13 `_poolAddress`, 14 `eventEmitter`, 15 `poolTokenId`, 16 `poolIndexMigrationCursor`, 17 `poolIndexReady`.

- SidioraPool: 0 `tokenAddress`, 1 `usdlAddress`, 2 `opticalAddress`, 3 `feeAccumulator`, 4 `eventEmitter`, 5 `protocolConfig`, 6 `guardian`, 7 `virtualUsdlReserve`, 8 `realUsdlBalance`, 9 `tokenReserve`, 10 `creationTimestamp`, 11 `cumulativeVolume`, 12 `priceSnapshots`, 20 `snapshotIndex`, 21 `snapshotCount`, 22 `accumulatedUsdlFees`, 23 `accumulatedTokenFees`, 24 `factory`, 25 `pendingGuardian`, 26 `lastSnapshotTimestamp`, 27 `cumulativeBuyUsdlVolume`, 28 `cumulativeSellUsdlVolume`.

- FeeAccumulator: 0 `_roles`, 1 `protocolConfig`, 2 `treasury`, 3 `poolRegistry`, 4 `eventEmitter`, 5 `usdlAddress`, 6 `_accumulatedFees`, 7 `_protocolFeesPending`, 8 `_airdropBalance`, 9 `_airdropEpoch`, 10 `_airdropEpochAmount`, 11 `_airdropClaimed`, 12 `_lpRewardsBalance`, 13 `_opticalSurplus`, 14 `totalLiabilities`, 15 `airdropSnapshotBlock`, 16 `airdropEligibleSupply`, 17 `airdropEpochRemaining`, 18 `airdropClaimedSupply`, 19 `accountingInitialized`, 20 `accountingMigrationCursor`, 21 `_accountingMigratedPool`, 22 `legacyAirdropUnallocated`, 23 `legacyAirdropRoot`, 24 `legacyAirdropRemaining`.

- PoolRegistry: 0 `_roles`, 1 `_tokenToPool`, 2 `_poolToNftId`, 3 `_creatorToPools`, 4 `_poolMetadata`, 5 `_registeredPools`, 6 `_allPools`, 7 `eventEmitter`.

- FeesRouter: 0 `_roles`, 1 `nftContract`, 2 `feeAccumulator`, 3 `poolRegistry`.

- Quoter: 0 `_roles`, 1 `poolRegistry`, 2 `protocolConfig`.

- Router: 0 `_roles`, 1 `factory`, 2 `poolRegistry`, 3 `protocolConfig`, 4 `usdlAddress`.

- ProtocolConfig: 0 `_roles`, 1 `usdlAddress`, 2 `virtualUsdlDefault`, 3 `virtualTokenDefault`, 4 `minFeeBps`, 5 `maxFeeBps`, 6 `baseFeeBps`, 7 `protocolFeeBps`, 8 `feeDecayRate`, 9 `volatilityWeight`, 10 `concentrationWeight`, 11 `creationFee`, 12 `eventEmitter`.

- Treasury: 0 `_roles`, 1 `_balances`, 2 `eventEmitter` (contracts/launchpad/storage-layout/core/Treasury.json).

The generation 1 layout is a release input in modules/launchpad/keeper/testdata/v6120/layout.json; the migration refuses a pool whose code hash is not listed there.

## 15. Gas

- RequiredGas is Gas(inputBytes, writes), with BaseGas 3000, GasPerByte 16 and GasPerWrite 5000 (precompiles/launchpad/gas.go). Writes assigns each method its write surcharge; nested calls consume the remaining gas through RunAndCalculateGas (precompiles/launchpad/run.go).
- Nested evm.Call and CREATE2 gas for token deployment, USDL and pool-token transfers, optical hooks and the fee-right receiver hook is metered by the EVM, as the xweb precompile meters its callback (precompiles/xweb/xweb.go).
- The legacy gas checks remain in precompiles/launchpad/legacy/v610/launchpad_test.go.

## 16. Activation

- 0x1017 is guarded by activationPrecompileGate (node/upgrades.go).
- GetVersioned (precompiles/launchpad/setup.go) maps latestUpgrade and v6.12.0 to the current precompile, and v6.10 to precompiles/launchpad/legacy/v610.
- The v6.12.0 handler runs RunMigrations, moveSidioraMintAuthority and migrateSIDHolders, followed by createV6120Pointers, launchpad MigrateV6120, pricefeed InitV6120, perps SeedV6120 and spot InitV6120 (node/upgrades_v6120.go). The launchpad migration uses CallEVM and wraps its error with the upgrade and hook names.
- The handler switches the versioned precompile set live; a restarted node restores activation from state. The legacy keeper and precompile serve heights below activation.
- TestLaunchpadDomainWiring (modules/launchpad/launchpadtest/domain_test.go) covers creation, buying, selling, fee claims and a migrated pool read in one store.

## 17. Invariants

- HeldQuote(pool) covers realUsdlBalance and token.balanceOf(pool) covers tokenReserve (contracts/launchpad/contracts/core/SidioraPool.sol, modules/launchpad/keeper/invariants.go).
- FeeHeld covers totalLiabilities (contracts/launchpad/contracts/data/FeeAccumulator.sol) and TreasuryHeld covers the Treasury ledger.
- USDL.balanceOf(launchpad module EVM address) covers the sum of HeldQuote, FeeHeld and TreasuryHeld(USDL); USDL totalSupply is unchanged by the upgrade.
- SolvencyInvariant (modules/launchpad/keeper/invariants.go) checks, for every market, that HeldQuote(pool) is at least realUsdlBalance and token.balanceOf(pool) is at least tokenReserve, the condition of contracts/launchpad/contracts/core/SidioraPool.sol.
- LiabilitiesInvariant checks that FeeHeld is at least totalLiabilities (contracts/launchpad/contracts/data/FeeAccumulator.sol) and that TreasuryHeld is at least TreasuryLedger for every token, and is registered in RegisterInvariants (modules/launchpad/keeper/invariants.go).
- CustodyInvariant checks that USDL.balanceOf(launchpad module EVM address), read by StaticCallEVM, is at least the sum of every HeldQuote, FeeHeld and TreasuryHeld(USDL), and that the custody ledgers change only by sweeps, pulls, payouts and the ledger moves

## 18. Dependents and backend binding

- The hosted indexer registry entry ("launchpad", 0x1017) (services/platform/hosted/indexer/src/abi.rs) loads precompiles/launchpad/abi.json, and signatures_and_topics_match_solidity (services/platform/hosted/indexer/src/abi.rs) covers the twelve launchpad events.
- The gateway launchpad probe sends getPoolCount(), selector 0x8eec5d70, to LAUNCHPAD_PRECOMPILE and decodes one word (services/platform/hosted/gateway/src/capabilities.rs, services/platform/hosted/gateway/src/evm.rs).
- The indexer ingest test replays a Swap and a MarketCreated log at 0x1017 and stores one row each.
- The deployment manifest defines surface roles and LaunchpadPrecompile at 0x0000000000000000000000000000000000001017 with kind "precompile". Surface roles have no implementation or beacon fields. The recorded surfaces carry runtime hashes, and `AcceptsRuntimeHash` accepts their installed hash or an explicitly declared staged hash with an upgrade name (`sdk/launchpad/shared/config/deployment/manifest.go`, `sdk/launchpad/shared/config/deployment/attest.go`).
- `sdk/launchpad/shared/config/deployment/current.json` records nine generation 2 surface roles, the LaunchpadPrecompile, USDL as a direct contract, EventEmitter and OpticalRegistry as ERC1967 contracts, and PoolBeacon as a beacon. It records chain id 125, deploymentVersion `paxeer125-v6.12.0`, activationHeight 37220320, six-decimal USDL settlement and the governance configuration.
- The decode registry (services/launchpad/indexer/internal/decode/registry.go) binds LaunchpadPrecompile at 0x1017 to the EventEmitter ABI and dispatches each log there by topic0 to its event, beside the generation 2 EventEmitter binding that keeps decoding the logs of other systems (services/launchpad/indexer/internal/decode/registry.go).
- parseContractAddresses (sdk/launchpad/shared/chain/contracts.go) accepts kind "surface" and the LaunchpadPrecompile role, and CreateContracts (sdk/launchpad/shared/chain/contracts.go) binds each role at its generation 2 address with its role ABI and binds the EventEmitter ABI at 0x1017 for the launchpad family events.
- sdk/launchpad/protocol/addresses.go keeps the generation 1 addresses (sdk/launchpad/protocol/addresses.go) for historical decoding, marks them retired from the v6.12.0 height, and adds the precompile address 0x1017 and the generation 2 role addresses as the active set.

## 19. Custody and re-entrancy

- swap, swapFor and syncReserves of a pool run under a per-pool lock held in the transient store of the precompile run; a second state-changing entry for the same pool while the lock is held, from an optical hook, a token hook or any nested call, reverts ReentrancyGuardReentrantCall() (0x3ee5aeb5) as the nonReentrant guard of contracts/launchpad/contracts/core/SidioraPool.sol, contracts/launchpad/contracts/core/SidioraPool.sol and contracts/launchpad/contracts/core/SidioraPool.sol does (contracts/launchpad/contracts/base/ReentrancyGuard.sol).
- The FeeAccumulator role holds one lock across recordFee, claim, burn, triggerAirdrop, claimAirdrop, claimAirdropEpoch, sendLpRewards, claimOpticalSurplus, claimLegacyAirdrop and allocateLegacyAirdrop as its nonReentrant functions do (contracts/launchpad/contracts/data/FeeAccumulator.sol), the Treasury role holds the lock of contracts/launchpad/contracts/protocol/Treasury.sol and reverts ReentrantCall() (0x37ed32e8), and the Router and FeesRouter roles hold the locks of their nonReentrant entry points.
- Locks live only for one precompile run and are released on return and on revert; view functions are answered while a lock is held, so the opticalAddress() read of a pool-token transfer and the getReserves() read of an optical hook succeed during a swap.
- Before each nested call (optical hook, USDL or pool-token transfer, permit, fee-right receiver hook) the precompile writes every keeper change of the run so far, so the nested call and any view it makes see the committed outer state, in the order of the contract flow: reserves and snapshot before afterSwap, fee split before afterFeeDistribution.
- A nested call that reverts reverts the whole precompile call with the nested revert data, except the permit of the *WithPermit variants, whose failure the contract ignores (contracts/launchpad/contracts/periphery/Router.sol); the fee-right receiver hook follows the contract rule: a wrong return value or an empty revert gives NonERC721Receiver() (0x80526d0c) and any other revert data is returned unchanged (contracts/launchpad/contracts/core/SidioraNFT.sol, contracts/launchpad/contracts/base/ERC721Base.sol).

## 20. Treasury

- The generation 2 Treasury address read from SidioraFactory slot 6 (contracts/launchpad/contracts/core/SidioraFactory.sol) and recorded in modules/launchpad/keeper/testdata/v6120/layout.json holds the surface runtime and serves the Treasury ABI of contracts/launchpad/contracts/protocol/Treasury.sol (17 functions, 8 events, 19 errors, the eleventh role file) for the treasury of every generation, the generation 1 Treasury 0x15405D535ce533BfFb98c83e42f4DD242AA5e079 (sdk/launchpad/protocol/addresses.go) being retired by requirement 2.36.
- deposit(token, amount) (0x47e7ef24) pulls token.transferFrom(caller, launchpad module EVM address, amount) for a DEPOSITOR_ROLE caller, credits the received amount measured by balanceOf before and after to TreasuryLedger and TreasuryHeld and emits Deposited(token, caller, received) at the Treasury address (contracts/launchpad/contracts/protocol/Treasury.sol, ITreasury.sol:14); recordDeposit(token, amount) (0x1e578bae) sweeps the Treasury address, requires amount at most TreasuryHeld minus TreasuryLedger and credits the ledger (contracts/launchpad/contracts/protocol/Treasury.sol).
- getBalance(token) (0xf8b2cb4f) returns TreasuryLedger(token) (contracts/launchpad/contracts/protocol/Treasury.sol) and rises after every createMarket by the creation fee and after every recordFee by the protocol cut; eventEmitter() (0x9ff78c30) returns 0x622E…3EF6 (sdk/launchpad/shared/config/deployment/current.json); DEPOSITOR_ROLE(), hasRole and getRoleAdmin answer from the migrated role table; a token address without code, or whose balanceOf staticcall fails or returns other than 32 bytes, reverts InvalidToken() (0xc1ab6dc1) as _actualBalance does (contracts/launchpad/contracts/protocol/Treasury.sol).
- withdraw (0xd9caed12, contracts/launchpad/contracts/protocol/Treasury.sol) and recoverSurplus (0x72df7f5b, contracts/launchpad/contracts/protocol/Treasury.sol) revert `AccessControlUnauthorizedAccount(address,bytes32)` (0xe2517d3f) with the caller and 0x00 at the surface and run only through MsgTreasuryWithdraw; initialize (0x485cc955, contracts/launchpad/contracts/protocol/Treasury.sol) reverts AlreadyInitialized() and upgradeToAndCall reverts as in requirement 2.13.
- MigrateV6120 carries the Treasury _balances ledger (contracts/launchpad/contracts/protocol/Treasury.sol) of both generations per token listed in the fixture into TreasuryLedger, and the DEPOSITOR_ROLE members of both generations into the role table of the generation 2 Treasury.

## 21. Retired addresses

| Generation 1 address | Role | Replacement |
|---|---|---|
| `0x8a1A09CEe72c1D39dF33B8284E38baeF8371f465` (sdk/launchpad/protocol/addresses.go) | SidioraFactory | `0x3993D004166738d1069B5C4e0a923f7B9D5D8830` |
| `0xCC7298801112682e10ee14b8a520309caD80336d` (sdk/launchpad/protocol/addresses.go) | Router | `0x78FB06f525cD964388f2305A489035263E31C6e4` |
| `0xB768e183b6EfDeDf8b2AA7af732039D1C3c452d0` (sdk/launchpad/protocol/addresses.go) | Quoter | `0x793006f4F321b8863F26DBD75b11bAa01DA181dc` |
| `0xEeDF5409cFD30bd14D0399318c7d2150265575e5` (sdk/launchpad/protocol/addresses.go) | ProtocolConfig | `0xF3669eDc475Cb6E1F0d7635240ABD8e8FB7E20C6` |
| `0x50C69dF6637b3DCE6a7407C5A4b4F99E68514A76` (sdk/launchpad/protocol/addresses.go) | FeeAccumulator | `0x7b14808b72f61664cDb79D25E6d1587655571a66` |
| `0x7684382c89f79104574D8EF9b31eFf2eD2C2BA0b` (sdk/launchpad/protocol/addresses.go) | PoolRegistry | `0x1dC357d2B7B850332a91F06A9E66634F46fC4Ba7` |
| `0xDF73b354ed9dcB473cc9D01541c46f507591e190` (sdk/launchpad/protocol/addresses.go) | SidioraNFT | `0xe8DcC844a8006dbf64f5Bb851bCad90292a6409B` |
| `0x02Df12a44F2658080E76fbcF7D6B34Baa97843b6` (sdk/launchpad/protocol/addresses.go) | FeesRouter | `0x05681dA6f611C2dfFD3E92611F6dE2d247647927` |
| `0x15405D535ce533BfFb98c83e42f4DD242AA5e079` (sdk/launchpad/protocol/addresses.go) | Treasury | read from SidioraFactory slot 6 (contracts/launchpad/contracts/core/SidioraFactory.sol), recorded in modules/launchpad/keeper/testdata/v6120/layout.json |
| `0x4CdA6e48632d51Ee4Fa735D81BF09F7543f644a1` (sdk/launchpad/protocol/addresses.go) | OpticalRegistry | `0x0FB5E510372Bd3EddFED16A5Dd766d4473CffcF0` |

- MigrateV6120 installs at the generation 1 SidioraFactory, Router, Quoter, ProtocolConfig, FeeAccumulator, PoolRegistry, fee-right collection and FeesRouter addresses (sdk/launchpad/protocol/addresses.go), the generation 1 Treasury 0x15405D535ce533BfFb98c83e42f4DD242AA5e079 (sdk/launchpad/protocol/addresses.go) and the generation 1 OpticalRegistry (sdk/launchpad/protocol/addresses.go) the fixed retirement runtime that reverts Retired(address) (0xda9d2e31) carrying the generation 2 address of the same role, for every selector, after their USDL is moved in step 1.
- Storage at the retired addresses is left as it was at the halt height, the generation 1 EventEmitter (sdk/launchpad/protocol/addresses.go) and PoolBeacon (sdk/launchpad/protocol/addresses.go) keep their code, the Timelock and GovernanceModule (sdk/launchpad/protocol/addresses.go) keep their code, and generation 1 pool addresses are not retired: they serve as generation 2 pools by requirement 2.24.
- The retired list (address, role, replacement) is written to the migration log, served by a keeper query and listed in spec/release-v6-12-0/contracts/launchpad.md section 21, and the release runbook announces it before the halt.

## 22. Retained contracts

- The generation 2 EventEmitter 0x622E…3EF6 (sdk/launchpad/shared/config/deployment/current.json) stays deployed and keeps serving the other systems; its authorization reads (_isAuthorized, _requireTyped and _requirePool at contracts/launchpad/contracts/data/EventEmitter.sol and contracts/launchpad/contracts/data/EventEmitter.sol) go to the PoolRegistry, FeeAccumulator and Factory surfaces, which answer from the keeper, and the launchpad family events are emitted at 0x1017 by requirement 2.16, so MirrorFailed (contracts/launchpad/contracts/data/FeeAccumulator.sol, contracts/launchpad/contracts/data/PoolRegistry.sol, contracts/launchpad/contracts/periphery/FeesRouter.sol) is never emitted after the upgrade.
- The generation 2 OpticalRegistry 0x0FB5…fcF0 (sdk/launchpad/shared/config/deployment/current.json), LaunchpadOpticalFactory and every optical keep their code; createLaunchpadOptical (contracts/launchpad/contracts/opticals/LaunchpadOpticalFactory.sol) calls authorizeOptical (contracts/launchpad/contracts/opticals/LaunchpadOpticalFactory.sol) at the generation 2 FeeAccumulator surface, which accepts it through the migrated OPTICAL_GRANTER_ROLE, and onlyAttachedPool (contracts/launchpad/contracts/opticals/BaseOptical.sol) passes because isRegisteredPool at the generation 2 PoolRegistry surface answers for pools of every generation.
- The generation 2 PoolBeacon 0x0c60…71BE (sdk/launchpad/shared/config/deployment/current.json) keeps its code and is the beacon named in the predicted pool address; BeaconCallFailed and the beacon functions answer as the contract does.
- Operations queued in the Timelock or GovernanceModule that call a DEFAULT_ADMIN_ROLE method at a surface revert `AccessControlUnauthorizedAccount(address,bytes32)` (0xe2517d3f) when executed; EmergencyPauseController.pauseTargets (contracts/launchpad/contracts/protocol/EmergencyPauseController.sol) calling pause() at a pool surface reverts Unauthorized() unless the controller is the pool guardian, exactly as at the halt height, within the 150000 and 30000 gas budgets of the controller.
- The meta-ag SidioraAdapter reads the Router, Quoter and PoolRegistry addresses it holds (contracts/launchpad/contracts/meta-ag/adapters/SidioraAdapter.sol), which are generation 2 surfaces, and approves its Router (contracts/launchpad/contracts/meta-ag/adapters/SidioraAdapter.sol); since pulls run with spender 0x1017, the release runbook calls setSidioraRouter(0x0000000000000000000000000000000000001017) through its admin, and getPoolByToken answers for tokens of every generation.

## 23. Governance administration

- MsgUpdateParams (modules/launchpad/keeper/msg_server.go) is the only path for role administration: its role_grants and role_revokes fields grant and revoke non-admin roles in the migrated role table under the governance authority and emit RoleGranted and RoleRevoked at the surface of the role, and every DEFAULT_ADMIN_ROLE method at a surface reverts `AccessControlUnauthorizedAccount(address,bytes32)` (0xe2517d3f), the single accepted divergence.
- MsgTreasuryWithdraw (authority, token, to, amount, surplus flag), declared in api/launchpad/tx.proto and registered in modules/launchpad/types/codec.go beside MsgUpdateParams (modules/launchpad/types/codec.go, modules/launchpad/types/codec.go), runs only under the governance authority, pays out from TreasuryHeld by token.transfer from the launchpad module EVM address, debits TreasuryLedger for a withdraw and only the surplus for a recovery, and emits Withdrawn (contracts/launchpad/contracts/interfaces/ITreasury.sol) or SurplusRecovered (contracts/launchpad/contracts/protocol/Treasury.sol) at the Treasury address.
