# Arbitrum Reader Modules

This directory contains modules that provide structured access to Arbitrum-specific data stored in the Blockscout database.

## Module Overview

- `api/` - API endpoint-specific functions:
  - `messages.ex` - Cross-chain message queries
  - `settlement.ex` - Batch management, DA blob data, and rollup blocks
  - `general.ex` - General utility functions like transaction log queries
- `common.ex` - Core query functionality shared between different components (API, Indexer) with configurable database selection
- `indexer/messages.ex` - Cross-chain message handling
- `indexer/parent_chain_transactions.ex` - L1 transaction lifecycle
- `indexer/settlement.ex` - Batch and state confirmation data
- `indexer/general.ex` - Chain-agnostic functions

## Important Usage Note

Functions in the `indexer/` modules should not be called directly. Instead, use the corresponding wrapper functions provided in the `Indexer.Fetcher.Arbitrum.Utils.Db.*` modules (`apps/indexer/lib/indexer/fetcher/arbitrum/utils/db/`). The wrapper functions provide:

- Additional data transformation specific to indexer needs
- Enhanced error handling

This separation ensures that database operations are properly handled and maintains a clear boundary between raw database access and indexer-specific business logic.
