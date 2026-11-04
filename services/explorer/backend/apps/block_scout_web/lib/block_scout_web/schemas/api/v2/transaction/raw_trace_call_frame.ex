defmodule BlockScoutWeb.Schemas.API.V2.Transaction.RawTraceCallFrame do
  @moduledoc false
  require OpenApiSpex

  alias BlockScoutWeb.Schemas.API.V2.General
  alias OpenApiSpex.Schema

  OpenApiSpex.schema(%Schema{
    title: "RawTraceCallFrame",
    description: "A Geth callTracer frame, returned unchanged by Geth, PaxeerX and RSK.",
    type: :object,
    properties: %{
      type: %Schema{
        type: :string,
        enum: ~w(CALL CALLCODE DELEGATECALL STATICCALL CREATE CREATE2 SELFDESTRUCT STOP INVALID)
      },
      from: General.AddressHash,
      to: General.AddressHash,
      gas: General.HexString,
      gasUsed: General.HexString,
      input: General.HexString,
      output: General.HexString,
      value: General.HexString,
      error: %Schema{type: :string},
      revertReason: %Schema{type: :string},
      calls: %Schema{type: :array, items: __MODULE__}
    },
    required: [:type, :from, :gas, :gasUsed, :input],
    additionalProperties: false
  })
end
