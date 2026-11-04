# The px namespace at the node root

Task 22.34.1, design for decisions 2521 and 2514. The hosted contract is
`services/platform/hosted/gateway/openrpc.json`. This document specifies future
node behaviour; it does not claim these handlers have been implemented here.

## Root routing and authority

Serve the read family on JSON-RPC HTTP POST `/`, on the existing server in
`rpc/server.go`, alongside eth, net, pax, debug, txpool, echo and test. Do not
require `/evm` or introduce a second px server. Register the px API and update
`rpc/served_methods.go` with the actual exposed methods. Reuse the shared
watermarks in `rpc/watermark_manager.go` to pin multi-read results to one
committed height. The hosted `/rpc` compatibility path remains a hosted concern;
a node advertises it only if it really serves it.

The target is 15 served reads and two deliberate refusals. Here **served** means
implemented from local chain/consensus state, configured indexer-backed state,
or the existing sequencer receipt source/explicit forwarding path. A missing
backend produces its documented unavailable error, never invented data. An
ordinary node does not need a hosted gateway upstream to read chain state.

The original inventory reported -32601 for px at the root. The checked-out
baseline already registers receiptKeysAPI under px and SequencerReceiptAPI
under pax in `rpc/server.go`; reuse those implementations in
`rpc/sequencer_receipt.go`. Their role-dependent refusal and params handling
still need contract alignment in 22.34.4. Do not remove working registrations.

Registration and starter funding are hosted service operations. For each,
validate positional parameters first (-32602 for invalid params), then have the
node return -32004, message `Hosted gateway service operation`, and data
`{code: "hosted_gateway_service_required", method: <requested method>}`. This is
a new node-specific refusal, not a change to the hosted success/error contract.
22.34.2 owns both refusal registrations as part of completing the px namespace;
they are excluded from the served-method partition below. Clients continue to
send these operations to their explicitly configured hosted gateway.

All hosted gateway behaviour remains unchanged by this design task: every
method continues to be served there, with current authentication, budgets,
validation, errors and payloads. The hosted gateway keeps serving every method
until the node serves it and the corresponding clients have passed cutover
checks. The two refused service operations remain hosted indefinitely unless a
separate owner decision supplies their service authority.

## Implementation groups

These three groups partition the served methods exactly. `node_sources` lists
existing integration and data-source files the implementer starts from; it is
not authorization to modify every data owner. Add adapters in rpc, reuse keeper,
kernel and precompile interfaces, and change an owner module only when its
public read interface actually needs extension. New rpc adapters do not exist
yet and are deliberately not presented as existing sources. Each group wires
its methods through `rpc/server.go` and `rpc/served_methods.go`.

```json
{
  "22.34.2": {
    "methods": [
      "px_resolveAccount",
      "px_getAccount",
      "px_getBalances",
      "px_listAssets",
      "px_getNetwork",
      "px_getCapabilities",
      "px_getRouteCatalogue"
    ],
    "node_sources": [
      "consensus/rpc/client/local/local.go",
      "precompiles/addr/addr.go",
      "precompiles/bank/bank.go",
      "precompiles/layerxauth/views.go",
      "precompiles/layerxledger/views.go",
      "precompiles/layerxpay/budget.go",
      "precompiles/layerxprograms/view_fees.go",
      "precompiles/setup.go",
      "rpc/association.go",
      "rpc/block.go",
      "rpc/info.go",
      "rpc/served_methods.go",
      "rpc/server.go",
      "rpc/simulate.go",
      "rpc/state.go",
      "rpc/watermark_manager.go",
      "sdk/client/tendermint.go",
      "services/platform/hosted/indexer/src/api.rs",
      "services/platform/hosted/indexer/src/store.rs",
      "src/modules/asset/lx_asset_registry.c",
      "src/modules/asset/lx_asset_state.c",
      "tools/paxeer-x/route-catalogue.json"
    ]
  },
  "22.34.3": {
    "methods": [
      "px_getHistory",
      "px_getUnifiedHistory",
      "px_listHolderRecords"
    ],
    "node_sources": [
      "precompiles/addr/addr.go",
      "precompiles/layerxauth/grants.go",
      "precompiles/layerxledger/views.go",
      "precompiles/layerxpay/budget.go",
      "rpc/association.go",
      "rpc/served_methods.go",
      "rpc/server.go",
      "rpc/state.go",
      "rpc/watermark_manager.go",
      "services/platform/hosted/indexer/src/api.rs",
      "services/platform/hosted/indexer/src/store.rs",
      "src/ledger/lx_account_read.c",
      "src/ledger/lx_account_registry.c",
      "src/modules/budget/lx_budget_create.c"
    ]
  },
  "22.34.4": {
    "methods": [
      "px_getCommit",
      "px_getValidators",
      "px_getReceiptKeys",
      "px_getSequenceBatch",
      "pax_sequencerReceipt"
    ],
    "node_sources": [
      "consensus/rpc/client/local/local.go",
      "consensus/types/tx.go",
      "modules/sequencer/keeper/keeper.go",
      "modules/sequencer/keeper/pins.go",
      "modules/sequencer/keeper/receipt_keys.go",
      "modules/sequencer/types/batch_tx.go",
      "modules/sequencer/types/batch_verify.go",
      "rpc/sequencer_receipt.go",
      "rpc/served_methods.go",
      "rpc/server.go",
      "rpc/watermark_manager.go",
      "sdk/client/tendermint.go"
    ]
  }
}
```

22.34.2 implements the seven account/discovery reads plus the two service
refusals. Account and balance reads use addr/bank/EVM keepers. Asset discovery
requires the configured indexer plus bank/ledger metadata, backed by
`src/modules/asset/lx_asset_registry.c` and
`src/modules/asset/lx_asset_state.c`. Capability probes use the EVM call path;
route definitions are not stored in the kernel.

22.34.3 implements the three indexer-backed history reads after account
resolution is available. Port the gateway cursor, envelope and merge rules;
reuse the indexer API/store rather than scanning unrestricted logs. Resolve the
unified-history discrepancy documented below before migrating native-history
consumers. Missing indexer or required native enumeration must fail explicitly.

22.34.4 implements the five consensus/receipt reads, retaining the existing
pax_sequencerReceipt name as the compatibility alias. Use local consensus
Commit/Validators/Block/Status interfaces, governed pins and sequencer types;
never point the node back at the hosted gateway to obtain its own consensus.

## Method inventory

Each section has a machine-readable node decision followed by the complete
method entry copied from OpenRPC. The contract JSON preserves parameters,
required flags, result names/schemas, descriptions and errors verbatim (only
indentation changes). Generic object schemas are intentionally not expanded
into guessed contracts; the source behaviour is explained in prose.

### px_register

```json
{
  "decision": "refused",
  "implementation_task": "22.34.2",
  "node_sources": [
    "rpc/server.go",
    "rpc/served_methods.go"
  ],
  "gateway_sources": [
    "services/platform/hosted/gateway/src/rpc_register.rs"
  ],
  "refusal": {
    "code": -32004,
    "message": "Hosted gateway service operation",
    "data": {
      "code": "hosted_gateway_service_required",
      "method": "px_register"
    }
  }
}
```

No node store owns a beta-tenant identity principal. The addr precompile binds chain identities; it does not issue tenant principals, sessions or audiences. Refuse this hosted service operation explicitly. The hosted gateway keeps signature proof of possession, derived subjects, idempotent registration, its per-minute budget and identity-backend errors unchanged.

Hosted contract:

```json
{
  "name": "px_register",
  "paramStructure": "by-position",
  "params": [
    {
      "name": "signer_public_key",
      "required": true,
      "schema": {
        "type": "string"
      }
    },
    {
      "name": "registration_signature",
      "required": true,
      "schema": {
        "type": "string"
      }
    }
  ],
  "result": {
    "name": "result",
    "schema": {
      "type": "object"
    }
  },
  "description": "Creates one self-service identity principal in the beta tenant. Both arguments are lowercase hexadecimal: a 32-byte Ed25519 signer public key and its 64-byte signature over SHA-256(len-prefixed \"layerx-register-binding-v1\", tenant, signer_public_key). The subject is derived as beta.<first 16 bytes of SHA-256(len-prefixed \"layerx-register-subject-v1\", tenant, signer_public_key)>, so registration is idempotent and cannot name another key holder's principal. The result is the identity principal record: tenant, sub, allowed_signer_public_keys, account and audiences. Malformed arguments are -32602; a signature that does not prove possession of the named key is -32002; the per-minute self-registration budget is -32005; an unconfigured or unavailable identity service is -32001 with a machine-readable data.code.",
  "errors": [
    {
      "code": -32602,
      "message": "Invalid params"
    },
    {
      "code": -32002,
      "message": "Registration refused"
    },
    {
      "code": -32001,
      "message": "Registration unavailable"
    },
    {
      "code": -32005,
      "message": "Registration unavailable"
    },
    {
      "code": -32010,
      "message": "Kernel unavailable",
      "data": {
        "type": "object",
        "required": [
          "code",
          "backend",
          "reason"
        ],
        "properties": {
          "code": {
            "const": "kernel_unavailable"
          },
          "backend": {
            "enum": [
              "core_agent_boundary",
              "public_core",
              "independent_receipt_authority",
              "identity",
              "program_registry"
            ]
          },
          "reason": {
            "enum": [
              "not_configured",
              "unreachable"
            ]
          }
        }
      }
    }
  ]
}
```

### px_getCommit

```json
{
  "decision": "served",
  "implementation_task": "22.34.4",
  "node_sources": [
    "sdk/client/tendermint.go",
    "consensus/rpc/client/local/local.go"
  ],
  "gateway_sources": [
    "services/platform/hosted/gateway/src/chain_reads.rs"
  ]
}
```

Call the local consensus client Commit at the requested height and serialize its signed header without re-encoding consensus fields into EVM quantities. Preserve decimal height, signedHeader.header and signedHeader.commit. Missing, pruned, inconsistent or unreachable consensus state returns -32003 with data.code consensus_unavailable. Hosted gateway consensus reads and public read budgeting stay identical.

Hosted contract:

