defmodule BlockScoutWeb.Schemas.API.V2.Transaction.RawTraceOpcode do
  @moduledoc false
  require OpenApiSpex

  alias OpenApiSpex.Schema

  word = %Schema{type: :string, pattern: "^[a-fA-F0-9]{64}$"}
  non_negative = %Schema{type: :integer, minimum: 0}

  OpenApiSpex.schema(%Schema{
    title: "RawTraceOpcode",
    description: "The Geth struct logger result used by opcode and polygon_edge tracing.",
    type: :object,
    properties: %{
      gas: non_negative,
      failed: %Schema{type: :boolean},
      returnValue: %Schema{type: :string, pattern: "^(0x)?([a-fA-F0-9]{2})*$"},
      structLogs: %Schema{
        type: :array,
        items: %Schema{
          type: :object,
          properties: %{
            pc: non_negative,
            op: %Schema{type: :string},
            gas: non_negative,
            gasCost: non_negative,
            depth: non_negative,
            error: %Schema{type: :string},
            refund: non_negative,
            refundCounter: non_negative,
            memSize: non_negative,
            stack: %Schema{
              type: :array,
              items: %Schema{
                anyOf: [word, %Schema{type: :string, pattern: "^0x[a-fA-F0-9]{1,64}$"}]
              }
            },
            memory: %Schema{type: :array, items: word},
            returnData: %Schema{type: :string, pattern: "^(0x)?([a-fA-F0-9]{2})*$"},
            storage: %Schema{type: :object, additionalProperties: word}
          },
          required: [:pc, :op, :gas, :gasCost, :depth],
          additionalProperties: false
        }
      }
    },
    required: [:gas, :failed, :returnValue, :structLogs],
    additionalProperties: false
  })
end
