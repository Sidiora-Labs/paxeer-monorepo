# Merchant checkout RPC profiles

`layerx.example.json` selects beta JSON-RPC at the public node root on EVM chain 125
(`eip155:125`). Use the root path. `npm run start:beta` selects this profile.

The emulator profile selects a loopback endpoint on port 9402.
The adapter must supply `eth_getRawTransactionByHash` and
`pax_sequencerReceipt` with native transaction bytes and verifiable signed
receipts; emulator health alone does not establish that compatibility.
`npm run start:emulator` selects this profile.

The shared merchant application passes `rpcUrl` into `TransactionReceipts`
and `SequencerReceiptClient`. It retrieves raw transaction bytes by hash,
checks their hash, then reads and verifies the sequencer receipt against those
bytes and independently pinned receipt keys. Beta uses
`LAYERX_TESTNET_RECEIPT_KEYS_JSON`; emulator uses
`LAYERX_EMULATOR_RECEIPT_KEYS_JSON`. A submitted receipt remains pending;
unreachable or mismatched evidence cannot authorize a paid order.

`settlementUrl` names a separate HTTP settlement API, not the JSON-RPC route.
Configure that field with the operator-supplied settlement service origin.
Node RPC configuration alone does not provide a settlement service.
