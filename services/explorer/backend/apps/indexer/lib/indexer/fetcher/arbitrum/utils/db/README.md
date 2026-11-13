# Database Utility Modules

This directory contains modules that provide structured database access and manipulation for Arbitrum-specific data in the Blockscout indexer.

## Module Overview

- `common.ex` - Chain-agnostic database utility functions for block-related operations
- `messages.ex` - Functions for querying and managing cross-chain message data
- `parent_chain_transactions.ex` - Handles L1 transaction indexing and lifecycle management
- `settlement.ex` - Manages batch commitment and state confirmation data
- `tools.ex` - Internal helper functions for database record processing

## Usage Guidelines

1. Use logging judiciously to avoid overwhelming the logs with unnecessary information

2. Use Reader modules from `Explorer.Chain.Arbitrum.Reader` namespace for raw database access if other modules under `Explorer.Chain` do not provide the functionality you need.

3. Apply additional data transformation to maintain consistency with structures used for data import

4. Implement proper error handling when database queries return `nil`
