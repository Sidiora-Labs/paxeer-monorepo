# Node-root inference design supplement

The [node implementation](../../../../rpc/px_inference.go) serves five inference
reads. The complete contract inventory contains 22 methods: 20 node reads and
two hosted-service refusals. The account, history, commitment and inference
groups partition the 20 reads. Source and complete contract objects below come
from the served node implementation and current OpenRPC publication.

## Implementation groups

```json
{
  "22.38.12": {
    "methods": [
      "px_getInferenceRooms",
      "px_getInferenceRoom",
      "px_getInferenceDeal",
      "px_getInferencePeer",
      "px_getInferenceReceipts"
    ],
    "node_sources": [
      "rpc/server.go",
      "rpc/served_methods.go",
      "rpc/px_account.go",
      "rpc/px_inference.go",
      "programs/crates/layerx-programs-ai-market/src/queries.rs",
      "programs/crates/layerx-programs-ai-market/src/rooms.rs",
      "programs/crates/layerx-programs-ai-market/src/laps.rs",
      "precompiles/layerxpay/budget.go"
    ]
  }
}
```

## Method inventory

### px_getInferenceRooms

```json
{
  "decision": "served",
  "implementation_task": "22.38.12",
  "node_sources": [
    "rpc/server.go",
    "rpc/served_methods.go",
    "rpc/px_account.go",
    "rpc/px_inference.go",
    "programs/crates/layerx-programs-ai-market/src/queries.rs",
    "programs/crates/layerx-programs-ai-market/src/rooms.rs",
    "programs/crates/layerx-programs-ai-market/src/laps.rs",
    "precompiles/layerxpay/budget.go"
  ],
  "gateway_sources": [
    "services/platform/hosted/gateway/src/rpc.rs",
    "services/platform/hosted/gateway/src/lib.rs"
  ]
}
```

Node HTTP and WebSocket roots register pxInferenceAPI. Parameters are strict; reads use one committed height and refuse unavailable or malformed program state. Room reads bind the pay stream; peer reads bind the 0x0907 attestation. Fixed-width decoders reject truncated, trailing and noncanonical bytes.

Hosted gateway dispatch remains unchanged: its 17-method router does not serve inference. The five contracts are published for the node reads; this publication confers no hosted execution authority.

