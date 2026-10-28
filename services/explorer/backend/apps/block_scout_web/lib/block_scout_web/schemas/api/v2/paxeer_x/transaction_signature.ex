defmodule BlockScoutWeb.Schemas.API.V2.PaxeerX.TransactionSignature do
  @moduledoc false
  require OpenApiSpex

  alias BlockScoutWeb.Schemas.API.V2.General
  alias BlockScoutWeb.Schemas.API.V2.PaxeerX.SettlementRung
  alias OpenApiSpex.Schema

  OpenApiSpex.schema(%{
    title: "PaxeerXTransactionSignature",
    type: :object,
    properties: %{
      rung: SettlementRung,
      committed_height: %Schema{
        type: :integer,
        nullable: true,
        description: "Height of the commit verified by R7 and R8"
      },
      receipt: %Schema{
        type: :string,
        pattern: "^0x([A-Fa-f0-9]{2})+$",
        description: "Signed sequencer receipt bytes encoded as 0x-prefixed hex"
      },
      receipt_key: %Schema{
        type: :object,
        properties: %{
          key_id: General.FullHash,
          public_key: General.FullHash
        },
        required: [:key_id, :public_key],
        additionalProperties: false
      },
      signature: %Schema{
        type: :string,
        pattern: "^0x[A-Fa-f0-9]{128}$",
        description: "64-byte Ed25519 signature encoded as 0x-prefixed hex"
      }
    },
    required: [:rung, :receipt, :receipt_key, :signature],
    nullable: false,
    additionalProperties: false
  })
end
