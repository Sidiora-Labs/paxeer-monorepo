<!-- Source: https://docs.paxeer.app/system-map/ -->

# System map

Explore every Paxeer 7.0.0 system by kind, release change, and name, with build commands, configuration, health checks, dependencies, and documentation.

Choose a system to expand its details. Follow a dependency to open that system, including when it is outside the current filters. Build commands run from the repository root unless they include a change of directory.

## node

### Paxeer chain node

Identifier: chain-node
Change in 7.0.0: changed

Directory: daemon/paxd

Build command:

```sh
make build
```

Build output: build/paxd

Configuration: node/params/config.go and docs/release/7.0.0-validator-runbook.md

Ports: 26656, 26657, 8545, 8546, 1317

Health check: paxd status

Depends on: None

Site pages: [/run-node](https://docs.paxeer.app/run-node)

Repository references: docs/release/7.0.0-upgrade-plan.md, docs/release/7.0.0-validator-runbook.md, docs/site/paxeer-docs/src/app/run-node/page.tsx

## kernel

### LayerX kernel host

Identifier: kernel
Change in 7.0.0: changed

Directory: modules/lxphost

Build command:

```sh
make build
```

Build output: build/paxd with the native kernel archive

Configuration: modules/lxphost and docs/site/paxeer-docs/src/app/layerx-kernel/page.tsx

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/layerx-kernel](https://docs.paxeer.app/layerx-kernel)

Repository references: docs/system/kernel-modules.md, docs/site/paxeer-docs/src/app/layerx-kernel/page.tsx

## module

### Epoch module

Identifier: module-epoch
Change in 7.0.0: unchanged

Directory: modules/epoch

Build command:

```sh
go build ./modules/epoch/...
```

Build output: linked into build/paxd

Configuration: modules/epoch/types and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/modules](https://docs.paxeer.app/modules)

Repository references: docs/system/chain-modules.md, docs/site/paxeer-docs/src/app/modules/page.tsx

### EVM module

Identifier: module-evm
Change in 7.0.0: unchanged

Directory: modules/evm

Build command:

```sh
go build ./modules/evm/...
```

Build output: linked into build/paxd

Configuration: modules/evm/types and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/modules](https://docs.paxeer.app/modules)

Repository references: docs/system/chain-modules.md, docs/site/paxeer-docs/src/app/modules/page.tsx

### Launchpad module

Identifier: module-launchpad
Change in 7.0.0: changed

Directory: modules/launchpad

Build command:

```sh
go build ./modules/launchpad/...
```

Build output: linked into build/paxd

Configuration: modules/launchpad/types and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/modules](https://docs.paxeer.app/modules)

Repository references: docs/system/chain-modules.md, docs/site/paxeer-docs/src/app/modules/page.tsx

### LayerX agent module

Identifier: module-layerx-agent
Change in 7.0.0: new

Directory: modules/layerxagent

Build command:

```sh
go build ./modules/layerxagent/...
```

Build output: linked into build/paxd

Configuration: modules/layerxagent/types and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/modules](https://docs.paxeer.app/modules)

Repository references: docs/system/chain-modules.md, docs/site/paxeer-docs/src/app/modules/page.tsx

### LayerX anchor module

Identifier: module-layerx-anchor
Change in 7.0.0: changed

Directory: modules/layerxanchor

Build command:

```sh
go build ./modules/layerxanchor/...
```

Build output: linked into build/paxd

Configuration: modules/layerxanchor/types and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/modules](https://docs.paxeer.app/modules)

Repository references: docs/system/chain-modules.md, docs/site/paxeer-docs/src/app/modules/page.tsx

### LayerX auth module

Identifier: module-layerx-auth
Change in 7.0.0: new

Directory: modules/layerxauth

Build command:

```sh
go build ./modules/layerxauth/...
```

Build output: linked into build/paxd

Configuration: modules/layerxauth/types and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/modules](https://docs.paxeer.app/modules)

Repository references: docs/system/chain-modules.md, docs/site/paxeer-docs/src/app/modules/page.tsx

### LayerX bridge module

Identifier: module-layerx-bridge
Change in 7.0.0: unchanged

Directory: modules/layerxbridge

Build command:

```sh
go build ./modules/layerxbridge/...
```

Build output: linked into build/paxd

Configuration: modules/layerxbridge/types and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/modules](https://docs.paxeer.app/modules)

Repository references: docs/system/chain-modules.md, docs/site/paxeer-docs/src/app/modules/page.tsx

### LayerX chains module

Identifier: module-layerx-chains
Change in 7.0.0: new

Directory: modules/layerxchains

Build command:

```sh
go build ./modules/layerxchains/...
```

Build output: linked into build/paxd

Configuration: modules/layerxchains/types and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/modules](https://docs.paxeer.app/modules)

Repository references: docs/system/chain-modules.md, docs/site/paxeer-docs/src/app/modules/page.tsx

### LayerX custody module

Identifier: module-layerx-custody
Change in 7.0.0: changed

Directory: modules/layerxcustody

Build command:

```sh
go build ./modules/layerxcustody/...
```

Build output: linked into build/paxd

Configuration: modules/layerxcustody/types and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/modules](https://docs.paxeer.app/modules)

Repository references: docs/system/chain-modules.md, docs/site/paxeer-docs/src/app/modules/page.tsx

### LayerX exchange module

Identifier: module-layerx-exchange
Change in 7.0.0: unchanged

Directory: modules/layerxexchange

Build command:

```sh
go build ./modules/layerxexchange/...
```

Build output: linked into build/paxd

Configuration: modules/layerxexchange/types and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/modules](https://docs.paxeer.app/modules)

Repository references: docs/system/chain-modules.md, docs/site/paxeer-docs/src/app/modules/page.tsx

### LayerX governance module

Identifier: module-layerx-gov
Change in 7.0.0: unchanged

Directory: modules/layerxgov

Build command:

```sh
go build ./modules/layerxgov/...
```

Build output: linked into build/paxd

Configuration: modules/layerxgov/types and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/modules](https://docs.paxeer.app/modules)

Repository references: docs/system/chain-modules.md, docs/site/paxeer-docs/src/app/modules/page.tsx

### LayerX ledger module

Identifier: module-layerx-ledger
Change in 7.0.0: new

Directory: modules/layerxledger

Build command:

```sh
go build ./modules/layerxledger/...
```

Build output: linked into build/paxd

Configuration: modules/layerxledger/types and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/modules](https://docs.paxeer.app/modules)

Repository references: docs/system/chain-modules.md, docs/site/paxeer-docs/src/app/modules/page.tsx

### LayerX pay module

Identifier: module-layerx-pay
Change in 7.0.0: new

Directory: modules/layerxpay

Build command:

```sh
go build ./modules/layerxpay/...
```

Build output: linked into build/paxd

Configuration: modules/layerxpay/types and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/modules](https://docs.paxeer.app/modules)

Repository references: docs/system/chain-modules.md, docs/site/paxeer-docs/src/app/modules/page.tsx

### Mint module

Identifier: module-mint
Change in 7.0.0: unchanged

Directory: modules/mint

Build command:

```sh
go build ./modules/mint/...
```

Build output: linked into build/paxd

Configuration: modules/mint/types and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/modules](https://docs.paxeer.app/modules)

Repository references: docs/system/chain-modules.md, docs/site/paxeer-docs/src/app/modules/page.tsx

### Oracle module

Identifier: module-oracle
Change in 7.0.0: unchanged

Directory: modules/oracle

Build command:

```sh
go build ./modules/oracle/...
```

Build output: linked into build/paxd

Configuration: modules/oracle/types and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/modules](https://docs.paxeer.app/modules)

Repository references: docs/system/chain-modules.md, docs/site/paxeer-docs/src/app/modules/page.tsx

### Perpetuals module

Identifier: module-perps
Change in 7.0.0: unchanged

Directory: modules/perps

Build command:

```sh
go build ./modules/perps/...
```

Build output: linked into build/paxd

Configuration: modules/perps/types and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/modules](https://docs.paxeer.app/modules)

Repository references: docs/system/chain-modules.md, docs/site/paxeer-docs/src/app/modules/page.tsx

### Price feed module

Identifier: module-pricefeed
Change in 7.0.0: unchanged

Directory: modules/pricefeed

Build command:

```sh
go build ./modules/pricefeed/...
```

Build output: linked into build/paxd

Configuration: modules/pricefeed/types and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/modules](https://docs.paxeer.app/modules)

Repository references: docs/system/chain-modules.md, docs/site/paxeer-docs/src/app/modules/page.tsx

### Embedded sequencer module

Identifier: module-sequencer
Change in 7.0.0: new

Directory: modules/sequencer

Build command:

```sh
go build ./modules/sequencer/...
```

Build output: linked into build/paxd

Configuration: modules/sequencer/types and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/modules](https://docs.paxeer.app/modules)

Repository references: docs/system/chain-modules.md, docs/site/paxeer-docs/src/app/modules/page.tsx

### Spot module

Identifier: module-spot
Change in 7.0.0: unchanged

Directory: modules/spot

Build command:

```sh
go build ./modules/spot/...
```

Build output: linked into build/paxd

Configuration: modules/spot/types and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/modules](https://docs.paxeer.app/modules)

Repository references: docs/system/chain-modules.md, docs/site/paxeer-docs/src/app/modules/page.tsx

### Store module

Identifier: module-store
Change in 7.0.0: unchanged

Directory: modules/store

Build command:

```sh
go build ./modules/store/...
```

Build output: linked into build/paxd

Configuration: modules/store/types and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/modules](https://docs.paxeer.app/modules)

Repository references: docs/system/chain-modules.md, docs/site/paxeer-docs/src/app/modules/page.tsx

### Token factory module

Identifier: module-tokenfactory
Change in 7.0.0: unchanged

Directory: modules/tokenfactory

Build command:

```sh
go build ./modules/tokenfactory/...
```

Build output: linked into build/paxd

Configuration: modules/tokenfactory/types and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/modules](https://docs.paxeer.app/modules)

Repository references: docs/system/chain-modules.md, docs/site/paxeer-docs/src/app/modules/page.tsx

### XWeb module

Identifier: module-xweb
Change in 7.0.0: unchanged

Directory: modules/xweb

Build command:

```sh
go build ./modules/xweb/...
```

Build output: linked into build/paxd

Configuration: modules/xweb/types and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/modules](https://docs.paxeer.app/modules)

Repository references: docs/system/chain-modules.md, docs/site/paxeer-docs/src/app/modules/page.tsx

## precompile

### Address binding precompiles

Identifier: precompile-addr
Change in 7.0.0: changed

Directory: precompiles/addr

Build command:

```sh
go build ./precompiles/addr/...
```

Build output: registered in build/paxd

Configuration: precompiles/addr/abi.json and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/precompiles](https://docs.paxeer.app/precompiles)

Repository references: docs/system/precompiles.md, docs/site/paxeer-docs/src/app/precompiles/page.tsx

### Bank precompile

Identifier: precompile-bank
Change in 7.0.0: unchanged

Directory: precompiles/bank

Build command:

```sh
go build ./precompiles/bank/...
```

Build output: registered in build/paxd

Configuration: precompiles/bank/abi.json and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/precompiles](https://docs.paxeer.app/precompiles)

Repository references: docs/system/precompiles.md, docs/site/paxeer-docs/src/app/precompiles/page.tsx

### Common token precompiles

Identifier: precompile-common
Change in 7.0.0: unchanged

Directory: precompiles/common

Build command:

```sh
go build ./precompiles/common/...
```

Build output: registered in build/paxd

Configuration: precompiles/common/abi.json and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/precompiles](https://docs.paxeer.app/precompiles)

Repository references: docs/system/precompiles.md, docs/site/paxeer-docs/src/app/precompiles/page.tsx

### Distribution precompiles

Identifier: precompile-distribution
Change in 7.0.0: unchanged

Directory: precompiles/distribution

Build command:

```sh
go build ./precompiles/distribution/...
```

Build output: registered in build/paxd

Configuration: precompiles/distribution/abi.json and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/precompiles](https://docs.paxeer.app/precompiles)

Repository references: docs/system/precompiles.md, docs/site/paxeer-docs/src/app/precompiles/page.tsx

### Fee token precompile

Identifier: precompile-feetoken
Change in 7.0.0: changed

Directory: precompiles/feetoken

Build command:

```sh
go build ./precompiles/feetoken/...
```

Build output: registered in build/paxd

Configuration: precompiles/feetoken/abi.json and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/precompiles](https://docs.paxeer.app/precompiles)

Repository references: docs/system/precompiles.md, docs/site/paxeer-docs/src/app/precompiles/page.tsx

### Governance precompile

Identifier: precompile-gov
Change in 7.0.0: unchanged

Directory: precompiles/gov

Build command:

```sh
go build ./precompiles/gov/...
```

Build output: registered in build/paxd

Configuration: precompiles/gov/abi.json and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/precompiles](https://docs.paxeer.app/precompiles)

Repository references: docs/system/precompiles.md, docs/site/paxeer-docs/src/app/precompiles/page.tsx

### IBC precompiles

Identifier: precompile-ibc
Change in 7.0.0: unchanged

Directory: precompiles/ibc

Build command:

```sh
go build ./precompiles/ibc/...
```

Build output: registered in build/paxd

Configuration: precompiles/ibc/abi.json and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/precompiles](https://docs.paxeer.app/precompiles)

Repository references: docs/system/precompiles.md, docs/site/paxeer-docs/src/app/precompiles/page.tsx

### JSON precompile

Identifier: precompile-json
Change in 7.0.0: unchanged

Directory: precompiles/json

Build command:

```sh
go build ./precompiles/json/...
```

Build output: registered in build/paxd

Configuration: precompiles/json/abi.json and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/precompiles](https://docs.paxeer.app/precompiles)

Repository references: docs/system/precompiles.md, docs/site/paxeer-docs/src/app/precompiles/page.tsx

### Launchpad precompiles

Identifier: precompile-launchpad
Change in 7.0.0: changed

Directory: precompiles/launchpad

Build command:

```sh
go build ./precompiles/launchpad/...
```

Build output: registered in build/paxd

Configuration: precompiles/launchpad/abi.json and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/precompiles](https://docs.paxeer.app/precompiles)

Repository references: docs/system/precompiles.md, docs/site/paxeer-docs/src/app/precompiles/page.tsx

### LayerX agent precompiles

Identifier: precompile-layerx-agent
Change in 7.0.0: new

Directory: precompiles/layerxagent

Build command:

```sh
go build ./precompiles/layerxagent/...
```

Build output: registered in build/paxd

Configuration: precompiles/layerxagent/abi.json and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/precompiles](https://docs.paxeer.app/precompiles)

Repository references: docs/system/precompiles.md, docs/site/paxeer-docs/src/app/precompiles/page.tsx

### LayerX anchor precompile

Identifier: precompile-layerx-anchor
Change in 7.0.0: changed

Directory: precompiles/layerxanchor

Build command:

```sh
go build ./precompiles/layerxanchor/...
```

Build output: registered in build/paxd

Configuration: precompiles/layerxanchor/abi.json and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/precompiles](https://docs.paxeer.app/precompiles)

Repository references: docs/system/precompiles.md, docs/site/paxeer-docs/src/app/precompiles/page.tsx

### LayerX auth precompile

Identifier: precompile-layerx-auth
Change in 7.0.0: new

Directory: precompiles/layerxauth

Build command:

```sh
go build ./precompiles/layerxauth/...
```

Build output: registered in build/paxd

Configuration: precompiles/layerxauth/abi.json and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/precompiles](https://docs.paxeer.app/precompiles)

Repository references: docs/system/precompiles.md, docs/site/paxeer-docs/src/app/precompiles/page.tsx

### LayerX bridge precompile

Identifier: precompile-layerx-bridge
Change in 7.0.0: changed

Directory: precompiles/layerxbridge

Build command:

```sh
go build ./precompiles/layerxbridge/...
```

Build output: registered in build/paxd

Configuration: precompiles/layerxbridge/abi.json and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/precompiles](https://docs.paxeer.app/precompiles)

Repository references: docs/system/precompiles.md, docs/site/paxeer-docs/src/app/precompiles/page.tsx

### LayerX chains precompile

Identifier: precompile-layerx-chains
Change in 7.0.0: new

Directory: precompiles/layerxchains

Build command:

```sh
go build ./precompiles/layerxchains/...
```

Build output: registered in build/paxd

Configuration: precompiles/layerxchains/abi.json and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/precompiles](https://docs.paxeer.app/precompiles)

Repository references: docs/system/precompiles.md, docs/site/paxeer-docs/src/app/precompiles/page.tsx

### LayerX custody precompile

Identifier: precompile-layerx-custody
Change in 7.0.0: changed

Directory: precompiles/layerxcustody

Build command:

```sh
go build ./precompiles/layerxcustody/...
```

Build output: registered in build/paxd

Configuration: precompiles/layerxcustody/abi.json and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/precompiles](https://docs.paxeer.app/precompiles)

Repository references: docs/system/precompiles.md, docs/site/paxeer-docs/src/app/precompiles/page.tsx

### LayerX exchange precompile

Identifier: precompile-layerx-exchange
Change in 7.0.0: unchanged

Directory: precompiles/layerxexchange

Build command:

```sh
go build ./precompiles/layerxexchange/...
```

Build output: registered in build/paxd

Configuration: precompiles/layerxexchange/abi.json and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/precompiles](https://docs.paxeer.app/precompiles)

Repository references: docs/system/precompiles.md, docs/site/paxeer-docs/src/app/precompiles/page.tsx

### LayerX ledger precompile

Identifier: precompile-layerx-ledger
Change in 7.0.0: new

Directory: precompiles/layerxledger

Build command:

```sh
go build ./precompiles/layerxledger/...
```

Build output: registered in build/paxd

Configuration: precompiles/layerxledger/abi.json and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/precompiles](https://docs.paxeer.app/precompiles)

Repository references: docs/system/precompiles.md, docs/site/paxeer-docs/src/app/precompiles/page.tsx

### LayerX pay precompile

Identifier: precompile-layerx-pay
Change in 7.0.0: new

Directory: precompiles/layerxpay

Build command:

```sh
go build ./precompiles/layerxpay/...
```

Build output: registered in build/paxd

Configuration: precompiles/layerxpay/abi.json and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/precompiles](https://docs.paxeer.app/precompiles)

Repository references: docs/system/precompiles.md, docs/site/paxeer-docs/src/app/precompiles/page.tsx

### LayerX programs precompile

Identifier: precompile-layerx-programs
Change in 7.0.0: changed

Directory: precompiles/layerxprograms

Build command:

```sh
go build ./precompiles/layerxprograms/...
```

Build output: registered in build/paxd

Configuration: precompiles/layerxprograms/abi.json and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/precompiles](https://docs.paxeer.app/precompiles)

Repository references: docs/system/precompiles.md, docs/site/paxeer-docs/src/app/precompiles/page.tsx

### LayerX verify precompile

Identifier: precompile-layerx-verify
Change in 7.0.0: changed

Directory: precompiles/layerxverify

Build command:

```sh
go build ./precompiles/layerxverify/...
```

Build output: registered in build/paxd

Configuration: precompiles/layerxverify/abi.json and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/precompiles](https://docs.paxeer.app/precompiles)

Repository references: docs/system/precompiles.md, docs/site/paxeer-docs/src/app/precompiles/page.tsx

### Oracle precompile

Identifier: precompile-oracle
Change in 7.0.0: unchanged

Directory: precompiles/oracle

Build command:

```sh
go build ./precompiles/oracle/...
```

Build output: registered in build/paxd

Configuration: precompiles/oracle/abi.json and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/precompiles](https://docs.paxeer.app/precompiles)

Repository references: docs/system/precompiles.md, docs/site/paxeer-docs/src/app/precompiles/page.tsx

### P-256 precompile

Identifier: precompile-p256
Change in 7.0.0: unchanged

Directory: precompiles/p256

Build command:

```sh
go build ./precompiles/p256/...
```

Build output: registered in build/paxd

Configuration: precompiles/p256/abi.json and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/precompiles](https://docs.paxeer.app/precompiles)

Repository references: docs/system/precompiles.md, docs/site/paxeer-docs/src/app/precompiles/page.tsx

### Perpetuals precompile

Identifier: precompile-perps
Change in 7.0.0: unchanged

Directory: precompiles/perps

Build command:

```sh
go build ./precompiles/perps/...
```

Build output: registered in build/paxd

Configuration: precompiles/perps/abi.json and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/precompiles](https://docs.paxeer.app/precompiles)

Repository references: docs/system/precompiles.md, docs/site/paxeer-docs/src/app/precompiles/page.tsx

### Pointer precompile

Identifier: precompile-pointer
Change in 7.0.0: unchanged

Directory: precompiles/pointer

Build command:

```sh
go build ./precompiles/pointer/...
```

Build output: registered in build/paxd

Configuration: precompiles/pointer/abi.json and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/precompiles](https://docs.paxeer.app/precompiles)

Repository references: docs/system/precompiles.md, docs/site/paxeer-docs/src/app/precompiles/page.tsx

### Pointer view precompile

Identifier: precompile-pointerview
Change in 7.0.0: unchanged

Directory: precompiles/pointerview

Build command:

```sh
go build ./precompiles/pointerview/...
```

Build output: registered in build/paxd

Configuration: precompiles/pointerview/abi.json and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/precompiles](https://docs.paxeer.app/precompiles)

Repository references: docs/system/precompiles.md, docs/site/paxeer-docs/src/app/precompiles/page.tsx

### Price feed precompile

Identifier: precompile-pricefeed
Change in 7.0.0: unchanged

Directory: precompiles/pricefeed

Build command:

```sh
go build ./precompiles/pricefeed/...
```

Build output: registered in build/paxd

Configuration: precompiles/pricefeed/abi.json and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/precompiles](https://docs.paxeer.app/precompiles)

Repository references: docs/system/precompiles.md, docs/site/paxeer-docs/src/app/precompiles/page.tsx

### Solo precompile

Identifier: precompile-solo
Change in 7.0.0: unchanged

Directory: precompiles/solo

Build command:

```sh
go build ./precompiles/solo/...
```

Build output: registered in build/paxd

Configuration: precompiles/solo/abi.json and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/precompiles](https://docs.paxeer.app/precompiles)

Repository references: docs/system/precompiles.md, docs/site/paxeer-docs/src/app/precompiles/page.tsx

### Spot precompiles

Identifier: precompile-spot
Change in 7.0.0: unchanged

Directory: precompiles/spot

Build command:

```sh
go build ./precompiles/spot/...
```

Build output: registered in build/paxd

Configuration: precompiles/spot/abi.json and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/precompiles](https://docs.paxeer.app/precompiles)

Repository references: docs/system/precompiles.md, docs/site/paxeer-docs/src/app/precompiles/page.tsx

### Staking precompile

Identifier: precompile-staking
Change in 7.0.0: unchanged

Directory: precompiles/staking

Build command:

```sh
go build ./precompiles/staking/...
```

Build output: registered in build/paxd

Configuration: precompiles/staking/abi.json and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/precompiles](https://docs.paxeer.app/precompiles)

Repository references: docs/system/precompiles.md, docs/site/paxeer-docs/src/app/precompiles/page.tsx

### Wasm precompile

Identifier: precompile-wasmd
Change in 7.0.0: unchanged

Directory: precompiles/wasmd

Build command:

```sh
go build ./precompiles/wasmd/...
```

Build output: registered in build/paxd

Configuration: precompiles/wasmd/abi.json and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/precompiles](https://docs.paxeer.app/precompiles)

Repository references: docs/system/precompiles.md, docs/site/paxeer-docs/src/app/precompiles/page.tsx

### XWeb precompile

Identifier: precompile-xweb
Change in 7.0.0: unchanged

Directory: precompiles/xweb

Build command:

```sh
go build ./precompiles/xweb/...
```

Build output: registered in build/paxd

Configuration: precompiles/xweb/abi.json and node/app.go

Ports: None listed

Health check: paxd status

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/precompiles](https://docs.paxeer.app/precompiles)

Repository references: docs/system/precompiles.md, docs/site/paxeer-docs/src/app/precompiles/page.tsx

## service

### Hosted gateway

Identifier: gateway
Change in 7.0.0: changed

Directory: services/platform/hosted/gateway

Build command:

```sh
cargo build --locked --release --manifest-path services/platform/Cargo.toml --package layerx-platform-gateway
```

Build output: services/platform/target/release/layerx-gateway

Configuration: services/platform/hosted/gateway/deployment.yaml and README.md

Ports: 9443

Health check: GET /readyz

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/services](https://docs.paxeer.app/services)

Repository references: docs/release/7.0.0-service-operator-runbook.md, docs/site/paxeer-docs/src/app/services/page.tsx

### Explorer backend

Identifier: explorer-backend
Change in 7.0.0: changed

Directory: services/explorer/backend

Build command:

```sh
mix release
```

Build output: _build/prod/rel/blockscout/bin/blockscout

Configuration: services/explorer/backend/config/runtime.exs

Ports: 4000

Health check: GET /api/healthz

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node), [Hosted gateway](https://docs.paxeer.app/system-map#system-gateway)

Site pages: See repository references.

Repository references: docs/release/7.0.0-service-operator-runbook.md, docs/explorer/ENVS.md

### Agent daemon

Identifier: agent-daemon
Change in 7.0.0: changed

Directory: services/agent

Build command:

```sh
cargo build --locked --release --manifest-path services/agent/Cargo.toml --package layerx-agentd-host
```

Build output: services/agent/target/release/layerx-agentd

Configuration: services/agent/crates/layerx-agentd README and environment variables

Ports: 8790

Health check: GET /healthz

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node), [Hosted gateway](https://docs.paxeer.app/system-map#system-gateway)

Site pages: [/agent-runtime](https://docs.paxeer.app/agent-runtime)

Repository references: docs/release/7.0.0-service-operator-runbook.md, docs/site/paxeer-docs/src/app/agent-runtime/page.tsx

### Interop workspace

Identifier: interop
Change in 7.0.0: changed

Directory: services/interop

Build command:

```sh
cargo build --locked --release --manifest-path services/interop/Cargo.toml
```

Build output: services/interop/target/release binaries

Configuration: services/interop/deploy and README.md

Ports: None listed

Health check: cargo run --manifest-path services/interop/Cargo.toml --bin layerx-interop-gateway -- --help

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/interop](https://docs.paxeer.app/interop)

Repository references: services/interop/README.md, docs/site/paxeer-docs/src/app/interop/page.tsx

### Bridge relayer

Identifier: bridge-relayer
Change in 7.0.0: changed

Directory: services/interop/crates/layerx-bridge-relayer

Build command:

```sh
cargo build --locked --release --manifest-path services/interop/Cargo.toml --bin layerx-bridge-relayer
```

Build output: services/interop/target/release/layerx-bridge-relayer

Configuration: services/interop/deploy/bridge-relayer and contracts/bridge/README.md

Ports: 8545

Health check: layerx-bridge-relayer --help

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node), [LayerX bridge module](https://docs.paxeer.app/system-map#system-module-layerx-bridge)

Site pages: See repository references.

Repository references: docs/release/7.0.0-service-operator-runbook.md, contracts/bridge/README.md

### Launchpad indexer

Identifier: launchpad-indexer
Change in 7.0.0: changed

Directory: services/launchpad/indexer

Build command:

```sh
go build ./...
```

Build output: services/launchpad/indexer/indexerd

Configuration: services/launchpad/indexer/internal/config/config.go

Ports: 8080

Health check: GET /health/ready

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node), [LayerX chains module](https://docs.paxeer.app/system-map#system-module-layerx-chains)

Site pages: [/launchpad](https://docs.paxeer.app/launchpad)

Repository references: docs/release/7.0.0-service-operator-runbook.md, docs/site/paxeer-docs/src/app/launchpad/page.tsx

### Wallet gateway

Identifier: wallet-gateway
Change in 7.0.0: changed

Directory: services/wallet/gateway

Build command:

```sh
pnpm --dir services/wallet --filter ./gateway build
```

Build output: services/wallet/gateway/dist

Configuration: services/wallet/deploy/env and gateway/src/env.ts

Ports: 8787

Health check: GET /health

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node), [Hosted gateway](https://docs.paxeer.app/system-map#system-gateway)

Site pages: [/accounts](https://docs.paxeer.app/accounts)

Repository references: docs/wallet/service/cutover.md

### Human service

Identifier: human-service
Change in 7.0.0: unchanged

Directory: services/human

Build command:

```sh
cargo build --locked --release --manifest-path services/human/Cargo.toml
```

Build output: services/human/target/release binaries

Configuration: services/human/crates/layerx-human-service README.md

Ports: 8080

Health check: GET /readyz

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node), [Hosted gateway](https://docs.paxeer.app/system-map#system-gateway)

Site pages: [/human-interface](https://docs.paxeer.app/human-interface)

Repository references: services/human/README.md, docs/site/paxeer-docs/src/app/human-interface/page.tsx

### Bridge deployment service

Identifier: service-bridge
Change in 7.0.0: changed

Directory: services/bridge

Build command:

```sh
bash services/bridge/deploy/checklist.sh --all
```

Build output: deployment validation output

Configuration: services/bridge/deploy and contracts/bridge/README.md

Ports: None listed

Health check: bash services/bridge/deploy/checklist.sh --all

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node), [LayerX bridge module](https://docs.paxeer.app/system-map#system-module-layerx-bridge)

Site pages: See repository references.

Repository references: docs/release/7.0.0-service-operator-runbook.md, contracts/bridge/README.md

### Launchpad service workspace

Identifier: service-launchpad
Change in 7.0.0: changed

Directory: services/launchpad

Build command:

```sh
make -C services/launchpad build
```

Build output: services/launchpad binaries

Configuration: services/launchpad/Makefile and services/launchpad/indexer/internal/config/config.go

Ports: PORT

Health check: GET /health/ready

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node), [LayerX chains module](https://docs.paxeer.app/system-map#system-module-layerx-chains)

Site pages: [/launchpad](https://docs.paxeer.app/launchpad)

Repository references: docs/release/7.0.0-service-operator-runbook.md, docs/site/paxeer-docs/src/app/launchpad/page.tsx

### Platform hosted services

Identifier: service-platform
Change in 7.0.0: changed

Directory: services/platform

Build command:

```sh
cargo build --locked --release --manifest-path services/platform/Cargo.toml
```

Build output: services/platform/target/release binaries

Configuration: services/platform/hosted/gateway/README.md and deployment.yaml

Ports: 9443

Health check: GET /readyz (HTTPS)

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/services](https://docs.paxeer.app/services)

Repository references: docs/site/paxeer-docs/src/app/services/page.tsx, docs/release/7.0.0-service-operator-runbook.md

## application

### Bridge application

Identifier: bridge
Change in 7.0.0: changed

Directory: apps/bridge

Build command:

```sh
pnpm --dir apps/bridge build
```

Build output: apps/bridge/.next

Configuration: apps/bridge/src/lib/bridge/config.ts

Ports: 3000

Health check: GET /

Depends on: [Hosted gateway](https://docs.paxeer.app/system-map#system-gateway), [Bridge relayer](https://docs.paxeer.app/system-map#system-bridge-relayer)

Site pages: [/bridge](https://docs.paxeer.app/bridge)

Repository references: docs/release/7.0.0-service-operator-runbook.md, docs/site/paxeer-docs/src/app/bridge/page.tsx

### Explorer application

Identifier: explorer
Change in 7.0.0: changed

Directory: apps/explorer

Build command:

```sh
pnpm --dir apps/explorer build
```

Build output: apps/explorer/.next

Configuration: apps/explorer/configs and docs/explorer/ENVS.md

Ports: 3000

Health check: GET /api/healthz

Depends on: [Explorer backend](https://docs.paxeer.app/system-map#system-explorer-backend)

Site pages: [/services](https://docs.paxeer.app/services)

Repository references: docs/release/7.0.0-service-operator-runbook.md

### Inference peer

Identifier: inference-peer
Change in 7.0.0: unchanged

Directory: apps/inference-peer

Build command:

```sh
npm --prefix apps/inference-peer test
```

Build output: apps/inference-peer/engine

Configuration: apps/inference-peer/package.json

Ports: 8787

Health check: npm --prefix apps/inference-peer test

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/agents](https://docs.paxeer.app/agents)

Repository references: docs/site/paxeer-docs/src/app/agents/page.tsx

### Launchpad web application

Identifier: launchpad
Change in 7.0.0: changed

Directory: apps/launchpad/web

Build command:

```sh
pnpm --dir apps/launchpad/web build
```

Build output: apps/launchpad/web/.next

Configuration: apps/launchpad/web/src/core/rpc.ts

Ports: 3000

Health check: GET /

Depends on: [Launchpad indexer](https://docs.paxeer.app/system-map#system-launchpad-indexer), [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/launchpad](https://docs.paxeer.app/launchpad)

Repository references: docs/release/7.0.0-service-operator-runbook.md, docs/site/paxeer-docs/src/app/launchpad/page.tsx

### Names application

Identifier: names
Change in 7.0.0: unchanged

Directory: apps/names

Build command:

```sh
pnpm --dir apps/names build
```

Build output: apps/names/.next

Configuration: apps/names/package.json

Ports: 3000

Health check: GET /

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/sdk](https://docs.paxeer.app/sdk)

Repository references: docs/system/sdk.md

### Perpetuals application

Identifier: perps
Change in 7.0.0: unchanged

Directory: apps/perps

Build command:

```sh
pnpm --dir apps/perps build
```

Build output: apps/perps/.next

Configuration: apps/perps/deploy and package.json

Ports: 3000

Health check: GET /

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/perpetuals](https://docs.paxeer.app/perpetuals)

Repository references: docs/site/paxeer-docs/src/app/perpetuals/page.tsx, docs/perps/data-api.md

### Wallet application

Identifier: wallet
Change in 7.0.0: changed

Directory: apps/wallet

Build command:

```sh
pnpm --dir apps/wallet build
```

Build output: apps/wallet/.next

Configuration: apps/wallet/src/account/config.ts

Ports: 3000

Health check: GET /

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node), [Hosted gateway](https://docs.paxeer.app/system-map#system-gateway)

Site pages: [/accounts](https://docs.paxeer.app/accounts)

Repository references: docs/release/7.0.0-service-operator-runbook.md

### Human web application

Identifier: human-web
Change in 7.0.0: unchanged

Directory: apps/web

Build command:

```sh
npm --prefix apps/web run build
```

Build output: apps/web/.next

Configuration: apps/web/package.json and docs/platform/site.kvx

Ports: 3000

Health check: GET /

Depends on: [Human service](https://docs.paxeer.app/system-map#system-human-service)

Site pages: [/human-interface](https://docs.paxeer.app/human-interface)

Repository references: docs/site/paxeer-docs/src/app/human-interface/page.tsx, docs/platform/content/index.md

### Bridge application

Identifier: app-bridge
Change in 7.0.0: changed

Directory: apps/bridge

Build command:

```sh
pnpm --dir apps/bridge build
```

Build output: apps/bridge/.next

Configuration: apps/bridge/src/lib/bridge/config.ts

Ports: 3000

Health check: GET /

Depends on: [Hosted gateway](https://docs.paxeer.app/system-map#system-gateway)

Site pages: [/bridge](https://docs.paxeer.app/bridge)

Repository references: docs/release/7.0.0-service-operator-runbook.md, docs/site/paxeer-docs/src/app/bridge/page.tsx

### Explorer application

Identifier: app-explorer
Change in 7.0.0: changed

Directory: apps/explorer

Build command:

```sh
pnpm --dir apps/explorer build
```

Build output: apps/explorer/.next

Configuration: apps/explorer/configs and docs/explorer/ENVS.md

Ports: 3000

Health check: GET /api/healthz

Depends on: [Explorer backend](https://docs.paxeer.app/system-map#system-explorer-backend)

Site pages: [/services](https://docs.paxeer.app/services)

Repository references: docs/release/7.0.0-service-operator-runbook.md, docs/site/paxeer-docs/src/app/services/page.tsx

### Launchpad web application

Identifier: app-launchpad
Change in 7.0.0: changed

Directory: apps/launchpad/web

Build command:

```sh
pnpm --dir apps/launchpad/web build
```

Build output: apps/launchpad/web/.next

Configuration: apps/launchpad/web/src/core/rpc.ts

Ports: 3000

Health check: GET /

Depends on: [Launchpad indexer](https://docs.paxeer.app/system-map#system-launchpad-indexer), [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/launchpad](https://docs.paxeer.app/launchpad)

Repository references: docs/release/7.0.0-service-operator-runbook.md, docs/site/paxeer-docs/src/app/launchpad/page.tsx

### Wallet application

Identifier: app-wallet
Change in 7.0.0: changed

Directory: apps/wallet

Build command:

```sh
pnpm --dir apps/wallet build
```

Build output: apps/wallet/.next

Configuration: apps/wallet/src/account/config.ts

Ports: 3000

Health check: GET /

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node), [Hosted gateway](https://docs.paxeer.app/system-map#system-gateway)

Site pages: [/accounts](https://docs.paxeer.app/accounts)

Repository references: docs/release/7.0.0-service-operator-runbook.md, docs/site/paxeer-docs/src/app/accounts/page.tsx

### Human web application

Identifier: app-web
Change in 7.0.0: unchanged

Directory: apps/web

Build command:

```sh
npm --prefix apps/web run build
```

Build output: apps/web/.next

Configuration: apps/web/package.json

Ports: 3000

Health check: GET /

Depends on: [Human service](https://docs.paxeer.app/system-map#system-human-service)

Site pages: [/human-interface](https://docs.paxeer.app/human-interface)

Repository references: docs/platform/content/index.md, docs/site/paxeer-docs/src/app/human-interface/page.tsx

### Inference peer

Identifier: app-inference-peer
Change in 7.0.0: unchanged

Directory: apps/inference-peer

Build command:

```sh
npm --prefix apps/inference-peer test
```

Build output: apps/inference-peer/engine

Configuration: apps/inference-peer/scripts/serve.mjs

Ports: 8787

Health check: GET /probe

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/agents](https://docs.paxeer.app/agents)

Repository references: docs/site/paxeer-docs/src/app/agents/page.tsx

### Names application

Identifier: app-names
Change in 7.0.0: unchanged

Directory: apps/names

Build command:

```sh
pnpm --dir apps/names build
```

Build output: apps/names/.next

Configuration: apps/names/package.json

Ports: 3000

Health check: GET /

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: See repository references.

Repository references: docs/system/sdk.md

Additional inventory reference: /names (no published site page)
### Perpetuals application

Identifier: app-perps
Change in 7.0.0: unchanged

Directory: apps/perps

Build command:

```sh
pnpm --dir apps/perps build
```

Build output: apps/perps/.next

Configuration: apps/perps/deploy and package.json

Ports: 3000

Health check: GET /

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/perpetuals](https://docs.paxeer.app/perpetuals)

Repository references: docs/perps/data-api.md, docs/site/paxeer-docs/src/app/perpetuals/page.tsx

## sdk

### Wallet SDK

Identifier: wallet-sdk
Change in 7.0.0: changed

Directory: sdk/wallet

Build command:

```sh
pnpm --dir sdk/wallet build
```

Build output: sdk/wallet/dist

Configuration: sdk/wallet/package.json

Ports: None listed

Health check: package exports load

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/sdk](https://docs.paxeer.app/sdk)

Repository references: docs/system/sdk.md, docs/site/paxeer-docs/src/app/sdk/page.tsx

### Cosmos TypeScript SDK

Identifier: cosmos-sdk
Change in 7.0.0: unchanged

Directory: sdk/js

Build command:

```sh
npm --prefix sdk/js test
```

Build output: sdk/js/dist

Configuration: sdk/js/package.json

Ports: None listed

Health check: node --test sdk/js/test/

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/sdk](https://docs.paxeer.app/sdk)

Repository references: docs/system/sdk.md, docs/site/paxeer-docs/src/app/sdk/page.tsx

### Names SDK

Identifier: names-sdk
Change in 7.0.0: unchanged

Directory: sdk/names

Build command:

```sh
pnpm --dir sdk/names build
```

Build output: sdk/names/dist

Configuration: sdk/names/package.json

Ports: None listed

Health check: package exports load

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/sdk](https://docs.paxeer.app/sdk)

Repository references: docs/site/paxeer-docs/src/app/sdk/page.tsx

### Agent TypeScript SDK

Identifier: agent-sdk
Change in 7.0.0: changed

Directory: services/agent/sdk/typescript

Build command:

```sh
npm --prefix services/agent/sdk/typescript run build
```

Build output: services/agent/sdk/typescript/dist

Configuration: services/agent/sdk/typescript/package.json

Ports: None listed

Health check: package exports load

Depends on: [Agent daemon](https://docs.paxeer.app/system-map#system-agent-daemon), [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/sdk](https://docs.paxeer.app/sdk)

Repository references: docs/system/sdk.md, docs/site/paxeer-docs/src/app/sdk/page.tsx

### Chainflow data SDK

Identifier: chainflow-sdk
Change in 7.0.0: unchanged

Directory: sdk/chainflow/typescript

Build command:

```sh
npm --prefix sdk/chainflow/typescript run build
```

Build output: sdk/chainflow/typescript/dist

Configuration: sdk/chainflow/typescript/pyproject.toml and package.json

Ports: None listed

Health check: package exports load

Depends on: None

Site pages: [/sdk](https://docs.paxeer.app/sdk)

Repository references: docs/site/paxeer-docs/src/app/sdk/page.tsx

### Programs Rust SDK

Identifier: program-rust-sdk
Change in 7.0.0: changed

Directory: programs/sdk/rust

Build command:

```sh
cargo build --locked --manifest-path programs/sdk/rust/Cargo.toml
```

Build output: programs/sdk/rust/target artifacts

Configuration: programs/sdk/rust/Cargo.toml

Ports: None listed

Health check: cargo test --manifest-path programs/sdk/rust/Cargo.toml

Depends on: [LayerX kernel host](https://docs.paxeer.app/system-map#system-kernel)

Site pages: [/programs](https://docs.paxeer.app/programs)

Repository references: docs/system/sdk.md, docs/site/paxeer-docs/src/app/programs/page.tsx

### Programs AssemblyScript SDK

Identifier: program-assemblyscript-sdk
Change in 7.0.0: unchanged

Directory: programs/sdk/assemblyscript

Build command:

```sh
npm --prefix programs/sdk/assemblyscript pack --dry-run
```

Build output: programs/sdk/assemblyscript/package archive

Configuration: programs/sdk/assemblyscript/package.json

Ports: None listed

Health check: package exports load

Depends on: [LayerX kernel host](https://docs.paxeer.app/system-map#system-kernel)

Site pages: [/programs](https://docs.paxeer.app/programs)

Repository references: docs/site/paxeer-docs/src/app/programs/page.tsx

### Agent Python SDK

Identifier: agent-python-sdk
Change in 7.0.0: changed

Directory: services/agent/sdk/python

Build command:

```sh
python3 -m build services/agent/sdk/python
```

Build output: services/agent/sdk/python/dist

Configuration: services/agent/sdk/python/pyproject.toml

Ports: None listed

Health check: python3 -m build services/agent/sdk/python

Depends on: [Agent daemon](https://docs.paxeer.app/system-map#system-agent-daemon)

Site pages: [/sdk](https://docs.paxeer.app/sdk)

Repository references: docs/system/sdk.md

### Agent JVM SDK

Identifier: agent-jvm-sdk
Change in 7.0.0: unchanged

Directory: sdk/platform/jvm

Build command:

```sh
mvn -f sdk/platform/jvm/pom.xml package
```

Build output: sdk/platform/jvm/target/*.jar

Configuration: sdk/platform/jvm/pom.xml

Ports: None listed

Health check: mvn -f sdk/platform/jvm/pom.xml test

Depends on: [Agent daemon](https://docs.paxeer.app/system-map#system-agent-daemon)

Site pages: [/sdk](https://docs.paxeer.app/sdk)

Repository references: docs/site/paxeer-docs/src/app/sdk/page.tsx, sdk/platform/jvm/README.md

### Agent .NET SDK

Identifier: agent-dotnet-sdk
Change in 7.0.0: unchanged

Directory: sdk/platform/dotnet

Build command:

```sh
dotnet build sdk/platform/dotnet/LayerX.Sdk.csproj
```

Build output: sdk/platform/dotnet/bin and obj

Configuration: sdk/platform/dotnet/LayerX.Sdk.csproj

Ports: None listed

Health check: dotnet build sdk/platform/dotnet/LayerX.Sdk.csproj

Depends on: [Agent daemon](https://docs.paxeer.app/system-map#system-agent-daemon)

Site pages: [/sdk](https://docs.paxeer.app/sdk)

Repository references: docs/platform/content/quickstart/csharp.md

### Agent Swift SDK

Identifier: agent-swift-sdk
Change in 7.0.0: unchanged

Directory: sdk/platform/swift

Build command:

```sh
swift build --package-path sdk/platform/swift
```

Build output: sdk/platform/swift/.build

Configuration: sdk/platform/swift/Package.swift

Ports: None listed

Health check: swift test --package-path sdk/platform/swift

Depends on: [Agent daemon](https://docs.paxeer.app/system-map#system-agent-daemon)

Site pages: [/sdk](https://docs.paxeer.app/sdk)

Repository references: docs/platform/content/quickstart/swift.md

### Chainflow Go SDK

Identifier: chainflow-go-sdk
Change in 7.0.0: unchanged

Directory: sdk/chainflow/go

Build command:

```sh
go build ./...
```

Build output: Go package artifacts

Configuration: sdk/chainflow/go/go.mod

Ports: None listed

Health check: go test ./...

Depends on: None

Site pages: See repository references.

Repository references: sdk/chainflow/go/README.md

### Chainflow Python SDK

Identifier: chainflow-python-sdk
Change in 7.0.0: unchanged

Directory: sdk/chainflow/python

Build command:

```sh
python3 -m build sdk/chainflow/python
```

Build output: sdk/chainflow/python/dist

Configuration: sdk/chainflow/python/pyproject.toml

Ports: None listed

Health check: python3 -m build sdk/chainflow/python

Depends on: None

Site pages: See repository references.

Repository references: docs/perps/data-api.md, sdk/chainflow/python/README.md

### Chainflow Rust SDK

Identifier: chainflow-rust-sdk
Change in 7.0.0: unchanged

Directory: sdk/chainflow/rust

Build command:

```sh
cargo build --locked --release --manifest-path sdk/chainflow/rust/Cargo.toml
```

Build output: sdk/chainflow/rust/target artifacts

Configuration: sdk/chainflow/rust/Cargo.toml

Ports: None listed

Health check: cargo test --manifest-path sdk/chainflow/rust/Cargo.toml

Depends on: None

Site pages: See repository references.

Repository references: docs/perps/data-api.md, sdk/chainflow/rust/README.md

### Wallet SDK

Identifier: sdk-wallet
Change in 7.0.0: changed

Directory: sdk/wallet

Build command:

```sh
pnpm --dir sdk/wallet build
```

Build output: sdk/wallet/dist

Configuration: sdk/wallet/package.json

Ports: None listed

Health check: package exports load

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/sdk](https://docs.paxeer.app/sdk)

Repository references: docs/system/sdk.md, docs/site/paxeer-docs/src/app/sdk/page.tsx

### Cosmos TypeScript SDK

Identifier: sdk-cosmos
Change in 7.0.0: unchanged

Directory: sdk/js

Build command:

```sh
npm --prefix sdk/js test
```

Build output: sdk/js/dist

Configuration: sdk/js/package.json

Ports: None listed

Health check: package exports load

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/sdk](https://docs.paxeer.app/sdk)

Repository references: docs/system/sdk.md, docs/site/paxeer-docs/src/app/sdk/page.tsx

### Names SDK

Identifier: sdk-names
Change in 7.0.0: unchanged

Directory: sdk/names

Build command:

```sh
pnpm --dir sdk/names build
```

Build output: sdk/names/dist

Configuration: sdk/names/package.json

Ports: None listed

Health check: package exports load

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/sdk](https://docs.paxeer.app/sdk)

Repository references: docs/site/paxeer-docs/src/app/sdk/page.tsx

### Agent TypeScript SDK

Identifier: sdk-agent-typescript
Change in 7.0.0: changed

Directory: services/agent/sdk/typescript

Build command:

```sh
npm --prefix services/agent/sdk/typescript run build
```

Build output: services/agent/sdk/typescript/dist

Configuration: services/agent/sdk/typescript/package.json

Ports: None listed

Health check: package exports load

Depends on: [Agent daemon](https://docs.paxeer.app/system-map#system-agent-daemon)

Site pages: [/sdk](https://docs.paxeer.app/sdk)

Repository references: docs/system/sdk.md, docs/site/paxeer-docs/src/app/sdk/page.tsx

### Programs Rust SDK

Identifier: sdk-program-rust
Change in 7.0.0: changed

Directory: programs/sdk/rust

Build command:

```sh
cargo build --locked --release --manifest-path programs/Cargo.toml -p layerx-program-sdk
```

Build output: programs/target/release/deps

Configuration: programs/sdk/rust/Cargo.toml

Ports: None listed

Health check: cargo test --manifest-path programs/sdk/rust/Cargo.toml

Depends on: [LayerX kernel host](https://docs.paxeer.app/system-map#system-kernel)

Site pages: [/programs](https://docs.paxeer.app/programs)

Repository references: docs/system/sdk.md, docs/site/paxeer-docs/src/app/programs/page.tsx

### Programs AssemblyScript SDK

Identifier: sdk-program-assemblyscript
Change in 7.0.0: changed

Directory: programs/sdk/assemblyscript

Build command:

```sh
npm --prefix programs/sdk/assemblyscript run test:source
```

Build output: programs/sdk/assemblyscript/build/safety.wasm

Configuration: programs/sdk/assemblyscript/package.json

Ports: None listed

Health check: package exports load

Depends on: [LayerX kernel host](https://docs.paxeer.app/system-map#system-kernel)

Site pages: [/programs](https://docs.paxeer.app/programs)

Repository references: docs/site/paxeer-docs/src/app/programs/page.tsx

### Chainflow TypeScript SDK

Identifier: sdk-chainflow-typescript
Change in 7.0.0: unchanged

Directory: sdk/chainflow/typescript

Build command:

```sh
npm --prefix sdk/chainflow/typescript run build
```

Build output: sdk/chainflow/typescript/dist

Configuration: sdk/chainflow/typescript/package.json

Ports: None listed

Health check: package exports load

Depends on: [Hosted gateway](https://docs.paxeer.app/system-map#system-gateway)

Site pages: [/sdk](https://docs.paxeer.app/sdk)

Repository references: docs/site/paxeer-docs/src/app/sdk/page.tsx

### Chainflow Go SDK

Identifier: sdk-chainflow-go
Change in 7.0.0: unchanged

Directory: sdk/chainflow/go

Build command:

```sh
go build ./...
```

Build output: Go package artifacts

Configuration: sdk/chainflow/go/go.mod

Ports: None listed

Health check: go test ./...

Depends on: None

Site pages: See repository references.

Repository references: sdk/chainflow/go/README.md

### Chainflow Python SDK

Identifier: sdk-chainflow-python
Change in 7.0.0: unchanged

Directory: sdk/chainflow/python

Build command:

```sh
python3 -m build sdk/chainflow/python
```

Build output: sdk/chainflow/python/dist

Configuration: sdk/chainflow/python/pyproject.toml

Ports: None listed

Health check: python3 -m build sdk/chainflow/python

Depends on: None

Site pages: See repository references.

Repository references: docs/perps/data-api.md, sdk/chainflow/python/README.md

### Chainflow Rust SDK

Identifier: sdk-chainflow-rust
Change in 7.0.0: unchanged

Directory: sdk/chainflow/rust

Build command:

```sh
cargo build --locked --release --manifest-path sdk/chainflow/rust/Cargo.toml
```

Build output: sdk/chainflow/rust/target artifacts

Configuration: sdk/chainflow/rust/Cargo.toml

Ports: None listed

Health check: cargo test --manifest-path sdk/chainflow/rust/Cargo.toml

Depends on: None

Site pages: See repository references.

Repository references: docs/perps/data-api.md, sdk/chainflow/rust/README.md

### Agent JVM SDK

Identifier: sdk-agent-jvm
Change in 7.0.0: unchanged

Directory: sdk/platform/jvm

Build command:

```sh
mvn -f sdk/platform/jvm/pom.xml package
```

Build output: sdk/platform/jvm/target/*.jar

Configuration: sdk/platform/jvm/pom.xml

Ports: None listed

Health check: mvn -f sdk/platform/jvm/pom.xml test

Depends on: [Agent daemon](https://docs.paxeer.app/system-map#system-agent-daemon)

Site pages: [/sdk](https://docs.paxeer.app/sdk)

Repository references: docs/site/paxeer-docs/src/app/sdk/page.tsx

### Agent .NET SDK

Identifier: sdk-agent-dotnet
Change in 7.0.0: unchanged

Directory: sdk/platform/dotnet

Build command:

```sh
dotnet build sdk/platform/dotnet/LayerX.Sdk.csproj
```

Build output: sdk/platform/dotnet/bin and obj

Configuration: sdk/platform/dotnet/LayerX.Sdk.csproj

Ports: None listed

Health check: dotnet build sdk/platform/dotnet/LayerX.Sdk.csproj

Depends on: [Agent daemon](https://docs.paxeer.app/system-map#system-agent-daemon)

Site pages: [/sdk](https://docs.paxeer.app/sdk)

Repository references: docs/site/paxeer-docs/src/app/sdk/page.tsx

### Agent Swift SDK

Identifier: sdk-agent-swift
Change in 7.0.0: unchanged

Directory: sdk/platform/swift

Build command:

```sh
swift build --package-path sdk/platform/swift
```

Build output: sdk/platform/swift/.build

Configuration: sdk/platform/swift/Package.swift

Ports: None listed

Health check: swift test --package-path sdk/platform/swift

Depends on: [Agent daemon](https://docs.paxeer.app/system-map#system-agent-daemon)

Site pages: [/sdk](https://docs.paxeer.app/sdk)

Repository references: docs/site/paxeer-docs/src/app/sdk/page.tsx

### Agent Go SDK

Identifier: sdk-agent-go
Change in 7.0.0: unchanged

Directory: sdk/platform/go

Build command:

```sh
cd sdk/platform/go && go build ./...
```

Build output: Go package artifacts

Configuration: sdk/platform/go/go.mod

Ports: None listed

Health check: go test ./...

Depends on: [Agent daemon](https://docs.paxeer.app/system-map#system-agent-daemon)

Site pages: [/sdk](https://docs.paxeer.app/sdk)

Repository references: docs/site/paxeer-docs/src/app/sdk/page.tsx

## tool

### Release tools

Identifier: release-tools
Change in 7.0.0: changed

Directory: tools/release

Build command:

```sh
python3 tools/release/upgrade_info.py --help
```

Build output: command output and validated plan-info

Configuration: tools/release/version.json and tools/release/upgrade_info.py

Ports: None listed

Health check: python3 tools/release/upgrade_info.py --help

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: See repository references.

Repository references: docs/release/7.0.0-upgrade-plan.md, docs/release/7.0.0-validator-runbook.md

### Paxctl operator CLI

Identifier: paxctl
Change in 7.0.0: changed

Directory: tools/paxctl

Build command:

```sh
make -C tools/paxctl build
```

Build output: tools/paxctl/bin/paxctl

Configuration: tools/paxctl/README.md

Ports: None listed

Health check: paxctl --help

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/cli](https://docs.paxeer.app/cli)

Repository references: docs/site/paxeer-docs/src/app/cli/page.tsx

### LayerX developer CLI

Identifier: layerx-cli
Change in 7.0.0: changed

Directory: tools/platform/cli

Build command:

```sh
cargo build --locked --release --manifest-path tools/platform/Cargo.toml --package layerx-platform-cli
```

Build output: tools/platform/target/release/layerx

Configuration: tools/platform/cli/src/config.rs

Ports: None listed

Health check: layerx --help

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/cli](https://docs.paxeer.app/cli)

Repository references: docs/system/cli.md, docs/site/paxeer-docs/src/app/cli/page.tsx

### Wasm tools

Identifier: wasm-tools
Change in 7.0.0: unchanged

Directory: tools/wasm-tools

Build command:

```sh
cargo build --locked --release --manifest-path tools/wasm-tools/Cargo.toml
```

Build output: tools/wasm-tools/target/release binaries

Configuration: tools/wasm-tools/Cargo.toml

Ports: None listed

Health check: cargo run --manifest-path tools/wasm-tools/Cargo.toml -- --help

Depends on: [LayerX kernel host](https://docs.paxeer.app/system-map#system-kernel)

Site pages: [/wasm](https://docs.paxeer.app/wasm)

Repository references: docs/site/paxeer-docs/src/app/wasm/page.tsx

### Load test tool

Identifier: loadtest
Change in 7.0.0: unchanged

Directory: tools/loadtest

Build command:

```sh
make -C tools/loadtest build
```

Build output: tools/loadtest/bin artifacts

Configuration: tools/loadtest/config

Ports: None listed

Health check: loadtest --help

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/debugging-transactions](https://docs.paxeer.app/debugging-transactions)

Repository references: docs/site/paxeer-docs/src/app/debugging-transactions/page.tsx

### System documentation generator

Identifier: docs-generator
Change in 7.0.0: unchanged

Directory: tools/docs

Build command:

```sh
bash tools/docs/gen-system-docs.sh
```

Build output: docs/system generated pages

Configuration: tools/docs/gen-system-docs.sh

Ports: None listed

Health check: bash tools/docs/gen-system-docs.sh --check

Depends on: None

Site pages: See repository references.

Repository references: docs/system/INDEX.md

### Node bring-up tool

Identifier: bringup
Change in 7.0.0: changed

Directory: tools/bringup

Build command:

```sh
go build ./tools/bringup/...
```

Build output: tools/bringup binaries

Configuration: tools/bringup/cmd

Ports: None listed

Health check: bringup --help

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/installation](https://docs.paxeer.app/installation)

Repository references: docs/site/paxeer-docs/src/app/installation/page.tsx

### Release rollback tool

Identifier: rollback-tool
Change in 7.0.0: changed

Directory: tools/release

Build command:

```sh
bash tools/release/rollback.sh --help
```

Build output: rollback command

Configuration: tools/release/rollback.sh

Ports: None listed

Health check: bash tools/release/rollback.sh --help

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: See repository references.

Repository references: docs/release/7.0.0-upgrade-plan.md

### Backup and monitoring tools

Identifier: operations
Change in 7.0.0: changed

Directory: tools/ops

Build command:

```sh
bash tools/ops/monitoring/render.sh
```

Build output: tools/ops/monitoring/rendered configurations

Configuration: tools/ops/monitoring and tools/ops/backup

Ports: 9090, 9093

Health check: python3 tools/ops/monitoring/readiness.py --help

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/network-operations](https://docs.paxeer.app/network-operations)

Repository references: docs/release/7.0.0-validator-runbook.md, docs/site/paxeer-docs/src/app/network-operations/page.tsx

### Backup and restore tools

Identifier: backup-tools
Change in 7.0.0: unchanged

Directory: tools/ops/backup

Build command:

```sh
bash tools/ops/backup/backup.sh --help
```

Build output: backup archives and checkpoint metadata

Configuration: tools/ops/backup/backup.env.example

Ports: None listed

Health check: systemctl status paxeer-backup.timer

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/network-operations](https://docs.paxeer.app/network-operations)

Repository references: docs/release/7.0.0-validator-runbook.md, docs/site/paxeer-docs/src/app/network-operations/page.tsx

### Monitoring stack

Identifier: monitoring-tools
Change in 7.0.0: changed

Directory: tools/ops/monitoring

Build command:

```sh
bash tools/ops/monitoring/render.sh
```

Build output: rendered monitoring configuration

Configuration: tools/ops/monitoring/prometheus.yml, rules.yml and blackbox.yml

Ports: 9090, 9093

Health check: GET /-/ready (Prometheus)

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/network-operations](https://docs.paxeer.app/network-operations)

Repository references: docs/release/7.0.0-validator-runbook.md, docs/site/paxeer-docs/src/app/network-operations/page.tsx

### Paxeer load generator

Identifier: paxload
Change in 7.0.0: unchanged

Directory: tools/loadtest

Build command:

```sh
make -C tools/loadtest build
```

Build output: tools/loadtest/build/paxload

Configuration: tools/loadtest/config and profiles

Ports: None listed

Health check: paxload --help

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/operators](https://docs.paxeer.app/operators)

Repository references: docs/site/paxeer-docs/src/app/operators/page.tsx

### Cosmovisor supervisor

Identifier: cosmovisor
Change in 7.0.0: unchanged

Directory: sdk/cosmovisor

Build command:

```sh
make -C sdk/cosmovisor
```

Build output: sdk/cosmovisor/cosmovisor

Configuration: sdk/cosmovisor/README.md

Ports: None listed

Health check: cosmovisor version

Depends on: [Paxeer chain node](https://docs.paxeer.app/system-map#system-chain-node)

Site pages: [/operators](https://docs.paxeer.app/operators)

Repository references: docs/release/7.0.0-validator-runbook.md, docs/site/paxeer-docs/src/app/operators/page.tsx