```json
{
  "name": "px_getCommit",
  "paramStructure": "by-position",
  "params": [
    {
      "name": "height",
      "required": true,
      "schema": {
        "type": "string",
        "pattern": "^[0-9]*[1-9][0-9]*$",
        "description": "Positive decimal u64 block height."
      }
    }
  ],
  "result": {
    "name": "result",
    "schema": {
      "type": "object",
      "required": [
        "height",
        "signedHeader"
      ],
      "properties": {
        "height": {
          "type": "string",
          "pattern": "^[0-9]*[1-9][0-9]*$",
          "description": "Positive decimal u64 block height."
        },
        "signedHeader": {
          "type": "object",
          "required": [
            "header",
            "commit"
          ],
          "properties": {
            "header": {
              "type": "object",
              "required": [
                "height"
              ],
              "properties": {
                "height": {
                  "type": "string",
                  "pattern": "^[0-9]*[1-9][0-9]*$",
                  "description": "Positive decimal u64 block height."
                }
              }
            },
            "commit": {
              "type": "object"
            }
          }
        }
      }
    }
  },
  "description": "Reads the consensus commit at a positive decimal u64 height. Returns the decimal height and the consensus signed_header unchanged as signedHeader, including its header and commit. An unconfigured, unreachable or invalid consensus response is -32003 with data.code consensus_unavailable.",
  "errors": [
    {
      "code": -32602,
      "message": "Invalid params"
    },
    {
      "code": -32003,
      "message": "Chain read unavailable",
      "data": {
        "type": "object",
        "required": [
          "code"
        ],
        "properties": {
          "code": {
            "const": "consensus_unavailable"
          }
        }
      }
    },
    {
      "code": -32005,
      "message": "Read unavailable",
      "data": {
        "type": "object",
        "required": [
          "code"
        ],
        "properties": {
          "code": {
            "const": "public_read_rate_limit"
          }
        }
      }
    }
  ]
}
```

### px_getValidators

```json
{
  "decision": "served",
  "implementation_task": "22.34.4",
  "node_sources": [
    "sdk/client/tendermint.go",
    "consensus/rpc/client/local/local.go"
  ],
  "gateway_sources": [
    "services/platform/hosted/gateway/src/chain_reads.rs"
  ]
}
```

Call local Validators for every page at the same height, validate consistent totals and heights, and return the complete set. Preserve address, pubKey (the consensus pub_key object), decimal-string votingPower and total. Incomplete or inconsistent sets return -32003 with data.code consensus_unavailable. Hosted gateway pagination and wire conversion stay identical.

Hosted contract:

```json
{
  "name": "px_getValidators",
  "paramStructure": "by-position",
  "params": [
    {
      "name": "height",
      "required": true,
      "schema": {
        "type": "string",
        "pattern": "^[0-9]*[1-9][0-9]*$",
        "description": "Positive decimal u64 block height."
      }
    }
  ],
  "result": {
    "name": "result",
    "schema": {
      "type": "object",
      "required": [
        "height",
        "validators",
        "total"
      ],
      "properties": {
        "height": {
          "type": "string",
          "pattern": "^[0-9]*[1-9][0-9]*$",
          "description": "Positive decimal u64 block height."
        },
        "validators": {
          "type": "array",
          "items": {
            "type": "object",
            "required": [
              "address",
              "pubKey",
              "votingPower"
            ],
            "properties": {
              "address": {
                "type": "string"
              },
              "pubKey": {
                "type": "object",
                "required": [
                  "type",
                  "value"
                ],
                "properties": {
                  "type": {
                    "type": "string"
                  },
                  "value": {
                    "type": "string"
                  }
                }
              },
              "votingPower": {
                "type": "string",
                "pattern": "^[0-9]+$"
              }
            }
          }
        },
        "total": {
          "type": "string",
          "pattern": "^[0-9]+$"
        }
      }
    }
  },
  "description": "Reads the full validator set at a positive decimal u64 height, collecting all consensus pages. Returns decimal-string height, total and votingPower; pubKey preserves the consensus pub_key object. An unconfigured, unreachable or inconsistent consensus response is -32003 with data.code consensus_unavailable.",
  "errors": [
    {
      "code": -32602,
      "message": "Invalid params"
    },
    {
      "code": -32003,
      "message": "Chain read unavailable",
      "data": {
        "type": "object",
        "required": [
          "code"
        ],
        "properties": {
          "code": {
            "const": "consensus_unavailable"
          }
        }
      }
    },
    {
      "code": -32005,
      "message": "Read unavailable",
      "data": {
        "type": "object",
        "required": [
          "code"
        ],
        "properties": {
          "code": {
            "const": "public_read_rate_limit"
          }
        }
      }
    }
  ]
}
```

### px_getReceiptKeys

```json
{
  "decision": "served",
  "implementation_task": "22.34.4",
  "node_sources": [
    "rpc/sequencer_receipt.go",
    "modules/sequencer/keeper/keeper.go",
    "modules/sequencer/keeper/receipt_keys.go",
    "modules/sequencer/keeper/pins.go"
  ],
  "gateway_sources": [
    "services/platform/hosted/gateway/src/chain_reads.rs"
  ]
}
```

Reuse receiptKeysAPI.GetReceiptKeys and the governed keeper ReceiptKeys/ReceiptKeyPins; these are the same authority pins used to authenticate receipts. Preserve keyId/publicKey as 0x-prefixed 32-byte values and inclusive firstHeight/lastHeight as JSON integers. Require an explicit empty positional params array. Map an unavailable authority configuration to -32003 with data.code receipt_keys_unavailable. An explicitly configured empty list is distinct from missing configuration. The baseline handler currently depends on local-source/forwarding options; 22.34.4 must make ordinary nodes read local governed pins and align its refusal with this contract. Hosted gateway configured pins and unavailable error remain unchanged until an explicitly verified pin-source cutover; never silently trust a key embedded in a receipt.

Hosted contract:

```json
{
  "name": "px_getReceiptKeys",
  "paramStructure": "by-position",
  "params": [],
  "result": {
    "name": "result",
    "schema": {
      "type": "object",
      "required": [
        "keys"
      ],
      "properties": {
        "keys": {
          "type": "array",
          "items": {
            "type": "object",
            "required": [
              "keyId",
              "publicKey",
              "firstHeight",
              "lastHeight"
            ],
            "properties": {
              "keyId": {
                "type": "string",
                "pattern": "^0x[0-9a-fA-F]{64}$"
              },
              "publicKey": {
                "type": "string",
                "pattern": "^0x[0-9a-fA-F]{64}$"
              },
              "firstHeight": {
                "type": "integer",
                "minimum": 0,
                "maximum": 18446744073709551615
              },
              "lastHeight": {
                "type": "integer",
                "minimum": 0,
                "maximum": 18446744073709551615
              }
            }
          }
        }
      }
    }
  },
  "description": "Publishes the configured pinned receipt signing keys and their inclusive height ranges. keyId and publicKey are 0x-prefixed 32-byte hexadecimal strings; firstHeight and lastHeight are JSON integers. Requires an explicit empty positional params array. An unconfigured list is -32003 with data.code receipt_keys_unavailable.",
  "errors": [
    {
      "code": -32602,
      "message": "Invalid params"
    },
    {
      "code": -32003,
      "message": "Chain read unavailable",
      "data": {
        "type": "object",
        "required": [
          "code"
        ],
        "properties": {
          "code": {
            "const": "receipt_keys_unavailable"
          }
        }
      }
    },
    {
      "code": -32005,
      "message": "Read unavailable",
      "data": {
        "type": "object",
        "required": [
          "code"
        ],
        "properties": {
          "code": {
            "const": "public_read_rate_limit"
          }
        }
      }
    }
  ]
}
```

### px_getSequenceBatch

```json
{
  "decision": "served",
  "implementation_task": "22.34.4",
  "node_sources": [
    "sdk/client/tendermint.go",
    "consensus/rpc/client/local/local.go",
    "consensus/types/tx.go",
    "modules/sequencer/types/batch_tx.go",
    "modules/sequencer/types/batch_verify.go"
  ],
  "gateway_sources": [
    "services/platform/hosted/gateway/src/chain_reads.rs"
  ]
}
```

Read transaction zero of the local consensus Block, recognize and validate MsgSequenceBatch with the sequencer types, construct its consensus transaction Merkle proof, and verify against header.data_hash before serving. Do not confuse the batch activity TxRoot with the consensus transaction proof. Preserve 0x transaction bytes, total, index=0, leafHash and aunts. No batch returns -32602/data.code no_sequence_batch; mismatched proof returns -32003/data.code invalid_consensus_response; unavailable consensus returns -32003/data.code consensus_unavailable. Hosted proof verification and wire representation stay identical.

Hosted contract:

```json
{
  "name": "px_getSequenceBatch",
  "paramStructure": "by-position",
  "params": [
    {
      "name": "height",
      "required": true,
      "schema": {
        "type": "string",
        "pattern": "^[0-9]*[1-9][0-9]*$",
        "description": "Positive decimal u64 block height."
      }
    }
  ],
  "result": {
    "name": "result",
    "schema": {
      "type": "object",
      "required": [
        "height",
        "tx",
        "proof"
      ],
      "properties": {
        "height": {
          "type": "string",
          "pattern": "^[0-9]*[1-9][0-9]*$",
          "description": "Positive decimal u64 block height."
        },
        "tx": {
          "type": "string",
          "pattern": "^0x([0-9a-fA-F]{2})*$"
        },
        "proof": {
          "type": "object",
          "required": [
            "total",
            "index",
            "leafHash",
            "aunts"
          ],
          "properties": {
            "total": {
              "type": "integer",
              "minimum": 1
            },
            "index": {
              "type": "integer",
              "const": 0
            },
            "leafHash": {
              "type": "string",
              "pattern": "^0x[0-9a-fA-F]{64}$"
            },
            "aunts": {
              "type": "array",
              "items": {
                "type": "string",
                "pattern": "^0x[0-9a-fA-F]{64}$"
              }
            }
          }
        }
      }
    }
  },
  "description": "Reads transaction 0 of the consensus block at a positive decimal u64 height. tx is the 0x-hex encoded MsgSequenceBatch transaction; proof contains total, index, leafHash and aunts for its Merkle inclusion under header.data_hash. The proof is verified before a result is served. A block without a MsgSequenceBatch at transaction 0 is -32602 with data.code no_sequence_batch; a proof mismatch is -32003 with data.code invalid_consensus_response.",
  "errors": [
    {
      "code": -32602,
      "message": "Invalid params"
    },
    {
      "code": -32602,
      "message": "Invalid params",
      "data": {
        "type": "object",
        "required": [
          "code"
        ],
        "properties": {
          "code": {
            "const": "no_sequence_batch"
          }
        }
      }
    },
    {
      "code": -32003,
      "message": "Chain read unavailable",
      "data": {
        "type": "object",
        "required": [
          "code"
        ],
        "properties": {
          "code": {
            "const": "consensus_unavailable"
          }
        }
      }
    },
    {
      "code": -32003,
      "message": "Chain read unavailable",
      "data": {
        "type": "object",
        "required": [
          "code"
        ],
        "properties": {
          "code": {
            "const": "invalid_consensus_response"
          }
        }
      }
    },
    {
      "code": -32005,
      "message": "Read unavailable",
      "data": {
        "type": "object",
        "required": [
          "code"
        ],
        "properties": {
          "code": {
            "const": "public_read_rate_limit"
          }
        }
      }
    }
  ]
}
```