```json
{
  "name": "px_getInferenceRooms",
  "paramStructure": "by-position",
  "params": [],
  "result": {
    "name": "result",
    "schema": {
      "type": "array",
      "items": {
        "type": "object",
        "required": [
          "room_id",
          "host_key",
          "model_digest",
          "runtime_digest",
          "profile",
          "state",
          "opened_at_epoch",
          "open_asks",
          "open_bids",
          "ask_streams"
        ],
        "additionalProperties": false,
        "properties": {
          "room_id": {
            "type": "string",
            "pattern": "^[0-9a-f]{64}$"
          },
          "host_key": {
            "type": "string",
            "pattern": "^[0-9a-f]{64}$"
          },
          "model_digest": {
            "type": "string",
            "pattern": "^[0-9a-f]{64}$"
          },
          "runtime_digest": {
            "type": "string",
            "pattern": "^[0-9a-f]{64}$"
          },
          "profile": {
            "type": "object",
            "required": [
              "minimum_bond",
              "lap_size_tokens",
              "challenge_window_blocks",
              "canary_ratio_bps",
              "slash_fraction_bps",
              "host_share_bps",
              "protocol_share_bps",
              "settlement_account"
            ],
            "additionalProperties": false,
            "properties": {
              "minimum_bond": {
                "type": "string",
                "pattern": "^(0|[1-9][0-9]*)$"
              },
              "lap_size_tokens": {
                "type": "string",
                "pattern": "^(0|[1-9][0-9]*)$"
              },
              "challenge_window_blocks": {
                "type": "string",
                "pattern": "^(0|[1-9][0-9]*)$"
              },
              "canary_ratio_bps": {
                "type": "integer",
                "minimum": 0,
                "maximum": 10000
              },
              "slash_fraction_bps": {
                "type": "integer",
                "minimum": 0,
                "maximum": 10000
              },
              "host_share_bps": {
                "type": "integer",
                "minimum": 0,
                "maximum": 10000
              },
              "protocol_share_bps": {
                "type": "integer",
                "minimum": 0,
                "maximum": 10000
              },
              "settlement_account": {
                "type": "string",
                "pattern": "^[0-9a-f]{64}$"
              }
            }
          },
          "state": {
            "enum": [
              0,
              1
            ]
          },
          "opened_at_epoch": {
            "type": "string",
            "pattern": "^(0|[1-9][0-9]*)$"
          },
          "open_asks": {
            "type": "array",
            "items": {
              "type": "string",
              "pattern": "^[0-9a-f]{64}$"
            },
            "maxItems": 64
          },
          "open_bids": {
            "type": "array",
            "items": {
              "type": "string",
              "pattern": "^[0-9a-f]{64}$"
            },
            "maxItems": 128
          },
          "ask_streams": {
            "type": "array",
            "items": {
              "type": "object",
              "required": [
                "ask_id",
                "stream_id",
                "stream"
              ],
              "additionalProperties": false,
              "properties": {
                "ask_id": {
                  "type": "string",
                  "pattern": "^[0-9a-f]{64}$"
                },
                "stream_id": {
                  "type": "string",
                  "pattern": "^[0-9a-f]{64}$"
                },
                "stream": {
                  "anyOf": [
                    {
                      "type": "null"
                    },
                    {
                      "type": "object",
                      "required": [
                        "payer",
                        "stream_account",
                        "recipient",
                        "asset_id",
                        "mode",
                        "rate",
                        "rate_unit",
                        "start_ms",
                        "last_accrual_ms",
                        "end_ms",
                        "total_cap",
                        "accrued_total",
                        "settled_total",
                        "remainder_carry",
                        "cumulative_meter",
                        "meter_authorities",
                        "underfunded",
                        "paused",
                        "closed"
                      ],
                      "additionalProperties": false,
                      "properties": {
                        "payer": {
                          "type": "string",
                          "pattern": "^[0-9a-f]{64}$"
                        },
                        "stream_account": {
                          "type": "string",
                          "pattern": "^[0-9a-f]{64}$"
                        },
                        "recipient": {
                          "type": "string",
                          "pattern": "^[0-9a-f]{64}$"
                        },
                        "asset_id": {
                          "type": "string",
                          "pattern": "^[0-9a-f]{64}$"
                        },
                        "mode": {
                          "type": "integer",
                          "minimum": 0
                        },
                        "rate": {
                          "type": "string",
                          "pattern": "^(0|[1-9][0-9]*)$"
                        },
                        "rate_unit": {
                          "type": "string",
                          "pattern": "^(0|[1-9][0-9]*)$"
                        },
                        "start_ms": {
                          "type": "string",
                          "pattern": "^(0|[1-9][0-9]*)$"
                        },
                        "last_accrual_ms": {
                          "type": "string",
                          "pattern": "^(0|[1-9][0-9]*)$"
                        },
                        "end_ms": {
                          "type": "string",
                          "pattern": "^(0|[1-9][0-9]*)$"
                        },
                        "total_cap": {
                          "type": "string",
                          "pattern": "^(0|[1-9][0-9]*)$"
                        },
                        "accrued_total": {
                          "type": "string",
                          "pattern": "^(0|[1-9][0-9]*)$"
                        },
                        "settled_total": {
                          "type": "string",
                          "pattern": "^(0|[1-9][0-9]*)$"
                        },
                        "remainder_carry": {
                          "type": "string",
                          "pattern": "^(0|[1-9][0-9]*)$"
                        },
                        "cumulative_meter": {
                          "type": "string",
                          "pattern": "^(0|[1-9][0-9]*)$"
                        },
                        "meter_authorities": {
                          "type": "array",
                          "items": {
                            "type": "string",
                            "pattern": "^[0-9a-f]{64}$"
                          },
                          "maxItems": 8
                        },
                        "underfunded": {
                          "type": "boolean"
                        },
                        "paused": {
                          "type": "boolean"
                        },
                        "closed": {
                          "type": "boolean"
                        }
                      }
                    }
                  ]
                }
              }
            },
            "maxItems": 64
          }
        }
      },
      "maxItems": 16
    }
  },
  "description": "Node root read of the deployed ai-market F11 program at one committed height. IDs are 32-byte lowercase hexadecimal without a prefix. The node must configure WithPxInferenceProgram with the deployed program and a bound read identity. Integers wider than 32 bits are decimal strings. Extra parameters are refused. Room answers include open ask streams read from 0x101B. An unavailable program, stream or binding is an explicit -32001 refusal. Hosted operators must route these reads to the configured node root.",
  "errors": [
    {
      "code": -32602,
      "message": "Invalid params"
    },
    {
      "code": -32001,
      "message": "Paxeer read unavailable"
    }
  ]
}
```

