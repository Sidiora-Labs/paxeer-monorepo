defmodule BlockScoutWeb.Schemas.API.V2.Transaction.RawTrace do
  @moduledoc """
  This module defines the schema for a transaction raw trace API response.
  """
  require OpenApiSpex

  alias BlockScoutWeb.Schemas.API.V2.General
  alias BlockScoutWeb.Schemas.API.V2.Transaction.{RawTraceCallFrame, RawTraceJS, RawTraceOpcode}
  alias OpenApiSpex.Schema

  action_schema = %Schema{
    type: :object,
    properties: %{
      callType: %Schema{type: :string, enum: ["call", "callcode", "delegatecall", "staticcall"]},
      from: General.AddressHash,
      gas: General.HexString,
      input: General.HexString,
      init: General.HexString,
      to: General.AddressHash,
      value: General.HexString,
      address: General.AddressHash,
      balance: General.HexString,
      refundAddress: General.AddressHash,
      author: General.AddressHash,
      rewardType: %Schema{type: :string, enum: ["block", "uncle", "emptyStep", "external"]}
    },
    additionalProperties: false
  }

  action_schema = %Schema{
    oneOf:
      Enum.map(
        [
          [:from, :to, :gas, :input, :value, :callType],
          [:from, :gas, :init, :value],
          [:address, :balance, :refundAddress],
          [:author, :rewardType, :value]
        ],
        &%Schema{action_schema | required: &1}
      )
  }

  result_schema = %Schema{
    type: :object,
    properties: %{
      gasUsed: General.HexString,
      output: General.HexString,
      address: General.AddressHash,
      code: General.HexString
    },
    additionalProperties: false
  }

  result_schema = %Schema{
    oneOf: [
      %Schema{result_schema | required: [:gasUsed, :output]},
      %Schema{result_schema | required: [:gasUsed, :address, :code]}
    ]
  }

  trace_schema = %Schema{
    type: :object,
    properties: %{
      action: action_schema,
      result: result_schema,
      error: %Schema{type: :string},
      subtraces: %Schema{type: :integer, minimum: 0},
      traceAddress: %Schema{type: :array, items: %Schema{type: :integer, minimum: 0}},
      transactionHash: General.FullHashNullable,
      type: %Schema{type: :string, enum: ["call", "create", "create2", "reward", "selfdestruct", "stop", "invalid"]}
    },
    required: [:action, :subtraces, :traceAddress, :type],
    additionalProperties: false
  }

  OpenApiSpex.schema(
    %Schema{
      description: "Unmodified node trace: callTracer tree, Parity traces, JavaScript frames or opcode logs.",
      anyOf: [
        RawTraceCallFrame,
        %Schema{type: :array, items: trace_schema},
        %Schema{type: :array, items: RawTraceJS},
        RawTraceOpcode
      ]
    },
    struct?: false
  )
end