### px_requestFunds

```json
{
  "decision": "refused",
  "implementation_task": "22.34.2",
  "node_sources": [
    "rpc/server.go",
    "rpc/served_methods.go"
  ],
  "gateway_sources": [
    "services/platform/hosted/gateway/src/rpc_faucet.rs"
  ],
  "refusal": {
    "code": -32004,
    "message": "Hosted gateway service operation",
    "data": {
      "code": "hosted_gateway_service_required",
      "method": "px_requestFunds"
    }
  }
}
```

The node has chain balances and transaction receipts, but no authenticated identity session, starter-credit grant, faucet signer or tenant idempotency store. Refuse this hosted service operation explicitly. This decision does not create a node faucet or submit a transaction. The hosted gateway keeps bearer authorisation of the bound DID/signer, principal/grant idempotency, settled funding evidence, amount matching, pending-claim errors and identity-backend errors unchanged.

Hosted contract:

```json
{
  "name": "px_requestFunds",
  "paramStructure": "by-position",
  "params": [
    {
      "name": "did",
      "required": true,
      "schema": {
        "type": "string",
        "pattern": "^did:[0-9A-Za-z._:-]+$",
        "maxLength": 512
      }
    },
    {
      "name": "signer_public_key",
      "required": true,
      "schema": {
        "type": "string",
        "pattern": "^[0-9a-f]{64}$"
      }
    }
  ],
  "result": {
    "name": "claim",
    "schema": {
      "type": "object",
      "required": [
        "funded",
        "funding_id",
        "transaction_id",
        "amount",
        "network",
        "starter_credit"
      ],
      "properties": {
        "funded": {
          "type": "boolean",
          "const": true
        },
        "funding_id": {
          "type": "string",
          "pattern": "^[0-9a-fA-F]+$",
          "maxLength": 128
        },
        "transaction_id": {
          "type": "string",
          "minLength": 1,
          "maxLength": 128
        },
        "amount": {
          "type": "string",
          "pattern": "^[0-9]+$",
          "maxLength": 39
        },
        "network": {
          "type": "string",
          "minLength": 1,
          "maxLength": 64
        },
        "starter_credit": {
          "type": "object"
        }
      }
    }
  },
  "description": "Claims the authenticated identity principal's starter credit through the faucet claim path. Params are a DID and its bound, nonzero 32-byte lowercase hexadecimal Ed25519 signer public key without a 0x prefix. The bearer session must authorise the signer. The result includes funded, funding_id, transaction_id, decimal-string amount, network and the settled starter_credit record. Idempotency is bound to the principal and starter-credit grant, so retries reuse the grant. Incomplete funding evidence or an amount mismatch is -32603; pending claims are -32001 with data.code faucet_claim_pending. An unavailable identity backend is -32010 Kernel unavailable.",
  "errors": [
    {
      "code": -32602,
      "message": "Invalid params"
    },
    {
      "code": -32002,
      "message": "Faucet claim refused"
    },
    {
      "code": -32001,
      "message": "Faucet claim unavailable"
    },
    {
      "code": -32005,
      "message": "Faucet claim unavailable"
    },
    {
      "code": -32010,
      "message": "Kernel unavailable",
      "data": {
        "type": "object",
        "required": [
          "code",
          "backend",
          "reason"
        ],
        "properties": {
          "code": {
            "const": "kernel_unavailable"
          },
          "backend": {
            "enum": [
              "core_agent_boundary",
              "public_core",
              "independent_receipt_authority",
              "identity",
              "program_registry"
            ]
          },
          "reason": {
            "enum": [
              "not_configured",
              "unreachable"
            ]
          }
        }
      }
    },
    {
      "code": -32603,
      "message": "Faucet claim evidence incomplete"
    }
  ]
}
```

### pax_sequencerReceipt

```json
{
  "decision": "served",
  "implementation_task": "22.34.4",
  "node_sources": [
    "rpc/sequencer_receipt.go",
    "modules/sequencer/keeper/keeper.go",
    "modules/sequencer/keeper/receipt_keys.go"
  ],
  "gateway_sources": [
    "services/platform/hosted/gateway/src/paxeer.rs"
  ]
}
```

Reuse SequencerReceiptAPI.SequencerReceipt and its local sequencer send record, governed receipt pins and committed keeper LastSeq/LastSeqHash. This existing pax_ compatibility method belongs to the px design inventory, with its exact name retained; do not invent a px_sequencerReceipt wire method. Unknown hashes return null. Keep submitted/receipted/committed rungs, hexadecimal quantities, decoded signed outputs and covering block coordinates. A node without the sequencer send record may use its explicitly configured sequencer forwarding path, otherwise returns the existing -32000 "pax_sequencerReceipt served by sequencer"; never synthesize a send record from a committed transaction. The hosted gateway continues relaying this node response unchanged.

Hosted contract:

```json
{
  "name": "pax_sequencerReceipt",
  "paramStructure": "by-position",
  "params": [
    {
      "name": "txHash",
      "required": true,
      "schema": {
        "type": "string",
        "pattern": "^0x[0-9a-fA-F]{64}$"
      }
    }
  ],
  "result": {
    "name": "result",
    "schema": {
      "oneOf": [
        {
          "type": "null"
        },
        {
          "type": "object",
          "required": [
            "rung",
            "txHash",
            "nonce"
          ],
          "properties": {
            "rung": {
              "enum": [
                "submitted",
                "receipted",
                "committed"
              ]
            },
            "txHash": {
              "type": "string",
              "pattern": "^0x[0-9a-fA-F]{64}$"
            },
            "nonce": {
              "type": "string",
              "pattern": "^0x(0|[1-9a-fA-F][0-9a-fA-F]*)$"
            },
            "seq": {
              "type": "string",
              "pattern": "^0x(0|[1-9a-fA-F][0-9a-fA-F]*)$"
            },
            "height": {
              "type": "string",
              "pattern": "^0x(0|[1-9a-fA-F][0-9a-fA-F]*)$"
            },
            "index": {
              "type": "string",
              "pattern": "^0x(0|[1-9a-fA-F][0-9a-fA-F]*)$"
            },
            "timestamp": {
              "type": "string",
              "pattern": "^0x(0|[1-9a-fA-F][0-9a-fA-F]*)$"
            },
            "status": {
              "type": "string",
              "enum": [
                "0x0",
                "0x1"
              ]
            },
            "returnData": {
              "type": "string",
              "pattern": "^0x([0-9a-fA-F]{2})*$"
            },
            "logs": {
              "type": "array",
              "items": {
                "type": "object",
                "required": [
                  "address",
                  "topics",
                  "data",
                  "logIndex"
                ],
                "properties": {
                  "address": {
                    "type": "string",
                    "pattern": "^0x[0-9a-fA-F]{40}$"
                  },
                  "topics": {
                    "type": "array",
                    "items": {
                      "type": "string",
                      "pattern": "^0x[0-9a-fA-F]{64}$"
                    },
                    "maxItems": 4
                  },
                  "data": {
                    "type": "string",
                    "pattern": "^0x([0-9a-fA-F]{2})*$"
                  },
                  "logIndex": {
                    "type": "string",
                    "pattern": "^0x(0|[1-9a-fA-F][0-9a-fA-F]*)$"
                  }
                }
              }
            },
            "gasUsed": {
              "type": "string",
              "pattern": "^0x(0|[1-9a-fA-F][0-9a-fA-F]*)$"
            },
            "cumulativeGasUsed": {
              "type": "string",
              "pattern": "^0x(0|[1-9a-fA-F][0-9a-fA-F]*)$"
            },
            "effectiveGasPrice": {
              "type": "string",
              "pattern": "^0x(0|[1-9a-fA-F][0-9a-fA-F]*)$"
            },
            "feeDenom": {
              "type": "string"
            },
            "feeAmount": {
              "type": "string",
              "pattern": "^0x(0|[1-9a-fA-F][0-9a-fA-F]*)$"
            },
            "sponsor": {
              "type": "string",
              "pattern": "^0x[0-9a-fA-F]{40}$"
            },
            "from": {
              "type": "string",
              "pattern": "^0x[0-9a-fA-F]{40}$"
            },
            "to": {
              "type": "string",
              "pattern": "^0x[0-9a-fA-F]{40}$"
            },
            "contractAddress": {
              "type": "string",
              "pattern": "^0x[0-9a-fA-F]{40}$"
            },
            "vmError": {
              "type": "string"
            },
            "keyId": {
              "type": "string",
              "pattern": "^0x[0-9a-fA-F]{64}$"
            },
            "signingKey": {
              "type": "string",
              "pattern": "^0x[0-9a-fA-F]{64}$"
            },
            "receipt": {
              "type": "string",
              "pattern": "^0x([0-9a-fA-F]{2})*$"
            },
            "blockNumber": {
              "type": "string",
              "pattern": "^0x(0|[1-9a-fA-F][0-9a-fA-F]*)$"
            },
            "blockHash": {
              "type": "string",
              "pattern": "^0x[0-9a-fA-F]{64}$"
            }
          },
          "oneOf": [
            {
              "properties": {
                "rung": {
                  "const": "submitted"
                }
              },
              "not": {
                "anyOf": [
                  {
                    "required": [
                      "seq"
                    ]
                  },
                  {
                    "required": [
                      "height"
                    ]
                  },
                  {
                    "required": [
                      "index"
                    ]
                  },
                  {
                    "required": [
                      "timestamp"
                    ]
                  },
                  {
                    "required": [
                      "status"
                    ]
                  },
                  {
                    "required": [
                      "returnData"
                    ]
                  },
                  {
                    "required": [
                      "logs"
                    ]
                  },
                  {
                    "required": [
                      "gasUsed"
                    ]
                  },
                  {
                    "required": [
                      "cumulativeGasUsed"
                    ]
                  },
                  {
                    "required": [
                      "effectiveGasPrice"
                    ]
                  },
                  {
                    "required": [
                      "feeDenom"
                    ]
                  },
                  {
                    "required": [
                      "feeAmount"
                    ]
                  },
                  {
                    "required": [
                      "sponsor"
                    ]
                  },
                  {
                    "required": [
                      "from"
                    ]
                  },
                  {
                    "required": [
                      "to"
                    ]
                  },
                  {
                    "required": [
                      "contractAddress"
                    ]
                  },
                  {
                    "required": [
                      "vmError"
                    ]
                  },
                  {
                    "required": [
                      "keyId"
                    ]
                  },
                  {
                    "required": [
                      "signingKey"
                    ]
                  },
                  {
                    "required": [
                      "receipt"
                    ]
                  },
                  {
                    "required": [
                      "blockNumber"
                    ]
                  },
                  {
                    "required": [
                      "blockHash"
                    ]
                  }
                ]
              }
            },
            {
              "properties": {
                "rung": {
                  "const": "receipted"
                }
              },
              "required": [
                "rung",
                "txHash",
                "nonce",
                "seq",
                "height",
                "index",
                "timestamp",
                "status",
                "returnData",
                "logs",
                "gasUsed",
                "cumulativeGasUsed",
                "effectiveGasPrice",
                "feeDenom",
                "feeAmount",
                "sponsor",
                "from",
                "to",
                "contractAddress",
                "vmError",
                "keyId",
                "signingKey",
                "receipt"
              ],
              "not": {
                "anyOf": [
                  {
                    "required": [
                      "blockNumber"
                    ]
                  },
                  {
                    "required": [
                      "blockHash"
                    ]
                  }
                ]
              }
            },
            {
              "properties": {
                "rung": {
                  "const": "committed"
                }
              },
              "required": [
                "rung",
                "txHash",
                "nonce",
                "seq",
                "height",
                "index",
                "timestamp",
                "status",
                "returnData",
                "logs",
                "gasUsed",
                "cumulativeGasUsed",
                "effectiveGasPrice",
                "feeDenom",
                "feeAmount",
                "sponsor",
                "from",
                "to",
                "contractAddress",
                "vmError",
                "keyId",
                "signingKey",
                "receipt",
                "blockNumber",
                "blockHash"
              ]
            }
          ]
        }
      ]
    }
  },
  "description": "Relays the node response unchanged for one 0x-prefixed 32-byte transaction hash. Unknown hashes return null. submitted carries rung, txHash and nonce; receipted adds the decoded signed receipt, signing key and execution outputs; committed also includes blockNumber and blockHash for the covering MsgSequenceBatch. Quantities are 0x-prefixed hexadecimal strings. Receipt consistency and signature checks do not establish signing authority without pinned receipt keys. The node may refuse with -32000 when the method is served only by a sequencer.",
  "errors": [
    {
      "code": -32602,
      "message": "Invalid params"
    },
    {
      "code": -32000,
      "message": "pax_sequencerReceipt served by sequencer"
    },
    {
      "code": -32001,
      "message": "Paxeer read unavailable"
    }
  ]
}
```