### px_getInferenceRoom

```json
{
  "decision": "served",
  "implementation_task": "22.38.12",
  "node_sources": [
    "rpc/server.go",
    "rpc/served_methods.go",
    "rpc/px_account.go",
    "rpc/px_inference.go",
    "programs/crates/layerx-programs-ai-market/src/queries.rs",
    "programs/crates/layerx-programs-ai-market/src/rooms.rs",
    "programs/crates/layerx-programs-ai-market/src/laps.rs",
    "precompiles/layerxpay/budget.go"
  ],
  "gateway_sources": [
    "services/platform/hosted/gateway/src/rpc.rs",
    "services/platform/hosted/gateway/src/lib.rs"
  ]
}
```

Node HTTP and WebSocket roots register pxInferenceAPI. Parameters are strict; reads use one committed height and refuse unavailable or malformed program state. Room reads bind the pay stream; peer reads bind the 0x0907 attestation. Fixed-width decoders reject truncated, trailing and noncanonical bytes.

Hosted gateway dispatch remains unchanged: its 17-method router does not serve inference. The five contracts are published for the node reads; this publication confers no hosted execution authority.

```json
{
  "name": "px_getInferenceRoom",
  "paramStructure": "by-position",
  "params": [
    {
      "name": "roomId",
      "required": true,
      "schema": {
        "type": "string",
        "pattern": "^[0-9a-f]{64}$"
      }
    }
  ],
  "result": {
    "name": "result",
    "schema": {
      "type": "object",
      "required": [
        "room_id",
        "host_key",
        "model_digest",
        "runtime_digest",
        "profile",
        "state",
        "opened_at_epoch",
        "open_asks",
        "open_bids",
        "ask_streams"
      ],
      "additionalProperties": false,
      "properties": {
        "room_id": {
          "type": "string",
          "pattern": "^[0-9a-f]{64}$"
        },
        "host_key": {
          "type": "string",
          "pattern": "^[0-9a-f]{64}$"
        },
        "model_digest": {
          "type": "string",
          "pattern": "^[0-9a-f]{64}$"
        },
        "runtime_digest": {
          "type": "string",
          "pattern": "^[0-9a-f]{64}$"
        },
        "profile": {
          "type": "object",
          "required": [
            "minimum_bond",
            "lap_size_tokens",
            "challenge_window_blocks",
            "canary_ratio_bps",
            "slash_fraction_bps",
            "host_share_bps",
            "protocol_share_bps",
            "settlement_account"
          ],
          "additionalProperties": false,
          "properties": {
            "minimum_bond": {
              "type": "string",
              "pattern": "^(0|[1-9][0-9]*)$"
            },
            "lap_size_tokens": {
              "type": "string",
              "pattern": "^(0|[1-9][0-9]*)$"
            },
            "challenge_window_blocks": {
              "type": "string",
              "pattern": "^(0|[1-9][0-9]*)$"
            },
            "canary_ratio_bps": {
              "type": "integer",
              "minimum": 0,
              "maximum": 10000
            },
            "slash_fraction_bps": {
              "type": "integer",
              "minimum": 0,
              "maximum": 10000
            },
            "host_share_bps": {
              "type": "integer",
              "minimum": 0,
              "maximum": 10000
            },
            "protocol_share_bps": {
              "type": "integer",
              "minimum": 0,
              "maximum": 10000
            },
            "settlement_account": {
              "type": "string",
              "pattern": "^[0-9a-f]{64}$"
            }
          }
        },
        "state": {
          "enum": [
            0,
            1
          ]
        },
        "opened_at_epoch": {
          "type": "string",
          "pattern": "^(0|[1-9][0-9]*)$"
        },
        "open_asks": {
          "type": "array",
          "items": {
            "type": "string",
            "pattern": "^[0-9a-f]{64}$"
          },
          "maxItems": 64
        },
        "open_bids": {
          "type": "array",
          "items": {
            "type": "string",
            "pattern": "^[0-9a-f]{64}$"
          },
          "maxItems": 128
        },
        "ask_streams": {
          "type": "array",
          "items": {
            "type": "object",
            "required": [
              "ask_id",
              "stream_id",
              "stream"
            ],
            "additionalProperties": false,
            "properties": {
              "ask_id": {
                "type": "string",
                "pattern": "^[0-9a-f]{64}$"
              },
              "stream_id": {
                "type": "string",
                "pattern": "^[0-9a-f]{64}$"
              },
              "stream": {
                "anyOf": [
                  {
                    "type": "null"
                  },
                  {
                    "type": "object",
                    "required": [
                      "payer",
                      "stream_account",
                      "recipient",
                      "asset_id",
                      "mode",
                      "rate",
                      "rate_unit",
                      "start_ms",
                      "last_accrual_ms",
                      "end_ms",
                      "total_cap",
                      "accrued_total",
                      "settled_total",
                      "remainder_carry",
                      "cumulative_meter",
                      "meter_authorities",
                      "underfunded",
                      "paused",
                      "closed"
                    ],
                    "additionalProperties": false,
                    "properties": {
                      "payer": {
                        "type": "string",
                        "pattern": "^[0-9a-f]{64}$"
                      },
                      "stream_account": {
                        "type": "string",
                        "pattern": "^[0-9a-f]{64}$"
                      },
                      "recipient": {
                        "type": "string",
                        "pattern": "^[0-9a-f]{64}$"
                      },
                      "asset_id": {
                        "type": "string",
                        "pattern": "^[0-9a-f]{64}$"
                      },
                      "mode": {
                        "type": "integer",
                        "minimum": 0
                      },
                      "rate": {
                        "type": "string",
                        "pattern": "^(0|[1-9][0-9]*)$"
                      },
                      "rate_unit": {
                        "type": "string",
                        "pattern": "^(0|[1-9][0-9]*)$"
                      },
                      "start_ms": {
                        "type": "string",
                        "pattern": "^(0|[1-9][0-9]*)$"
                      },
                      "last_accrual_ms": {
                        "type": "string",
                        "pattern": "^(0|[1-9][0-9]*)$"
                      },
                      "end_ms": {
                        "type": "string",
                        "pattern": "^(0|[1-9][0-9]*)$"
                      },
                      "total_cap": {
                        "type": "string",
                        "pattern": "^(0|[1-9][0-9]*)$"
                      },
                      "accrued_total": {
                        "type": "string",
                        "pattern": "^(0|[1-9][0-9]*)$"
                      },
                      "settled_total": {
                        "type": "string",
                        "pattern": "^(0|[1-9][0-9]*)$"
                      },
                      "remainder_carry": {
                        "type": "string",
                        "pattern": "^(0|[1-9][0-9]*)$"
                      },
                      "cumulative_meter": {
                        "type": "string",
                        "pattern": "^(0|[1-9][0-9]*)$"
                      },
                      "meter_authorities": {
                        "type": "array",
                        "items": {
                          "type": "string",
                          "pattern": "^[0-9a-f]{64}$"
                        },
                        "maxItems": 8
                      },
                      "underfunded": {
                        "type": "boolean"
                      },
                      "paused": {
                        "type": "boolean"
                      },
                      "closed": {
                        "type": "boolean"
                      }
                    }
                  }
                ]
              }
            }
          },
          "maxItems": 64
        }
      }
    }
  },
  "description": "Node root read of the deployed ai-market F11 program at one committed height. IDs are 32-byte lowercase hexadecimal without a prefix. The node must configure WithPxInferenceProgram with the deployed program and a bound read identity. Integers wider than 32 bits are decimal strings. Extra parameters are refused. Room answers include open ask streams read from 0x101B. An unavailable program, stream or binding is an explicit -32001 refusal. Hosted operators must route these reads to the configured node root.",
  "errors": [
    {
      "code": -32602,
      "message": "Invalid params"
    },
    {
      "code": -32001,
      "message": "Paxeer read unavailable"
    }
  ]
}
```

