# Custody

Managed signing and owner self-custody operate on the same Paxeer network,
with EVM chain ID `125`. A transaction is final when its block is committed.
The application's login credential, protocol identity and transaction signing
key have distinct roles.

[Human custody](HumanCustody.md) describes managed signing, key-provider
boundaries and owner handover. [Human journeys](HumanJourneys.md) describes
the application operations and their evidence levels.

## Funding and payments

For service-managed ledger payments, use the
[Human service payment path](PaymentsQuickstart.md).
Funding an EVM wallet for gas and funding a per-asset ledger account affect
different balances; a DID binding alone does not fund either one.

Native denominations, registered assets, issuance and per-asset accounts are
documented in [Assets](Assets.md).

[Home](Home.md)
