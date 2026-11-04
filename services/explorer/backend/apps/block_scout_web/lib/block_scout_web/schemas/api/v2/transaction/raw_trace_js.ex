defmodule BlockScoutWeb.Schemas.API.V2.Transaction.RawTraceJS do
  @moduledoc false
  require OpenApiSpex

  alias BlockScoutWeb.Schemas.API.V2.General
  alias OpenApiSpex.Schema

  OpenApiSpex.schema(%Schema{
    title: "RawTraceJS",
    description: "A flat frame from Blockscout's configured Geth JavaScript tracer.",
    type: :object,
    properties: %{
      type: %Schema{type: :string, enum: ~w(call create create2 selfdestruct stop invalid)},
      callType: %Schema{type: :string, enum: ~w(call callcode delegatecall staticcall)},
      from: General.AddressHash,
      to: General.AddressHash,
      gas: General.HexString,
      gasUsed: General.HexString,
      input: General.HexString,
      output: General.HexString,
      init: General.HexString,
      value: General.HexString,
      createdContractAddressHash: General.AddressHash,
      createdContractCode: General.HexString,
      error: %Schema{type: :string},
      traceAddress: %Schema{type: :array, items: %Schema{type: :integer, minimum: 0}}
    },
    required: [:type, :from, :gas, :gasUsed, :traceAddress],
    additionalProperties: false
  })
end