### px_getInferenceDeal

```json
{
  "decision": "served",
  "implementation_task": "22.38.12",
  "node_sources": [
    "rpc/server.go",
    "rpc/served_methods.go",
    "rpc/px_account.go",
    "rpc/px_inference.go",
    "programs/crates/layerx-programs-ai-market/src/queries.rs",
    "programs/crates/layerx-programs-ai-market/src/rooms.rs",
    "programs/crates/layerx-programs-ai-market/src/laps.rs",
    "precompiles/layerxpay/budget.go"
  ],
  "gateway_sources": [
    "services/platform/hosted/gateway/src/rpc.rs",
    "services/platform/hosted/gateway/src/lib.rs"
  ]
}
```

Node HTTP and WebSocket roots register pxInferenceAPI. Parameters are strict; reads use one committed height and refuse unavailable or malformed program state. Room reads bind the pay stream; peer reads bind the 0x0907 attestation. Fixed-width decoders reject truncated, trailing and noncanonical bytes.

Hosted gateway dispatch remains unchanged: its 17-method router does not serve inference. The five contracts are published for the node reads; this publication confers no hosted execution authority.

```json
{
  "name": "px_getInferenceDeal",
  "paramStructure": "by-position",
  "params": [
    {
      "name": "dealId",
      "required": true,
      "schema": {
        "type": "string",
        "pattern": "^[0-9a-f]{64}$"
      }
    }
  ],
  "result": {
    "name": "result",
    "schema": {
      "type": "object",
      "required": [
        "room_id",
        "deal_id",
        "ask_id",
        "roster_digest",
        "agreed_price_per_thousand_tokens",
        "state",
        "layers",
        "settled_tokens",
        "settled_amount_owed"
      ],
      "additionalProperties": false,
      "properties": {
        "room_id": {
          "type": "string",
          "pattern": "^[0-9a-f]{64}$"
        },
        "deal_id": {
          "type": "string",
          "pattern": "^[0-9a-f]{64}$"
        },
        "ask_id": {
          "type": "string",
          "pattern": "^[0-9a-f]{64}$"
        },
        "roster_digest": {
          "type": "string",
          "pattern": "^[0-9a-f]{64}$"
        },
        "agreed_price_per_thousand_tokens": {
          "type": "string",
          "pattern": "^(0|[1-9][0-9]*)$"
        },
        "state": {
          "enum": [
            0,
            1
          ]
        },
        "layers": {
          "type": "array",
          "items": {
            "type": "object",
            "required": [
              "bid_id",
              "first_layer",
              "end_layer"
            ],
            "additionalProperties": false,
            "properties": {
              "bid_id": {
                "type": "string",
                "pattern": "^[0-9a-f]{64}$"
              },
              "first_layer": {
                "type": "integer",
                "minimum": 0
              },
              "end_layer": {
                "type": "integer",
                "minimum": 0
              }
            }
          },
          "maxItems": 32
        },
        "settled_tokens": {
          "type": "string",
          "pattern": "^(0|[1-9][0-9]*)$"
        },
        "settled_amount_owed": {
          "type": "string",
          "pattern": "^(0|[1-9][0-9]*)$"
        }
      }
    }
  },
  "description": "Node root read of the deployed ai-market F11 program at one committed height. IDs are 32-byte lowercase hexadecimal without a prefix. The node must configure WithPxInferenceProgram with the deployed program and a bound read identity. Integers wider than 32 bits are decimal strings. Extra parameters are refused. An unavailable program, stream or binding is an explicit -32001 refusal. Hosted operators must route these reads to the configured node root.",
  "errors": [
    {
      "code": -32602,
      "message": "Invalid params"
    },
    {
      "code": -32001,
      "message": "Paxeer read unavailable"
    }
  ]
}
```

