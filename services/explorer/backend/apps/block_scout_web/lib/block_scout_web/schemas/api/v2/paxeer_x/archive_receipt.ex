defmodule BlockScoutWeb.Schemas.API.V2.PaxeerX.ArchiveReceipt do
  require OpenApiSpex

  alias BlockScoutWeb.Schemas.API.V2.PaxeerX.Receipt.Details
  alias BlockScoutWeb.Schemas.Helper
  alias OpenApiSpex.Schema

  OpenApiSpex.schema(
    Details.schema()
    |> Map.delete(:"x-struct")
    |> Map.update!(:properties, &Map.delete(&1, :status))
    |> Map.update!(:required, &List.delete(&1, :status))
    |> Helper.extend_schema(
      title: "PaxeerXArchiveReceipt",
      description: "An archived receipt with its provenance, verification details and a required frozen marker.",
      properties: %{frozen: %Schema{type: :boolean, enum: [true], nullable: false}},
      required: [:frozen]
    )
  )
end
