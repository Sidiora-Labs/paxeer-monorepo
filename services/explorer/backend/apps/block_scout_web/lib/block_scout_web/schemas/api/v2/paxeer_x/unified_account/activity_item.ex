defmodule BlockScoutWeb.Schemas.API.V2.PaxeerX.UnifiedAccount.ActivityItem do
  @moduledoc """
  This module defines the schema for one item of the unified account activity feed.
  """
  require OpenApiSpex

  alias BlockScoutWeb.Schemas.API.V2.General
  alias BlockScoutWeb.Schemas.API.V2.PaxeerX.SettlementRung
  alias BlockScoutWeb.Schemas.API.V2.PaxeerX.UnifiedAccount.AssetNullable
  alias OpenApiSpex.Schema

  @chain_kinds ["transaction", "token_transfer"]

  OpenApiSpex.schema(%{
    title: "PaxeerXActivityItem",
    description:
      "One item of the chain's transaction and token transfer feed, naming the asset it moves and " <>
        "the rung it has reached on the settlement ladder.",
    type: :object,
    properties: %{
      kind: %Schema{
        type: :string,
        enum: @chain_kinds,
        nullable: false,
        description: "What the item is"
      },
      hash: General.FullHash,
      block_number: %Schema{type: :integer, nullable: false, description: "Block the item was recorded in"},
      ordinal: %Schema{type: :integer, minimum: 0, description: "Transaction index or log index within the block"},
      status: SettlementRung,
      result: %Schema{
        type: :string,
        enum: ["ok", "error"],
        nullable: true,
        description: "Execution result of the transaction"
      },
      timestamp: General.TimestampNullable,
      asset: AssetNullable,
      amount: General.IntegerStringNullable,
      counterparty: %Schema{
        type: :string,
        nullable: true,
        description: "EVM address hash of the other party"
      }
    },
    required: [:kind, :hash, :block_number, :ordinal, :status, :result, :timestamp, :asset, :amount, :counterparty],
    nullable: false,
    additionalProperties: false
  })
end