### px_getInferencePeer

```json
{
  "decision": "served",
  "implementation_task": "22.38.12",
  "node_sources": [
    "rpc/server.go",
    "rpc/served_methods.go",
    "rpc/px_account.go",
    "rpc/px_inference.go",
    "programs/crates/layerx-programs-ai-market/src/queries.rs",
    "programs/crates/layerx-programs-ai-market/src/rooms.rs",
    "programs/crates/layerx-programs-ai-market/src/laps.rs",
    "precompiles/layerxpay/budget.go"
  ],
  "gateway_sources": [
    "services/platform/hosted/gateway/src/rpc.rs",
    "services/platform/hosted/gateway/src/lib.rs"
  ]
}
```

Node HTTP and WebSocket roots register pxInferenceAPI. Parameters are strict; reads use one committed height and refuse unavailable or malformed program state. Room reads bind the pay stream; peer reads bind the 0x0907 attestation. Fixed-width decoders reject truncated, trailing and noncanonical bytes.

Hosted gateway dispatch remains unchanged: its 17-method router does not serve inference. The five contracts are published for the node reads; this publication confers no hosted execution authority.

```json
{
  "name": "px_getInferencePeer",
  "paramStructure": "by-position",
  "params": [
    {
      "name": "peerKey",
      "required": true,
      "schema": {
        "type": "string",
        "pattern": "^[0-9a-f]{64}$"
      }
    }
  ],
  "result": {
    "name": "result",
    "schema": {
      "type": "object",
      "required": [
        "peer_key",
        "worker",
        "delegate",
        "metadata",
        "generation",
        "key_version",
        "metadata_revision",
        "valid_from",
        "expiry",
        "state",
        "enrolled",
        "capabilities_status",
        "capabilities",
        "bond_status",
        "fault_count",
        "attestation_binding"
      ],
      "additionalProperties": false,
      "properties": {
        "peer_key": {
          "type": "string",
          "pattern": "^[0-9a-f]{64}$"
        },
        "worker": {
          "type": "string",
          "pattern": "^[0-9a-f]{64}$"
        },
        "delegate": {
          "type": "string",
          "pattern": "^[0-9a-f]{64}$"
        },
        "metadata": {
          "type": "string",
          "pattern": "^[0-9a-f]{64}$"
        },
        "generation": {
          "type": "string",
          "pattern": "^(0|[1-9][0-9]*)$"
        },
        "key_version": {
          "type": "string",
          "pattern": "^(0|[1-9][0-9]*)$"
        },
        "metadata_revision": {
          "type": "string",
          "pattern": "^(0|[1-9][0-9]*)$"
        },
        "valid_from": {
          "type": "string",
          "pattern": "^(0|[1-9][0-9]*)$"
        },
        "expiry": {
          "type": "string",
          "pattern": "^(0|[1-9][0-9]*)$"
        },
        "state": {
          "enum": [
            1,
            2,
            3,
            4,
            5,
            6
          ]
        },
        "enrolled": {
          "type": "boolean"
        },
        "capabilities_status": {
          "enum": [
            1,
            4
          ]
        },
        "capabilities": {
          "anyOf": [
            {
              "type": "null"
            },
            {
              "type": "object",
              "required": [
                "accelerator_memory_mib",
                "system_memory_mib",
                "bandwidth_class",
                "attestation_family",
                "runtime_kind",
                "region_code"
              ],
              "additionalProperties": false,
              "properties": {
                "accelerator_memory_mib": {
                  "type": "integer",
                  "minimum": 0
                },
                "system_memory_mib": {
                  "type": "integer",
                  "minimum": 0
                },
                "bandwidth_class": {
                  "type": "integer",
                  "minimum": 0
                },
                "attestation_family": {
                  "type": "integer",
                  "minimum": 0
                },
                "runtime_kind": {
                  "type": "integer",
                  "minimum": 0
                },
                "region_code": {
                  "type": "string",
                  "pattern": "^[A-Za-z]{2}$"
                }
              }
            }
          ]
        },
        "bond_status": {
          "const": 2
        },
        "fault_count": {
          "type": "integer",
          "minimum": 0
        },
        "attestation_binding": {
          "type": "object",
          "required": [
            "family",
            "model_digest",
            "runtime_digest",
            "block_height"
          ],
          "additionalProperties": false,
          "properties": {
            "family": {
              "type": "integer",
              "minimum": 0
            },
            "model_digest": {
              "type": "string",
              "pattern": "^[0-9a-f]{64}$"
            },
            "runtime_digest": {
              "type": "string",
              "pattern": "^[0-9a-f]{64}$"
            },
            "block_height": {
              "type": "string",
              "pattern": "^(0|[1-9][0-9]*)$"
            }
          }
        }
      }
    }
  },
  "description": "Node root read of the deployed ai-market F11 program at one committed height. IDs are 32-byte lowercase hexadecimal without a prefix. The node must configure WithPxInferenceProgram with the deployed program and a bound read identity. Integers wider than 32 bits are decimal strings. Extra parameters are refused. Peer answers include the binding read from 0x0907; zero fields mean no binding. An unavailable program, stream or binding is an explicit -32001 refusal. Hosted operators must route these reads to the configured node root.",
  "errors": [
    {
      "code": -32602,
      "message": "Invalid params"
    },
    {
      "code": -32001,
      "message": "Paxeer read unavailable"
    }
  ]
}
```

