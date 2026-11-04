defmodule BlockScoutWeb.Schemas.API.V2.PaxeerX.UnifiedAccount.Asset do
  @moduledoc """
  This module defines the schema for the asset an amount of the unified account is denominated
  in.
  """
  require OpenApiSpex

  alias OpenApiSpex.Schema

  OpenApiSpex.schema(%{
    title: "PaxeerXAsset",
    description:
      "The asset an amount is denominated in: the native coin or an ERC-20 token on the chain. " <>
        "The native coin uses the configured coin symbol and 18 decimals. Token symbol and decimals " <>
        "are null when their metadata is unknown.",
    type: :object,
    properties: %{
      id: %Schema{
        type: :string,
        nullable: false,
        description: "`native` for the coin, or the checksummed ERC-20 token contract address"
      },
      denom: %Schema{
        type: :string,
        nullable: false,
        description:
          "Configured coin symbol for native amounts; token symbol when known, otherwise the checksummed token contract address"
      },
      symbol: %Schema{
        type: :string,
        nullable: true,
        description: "Configured coin symbol for the native coin; token symbol, or null when unknown"
      },
      decimals: %Schema{
        type: :integer,
        nullable: true,
        description: "18 for the native coin; token decimals, or null when unknown"
      }
    },
    example: %{id: "native", denom: "PAX", symbol: "PAX", decimals: 18},
    required: [:id, :denom, :symbol, :decimals],
    nullable: false,
    additionalProperties: false
  })
end