### px_resolveAccount

```json
{
  "decision": "served",
  "implementation_task": "22.34.2",
  "node_sources": [
    "precompiles/addr/addr.go",
    "rpc/association.go",
    "rpc/state.go"
  ],
  "gateway_sources": [
    "services/platform/hosted/gateway/src/paxeer.rs"
  ]
}
```

Read addr 0x1004 getUnifiedAccount/getEvmAddrByLayerX via the local EVM state, checking both directions and the derived main account. Return evm_address, pax_address, layerx_did, layerx_account and bound; absent halves are null. Keep the original EVM/DID selector. A bare native account key is not a DID: preserve -32001/data.code native_account_reverse_resolution_unsupported; conflicting joins preserve conflicting_identity_binding. No hosted identity service is needed. Hosted selector validation, resolution joins and typed refusals stay identical.

Hosted contract:

```json
{
  "name": "px_resolveAccount",
  "paramStructure": "by-position",
  "params": [
    {
      "name": "account",
      "required": true,
      "schema": {
        "type": "string",
        "pattern": "^(0x[0-9a-fA-F]{40}|did:layerx:[0-9a-fA-F]{64})$"
      },
      "description": "EVM or explicit DID lookup selector retained from the original lookup; layerx_account is a separate native reference and cannot be used for reverse resolution."
    }
  ],
  "result": {
    "name": "result",
    "schema": {
      "type": "object"
    }
  },
  "description": "Resolves one account across both domains. The argument is either a 0x-prefixed 20-byte EVM address or an explicit LayerX DID (did:layerx:<64 hex>). The result is {evm_address, pax_address, layerx_did, layerx_account, bound}: every field comes from the addr precompile's getUnifiedAccount/getEvmAddrByLayerX, the halves that are not bound are null, and bound is false when the EVM address carries no LayerX DID. A bare 64-hex native account identifier is refused with -32001 and data.code=native_account_reverse_resolution_unsupported; it is never reinterpreted as a DID public key. Retain the original EVM/DID lookup selector for repeated account, balance and history reads; layerx_account is a separate native reference. Conflicting forward/reverse joins refuse with data.code=conflicting_identity_binding. An unconfigured Paxeer endpoint is -32001 with data.code=paxeer_rpc_not_configured.",
  "errors": [
    {
      "code": -32602,
      "message": "Invalid params"
    },
    {
      "code": -32001,
      "message": "Paxeer read unavailable"
    },
    {
      "code": -32005,
      "message": "Read unavailable"
    }
  ]
}
```

### px_getAccount

```json
{
  "decision": "served",
  "implementation_task": "22.34.2",
  "node_sources": [
    "precompiles/addr/addr.go",
    "rpc/association.go",
    "rpc/state.go",
    "rpc/block.go"
  ],
  "gateway_sources": [
    "services/platform/hosted/gateway/src/paxeer.rs"
  ]
}
```

Select one local committed EVM height using the existing watermark resolver, resolve at that height and read the same StateAPI balance and nonce paths used by eth_getBalance/eth_getTransactionCount. Return account and paxeer {address,balance,nonce,height}, with hexadecimal balance/nonce and integer height; paxeer is null for an unbound DID. Do not add a kernel-backend dependency. Hosted snapshot consistency, unbound behaviour and result shape stay identical.

Hosted contract:

```json
{
  "name": "px_getAccount",
  "paramStructure": "by-position",
  "params": [
    {
      "name": "account",
      "required": true,
      "schema": {
        "type": "string",
        "pattern": "^(0x[0-9a-fA-F]{40}|did:layerx:[0-9a-fA-F]{64})$"
      },
      "description": "EVM or explicit DID lookup selector retained from the original lookup; layerx_account is a separate native reference and cannot be used for reverse resolution."
    }
  ],
  "result": {
    "name": "result",
    "schema": {
      "type": "object",
      "required": [
        "account",
        "paxeer"
      ],
      "properties": {
        "account": {
          "type": "object",
          "required": [
            "evm_address",
            "pax_address",
            "layerx_did",
            "layerx_account",
            "bound"
          ],
          "properties": {
            "evm_address": {
              "type": [
                "string",
                "null"
              ]
            },
            "pax_address": {
              "type": [
                "string",
                "null"
              ]
            },
            "layerx_did": {
              "type": [
                "string",
                "null"
              ]
            },
            "layerx_account": {
              "type": [
                "string",
                "null"
              ]
            },
            "bound": {
              "type": "boolean"
            }
          },
          "additionalProperties": false
        },
        "paxeer": {
          "oneOf": [
            {
              "type": "null"
            },
            {
              "type": "object",
              "required": [
                "address",
                "balance",
                "nonce",
                "height"
              ],
              "properties": {
                "address": {
                  "type": "string",
                  "pattern": "^0x[0-9a-fA-F]{40}$"
                },
                "balance": {
                  "type": "string",
                  "pattern": "^0x[0-9a-fA-F]+$"
                },
                "nonce": {
                  "type": "string",
                  "pattern": "^0x[0-9a-fA-F]+$"
                },
                "height": {
                  "type": "integer",
                  "minimum": 0
                }
              },
              "additionalProperties": false
            }
          ]
        }
      },
      "additionalProperties": false
    }
  },
  "description": "Reads account resolution, EVM balance and nonce from chain state at one height returned by eth_blockNumber. The result is {account, paxeer:{address, balance, nonce, height}}; paxeer is null for an account without an EVM binding. No kernel backend is required.",
  "errors": [
    {
      "code": -32602,
      "message": "Invalid params"
    },
    {
      "code": -32001,
      "message": "Paxeer read unavailable"
    },
    {
      "code": -32005,
      "message": "Read unavailable"
    }
  ]
}
```

### px_getBalances

```json
{
  "decision": "served",
  "implementation_task": "22.34.2",
  "node_sources": [
    "precompiles/addr/addr.go",
    "rpc/association.go",
    "rpc/state.go",
    "precompiles/bank/bank.go"
  ],
  "gateway_sources": [
    "services/platform/hosted/gateway/src/paxeer.rs"
  ]
}
```

Read bank 0x1001 all_balances(address) at the same local height as account resolution. The bank keeper owns the coin list; do not substitute kernel asset balances or ERC20 token discovery. Preserve exact 256-bit decimal amounts, denoms, account and integer height. An unbound DID cannot have an EVM coin list and returns -32001/data.code identity_binding_unavailable. Hosted pinning and result/error behaviour stay identical.

Hosted contract:

```json
{
  "name": "px_getBalances",
  "paramStructure": "by-position",
  "params": [
    {
      "name": "account",
      "required": true,
      "schema": {
        "type": "string",
        "pattern": "^(0x[0-9a-fA-F]{40}|did:layerx:[0-9a-fA-F]{64})$"
      },
      "description": "EVM or explicit DID lookup selector retained from the original lookup; layerx_account is a separate native reference and cannot be used for reverse resolution."
    }
  ],
  "result": {
    "name": "result",
    "schema": {
      "type": "object",
      "required": [
        "account",
        "height",
        "balances"
      ],
      "properties": {
        "account": {
          "type": "object",
          "required": [
            "evm_address",
            "pax_address",
            "layerx_did",
            "layerx_account",
            "bound"
          ],
          "properties": {
            "evm_address": {
              "type": [
                "string",
                "null"
              ]
            },
            "pax_address": {
              "type": [
                "string",
                "null"
              ]
            },
            "layerx_did": {
              "type": [
                "string",
                "null"
              ]
            },
            "layerx_account": {
              "type": [
                "string",
                "null"
              ]
            },
            "bound": {
              "type": "boolean"
            }
          },
          "additionalProperties": false
        },
        "height": {
          "type": "integer",
          "minimum": 0
        },
        "balances": {
          "type": "array",
          "items": {
            "type": "object",
            "required": [
              "denom",
              "amount"
            ],
            "properties": {
              "denom": {
                "type": "string"
              },
              "amount": {
                "type": "string",
                "pattern": "^[0-9]+$"
              }
            },
            "additionalProperties": false
          }
        }
      },
      "additionalProperties": false
    }
  },
  "description": "Reads the bound EVM address coin list from bank all_balances(address), with account resolution and every eth_call pinned to one height returned by eth_blockNumber. The result is {account, height, balances:[{denom, amount}]}; amounts are exact decimal strings of on-chain 256-bit values. No kernel backend is required.",
  "errors": [
    {
      "code": -32602,
      "message": "Invalid params"
    },
    {
      "code": -32001,
      "message": "Paxeer read unavailable"
    },
    {
      "code": -32005,
      "message": "Read unavailable"
    }
  ]
}
```