### px_getInferenceReceipts

```json
{
  "decision": "served",
  "implementation_task": "22.38.12",
  "node_sources": [
    "rpc/server.go",
    "rpc/served_methods.go",
    "rpc/px_account.go",
    "rpc/px_inference.go",
    "programs/crates/layerx-programs-ai-market/src/queries.rs",
    "programs/crates/layerx-programs-ai-market/src/rooms.rs",
    "programs/crates/layerx-programs-ai-market/src/laps.rs",
    "precompiles/layerxpay/budget.go"
  ],
  "gateway_sources": [
    "services/platform/hosted/gateway/src/rpc.rs",
    "services/platform/hosted/gateway/src/lib.rs"
  ]
}
```

Node HTTP and WebSocket roots register pxInferenceAPI. Parameters are strict; reads use one committed height and refuse unavailable or malformed program state. Room reads bind the pay stream; peer reads bind the 0x0907 attestation. Fixed-width decoders reject truncated, trailing and noncanonical bytes.

Hosted gateway dispatch remains unchanged: its 17-method router does not serve inference. The five contracts are published for the node reads; this publication confers no hosted execution authority.

```json
{
  "name": "px_getInferenceReceipts",
  "paramStructure": "by-position",
  "params": [
    {
      "name": "dealId",
      "required": true,
      "schema": {
        "type": "string",
        "pattern": "^[0-9a-f]{64}$"
      }
    }
  ],
  "result": {
    "name": "result",
    "schema": {
      "type": "array",
      "items": {
        "type": "object",
        "required": [
          "deal_id",
          "first_lap_index",
          "last_lap_index",
          "cumulative_tokens",
          "batch_digest",
          "countersign_quorum"
        ],
        "additionalProperties": false,
        "properties": {
          "deal_id": {
            "type": "string",
            "pattern": "^[0-9a-f]{64}$"
          },
          "first_lap_index": {
            "type": "string",
            "pattern": "^(0|[1-9][0-9]*)$"
          },
          "last_lap_index": {
            "type": "string",
            "pattern": "^(0|[1-9][0-9]*)$"
          },
          "cumulative_tokens": {
            "type": "string",
            "pattern": "^(0|[1-9][0-9]*)$"
          },
          "batch_digest": {
            "type": "string",
            "pattern": "^[0-9a-f]{64}$"
          },
          "countersign_quorum": {
            "type": "integer",
            "minimum": 0
          }
        }
      },
      "maxItems": 16
    }
  },
  "description": "Node root read of the deployed ai-market F11 program at one committed height. IDs are 32-byte lowercase hexadecimal without a prefix. The node must configure WithPxInferenceProgram with the deployed program and a bound read identity. Integers wider than 32 bits are decimal strings. Extra parameters are refused. The producer retains the current receipt batch per deal; this returns that batch, or an empty list before settlement. An unavailable program, stream or binding is an explicit -32001 refusal. Hosted operators must route these reads to the configured node root.",
  "errors": [
    {
      "code": -32602,
      "message": "Invalid params"
    },
    {
      "code": -32001,
      "message": "Paxeer read unavailable"
    }
  ]
}
```

## Client cutover order

Clients send inference reads to the configured node root.
