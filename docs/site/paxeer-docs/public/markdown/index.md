<!-- Source: https://docs.paxeer.app/ -->

PAXEER X / NETWORK DOCUMENTATION

# Applications. Payments. Programs. Build on Paxeer.

Paxeer mainnet-beta runs EVM applications and the embedded LayerX kernel on one BFT network, with native payments, markets and deterministic programs.

[Explore the network](https://docs.paxeer.app/unified-network)[Start with the XDK](https://docs.paxeer.app/xdk)

One BFT networkEVM chain ID `125`Native token `PAX`

    Paxeer mainnet-beta: EVM applications and the LayerX kernel execute on one BFT network.EVM 125 · PAX

NETWORK INTERFACES

[**XDK**Developer toolkit](https://docs.paxeer.app/xdk) [**7 SDKs**Language quickstarts](https://docs.paxeer.app/sdk-quickstarts) [**MCP**Scoped daemon tools](https://docs.paxeer.app/mcp) [**JSON-RPC**EVM chain 125](https://docs.paxeer.app/json-rpc) [**CLI**Command line](https://docs.paxeer.app/cli)

[**Execution you can reproduce**The node orders native activities and commits their state with BFT blocks. Deterministic execution supports replay with the required state and artifacts.](https://docs.paxeer.app/layerx-kernel) [**Authority with clear boundaries**Identity keys, scoped capabilities, delegated spending limits and revocation constrain authorized actions.](https://docs.paxeer.app/identity) [**Tools you already know**Deploy Solidity, connect wallets and call native precompiles through standard EVM tooling and JSON-RPC on chain 125.](https://docs.paxeer.app/evm)

APPLICATION NEEDS

- Reconcile payment outcomes

- Identify an authorized actor

- Link payment and delivery

- Reproduce program execution

- Check settlement evidence

- Connect agent tools

PAXEER X

- Native transfer settlement (402LXP)

- EVM address bound to a LayerX DID

- Escrow, acceptance and receipts linked

- Deterministic, replayable transitions

- Signed receipts and committed blocks

- Scoped MCP tools and agent budgets

[Start with the XDK](https://docs.paxeer.app/xdk)

AGENTS

### Configure agent authority.

Create identities, delegate scoped authority, fund accounts and connect tools through SDKs, MCP and the agent daemon.

This path covers:

- Agent identities and DIDs

- Budgets, mandates and revocation

- Scoped MCP tools

- Agent-to-agent tasks

 [Build an agent](https://docs.paxeer.app/agents)

PAYMENTS START HERE

### Integrate native payments.

Integrate transfers, escrow, budgets and payment streams. Connect service delivery to acceptance, receipts and disputes.

This path covers:

- Transfers, holds and escrow

- Paid APIs and x402 / 402LXP

- Payment streams and budgets

- Receipts and settlement proof

 [Make your first payment](https://docs.paxeer.app/payments-quickstart)

EVM APPLICATIONS

### Use the tools you know.

Deploy Solidity, connect wallets and call native precompiles for account binding, native payments, exchange and asset launches.

This path covers:

- Solidity with Foundry and Hardhat

- JSON-RPC on chain 125

- Native precompiles

- Assets, launchpad and markets

 [Build on the EVM](https://docs.paxeer.app/evm)

BUILD WITH THE FULL SYSTEM

## Find your interface. Follow it deeper.

Architecture, integration guides and operational reference for Paxeer X.

[### LayerX kernel

Canonical sequencing, accounting, modules and deterministic execution.](https://docs.paxeer.app/layerx-kernel) [### Platform APIs

Application gateways, hosted services, webhooks and integration surfaces.](https://docs.paxeer.app/platform-api) [### EVM JSON-RPC

Wallet integration, chain requests, contracts and transaction inspection.](https://docs.paxeer.app/json-rpc) [### Operator guides

Nodes, configuration, network services and settlement infrastructure.](https://docs.paxeer.app/operators)

What is the difference between Paxeer X and LayerX?

Paxeer mainnet-beta is one BFT network. LayerX is its embedded kernel for native activities and deterministic execution. EVM transactions call native precompiles, and receipt finality comes from the covering block commit. [Explore the network architecture](https://docs.paxeer.app/paxeer-vs-layerx)

 Which chain ID and token does the network use?

EVM chain ID `125` (`hyperpax_125-1`) with native token `PAX`. Denominations, gas limits and endpoints are listed with the network parameters. [Network parameters](https://docs.paxeer.app/network-parameters)

 How does an agent get authority to spend?

Through scoped, revocable delegation: primary and session keys, budget limits and mandates define exactly what an agent may do with money, and owners can revoke or rotate that authority. [Agent budgets](https://docs.paxeer.app/agent-budgets)

 How do I verify a payment actually settled?

Verify the signed execution receipt against trusted receipt keys, check its transaction and outcome, and confirm the covering BFT block is committed. [Receipts and proofs](https://docs.paxeer.app/receipts)

 Can I use my existing EVM tooling?

Yes. Deploy Solidity with Foundry or Hardhat, connect wallets over JSON-RPC, and call native precompiles for account binding, native payments and markets on chain 125. [Build on the EVM](https://docs.paxeer.app/evm)

 How do I run a node or operate the network?

Start with installation and configuration, then the operator guides for BFT nodes, validators, RPC endpoints and retained chain history. [Run a node](https://docs.paxeer.app/run-node)

## Build an application. Connect to Paxeer.

Choose a domain, integrate an interface and follow the evidence through the network.

[Open the developer toolkit](https://docs.paxeer.app/xdk)[Explore the network](https://docs.paxeer.app/unified-network)