### px_listAssets

```json
{
  "decision": "served",
  "implementation_task": "22.34.2",
  "node_sources": [
    "services/platform/hosted/indexer/src/api.rs",
    "services/platform/hosted/indexer/src/store.rs",
    "precompiles/bank/bank.go",
    "precompiles/layerxledger/views.go",
    "src/modules/asset/lx_asset_registry.c",
    "src/modules/asset/lx_asset_state.c"
  ],
  "gateway_sources": [
    "services/platform/hosted/gateway/src/paxeer.rs"
  ]
}
```

Use indexer /v1/assets as the enumeration source; sort by asset_id and cap the joined result at 16. Read bank 0x1001 metadata for denoms, or ledger 0x101a getAsset backed by the kernel asset registry/state, all at the returned local height. Preserve optional paused and verification=authenticated_committed_snapshot only for successful ledger reads. Unsupported/failed per-asset metadata reads leave name/symbol/decimals/supply null and omit those optional fields, as explicitly specified by OpenRPC; a missing indexer fails the operation rather than returning an empty registry. This is a configured indexer-backed read, not a new chain asset enumeration API. Hosted joins, limits, null semantics and exact decimal supply stay identical.

Hosted contract:

```json
{
  "name": "px_listAssets",
  "paramStructure": "by-position",
  "params": [],
  "result": {
    "name": "result",
    "schema": {
      "type": "object",
      "required": [
        "height",
        "assets",
        "joined_limit"
      ],
      "additionalProperties": false,
      "properties": {
        "height": {
          "type": "integer",
          "minimum": 0
        },
        "assets": {
          "type": "array",
          "maxItems": 16,
          "items": {
            "type": "object",
            "required": [
              "asset_id",
              "denom",
              "kind",
              "name",
              "symbol",
              "decimals",
              "supply"
            ],
            "additionalProperties": false,
            "properties": {
              "asset_id": {
                "type": "string"
              },
              "denom": {
                "type": [
                  "string",
                  "null"
                ]
              },
              "kind": {
                "type": "string"
              },
              "name": {
                "type": [
                  "string",
                  "null"
                ]
              },
              "symbol": {
                "type": [
                  "string",
                  "null"
                ]
              },
              "decimals": {
                "type": [
                  "integer",
                  "null"
                ],
                "minimum": 0,
                "maximum": 255
              },
              "supply": {
                "type": [
                  "string",
                  "null"
                ],
                "pattern": "^[0-9]+$"
              },
              "paused": {
                "type": "boolean"
              },
              "verification": {
                "type": "string",
                "enum": [
                  "authenticated_committed_snapshot"
                ]
              }
            }
          }
        },
        "joined_limit": {
          "type": "integer",
          "const": 16
        }
      }
    }
  },
  "description": "Lists indexer assets sorted by asset_id, limited to 16. Bank denoms read name, symbol, decimals and supply from bank 0x1001; ledger assets read getAsset at 0x101A. Successful ledger reads also include paused (boolean) and verification (authenticated_committed_snapshot). All chain reads use the returned height. A failed or unsupported asset read leaves name, symbol, decimals and supply null and omits paused and verification. Supply is an integer in base units rendered as a decimal string. Takes no arguments.",
  "errors": [
    {
      "code": -32602,
      "message": "Invalid params"
    },
    {
      "code": -32001,
      "message": "Paxeer read unavailable"
    },
    {
      "code": -32005,
      "message": "Read unavailable"
    },
    {
      "code": -32010,
      "message": "Kernel unavailable",
      "data": {
        "type": "object",
        "required": [
          "code",
          "backend",
          "reason"
        ],
        "properties": {
          "code": {
            "const": "kernel_unavailable"
          },
          "backend": {
            "enum": [
              "core_agent_boundary",
              "public_core",
              "independent_receipt_authority",
              "identity",
              "program_registry"
            ]
          },
          "reason": {
            "enum": [
              "not_configured",
              "unreachable"
            ]
          }
        }
      }
    }
  ]
}
```

### px_getNetwork

```json
{
  "decision": "served",
  "implementation_task": "22.34.2",
  "node_sources": [
    "rpc/info.go",
    "rpc/block.go",
    "sdk/client/tendermint.go",
    "consensus/rpc/client/local/local.go"
  ],
  "gateway_sources": [
    "services/platform/hosted/gateway/src/paxeer.rs",
    "services/platform/hosted/gateway/src/chain_reads.rs"
  ]
}
```

Read local chain configuration, BlockNumber and consensus Status. Preserve network_id, hexadecimal paxeer.chain_id/latest_block, decimal consensus.latest_height, consensus hash encoding and catching_up. If consensus status is unavailable, return consensus=null and reason=consensus_unavailable while retaining the available EVM head. Do not advertise gateway environment defaults as node identity. Hosted optional consensus behaviour and encoding stay identical.

Hosted contract:

```json
{
  "name": "px_getNetwork",
  "paramStructure": "by-position",
  "params": [],
  "result": {
    "name": "result",
    "schema": {
      "type": "object",
      "required": [
        "network_id",
        "paxeer",
        "consensus"
      ],
      "additionalProperties": false,
      "properties": {
        "network_id": {
          "type": "string"
        },
        "paxeer": {
          "type": "object",
          "required": [
            "chain_id",
            "latest_block"
          ],
          "additionalProperties": false,
          "properties": {
            "chain_id": {
              "type": "string",
              "pattern": "^0x[0-9a-fA-F]+$"
            },
            "latest_block": {
              "type": "string",
              "pattern": "^0x[0-9a-fA-F]+$"
            }
          }
        },
        "consensus": {
          "type": [
            "object",
            "null"
          ],
          "required": [
            "latest_height",
            "latest_block_hash",
            "latest_app_hash",
            "catching_up"
          ],
          "additionalProperties": false,
          "properties": {
            "latest_height": {
              "type": "string",
              "pattern": "^[0-9]+$"
            },
            "latest_block_hash": {
              "type": "string",
              "pattern": "^([0-9a-fA-F]{2})*$"
            },
            "latest_app_hash": {
              "type": "string",
              "pattern": "^([0-9a-fA-F]{2})*$"
            },
            "catching_up": {
              "type": "boolean"
            }
          }
        },
        "reason": {
          "type": "string",
          "enum": [
            "consensus_unavailable"
          ]
        }
      },
      "oneOf": [
        {
          "properties": {
            "consensus": {
              "type": "object"
            }
          },
          "not": {
            "required": [
              "reason"
            ]
          }
        },
        {
          "required": [
            "reason"
          ],
          "properties": {
            "consensus": {
              "type": "null"
            }
          }
        }
      ]
    }
  },
  "description": "Network head: network_id and paxeer {chain_id, latest_block} from eth_chainId and eth_blockNumber, beside consensus {latest_height, latest_block_hash, latest_app_hash, catching_up} from the consensus status method. latest_height is a decimal string; hashes retain the consensus hexadecimal encoding. When the consensus endpoint is unset, unreachable, or returns an invalid response, consensus is null and reason is consensus_unavailable; the Paxeer chain ID and latest block still answer. Takes no arguments.",
  "errors": [
    {
      "code": -32602,
      "message": "Invalid params"
    },
    {
      "code": -32001,
      "message": "Paxeer read unavailable"
    },
    {
      "code": -32005,
      "message": "Read unavailable"
    }
  ]
}
```

### px_getHistory

```json
{
  "decision": "served",
  "implementation_task": "22.34.3",
  "node_sources": [
    "services/platform/hosted/indexer/src/api.rs",
    "services/platform/hosted/indexer/src/store.rs",
    "precompiles/addr/addr.go",
    "rpc/association.go",
    "rpc/state.go"
  ],
  "gateway_sources": [
    "services/platform/hosted/gateway/src/history.rs",
    "services/platform/hosted/gateway/src/paxeer.rs"
  ]
}
```

Resolve an EVM/DID selector locally via addr 0x1004, then page configured indexer /v1/history/{account}. The indexer store owns ordered history, joined asset_metadata and finality flags; the node block store alone cannot replace it. Preserve newest-first (blockHeight,txIndex,logIndex) ordering, chain tags, metadata, exact amounts and 32-lowercase-hex position cursors; reject 112-hex cursors and unbound DID selectors with -32602. Return indexedHeight, local chainHeight and indexer /readyz freshness without treating index freshness as committed finality. Missing indexer returns -32001/data.code indexer_not_configured, not an empty successful page. Hosted history validation, cursor paging and freshness envelope stay identical.

Hosted contract:

