defmodule BlockScoutWeb.Schemas.API.V2.PaxeerX.ArchiveAnchor do
  require OpenApiSpex

  alias BlockScoutWeb.Schemas.API.V2.PaxeerX.Anchor
  alias BlockScoutWeb.Schemas.Helper
  alias OpenApiSpex.Schema

  OpenApiSpex.schema(
    Anchor.schema()
    |> Map.delete(:"x-struct")
    |> Helper.extend_schema(
      title: "PaxeerXArchiveAnchor",
      description: "An archived anchor checkpoint with a required frozen marker.",
      properties: %{frozen: %Schema{type: :boolean, enum: [true], nullable: false}},
      required: [:frozen]
    )
  )
end
