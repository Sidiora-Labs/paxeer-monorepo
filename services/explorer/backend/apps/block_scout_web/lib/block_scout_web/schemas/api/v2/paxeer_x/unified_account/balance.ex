defmodule BlockScoutWeb.Schemas.API.V2.PaxeerX.UnifiedAccount.Balance do
  @moduledoc """
  This module defines the schema for one asset of a unified account and its total balance.
  """
  require OpenApiSpex

  alias BlockScoutWeb.Schemas.API.V2.General
  alias BlockScoutWeb.Schemas.API.V2.PaxeerX.UnifiedAccount.Asset

  OpenApiSpex.schema(%{
    title: "PaxeerXBalance",
    description:
      "One asset of the account. Amounts are raw units: the native coin in wei, a token in its own " <>
        "smallest unit.",
    type: :object,
    properties: %{
      asset: Asset,
      total: General.IntegerString
    },
    required: [:asset, :total],
    nullable: false,
    additionalProperties: false
  })
end