```json
{
  "name": "px_getHistory",
  "paramStructure": "by-position",
  "params": [
    {
      "name": "account",
      "required": true,
      "schema": {
        "type": "string",
        "pattern": "^(0x[0-9a-fA-F]{40}|did:layerx:[0-9a-fA-F]{64})$"
      },
      "description": "A Paxeer EVM address, or an explicit DID resolved to its bound EVM address through addr 0x0000000000000000000000000000000000001004; an unbound DID is refused with -32602."
    },
    {
      "name": "cursor",
      "required": false,
      "schema": {
        "type": [
          "string",
          "null"
        ],
        "pattern": "^[0-9a-f]{32}$"
      },
      "description": "The next_cursor of the previous page: the indexer's 32-lowercase-hex (blockHeight, txIndex, logIndex) position cursor, passed through unchanged; null or absent for the first page. Any other form, including the 112-hex form, is refused with -32602."
    },
    {
      "name": "limit",
      "required": false,
      "schema": {
        "type": [
          "integer",
          "null"
        ],
        "minimum": 1,
        "maximum": 100,
        "default": 50
      },
      "description": "Rows per page."
    },
    {
      "name": "kind",
      "required": false,
      "schema": {
        "type": [
          "string",
          "null"
        ],
        "pattern": "^[a-z0-9_]{1,64}$"
      },
      "description": "Only rows of this indexer kind, for example native_transfer, erc20_transfer, pointer_transfer or bank_transfer."
    }
  ],
  "result": {
    "name": "result",
    "schema": {
      "type": "object",
      "required": [
        "account",
        "items",
        "next_cursor",
        "indexedHeight",
        "chainHeight",
        "freshness"
      ],
      "additionalProperties": false,
      "properties": {
        "account": {
          "type": "string",
          "pattern": "^0x[0-9a-f]{40}$"
        },
        "items": {
          "type": "array",
          "maxItems": 100,
          "items": {
            "type": "object",
            "required": [
              "id",
              "height_or_seq",
              "chain",
              "kind",
              "direction",
              "account",
              "counterparty",
              "asset",
              "amount",
              "tx_id",
              "ordinal",
              "final",
              "decoded",
              "asset_metadata",
              "blockHeight",
              "txIndex",
              "logIndex"
            ],
            "additionalProperties": false,
            "properties": {
              "id": {
                "type": "string",
                "pattern": "^[1-9][0-9]*$",
                "description": "The indexer row id; rows are ordered by (blockHeight, txIndex, logIndex)."
              },
              "height_or_seq": {
                "type": "string",
                "pattern": "^(0|[1-9][0-9]*)$",
                "description": "The LayerX sequence or Paxeer block height of the row."
              },
              "chain": {
                "type": "string",
                "enum": [
                  "layerx",
                  "paxeer"
                ],
                "description": "The chain that recorded the row."
              },
              "kind": {
                "type": "string",
                "pattern": "^[a-z0-9_]{1,64}$"
              },
              "direction": {
                "type": "string",
                "enum": [
                  "in",
                  "out"
                ]
              },
              "account": {
                "type": "string"
              },
              "counterparty": {
                "type": [
                  "string",
                  "null"
                ]
              },
              "asset": {
                "type": "string"
              },
              "amount": {
                "type": "string",
                "pattern": "^(0|[1-9][0-9]*)$",
                "description": "Exact decimal amount in the asset's base units."
              },
              "tx_id": {
                "type": "string"
              },
              "ordinal": {
                "type": "string",
                "pattern": "^(0|[1-9][0-9]*)$"
              },
              "final": {
                "type": "boolean",
                "description": "True once the row is below its chain's finalized boundary."
              },
              "decoded": {
                "description": "The indexer's decoded payload for the row."
              },
              "asset_metadata": {
                "type": [
                  "object",
                  "null"
                ],
                "description": "The row's asset joined from the indexer's /v1/assets record and the LayerX asset registry, or null when the indexer has no record of the asset.",
                "required": [
                  "asset",
                  "chain",
                  "kind",
                  "address",
                  "denom",
                  "symbol",
                  "decimals",
                  "native_id",
                  "pointer",
                  "metadata"
                ],
                "additionalProperties": false,
                "properties": {
                  "asset": {
                    "type": "string",
                    "description": "The indexer asset label: 64-hex LayerX asset id, evm:native, evm:<address> or denom:<denom>."
                  },
                  "chain": {
                    "type": "string",
                    "enum": [
                      "layerx",
                      "paxeer"
                    ]
                  },
                  "kind": {
                    "type": "string",
                    "description": "The indexer asset kind: layerx, native, erc20, pointer, bank or tokenfactory."
                  },
                  "address": {
                    "type": [
                      "string",
                      "null"
                    ],
                    "description": "The EVM contract address for erc20 and pointer assets."
                  },
                  "denom": {
                    "type": [
                      "string",
                      "null"
                    ],
                    "description": "The Cosmos denom for bank, tokenfactory and pointer assets."
                  },
                  "symbol": {
                    "type": [
                      "string",
                      "null"
                    ],
                    "description": "The indexer metadata symbol, else the LayerX registry symbol of the asset (by asset id) or of its denom; null when neither declares one."
                  },
                  "decimals": {
                    "type": [
                      "integer",
                      "null"
                    ],
                    "minimum": 0,
                    "description": "The indexer metadata decimals, else the LayerX registry decimals, else 18 for the EVM native coin; null when nothing declares them."
                  },
                  "native_id": {
                    "type": [
                      "string",
                      "null"
                    ],
                    "pattern": "^[0-9a-f]{64}$",
                    "description": "The LayerX asset id this asset is, or that the registry maps its denom to."
                  },
                  "pointer": {
                    "type": [
                      "string",
                      "null"
                    ],
                    "pattern": "^0x[0-9a-f]{40}$",
                    "description": "The EVM pointer contract address when the asset is a pointer."
                  },
                  "metadata": {
                    "description": "The indexer's own metadata document for the asset."
                  }
                }
              },
              "blockHeight": {
                "type": "integer",
                "minimum": 0,
                "description": "The block height of the row's position."
              },
              "txIndex": {
                "type": "integer",
                "minimum": 0,
                "description": "The transaction index of the row's position."
              },
              "logIndex": {
                "type": "integer",
                "minimum": 0,
                "description": "The log index of the row's position."
              },
              "tx_index": {
                "type": "string",
                "pattern": "^(0|[1-9][0-9]*)$"
              },
              "log_index": {
                "type": "string",
                "pattern": "^(0|[1-9][0-9]*)$"
              }
            }
          }
        },
        "next_cursor": {
          "type": [
            "string",
            "null"
          ],
          "description": "The opaque cursor of the next older page, or null when this page is the last."
        },
        "indexedHeight": {
          "type": [
            "integer",
            "null"
          ],
          "minimum": 0,
          "description": "The Paxeer height the indexer has indexed (indexer freshness); null before the first indexed block."
        },
        "chainHeight": {
          "type": "integer",
          "minimum": 0,
          "description": "The Paxeer head height eth_blockNumber answers."
        },
        "freshness": {
          "type": "object",
          "description": "The indexer readiness document."
        }
      }
    }
  },
  "description": "The indexed transfer history of one Paxeer EVM address, newest first, from the layerx-indexer read API: native, ERC-20, pointer, bank, tokenfactory and custody rows, each joined with asset_metadata. Cursors are the indexer's, passed through unchanged. An address the indexer has never seen answers an empty page.",
  "errors": [
    {
      "code": -32602,
      "message": "Invalid params"
    },
    {
      "code": -32001,
      "message": "History unavailable"
    },
    {
      "code": -32005,
      "message": "Read unavailable"
    }
  ]
}
```

### px_getUnifiedHistory

```json
{
  "decision": "served",
  "implementation_task": "22.34.3",
  "node_sources": [
    "services/platform/hosted/indexer/src/api.rs",
    "services/platform/hosted/indexer/src/store.rs",
    "precompiles/addr/addr.go",
    "rpc/association.go",
    "rpc/state.go",
    "src/ledger/lx_account_registry.c",
    "src/ledger/lx_account_read.c"
  ],
  "gateway_sources": [
    "services/platform/hosted/gateway/src/history.rs",
    "services/platform/hosted/gateway/src/paxeer.rs"
  ]
}
```

Resolve the account with the same local binding rules, enumerate its owned native accounts from the integrated ledger registry when available, and read each side from the indexer using the same position cursor. Merge/deduplicate by indexer position, tag every row with side, cap at 16 account keys and 100 rows, and derive next_cursor from the last returned row without gaps or repeats. Preserve account/accounts/items, asset_metadata, indexedHeight, chainHeight and freshness. Missing indexer is -32001/data.code indexer_not_configured. If required native-account enumeration is unavailable, return -32001 with an explicit unavailable reason, never quietly serve a partial unified page. The OpenRPC description promises the EVM plus native account merge, while its account parameter description says EVM-only and the current gateway served path selects only the EVM account. This existing contract/implementation discrepancy must be resolved by the history owner before claiming native-history cutover; retain both descriptions verbatim below. Hosted behaviour is unchanged by this design task; no client is migrated on an assumed native-account merge.

Hosted contract:

```json
{
  "name": "px_getUnifiedHistory",
  "paramStructure": "by-position",
  "params": [
    {
      "name": "account",
      "required": true,
      "schema": {
        "type": "string",
        "pattern": "^(0x[0-9a-fA-F]{40}|did:layerx:[0-9a-fA-F]{64})$"
      },
      "description": "An EVM address or explicit DID; a DID resolves to its bound EVM address through addr 0x0000000000000000000000000000000000001004 and history is read for that EVM address only. An unbound DID is refused with -32602."
    },
    {
      "name": "cursor",
      "required": false,
      "schema": {
        "type": [
          "string",
          "null"
        ],
        "pattern": "^[0-9a-f]{32}$"
      },
      "description": "The next_cursor of the previous page: the indexer's 32-lowercase-hex (blockHeight, txIndex, logIndex) position cursor, passed through unchanged; null or absent for the first page. Any other form, including the 112-hex form, is refused with -32602."
    },
    {
      "name": "limit",
      "required": false,
      "schema": {
        "type": [
          "integer",
          "null"
        ],
        "minimum": 1,
        "maximum": 100,
        "default": 50
      },
      "description": "Rows per page."
    },
    {
      "name": "kind",
      "required": false,
      "schema": {
        "type": [
          "string",
          "null"
        ],
        "pattern": "^[a-z0-9_]{1,64}$"
      },
      "description": "Only rows of this indexer kind, for example native_transfer, erc20_transfer, pointer_transfer or bank_transfer."
    }
  ],
  "result": {
    "name": "result",
    "schema": {
      "type": "object",
      "required": [
        "account",
        "accounts",
        "items",
        "next_cursor",
        "indexedHeight",
        "chainHeight",
        "freshness"
      ],
      "additionalProperties": false,
      "properties": {
        "account": {
          "type": "object",
          "description": "The px_resolveAccount document of the account."
        },
        "accounts": {
          "type": "array",
          "maxItems": 16,
          "items": {
            "type": "object",
            "required": [
              "side",
              "account"
            ],
            "additionalProperties": false,
            "properties": {
              "side": {
                "type": "string",
                "enum": [
                  "layerx",
                  "paxeer"
                ]
              },
              "account": {
                "type": "string"
              }
            }
          },
          "description": "The indexer account keys merged: the EVM address and every LayerX account of the bound DID (main account and the public core's per-asset accounts), at most 16."
        },
        "items": {
          "type": "array",
          "maxItems": 100,
          "items": {
            "type": "object",
            "required": [
              "id",
              "height_or_seq",
              "chain",
              "kind",
              "direction",
              "account",
              "counterparty",
              "asset",
              "amount",
              "tx_id",
              "ordinal",
              "final",
              "decoded",
              "asset_metadata",
              "side",
              "blockHeight",
              "txIndex",
              "logIndex"
            ],
            "additionalProperties": false,
            "properties": {
              "id": {
                "type": "string",
                "pattern": "^[1-9][0-9]*$",
                "description": "The indexer row id; rows are ordered by (blockHeight, txIndex, logIndex)."
              },
              "height_or_seq": {
                "type": "string",
                "pattern": "^(0|[1-9][0-9]*)$",
                "description": "The LayerX sequence or Paxeer block height of the row."
              },
              "chain": {
                "type": "string",
                "enum": [
                  "layerx",
                  "paxeer"
                ],
                "description": "The chain that recorded the row."
              },
              "kind": {
                "type": "string",
                "pattern": "^[a-z0-9_]{1,64}$"
              },
              "direction": {
                "type": "string",
                "enum": [
                  "in",
                  "out"
                ]
              },
              "account": {
                "type": "string"
              },
              "counterparty": {
                "type": [
                  "string",
                  "null"
                ]
              },
              "asset": {
                "type": "string"
              },
              "amount": {
                "type": "string",
                "pattern": "^(0|[1-9][0-9]*)$",
                "description": "Exact decimal amount in the asset's base units."
              },
              "tx_id": {
                "type": "string"
              },
              "ordinal": {
                "type": "string",
                "pattern": "^(0|[1-9][0-9]*)$"
              },
              "final": {
                "type": "boolean",
                "description": "True once the row is below its chain's finalized boundary."
              },
              "decoded": {
                "description": "The indexer's decoded payload for the row."
              },
              "asset_metadata": {
                "type": [
                  "object",
                  "null"
                ],
                "description": "The row's asset joined from the indexer's /v1/assets record and the LayerX asset registry, or null when the indexer has no record of the asset.",
                "required": [
                  "asset",
                  "chain",
                  "kind",
                  "address",
                  "denom",
                  "symbol",
                  "decimals",
                  "native_id",
                  "pointer",
                  "metadata"
                ],
                "additionalProperties": false,
                "properties": {
                  "asset": {
                    "type": "string",
                    "description": "The indexer asset label: 64-hex LayerX asset id, evm:native, evm:<address> or denom:<denom>."
                  },
                  "chain": {
                    "type": "string",
                    "enum": [
                      "layerx",
                      "paxeer"
                    ]
                  },
                  "kind": {
                    "type": "string",
                    "description": "The indexer asset kind: layerx, native, erc20, pointer, bank or tokenfactory."
                  },
                  "address": {
                    "type": [
                      "string",
                      "null"
                    ],
                    "description": "The EVM contract address for erc20 and pointer assets."
                  },
                  "denom": {
                    "type": [
                      "string",
                      "null"
                    ],
                    "description": "The Cosmos denom for bank, tokenfactory and pointer assets."
                  },
                  "symbol": {
                    "type": [
                      "string",
                      "null"
                    ],
                    "description": "The indexer metadata symbol, else the LayerX registry symbol of the asset (by asset id) or of its denom; null when neither declares one."
                  },
                  "decimals": {
                    "type": [
                      "integer",
                      "null"
                    ],
                    "minimum": 0,
                    "description": "The indexer metadata decimals, else the LayerX registry decimals, else 18 for the EVM native coin; null when nothing declares them."
                  },
                  "native_id": {
                    "type": [
                      "string",
                      "null"
                    ],
                    "pattern": "^[0-9a-f]{64}$",
                    "description": "The LayerX asset id this asset is, or that the registry maps its denom to."
                  },
                  "pointer": {
                    "type": [
                      "string",
                      "null"
                    ],
                    "pattern": "^0x[0-9a-f]{40}$",
                    "description": "The EVM pointer contract address when the asset is a pointer."
                  },
                  "metadata": {
                    "description": "The indexer's own metadata document for the asset."
                  }
                }
              },
              "side": {
                "type": "string",
                "enum": [
                  "layerx",
                  "paxeer"
                ],
                "description": "Which half of the unified account the row belongs to."
              },
              "blockHeight": {
                "type": "integer",
                "minimum": 0,
                "description": "The block height of the row's position."
              },
              "txIndex": {
                "type": "integer",
                "minimum": 0,
                "description": "The transaction index of the row's position."
              },
              "logIndex": {
                "type": "integer",
                "minimum": 0,
                "description": "The log index of the row's position."
              },
              "tx_index": {
                "type": "string",
                "pattern": "^(0|[1-9][0-9]*)$"
              },
              "log_index": {
                "type": "string",
                "pattern": "^(0|[1-9][0-9]*)$"
              }
            }
          }
        },
        "next_cursor": {
          "type": [
            "string",
            "null"
          ],
          "description": "The opaque cursor of the next older page, or null when this page is the last."
        },
        "indexedHeight": {
          "type": [
            "integer",
            "null"
          ],
          "minimum": 0,
          "description": "The Paxeer height the indexer has indexed (indexer freshness); null before the first indexed block."
        },
        "chainHeight": {
          "type": "integer",
          "minimum": 0,
          "description": "The Paxeer head height eth_blockNumber answers."
        },
        "freshness": {
          "type": "object",
          "description": "The indexer readiness document."
        }
      }
    }
  },
  "description": "One history for the whole unified account: the account is resolved as px_resolveAccount does and the indexer pages of its EVM address and of each of its LayerX accounts are merged newest first in the indexer's row order, each row tagged with side and carrying its chain and asset_metadata. Every side is read at the same cursor, so next_cursor is the indexer row cursor of the last row returned and pages chain without gaps or repeats.",
  "errors": [
    {
      "code": -32602,
      "message": "Invalid params"
    },
    {
      "code": -32001,
      "message": "Paxeer read unavailable"
    },
    {
      "code": -32001,
      "message": "History unavailable"
    },
    {
      "code": -32005,
      "message": "Read unavailable"
    }
  ]
}
```

### px_listHolderRecords

```json
{
  "decision": "served",
  "implementation_task": "22.34.3",
  "node_sources": [
    "services/platform/hosted/indexer/src/api.rs",
    "services/platform/hosted/indexer/src/store.rs",
    "precompiles/layerxpay/budget.go",
    "precompiles/layerxauth/grants.go",
    "precompiles/layerxledger/views.go",
    "src/modules/budget/lx_budget_create.c"
  ],
  "gateway_sources": [
    "services/platform/hosted/gateway/src/history.rs"
  ]
}
```

Page configured indexer /v1/holders/{address} by kind budget or grant. The precompile budget/grant state and kernel budget creation are provenance, not a replacement for the indexer holder enumeration. Preserve items, next_cursor, indexedHeight and local chainHeight; limit defaults to 50, maximum 100, and a cursor is exactly 32 lowercase hex characters. Missing/unavailable indexer returns -32001 History unavailable, never an empty successful list. Hosted kind validation, cursor limits and record payloads stay identical.

Hosted contract:

```json
{
  "name": "px_listHolderRecords",
  "paramStructure": "by-position",
  "params": [
    {
      "name": "address",
      "required": true,
      "schema": {
        "type": "string",
        "pattern": "^0x[0-9A-Fa-f]{40}$"
      },
      "description": "The holder's Paxeer EVM address."
    },
    {
      "name": "kind",
      "required": true,
      "schema": {
        "type": "string",
        "enum": [
          "budget",
          "grant"
        ]
      },
      "description": "The record kind: layerxpay budgets or ledger and grant precompile grants."
    },
    {
      "name": "cursor",
      "required": false,
      "schema": {
        "type": [
          "string",
          "null"
        ],
        "pattern": "^[0-9a-f]{32}$"
      },
      "description": "The next_cursor of the previous page: the indexer's 32-lowercase-hex (blockHeight, txIndex, logIndex) position cursor, passed through unchanged; null or absent for the first page. Any other form, including the 112-hex form, is refused with -32602."
    },
    {
      "name": "limit",
      "required": false,
      "schema": {
        "type": [
          "integer",
          "null"
        ],
        "minimum": 1,
        "maximum": 100,
        "default": 50
      },
      "description": "Records per page; above 100 is refused with -32602."
    }
  ],
  "result": {
    "name": "result",
    "schema": {
      "type": "object",
      "required": [
        "items",
        "next_cursor",
        "indexedHeight",
        "chainHeight"
      ],
      "additionalProperties": false,
      "properties": {
        "items": {
          "type": "array",
          "maxItems": 100,
          "items": {
            "type": "object",
            "description": "One indexer holder record of the requested kind held by the address."
          }
        },
        "next_cursor": {
          "type": [
            "string",
            "null"
          ],
          "pattern": "^[0-9a-f]{32}$"
        },
        "indexedHeight": {
          "type": [
            "integer",
            "null"
          ],
          "minimum": 0,
          "description": "The Paxeer height the indexer has indexed (indexer freshness); null before the first indexed block."
        },
        "chainHeight": {
          "type": "integer",
          "minimum": 0,
          "description": "The Paxeer head height eth_blockNumber answers."
        }
      }
    }
  },
  "errors": [
    {
      "code": -32602,
      "message": "Invalid params"
    },
    {
      "code": -32001,
      "message": "History unavailable"
    },
    {
      "code": -32005,
      "message": "Read unavailable"
    }
  ]
}
```

### px_getCapabilities

```json
{
  "decision": "served",
  "implementation_task": "22.34.2",
  "node_sources": [
    "rpc/simulate.go",
    "precompiles/setup.go",
    "precompiles/layerxledger/views.go",
    "precompiles/layerxpay/budget.go",
    "precompiles/layerxauth/views.go",
    "precompiles/layerxprograms/view_fees.go"
  ],
  "gateway_sources": [
    "services/platform/hosted/gateway/src/capabilities.rs"
  ]
}
```

Run the seven exact view probes in the OpenRPC description against one local committed height through the same EVM call path. Addresses are exchange 0x1015, bridge 0x1016, launchpad 0x1017, ledger 0x101a, pay 0x101b, auth 0x101c and programs 0x101d. A true value requires ABI-decodable output, not merely code or registration; revert, malformed and empty output mean false. Preserve addresses, Unix-seconds probed_at and decimal-string rpc_height. Preserve the hosted 15-second default capability cache and its surface-unavailable write gate; reads/history stay ungated. Hosted gateway capability probes, caching and write refusals remain unchanged. Node registration alone must not declare a hosted product healthy.

Hosted contract:

