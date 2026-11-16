defmodule BlockScoutWeb.Schemas.API.V2.AdvancedFilter do
  @moduledoc false
  require OpenApiSpex

  alias BlockScoutWeb.Schemas.API.V2.{Address, AddressNullable, General, Token, TokenTransfer}
  alias OpenApiSpex.Schema

  properties = %{
    hash: General.FullHash,
    type: %Schema{
      type: :string,
      enum: ~w(coin_transfer contract_interaction contract_creation ERC-20 ERC-404 ERC-721 ERC-1155 ERC-7984 ZRC-2)
    },
    status: BlockScoutWeb.Schemas.API.V2.Transaction.Response.schema().properties.result,
    method: General.MethodNameNullable,
    from: Address,
    to: AddressNullable,
    created_contract: AddressNullable,
    value: General.IntegerStringNullable,
    total: %Schema{
      anyOf: [TokenTransfer.Total, TokenTransfer.TotalERC721, TokenTransfer.TotalERC1155, TokenTransfer.TotalERC7984],
      nullable: true
    },
    token: %Schema{allOf: [Token], nullable: true},
    timestamp: General.Timestamp,
    block_number: %Schema{type: :integer, minimum: 0},
    transaction_index: %Schema{type: :integer, minimum: 0},
    internal_transaction_index: %Schema{type: :integer, minimum: 0, nullable: true},
    token_transfer_index: %Schema{type: :integer, minimum: 0, nullable: true},
    token_transfer_batch_index: %Schema{type: :integer, minimum: 0, nullable: true},
    fee: General.IntegerStringNullable
  }

  OpenApiSpex.schema(%Schema{
    title: "AdvancedFilter",
    type: :object,
    properties: properties,
    required: Map.keys(properties),
    additionalProperties: false
  })
end
