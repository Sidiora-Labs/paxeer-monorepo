defmodule BlockScoutWeb.Schemas.API.V2.PaxeerX.ArchiveAnchor do
  require OpenApiSpex

  alias BlockScoutWeb.Schemas.API.V2.General
  alias BlockScoutWeb.Schemas.Helper
  alias OpenApiSpex.Schema

  OpenApiSpex.schema(
    %{
      title: "PaxeerXArchiveAnchor",
      description:
        "One anchor checkpoint as the newest log of its batch reports it: the heights it records and " <>
          "seals, the state root it commits to and the block it was observed in.",
      type: :object,
      properties: %{
        batch_number: %Schema{type: :integer, nullable: false, description: "Batch the checkpoint records"},
        checkpoint_id: General.FullHash,
        checkpoint_height: %Schema{
          type: :integer,
          nullable: true,
          description: "Kernel height the checkpoint itself records, null while the log carries none"
        },
        sealed_height: %Schema{
          type: :integer,
          nullable: true,
          description: "Highest chain height the checkpoint seals, null while the log carries none"
        },
        state_root: General.FullHashNullable,
        block_number: %Schema{type: :integer, nullable: false, description: "Block the checkpoint was observed in"},
        timestamp: General.Timestamp
      },
      required: [
        :batch_number,
        :checkpoint_id,
        :checkpoint_height,
        :sealed_height,
        :state_root,
        :block_number,
        :timestamp
      ],
      nullable: false,
      additionalProperties: false
    }
    |> Helper.extend_schema(
      title: "PaxeerXArchiveAnchor",
      description: "An archived anchor checkpoint with a required frozen marker.",
      properties: %{frozen: %Schema{type: :boolean, enum: [true], nullable: false}},
      required: [:frozen]
    )
  )
end