```json
{
  "name": "px_getCapabilities",
  "paramStructure": "by-position",
  "params": [],
  "result": {
    "name": "result",
    "schema": {
      "type": "object",
      "required": [
        "exchange",
        "bridge",
        "launchpad",
        "ledger",
        "pay",
        "auth",
        "programs",
        "addresses",
        "probed_at",
        "rpc_height"
      ],
      "properties": {
        "exchange": {
          "type": "boolean",
          "description": "True when eth_call of totalMarkets() at the exchange precompile 0x0000000000000000000000000000000000001015 decodes as the view's uint256 return value; false when the call reverts or answers a malformed value."
        },
        "bridge": {
          "type": "boolean",
          "description": "True when eth_call of isPaused() at the bridge precompile 0x0000000000000000000000000000000000001016 decodes as the view's bool return value; false when the call reverts or answers a malformed value."
        },
        "launchpad": {
          "type": "boolean",
          "description": "True when eth_call of getPoolCount() at the launchpad precompile 0x0000000000000000000000000000000000001017 decodes as the view's uint256 return value; false when the call reverts or answers a malformed value."
        },
        "ledger": {
          "type": "boolean",
          "description": "True when eth_call of accountId(bytes32,bytes32) at the ledger precompile 0x000000000000000000000000000000000000101a using two zero bytes32 arguments decodes as the view's bytes32 return value; false when the call reverts or answers a malformed value."
        },
        "pay": {
          "type": "boolean",
          "description": "True when eth_call of budgetRemaining(bytes32) at the pay precompile 0x000000000000000000000000000000000000101b using a zero bytes32 argument decodes as the view's uint128 return value; false when the call reverts or answers a malformed value."
        },
        "auth": {
          "type": "boolean",
          "description": "True when eth_call of isLive(bytes32,uint64,uint64) at the auth precompile 0x000000000000000000000000000000000000101c using a zero bytes32 and two zero uint64 arguments decodes as the view's bool and string return values; false when the call reverts or answers a malformed value."
        },
        "programs": {
          "type": "boolean",
          "description": "True when eth_call of getCurrentFeeSchedule() at the programs precompile 0x000000000000000000000000000000000000101d decodes as the view's uint32, uint64[7] and bytes32 return values; false when the call reverts or answers a malformed value."
        },
        "addresses": {
          "type": "object",
          "required": [
            "exchange",
            "bridge",
            "launchpad",
            "ledger",
            "pay",
            "auth",
            "programs"
          ],
          "properties": {
            "exchange": {
              "type": "string"
            },
            "bridge": {
              "type": "string"
            },
            "launchpad": {
              "type": "string"
            },
            "ledger": {
              "type": "string"
            },
            "pay": {
              "type": "string"
            },
            "auth": {
              "type": "string"
            },
            "programs": {
              "type": "string"
            }
          }
        },
        "probed_at": {
          "type": "integer",
          "minimum": 0,
          "description": "Unix seconds at which the gateway took the probe."
        },
        "rpc_height": {
          "type": "string",
          "pattern": "^(0|[1-9][0-9]*)$",
          "description": "The Paxeer block height, as a decimal string, at which every probe call was made."
        }
      }
    }
  },
  "description": "Which Paxeer X fork surfaces the chain answers for. The gateway reads eth_blockNumber and then uses eth_call at that height to probe each precompile's view: exchange at 0x0000000000000000000000000000000000001015 with totalMarkets(), bridge at 0x0000000000000000000000000000000000001016 with isPaused(), launchpad at 0x0000000000000000000000000000000000001017 with getPoolCount(), ledger at 0x000000000000000000000000000000000000101a with accountId(bytes32,bytes32) using two zero bytes32 arguments, pay at 0x000000000000000000000000000000000000101b with budgetRemaining(bytes32) using a zero bytes32 argument, auth at 0x000000000000000000000000000000000000101c with isLive(bytes32,uint64,uint64) using a zero bytes32 and two zero uint64 arguments, and programs at 0x000000000000000000000000000000000000101d with getCurrentFeeSchedule(). A surface is true when the answer decodes as its view's return value, and false when the call reverts or answers a malformed value, including an empty answer. The answer is cached for LAYERX_GATEWAY_CAPABILITIES_TTL_SECONDS (default 15). While a surface is false, eth_sendRawTransaction and eth_sendRawTransactionSync addressed to its precompile are refused with -32003 Surface unavailable and data {code: surface_unavailable, surface, address}; reads and history are never gated. Takes no arguments.",
  "errors": [
    {
      "code": -32602,
      "message": "Invalid params"
    },
    {
      "code": -32001,
      "message": "Paxeer read unavailable"
    },
    {
      "code": -32001,
      "message": "Paxeer endpoint not configured"
    }
  ]
}
```

### px_getRouteCatalogue

```json
{
  "decision": "served",
  "implementation_task": "22.34.2",
  "node_sources": [
    "rpc/server.go",
    "rpc/served_methods.go",
    "tools/paxeer-x/route-catalogue.json"
  ],
  "gateway_sources": [
    "services/platform/hosted/gateway/src/routes.rs"
  ]
}
```

There is no node route-catalogue provider yet. Implement a local catalogue adapter alongside root RPC registration, using the committed route catalogue as definitions and actual node registration/configuration as readiness evidence. Return schema=paxeer-x.routes.v1, chain_id=125, services/routes arrays and readiness object. The optional canonical_origin, when included, retains its contract constant; it does not redirect node reads. rpc_paths advertises only paths actually served. Hosted routes may be described, but their readiness must be explicitly unavailable/unknown unless measured by a configured dependency check; a route entry never proves end-to-end success. Do not copy gateway secrets, sessions or private upstream topology into a public node response. The hosted gateway keeps its authenticated catalogue, dependency health and identity checks unchanged; node readiness is scoped to this node and is not a claim to serve the hosted product routes.

Hosted contract:

```json
{
  "name": "px_getRouteCatalogue",
  "paramStructure": "by-position",
  "summary": "Authenticated product route catalogue and dependency readiness.",
  "params": [],
  "result": {
    "name": "catalogue",
    "schema": {
      "type": "object",
      "required": [
        "schema",
        "chain_id",
        "services",
        "routes",
        "readiness"
      ],
      "properties": {
        "schema": {
          "const": "paxeer-x.routes.v1"
        },
        "chain_id": {
          "const": 125
        },
        "services": {
          "type": "array",
          "items": {
            "type": "object"
          }
        },
        "routes": {
          "type": "array",
          "items": {
            "type": "object"
          }
        },
        "readiness": {
          "type": "object"
        },
        "canonical_origin": {
          "const": "https://api-mainnet-beta.paxeer.network"
        },
        "rpc_paths": {
          "type": "array",
          "items": {
            "type": "string",
            "enum": [
              "/",
              "/rpc"
            ]
          }
        }
      }
    }
  },
  "description": "Returns the unified-origin route catalogue and dependency readiness. Readiness reports dependency health and identity checks; route entries describe request contracts and do not establish successful end-to-end feature execution."
}
```

## Client cutover order

1. Deploy and qualify node groups 22.34.2, 22.34.4 and then 22.34.3 on the
   intended node pool. Check root method availability and result/error parity,
   network identity 125, pinned heights, cursor continuity and receipt authority
   before any client endpoint change. A registered method returning an
   unavailable error is not readiness for that consumer. Keep the hosted gateway
   serving every method throughout deployment and client migration.
2. Move reads in `services/agent/crates/layerx-client/src/px.rs` to explicitly
   configured node-root endpoints after 22.34.2; retain all decoding and identity
   checks. Keep service operations on their hosted endpoint rather than blindly
   changing a shared transport for all methods.
3. Move the daemon's `node_endpoint` / `LAYERX_NODE_ENDPOINT` in
   `services/agent/crates/layerx-agentd/src/config.rs` only after 22.34.4 is
   available as well. Retain `expected_chain_id` / `LAYERX_EXPECTED_CHAIN_ID`,
   `receipt_key_source` / `LAYERX_RECEIPT_KEY_SOURCE`, and independently trusted
   checkpoints. Review `services/agent/crates/layerx-agentd/src/read/checkpoint.rs`,
   `services/agent/crates/layerx-agentd/src/read/committed.rs`,
   `services/agent/crates/layerx-client/src/rpc/commit.rs` and
   `services/agent/crates/layerx-client/src/rpc/receipt_commit.rs` together:
   commit/validator/batch and state witnesses must describe the same authenticated
   committed block. An endpoint change never lowers the verification level or
   turns an endpoint-supplied signing key into independent trust.
4. Move `apps/web/src/api/gateway.ts` read calls after all reads it consumes,
   including 22.34.3 history, pass node parity. Its current shared
   `LAYERX_GATEWAY_URL` cannot simply be repointed: introduce explicit node/read
   routing while keeping registration, funding, authenticated route catalogue
   access and other hosted operations on the gateway. Preserve the original
   EVM/DID lookup selector, validators and meaningful unavailable errors.
5. Move the endpoint checks in `scripts/wallet/check-live.sh` after the matching
   web/read cutover; keep its hosted gateway, session and service checks targeting
   the gateway. Move the receipt-key fetch in `docker/gas-station/init.sh` only
   after the chosen node serves governed `px_getReceiptKeys`. Preserve
   `GAS_STATION_RECEIPT_KEYS` explicit pins, range validation and fail-closed
   startup; never accept empty/untrusted pins merely because transport succeeds.

These are follow-up client changes, outside this task's scope. Roll back a
consumer by restoring its prior configured gateway route; do not remove the
hosted method during the rollout. Retire a hosted read only through a separate
explicit decision after node readiness and all its consumers are demonstrated.
Registration/funding are never included in that retirement.

## Qualification and limits

`tools/qualification/audit_repairs/test_px_node_root_design.py` reads the real
OpenRPC file and this document in-process. It checks all seventeen entries,
exact contract content, existing source files, explicit decisions, refusal
codes, group membership without overlap, and the client cutover obligations.
It uses no network, credentials, subprocesses or native toolchain (gotcha 2962).
This proves the design inventory; it does not prove execution of future node
handlers, hosted/native-history parity, or a live cutover.

The repair record is the bounded gate evidence: `hf-build` records
`python3 --version`; `hf-done` records `cg spec done 22.34.1 -f layerx-merge
--root "$TASK_ROOT"` and the declared pytest verification. The private gate
ledger records the exact verified candidate revision, command, exit code and
log path; the final task handoff copies that evidence. No gate is claimed here
before it runs. Rollback is a revert of the runner's task commit.
